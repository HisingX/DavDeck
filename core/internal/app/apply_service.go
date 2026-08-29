package app

import (
	"bytes"
	"context"
	"errors"
	"strings"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type SnapshotProvider interface {
	Snapshot(context.Context) (domain.RuntimeConfigInput, error)
}
type ConfigCompiler interface {
	Compile(domain.RuntimeConfigInput) (caddyruntime.CompiledConfig, error)
}
type RevisionRepository interface {
	Create(context.Context, domain.ConfigRevision) (domain.ConfigRevision, error)
	FindByHash(context.Context, string) (domain.ConfigRevision, bool, error)
	SetDesired(context.Context, domain.ID, domain.Timestamp) error
	MarkApplied(context.Context, domain.ID, domain.Timestamp) error
	MarkFailed(context.Context, domain.ID, string, string) error
	RestoreState(context.Context, domain.RuntimeConfigInput, domain.ID, domain.Timestamp) error
	Delete(context.Context, domain.ID) error
	List(context.Context) ([]domain.ConfigRevision, error)
	Get(context.Context, domain.ID) (domain.ConfigRevision, error)
	Active(context.Context) (domain.ConfigRevision, bool, error)
	State(context.Context) (RevisionState, error)
}

type ValidationResult struct {
	Valid      bool
	ConfigHash string
	Warnings   []string
}
type CaddyRuntime interface {
	Start(context.Context, []byte) error
	Reload(context.Context, []byte) error
	Stop(context.Context) error
	Status(context.Context) caddyruntime.RuntimeState
}

type RevisionState struct {
	DesiredRevision *uint64 `json:"desired_revision"`
	ActiveRevision  *uint64 `json:"active_revision"`
	Dirty           bool    `json:"dirty"`
	Pending         bool    `json:"pending"`
}

type ApplyService struct {
	snapshots  SnapshotProvider
	compiler   ConfigCompiler
	validator  caddyruntime.Validator
	runtime    CaddyRuntime
	revisions  RevisionRepository
	ids        IDGenerator
	clock      Clock
	appVersion string
	lock       chan struct{}
}

func NewApplyService(snapshots SnapshotProvider, compiler ConfigCompiler, validator caddyruntime.Validator, runtime CaddyRuntime, revisions RevisionRepository, ids IDGenerator, clock Clock, appVersion string) *ApplyService {
	return &ApplyService{snapshots: snapshots, compiler: compiler, validator: validator, runtime: runtime, revisions: revisions, ids: ids, clock: clock, appVersion: appVersion, lock: make(chan struct{}, 1)}
}

func (s *ApplyService) Apply(ctx context.Context) (domain.ConfigRevision, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return domain.ConfigRevision{}, &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	return s.apply(ctx)
}

// Validate compiles and validates the desired state without persisting a
// revision or changing the managed runtime.
func (s *ApplyService) Validate(ctx context.Context) (ValidationResult, error) {
	snapshot, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		return ValidationResult{}, databaseError(err)
	}
	compiled, err := s.compiler.Compile(snapshot)
	if err != nil {
		return ValidationResult{}, &Error{Code: CodeCaddyValidateFailed, Message: "Desired state could not be compiled", Cause: err}
	}
	if err := s.validator.Validate(ctx, compiled.JSON); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyValidateFailed, "Caddy rejected the generated configuration")
		return ValidationResult{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	return ValidationResult{Valid: true, ConfigHash: compiled.SHA256, Warnings: append([]string(nil), compiled.Warnings...)}, nil
}

func (s *ApplyService) apply(ctx context.Context) (domain.ConfigRevision, error) {
	snapshot, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	compiled, err := s.compiler.Compile(snapshot)
	if err != nil {
		return domain.ConfigRevision{}, &Error{Code: CodeCaddyValidateFailed, Message: "Desired state could not be compiled", Cause: err}
	}
	if err := s.validator.Validate(ctx, compiled.JSON); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyValidateFailed, "Caddy rejected the generated configuration")
		return domain.ConfigRevision{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}

	stateSnapshot, err := domain.MarshalConfigRevisionSnapshot(snapshot)
	if err != nil {
		return domain.ConfigRevision{}, &Error{Code: CodeDatabase, Message: "Desired state could not be snapshotted", Cause: err}
	}

	revision, found, err := s.revisions.FindByHash(ctx, compiled.SHA256)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	created := !found || !bytes.Equal(revision.StateSnapshotJSON, stateSnapshot)
	if created {
		revision, err = s.newRevision(compiled, stateSnapshot)
		if err != nil {
			return domain.ConfigRevision{}, databaseError(err)
		}
		revision.ValidationStatus = domain.RevisionValidationValid
	}
	previousActive, hadActive, err := s.revisions.Active(ctx)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	if created {
		revision, err = s.revisions.Create(ctx, revision)
	} else {
		err = s.revisions.SetDesired(ctx, revision.ID, stamp)
	}
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}

	if hadActive && previousActive.ConfigHash == compiled.SHA256 && s.runtime.Status(ctx) == caddyruntime.RuntimeRunning {
		if err := s.revisions.MarkApplied(ctx, revision.ID, stamp); err != nil {
			if created {
				_ = s.revisions.MarkFailed(ctx, revision.ID, string(CodeDatabase), "Runtime activation metadata could not be saved")
			}
			return revision, databaseError(err)
		}
		revision.ApplyStatus = domain.RevisionApplyApplied
		return revision, nil
	}
	err = s.activate(ctx, compiled.JSON)
	if err == nil && s.runtime.Status(ctx) != caddyruntime.RuntimeRunning {
		err = &caddyruntime.RuntimeError{Code: caddyruntime.CodeRuntimeUnhealthy, Message: "Caddy runtime health check failed"}
	}
	if err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyReloadFailed, "Caddy could not apply the configuration")
		if created {
			if markErr := s.revisions.MarkFailed(ctx, revision.ID, string(code), summary); markErr != nil {
				return revision, databaseError(markErr)
			}
			revision.ApplyStatus, revision.ErrorCode, revision.ErrorSummary = domain.RevisionApplyFailed, string(code), summary
		}
		return revision, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	if err := s.revisions.MarkApplied(ctx, revision.ID, stamp); err != nil {
		if hadActive {
			_ = s.runtime.Reload(ctx, previousActive.ConfigJSON)
		} else {
			_ = s.runtime.Stop(ctx)
		}
		if created {
			_ = s.revisions.MarkFailed(ctx, revision.ID, string(CodeDatabase), "Runtime activation metadata could not be saved")
		}
		return revision, databaseError(err)
	}
	revision.ApplyStatus = domain.RevisionApplyApplied
	return revision, nil
}

// Start starts the current active revision. It only creates a revision for the
// first-ever activation when no active revision exists yet.
func (s *ApplyService) Start(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	active, found, err := s.revisions.Active(ctx)
	if err != nil {
		return databaseError(err)
	}
	if !found {
		_, err := s.apply(ctx)
		return err
	}
	if len(active.StateSnapshotJSON) == 0 {
		_, err := s.apply(ctx)
		return err
	}
	if err := s.activateStart(ctx, active.ConfigJSON); err != nil {
		return err
	}
	return nil
}

// Stop stops only the managed Caddy runtime; davd and its Management API remain available.
func (s *ApplyService) Stop(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	if err := s.runtime.Stop(ctx); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyStopFailed, "Caddy could not stop")
		return &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	return nil
}

// Restart stops and starts the current active revision. Pending desired
// configuration remains pending until an explicit Apply.
func (s *ApplyService) Restart(ctx context.Context) error {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	if err := s.runtime.Stop(ctx); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyStopFailed, "Caddy could not stop")
		return &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	active, found, err := s.revisions.Active(ctx)
	if err != nil {
		return databaseError(err)
	}
	if !found {
		_, err := s.apply(ctx)
		return err
	}
	if len(active.StateSnapshotJSON) == 0 {
		_, err := s.apply(ctx)
		return err
	}
	if err := s.activateStart(ctx, active.ConfigJSON); err != nil {
		return err
	}
	return nil
}

func (s *ApplyService) activate(ctx context.Context, configuration []byte) error {
	state := s.runtime.Status(ctx)
	if state == caddyruntime.RuntimeStopped || state == caddyruntime.RuntimeFailed || state == caddyruntime.RuntimeUnknown || state == caddyruntime.RuntimeNotInstalled {
		return s.runtime.Start(ctx, configuration)
	}
	return s.runtime.Reload(ctx, configuration)
}

func (s *ApplyService) activateStart(ctx context.Context, configuration []byte) error {
	if err := s.runtime.Start(ctx, configuration); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyStartFailed, "Caddy could not start")
		return &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	if s.runtime.Status(ctx) != caddyruntime.RuntimeRunning {
		return &Error{Code: CodeRuntimeUnhealthy, Message: "Caddy runtime health check failed"}
	}
	return nil
}

func (s *ApplyService) RuntimeStatus(ctx context.Context) caddyruntime.RuntimeState {
	return s.runtime.Status(ctx)
}

// RuntimeStatusSnapshot exposes the daemon-owned runtime details without
// making the API depend on a concrete Caddy process manager. Older test and
// alternate runtimes can still provide the legacy state method.
func (s *ApplyService) RuntimeStatusSnapshot(ctx context.Context) caddyruntime.RuntimeSnapshot {
	if provider, ok := s.runtime.(interface {
		StatusSnapshot(context.Context) caddyruntime.RuntimeSnapshot
	}); ok {
		return provider.StatusSnapshot(ctx)
	}
	state := s.runtime.Status(ctx)
	return caddyruntime.RuntimeSnapshot{Caddy: state, WebDAV: state}
}

func (s *ApplyService) State(ctx context.Context) (RevisionState, error) {
	state, err := s.revisions.State(ctx)
	if err != nil {
		return RevisionState{}, databaseError(err)
	}
	return state, nil
}
func (s *ApplyService) List(ctx context.Context) ([]domain.ConfigRevision, error) {
	revisions, err := s.revisions.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	return revisions, nil
}
func (s *ApplyService) Get(ctx context.Context, id domain.ID) (domain.ConfigRevision, error) {
	revision, err := s.revisions.Get(ctx, id)
	if errors.Is(err, ErrRevisionNotFound) {
		return domain.ConfigRevision{}, &Error{Code: CodeRevisionNotFound, Message: "Revision was not found", Cause: err}
	}
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	return revision, nil
}

func (s *ApplyService) Delete(ctx context.Context, id domain.ID) error {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	if err := s.revisions.Delete(ctx, id); err != nil {
		switch {
		case errors.Is(err, ErrRevisionNotFound):
			return &Error{Code: CodeRevisionNotFound, Message: "Revision was not found", Cause: err}
		case errors.Is(err, ErrRevisionActive):
			return &Error{Code: CodeRevisionActive, Message: "The active configuration revision cannot be deleted", Cause: err}
		case errors.Is(err, ErrRevisionDesired):
			return &Error{Code: CodeRevisionDesired, Message: "The desired configuration revision cannot be deleted", Cause: err}
		default:
			return databaseError(err)
		}
	}
	return nil
}

// Restore validates and activates a complete previously generated revision,
// then restores the matching authoritative application state. The restored
// revision becomes both the desired and active revision so the configuration
// state does not report a false pending change.
func (s *ApplyService) Restore(ctx context.Context, id domain.ID) (domain.ConfigRevision, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return domain.ConfigRevision{}, &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	revision, err := s.Get(ctx, id)
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	if revision.ValidationStatus != domain.RevisionValidationValid {
		return domain.ConfigRevision{}, &Error{Code: CodeCaddyValidateFailed, Message: "Only a valid configuration revision can be restored"}
	}
	if len(revision.StateSnapshotJSON) == 0 {
		return domain.ConfigRevision{}, &Error{Code: CodeRevisionStateUnavailable, Message: "This revision does not contain a complete application-state snapshot and cannot be safely restored"}
	}
	snapshot, err := domain.ParseConfigRevisionSnapshot(revision.StateSnapshotJSON)
	if err != nil {
		return domain.ConfigRevision{}, &Error{Code: CodeRevisionStateUnavailable, Message: "This revision contains an invalid application-state snapshot", Cause: err}
	}
	compiled, err := s.compiler.Compile(snapshot)
	if err != nil || compiled.SHA256 != revision.ConfigHash || !bytes.Equal(compiled.JSON, revision.ConfigJSON) {
		if err == nil {
			err = errors.New("stored application state does not match its generated configuration")
		}
		return domain.ConfigRevision{}, &Error{Code: CodeRevisionStateUnavailable, Message: "This revision cannot be safely restored because its application state and runtime configuration do not match", Cause: err}
	}
	if err := s.validator.Validate(ctx, compiled.JSON); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyValidateFailed, "Caddy rejected the stored configuration revision")
		return domain.ConfigRevision{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	previousActive, hadActive, err := s.revisions.Active(ctx)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	if state := s.runtime.Status(ctx); state == caddyruntime.RuntimeStopped || state == caddyruntime.RuntimeFailed || state == caddyruntime.RuntimeUnknown || state == caddyruntime.RuntimeNotInstalled {
		err = s.runtime.Start(ctx, compiled.JSON)
	} else {
		err = s.runtime.Reload(ctx, compiled.JSON)
	}
	if err == nil && s.runtime.Status(ctx) != caddyruntime.RuntimeRunning {
		err = &caddyruntime.RuntimeError{Code: caddyruntime.CodeRuntimeUnhealthy, Message: "Caddy runtime health check failed"}
	}
	if err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyReloadFailed, "Caddy could not restore the configuration")
		return domain.ConfigRevision{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	if err := s.revisions.RestoreState(ctx, snapshot, revision.ID, stamp); err != nil {
		if hadActive {
			_ = s.runtime.Reload(ctx, previousActive.ConfigJSON)
		} else {
			_ = s.runtime.Stop(ctx)
		}
		return domain.ConfigRevision{}, databaseError(err)
	}
	revision.ApplyStatus = domain.RevisionApplyApplied
	revision.ErrorCode, revision.ErrorSummary = "", ""
	return revision, nil
}

func (s *ApplyService) newRevision(compiled caddyruntime.CompiledConfig, stateSnapshot []byte) (domain.ConfigRevision, error) {
	id, err := s.ids.NewID()
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	return domain.ConfigRevision{ID: id, CreatedAt: stamp, ConfigJSON: append([]byte(nil), compiled.JSON...), StateSnapshotJSON: append([]byte(nil), stateSnapshot...), ConfigHash: compiled.SHA256, ValidationStatus: domain.RevisionValidationPending, ApplyStatus: domain.RevisionApplyNotApplied, AppVersion: s.appVersion}, nil
}

func safeRuntimeFailure(err error, fallback caddyruntime.RuntimeErrorCode, fallbackSummary string) (caddyruntime.RuntimeErrorCode, string) {
	code, summary := fallback, fallbackSummary
	var runtimeError *caddyruntime.RuntimeError
	if errors.As(err, &runtimeError) {
		code, summary = runtimeError.Code, runtimeError.Message
	}
	summary = strings.Map(func(value rune) rune {
		if value < 0x20 || value == 0x7f {
			return -1
		}
		return value
	}, strings.TrimSpace(summary))
	characters := []rune(summary)
	if len(characters) > 512 {
		summary = string(characters[:512])
	}
	if summary == "" {
		summary = fallbackSummary
	}
	return code, summary
}

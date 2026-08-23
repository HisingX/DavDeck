package app

import (
	"context"
	"errors"
	"strings"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

var ErrRevisionNotFound = errors.New("revision not found")

type SnapshotProvider interface {
	Snapshot(context.Context) (domain.RuntimeConfigInput, error)
}
type ConfigCompiler interface {
	Compile(domain.RuntimeConfigInput) (caddyruntime.CompiledConfig, error)
}
type RevisionRepository interface {
	Create(context.Context, domain.ConfigRevision) (domain.ConfigRevision, error)
	MarkApplied(context.Context, domain.ID, domain.Timestamp) error
	MarkFailed(context.Context, domain.ID, string, string) error
	MarkRestored(context.Context, domain.ID, domain.Timestamp) error
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
	revision, err := s.newRevision(compiled)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	if err := s.validator.Validate(ctx, compiled.JSON); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyValidateFailed, "Caddy rejected the generated configuration")
		revision.ValidationStatus, revision.ErrorCode, revision.ErrorSummary = domain.RevisionValidationInvalid, string(code), summary
		revision, createErr := s.revisions.Create(ctx, revision)
		if createErr != nil {
			return domain.ConfigRevision{}, databaseError(createErr)
		}
		return revision, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	revision.ValidationStatus = domain.RevisionValidationValid
	previousActive, hadActive, err := s.revisions.Active(ctx)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	revision, err = s.revisions.Create(ctx, revision)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	state := s.runtime.Status(ctx)
	if state == caddyruntime.RuntimeStopped || state == caddyruntime.RuntimeFailed || state == caddyruntime.RuntimeUnknown || state == caddyruntime.RuntimeNotInstalled {
		err = s.runtime.Start(ctx, compiled.JSON)
	} else {
		err = s.runtime.Reload(ctx, compiled.JSON)
	}
	if err == nil && s.runtime.Status(ctx) != caddyruntime.RuntimeRunning {
		err = &caddyruntime.RuntimeError{Code: caddyruntime.CodeRuntimeUnhealthy, Message: "Caddy runtime health check failed"}
	}
	if err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyReloadFailed, "Caddy could not apply the configuration")
		if markErr := s.revisions.MarkFailed(ctx, revision.ID, string(code), summary); markErr != nil {
			return revision, databaseError(markErr)
		}
		revision.ApplyStatus, revision.ErrorCode, revision.ErrorSummary = domain.RevisionApplyFailed, string(code), summary
		return revision, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	stamp, stampErr := domain.NewTimestamp(s.clock.Now())
	if stampErr != nil {
		return revision, databaseError(stampErr)
	}
	if err := s.revisions.MarkApplied(ctx, revision.ID, stamp); err != nil {
		if hadActive {
			_ = s.runtime.Reload(ctx, previousActive.ConfigJSON)
		} else {
			_ = s.runtime.Stop(ctx)
		}
		_ = s.revisions.MarkFailed(ctx, revision.ID, string(CodeDatabase), "Runtime activation metadata could not be saved")
		return revision, databaseError(err)
	}
	revision.ApplyStatus = domain.RevisionApplyApplied
	return revision, nil
}

// Start compiles and activates the current desired state when the runtime is stopped.
func (s *ApplyService) Start(ctx context.Context) (domain.ConfigRevision, error) { return s.Apply(ctx) }

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

// Restart stops the managed runtime then validates and activates current desired state under one lock.
func (s *ApplyService) Restart(ctx context.Context) (domain.ConfigRevision, error) {
	select {
	case s.lock <- struct{}{}:
		defer func() { <-s.lock }()
	default:
		return domain.ConfigRevision{}, &Error{Code: CodeApplyInProgress, Message: "Another configuration apply is in progress"}
	}
	if err := s.runtime.Stop(ctx); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyStopFailed, "Caddy could not stop")
		return domain.ConfigRevision{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	return s.apply(ctx)
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

// Restore validates and activates a previously generated valid revision. The
// restored revision becomes both the desired and active revision so the
// configuration state does not report a false pending change.
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
	if err := s.validator.Validate(ctx, revision.ConfigJSON); err != nil {
		code, summary := safeRuntimeFailure(err, caddyruntime.CodeCaddyValidateFailed, "Caddy rejected the stored configuration revision")
		return domain.ConfigRevision{}, &Error{Code: ErrorCode(code), Message: summary, Cause: err}
	}
	previousActive, hadActive, err := s.revisions.Active(ctx)
	if err != nil {
		return domain.ConfigRevision{}, databaseError(err)
	}
	if state := s.runtime.Status(ctx); state == caddyruntime.RuntimeStopped || state == caddyruntime.RuntimeFailed || state == caddyruntime.RuntimeUnknown || state == caddyruntime.RuntimeNotInstalled {
		err = s.runtime.Start(ctx, revision.ConfigJSON)
	} else {
		err = s.runtime.Reload(ctx, revision.ConfigJSON)
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
	if err := s.revisions.MarkRestored(ctx, revision.ID, stamp); err != nil {
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

func (s *ApplyService) newRevision(compiled caddyruntime.CompiledConfig) (domain.ConfigRevision, error) {
	id, err := s.ids.NewID()
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.ConfigRevision{}, err
	}
	return domain.ConfigRevision{ID: id, CreatedAt: stamp, ConfigJSON: append([]byte(nil), compiled.JSON...), ConfigHash: compiled.SHA256, ValidationStatus: domain.RevisionValidationPending, ApplyStatus: domain.RevisionApplyNotApplied, AppVersion: s.appVersion}, nil
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

package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type fakeSnapshots struct {
	input domain.RuntimeConfigInput
	err   error
}

func (f fakeSnapshots) Snapshot(context.Context) (domain.RuntimeConfigInput, error) {
	return f.input, f.err
}

type fakeCompiler struct {
	result caddyruntime.CompiledConfig
	err    error
}

func (f fakeCompiler) Compile(domain.RuntimeConfigInput) (caddyruntime.CompiledConfig, error) {
	return f.result, f.err
}

type fakeValidator struct {
	err     error
	entered chan struct{}
	release chan struct{}
}

func (f *fakeValidator) Validate(context.Context, []byte) error {
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	return f.err
}

type fakeRuntime struct {
	state               caddyruntime.RuntimeState
	startErr, reloadErr error
	started, reloaded   bool
}

func (f *fakeRuntime) Start(context.Context, []byte) error {
	f.started = true
	if f.startErr == nil {
		f.state = caddyruntime.RuntimeRunning
	}
	return f.startErr
}
func (f *fakeRuntime) Reload(context.Context, []byte) error             { f.reloaded = true; return f.reloadErr }
func (f *fakeRuntime) Stop(context.Context) error                       { f.state = caddyruntime.RuntimeStopped; return nil }
func (f *fakeRuntime) Status(context.Context) caddyruntime.RuntimeState { return f.state }

type environmentRuntime struct {
	fakeRuntime
	environment map[string]string
	starts      int
	stops       int
}

func (r *environmentRuntime) StartWithEnvironment(_ context.Context, _ []byte, environment map[string]string) error {
	r.starts++
	r.environment = cloneStringMap(environment)
	r.state = caddyruntime.RuntimeRunning
	return nil
}
func (r *environmentRuntime) Stop(ctx context.Context) error {
	r.stops++
	return r.fakeRuntime.Stop(ctx)
}
func (r *environmentRuntime) EnvironmentMatches(environment map[string]string) bool {
	if len(r.environment) != len(environment) {
		return false
	}
	for key, value := range environment {
		if r.environment[key] != value {
			return false
		}
	}
	return r.state == caddyruntime.RuntimeRunning
}
func (r *environmentRuntime) CurrentEnvironment() map[string]string {
	return cloneStringMap(r.environment)
}

type mutableRuntimeEnvironment struct{ values map[string]string }

func (e *mutableRuntimeEnvironment) Environment(context.Context, domain.RuntimeConfigInput) (map[string]string, error) {
	return cloneStringMap(e.values), nil
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

type memoryRevisions struct {
	mu             sync.Mutex
	values         []domain.ConfigRevision
	state          RevisionState
	markAppliedErr error
}

func (r *memoryRevisions) Create(_ context.Context, revision domain.ConfigRevision) (domain.ConfigRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	revision.Number = uint64(len(r.values) + 1)
	r.values = append(r.values, revision)
	r.state.DesiredRevision = &revision.Number
	r.state.Dirty = false
	r.state.Pending = true
	return revision, nil
}
func (r *memoryRevisions) FindByHash(_ context.Context, hash string) (domain.ConfigRevision, bool, error) {
	for index := len(r.values) - 1; index >= 0; index-- {
		if r.values[index].ConfigHash == hash && r.values[index].ValidationStatus == domain.RevisionValidationValid {
			return r.values[index], true, nil
		}
	}
	return domain.ConfigRevision{}, false, nil
}
func (r *memoryRevisions) SetDesired(_ context.Context, id domain.ID, _ domain.Timestamp) error {
	for _, value := range r.values {
		if value.ID == id {
			number := value.Number
			r.state.DesiredRevision, r.state.Dirty = &number, false
			r.state.Pending = r.state.ActiveRevision == nil || *r.state.ActiveRevision != number
			return nil
		}
	}
	return ErrRevisionNotFound
}
func (r *memoryRevisions) MarkApplied(_ context.Context, id domain.ID, _ domain.Timestamp) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markAppliedErr != nil {
		return r.markAppliedErr
	}
	for index := range r.values {
		if r.values[index].ID == id {
			r.values[index].ApplyStatus = domain.RevisionApplyApplied
			value := r.values[index].Number
			r.state.ActiveRevision = &value
			r.state.Pending = false
			return nil
		}
	}
	return ErrRevisionNotFound
}
func (r *memoryRevisions) MarkFailed(_ context.Context, id domain.ID, code, summary string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.values {
		if r.values[index].ID == id {
			r.values[index].ApplyStatus, r.values[index].ErrorCode, r.values[index].ErrorSummary = domain.RevisionApplyFailed, code, summary
			return nil
		}
	}
	return ErrRevisionNotFound
}
func (r *memoryRevisions) MarkRestored(_ context.Context, id domain.ID, _ domain.Timestamp) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.values {
		if r.values[index].ID == id {
			r.values[index].ApplyStatus, r.values[index].ErrorCode, r.values[index].ErrorSummary = domain.RevisionApplyApplied, "", ""
			value := r.values[index].Number
			r.state.DesiredRevision, r.state.ActiveRevision = &value, &value
			r.state.Dirty, r.state.Pending = false, false
			return nil
		}
	}
	return ErrRevisionNotFound
}
func (r *memoryRevisions) RestoreState(_ context.Context, _ domain.RuntimeConfigInput, id domain.ID, _ domain.Timestamp) error {
	return r.MarkRestored(context.Background(), id, domain.Timestamp{})
}
func (r *memoryRevisions) Delete(_ context.Context, id domain.ID) error {
	for _, value := range r.values {
		if value.ID != id {
			continue
		}
		if r.state.ActiveRevision != nil && *r.state.ActiveRevision == value.Number {
			return ErrRevisionActive
		}
		if r.state.DesiredRevision != nil && *r.state.DesiredRevision == value.Number {
			return ErrRevisionDesired
		}
		return nil
	}
	return ErrRevisionNotFound
}
func (r *memoryRevisions) List(context.Context) ([]domain.ConfigRevision, error) {
	return r.values, nil
}
func (r *memoryRevisions) Get(_ context.Context, id domain.ID) (domain.ConfigRevision, error) {
	for _, value := range r.values {
		if value.ID == id {
			return value, nil
		}
	}
	return domain.ConfigRevision{}, ErrRevisionNotFound
}
func (r *memoryRevisions) Active(context.Context) (domain.ConfigRevision, bool, error) {
	if r.state.ActiveRevision == nil {
		return domain.ConfigRevision{}, false, nil
	}
	for _, value := range r.values {
		if value.Number == *r.state.ActiveRevision {
			return value, true, nil
		}
	}
	return domain.ConfigRevision{}, false, ErrRevisionNotFound
}
func (r *memoryRevisions) State(context.Context) (RevisionState, error) { return r.state, nil }

func TestApplyServiceSuccessMarksRevisionActive(t *testing.T) {
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	repository := &memoryRevisions{}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	revision, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if revision.ValidationStatus != domain.RevisionValidationValid || revision.ApplyStatus != domain.RevisionApplyApplied || !runtime.started {
		t.Fatalf("revision = %#v, runtime = %#v", revision, runtime)
	}
	state, err := service.State(context.Background())
	if err != nil || state.Pending || state.ActiveRevision == nil {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
}

func TestApplyServiceRecordsValidationAndReloadFailures(t *testing.T) {
	validationRepository := &memoryRevisions{}
	validationError := &caddyruntime.RuntimeError{Code: caddyruntime.CodeCaddyValidateFailed, Message: "Generated configuration is invalid"}
	service := applyFixture(&fakeValidator{err: validationError}, &fakeRuntime{state: caddyruntime.RuntimeRunning}, validationRepository)
	revision, err := service.Apply(context.Background())
	if !hasCode(err, CodeCaddyValidateFailed) || revision.ID != "" || len(validationRepository.values) != 0 {
		t.Fatalf("revision = %#v, err = %v", revision, err)
	}
	reloadRepository := &memoryRevisions{}
	runtime := &fakeRuntime{state: caddyruntime.RuntimeRunning, reloadErr: &caddyruntime.RuntimeError{Code: caddyruntime.CodeCaddyReloadFailed, Message: "Reload failed"}}
	service = applyFixture(&fakeValidator{}, runtime, reloadRepository)
	revision, err = service.Apply(context.Background())
	if !hasCode(err, ErrorCode(caddyruntime.CodeCaddyReloadFailed)) || revision.ApplyStatus != domain.RevisionApplyFailed || reloadRepository.state.ActiveRevision != nil {
		t.Fatalf("revision = %#v, state = %#v, err = %v", revision, reloadRepository.state, err)
	}
}

func TestApplyServiceValidateDoesNotPersistOrStartRuntime(t *testing.T) {
	repository := &memoryRevisions{}
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	result, err := service.Validate(context.Background())
	if err != nil || !result.Valid || result.ConfigHash == "" || runtime.started || len(repository.values) != 0 {
		t.Fatalf("result=%#v err=%v runtime=%#v revisions=%d", result, err, runtime, len(repository.values))
	}
}

func TestApplyServiceRestoresValidRevision(t *testing.T) {
	repository := &memoryRevisions{}
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	revision, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	runtime.state = caddyruntime.RuntimeRunning
	restored, err := service.Restore(context.Background(), revision.ID)
	if err != nil || restored.ID != revision.ID || !runtime.reloaded || repository.state.Pending {
		t.Fatalf("restored=%#v err=%v runtime=%#v state=%#v", restored, err, runtime, repository.state)
	}
}

func TestApplyServiceRejectsRuntimeOnlyRevisionRestore(t *testing.T) {
	repository := &memoryRevisions{}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC))
	body := []byte("{}\n")
	repository.values = []domain.ConfigRevision{{
		ID: testUserID, Number: 1, CreatedAt: stamp, ConfigJSON: body,
		ConfigHash: domain.HashConfigJSON(body), ValidationStatus: domain.RevisionValidationValid,
		ApplyStatus: domain.RevisionApplyApplied, AppVersion: "test",
	}}
	repository.state.ActiveRevision = new(uint64)
	*repository.state.ActiveRevision = 1
	service := applyFixture(&fakeValidator{}, &fakeRuntime{state: caddyruntime.RuntimeRunning}, repository)
	_, err := service.Restore(context.Background(), testUserID)
	if !hasCode(err, CodeRevisionStateUnavailable) {
		t.Fatalf("error = %v, want %s", err, CodeRevisionStateUnavailable)
	}
}

func TestApplyServiceRejectsConcurrentApply(t *testing.T) {
	validator := &fakeValidator{entered: make(chan struct{}), release: make(chan struct{})}
	service := applyFixture(validator, &fakeRuntime{state: caddyruntime.RuntimeStopped}, &memoryRevisions{})
	done := make(chan error, 1)
	go func() { _, err := service.Apply(context.Background()); done <- err }()
	<-validator.entered
	if _, err := service.Apply(context.Background()); !hasCode(err, CodeApplyInProgress) {
		t.Fatalf("error = %v", err)
	}
	close(validator.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestApplyServiceStopsNewRuntimeWhenActivationMetadataFails(t *testing.T) {
	repository := &memoryRevisions{markAppliedErr: errors.New("write failed")}
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	revision, err := service.Apply(context.Background())
	if !hasCode(err, CodeDatabase) || runtime.state != caddyruntime.RuntimeStopped || revision.ApplyStatus == domain.RevisionApplyApplied {
		t.Fatalf("revision = %#v, runtime = %#v, err = %v", revision, runtime, err)
	}
}

func TestApplyServiceMapsRevisionNotFound(t *testing.T) {
	service := applyFixture(&fakeValidator{}, &fakeRuntime{}, &memoryRevisions{})
	_, err := service.Get(context.Background(), testUserID)
	if !hasCode(err, CodeRevisionNotFound) {
		t.Fatalf("error = %v", err)
	}
	if !errors.Is(err, ErrRevisionNotFound) {
		t.Fatal("revision cause was not retained")
	}
}

func TestApplyServiceRuntimeControlsUseValidatedApplyPath(t *testing.T) {
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	repository := &memoryRevisions{}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	if service.RuntimeStatus(context.Background()) != caddyruntime.RuntimeStopped {
		t.Fatal("initial runtime state was not stopped")
	}
	if err := service.Start(context.Background()); err != nil || !runtime.started || len(repository.values) != 1 {
		t.Fatalf("start err=%v runtime=%#v", err, runtime)
	}
	if err := service.Restart(context.Background()); err != nil || !runtime.started || len(repository.values) != 1 {
		t.Fatalf("restart err=%v runtime=%#v", err, runtime)
	}
	if err := service.Stop(context.Background()); err != nil || service.RuntimeStatus(context.Background()) != caddyruntime.RuntimeStopped {
		t.Fatalf("stop err=%v state=%s", err, service.RuntimeStatus(context.Background()))
	}
}

func TestApplyServiceRepeatedApplyReusesRevision(t *testing.T) {
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	repository := &memoryRevisions{}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	first, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.Number != second.Number || len(repository.values) != 1 {
		t.Fatalf("first=%#v second=%#v revisions=%d", first, second, len(repository.values))
	}
}

func TestApplyServiceRestartsCaddyWhenRuntimeEnvironmentChanges(t *testing.T) {
	runtime := &environmentRuntime{fakeRuntime: fakeRuntime{state: caddyruntime.RuntimeStopped}}
	environment := &mutableRuntimeEnvironment{values: map[string]string{"DAVDECK_DNS_TOKEN": "first"}}
	repository := &memoryRevisions{}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	service.SetRuntimeEnvironmentProvider(environment)
	if _, err := service.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	environment.values["DAVDECK_DNS_TOKEN"] = "second"
	if _, err := service.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if runtime.starts != 2 || runtime.stops != 1 || runtime.environment["DAVDECK_DNS_TOKEN"] != "second" {
		t.Fatalf("runtime starts=%d stops=%d environment=%#v", runtime.starts, runtime.stops, runtime.environment)
	}
}

func TestApplyServiceDoesNotDeleteActiveRevision(t *testing.T) {
	runtime := &fakeRuntime{state: caddyruntime.RuntimeStopped}
	repository := &memoryRevisions{}
	service := applyFixture(&fakeValidator{}, runtime, repository)
	revision, err := service.Apply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(context.Background(), revision.ID); !hasCode(err, CodeRevisionActive) {
		t.Fatalf("delete error = %v, want %s", err, CodeRevisionActive)
	}
}

func applyFixture(validator caddyruntime.Validator, runtime CaddyRuntime, revisions RevisionRepository) *ApplyService {
	body := []byte("{}\n")
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC))
	input := domain.RuntimeConfigInput{ServerSettings: domain.ServerSettings{ID: testUserID, PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}}
	return NewApplyService(fakeSnapshots{input: input}, fakeCompiler{result: caddyruntime.CompiledConfig{JSON: body, SHA256: domain.HashConfigJSON(body)}}, validator, runtime, revisions, fixedID{}, fixedClock{}, "test")
}

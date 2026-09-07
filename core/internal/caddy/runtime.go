package caddy

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"davdeck.dev/davdeck/core/internal/status"
)

type RuntimeState = status.State

const (
	RuntimeNotInstalled = status.StateNotInstalled
	RuntimeStopped      = status.StateStopped
	RuntimeStarting     = status.StateStarting
	RuntimeRunning      = status.StateRunning
	RuntimeStopping     = status.StateStopping
	RuntimeDegraded     = status.StateDegraded
	RuntimeFailed       = status.StateFailed
	RuntimeUnknown      = status.StateUnknown
)

// RuntimeSnapshot is the safe status returned to management clients. WebDAV
// is reported separately even though the first runtime implementation derives
// its health from the managed Caddy Admin endpoint.
type RuntimeSnapshot struct {
	Caddy         RuntimeState
	WebDAV        RuntimeState
	LastErrorCode RuntimeErrorCode
}

type RuntimeManager struct {
	mu                     sync.Mutex
	binaryPath             string
	configPath             string
	validator              Validator
	admin                  Admin
	stdout                 io.Writer
	stderr                 io.Writer
	command                *exec.Cmd
	done                   chan error
	lastErrorCode          RuntimeErrorCode
	environment            map[string]string
	storagePath            string
	certificateErrorReader CertificateErrorReader
	renewalTimeout         time.Duration
	renewalPollInterval    time.Duration
	renewals               map[string]*certificateRenewal
	logger                 *slog.Logger
	startTimeout           time.Duration
	stopTimeout            time.Duration
}

const (
	defaultRuntimeStartTimeout            = 15 * time.Second
	defaultRuntimeStopTimeout             = 5 * time.Second
	defaultCertificateRenewalTimeout      = 15 * time.Minute
	defaultCertificateRenewalPollInterval = time.Second
)

func NewRuntimeManager(binaryPath, configPath string, validator Validator, admin Admin, stdout, stderr io.Writer) *RuntimeManager {
	return &RuntimeManager{binaryPath: binaryPath, configPath: configPath, validator: validator, admin: admin, stdout: stdout, stderr: stderr, storagePath: defaultCaddyStoragePath(), renewalTimeout: defaultCertificateRenewalTimeout, renewalPollInterval: defaultCertificateRenewalPollInterval, renewals: make(map[string]*certificateRenewal), startTimeout: defaultRuntimeStartTimeout, stopTimeout: defaultRuntimeStopTimeout}
}

// SetLogger connects runtime lifecycle failures to the daemon-owned logging
// boundary without exposing raw Caddy command output.
func (m *RuntimeManager) SetLogger(logger *slog.Logger) { m.logger = logger }

// SetCertificateErrorReader connects the runtime to the sanitized Caddy log
// boundary. ACME errors generally do not make the Caddy process unhealthy, so
// this lets certificate status distinguish a retrying failure from issuance in
// progress without reading raw logs or exposing secret values.
func (m *RuntimeManager) SetCertificateErrorReader(reader CertificateErrorReader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.certificateErrorReader = reader
}

func (m *RuntimeManager) Start(ctx context.Context, configuration []byte) error {
	return m.StartWithEnvironment(ctx, configuration, nil)
}

// StartWithEnvironment starts Caddy with runtime-only secret values. The
// values are never written to the generated config or revision metadata.
func (m *RuntimeManager) StartWithEnvironment(ctx context.Context, configuration []byte, environment map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runningLocked() {
		return nil
	}
	m.cancelRenewalsLocked()
	if err := validateWithEnvironment(ctx, m.validator, configuration, environment); err != nil {
		return m.recordFailure(err)
	}
	if err := writeConfigAtomically(m.configPath, configuration); err != nil {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyStartFailed, Message: "Unable to save Caddy configuration", Cause: err})
	}
	command := exec.Command(m.binaryPath, "run", "--config", m.configPath)
	command.Env = environmentWithOverrides(environment)
	command.Stdout, command.Stderr = m.stdout, m.stderr
	if err := command.Start(); err != nil {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyStartFailed, Message: "Unable to start Caddy", Cause: err})
	}
	m.command, m.done = command, make(chan error, 1)
	go func() { m.done <- command.Wait() }()
	healthContext, cancel := context.WithTimeout(ctx, m.startTimeout)
	defer cancel()
	for {
		if err := m.admin.Health(healthContext); err == nil {
			m.lastErrorCode = ""
			m.environment = cloneEnvironment(environment)
			return nil
		}
		select {
		case processErr := <-m.done:
			m.command = nil
			return m.recordFailure(&RuntimeError{Code: CodeCaddyStartFailed, Message: "Caddy exited during startup", Cause: processErr})
		case <-healthContext.Done():
			_ = command.Process.Kill()
			m.command = nil
			return m.recordFailure(&RuntimeError{Code: CodeRuntimeUnhealthy, Message: "Caddy did not become healthy", Cause: healthContext.Err()})
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func validateWithEnvironment(ctx context.Context, validator Validator, configuration []byte, environment map[string]string) error {
	if validatorWithEnvironment, ok := validator.(interface {
		ValidateWithEnvironment(context.Context, []byte, map[string]string) error
	}); ok {
		return validatorWithEnvironment.ValidateWithEnvironment(ctx, configuration, environment)
	}
	return validator.Validate(ctx, configuration)
}

// EnvironmentMatches reports whether the running Caddy process inherited the
// requested environment. It is used to turn provider-secret changes into a
// restart instead of an ineffective Admin API reload.
func (m *RuntimeManager) EnvironmentMatches(environment map[string]string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.runningLocked() {
		return false
	}
	if len(m.environment) != len(environment) {
		return false
	}
	for key, value := range environment {
		if m.environment[key] != value {
			return false
		}
	}
	return true
}

// CurrentEnvironment returns a copy of the environment inherited by the
// running Caddy process so a failed credential restart can restore the last
// working runtime without exposing values outside the daemon process.
func (m *RuntimeManager) CurrentEnvironment() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneEnvironment(m.environment)
}

func cloneEnvironment(environment map[string]string) map[string]string {
	if len(environment) == 0 {
		return nil
	}
	result := make(map[string]string, len(environment))
	for key, value := range environment {
		result[key] = value
	}
	return result
}

func (m *RuntimeManager) Reload(ctx context.Context, configuration []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelRenewalsLocked()
	if !m.runningLocked() {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyReloadFailed, Message: "Caddy is not running"})
	}
	if err := m.validator.Validate(ctx, configuration); err != nil {
		return m.recordFailure(err)
	}
	if err := m.admin.Reload(ctx, configuration); err != nil {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyReloadFailed, Message: "Caddy rejected the reload", Cause: err})
	}
	if err := writeConfigAtomically(m.configPath, configuration); err != nil {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyReloadFailed, Message: "Caddy reloaded but the startup configuration could not be saved", Cause: err})
	}
	m.lastErrorCode = ""
	return nil
}

func (m *RuntimeManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelRenewalsLocked()
	if !m.runningLocked() {
		if m.command != nil && m.done != nil {
			select {
			case processErr := <-m.done:
				if processErr != nil {
					m.lastErrorCode = CodeCaddyStartFailed
				}
			default:
			}
		}
		m.command = nil
		m.lastErrorCode = ""
		return nil
	}
	if err := m.admin.Stop(ctx); err != nil {
		return m.recordFailure(&RuntimeError{Code: CodeCaddyStopFailed, Message: "Unable to request Caddy shutdown", Cause: err})
	}
	timer := time.NewTimer(m.stopTimeout)
	defer timer.Stop()
	select {
	case err := <-m.done:
		m.command = nil
		if err != nil {
			var exitError *exec.ExitError
			if !errors.As(err, &exitError) {
				return m.recordFailure(&RuntimeError{Code: CodeCaddyStopFailed, Message: "Caddy shutdown failed", Cause: err})
			}
		}
		m.lastErrorCode = ""
		return nil
	case <-ctx.Done():
		return m.recordFailure(&RuntimeError{Code: CodeCaddyStopFailed, Message: "Caddy shutdown was canceled", Cause: ctx.Err()})
	case <-timer.C:
		_ = m.command.Process.Kill()
		m.command = nil
		return m.recordFailure(&RuntimeError{Code: CodeCaddyStopFailed, Message: "Caddy shutdown timed out"})
	}
}

func (m *RuntimeManager) Restart(ctx context.Context, configuration []byte) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	return m.Start(ctx, configuration)
}

func (m *RuntimeManager) Status(ctx context.Context) RuntimeState {
	return m.StatusSnapshot(ctx).Caddy
}

// StatusSnapshot returns the current Caddy/WebDAV state and the last safe
// runtime error code. A failed start remains FAILED until a later successful
// operation or an explicit stop clears it.
func (m *RuntimeManager) StatusSnapshot(ctx context.Context) RuntimeSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.runningLocked() {
		m.command = nil
		state := RuntimeStopped
		if m.lastErrorCode != "" {
			state = RuntimeFailed
		}
		return RuntimeSnapshot{Caddy: state, WebDAV: state, LastErrorCode: m.lastErrorCode}
	}
	if err := m.admin.Health(ctx); err != nil {
		return RuntimeSnapshot{Caddy: RuntimeDegraded, WebDAV: RuntimeDegraded, LastErrorCode: CodeRuntimeUnhealthy}
	}
	return RuntimeSnapshot{Caddy: RuntimeRunning, WebDAV: RuntimeRunning, LastErrorCode: m.lastErrorCode}
}

func (m *RuntimeManager) recordFailure(err error) error {
	var runtimeError *RuntimeError
	code := CodeRuntimeUnhealthy
	message := "Managed Caddy runtime failed"
	if errors.As(err, &runtimeError) {
		m.lastErrorCode = runtimeError.Code
		code = runtimeError.Code
		message = runtimeError.Message
	} else {
		m.lastErrorCode = code
	}
	if m.logger != nil {
		m.logger.Error("managed Caddy operation failed", "error_code", string(code), "error", safeRuntimeLogMessage(message))
	}
	return err
}

func safeRuntimeLogMessage(value string) string {
	value = strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return -1
		}
		return character
	}, strings.TrimSpace(value))
	characters := []rune(value)
	if len(characters) > 512 {
		return string(characters[:512])
	}
	if value == "" {
		return "Managed Caddy runtime failed"
	}
	return value
}

func (m *RuntimeManager) runningLocked() bool {
	if m.command == nil || m.done == nil {
		return false
	}
	select {
	case <-m.done:
		return false
	default:
		return true
	}
}

package diagnostics

import (
	"context"
	"database/sql"
	"errors"
	"os"

	"davdeck.dev/davdeck/core/internal/app"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type DatabaseCheck struct {
	Database      *sql.DB
	SchemaVersion int
}

func (DatabaseCheck) ID() string { return "database" }
func (c DatabaseCheck) Run(ctx context.Context) Result {
	if c.Database == nil || c.Database.PingContext(ctx) != nil {
		return Result{ID: c.ID(), Title: "Database", Status: StatusFail, Code: "DATABASE_UNAVAILABLE", Message: "SQLite is unavailable"}
	}
	return Result{ID: c.ID(), Title: "Database", Status: StatusPass, Message: "SQLite is ready at schema version " + safeInteger(c.SchemaVersion)}
}

type DirectoryCheck struct {
	Name     string
	Path     string
	Required bool
}

func (c DirectoryCheck) ID() string { return "directory_" + c.Name }
func (c DirectoryCheck) Run(context.Context) Result {
	info, err := os.Stat(c.Path)
	if err != nil || !info.IsDir() {
		status := StatusWarn
		if c.Required {
			status = StatusFail
		}
		return Result{ID: c.ID(), Title: "Application directory", Status: status, Code: "DIRECTORY_UNAVAILABLE", Message: c.Name + " directory is unavailable"}
	}
	file, err := os.CreateTemp(c.Path, ".davdeck-diagnostic-*")
	if err != nil {
		return Result{ID: c.ID(), Title: "Application directory", Status: StatusWarn, Code: "DIRECTORY_NOT_WRITABLE", Message: c.Name + " directory is not writable"}
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	if closeErr != nil || removeErr != nil {
		return Result{ID: c.ID(), Title: "Application directory", Status: StatusWarn, Code: "DIRECTORY_CLEANUP_FAILED", Message: c.Name + " directory diagnostic cleanup failed"}
	}
	return Result{ID: c.ID(), Title: "Application directory", Status: StatusPass, Message: c.Name + " directory is writable"}
}

type BinaryInspector interface {
	Inspect(context.Context) (caddyruntime.BinaryInfo, error)
}

type CaddyBinaryCheck struct{ Inspector BinaryInspector }

func (CaddyBinaryCheck) ID() string { return "caddy_binary" }
func (c CaddyBinaryCheck) Run(ctx context.Context) Result {
	if c.Inspector == nil {
		return Result{ID: c.ID(), Title: "Caddy runtime", Status: StatusFail, Code: "CADDY_NOT_CONFIGURED", Message: "Caddy binary inspection is unavailable"}
	}
	info, err := c.Inspector.Inspect(ctx)
	if err != nil {
		code := "CADDY_NOT_FOUND"
		var runtimeError *caddyruntime.RuntimeError
		if errors.As(err, &runtimeError) {
			code = string(runtimeError.Code)
		}
		return Result{ID: c.ID(), Title: "Caddy runtime", Status: StatusFail, Code: code, Message: "Pinned Caddy runtime or WebDAV module is unavailable"}
	}
	return Result{ID: c.ID(), Title: "Caddy runtime", Status: StatusPass, Message: "Caddy and the WebDAV module are available (" + safeToken(info.Version) + ")"}
}

type RuntimeStatus interface {
	Status(context.Context) caddyruntime.RuntimeState
}

type CaddyRuntimeCheck struct{ Runtime RuntimeStatus }

func (CaddyRuntimeCheck) ID() string { return "caddy_runtime" }
func (c CaddyRuntimeCheck) Run(ctx context.Context) Result {
	if c.Runtime == nil {
		return Result{ID: c.ID(), Title: "Caddy process", Status: StatusWarn, Code: "RUNTIME_UNKNOWN", Message: "Caddy runtime state is unavailable"}
	}
	switch c.Runtime.Status(ctx) {
	case caddyruntime.RuntimeRunning:
		return Result{ID: c.ID(), Title: "Caddy process", Status: StatusPass, Message: "Caddy is running"}
	case caddyruntime.RuntimeStopped:
		return Result{ID: c.ID(), Title: "Caddy process", Status: StatusWarn, Code: "RUNTIME_STOPPED", Message: "Caddy is stopped"}
	default:
		return Result{ID: c.ID(), Title: "Caddy process", Status: StatusFail, Code: "RUNTIME_UNHEALTHY", Message: "Caddy runtime health is unknown"}
	}
}

type SnapshotProvider interface {
	Snapshot(context.Context) (domain.RuntimeConfigInput, error)
}
type ConfigCompiler interface {
	Compile(domain.RuntimeConfigInput) (caddyruntime.CompiledConfig, error)
}

type ConfigCheck struct {
	Snapshots   SnapshotProvider
	Compiler    ConfigCompiler
	Validator   caddyruntime.Validator
	Environment app.RuntimeEnvironmentProvider
}

func (ConfigCheck) ID() string { return "desired_configuration" }
func (c ConfigCheck) Run(ctx context.Context) Result {
	if c.Snapshots == nil || c.Compiler == nil || c.Validator == nil {
		return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: "CONFIG_CHECK_UNAVAILABLE", Message: "Desired configuration validation is unavailable"}
	}
	snapshot, err := c.Snapshots.Snapshot(ctx)
	if err != nil {
		return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: "DATABASE_ERROR", Message: "Desired configuration could not be loaded"}
	}
	compiled, err := c.Compiler.Compile(snapshot)
	if err != nil {
		return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: "TLS_CONFIGURATION_ERROR", Message: "Desired configuration could not be compiled"}
	}
	var environment map[string]string
	if c.Environment != nil {
		environment, err = c.Environment.Environment(ctx, snapshot)
		if err != nil {
			var applicationError *app.Error
			code := "DNS_PROVIDER_ERROR"
			if errors.As(err, &applicationError) {
				code = string(applicationError.Code)
			}
			return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: code, Message: "DNS provider credentials could not be prepared"}
		}
	}
	if validatorWithEnvironment, ok := c.Validator.(interface {
		ValidateWithEnvironment(context.Context, []byte, map[string]string) error
	}); ok {
		if err := validatorWithEnvironment.ValidateWithEnvironment(ctx, compiled.JSON, environment); err != nil {
			return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: "CADDY_VALIDATE_FAILED", Message: "Caddy rejected the desired configuration"}
		}
	} else if err := c.Validator.Validate(ctx, compiled.JSON); err != nil {
		return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusFail, Code: "CADDY_VALIDATE_FAILED", Message: "Caddy rejected the desired configuration"}
	}
	return Result{ID: c.ID(), Title: "Desired configuration", Status: StatusPass, Message: "Desired Caddy configuration is valid"}
}

type ShareLister interface {
	List(context.Context) ([]domain.Share, error)
}
type SharePathValidator interface{ ValidateSharePath(string) error }

type SharePathsCheck struct {
	Shares ShareLister
	Paths  SharePathValidator
}

func (SharePathsCheck) ID() string { return "share_paths" }
func (c SharePathsCheck) Run(ctx context.Context) Result {
	shares, err := c.Shares.List(ctx)
	if err != nil {
		return Result{ID: c.ID(), Title: "Share directories", Status: StatusFail, Code: "DATABASE_ERROR", Message: "Share metadata could not be loaded"}
	}
	invalid := 0
	for _, share := range shares {
		if share.Enabled && c.Paths.ValidateSharePath(share.Path) != nil {
			invalid++
		}
	}
	if invalid > 0 {
		return Result{ID: c.ID(), Title: "Share directories", Status: StatusFail, Code: "SHARE_PATH_UNAVAILABLE", Message: safeInteger(invalid) + " enabled share directories failed access checks"}
	}
	return Result{ID: c.ID(), Title: "Share directories", Status: StatusPass, Message: safeInteger(len(shares)) + " share directories passed metadata checks"}
}

type TLSChecker interface {
	Check(context.Context) (app.TLSCheckResult, error)
}

type TLSCheck struct{ TLS TLSChecker }

func (TLSCheck) ID() string { return "tls" }
func (c TLSCheck) Run(ctx context.Context) Result {
	_, err := c.TLS.Check(ctx)
	if err == nil {
		return Result{ID: c.ID(), Title: "TLS", Status: StatusPass, Message: "TLS preflight passed"}
	}
	code := "TLS_CONFIGURATION_ERROR"
	var applicationError *app.Error
	if errors.As(err, &applicationError) {
		code = string(applicationError.Code)
		if applicationError.Code == app.CodeTLSConfiguration && applicationError.Message == "TLS is not configured" {
			return Result{ID: c.ID(), Title: "TLS", Status: StatusWarn, Code: code, Message: "TLS is not configured"}
		}
	}
	return Result{ID: c.ID(), Title: "TLS", Status: StatusFail, Code: code, Message: "TLS preflight failed"}
}

func safeInteger(value int) string {
	if value < 0 {
		return "0"
	}
	const digits = "0123456789"
	if value < 10 {
		return string(digits[value])
	}
	return safeInteger(value/10) + string(digits[value%10])
}

func safeToken(value string) string {
	result := make([]rune, 0, 64)
	for _, character := range value {
		if len(result) >= 64 {
			break
		}
		if character >= 0x20 && character < 0x7f && character != '"' && character != '\\' {
			result = append(result, character)
		}
	}
	if len(result) == 0 {
		return "version unavailable"
	}
	return string(result)
}

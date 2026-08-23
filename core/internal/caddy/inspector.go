package caddy

import (
	"context"
	"os/exec"
	"strings"
)

type BinaryInfo struct {
	Version      string
	WebDAVModule bool
}

type ModuleInspector struct{ BinaryPath string }

func (i ModuleInspector) Inspect(ctx context.Context) (BinaryInfo, error) {
	if err := validateBinary(i.BinaryPath); err != nil {
		return BinaryInfo{}, err
	}
	version, err := exec.CommandContext(ctx, i.BinaryPath, "version").Output()
	if err != nil {
		return BinaryInfo{}, &RuntimeError{Code: CodeCaddyNotFound, Message: "Unable to inspect Caddy version", Cause: err}
	}
	modules, err := exec.CommandContext(ctx, i.BinaryPath, "list-modules", "--packages", "--versions").Output()
	if err != nil {
		return BinaryInfo{}, &RuntimeError{Code: CodeCaddyModuleMissing, Message: "Unable to inspect Caddy modules", Cause: err}
	}
	found := false
	for _, line := range strings.Split(string(modules), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "http.handlers.webdav" {
			found = true
			break
		}
	}
	if !found {
		return BinaryInfo{}, &RuntimeError{Code: CodeCaddyModuleMissing, Message: "Caddy WebDAV module is missing"}
	}
	return BinaryInfo{Version: strings.TrimSpace(string(version)), WebDAVModule: true}, nil
}

package caddy

import (
	"context"
	"os/exec"
	"strings"
)

type BinaryInfo struct {
	Version         string
	WebDAVModule    bool
	DiscoveryModule bool
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
	foundWebDAV := false
	foundDiscovery := false
	for _, line := range strings.Split(string(modules), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "http.handlers.webdav":
			foundWebDAV = true
		case "http.handlers.davdeck_index":
			foundDiscovery = true
		}
	}
	if !foundWebDAV {
		return BinaryInfo{}, &RuntimeError{Code: CodeCaddyModuleMissing, Message: "Caddy WebDAV module is missing"}
	}
	if !foundDiscovery {
		return BinaryInfo{}, &RuntimeError{Code: CodeCaddyModuleMissing, Message: "Caddy DavDeck discovery module is missing"}
	}
	return BinaryInfo{Version: strings.TrimSpace(string(version)), WebDAVModule: true, DiscoveryModule: true}, nil
}

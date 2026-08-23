package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceConfigValidation(t *testing.T) {
	valid := ServiceConfig{Executable: filepath.Join(t.TempDir(), "davd"), Arguments: []string{"--listen", "127.0.0.1:8090"}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Executable = "davd"
	if err := invalid.Validate(); err == nil {
		t.Fatal("relative executable was accepted")
	}
	invalid = valid
	invalid.Arguments = []string{"--data-dir", "bad\nvalue"}
	if err := invalid.Validate(); err == nil {
		t.Fatal("control character was accepted")
	}
}

func TestServiceDefinitionsEscapeArgumentsDeterministically(t *testing.T) {
	config := ServiceConfig{Executable: filepath.Join(t.TempDir(), "davd"), Arguments: []string{"--data-dir", "/var/lib/DavDeck & files", "--label=100%"}, Description: "DavDeck", User: "davdeck"}
	launchd, err := renderLaunchdDefinition(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(launchd), "/var/lib/DavDeck &amp; files") || !strings.Contains(string(launchd), "<string>davdeck</string>") {
		t.Fatalf("launchd definition was not escaped: %s", launchd)
	}
	systemd, err := renderSystemdDefinition(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(systemd), `"/var/lib/DavDeck & files"`) || !strings.Contains(string(systemd), `"--label=100%%"`) || !strings.Contains(string(systemd), "User=davdeck") {
		t.Fatalf("systemd definition was not escaped: %s", systemd)
	}
	second, err := renderSystemdDefinition(config)
	if err != nil || string(second) != string(systemd) {
		t.Fatal("systemd definition is not deterministic")
	}
}

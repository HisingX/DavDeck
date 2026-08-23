package platform

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveCaddyBinary(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "bin", "davd")
	cases := []struct {
		name     string
		override string
		exists   map[string]bool
		want     string
	}{
		{name: "explicit override", override: "/custom/caddy", want: "/custom/caddy"},
		{name: "sibling bundled binary", exists: map[string]bool{filepath.Join(filepath.Dir(executable), caddyExecutableName()): true}, want: filepath.Join(filepath.Dir(executable), caddyExecutableName())},
		{name: "release libexec binary", exists: map[string]bool{filepath.Join(filepath.Dir(executable), "..", "libexec", caddyExecutableName()): true}, want: filepath.Join(filepath.Dir(executable), "..", "libexec", caddyExecutableName())},
		{name: "path fallback", want: caddyExecutableName()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := resolveCaddyBinary(testCase.override, func() (string, error) { return executable, nil }, func(path string) bool { return testCase.exists[path] })
			if got != testCase.want {
				t.Fatalf("ResolveCaddyBinary() = %q, want %q", got, testCase.want)
			}
		})
	}
	if got := resolveCaddyBinary("", func() (string, error) { return "", errors.New("unavailable") }, func(string) bool { return false }); got != caddyExecutableName() {
		t.Fatalf("fallback = %q", got)
	}
}

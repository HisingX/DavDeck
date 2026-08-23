//go:build darwin || linux

package platform

func caddyExecutableName() string { return "caddy" }

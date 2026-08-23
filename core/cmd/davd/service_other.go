//go:build !windows

package main

func runPlatform() error {
	return run()
}

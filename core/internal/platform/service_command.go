//go:build darwin || linux

package platform

import (
	"context"
	"os/exec"
)

type serviceCommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type execServiceCommandRunner struct{}

func (execServiceCommandRunner) Run(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, arguments...).CombinedOutput()
}

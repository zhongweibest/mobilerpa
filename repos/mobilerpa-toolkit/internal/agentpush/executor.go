package agentpush

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Executor interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type CommandExecutor struct{}

func NewCommandExecutor() *CommandExecutor {
	return &CommandExecutor{}
}

func (c *CommandExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errorText := strings.TrimSpace(stderr.String())
		if errorText == "" {
			errorText = strings.TrimSpace(stdout.String())
		}
		if errorText == "" {
			errorText = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", filepath.Base(name), strings.Join(args, " "), errorText)
	}

	return stdout.String(), nil
}


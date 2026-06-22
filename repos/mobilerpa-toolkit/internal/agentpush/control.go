package agentpush

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type AgentController struct {
	executor Executor
}

func NewAgentController(executor Executor) *AgentController {
	return &AgentController{executor: executor}
}

func (c *AgentController) Start(ctx context.Context, options AgentControlOptions) error {
	device := strings.TrimSpace(options.Device)
	if device == "" {
		return fmt.Errorf("device is required")
	}

	remoteEntryPath := agentEntryPath(options.RemoteRoot)
	runFile := "file://" + remoteEntryPath

	if err := c.clearStopSignal(ctx, options); err != nil {
		return err
	}

	if _, err := c.runDeviceADB(ctx, options, "shell", "am", "start", "-n", options.AutoJSComponent, "-d", runFile); err != nil {
		return err
	}

	return nil
}

func (c *AgentController) Stop(ctx context.Context, options AgentControlOptions) error {
	device := strings.TrimSpace(options.Device)
	if device == "" {
		return fmt.Errorf("device is required")
	}

	if err := c.writeStopSignal(ctx, options); err != nil {
		return err
	}

	return nil
}

func (c *AgentController) clearStopSignal(ctx context.Context, options AgentControlOptions) error {
	if _, err := c.runDeviceADB(ctx, options, "shell", "rm", "-f", stopSignalPath(options.RemoteRoot)); err != nil {
		return err
	}
	return nil
}

func (c *AgentController) writeStopSignal(ctx context.Context, options AgentControlOptions) error {
	if _, err := c.runDeviceADB(ctx, options, "shell", "mkdir", "-p", runtimeDirPath(options.RemoteRoot)); err != nil {
		return err
	}

	stopFilePath, err := writeStopSignalFile()
	if err != nil {
		return err
	}
	defer os.Remove(stopFilePath)

	if _, err := c.runDeviceADB(ctx, options, "push", stopFilePath, stopSignalPath(options.RemoteRoot)); err != nil {
		return err
	}
	return nil
}

func (c *AgentController) runDeviceADB(ctx context.Context, options AgentControlOptions, args ...string) (string, error) {
	commandArgs := append([]string{"-s", options.Device}, args...)
	return c.executor.Run(ctx, options.ADBPath, commandArgs...)
}

func agentRootPath(remoteRoot string) string {
	return strings.TrimRight(strings.TrimSpace(remoteRoot), "/") + "/agent"
}

func runtimeDirPath(remoteRoot string) string {
	return agentRootPath(remoteRoot) + "/runtime"
}

func agentEntryPath(remoteRoot string) string {
	return agentRootPath(remoteRoot) + "/agent.js"
}

func stopSignalPath(remoteRoot string) string {
	return runtimeDirPath(remoteRoot) + "/stop.signal"
}

func writeStopSignalFile() (string, error) {
	file, err := os.CreateTemp("", "mobilerpa-stop-signal-*.txt")
	if err != nil {
		return "", fmt.Errorf("create stop signal temp file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString("stop\n"); err != nil {
		return "", fmt.Errorf("write stop signal temp file: %w", err)
	}

	return file.Name(), nil
}

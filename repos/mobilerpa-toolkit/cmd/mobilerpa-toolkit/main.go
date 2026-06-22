package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mobilerpa/mobilerpa-toolkit/internal/agentpush"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return agentpush.ErrUsage
	}

	ctx := context.Background()
	executor := agentpush.NewCommandExecutor()

	switch os.Args[1] {
	case "push-center":
		command, err := agentpush.ParseCenterCommand(os.Args[2:])
		if err != nil {
			return err
		}
		return agentpush.NewService(executor).Push(ctx, command.Options)
	case "push-manual":
		command, err := agentpush.ParseManualCommand(os.Args[2:])
		if err != nil {
			return err
		}
		return agentpush.NewManualRunner(executor).Run(ctx, command)
	case "start-agent":
		command, err := agentpush.ParseStartAgentCommand(os.Args[2:])
		if err != nil {
			return err
		}
		return agentpush.NewAgentController(executor).Start(ctx, command.Options)
	case "stop-agent":
		command, err := agentpush.ParseStopAgentCommand(os.Args[2:])
		if err != nil {
			return err
		}
		return agentpush.NewAgentController(executor).Stop(ctx, command.Options)
	case "-h", "--help", "help":
		return agentpush.ErrUsage
	default:
		return fmt.Errorf("unknown subcommand: %s\n\n%s", os.Args[1], agentpush.UsageText())
	}
}

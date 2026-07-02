package agentpush

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultADBPath         = "adb"
	DefaultRemoteRoot      = "/sdcard/脚本"
	DefaultAutoJSComponent = "org.autojs.autojs6/org.autojs.autojs.external.open.RunIntentActivity"
	DefaultAutoJSPackage   = "org.autojs.autojs6"
)

type PushOptions struct {
	ADBPath         string
	AgentRoot       string
	Device          string
	CenterBaseURL   string
	DeviceLinkSN    string
	RemoteRoot      string
	AutoJSComponent string
	ResetConfig     bool
	RunAgent        bool
}

type AgentControlOptions struct {
	ADBPath         string
	Device          string
	RemoteRoot      string
	AutoJSComponent string
	AutoJSPackage   string
}

type CenterCommand struct {
	Options PushOptions
}

type ManualCommand struct {
	Options PushOptions
	All     bool
}

type StartAgentCommand struct {
	Options AgentControlOptions
}

type StopAgentCommand struct {
	Options AgentControlOptions
}

func ParseCenterCommand(args []string) (CenterCommand, error) {
	fs := flag.NewFlagSet("push-center", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	options := defaultPushOptions()
	device := fs.String("device", "", "ADB device serial or endpoint")
	centerBaseURL := fs.String("center-base-url", "", "Center base URL")
	deviceLinkSN := fs.String("device-link-sn", "", "Device link SN for runtime config")
	agentRoot := fs.String("agent-root", "", "Agent root path")
	adbPath := fs.String("adb-path", DefaultADBPath, "ADB executable path")
	resetConfig := fs.Bool("reset-config", false, "Reset config.json on device")
	noRun := fs.Bool("no-run", false, "Do not auto-run agent after push")

	if err := fs.Parse(args); err != nil {
		return CenterCommand{}, fmt.Errorf("%w", err)
	}

	options.Device = strings.TrimSpace(*device)
	options.CenterBaseURL = strings.TrimSpace(*centerBaseURL)
	options.DeviceLinkSN = strings.TrimSpace(*deviceLinkSN)
	options.AgentRoot = strings.TrimSpace(*agentRoot)
	options.ADBPath = strings.TrimSpace(*adbPath)
	options.ResetConfig = *resetConfig
	options.RunAgent = !*noRun

	if options.Device == "" || options.CenterBaseURL == "" {
		return CenterCommand{}, fmt.Errorf("push-center requires --device and --center-base-url\n\n%s", UsageText())
	}

	resolvedAgentRoot, err := resolveAgentRoot(options.AgentRoot)
	if err != nil {
		return CenterCommand{}, err
	}
	options.AgentRoot = resolvedAgentRoot

	return CenterCommand{Options: options}, nil
}

func ParseManualCommand(args []string) (ManualCommand, error) {
	fs := flag.NewFlagSet("push-manual", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	options := defaultPushOptions()
	device := fs.String("device", "", "ADB device serial or endpoint")
	all := fs.Bool("all", false, "Push to all authorized devices")
	centerBaseURL := fs.String("center-base-url", "", "Center base URL")
	agentRoot := fs.String("agent-root", "", "Agent root path")
	adbPath := fs.String("adb-path", DefaultADBPath, "ADB executable path")
	resetConfig := fs.Bool("reset-config", false, "Reset config.json on device")
	noRun := fs.Bool("no-run", false, "Do not auto-run agent after push")

	if err := fs.Parse(args); err != nil {
		return ManualCommand{}, fmt.Errorf("%w", err)
	}

	options.Device = strings.TrimSpace(*device)
	options.CenterBaseURL = strings.TrimSpace(*centerBaseURL)
	options.AgentRoot = strings.TrimSpace(*agentRoot)
	options.ADBPath = strings.TrimSpace(*adbPath)
	options.ResetConfig = *resetConfig
	options.RunAgent = !*noRun

	if options.CenterBaseURL == "" {
		return ManualCommand{}, fmt.Errorf("push-manual requires --center-base-url\n\n%s", UsageText())
	}

	resolvedAgentRoot, err := resolveAgentRoot(options.AgentRoot)
	if err != nil {
		return ManualCommand{}, err
	}
	options.AgentRoot = resolvedAgentRoot

	return ManualCommand{
		Options: options,
		All:     *all,
	}, nil
}

func ParseStartAgentCommand(args []string) (StartAgentCommand, error) {
	fs := flag.NewFlagSet("start-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	options := defaultAgentControlOptions()
	device := fs.String("device", "", "ADB device serial or endpoint")
	adbPath := fs.String("adb-path", DefaultADBPath, "ADB executable path")
	remoteRoot := fs.String("remote-root", DefaultRemoteRoot, "Remote agent root parent path")
	autoJSComponent := fs.String("autojs-component", DefaultAutoJSComponent, "AutoJs6 activity component")

	if err := fs.Parse(args); err != nil {
		return StartAgentCommand{}, fmt.Errorf("%w", err)
	}

	options.Device = strings.TrimSpace(*device)
	options.ADBPath = strings.TrimSpace(*adbPath)
	options.RemoteRoot = strings.TrimSpace(*remoteRoot)
	options.AutoJSComponent = strings.TrimSpace(*autoJSComponent)

	if options.Device == "" {
		return StartAgentCommand{}, fmt.Errorf("start-agent requires --device\n\n%s", UsageText())
	}

	return StartAgentCommand{Options: options}, nil
}

func ParseStopAgentCommand(args []string) (StopAgentCommand, error) {
	fs := flag.NewFlagSet("stop-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	options := defaultAgentControlOptions()
	device := fs.String("device", "", "ADB device serial or endpoint")
	adbPath := fs.String("adb-path", DefaultADBPath, "ADB executable path")
	remoteRoot := fs.String("remote-root", DefaultRemoteRoot, "Remote agent root parent path")
	autoJSPackage := fs.String("autojs-package", DefaultAutoJSPackage, "AutoJs6 package name")

	if err := fs.Parse(args); err != nil {
		return StopAgentCommand{}, fmt.Errorf("%w", err)
	}

	options.Device = strings.TrimSpace(*device)
	options.ADBPath = strings.TrimSpace(*adbPath)
	options.RemoteRoot = strings.TrimSpace(*remoteRoot)
	options.AutoJSPackage = strings.TrimSpace(*autoJSPackage)

	if options.Device == "" {
		return StopAgentCommand{}, fmt.Errorf("stop-agent requires --device\n\n%s", UsageText())
	}

	return StopAgentCommand{Options: options}, nil
}

func defaultPushOptions() PushOptions {
	return PushOptions{
		ADBPath:         DefaultADBPath,
		RemoteRoot:      DefaultRemoteRoot,
		AutoJSComponent: DefaultAutoJSComponent,
		RunAgent:        true,
	}
}

func defaultAgentControlOptions() AgentControlOptions {
	return AgentControlOptions{
		ADBPath:         DefaultADBPath,
		RemoteRoot:      DefaultRemoteRoot,
		AutoJSComponent: DefaultAutoJSComponent,
		AutoJSPackage:   DefaultAutoJSPackage,
	}
}

func resolveAgentRoot(input string) (string, error) {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(input) != "" {
		candidates = append(candidates, input)
	}
	if envValue := strings.TrimSpace(os.Getenv("MOBILE_RPA_AGENT_ROOT")); envValue != "" {
		candidates = append(candidates, envValue)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "..", "mobilerpa-agent", "agent"))
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "..", "..", "mobilerpa-agent", "agent"))
	}

	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if stat, err := os.Stat(cleaned); err == nil && stat.IsDir() {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("agent root not found; pass --agent-root or set MOBILE_RPA_AGENT_ROOT")
}

package agentpush

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ManualRunner struct {
	executor Executor
}

func NewManualRunner(executor Executor) *ManualRunner {
	return &ManualRunner{executor: executor}
}

func (r *ManualRunner) Run(ctx context.Context, command ManualCommand) error {
	options := command.Options
	devices, err := r.listAuthorizedDevices(ctx, options.ADBPath)
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		return fmt.Errorf("no authorized devices found in adb devices")
	}

	state, _ := loadManualState()

	if command.All {
		for _, device := range devices {
			options.Device = device
			if err := NewService(r.executor).Push(ctx, options); err != nil {
				return fmt.Errorf("push to %s failed: %w", device, err)
			}
		}
		return nil
	}

	if strings.TrimSpace(options.Device) == "" {
		selectedDevice, err := chooseDeviceInteractively(devices, state.LastDevice)
		if err != nil {
			return err
		}
		options.Device = selectedDevice
	}

	nextState := manualState{
		LastDevice:        options.Device,
		LastCenterBaseURL: options.CenterBaseURL,
	}
	_ = saveManualState(nextState)

	return NewService(r.executor).Push(ctx, options)
}

func (r *ManualRunner) listAuthorizedDevices(ctx context.Context, adbPath string) ([]string, error) {
	output, err := r.executor.Run(ctx, adbPath, "devices")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	results := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "device" {
			results = append(results, fields[0])
		}
	}
	return results, nil
}

func chooseDeviceInteractively(devices []string, lastDevice string) (string, error) {
	fmt.Println("Available devices:")
	defaultIndex := 0
	for index, device := range devices {
		fmt.Printf("%d. %s\n", index+1, device)
		if device == lastDevice {
			defaultIndex = index
		}
	}

	defaultDevice := devices[defaultIndex]
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Choose device number or serial, default %s: ", defaultDevice)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read device selection: %w", err)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultDevice, nil
	}

	for index, device := range devices {
		if text == fmt.Sprintf("%d", index+1) || strings.EqualFold(text, device) {
			return device, nil
		}
	}

	return "", fmt.Errorf("invalid device selection: %s", text)
}

type manualState struct {
	LastDevice        string `json:"last_device"`
	LastCenterBaseURL string `json:"last_center_base_url"`
}

func loadManualState() (manualState, error) {
	path, err := manualStatePath()
	if err != nil {
		return manualState{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return manualState{}, err
	}

	var state manualState
	if err := json.Unmarshal(content, &state); err != nil {
		return manualState{}, err
	}
	return state, nil
}

func saveManualState(state manualState) error {
	path, err := manualStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func manualStatePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "MobileRPA", "push-agent.local.json"), nil
}


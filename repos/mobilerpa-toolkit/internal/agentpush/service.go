package agentpush

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	executor Executor
}

func NewService(executor Executor) *Service {
	return &Service{executor: executor}
}

func (s *Service) Push(ctx context.Context, options PushOptions) error {
	paths, err := resolveAgentPaths(options.AgentRoot)
	if err != nil {
		return err
	}

	device := strings.TrimSpace(options.Device)
	if device == "" {
		return fmt.Errorf("device is required")
	}

	if _, err := s.executor.Run(ctx, options.ADBPath, "connect", device); err != nil {
		// adb connect best-effort; many already connected devices still work even if connect is unnecessary.
	}

	remoteAgentDir := options.RemoteRoot + "/agent"
	remoteRuntimeDir := remoteAgentDir + "/runtime"
	remoteConfigPath := remoteRuntimeDir + "/config.json"
	remoteBootstrapPath := remoteRuntimeDir + "/bootstrap.json"
	remoteEntryPath := remoteAgentDir + "/agent.js"
	remoteScriptsDir := remoteAgentDir + "/scripts"

	if _, err := s.runDeviceADB(ctx, options, "shell", "mkdir", "-p", remoteAgentDir, remoteRuntimeDir, remoteScriptsDir); err != nil {
		return err
	}
	if _, err := s.runDeviceADB(ctx, options, "push", paths.AgentEntry, remoteEntryPath); err != nil {
		return err
	}

	for _, file := range paths.AgentFiles {
		remotePath := remoteAgentDir + "/" + filepath.ToSlash(file.RelativePath)
		remoteDir := filepath.ToSlash(filepath.Dir(remotePath))
		if remoteDir != "." && remoteDir != "" {
			if _, err := s.runDeviceADB(ctx, options, "shell", "mkdir", "-p", remoteDir); err != nil {
				return err
			}
		}
		if _, err := s.runDeviceADB(ctx, options, "push", file.LocalPath, remotePath); err != nil {
			return err
		}
	}

	needConfig := options.ResetConfig
	if !needConfig {
		if _, err := s.runDeviceADB(ctx, options, "shell", "ls", remoteConfigPath); err != nil {
			needConfig = true
		}
	}

	if options.ResetConfig {
		if _, err := s.runDeviceADB(ctx, options, "shell", "rm", "-f", remoteConfigPath); err != nil {
			return err
		}
	}

	bootstrapPath, err := writeBootstrapFile(options.CenterBaseURL, options.DeviceLinkSN)
	if err != nil {
		return err
	}
	defer os.Remove(bootstrapPath)

	if _, err := s.runDeviceADB(ctx, options, "push", bootstrapPath, remoteBootstrapPath); err != nil {
		return err
	}

	if options.RunAgent {
		controller := NewAgentController(s.executor)
		if err := controller.Start(ctx, AgentControlOptions{
			ADBPath:         options.ADBPath,
			Device:          options.Device,
			RemoteRoot:      options.RemoteRoot,
			AutoJSComponent: options.AutoJSComponent,
			AutoJSPackage:   DefaultAutoJSPackage,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) runDeviceADB(ctx context.Context, options PushOptions, args ...string) (string, error) {
	commandArgs := append([]string{"-s", options.Device}, args...)
	return s.executor.Run(ctx, options.ADBPath, commandArgs...)
}

type agentPaths struct {
	AgentEntry string
	AgentFiles []agentFile
}

type agentFile struct {
	LocalPath    string
	RelativePath string
}

func resolveAgentPaths(agentRoot string) (agentPaths, error) {
	root := filepath.Clean(agentRoot)
	entry := filepath.Join(root, "agent.js")

	if stat, err := os.Stat(entry); err != nil || stat.IsDir() {
		return agentPaths{}, fmt.Errorf("agent entry not found: %s", entry)
	}

	files := make([]agentFile, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relativePath = filepath.Clean(relativePath)

		if info.IsDir() {
			if relativePath == "runtime" {
				return filepath.SkipDir
			}
			return nil
		}

		if relativePath == "agent.js" {
			return nil
		}

		files = append(files, agentFile{
			LocalPath:    path,
			RelativePath: relativePath,
		})
		return nil
	})
	if err != nil {
		return agentPaths{}, fmt.Errorf("walk agent root: %w", err)
	}

	return agentPaths{
		AgentEntry: entry,
		AgentFiles: files,
	}, nil
}

func writeBootstrapFile(centerBaseURL string, deviceLinkSN string) (string, error) {
	type websocketConfig struct {
		Enabled             bool `json:"enabled"`
		HeartbeatIntervalMS int  `json:"heartbeat_interval_ms"`
	}
	type bootstrapConfig struct {
		CenterBaseURL string          `json:"center_base_url"`
		DeviceLinkSN  string          `json:"device_link_sn"`
		WebSocket     websocketConfig `json:"websocket"`
	}

	payload := bootstrapConfig{
		CenterBaseURL: centerBaseURL,
		DeviceLinkSN:  strings.TrimSpace(deviceLinkSN),
		WebSocket: websocketConfig{
			Enabled:             true,
			HeartbeatIntervalMS: 30000,
		},
	}

	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal bootstrap json: %w", err)
	}

	file, err := os.CreateTemp("", "mobilerpa-bootstrap-*.json")
	if err != nil {
		return "", fmt.Errorf("create bootstrap temp file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(content); err != nil {
		return "", fmt.Errorf("write bootstrap temp file: %w", err)
	}

	return file.Name(), nil
}

package agentpush

import (
	"context"
	"os"
	"strings"
	"testing"
)

type fakeExecutor struct {
	calls []string
}

func (f *fakeExecutor) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, call)

	if strings.Contains(call, " shell ls ") {
		return "", os.ErrNotExist
	}
	return "", nil
}

type fakeExecutorExistingConfig struct {
	calls []string
}

func (f *fakeExecutorExistingConfig) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, call)
	return "", nil
}

type fakeControllerExecutor struct {
	calls []string
}

func (f *fakeControllerExecutor) Run(_ context.Context, name string, args ...string) (string, error) {
	call := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.calls = append(f.calls, call)
	return "", nil
}

func TestWriteBootstrapFile(t *testing.T) {
	t.Parallel()

	path, err := writeBootstrapFile("http://127.0.0.1:18080")
	if err != nil {
		t.Fatalf("writeBootstrapFile returned error: %v", err)
	}
	defer os.Remove(path)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bootstrap file: %v", err)
	}

	text := string(content)
	if !strings.Contains(text, `"center_base_url": "http://127.0.0.1:18080"`) {
		t.Fatalf("bootstrap content missing center_base_url: %s", text)
	}
	if !strings.Contains(text, `"heartbeat_interval_ms": 30000`) {
		t.Fatalf("bootstrap content missing heartbeat interval: %s", text)
	}
}

func TestPushCreatesBootstrapWhenConfigMissing(t *testing.T) {
	t.Parallel()

	agentRoot := createTestAgentRoot(t)
	executor := &fakeExecutor{}
	service := NewService(executor)

	err := service.Push(context.Background(), PushOptions{
		ADBPath:         "adb",
		AgentRoot:       agentRoot,
		Device:          "device-001",
		CenterBaseURL:   "http://127.0.0.1:18080",
		RemoteRoot:      DefaultRemoteRoot,
		AutoJSComponent: DefaultAutoJSComponent,
		RunAgent:        true,
	})
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	joined := strings.Join(executor.calls, "\n")
	if !strings.Contains(joined, "/runtime/bootstrap.json") {
		t.Fatalf("expected bootstrap push call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "shell mkdir -p "+DefaultRemoteRoot+"/agent "+DefaultRemoteRoot+"/agent/runtime "+DefaultRemoteRoot+"/agent/scripts") {
		t.Fatalf("expected remote scripts dir creation call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "shell rm -f "+stopSignalPath(DefaultRemoteRoot)) {
		t.Fatalf("expected stop signal cleanup call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "shell am start") {
		t.Fatalf("expected agent start call, got:\n%s", joined)
	}
}

func TestPushKeepsExistingConfigAndNoRun(t *testing.T) {
	t.Parallel()

	agentRoot := createTestAgentRoot(t)
	executor := &fakeExecutorExistingConfig{}
	service := NewService(executor)

	err := service.Push(context.Background(), PushOptions{
		ADBPath:         "adb",
		AgentRoot:       agentRoot,
		Device:          "device-001",
		CenterBaseURL:   "http://127.0.0.1:18080",
		RemoteRoot:      DefaultRemoteRoot,
		AutoJSComponent: DefaultAutoJSComponent,
		RunAgent:        false,
	})
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}

	joined := strings.Join(executor.calls, "\n")
	if !strings.Contains(joined, "/runtime/bootstrap.json") {
		t.Fatalf("expected bootstrap push call for center address refresh, got:\n%s", joined)
	}
	if !strings.Contains(joined, "shell mkdir -p "+DefaultRemoteRoot+"/agent "+DefaultRemoteRoot+"/agent/runtime "+DefaultRemoteRoot+"/agent/scripts") {
		t.Fatalf("expected remote scripts dir creation call, got:\n%s", joined)
	}
	if strings.Contains(joined, "shell am start") {
		t.Fatalf("did not expect agent start call, got:\n%s", joined)
	}
}

func TestStartAgent(t *testing.T) {
	t.Parallel()

	executor := &fakeControllerExecutor{}
	controller := NewAgentController(executor)

	err := controller.Start(context.Background(), AgentControlOptions{
		ADBPath:         "adb",
		Device:          "device-001",
		RemoteRoot:      DefaultRemoteRoot,
		AutoJSComponent: DefaultAutoJSComponent,
	})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	joined := strings.Join(executor.calls, "\n")
	if !strings.Contains(joined, "shell rm -f "+stopSignalPath(DefaultRemoteRoot)) {
		t.Fatalf("expected stop signal cleanup call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "shell am start") {
		t.Fatalf("expected am start call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "file://"+agentEntryPath(DefaultRemoteRoot)) {
		t.Fatalf("expected agent entry path, got:\n%s", joined)
	}
}

func TestStopAgent(t *testing.T) {
	t.Parallel()

	executor := &fakeControllerExecutor{}
	controller := NewAgentController(executor)

	err := controller.Stop(context.Background(), AgentControlOptions{
		ADBPath:    "adb",
		Device:     "device-001",
		RemoteRoot: DefaultRemoteRoot,
	})
	if err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	joined := strings.Join(executor.calls, "\n")
	if !strings.Contains(joined, "shell mkdir -p "+runtimeDirPath(DefaultRemoteRoot)) {
		t.Fatalf("expected runtime dir creation call, got:\n%s", joined)
	}
	if !strings.Contains(joined, "push ") || !strings.Contains(joined, " "+stopSignalPath(DefaultRemoteRoot)) {
		t.Fatalf("expected stop signal push call, got:\n%s", joined)
	}
}

func createTestAgentRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	libDir := root + string(os.PathSeparator) + "lib"
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatalf("mkdir lib dir: %v", err)
	}
	if err := os.WriteFile(root+string(os.PathSeparator)+"agent.js", []byte("console.log('ok');"), 0o644); err != nil {
		t.Fatalf("write agent.js: %v", err)
	}
	if err := os.WriteFile(libDir+string(os.PathSeparator)+"runtime.js", []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatalf("write lib file: %v", err)
	}
	if err := os.WriteFile(libDir+string(os.PathSeparator)+"task_runner.js", []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatalf("write task_runner file: %v", err)
	}
	return root
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadUsesEnvFileDefaults(t *testing.T) {
	t.Setenv("CENTER_HTTP_ADDR", "")
	t.Setenv("CENTER_DB_PATH", "")
	t.Setenv("CENTER_CORS_ALLOWED_ORIGINS", "")
	t.Setenv("CENTER_HEARTBEAT_INTERVAL", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_TIMEOUT", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_SCAN_INTERVAL", "")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `
# 中心服务本地默认配置
CENTER_HTTP_ADDR=127.0.0.1:18080
CENTER_DB_PATH="D:\dev\code\mobilerpa\.tmp\mobilerpa-center-manual.db"
IGNORED_KEY=ignored
`
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	cfg := Load()
	if cfg.HTTPAddr != "127.0.0.1:18080" {
		t.Fatalf("unexpected http addr: %s", cfg.HTTPAddr)
	}
	if cfg.DBPath != `D:\dev\code\mobilerpa\.tmp\mobilerpa-center-manual.db` {
		t.Fatalf("unexpected db path: %s", cfg.DBPath)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("unexpected cors origins count: %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.HeartbeatInterval != defaultHeartbeatInterval {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.DeviceOfflineTimeout != defaultOfflineTimeout {
		t.Fatalf("unexpected offline timeout: %s", cfg.DeviceOfflineTimeout)
	}
	if cfg.DeviceOfflineScanInterval != defaultOfflineScanInterval {
		t.Fatalf("unexpected scan interval: %s", cfg.DeviceOfflineScanInterval)
	}
}

func TestLoadProcessEnvOverridesEnvFile(t *testing.T) {
	t.Setenv("CENTER_HTTP_ADDR", "127.0.0.1:19090")
	t.Setenv("CENTER_DB_PATH", `D:\override\center.db`)
	t.Setenv("CENTER_CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173")
	t.Setenv("CENTER_HEARTBEAT_INTERVAL", "45s")
	t.Setenv("CENTER_DEVICE_OFFLINE_TIMEOUT", "2m")
	t.Setenv("CENTER_DEVICE_OFFLINE_SCAN_INTERVAL", "20s")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `
CENTER_HTTP_ADDR=127.0.0.1:18080
CENTER_DB_PATH=D:\env\center.db
CENTER_CORS_ALLOWED_ORIGINS=http://localhost:3000
CENTER_HEARTBEAT_INTERVAL=30s
CENTER_DEVICE_OFFLINE_TIMEOUT=90s
CENTER_DEVICE_OFFLINE_SCAN_INTERVAL=15s
`
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	cfg := Load()
	if cfg.HTTPAddr != "127.0.0.1:19090" {
		t.Fatalf("unexpected http addr: %s", cfg.HTTPAddr)
	}
	if cfg.DBPath != `D:\override\center.db` {
		t.Fatalf("unexpected db path: %s", cfg.DBPath)
	}
	if len(cfg.CORSAllowedOrigins) != 3 {
		t.Fatalf("unexpected cors origins count: %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.CORSAllowedOrigins[2] != "http://localhost:4173" {
		t.Fatalf("unexpected cors origin: %s", cfg.CORSAllowedOrigins[2])
	}
	if cfg.HeartbeatInterval != 45*time.Second {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.DeviceOfflineTimeout != 2*time.Minute {
		t.Fatalf("unexpected offline timeout: %s", cfg.DeviceOfflineTimeout)
	}
	if cfg.DeviceOfflineScanInterval != 20*time.Second {
		t.Fatalf("unexpected scan interval: %s", cfg.DeviceOfflineScanInterval)
	}
}

func TestLoadFindsRepoRootEnvFileFromServerDir(t *testing.T) {
	t.Setenv("CENTER_HTTP_ADDR", "")
	t.Setenv("CENTER_DB_PATH", "")
	t.Setenv("CENTER_CORS_ALLOWED_ORIGINS", "")
	t.Setenv("CENTER_HEARTBEAT_INTERVAL", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_TIMEOUT", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_SCAN_INTERVAL", "")

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("create server dir: %v", err)
	}

	envPath := filepath.Join(tmpDir, ".env")
	envContent := `
CENTER_HTTP_ADDR=127.0.0.1:18082
CENTER_DB_PATH=D:\repo-root\center.db
CENTER_CORS_ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
CENTER_HEARTBEAT_INTERVAL=35s
CENTER_DEVICE_OFFLINE_TIMEOUT=95s
CENTER_DEVICE_OFFLINE_SCAN_INTERVAL=18s
`
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(serverDir); err != nil {
		t.Fatalf("chdir server dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	cfg := Load()
	if cfg.HTTPAddr != "127.0.0.1:18082" {
		t.Fatalf("unexpected http addr: %s", cfg.HTTPAddr)
	}
	if cfg.DBPath != `D:\repo-root\center.db` {
		t.Fatalf("unexpected db path: %s", cfg.DBPath)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("unexpected cors origins count: %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.HeartbeatInterval != 35*time.Second {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.DeviceOfflineTimeout != 95*time.Second {
		t.Fatalf("unexpected offline timeout: %s", cfg.DeviceOfflineTimeout)
	}
	if cfg.DeviceOfflineScanInterval != 18*time.Second {
		t.Fatalf("unexpected scan interval: %s", cfg.DeviceOfflineScanInterval)
	}
}

func TestLoadIgnoresEmptyEnvFileValues(t *testing.T) {
	t.Setenv("CENTER_HTTP_ADDR", "")
	t.Setenv("CENTER_DB_PATH", "")
	t.Setenv("CENTER_CORS_ALLOWED_ORIGINS", "")
	t.Setenv("CENTER_HEARTBEAT_INTERVAL", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_TIMEOUT", "")
	t.Setenv("CENTER_DEVICE_OFFLINE_SCAN_INTERVAL", "")

	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	envContent := `
CENTER_HTTP_ADDR=
CENTER_DB_PATH=
CENTER_CORS_ALLOWED_ORIGINS=
CENTER_HEARTBEAT_INTERVAL=
CENTER_DEVICE_OFFLINE_TIMEOUT=
CENTER_DEVICE_OFFLINE_SCAN_INTERVAL=
`
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir tmp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})

	cfg := Load()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("unexpected http addr: %s", cfg.HTTPAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("unexpected db path: %s", cfg.DBPath)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("unexpected cors origins count: %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.HeartbeatInterval != defaultHeartbeatInterval {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.DeviceOfflineTimeout != defaultOfflineTimeout {
		t.Fatalf("unexpected offline timeout: %s", cfg.DeviceOfflineTimeout)
	}
	if cfg.DeviceOfflineScanInterval != defaultOfflineScanInterval {
		t.Fatalf("unexpected scan interval: %s", cfg.DeviceOfflineScanInterval)
	}
}

func TestParseEnvLine(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantKey   string
		wantValue string
		wantOK    bool
	}{
		{name: "空行", line: "   ", wantOK: false},
		{name: "注释", line: "# comment", wantOK: false},
		{name: "普通键值", line: "CENTER_HTTP_ADDR=127.0.0.1:18080", wantKey: "CENTER_HTTP_ADDR", wantValue: "127.0.0.1:18080", wantOK: true},
		{name: "带 export", line: "export CENTER_DB_PATH=./data/dev.db", wantKey: "CENTER_DB_PATH", wantValue: "./data/dev.db", wantOK: true},
		{name: "双引号", line: `CENTER_DB_PATH="D:\dev\center.db"`, wantKey: "CENTER_DB_PATH", wantValue: `D:\dev\center.db`, wantOK: true},
		{name: "行尾注释", line: "CENTER_HTTP_ADDR=127.0.0.1:18080 # local", wantKey: "CENTER_HTTP_ADDR", wantValue: "127.0.0.1:18080", wantOK: true},
		{name: "引号内井号", line: `CENTER_DB_PATH="D:\tmp#1\center.db"`, wantKey: "CENTER_DB_PATH", wantValue: `D:\tmp#1\center.db`, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, ok := parseEnvLine(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("unexpected ok: %v", ok)
			}
			if key != tt.wantKey {
				t.Fatalf("unexpected key: %s", key)
			}
			if value != tt.wantValue {
				t.Fatalf("unexpected value: %s", value)
			}
		})
	}
}

func TestLoadDocsAuthConfig(t *testing.T) {
	t.Setenv("CENTER_DOCS_AUTH_ENABLED", "false")
	t.Setenv("CENTER_DOCS_AUTH_USERNAME", "docs_user")
	t.Setenv("CENTER_DOCS_AUTH_PASSWORD", "docs_pass")

	cfg := Load()
	if cfg.DocsAuthEnabled {
		t.Fatalf("expected docs auth disabled")
	}
	if cfg.DocsAuthUsername != "docs_user" {
		t.Fatalf("unexpected docs auth username: %s", cfg.DocsAuthUsername)
	}
	if cfg.DocsAuthPassword != "docs_pass" {
		t.Fatalf("unexpected docs auth password: %s", cfg.DocsAuthPassword)
	}
}

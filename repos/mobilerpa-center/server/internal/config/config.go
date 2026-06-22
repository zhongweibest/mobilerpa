package config

import (
	"bufio"
	"os"
	"strings"
	"time"
)

const (
	defaultHTTPAddr            = ":8080"
	defaultDBPath              = "./data/mobilerpa.db"
	defaultHeartbeatInterval   = 30 * time.Second
	defaultOfflineTimeout      = 90 * time.Second
	defaultOfflineScanInterval = 15 * time.Second
	defaultCORSAllowedOrigins  = "http://localhost:5173,http://127.0.0.1:5173"
	defaultADBPath             = "adb"
	defaultAgentRootPath       = "../../mobilerpa-agent/agent"
	defaultScriptRootPath      = "./data/scripts"
	defaultCenterBaseURL       = "http://127.0.0.1:8080"
	defaultToolkitPath          = ""
	defaultWorkflowScanInterval = 15 * time.Second
	defaultPlanScanInterval     = 1 * time.Second
	defaultPlanStartWorkers     = 2
	defaultPlanStartFanout      = 20
)

// Config 定义中心服务运行所需配置。
type Config struct {
	// HTTPAddr 是 HTTP 服务监听地址。
	HTTPAddr string
	// DBPath 是 SQLite 数据库文件路径。
	DBPath string
	// CORSAllowedOrigins 是允许访问中心服务 HTTP 接口的浏览器源列表。
	CORSAllowedOrigins []string
	// HeartbeatInterval 是设备预期心跳间隔。
	HeartbeatInterval time.Duration
	// DeviceOfflineTimeout 是设备离线超时阈值。
	DeviceOfflineTimeout time.Duration
	// DeviceOfflineScanInterval 是离线扫描周期。
	DeviceOfflineScanInterval time.Duration
	// ADBPath 是中心服务调用本机 adb 时使用的命令或绝对路径。
	ADBPath string
	// AgentRootPath 是 toolkit 查找 agent.js 和 lib 目录时使用的路径。
	AgentRootPath string
	// ScriptRootPath 是中心服务本地可供下载的脚本目录根目录。
	ScriptRootPath string
	// CenterBaseURL 是写入手机端 bootstrap 的默认中心地址。
	CenterBaseURL string
	// ToolkitPath 是网页下发 Agent 时使用的 toolkit 可执行文件路径。
	ToolkitPath string
	// WorkflowScanInterval 是工作流时间规则后台扫描周期。
	WorkflowScanInterval time.Duration
	PlanScanInterval     time.Duration
	PlanStartWorkers     int
	PlanStartFanout      int
}

// Load 按“内置默认值 -> .env -> 进程环境变量”的优先级加载配置。
func Load() Config {
	values := map[string]string{
		"CENTER_HTTP_ADDR":                    defaultHTTPAddr,
		"CENTER_DB_PATH":                      defaultDBPath,
		"CENTER_CORS_ALLOWED_ORIGINS":         defaultCORSAllowedOrigins,
		"CENTER_HEARTBEAT_INTERVAL":           defaultHeartbeatInterval.String(),
		"CENTER_DEVICE_OFFLINE_TIMEOUT":       defaultOfflineTimeout.String(),
		"CENTER_DEVICE_OFFLINE_SCAN_INTERVAL": defaultOfflineScanInterval.String(),
		"CENTER_ADB_PATH":                     defaultADBPath,
		"CENTER_AGENT_ROOT_PATH":              defaultAgentRootPath,
		"CENTER_SCRIPT_ROOT_PATH":             defaultScriptRootPath,
		"CENTER_BASE_URL":                     defaultCenterBaseURL,
		"CENTER_TOOLKIT_PATH":                 defaultToolkitPath,
		"CENTER_WORKFLOW_SCAN_INTERVAL":       defaultWorkflowScanInterval.String(),
		"CENTER_PLAN_SCAN_INTERVAL":           defaultPlanScanInterval.String(),
		"CENTER_PLAN_START_WORKERS":           "2",
		"CENTER_PLAN_START_FANOUT":            "20",
	}

	mergeEnvFile(values, findEnvFile())
	mergeProcessEnv(values)

	heartbeatInterval := parseDurationValue(values["CENTER_HEARTBEAT_INTERVAL"], defaultHeartbeatInterval)
	offlineTimeout := parseDurationValue(values["CENTER_DEVICE_OFFLINE_TIMEOUT"], defaultOfflineTimeout)
	scanInterval := parseDurationValue(values["CENTER_DEVICE_OFFLINE_SCAN_INTERVAL"], defaultOfflineScanInterval)
	workflowScanInterval := parseDurationValue(values["CENTER_WORKFLOW_SCAN_INTERVAL"], defaultWorkflowScanInterval)
	planScanInterval := parseDurationValue(values["CENTER_PLAN_SCAN_INTERVAL"], defaultPlanScanInterval)
	planStartWorkers := parseIntValue(values["CENTER_PLAN_START_WORKERS"], defaultPlanStartWorkers)
	planStartFanout := parseIntValue(values["CENTER_PLAN_START_FANOUT"], defaultPlanStartFanout)

	return Config{
		HTTPAddr:                  values["CENTER_HTTP_ADDR"],
		DBPath:                    values["CENTER_DB_PATH"],
		CORSAllowedOrigins:        parseCSVValues(values["CENTER_CORS_ALLOWED_ORIGINS"]),
		HeartbeatInterval:         heartbeatInterval,
		DeviceOfflineTimeout:      offlineTimeout,
		DeviceOfflineScanInterval: scanInterval,
		ADBPath:                   values["CENTER_ADB_PATH"],
		AgentRootPath:             values["CENTER_AGENT_ROOT_PATH"],
		ScriptRootPath:            values["CENTER_SCRIPT_ROOT_PATH"],
		CenterBaseURL:             values["CENTER_BASE_URL"],
		ToolkitPath:               values["CENTER_TOOLKIT_PATH"],
		WorkflowScanInterval:      workflowScanInterval,
		PlanScanInterval:          planScanInterval,
		PlanStartWorkers:          planStartWorkers,
		PlanStartFanout:           planStartFanout,
	}
}

func parseCSVValues(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func parseDurationValue(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func parseIntValue(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}

	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return fallback
		}
		value = value*10 + int(ch-'0')
	}
	if value <= 0 {
		return fallback
	}
	return value
}

func findEnvFile() string {
	candidates := []string{
		".env",
		"../.env",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ".env"
}

func mergeProcessEnv(values map[string]string) {
	for key := range values {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			values[key] = value
		}
	}
}

func mergeEnvFile(values map[string]string, path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := values[key]; exists {
			if strings.TrimSpace(value) == "" {
				continue
			}
			values[key] = value
		}
	}
}

func parseEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	line = strings.TrimPrefix(line, "export ")
	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" {
		return "", "", false
	}

	value = trimInlineComment(value)
	value = strings.TrimSpace(value)
	value = trimMatchingQuotes(value)

	return key, value, true
}

func trimMatchingQuotes(value string) string {
	if len(value) < 2 {
		return value
	}

	first := value[0]
	last := value[len(value)-1]
	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		return value[1 : len(value)-1]
	}

	return value
}

func trimInlineComment(value string) string {
	var quote rune
	for i, char := range value {
		switch char {
		case '\'', '"':
			if quote == 0 {
				quote = char
				continue
			}
			if quote == char {
				quote = 0
			}
		case '#':
			if quote == 0 && (i == 0 || value[i-1] == ' ' || value[i-1] == '\t') {
				return value[:i]
			}
		}
	}
	return value
}

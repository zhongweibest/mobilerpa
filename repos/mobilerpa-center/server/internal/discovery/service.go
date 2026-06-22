package discovery

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Device 表示一次局域网无线调试设备发现结果。
type Device struct {
	ServiceName    string `json:"service_name"`
	ServiceType    string `json:"service_type"`
	ADBEndpoint    string `json:"adb_endpoint"`
	DeviceName     string `json:"device_name"`
	DeviceID       string `json:"device_id"`
	Source         string `json:"source"`
	ConnectionKind string `json:"connection_kind"`
	Connected      bool   `json:"connected"`
	Connectable    bool   `json:"connectable"`
	LastError      string `json:"last_error"`
}

// DeployRequest 表示一次批量下发 Agent 请求。
type DeployRequest struct {
	ADBEndpoints  []string `json:"adb_endpoints"`
	CenterBaseURL string   `json:"center_base_url"`
	ResetConfig   bool     `json:"reset_config"`
	RunAgent      bool     `json:"run_agent"`
}

// DeployResult 表示单台设备的一次下发结果。
type DeployResult struct {
	ADBEndpoint string `json:"adb_endpoint"`
	Connected   bool   `json:"connected"`
	Pushed      bool   `json:"pushed"`
	Started     bool   `json:"started"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// AgentActionRequest 表示对单台发现设备执行的 Agent 控制动作。
type AgentActionRequest struct {
	ADBEndpoint string `json:"adb_endpoint"`
	Action      string `json:"action"`
}

// AgentActionResult 表示一次 Agent 控制动作结果。
type AgentActionResult struct {
	ADBEndpoint string `json:"adb_endpoint"`
	Action      string `json:"action"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// PairRequest 表示通过无线调试配对码发起的一次配对请求。
type PairRequest struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	PairCode string `json:"pair_code"`
}

// PairResult 表示一次配对操作结果。
type PairResult struct {
	ADBEndpoint string `json:"adb_endpoint"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

type connectedDeviceInfo struct {
	endpoint   string
	deviceName string
	model      string
	product    string
}

var (
	ErrADBEndpointRequired    = errors.New("adb_endpoints_required")
	ErrCenterBaseURLRequired  = errors.New("center_base_url_required")
	ErrAgentActionRequired    = errors.New("agent_action_required")
	ErrAgentActionUnsupported = errors.New("agent_action_unsupported")
	ErrPairHostRequired       = errors.New("pair_host_required")
	ErrPairPortRequired       = errors.New("pair_port_required")
	ErrPairCodeRequired       = errors.New("pair_code_required")
)

// Service 封装局域网无线调试设备发现与网页触发下发能力。
type Service struct {
	db                   *sql.DB
	adbPath              string
	agentRootPath        string
	defaultCenterBaseURL string
	toolkitPath          string
	commandTimeout       time.Duration
}

// NewService 创建设备发现服务。
func NewService(db *sql.DB, adbPath string, agentRootPath string, defaultCenterBaseURL string, toolkitPath string) *Service {
	return &Service{
		db:                   db,
		adbPath:              strings.TrimSpace(adbPath),
		agentRootPath:        strings.TrimSpace(agentRootPath),
		defaultCenterBaseURL: strings.TrimSpace(defaultCenterBaseURL),
		toolkitPath:          strings.TrimSpace(toolkitPath),
		commandTimeout:       45 * time.Second,
	}
}

// ListDevices 返回当前局域网可发现的无线调试设备，并补充已连接设备。
func (s *Service) ListDevices(ctx context.Context) ([]Device, error) {
	mdnsOutput, err := s.runCommand(ctx, s.adbPath, "mdns", "services")
	if err != nil {
		return nil, fmt.Errorf("run adb mdns services: %w", err)
	}

	connectedMap, connectedDetails, err := s.listConnectedDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list adb connected endpoints: %w", err)
	}

	devices := mergeDevices(
		parseMDNSServices(mdnsOutput, connectedMap),
		buildConnectedFallbackDevices(connectedDetails),
	)

	if err := s.attachDeviceIDs(ctx, devices); err != nil {
		return nil, fmt.Errorf("attach device ids: %w", err)
	}

	return devices, nil
}

// DeployAgent 对多台设备执行网页触发的 Agent 下发。
func (s *Service) DeployAgent(ctx context.Context, req DeployRequest) ([]DeployResult, error) {
	if len(req.ADBEndpoints) == 0 {
		return nil, ErrADBEndpointRequired
	}

	centerBaseURL := strings.TrimSpace(req.CenterBaseURL)
	if centerBaseURL == "" {
		centerBaseURL = s.defaultCenterBaseURL
	}
	if centerBaseURL == "" {
		return nil, ErrCenterBaseURLRequired
	}

	results := make([]DeployResult, 0, len(req.ADBEndpoints))
	for _, endpoint := range req.ADBEndpoints {
		result := DeployResult{
			ADBEndpoint: strings.TrimSpace(endpoint),
			Status:      "error",
		}
		if result.ADBEndpoint == "" {
			result.Message = "empty_adb_endpoint"
			results = append(results, result)
			continue
		}

		if _, err := s.runCommand(ctx, s.adbPath, "connect", result.ADBEndpoint); err != nil {
			result.Message = "adb_connect_failed: " + err.Error()
			results = append(results, result)
			continue
		}
		result.Connected = true

		if _, err := s.pushViaToolkit(ctx, result.ADBEndpoint, centerBaseURL, req.ResetConfig, req.RunAgent); err != nil {
			result.Message = "toolkit_push_failed: " + err.Error()
			results = append(results, result)
			continue
		}

		result.Pushed = true
		result.Started = req.RunAgent
		result.Status = "ok"
		if req.RunAgent {
			result.Message = "agent_deployed_and_started"
		} else {
			result.Message = "agent_deployed_without_start"
		}
		results = append(results, result)
	}

	return results, nil
}

// ControlAgent 对单台设备执行启动或停止 Agent 动作。
func (s *Service) ControlAgent(ctx context.Context, req AgentActionRequest) (AgentActionResult, error) {
	result := AgentActionResult{
		ADBEndpoint: strings.TrimSpace(req.ADBEndpoint),
		Action:      strings.TrimSpace(req.Action),
		Status:      "error",
	}

	if result.ADBEndpoint == "" {
		return result, ErrADBEndpointRequired
	}
	if result.Action == "" {
		return result, ErrAgentActionRequired
	}
	if result.Action != "start" && result.Action != "stop" && result.Action != "disconnect" {
		return result, ErrAgentActionUnsupported
	}

	if result.Action == "disconnect" {
		targets, resolveErr := s.resolveDisconnectTargets(ctx, result.ADBEndpoint)
		disconnectErrors := make([]string, 0)
		disconnected := false

		if resolveErr != nil {
			disconnectErrors = append(disconnectErrors, resolveErr.Error())
		}

		for _, target := range targets {
			if _, err := s.runCommand(ctx, s.adbPath, "disconnect", target); err != nil {
				disconnectErrors = append(disconnectErrors, err.Error())
				continue
			}
			disconnected = true
		}

		if !disconnected {
			result.Message = "adb_disconnect_failed: " + strings.Join(disconnectErrors, " | ")
			return result, nil
		}

		result.Status = "ok"
		result.Message = "device_disconnected"
		return result, nil
	}

	if _, err := s.runCommand(ctx, s.adbPath, "connect", result.ADBEndpoint); err != nil {
		result.Message = "adb_connect_failed: " + err.Error()
		return result, nil
	}

	if err := s.controlViaToolkit(ctx, result.ADBEndpoint, result.Action); err != nil {
		result.Message = "toolkit_agent_action_failed: " + err.Error()
		return result, nil
	}

	result.Status = "ok"
	if result.Action == "start" {
		result.Message = "agent_started"
	} else {
		result.Message = "agent_stopped"
	}

	return result, nil
}

// PairDevice 使用 adb pair 对指定无线调试地址发起配对。
func (s *Service) PairDevice(ctx context.Context, req PairRequest) (PairResult, error) {
	result := PairResult{
		ADBEndpoint: strings.TrimSpace(req.Host) + ":" + strings.TrimSpace(req.Port),
		Status:      "error",
	}

	host := strings.TrimSpace(req.Host)
	port := strings.TrimSpace(req.Port)
	pairCode := strings.TrimSpace(req.PairCode)

	if host == "" {
		return result, ErrPairHostRequired
	}
	if port == "" {
		return result, ErrPairPortRequired
	}
	if pairCode == "" {
		return result, ErrPairCodeRequired
	}

	endpoint := host + ":" + port
	output, err := s.runCommand(ctx, s.adbPath, "pair", endpoint, pairCode)
	if err != nil {
		result.Message = "adb_pair_failed: " + err.Error()
		return result, nil
	}

	result.ADBEndpoint = endpoint
	result.Status = "ok"
	result.Message = strings.TrimSpace(output)
	if result.Message == "" {
		result.Message = "adb_pair_ok"
	}
	return result, nil
}

func (s *Service) pushViaToolkit(ctx context.Context, endpoint string, centerBaseURL string, resetConfig bool, runAgent bool) (string, error) {
	toolkitPath := strings.TrimSpace(s.toolkitPath)
	if toolkitPath == "" {
		return "", fmt.Errorf("toolkit path not configured")
	}

	args := []string{
		"push-center",
		"--device", endpoint,
		"--center-base-url", centerBaseURL,
		"--agent-root", s.agentRootPath,
		"--adb-path", s.adbPath,
	}
	if resetConfig {
		args = append(args, "--reset-config")
	}
	if !runAgent {
		args = append(args, "--no-run")
	}

	return s.runCommand(ctx, toolkitPath, args...)
}

func (s *Service) controlViaToolkit(ctx context.Context, endpoint string, action string) error {
	toolkitPath := strings.TrimSpace(s.toolkitPath)
	if toolkitPath == "" {
		return fmt.Errorf("toolkit path not configured")
	}

	args := []string{
		action + "-agent",
		"--device", endpoint,
		"--adb-path", s.adbPath,
	}

	if action == "start" {
		args = append(args, "--remote-root", "/sdcard/脚本")
	}

	_, err := s.runCommand(ctx, toolkitPath, args...)
	return err
}

func (s *Service) listConnectedDevices(ctx context.Context) (map[string]struct{}, []connectedDeviceInfo, error) {
	output, err := s.runCommand(ctx, s.adbPath, "devices", "-l")
	if err != nil {
		return nil, nil, err
	}

	lines := strings.Split(output, "\n")
	connectedSet := make(map[string]struct{})
	results := make([]connectedDeviceInfo, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || fields[1] != "device" {
			continue
		}

		endpoint := strings.TrimSpace(fields[0])
		if endpoint == "" {
			continue
		}
		if _, exists := connectedSet[endpoint]; exists {
			continue
		}

		connectedSet[endpoint] = struct{}{}
		info := connectedDeviceInfo{
			endpoint:   endpoint,
			deviceName: extractDeviceName(endpoint),
		}
		for _, field := range fields[2:] {
			key, value, found := strings.Cut(field, ":")
			if !found {
				continue
			}
			switch key {
			case "model":
				info.model = value
			case "device":
				info.deviceName = value
			case "product":
				info.product = value
			}
		}
		if strings.TrimSpace(info.deviceName) == "" {
			if strings.TrimSpace(info.model) != "" {
				info.deviceName = strings.TrimSpace(info.model)
			} else if strings.TrimSpace(info.product) != "" {
				info.deviceName = strings.TrimSpace(info.product)
			}
		}
		results = append(results, info)
	}

	return connectedSet, results, nil
}

func parseMDNSServices(output string, connectedEndpoints map[string]struct{}) []Device {
	lines := strings.Split(output, "\n")
	results := make([]Device, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "list of discovered") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		endpoint := strings.TrimSpace(fields[len(fields)-1])
		serviceType := strings.TrimSpace(fields[len(fields)-2])
		serviceName := strings.TrimSpace(strings.Join(fields[:len(fields)-2], " "))

		_, connected := connectedEndpoints[endpoint]
		results = append(results, Device{
			ServiceName:    serviceName,
			ServiceType:    serviceType,
			ADBEndpoint:    endpoint,
			DeviceName:     extractDeviceName(serviceName),
			Source:         "mdns",
			ConnectionKind: classifyConnectionKind(serviceType, connected),
			Connected:      connected,
			Connectable:    isConnectableService(serviceType, endpoint),
		})
	}

	return results
}

func buildConnectedFallbackDevices(connectedDevices []connectedDeviceInfo) []Device {
	results := make([]Device, 0, len(connectedDevices))
	for _, item := range connectedDevices {
		results = append(results, Device{
			ServiceName:    item.endpoint,
			ServiceType:    "",
			ADBEndpoint:    item.endpoint,
			DeviceName:     item.deviceName,
			Source:         "adb_devices",
			ConnectionKind: "connected_device",
			Connected:      true,
			Connectable:    true,
		})
	}
	return results
}

func mergeDevices(primary []Device, fallback []Device) []Device {
	merged := make(map[string]Device, len(primary)+len(fallback))
	order := make([]string, 0, len(primary)+len(fallback))

	appendDevice := func(device Device) {
		key := strings.TrimSpace(device.ADBEndpoint)
		if key == "" {
			key = strings.TrimSpace(device.ServiceName)
		}
		if key == "" {
			return
		}

		if existing, exists := merged[key]; exists {
			if existing.ServiceName == "" && device.ServiceName != "" {
				existing.ServiceName = device.ServiceName
			}
			if existing.ServiceType == "" && device.ServiceType != "" {
				existing.ServiceType = device.ServiceType
			}
			if existing.ADBEndpoint == "" && device.ADBEndpoint != "" {
				existing.ADBEndpoint = device.ADBEndpoint
			}
			if existing.DeviceName == "" && device.DeviceName != "" {
				existing.DeviceName = device.DeviceName
			}
			existing.Connected = existing.Connected || device.Connected
			existing.Connectable = existing.Connectable || device.Connectable
			if existing.LastError == "" && device.LastError != "" {
				existing.LastError = device.LastError
			}
			existing.Source = mergeSource(existing.Source, device.Source)
			if existing.ConnectionKind == "" && device.ConnectionKind != "" {
				existing.ConnectionKind = device.ConnectionKind
			}
			if existing.ConnectionKind != "connected_device" && device.ConnectionKind == "connected_device" {
				existing.ConnectionKind = device.ConnectionKind
			}
			merged[key] = existing
			return
		}

		merged[key] = device
		order = append(order, key)
	}

	for _, device := range primary {
		appendDevice(device)
	}
	for _, device := range fallback {
		appendDevice(device)
	}

	sort.SliceStable(order, func(i int, j int) bool {
		left := merged[order[i]]
		right := merged[order[j]]
		if left.Connected != right.Connected {
			return left.Connected
		}
		return order[i] < order[j]
	})

	results := make([]Device, 0, len(order))
	for _, key := range order {
		results = append(results, merged[key])
	}
	return results
}

func mergeSource(left string, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	switch {
	case left == "":
		return right
	case right == "":
		return left
	case left == right:
		return left
	default:
		return "merged"
	}
}

func isConnectableService(serviceType string, endpoint string) bool {
	if strings.TrimSpace(endpoint) == "" {
		return false
	}
	serviceType = strings.TrimSpace(serviceType)
	return serviceType == "" || serviceType == "_adb-tls-connect._tcp"
}

func classifyConnectionKind(serviceType string, connected bool) string {
	if connected {
		return "connected_device"
	}
	switch strings.TrimSpace(serviceType) {
	case "_adb-tls-pairing._tcp":
		return "pairing_service"
	case "_adb-tls-connect._tcp":
		return "connect_service"
	default:
		return "unknown_service"
	}
}

func extractDeviceName(serviceName string) string {
	name := strings.TrimSpace(serviceName)
	if name == "" {
		return ""
	}

	if strings.HasPrefix(name, "adb-") && strings.Contains(name, "._adb-tls-connect._tcp") {
		name = strings.TrimPrefix(name, "adb-")
		name = strings.TrimSuffix(name, "._adb-tls-connect._tcp")
		return strings.TrimSpace(name)
	}

	if strings.Contains(name, "._adb-tls-connect._tcp") {
		name = strings.TrimSuffix(name, "._adb-tls-connect._tcp")
		return strings.TrimSpace(name)
	}

	if strings.Contains(name, "._adb-tls-pairing._tcp") {
		name = strings.TrimSuffix(name, "._adb-tls-pairing._tcp")
		return strings.TrimSpace(name)
	}

	if idx := strings.Index(name, " "); idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	return name
}

func (s *Service) resolveDisconnectTargets(ctx context.Context, adbEndpoint string) ([]string, error) {
	targets := collectDisconnectTargets(adbEndpoint, "")
	if isNetworkEndpoint(adbEndpoint) {
		return targets, nil
	}

	mdnsOutput, err := s.runCommand(ctx, s.adbPath, "mdns", "services")
	if err != nil {
		return targets, fmt.Errorf("resolve mdns endpoint: %w", err)
	}

	return collectDisconnectTargets(adbEndpoint, mdnsOutput), nil
}

func collectDisconnectTargets(adbEndpoint string, mdnsOutput string) []string {
	normalizedEndpoint := strings.TrimSpace(adbEndpoint)
	targets := make([]string, 0, 2)
	seen := make(map[string]struct{})
	appendTarget := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		targets = append(targets, value)
	}

	appendTarget(normalizedEndpoint)
	if normalizedEndpoint == "" || mdnsOutput == "" {
		return targets
	}

	normalizedDeviceName := extractDeviceName(normalizedEndpoint)
	for _, line := range strings.Split(mdnsOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "list of discovered") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		serviceName := strings.TrimSpace(strings.Join(fields[:len(fields)-2], " "))
		endpoint := strings.TrimSpace(fields[len(fields)-1])
		if endpoint == "" {
			continue
		}

		if serviceName == normalizedEndpoint || extractDeviceName(serviceName) == normalizedDeviceName {
			appendTarget(endpoint)
		}
	}

	return targets
}

func isNetworkEndpoint(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "._adb-tls-connect._tcp") || strings.Contains(value, "._adb-tls-pairing._tcp") {
		return false
	}
	host, port, found := strings.Cut(value, ":")
	if !found || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return false
	}
	for _, char := range port {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (s *Service) attachDeviceIDs(ctx context.Context, devices []Device) error {
	if s.db == nil || len(devices) == 0 {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, adb_serial
FROM devices
WHERE adb_serial != ''`)
	if err != nil {
		return fmt.Errorf("query device adb serials: %w", err)
	}
	defer rows.Close()

	deviceByADBSerial := make(map[string]string)
	for rows.Next() {
		var deviceID string
		var adbSerial string
		if err := rows.Scan(&deviceID, &adbSerial); err != nil {
			return fmt.Errorf("scan device adb serial: %w", err)
		}
		adbSerial = strings.TrimSpace(adbSerial)
		deviceID = strings.TrimSpace(deviceID)
		if adbSerial == "" || deviceID == "" {
			continue
		}
		deviceByADBSerial[adbSerial] = deviceID
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate device adb serials: %w", err)
	}

	for index := range devices {
		adbEndpoint := strings.TrimSpace(devices[index].ADBEndpoint)
		if adbEndpoint == "" {
			continue
		}
		if deviceID, exists := deviceByADBSerial[adbEndpoint]; exists {
			devices[index].DeviceID = deviceID
		}
	}
	return nil
}

func (s *Service) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	return s.runCommandWithEnv(ctx, name, args, nil)
}

func (s *Service) runCommandWithEnv(ctx context.Context, name string, args []string, env []string) (string, error) {
	commandCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		commandCtx, cancel = context.WithTimeout(ctx, s.commandTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(commandCtx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
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

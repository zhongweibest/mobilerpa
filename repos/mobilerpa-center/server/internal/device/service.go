package device

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrDeviceNotFound 表示没有查询到指定设备。
var ErrDeviceNotFound = errors.New("device not found")

// ErrDeviceOnline 表示设备当前在线，不能直接删除。
var ErrDeviceOnline = errors.New("device is online")

// ErrDeviceAccessibilityRequired 表示设备未开启无障碍，不允许进入执行阶段。
var ErrDeviceAccessibilityRequired = errors.New("device accessibility required")

// ErrDeviceForegroundServiceRequired 表示设备前台服务或通知保活状态不满足要求。
var ErrDeviceForegroundServiceRequired = errors.New("device foreground service required")

// ErrDeviceBatteryOptimizationRequired 表示设备未放开电量优化限制。
var ErrDeviceBatteryOptimizationRequired = errors.New("device battery optimization exemption required")

// ErrDeviceExecutionProfileUnknown 表示设备尚未上报执行环境自检结果。
var ErrDeviceExecutionProfileUnknown = errors.New("device execution profile unknown")

// 设备服务管理设备注册、设备查询和连接状态更新。
type Service struct {
	db *sql.DB

	mu       sync.RWMutex
	sessions map[string]SessionState
}

// 会话状态描述设备会话在内存中的连接状态。
type SessionState struct {
	// LastSeen 是最近一次在 WebSocket 通道上观察到该设备的时间。
	LastSeen time.Time `json:"last_seen"`
	// Connected 表示该设备当前是否被视为已连接。
	Connected bool `json:"connected"`
}

// 注册请求是设备向中心服务注册时提交的请求载荷。
type RegisterRequest struct {
	// AgentUUID 是设备端本地生成并持久保存的标识。
	AgentUUID string `json:"agent_uuid"`
	// DeviceName 是设备端上报的可读设备名称。
	DeviceName string `json:"device_name"`
	// Brand 是设备端上报的设备品牌。
	Brand string `json:"brand"`
	// Model 是设备端上报的设备型号。
	Model string `json:"model"`
	// AndroidID 是设备端上报的 Android 标识，主要用于排障参考。
	AndroidID string `json:"android_id"`
	// ADBSerial 是设备端可获取时上报的 ADB 序列号。
	ADBSerial string `json:"adb_serial"`
}

// 设备记录表示中心服务持久化并返回的设备信息。
type Device struct {
	// DeviceID 是中心服务生成的稳定设备标识。
	DeviceID string `json:"device_id"`
	// AgentUUID 是设备端生成并持久保存的标识。
	AgentUUID string `json:"agent_uuid"`
	// DeviceName 是设备最近一次上报的可读设备名称。
	DeviceName string `json:"device_name"`
	// PhysicalSlot 是运维维护的设备物理位置。
	PhysicalSlot string `json:"physical_slot"`
	// GroupName 是运维分配的设备逻辑分组。
	GroupName string `json:"group_name"`
	// Status 是设备当前的高层状态，例如在线或离线。
	Status string `json:"status"`
	// BindStatus 表示设备是否已经在后台完成人工绑定。
	BindStatus string `json:"bind_status"`
	// IP 是中心服务最近一次观察到的设备网络地址。
	IP string `json:"ip"`
	// Brand 是设备最近一次上报的品牌。
	Brand string `json:"brand"`
	// Model 是设备最近一次上报的型号。
	Model string `json:"model"`
	// AndroidID 是设备最近一次上报的 Android 标识。
	AndroidID string `json:"android_id"`
	// ADBSerial 是设备最近一次上报的 ADB 序列号。
	ADBSerial string `json:"adb_serial"`
	// CurrentTaskID 是当前关联到该设备的任务标识。
	CurrentTaskID string `json:"current_task_id"`
	// CurrentStep 是当前任务最近一次上报的执行步骤。
	CurrentStep string `json:"current_step"`
	// LastError 是最近一次持久化的设备级错误摘要。
	LastError string `json:"last_error"`
	// AccessibilityStatus 是无障碍状态。
	AccessibilityStatus string `json:"accessibility_status"`
	// ForegroundServiceStatus 是前台服务或通知保活状态。
	ForegroundServiceStatus string `json:"foreground_service_status"`
	// BatteryOptimizationIgnoredStatus 是电量优化忽略状态。
	BatteryOptimizationIgnoredStatus string `json:"battery_optimization_ignored_status"`
	// EnvCheckedAt 是最近一次环境自检时间。
	EnvCheckedAt string `json:"env_checked_at"`
	// EnvCheckMessage 是最近一次环境自检摘要。
	EnvCheckMessage string `json:"env_check_message"`
	// LastHeartbeatAt 是最近一次持久化的心跳时间。
	LastHeartbeatAt string `json:"last_heartbeat_at"`
	// CreatedAt 是设备记录创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是设备记录最近更新时间。
	UpdatedAt string `json:"updated_at"`
}

// 注册结果是设备注册成功后的响应结果。
type RegisterResult struct {
	// DeviceID 是中心服务分配的稳定设备标识。
	DeviceID string `json:"device_id"`
	// BindStatus 是设备当前的人工绑定状态。
	BindStatus string `json:"bind_status"`
	// Status 是返回给设备端的注册结果状态。
	Status string `json:"status"`
}

// ExecutionProfile 表示设备端上报的执行前环境自检结果。
type ExecutionProfile struct {
	AccessibilityStatus             string `json:"accessibility_status"`
	ForegroundServiceStatus         string `json:"foreground_service_status"`
	BatteryOptimizationIgnoredStatus string `json:"battery_optimization_ignored_status"`
	CheckedAt                       string `json:"checked_at"`
	Message                         string `json:"message"`
}

// 创建设备服务会使用指定数据库连接作为存储后端。
func NewService(db *sql.DB) *Service {
	return &Service{
		db:       db,
		sessions: make(map[string]SessionState),
	}
}

// 注册设备会根据设备端上报的 Agent 标识创建或更新设备记录。
func (s *Service) Register(ctx context.Context, req RegisterRequest, r *http.Request) (RegisterResult, error) {
	req.AgentUUID = strings.TrimSpace(req.AgentUUID)
	if req.AgentUUID == "" {
		return RegisterResult{}, fmt.Errorf("agent_uuid is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	ip := clientIP(r)

	existing, err := s.findByAgentUUID(ctx, req.AgentUUID)
	if err != nil {
		return RegisterResult{}, err
	}

	if existing != nil {
		if err := s.updateRegistration(ctx, existing.DeviceID, req, ip, now); err != nil {
			return RegisterResult{}, err
		}

		return RegisterResult{
			DeviceID:   existing.DeviceID,
			BindStatus: existing.BindStatus,
			Status:     "registered",
		}, nil
	}

	deviceID, err := s.insertDevice(ctx, req, ip, now)
	if err != nil {
		return RegisterResult{}, err
	}

	return RegisterResult{
		DeviceID:   deviceID,
		BindStatus: "pending",
		Status:     "registered",
	}, nil
}

// 查询设备列表会按创建时间返回所有已持久化的设备记录。
func (s *Service) List(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
       brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
       accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
       last_heartbeat_at, created_at, updated_at
FROM devices
ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(
			&d.DeviceID, &d.AgentUUID, &d.DeviceName, &d.PhysicalSlot, &d.GroupName, &d.Status, &d.BindStatus, &d.IP,
			&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &d.CurrentTaskID, &d.CurrentStep, &d.LastError,
			&d.AccessibilityStatus, &d.ForegroundServiceStatus, &d.BatteryOptimizationIgnoredStatus, &d.EnvCheckedAt, &d.EnvCheckMessage,
			&d.LastHeartbeatAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}

// ListOnline 返回当前状态为 online 的设备列表。
func (s *Service) ListOnline(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
       brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
       accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
       last_heartbeat_at, created_at, updated_at
FROM devices
WHERE status = 'online'
ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query online devices: %w", err)
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(
			&d.DeviceID, &d.AgentUUID, &d.DeviceName, &d.PhysicalSlot, &d.GroupName, &d.Status, &d.BindStatus, &d.IP,
			&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &d.CurrentTaskID, &d.CurrentStep, &d.LastError,
			&d.AccessibilityStatus, &d.ForegroundServiceStatus, &d.BatteryOptimizationIgnoredStatus, &d.EnvCheckedAt, &d.EnvCheckMessage,
			&d.LastHeartbeatAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan online device: %w", err)
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}

// GetByID 会按照中心服务分配的设备标识查询单个设备记录。
func (s *Service) GetByID(ctx context.Context, deviceID string) (Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Device{}, ErrDeviceNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id, agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
       brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
       accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
       last_heartbeat_at, created_at, updated_at
FROM devices
WHERE id = ?`, deviceID)

	d, err := scanDevice(row)
	if err != nil {
		return Device{}, fmt.Errorf("get device by id: %w", err)
	}

	return d, nil
}

// DeleteByID 会删除指定离线设备，用于清理历史无效设备记录。
func (s *Service) DeleteByID(ctx context.Context, deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ErrDeviceNotFound
	}

	current, err := s.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}

	if strings.TrimSpace(current.Status) == "online" {
		return ErrDeviceOnline
	}

	result, err := s.db.ExecContext(ctx, `
DELETE FROM devices
WHERE id = ?`, deviceID)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete device rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrDeviceNotFound
	}

	s.mu.Lock()
	delete(s.sessions, deviceID)
	s.mu.Unlock()

	return nil
}

// 标记设备在线会同时更新内存状态和持久化状态。
func (s *Service) MarkOnline(ctx context.Context, deviceID string, seenAt time.Time) (bool, error) {
	s.mu.RLock()
	previous, exists := s.sessions[deviceID]
	s.mu.RUnlock()

	s.mu.Lock()
	s.sessions[deviceID] = SessionState{LastSeen: seenAt, Connected: true}
	s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET status = ?, last_heartbeat_at = ?, updated_at = ?
WHERE id = ?`,
		"online",
		seenAt.UTC().Format(time.RFC3339),
		seenAt.UTC().Format(time.RFC3339),
		deviceID,
	)
	if err != nil {
		return false, fmt.Errorf("mark online: %w", err)
	}

	becameOnline := !exists || !previous.Connected
	return becameOnline, nil
}

// 标记设备离线会清理内存会话，并在存储中写入离线状态。
func (s *Service) MarkOffline(ctx context.Context, deviceID string, seenAt time.Time) error {
	s.mu.Lock()
	delete(s.sessions, deviceID)
	s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET status = ?, updated_at = ?
WHERE id = ?`,
		"offline",
		seenAt.UTC().Format(time.RFC3339),
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("mark offline: %w", err)
	}

	return nil
}

// MarkStaleOffline 会把最近心跳时间早于截止时间的在线设备批量标记为离线。
func (s *Service) MarkStaleOffline(ctx context.Context, cutoff time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id
FROM devices
WHERE status = 'online'
  AND last_heartbeat_at != ''
  AND last_heartbeat_at < ?`, cutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query stale devices: %w", err)
	}
	defer rows.Close()

	deviceIDs := make([]string, 0)
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			return nil, fmt.Errorf("scan stale device: %w", err)
		}
		deviceIDs = append(deviceIDs, deviceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale devices: %w", err)
	}

	if len(deviceIDs) == 0 {
		return nil, nil
	}

	markedAt := time.Now()
	for _, deviceID := range deviceIDs {
		if err := s.MarkOffline(ctx, deviceID, markedAt); err != nil {
			return nil, err
		}
	}

	return deviceIDs, nil
}

// UpdateExecutionProfile 持久化设备最近一次执行环境自检结果。
func (s *Service) UpdateExecutionProfile(ctx context.Context, deviceID string, profile ExecutionProfile) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ErrDeviceNotFound
	}

	accessibilityStatus := normalizeExecutionStatus(profile.AccessibilityStatus)
	foregroundServiceStatus := normalizeExecutionStatus(profile.ForegroundServiceStatus)
	batteryOptimizationStatus := normalizeExecutionStatus(profile.BatteryOptimizationIgnoredStatus)
	checkedAt := strings.TrimSpace(profile.CheckedAt)
	if checkedAt == "" {
		checkedAt = time.Now().UTC().Format(time.RFC3339)
	}
	message := strings.TrimSpace(profile.Message)

	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET accessibility_status = ?,
    foreground_service_status = ?,
    battery_optimization_ignored_status = ?,
    env_checked_at = ?,
    env_check_message = ?,
    updated_at = ?
WHERE id = ?`,
		accessibilityStatus,
		foregroundServiceStatus,
		batteryOptimizationStatus,
		checkedAt,
		message,
		time.Now().UTC().Format(time.RFC3339),
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("update execution profile: %w", err)
	}
	return nil
}

// EnsureExecutionReady 检查设备是否满足执行前环境要求。
func (s *Service) EnsureExecutionReady(ctx context.Context, deviceID string) error {
	current, err := s.GetByID(ctx, deviceID)
	if err != nil {
		return err
	}

	if normalizeExecutionStatus(current.AccessibilityStatus) == "unknown" &&
		normalizeExecutionStatus(current.ForegroundServiceStatus) == "unknown" &&
		normalizeExecutionStatus(current.BatteryOptimizationIgnoredStatus) == "unknown" {
		return fmt.Errorf("%w: %s", ErrDeviceExecutionProfileUnknown, current.DeviceID)
	}
	if normalizeExecutionStatus(current.AccessibilityStatus) != "enabled" {
		return fmt.Errorf("%w: %s", ErrDeviceAccessibilityRequired, current.DeviceID)
	}
	if normalizeExecutionStatus(current.ForegroundServiceStatus) != "enabled" {
		return fmt.Errorf("%w: %s", ErrDeviceForegroundServiceRequired, current.DeviceID)
	}
	if normalizeExecutionStatus(current.BatteryOptimizationIgnoredStatus) != "enabled" {
		return fmt.Errorf("%w: %s", ErrDeviceBatteryOptimizationRequired, current.DeviceID)
	}
	return nil
}

func (s *Service) findByAgentUUID(ctx context.Context, agentUUID string) (*Device, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
       brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
       accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
       last_heartbeat_at, created_at, updated_at
FROM devices
WHERE agent_uuid = ?`, agentUUID)

	d, err := scanDevice(row)
	if errors.Is(err, ErrDeviceNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find device by agent_uuid: %w", err)
	}
	return &d, nil
}

type deviceScanner interface {
	Scan(dest ...any) error
}

func scanDevice(scanner deviceScanner) (Device, error) {
	var (
		d             Device
		deviceID      int64
		currentTaskID int64
	)
	err := scanner.Scan(
		&deviceID, &d.AgentUUID, &d.DeviceName, &d.PhysicalSlot, &d.GroupName, &d.Status, &d.BindStatus, &d.IP,
		&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &currentTaskID, &d.CurrentStep, &d.LastError,
		&d.AccessibilityStatus, &d.ForegroundServiceStatus, &d.BatteryOptimizationIgnoredStatus, &d.EnvCheckedAt, &d.EnvCheckMessage,
		&d.LastHeartbeatAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Device{}, ErrDeviceNotFound
	}
	if err != nil {
		return Device{}, err
	}
	d.DeviceID = strconv.FormatInt(deviceID, 10)
	if currentTaskID > 0 {
		d.CurrentTaskID = strconv.FormatInt(currentTaskID, 10)
	}
	return d, nil
}

func normalizeExecutionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "enabled", "ok", "ready", "ignored":
		return "enabled"
	case "disabled", "denied", "restricted", "missing":
		return "disabled"
	default:
		return "unknown"
	}
}

func (s *Service) insertDevice(ctx context.Context, req RegisterRequest, ip string, now string) (string, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO devices (
    agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
    brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
    accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
    last_heartbeat_at, created_at, updated_at
) VALUES (?, ?, '', '', 'offline', 'pending', ?, ?, ?, ?, ?, 0, '', '', 'unknown', 'unknown', 'unknown', '', '', '', ?, ?)`,
		req.AgentUUID,
		req.DeviceName,
		ip,
		req.Brand,
		req.Model,
		req.AndroidID,
		req.ADBSerial,
		now,
		now,
	)
	if err != nil {
		return "", fmt.Errorf("insert device: %w", err)
	}
	insertedID, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("read inserted device id: %w", err)
	}
	return strconv.FormatInt(insertedID, 10), nil
}

func (s *Service) updateRegistration(ctx context.Context, deviceID string, req RegisterRequest, ip string, now string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET device_name = ?, ip = ?, brand = ?, model = ?, android_id = ?, adb_serial = ?, updated_at = ?
WHERE id = ?`,
		req.DeviceName,
		ip,
		req.Brand,
		req.Model,
		req.AndroidID,
		req.ADBSerial,
		now,
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("update device registration: %w", err)
	}
	return nil
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

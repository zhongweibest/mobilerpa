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
var ErrLocationNodeNotFound = errors.New("location node not found")
var ErrLocationNodeOccupied = errors.New("location node occupied")
var ErrLocationNodeFieldsRequired = errors.New("location node fields are required")
var ErrLocationNodeParentRequired = errors.New("location node parent required")
var ErrLocationNodeTypeUnsupported = errors.New("location node type unsupported")
var ErrLocationNodeHierarchyInvalid = errors.New("location node hierarchy invalid")
var ErrLocationNodeDeleteRootBound = errors.New("location node delete root bound")

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
	// DeviceLinkSN 是设备发现阶段下发的稳定链路标识，通常为 mDNS 服务名。
	DeviceLinkSN string `json:"device_link_sn"`
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
	// SlotZoneID 是设备所在分区节点 ID。
	SlotZoneID string `json:"slot_zone_id"`
	// SlotRowID 是设备所在排号节点 ID。
	SlotRowID string `json:"slot_row_id"`
	// SlotPositionID 是设备所在槽位节点 ID。
	SlotPositionID string `json:"slot_position_id"`
	// SlotZone 是设备所在分区。
	SlotZone string `json:"slot_zone"`
	// SlotRow 是设备所在排号。
	SlotRow string `json:"slot_row"`
	// SlotPosition 是设备所在槽位号。
	SlotPosition string `json:"slot_position"`
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
	// DeviceLinkSN 是设备发现阶段下发并由 Agent 回传的链路标识。
	DeviceLinkSN string `json:"device_link_sn"`
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

// LocationNode 表示位置树中的一个节点。
type LocationNode struct {
	NodeID    string `json:"node_id"`
	ParentID  string `json:"parent_id"`
	NodeType  string `json:"node_type"`
	NodeName  string `json:"node_name"`
	DeviceID  string `json:"device_id"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ZoneName  string `json:"zone_name"`
	RowName   string `json:"row_name"`
	SlotName  string `json:"slot_name"`
	PathText  string `json:"path_text"`
}

// CreateLocationNodeRequest 描述创建位置节点的请求。
type CreateLocationNodeRequest struct {
	ParentID  string `json:"parent_id"`
	NodeType  string `json:"node_type"`
	NodeName  string `json:"node_name"`
	SortOrder int    `json:"sort_order"`
}

// UpdateLocationNodeRequest 描述更新位置节点的请求。
type UpdateLocationNodeRequest struct {
	ParentID  string `json:"parent_id"`
	NodeName  string `json:"node_name"`
	SortOrder int    `json:"sort_order"`
}

// BindLocationNodeRequest 描述把设备绑定到槽位节点的请求。
type BindLocationNodeRequest struct {
	DeviceID string `json:"device_id"`
}

type DeviceListFilter struct {
	SlotZoneID     string
	SlotRowID      string
	SlotPositionID string
}

// ExecutionProfile 表示设备端上报的执行前环境自检结果。
type ExecutionProfile struct {
	AccessibilityStatus              string `json:"accessibility_status"`
	ForegroundServiceStatus          string `json:"foreground_service_status"`
	BatteryOptimizationIgnoredStatus string `json:"battery_optimization_ignored_status"`
	CheckedAt                        string `json:"checked_at"`
	Message                          string `json:"message"`
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
func (s *Service) List(ctx context.Context, filter DeviceListFilter) ([]Device, error) {
	query := `
SELECT id, agent_uuid, device_name, physical_slot, group_name, status, bind_status, ip,
       slot_zone_id, slot_row_id, slot_position_id, slot_zone, slot_row, slot_position,
       brand, model, android_id, adb_serial, device_link_sn, current_task_id, current_step, last_error,
       accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
       last_heartbeat_at, created_at, updated_at
FROM devices
WHERE 1 = 1`
	args := make([]any, 0, 3)
	if value := strings.TrimSpace(filter.SlotZoneID); value != "" {
		query += " AND slot_zone_id = ?"
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SlotRowID); value != "" {
		query += " AND slot_row_id = ?"
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.SlotPositionID); value != "" {
		query += " AND slot_position_id = ?"
		args = append(args, value)
	}
	query += " ORDER BY created_at ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	devices := make([]Device, 0)
	for rows.Next() {
		var d Device
		if err := rows.Scan(
			&d.DeviceID, &d.AgentUUID, &d.DeviceName, &d.PhysicalSlot, &d.GroupName, &d.Status, &d.BindStatus, &d.IP,
			&d.SlotZoneID, &d.SlotRowID, &d.SlotPositionID, &d.SlotZone, &d.SlotRow, &d.SlotPosition,
			&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &d.DeviceLinkSN, &d.CurrentTaskID, &d.CurrentStep, &d.LastError,
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
       slot_zone_id, slot_row_id, slot_position_id, slot_zone, slot_row, slot_position,
       brand, model, android_id, adb_serial, device_link_sn, current_task_id, current_step, last_error,
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
			&d.SlotZoneID, &d.SlotRowID, &d.SlotPositionID, &d.SlotZone, &d.SlotRow, &d.SlotPosition,
			&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &d.DeviceLinkSN, &d.CurrentTaskID, &d.CurrentStep, &d.LastError,
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
       slot_zone_id, slot_row_id, slot_position_id, slot_zone, slot_row, slot_position,
       brand, model, android_id, adb_serial, device_link_sn, current_task_id, current_step, last_error,
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

// UpdateDeviceLinkSN 持久化设备发现链路标识。
func (s *Service) UpdateDeviceLinkSN(ctx context.Context, deviceID string, deviceLinkSN string) error {
	deviceID = strings.TrimSpace(deviceID)
	deviceLinkSN = strings.TrimSpace(deviceLinkSN)
	if deviceID == "" || deviceLinkSN == "" {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
UPDATE devices
SET device_link_sn = ?, updated_at = ?
WHERE id = ?`,
		deviceLinkSN,
		time.Now().UTC().Format(time.RFC3339),
		deviceID,
	)
	if err != nil {
		return fmt.Errorf("update device_link_sn: %w", err)
	}
	return nil
}

// ListLocationNodes 返回位置树节点列表。
func (s *Service) ListLocationNodes(ctx context.Context) ([]LocationNode, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at
FROM location_nodes
ORDER BY parent_id ASC, sort_order ASC, node_name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query location nodes: %w", err)
	}
	defer rows.Close()

	nodes := make([]LocationNode, 0)
	for rows.Next() {
		node, err := scanLocationNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hydrateLocationNodePaths(nodes), nil
}

// CreateLocationNode 创建位置树节点，并强约束为 zone -> row -> slot。
func (s *Service) CreateLocationNode(ctx context.Context, req CreateLocationNodeRequest) (LocationNode, error) {
	req.ParentID = strings.TrimSpace(req.ParentID)
	req.NodeType = strings.ToLower(strings.TrimSpace(req.NodeType))
	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName == "" || req.NodeType == "" {
		return LocationNode{}, ErrLocationNodeFieldsRequired
	}
	if req.NodeType != "zone" && req.NodeType != "row" && req.NodeType != "slot" {
		return LocationNode{}, ErrLocationNodeTypeUnsupported
	}
	if req.NodeType == "zone" && req.ParentID != "" {
		return LocationNode{}, ErrLocationNodeHierarchyInvalid
	}
	if req.NodeType != "zone" && req.ParentID == "" {
		return LocationNode{}, ErrLocationNodeParentRequired
	}
	if req.NodeType != "zone" {
		parent, err := s.GetLocationNodeByID(ctx, req.ParentID)
		if err != nil {
			return LocationNode{}, err
		}
		if req.NodeType == "row" && parent.NodeType != "zone" {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
		if req.NodeType == "slot" && parent.NodeType != "row" {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
	}

	parentID := 0
	if req.ParentID != "" {
		parsed, err := strconv.Atoi(req.ParentID)
		if err != nil {
			return LocationNode{}, ErrLocationNodeParentRequired
		}
		parentID = parsed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at)
VALUES (?, ?, ?, 0, ?, ?, ?)`,
		parentID,
		req.NodeType,
		req.NodeName,
		req.SortOrder,
		now,
		now,
	)
	if err != nil {
		return LocationNode{}, fmt.Errorf("insert location node: %w", err)
	}
	insertedID, err := result.LastInsertId()
	if err != nil {
		return LocationNode{}, fmt.Errorf("read inserted location node id: %w", err)
	}
	return s.GetLocationNodeByID(ctx, strconv.FormatInt(insertedID, 10))
}

// GetLocationNodeByID 根据节点 ID 查询位置节点。
func (s *Service) GetLocationNodeByID(ctx context.Context, nodeID string) (LocationNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return LocationNode{}, ErrLocationNodeNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id, parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at
FROM location_nodes
WHERE id = ?`, nodeID)
	node, err := scanLocationNode(row)
	if err != nil {
		return LocationNode{}, err
	}
	return s.hydrateSingleLocationNode(ctx, node)
}

// UpdateLocationNode 更新位置树节点，支持修改父节点、节点名称和排序。
func (s *Service) UpdateLocationNode(ctx context.Context, nodeID string, req UpdateLocationNodeRequest) (LocationNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	req.ParentID = strings.TrimSpace(req.ParentID)
	req.NodeName = strings.TrimSpace(req.NodeName)
	if nodeID == "" {
		return LocationNode{}, ErrLocationNodeNotFound
	}
	if req.NodeName == "" {
		return LocationNode{}, ErrLocationNodeFieldsRequired
	}

	current, err := s.GetLocationNodeByID(ctx, nodeID)
	if err != nil {
		return LocationNode{}, err
	}

	if current.NodeType == "zone" {
		if req.ParentID != "" {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
	} else {
		if req.ParentID == "" {
			return LocationNode{}, ErrLocationNodeParentRequired
		}
		parent, err := s.GetLocationNodeByID(ctx, req.ParentID)
		if err != nil {
			return LocationNode{}, err
		}
		if parent.NodeID == current.NodeID {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
		if current.NodeType == "row" && parent.NodeType != "zone" {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
		if current.NodeType == "slot" && parent.NodeType != "row" {
			return LocationNode{}, ErrLocationNodeHierarchyInvalid
		}
	}

	parentID := 0
	if req.ParentID != "" {
		parsed, err := strconv.Atoi(req.ParentID)
		if err != nil {
			return LocationNode{}, ErrLocationNodeParentRequired
		}
		parentID = parsed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LocationNode{}, fmt.Errorf("begin update location node tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
UPDATE location_nodes
SET parent_id = ?, node_name = ?, sort_order = ?, updated_at = ?
WHERE id = ?`,
		parentID,
		req.NodeName,
		req.SortOrder,
		now,
		nodeID,
	); err != nil {
		return LocationNode{}, fmt.Errorf("update location node: %w", err)
	}

	if err := s.refreshLocationBindingsInTx(ctx, tx, []string{nodeID}, now); err != nil {
		return LocationNode{}, err
	}

	if err := tx.Commit(); err != nil {
		return LocationNode{}, fmt.Errorf("commit update location node tx: %w", err)
	}
	tx = nil

	return s.GetLocationNodeByID(ctx, nodeID)
}

// DeleteLocationNode 删除指定节点及其所有子孙节点，并清理相关设备绑定投影。
func (s *Service) DeleteLocationNode(ctx context.Context, nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return ErrLocationNodeNotFound
	}

	if _, err := s.GetLocationNodeByID(ctx, nodeID); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete location node tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	allNodes, err := s.listLocationNodesRawTx(ctx, tx)
	if err != nil {
		return err
	}

	descendantIDs := collectLocationSubtreeIDs(allNodes, nodeID)
	if len(descendantIDs) == 0 {
		return ErrLocationNodeNotFound
	}

	slotDeviceIDs, err := s.findBoundDeviceIDsByNodeIDsTx(ctx, tx, descendantIDs)
	if err != nil {
		return err
	}
	if len(slotDeviceIDs) > 0 {
		query, args := buildInClause(slotDeviceIDs)
		args = append([]any{now}, args...)
		if _, err := tx.ExecContext(ctx, `
UPDATE devices
SET physical_slot = '', slot_zone_id = '', slot_row_id = '', slot_position_id = '', slot_zone = '', slot_row = '', slot_position = '', bind_status = 'pending', updated_at = ?
WHERE id IN (`+query+`)`,
			args...,
		); err != nil {
			return fmt.Errorf("clear subtree device binding fields: %w", err)
		}
	}

	query, args := buildInClause(descendantIDs)
	if _, err := tx.ExecContext(ctx, `DELETE FROM location_nodes WHERE id IN (`+query+`)`, args...); err != nil {
		return fmt.Errorf("delete location subtree: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete location node tx: %w", err)
	}
	tx = nil
	return nil
}

// BindDeviceToLocationNode 把设备绑定到指定槽位节点。
func (s *Service) BindDeviceToLocationNode(ctx context.Context, nodeID string, req BindLocationNodeRequest) (LocationNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if nodeID == "" {
		return LocationNode{}, ErrLocationNodeNotFound
	}
	if req.DeviceID == "" {
		return LocationNode{}, ErrDeviceNotFound
	}

	node, err := s.GetLocationNodeByID(ctx, nodeID)
	if err != nil {
		return LocationNode{}, err
	}
	if node.NodeType != "slot" {
		return LocationNode{}, ErrLocationNodeHierarchyInvalid
	}
	if node.DeviceID != "" && node.DeviceID != req.DeviceID {
		return LocationNode{}, ErrLocationNodeOccupied
	}
	rowNode, err := s.GetLocationNodeByID(ctx, node.ParentID)
	if err != nil {
		return LocationNode{}, err
	}
	if rowNode.NodeType != "row" {
		return LocationNode{}, ErrLocationNodeHierarchyInvalid
	}
	zoneNode, err := s.GetLocationNodeByID(ctx, rowNode.ParentID)
	if err != nil {
		return LocationNode{}, err
	}
	if zoneNode.NodeType != "zone" {
		return LocationNode{}, ErrLocationNodeHierarchyInvalid
	}

	deviceItem, err := s.GetByID(ctx, req.DeviceID)
	if err != nil {
		return LocationNode{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LocationNode{}, fmt.Errorf("begin bind location node tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
UPDATE location_nodes
SET device_id = 0, updated_at = ?
WHERE node_type = 'slot' AND device_id = ?`,
		now,
		req.DeviceID,
	); err != nil {
		return LocationNode{}, fmt.Errorf("clear previous location node binding: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE location_nodes
SET device_id = ?, updated_at = ?
WHERE id = ?`,
		req.DeviceID,
		now,
		nodeID,
	); err != nil {
		return LocationNode{}, fmt.Errorf("bind location node: %w", err)
	}

	physicalSlot := fmt.Sprintf("%s-%s-%s", node.ZoneName, node.RowName, node.SlotName)
	if _, err := tx.ExecContext(ctx, `
UPDATE devices
SET physical_slot = ?, slot_zone_id = ?, slot_row_id = ?, slot_position_id = ?, slot_zone = ?, slot_row = ?, slot_position = ?, bind_status = 'bound', updated_at = ?
WHERE id = ?`,
		physicalSlot,
		zoneNode.NodeID,
		rowNode.NodeID,
		node.NodeID,
		node.ZoneName,
		node.RowName,
		node.SlotName,
		now,
		deviceItem.DeviceID,
	); err != nil {
		return LocationNode{}, fmt.Errorf("update device binding fields: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return LocationNode{}, fmt.Errorf("commit bind location node tx: %w", err)
	}
	tx = nil

	return s.GetLocationNodeByID(ctx, nodeID)
}

// UnbindLocationNode 清除槽位节点上的设备绑定。
func (s *Service) UnbindLocationNode(ctx context.Context, nodeID string) (LocationNode, error) {
	node, err := s.GetLocationNodeByID(ctx, nodeID)
	if err != nil {
		return LocationNode{}, err
	}
	if node.NodeType != "slot" {
		return LocationNode{}, ErrLocationNodeHierarchyInvalid
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LocationNode{}, fmt.Errorf("begin unbind location node tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if node.DeviceID != "" {
		if _, err := tx.ExecContext(ctx, `
UPDATE devices
SET physical_slot = '', slot_zone_id = '', slot_row_id = '', slot_position_id = '', slot_zone = '', slot_row = '', slot_position = '', bind_status = 'pending', updated_at = ?
WHERE id = ?`,
			now,
			node.DeviceID,
		); err != nil {
			return LocationNode{}, fmt.Errorf("clear device binding fields: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE location_nodes
SET device_id = 0, updated_at = ?
WHERE id = ?`,
		now,
		nodeID,
	); err != nil {
		return LocationNode{}, fmt.Errorf("clear location node binding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return LocationNode{}, fmt.Errorf("commit unbind location node tx: %w", err)
	}
	tx = nil

	return s.GetLocationNodeByID(ctx, nodeID)
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
       slot_zone_id, slot_row_id, slot_position_id, slot_zone, slot_row, slot_position,
       brand, model, android_id, adb_serial, device_link_sn, current_task_id, current_step, last_error,
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

type locationNodeScanner interface {
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
		&d.SlotZoneID, &d.SlotRowID, &d.SlotPositionID, &d.SlotZone, &d.SlotRow, &d.SlotPosition,
		&d.Brand, &d.Model, &d.AndroidID, &d.ADBSerial, &d.DeviceLinkSN, &currentTaskID, &d.CurrentStep, &d.LastError,
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

func scanLocationNode(scanner locationNodeScanner) (LocationNode, error) {
	var (
		node          LocationNode
		nodeID        int64
		parentID      int64
		boundDeviceID int64
	)
	err := scanner.Scan(&nodeID, &parentID, &node.NodeType, &node.NodeName, &boundDeviceID, &node.SortOrder, &node.CreatedAt, &node.UpdatedAt)
	if err == sql.ErrNoRows {
		return LocationNode{}, ErrLocationNodeNotFound
	}
	if err != nil {
		return LocationNode{}, err
	}
	node.NodeID = strconv.FormatInt(nodeID, 10)
	if parentID > 0 {
		node.ParentID = strconv.FormatInt(parentID, 10)
	}
	if boundDeviceID > 0 {
		node.DeviceID = strconv.FormatInt(boundDeviceID, 10)
	}
	return node, nil
}

func hydrateLocationNodePaths(nodes []LocationNode) []LocationNode {
	if len(nodes) == 0 {
		return nodes
	}

	nodeMap := make(map[string]LocationNode, len(nodes))
	for _, item := range nodes {
		nodeMap[item.NodeID] = item
	}

	result := make([]LocationNode, 0, len(nodes))
	for _, item := range nodes {
		result = append(result, hydrateLocationNodePath(item, nodeMap))
	}
	return result
}

func hydrateLocationNodePath(node LocationNode, nodeMap map[string]LocationNode) LocationNode {
	switch node.NodeType {
	case "zone":
		node.ZoneName = node.NodeName
	case "row":
		node.RowName = node.NodeName
		if parent, ok := nodeMap[node.ParentID]; ok {
			node.ZoneName = parent.NodeName
		}
	case "slot":
		node.SlotName = node.NodeName
		if rowNode, ok := nodeMap[node.ParentID]; ok {
			node.RowName = rowNode.NodeName
			if zoneNode, ok := nodeMap[rowNode.ParentID]; ok {
				node.ZoneName = zoneNode.NodeName
			}
		}
	}
	node.PathText = strings.Trim(strings.Join([]string{node.ZoneName, node.RowName, node.SlotName}, "-"), "-")
	return node
}

func (s *Service) listLocationNodesRawTx(ctx context.Context, tx *sql.Tx) ([]LocationNode, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT id, parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at
FROM location_nodes
ORDER BY parent_id ASC, sort_order ASC, node_name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query location nodes tx: %w", err)
	}
	defer rows.Close()

	nodes := make([]LocationNode, 0)
	for rows.Next() {
		node, err := scanLocationNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nodes, nil
}

func collectLocationSubtreeIDs(nodes []LocationNode, rootNodeID string) []string {
	childrenMap := make(map[string][]string, len(nodes))
	for _, item := range nodes {
		childrenMap[item.ParentID] = append(childrenMap[item.ParentID], item.NodeID)
	}

	result := make([]string, 0)
	queue := []string{rootNodeID}
	seen := make(map[string]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, ok := seen[current]; ok {
			continue
		}
		seen[current] = struct{}{}
		result = append(result, current)
		queue = append(queue, childrenMap[current]...)
	}
	return result
}

func (s *Service) findBoundDeviceIDsByNodeIDsTx(ctx context.Context, tx *sql.Tx, nodeIDs []string) ([]string, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	query, args := buildInClause(nodeIDs)
	rows, err := tx.QueryContext(ctx, `
SELECT device_id
FROM location_nodes
WHERE node_type = 'slot' AND device_id > 0 AND id IN (`+query+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("query subtree bound devices: %w", err)
	}
	defer rows.Close()

	deviceIDs := make([]string, 0)
	for rows.Next() {
		var deviceID int64
		if err := rows.Scan(&deviceID); err != nil {
			return nil, fmt.Errorf("scan subtree bound device: %w", err)
		}
		if deviceID > 0 {
			deviceIDs = append(deviceIDs, strconv.FormatInt(deviceID, 10))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subtree bound devices: %w", err)
	}
	return deviceIDs, nil
}

func buildInClause(ids []string) (string, []any) {
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	return strings.Join(placeholders, ","), args
}

func findZoneIDByRowID(nodes []LocationNode, rowID string) string {
	rowID = strings.TrimSpace(rowID)
	if rowID == "" {
		return ""
	}
	for _, item := range nodes {
		if item.NodeID == rowID {
			return strings.TrimSpace(item.ParentID)
		}
	}
	return ""
}

func (s *Service) refreshLocationBindingsInTx(ctx context.Context, tx *sql.Tx, rootNodeIDs []string, now string) error {
	if len(rootNodeIDs) == 0 {
		return nil
	}

	allNodes, err := s.listLocationNodesRawTx(ctx, tx)
	if err != nil {
		return err
	}

	affectedMap := make(map[string]struct{})
	for _, rootNodeID := range rootNodeIDs {
		for _, nodeID := range collectLocationSubtreeIDs(allNodes, rootNodeID) {
			affectedMap[nodeID] = struct{}{}
		}
	}

	affectedNodeIDs := make([]string, 0, len(affectedMap))
	for nodeID := range affectedMap {
		affectedNodeIDs = append(affectedNodeIDs, nodeID)
	}
	if len(affectedNodeIDs) == 0 {
		return nil
	}

	hydratedNodes := hydrateLocationNodePaths(allNodes)
	for _, item := range hydratedNodes {
		if item.NodeType != "slot" || item.DeviceID == "" {
			continue
		}
		if _, ok := affectedMap[item.NodeID]; !ok {
			continue
		}

		physicalSlot := fmt.Sprintf("%s-%s-%s", item.ZoneName, item.RowName, item.SlotName)
		if _, err := tx.ExecContext(ctx, `
UPDATE devices
SET physical_slot = ?, slot_zone_id = ?, slot_row_id = ?, slot_position_id = ?, slot_zone = ?, slot_row = ?, slot_position = ?, bind_status = 'bound', updated_at = ?
WHERE id = ?`,
			physicalSlot,
			findZoneIDByRowID(hydratedNodes, item.ParentID),
			item.ParentID,
			item.NodeID,
			item.ZoneName,
			item.RowName,
			item.SlotName,
			now,
			item.DeviceID,
		); err != nil {
			return fmt.Errorf("refresh location binding fields: %w", err)
		}
	}

	return nil
}

func (s *Service) hydrateSingleLocationNode(ctx context.Context, node LocationNode) (LocationNode, error) {
	nodes, err := s.ListLocationNodes(ctx)
	if err != nil {
		return LocationNode{}, err
	}
	for _, item := range nodes {
		if item.NodeID == node.NodeID {
			return item, nil
		}
	}
	return node, nil
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
    agent_uuid, device_name, physical_slot, group_name, slot_zone_id, slot_row_id, slot_position_id, slot_zone, slot_row, slot_position, status, bind_status, ip,
    brand, model, android_id, adb_serial, device_link_sn, current_task_id, current_step, last_error,
    accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
    last_heartbeat_at, created_at, updated_at
) VALUES (?, ?, '', '', '', '', '', '', '', '', 'offline', 'pending', ?, ?, ?, ?, ?, ?, 0, '', '', 'unknown', 'unknown', 'unknown', '', '', '', ?, ?)`,
		req.AgentUUID,
		req.DeviceName,
		ip,
		req.Brand,
		req.Model,
		req.AndroidID,
		req.ADBSerial,
		req.DeviceLinkSN,
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
SET device_name = ?, ip = ?, brand = ?, model = ?, android_id = ?, adb_serial = ?, device_link_sn = ?, updated_at = ?
WHERE id = ?`,
		req.DeviceName,
		ip,
		req.Brand,
		req.Model,
		req.AndroidID,
		req.ADBSerial,
		req.DeviceLinkSN,
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

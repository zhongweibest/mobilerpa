package plan

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

const (
	// TargetTypeScript 表示计划任务直接调度单脚本版本。
	TargetTypeScript = "script"
	// TargetTypeWorkflow 表示计划任务调度工作流定义。
	TargetTypeWorkflow = "workflow"

	// ScheduleTypeOnce 表示只执行一次。
	ScheduleTypeOnce = "once"
	// ScheduleTypeDaily 表示按自然天循环执行。
	ScheduleTypeDaily = "daily"

	// StatusEnabled 表示计划任务处于启用状态。
	StatusEnabled = "enabled"
	// StatusDisabled 表示计划任务处于停用状态。
	StatusDisabled = "disabled"

	// RunStatusPending 表示计划任务实例已创建但尚未实际下发。
	RunStatusPending = "pending"
	// RunStatusRunning 表示计划任务实例仍在执行中。
	RunStatusRunning = "running"
	// RunStatusSuccess 表示计划任务实例全部完成且无失败。
	RunStatusSuccess = "success"
	// RunStatusFailed 表示计划任务实例中存在失败设备。
	RunStatusFailed = "failed"
	// RunStatusStopped 表示计划任务实例被手工停止。
	RunStatusStopped = "stopped"

	// DeviceRunStatusPending 表示设备尚未被计划任务真正启动。
	DeviceRunStatusPending = "pending"
	// DeviceRunStatusRunning 表示设备正在执行计划任务。
	DeviceRunStatusRunning = "running"
	// DeviceRunStatusSuccess 表示设备执行成功完成。
	DeviceRunStatusSuccess = "success"
	// DeviceRunStatusFailed 表示设备执行失败结束。
	DeviceRunStatusFailed = "failed"
	// DeviceRunStatusStopped 表示设备被手工停止或移除。
	DeviceRunStatusStopped = "stopped"

	// EventTypePlanRunStarted 表示计划任务实例启动。
	EventTypePlanRunStarted = "plan_run_started"
	// EventTypePlanRunCompleted 表示计划任务实例结束。
	EventTypePlanRunCompleted = "plan_run_completed"
	// EventTypePlanRunStopped 表示计划任务实例被手工停止。
	EventTypePlanRunStopped = "plan_run_stopped"
	// EventTypePlanDeviceAdded 表示计划任务实例追加设备。
	EventTypePlanDeviceAdded = "plan_device_added"
	// EventTypePlanDeviceRemoved 表示计划任务实例移除设备。
	EventTypePlanDeviceRemoved = "plan_device_removed"
	// EventTypePlanDeviceStarted 表示某设备开始执行。
	EventTypePlanDeviceStarted = "plan_device_started"
	// EventTypePlanDeviceCompleted 表示某设备执行结束。
	EventTypePlanDeviceCompleted = "plan_device_completed"
)

var (
	// ErrPlanDefinitionNotFound 表示计划任务定义不存在。
	ErrPlanDefinitionNotFound = errors.New("plan definition not found")
	// ErrPlanRunNotFound 表示计划任务实例不存在。
	ErrPlanRunNotFound = errors.New("plan run not found")
	// ErrPlanDeviceRunNotFound 表示计划任务设备运行记录不存在。
	ErrPlanDeviceRunNotFound = errors.New("plan device run not found")
	// ErrPlanRunNotActive 表示计划任务实例不是运行中，不能继续追加或删除设备。
	ErrPlanRunNotActive = errors.New("plan run not active")
	// ErrPlanNameRequired 表示缺少计划任务名称。
	ErrPlanNameRequired = errors.New("plan_name is required")
	// ErrPlanTargetTypeUnsupported 表示不支持的目标类型。
	ErrPlanTargetTypeUnsupported = errors.New("plan target_type is unsupported")
	// ErrPlanScheduleTypeUnsupported 表示不支持的调度类型。
	ErrPlanScheduleTypeUnsupported = errors.New("plan schedule_type is unsupported")
	// ErrPlanTargetScriptNameRequired 表示脚本目标缺少脚本名。
	ErrPlanTargetScriptNameRequired = errors.New("plan target script_name is required")
	// ErrPlanTargetWorkflowDefIDRequired 表示工作流目标缺少工作流定义标识。
	ErrPlanTargetWorkflowDefIDRequired = errors.New("plan target workflow_def_id is required")
	// ErrPlanRowsRequired 表示缺少分区-排绑定。
	ErrPlanRowsRequired = errors.New("plan rows are required")
	// ErrPlanDailyStartTimeInvalid 表示每日开始时间格式不合法。
	ErrPlanDailyStartTimeInvalid = errors.New("plan daily_start_time is invalid")
	// ErrPlanDailyDeadlineTimeInvalid 表示每日截止时间格式不合法。
	ErrPlanDailyDeadlineTimeInvalid = errors.New("plan daily_deadline_time is invalid")
	ErrPlanDefinitionRunning        = errors.New("plan definition has active runs")
	ErrPlanTodayAlreadyStarted      = errors.New("plan today already started")
	ErrPlanRunDeleteNotAllowed      = errors.New("plan run delete not allowed")
)

// DeviceBusyDetail 描述计划任务启动时发现的设备占用情况。
type DeviceBusyDetail struct {
	DeviceID      string `json:"device_id"`
	OccupancyType string `json:"occupancy_type"`
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	Message       string `json:"message"`
}

// DeviceBusyError 表示某批设备中存在被占用的设备。
type DeviceBusyError struct {
	Details []DeviceBusyDetail
}

func (e *DeviceBusyError) Error() string {
	return "plan device busy"
}

// Definition 表示计划任务定义。
type Definition struct {
	PlanDefID           string   `json:"plan_def_id"`
	PlanName            string   `json:"plan_name"`
	Description         string   `json:"description"`
	TargetType          string   `json:"target_type"`
	TargetScriptName    string   `json:"target_script_name"`
	TargetScriptVersion string   `json:"target_script_version"`
	TargetWorkflowDefID string   `json:"target_workflow_def_id"`
	ScheduleType        string   `json:"schedule_type"`
	DailyStartTime      string   `json:"daily_start_time"`
	DailyDeadlineTime   string   `json:"daily_deadline_time"`
	Status              string   `json:"status"`
	Rows                []PlanRowBinding `json:"rows"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

// PlanRowBinding 表示计划任务绑定的分区-排。
type PlanRowBinding struct {
	PlanDefinitionRowID string `json:"plan_definition_row_id"`
	PlanDefID           string `json:"plan_def_id"`
	ZoneID              string `json:"zone_id"`
	RowID               string `json:"row_id"`
	ZoneName            string `json:"zone_name"`
	RowName             string `json:"row_name"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

// Run 表示计划任务实例。
type Run struct {
	PlanRunID   string      `json:"plan_run_id"`
	PlanDefID   string      `json:"plan_def_id"`
	PlanName    string      `json:"plan_name"`
	TargetType  string      `json:"target_type"`
	TargetRefID string      `json:"target_ref_id"`
	RunDate     string      `json:"run_date"`
	Status      string      `json:"status"`
	StartedAt   string      `json:"started_at"`
	FinishedAt  string      `json:"finished_at"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
	DeviceRuns  []DeviceRun `json:"device_runs"`
}

// DeviceRun 表示计划任务实例下单设备运行态。
type DeviceRun struct {
	PlanDeviceRunID string `json:"plan_device_run_id"`
	PlanRunID       string `json:"plan_run_id"`
	PlanDefID       string `json:"plan_def_id"`
	ZoneID          string `json:"zone_id"`
	RowID           string `json:"row_id"`
	SlotID          string `json:"slot_id"`
	DeviceID        string `json:"device_id"`
	TargetType      string `json:"target_type"`
	TargetRefID     string `json:"target_ref_id"`
	CurrentNodeID   string `json:"current_node_id"`
	Status          string `json:"status"`
	NextRetryAt     string `json:"next_retry_at"`
	StartedAt       string `json:"started_at"`
	FinishedAt      string `json:"finished_at"`
	LastError       string `json:"last_error"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// Event 表示计划任务事件。
type Event struct {
	PlanEventID int64          `json:"plan_event_id"`
	PlanRunID   string         `json:"plan_run_id"`
	PlanDefID   string         `json:"plan_def_id"`
	DeviceID    string         `json:"device_id"`
	EventType   string         `json:"event_type"`
	Message     string         `json:"message"`
	Extra       map[string]any `json:"extra"`
	CreatedAt   string         `json:"created_at"`
}

// CreateDefinitionRequest 描述创建计划任务定义时的请求体。
type CreateDefinitionRequest struct {
	PlanName            string   `json:"plan_name"`
	Description         string   `json:"description"`
	TargetType          string   `json:"target_type"`
	TargetScriptName    string   `json:"target_script_name"`
	TargetScriptVersion string   `json:"target_script_version"`
	TargetWorkflowDefID string   `json:"target_workflow_def_id"`
	ScheduleType        string   `json:"schedule_type"`
	DailyStartTime      string   `json:"daily_start_time"`
	DailyDeadlineTime   string   `json:"daily_deadline_time"`
	Status              string   `json:"status"`
	Rows                []PlanRowBinding `json:"rows"`
}

type UpdateDefinitionRowsRequest struct {
	Rows []PlanRowBinding `json:"rows"`
}

// StartRequest 描述启动计划任务时的请求。
type StartRequest struct {
	Manual bool `json:"-"`
}

// AddRowsRequest 描述追加分区-排时的请求。
type AddRowsRequest struct {
	Rows []PlanRowBinding `json:"rows"`
}

// TaskCreator 定义计划任务调度单脚本时需要的最小任务创建能力。
type TaskCreator interface {
	Create(ctx context.Context, req task.CreateRequest) (task.Task, error)
}

// TaskDispatcher 定义计划任务下发单脚本任务时需要的能力。
type TaskDispatcher interface {
	AssignTask(ctx context.Context, taskID string) (task.Task, error)
	StartWorkflowSession(ctx context.Context, payload protocol.StartWorkflowSessionPayload) error
	StopWorkflowSession(ctx context.Context, payload protocol.StopWorkflowSessionPayload) error
	HasDeviceConnection(deviceID string) bool
}

// WorkflowRunner 定义计划任务调度工作流时依赖的最小工作流能力。
type WorkflowRunner interface {
	GetDefinition(ctx context.Context, workflowDefID string) (workflow.Definition, error)
}

// Service 负责计划任务定义、实例与统一调度外壳。
type Service struct {
	db          *sql.DB
	devices     *device.Service
	tasks       TaskCreator
	dispatcher  TaskDispatcher
	workflows   WorkflowRunner
	startFanout int
	startMu     sync.Mutex
	starting    map[string]struct{}
}

// NewService 创建计划任务服务。
func NewService(db *sql.DB, devices *device.Service, tasks TaskCreator, dispatcher TaskDispatcher, workflows WorkflowRunner) *Service {
	return &Service{
		db:          db,
		devices:     devices,
		tasks:       tasks,
		dispatcher:  dispatcher,
		workflows:   workflows,
		startFanout: 20,
		starting:    make(map[string]struct{}),
	}
}

func (s *Service) SetStartFanout(value int) {
	if value <= 0 {
		return
	}
	s.startFanout = value
}

func uniquePlanRows(rows []PlanRowBinding) []PlanRowBinding {
	result := make([]PlanRowBinding, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, item := range rows {
		item.ZoneID = strings.TrimSpace(item.ZoneID)
		item.RowID = strings.TrimSpace(item.RowID)
		item.ZoneName = strings.TrimSpace(item.ZoneName)
		item.RowName = strings.TrimSpace(item.RowName)
		if item.ZoneID == "" || item.RowID == "" {
			continue
		}
		key := item.ZoneID + ":" + item.RowID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func normalizePlanRows(rows []PlanRowBinding) []PlanRowBinding {
	cleaned := make([]PlanRowBinding, 0, len(rows))
	for _, item := range rows {
		item.ZoneID = strings.TrimSpace(item.ZoneID)
		item.RowID = strings.TrimSpace(item.RowID)
		item.ZoneName = strings.TrimSpace(item.ZoneName)
		item.RowName = strings.TrimSpace(item.RowName)
		if item.ZoneID == "" || item.RowID == "" {
			continue
		}
		cleaned = append(cleaned, item)
	}
	return uniquePlanRows(cleaned)
}

type locationNodeInfo struct {
	NodeID    string
	ParentID  string
	NodeType  string
	NodeName  string
	DeviceID  string
	SortOrder int
}

func (s *Service) listLocationNodes(ctx context.Context) ([]locationNodeInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, parent_id, node_type, node_name, device_id, sort_order
FROM location_nodes
ORDER BY parent_id ASC, sort_order ASC, node_name ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query location nodes: %w", err)
	}
	defer rows.Close()

	items := make([]locationNodeInfo, 0)
	for rows.Next() {
		var item locationNodeInfo
		var nodeID int64
		var parentID int64
		var deviceID int64
		if err := rows.Scan(&nodeID, &parentID, &item.NodeType, &item.NodeName, &deviceID, &item.SortOrder); err != nil {
			return nil, fmt.Errorf("scan location node: %w", err)
		}
		item.NodeID = strconv.FormatInt(nodeID, 10)
		item.ParentID = strconv.FormatInt(parentID, 10)
		item.DeviceID = strconv.FormatInt(deviceID, 10)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate location nodes: %w", err)
	}
	return items, nil
}

type planDeviceTarget struct {
	ZoneID     string
	RowID      string
	SlotID     string
	DeviceID   string
	DeviceName string
}

func (s *Service) expandRowsToTargets(ctx context.Context, rows []PlanRowBinding) ([]planDeviceTarget, error) {
	nodes, err := s.listLocationNodes(ctx)
	if err != nil {
		return nil, err
	}

	children := make(map[string][]locationNodeInfo)
	byID := make(map[string]locationNodeInfo)
	for _, node := range nodes {
		byID[node.NodeID] = node
		children[node.ParentID] = append(children[node.ParentID], node)
	}

	sortedChildren := func(parentID string) []locationNodeInfo {
		items := append([]locationNodeInfo(nil), children[parentID]...)
		sort.Slice(items, func(i, j int) bool {
			if items[i].SortOrder != items[j].SortOrder {
				return items[i].SortOrder < items[j].SortOrder
			}
			if items[i].NodeName != items[j].NodeName {
				return items[i].NodeName < items[j].NodeName
			}
			return items[i].NodeID < items[j].NodeID
		})
		return items
	}

	targets := make([]planDeviceTarget, 0)
	for _, binding := range rows {
		for _, rowNode := range sortedChildren(binding.ZoneID) {
			if rowNode.NodeType != "row" || rowNode.NodeID != binding.RowID {
				continue
			}
			for _, slotNode := range sortedChildren(rowNode.NodeID) {
				if slotNode.NodeType != "slot" {
					continue
				}
				if strings.TrimSpace(slotNode.DeviceID) == "" || slotNode.DeviceID == "0" {
					continue
				}
				targets = append(targets, planDeviceTarget{
					ZoneID:   binding.ZoneID,
					RowID:    binding.RowID,
					SlotID:   slotNode.NodeID,
					DeviceID: slotNode.DeviceID,
				})
			}
		}
	}
	return targets, nil
}

func (s *Service) listPlanRowsByPlan(ctx context.Context) (map[string][]PlanRowBinding, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, plan_def_id, zone_id, row_id, created_at, updated_at
FROM plan_definition_rows
ORDER BY plan_def_id ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query plan definition rows: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]PlanRowBinding)
	for rows.Next() {
		var item PlanRowBinding
		var planDefID string
		if err := rows.Scan(&item.PlanDefinitionRowID, &planDefID, &item.ZoneID, &item.RowID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan plan definition row: %w", err)
		}
		item.PlanDefID = planDefID
		result[planDefID] = append(result[planDefID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan definition rows: %w", err)
	}
	return result, nil
}

// CreateDefinition 创建计划任务定义。
func (s *Service) CreateDefinition(ctx context.Context, req CreateDefinitionRequest) (Definition, error) {
	req = normalizeCreateDefinitionRequest(req)
	if err := validateDefinitionRequest(req); err != nil {
		return Definition{}, err
	}

	cleanRows := uniquePlanRows(req.Rows)
	if len(cleanRows) == 0 {
		return Definition{}, ErrPlanRowsRequired
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, fmt.Errorf("begin plan definition tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO plan_defs (
    plan_name, description, target_type, target_script_name, target_script_version,
    target_workflow_def_id, schedule_type, daily_start_time, daily_deadline_time, status, created_at, updated_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		req.PlanName,
		req.Description,
		req.TargetType,
		req.TargetScriptName,
		req.TargetScriptVersion,
		req.TargetWorkflowDefID,
		req.ScheduleType,
		req.DailyStartTime,
		req.DailyDeadlineTime,
		req.Status,
		now,
		now,
	)
	if err != nil {
		return Definition{}, fmt.Errorf("insert plan definition: %w", err)
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return Definition{}, fmt.Errorf("read inserted plan definition id: %w", err)
	}
	planDefID := strconv.FormatInt(insertedID, 10)

	for _, rowBinding := range cleanRows {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_definition_rows (
    plan_def_id, zone_id, row_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?)`,
			planDefID,
			rowBinding.ZoneID,
			rowBinding.RowID,
			now,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert plan row binding %s-%s: %w", rowBinding.ZoneID, rowBinding.RowID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Definition{}, fmt.Errorf("commit plan definition tx: %w", err)
	}
	tx = nil

	return s.GetDefinition(ctx, planDefID)
}

// ListDefinitions 返回计划任务定义列表。
func (s *Service) ListDefinitions(ctx context.Context) ([]Definition, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_def_id, plan_name, description, target_type, target_script_name, target_script_version,
       target_workflow_def_id, schedule_type, daily_start_time, daily_deadline_time, status, created_at, updated_at
FROM plan_defs
WHERE deleted_at = ''
ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query plan definitions: %w", err)
	}
	defer rows.Close()

	items := make([]Definition, 0)
	for rows.Next() {
		var item Definition
		if err := rows.Scan(
			&item.PlanDefID,
			&item.PlanName,
			&item.Description,
			&item.TargetType,
			&item.TargetScriptName,
			&item.TargetScriptVersion,
			&item.TargetWorkflowDefID,
			&item.ScheduleType,
			&item.DailyStartTime,
			&item.DailyDeadlineTime,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan plan definition: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan definitions: %w", err)
	}

	rowsByPlan, err := s.listPlanRowsByPlan(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Rows = rowsByPlan[items[index].PlanDefID]
	}
	return items, nil
}

// GetDefinition 返回单个计划任务定义详情。
func (s *Service) GetDefinition(ctx context.Context, planDefID string) (Definition, error) {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return Definition{}, ErrPlanDefinitionNotFound
	}

	var item Definition
	row := s.db.QueryRowContext(ctx, `
SELECT id AS plan_def_id, plan_name, description, target_type, target_script_name, target_script_version,
       target_workflow_def_id, schedule_type, daily_start_time, daily_deadline_time, status, created_at, updated_at
FROM plan_defs
WHERE id = ?
  AND deleted_at = ''`, planDefID)
	if err := row.Scan(
		&item.PlanDefID,
		&item.PlanName,
		&item.Description,
		&item.TargetType,
		&item.TargetScriptName,
		&item.TargetScriptVersion,
		&item.TargetWorkflowDefID,
		&item.ScheduleType,
		&item.DailyStartTime,
		&item.DailyDeadlineTime,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Definition{}, ErrPlanDefinitionNotFound
		}
		return Definition{}, fmt.Errorf("get plan definition: %w", err)
	}

	rowsByPlan, err := s.listPlanRowsByPlan(ctx)
	if err != nil {
		return Definition{}, err
	}
	item.Rows = rowsByPlan[item.PlanDefID]
	return item, nil
}

// Start 使用计划任务定义创建新的计划任务实例。
func (s *Service) Start(ctx context.Context, planDefID string, req StartRequest) (Run, error) {
	definition, err := s.GetDefinition(ctx, planDefID)
	if err != nil {
		return Run{}, err
	}

	now := time.Now()
	if req.Manual && !isManualStartAllowed(definition, now) {
		return Run{}, ErrPlanTodayAlreadyStarted
	}

	rowBindings := definition.Rows
	if len(rowBindings) == 0 {
		return Run{}, ErrPlanRowsRequired
	}

	return s.startPlanRunWithRows(ctx, definition, rowBindings)
}

func (s *Service) startPlanRunWithRows(ctx context.Context, definition Definition, rowBindings []PlanRowBinding) (Run, error) {
	targets, err := s.expandRowsToTargets(ctx, rowBindings)
	if err != nil {
		return Run{}, err
	}
	if len(targets) == 0 {
		return Run{}, ErrPlanRowsRequired
	}
	deviceIDs := make([]string, 0, len(targets))
	seenDeviceIDs := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if _, exists := seenDeviceIDs[target.DeviceID]; exists {
			continue
		}
		seenDeviceIDs[target.DeviceID] = struct{}{}
		deviceIDs = append(deviceIDs, target.DeviceID)
	}
	busyDetails, err := s.collectBusyDevices(ctx, definition.TargetType, "", deviceIDs)
	if err != nil {
		return Run{}, err
	}
	if len(busyDetails) > 0 {
		return Run{}, &DeviceBusyError{Details: busyDetails}
	}

	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	runDate := now.In(time.Local).Format("2006-01-02")
	targetRef := ""
	switch definition.TargetType {
	case TargetTypeScript:
		targetRef = scriptTargetRef(definition)
	case TargetTypeWorkflow:
		targetRef = definition.TargetWorkflowDefID
	default:
		return Run{}, ErrPlanTargetTypeUnsupported
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, fmt.Errorf("begin plan run tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO plan_runs (
    plan_def_id, target_ref_id, run_date, target_type, status, started_at, finished_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		definition.PlanDefID,
		targetRef,
		runDate,
		definition.TargetType,
		RunStatusRunning,
		nowText,
		nowText,
		nowText,
	)
	if err != nil {
		return Run{}, fmt.Errorf("insert plan run: %w", err)
	}
	insertedPlanRunID, err := result.LastInsertId()
	if err != nil {
		return Run{}, fmt.Errorf("read inserted plan run id: %w", err)
	}
	planRunID := strconv.FormatInt(insertedPlanRunID, 10)

	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id,
    status, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', '', ?, ?)`,
			planRunID,
			definition.PlanDefID,
			target.ZoneID,
			target.RowID,
			target.SlotID,
			target.DeviceID,
			definition.TargetType,
			targetRef,
			DeviceRunStatusPending,
			nowText,
			nowText,
		); err != nil {
			return Run{}, fmt.Errorf("insert plan device run: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Run{}, fmt.Errorf("commit plan run tx: %w", err)
	}
	tx = nil

	if err := s.appendEvent(ctx, planRunID, definition.PlanDefID, "", EventTypePlanRunStarted, "计划任务实例已启动", map[string]any{
		"source":      "center",
		"target_type": definition.TargetType,
		"target_ref":  targetRef,
	}); err != nil {
		return Run{}, err
	}

	switch definition.TargetType {
	case TargetTypeScript:
		if err := s.startScriptPlanTargets(ctx, definition, Run{PlanRunID: planRunID, PlanDefID: definition.PlanDefID}, targets); err != nil {
			return Run{}, err
		}
	case TargetTypeWorkflow:
		if err := s.startWorkflowPlanTargets(ctx, definition, Run{PlanRunID: planRunID, PlanDefID: definition.PlanDefID}, targets); err != nil {
			return Run{}, err
		}
	default:
		return Run{}, ErrPlanTargetTypeUnsupported
	}

	return s.GetRun(ctx, planRunID)
}

// StartDueDefinitions 按计划任务定义扫描并自动启动当日应执行但尚未启动的 daily 计划任务。
func (s *Service) StartDueDefinitions(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_def_id
FROM plan_defs
WHERE status = ?
  AND schedule_type = ?
  AND daily_start_time <> ''
  AND deleted_at = ''
ORDER BY id ASC`,
		StatusEnabled,
		ScheduleTypeDaily,
	)
	if err != nil {
		return nil, fmt.Errorf("query due plan definitions: %w", err)
	}
	defer rows.Close()

	planDefIDs := make([]string, 0)
	for rows.Next() {
		var planDefID string
		if err := rows.Scan(&planDefID); err != nil {
			return nil, fmt.Errorf("scan due plan definition: %w", err)
		}
		planDefIDs = append(planDefIDs, planDefID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due plan definitions: %w", err)
	}

	startedPlanRunIDs := make([]string, 0)
	for _, planDefID := range planDefIDs {
		if !s.tryAcquireStarting(planDefID) {
			continue
		}

		definition, err := s.GetDefinition(ctx, planDefID)
		if err != nil {
			s.releaseStarting(planDefID)
			return nil, err
		}
		if !isDailyStartReached(definition.DailyStartTime, now) {
			s.releaseStarting(planDefID)
			continue
		}
		if isDailyDeadlineReached(definition.DailyDeadlineTime, now) {
			s.releaseStarting(planDefID)
			continue
		}

		runDate := now.In(time.Local).Format("2006-01-02")
		exists, err := s.hasRunOnDate(ctx, planDefID, runDate)
		if err != nil {
			s.releaseStarting(planDefID)
			return nil, err
		}
		if exists {
			s.releaseStarting(planDefID)
			continue
		}

		run, err := s.Start(ctx, planDefID, StartRequest{})
		s.releaseStarting(planDefID)
		if err != nil {
			var busyErr *DeviceBusyError
			if errors.As(err, &busyErr) {
				continue
			}
			if errors.Is(err, device.ErrDeviceAccessibilityRequired) || errors.Is(err, device.ErrDeviceForegroundServiceRequired) || errors.Is(err, device.ErrDeviceBatteryOptimizationRequired) || errors.Is(err, device.ErrDeviceExecutionProfileUnknown) {
				continue
			}
			return nil, fmt.Errorf("auto start plan %s: %w", planDefID, err)
		}
		startedPlanRunIDs = append(startedPlanRunIDs, run.PlanRunID)
	}
	return startedPlanRunIDs, nil
}

// StopExpiredRuns 停止已命中每日截止时间的计划任务实例。
func (s *Service) StopExpiredRuns(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id AS plan_run_id, d.daily_deadline_time
FROM plan_runs r
JOIN plan_defs d
  ON d.id = r.plan_def_id
WHERE r.status IN (?, ?)
  AND d.schedule_type = ?
  AND d.deleted_at = ''
  AND d.daily_deadline_time <> ''
ORDER BY r.id ASC`,
		RunStatusPending,
		RunStatusRunning,
		ScheduleTypeDaily,
	)
	if err != nil {
		return nil, fmt.Errorf("query expired plan runs: %w", err)
	}
	defer rows.Close()

	type expiredItem struct {
		planRunID         string
		dailyDeadlineTime string
	}
	items := make([]expiredItem, 0)
	for rows.Next() {
		var item expiredItem
		if err := rows.Scan(&item.planRunID, &item.dailyDeadlineTime); err != nil {
			return nil, fmt.Errorf("scan expired plan run: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired plan runs: %w", err)
	}

	stoppedPlanRunIDs := make([]string, 0)
	for _, item := range items {
		if !isDailyDeadlineReached(item.dailyDeadlineTime, now) {
			continue
		}
		run, err := s.StopRun(ctx, item.planRunID)
		if err != nil {
			return nil, fmt.Errorf("stop expired plan run %s: %w", item.planRunID, err)
		}
		stoppedPlanRunIDs = append(stoppedPlanRunIDs, run.PlanRunID)
	}
	return stoppedPlanRunIDs, nil
}

func (s *Service) ListDueDefinitionIDs(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_def_id
FROM plan_defs
WHERE status = ?
  AND schedule_type = ?
  AND daily_start_time <> ''
  AND deleted_at = ''
ORDER BY id ASC`,
		StatusEnabled,
		ScheduleTypeDaily,
	)
	if err != nil {
		return nil, fmt.Errorf("query due plan definitions: %w", err)
	}
	defer rows.Close()

	planDefIDs := make([]string, 0)
	for rows.Next() {
		var planDefID string
		if err := rows.Scan(&planDefID); err != nil {
			return nil, fmt.Errorf("scan due plan definition: %w", err)
		}
		planDefIDs = append(planDefIDs, planDefID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due plan definitions: %w", err)
	}

	result := make([]string, 0, len(planDefIDs))
	for _, planDefID := range planDefIDs {
		definition, err := s.GetDefinition(ctx, planDefID)
		if err != nil {
			return nil, err
		}
		if !isDailyStartReached(definition.DailyStartTime, now) {
			continue
		}
		if isDailyDeadlineReached(definition.DailyDeadlineTime, now) {
			continue
		}

		runDate := now.In(time.Local).Format("2006-01-02")
		exists, err := s.hasRunOnDate(ctx, planDefID, runDate)
		if err != nil {
			return nil, err
		}
		if exists {
			continue
		}
		result = append(result, planDefID)
	}
	return result, nil
}

func (s *Service) AutoStartDefinition(ctx context.Context, planDefID string, now time.Time) (string, error) {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return "", ErrPlanDefinitionNotFound
	}
	if !s.tryAcquireStarting(planDefID) {
		return "", nil
	}
	defer s.releaseStarting(planDefID)

	definition, err := s.GetDefinition(ctx, planDefID)
	if err != nil {
		return "", err
	}
	if definition.Status != StatusEnabled || definition.ScheduleType != ScheduleTypeDaily {
		return "", nil
	}
	if !isDailyStartReached(definition.DailyStartTime, now) {
		return "", nil
	}
	if isDailyDeadlineReached(definition.DailyDeadlineTime, now) {
		return "", nil
	}

	runDate := now.In(time.Local).Format("2006-01-02")
	exists, err := s.hasRunOnDate(ctx, planDefID, runDate)
	if err != nil {
		return "", err
	}
	if exists {
		return "", nil
	}

	run, err := s.Start(ctx, planDefID, StartRequest{})
	if err != nil {
		var busyErr *DeviceBusyError
		if errors.As(err, &busyErr) {
			return "", nil
		}
		if errors.Is(err, device.ErrDeviceAccessibilityRequired) || errors.Is(err, device.ErrDeviceForegroundServiceRequired) || errors.Is(err, device.ErrDeviceBatteryOptimizationRequired) || errors.Is(err, device.ErrDeviceExecutionProfileUnknown) {
			return "", nil
		}
		return "", fmt.Errorf("auto start plan %s: %w", planDefID, err)
	}
	return run.PlanRunID, nil
}

func (s *Service) ListExpiredRunIDs(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id AS plan_run_id, d.daily_deadline_time
FROM plan_runs r
JOIN plan_defs d
  ON d.id = r.plan_def_id
WHERE r.status IN (?, ?)
  AND d.schedule_type = ?
  AND d.deleted_at = ''
  AND d.daily_deadline_time <> ''
ORDER BY r.id ASC`,
		RunStatusPending,
		RunStatusRunning,
		ScheduleTypeDaily,
	)
	if err != nil {
		return nil, fmt.Errorf("query expired plan runs: %w", err)
	}
	defer rows.Close()

	result := make([]string, 0)
	for rows.Next() {
		var planRunID string
		var deadline string
		if err := rows.Scan(&planRunID, &deadline); err != nil {
			return nil, fmt.Errorf("scan expired plan run: %w", err)
		}
		if !isDailyDeadlineReached(deadline, now) {
			continue
		}
		result = append(result, planRunID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired plan runs: %w", err)
	}
	return result, nil
}

func (s *Service) AutoStopRun(ctx context.Context, planRunID string) (string, error) {
	run, err := s.StopRun(ctx, planRunID)
	if err != nil {
		return "", fmt.Errorf("stop expired plan run %s: %w", planRunID, err)
	}
	return run.PlanRunID, nil
}

// RetryDueTargets 重新尝试已到重试时间的计划任务设备。
func (s *Service) RetryDueTargets(ctx context.Context, now time.Time, retryInterval time.Duration) ([]string, error) {
	targets, err := s.listRetryableTargets(ctx, now)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}
	if retryInterval <= 0 {
		retryInterval = 5 * time.Minute
	}

	nowText := now.UTC().Format(time.RFC3339)
	affectedRuns := make(map[string]struct{})
	processedRuns := make([]string, 0)

	for _, target := range targets {
		run, err := s.GetRun(ctx, target.PlanRunID)
		if err != nil {
			return nil, err
		}
		definition, err := s.GetDefinition(ctx, run.PlanDefID)
		if err != nil {
			return nil, err
		}
		if definition.Status != StatusEnabled {
			continue
		}
		if definition.ScheduleType != ScheduleTypeDaily {
			if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET next_retry_at = '', updated_at = ? WHERE id = ?`, nowText, target.PlanDeviceRunID); err != nil {
				return nil, fmt.Errorf("clear non-daily retry target: %w", err)
			}
			continue
		}
		if !isRetryWindowOpen(definition.DailyDeadlineTime, now) {
			if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET next_retry_at = '', updated_at = ? WHERE id = ?`, nowText, target.PlanDeviceRunID); err != nil {
				return nil, fmt.Errorf("clear expired retry target: %w", err)
			}
			continue
		}

		deviceRun := target
		if err := s.retryPlanDeviceTarget(ctx, definition, run, deviceRun, nowText, retryInterval); err != nil {
			return nil, err
		}
		affectedRuns[run.PlanRunID] = struct{}{}
	}

	for planRunID := range affectedRuns {
		if err := s.refreshRunStatus(ctx, planRunID); err != nil {
			return nil, err
		}
		processedRuns = append(processedRuns, planRunID)
	}

	return processedRuns, nil
}

func isRetryWindowOpen(deadlineTime string, now time.Time) bool {
	deadlineTime = strings.TrimSpace(deadlineTime)
	if deadlineTime == "" {
		return true
	}
	parsed, err := time.Parse("15:04:05", deadlineTime)
	if err != nil {
		return false
	}
	localNow := now.In(time.Local)
	deadlineAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	return localNow.Before(deadlineAt.Add(-30 * time.Minute))
}

func (s *Service) retryPlanDeviceTarget(ctx context.Context, definition Definition, run Run, deviceRun DeviceRun, nowText string, retryInterval time.Duration) error {
	_ = retryInterval
	reachable, err := s.isDeviceReachableForPlanStart(ctx, deviceRun.DeviceID)
	if err != nil {
		return err
	}
	if !reachable {
		return s.deferPlanDeviceStart(ctx, definition, run, deviceRun, nowText, true)
	}
	if s.devices == nil {
		return fmt.Errorf("device service is not configured")
	}

	if err := s.devices.EnsureExecutionReady(ctx, deviceRun.DeviceID); err != nil {
		return err
	}

	switch definition.TargetType {
	case TargetTypeScript:
		createdTask, err := s.tasks.Create(ctx, task.CreateRequest{DeviceID: deviceRun.DeviceID, ScriptName: definition.TargetScriptName, ScriptVersion: definition.TargetScriptVersion})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET plan_run_id = ?, plan_device_run_id = ?, task_source_type = 'plan_script' WHERE id = ?`, run.PlanRunID, deviceRun.PlanDeviceRunID, createdTask.TaskID); err != nil {
			return err
		}
		if err := s.dispatcherAssign(ctx, createdTask.TaskID); err != nil {
			if errors.Is(err, dispatch.ErrDeviceNotConnected) {
				if cleanupErr := s.deleteTaskRecord(ctx, createdTask.TaskID); cleanupErr != nil {
					return cleanupErr
				}
				return s.deferPlanDeviceStart(ctx, definition, run, deviceRun, nowText, true)
			}
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET status = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, next_retry_at = '', updated_at = ? WHERE id = ?`, DeviceRunStatusRunning, nowText, nowText, deviceRun.PlanDeviceRunID); err != nil {
			return fmt.Errorf("update plan device run retry start: %w", err)
		}
		if err := s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, deviceRun.DeviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{
			"source":             "center",
			"plan_device_run_id": deviceRun.PlanDeviceRunID,
			"task_id":            createdTask.TaskID,
			"script_name":        definition.TargetScriptName,
			"script_version":     definition.TargetScriptVersion,
		}); err != nil {
			return err
		}
		return nil
	case TargetTypeWorkflow:
		workflowDefinition, err := s.workflows.GetDefinition(ctx, definition.TargetWorkflowDefID)
		if err != nil {
			return err
		}
		entryNodeID, err := findWorkflowEntryNodeID(workflowDefinition)
		if err != nil {
			return err
		}
		sessionPayload, err := buildWorkflowSessionPayloadTemplate(workflowDefinition, entryNodeID)
		if err != nil {
			return err
		}
		sessionPayload.WorkflowSessionID = deviceRun.PlanDeviceRunID
		sessionPayload.PlanRunID = run.PlanRunID
		sessionPayload.PlanDeviceRunID = deviceRun.PlanDeviceRunID
		sessionPayload.DeviceID = deviceRun.DeviceID
		sessionPayload.WorkflowDefID = definition.TargetWorkflowDefID
		if err := s.dispatcher.StartWorkflowSession(ctx, sessionPayload); err != nil {
			if errors.Is(err, dispatch.ErrDeviceNotConnected) {
				return s.deferPlanDeviceStart(ctx, definition, run, deviceRun, nowText, true)
			}
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET status = ?, current_node_id = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, next_retry_at = '', updated_at = ? WHERE id = ?`, DeviceRunStatusRunning, entryNodeID, nowText, nowText, deviceRun.PlanDeviceRunID); err != nil {
			return fmt.Errorf("update plan device run retry start: %w", err)
		}
		if err := s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, deviceRun.DeviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{
			"source":             "center",
			"plan_device_run_id": deviceRun.PlanDeviceRunID,
			"workflow_def_id":    definition.TargetWorkflowDefID,
			"workflow_node_id":   entryNodeID,
		}); err != nil {
			return err
		}
		return nil
	default:
		return ErrPlanTargetTypeUnsupported
	}
}

// ListRuns 返回计划任务实例列表。
func (s *Service) ListRuns(ctx context.Context, planDefID string) ([]Run, error) {
	query := `
SELECT r.id AS plan_run_id, r.plan_def_id, COALESCE(d.plan_name, '') AS plan_name, r.target_type, r.target_ref_id, r.run_date, r.status,
       r.started_at, r.finished_at, r.created_at, r.updated_at
FROM plan_runs r
LEFT JOIN plan_defs d
  ON d.id = r.plan_def_id`
	args := make([]any, 0, 1)
	planDefID = strings.TrimSpace(planDefID)
	if planDefID != "" {
		query += `
WHERE r.plan_def_id = ?`
		args = append(args, planDefID)
	}
	query += `
ORDER BY r.id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query plan runs: %w", err)
	}
	defer rows.Close()

	items := make([]Run, 0)
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan runs: %w", err)
	}

	for index := range items {
		deviceRuns, err := s.listDeviceRunsByPlanRun(ctx, items[index].PlanRunID)
		if err != nil {
			return nil, err
		}
		items[index].DeviceRuns = deviceRuns
	}
	return items, nil
}

// GetRun 返回单个计划任务实例详情。
func (s *Service) GetRun(ctx context.Context, planRunID string) (Run, error) {
	planRunID = strings.TrimSpace(planRunID)
	if planRunID == "" {
		return Run{}, ErrPlanRunNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT r.id AS plan_run_id, r.plan_def_id, COALESCE(d.plan_name, '') AS plan_name, r.target_type, r.target_ref_id, r.run_date, r.status,
       r.started_at, r.finished_at, r.created_at, r.updated_at
FROM plan_runs r
LEFT JOIN plan_defs d
  ON d.id = r.plan_def_id
WHERE r.id = ?`, planRunID)
	item, err := scanRun(row)
	if err != nil {
		return Run{}, err
	}
	deviceRuns, err := s.listDeviceRunsByPlanRun(ctx, item.PlanRunID)
	if err != nil {
		return Run{}, err
	}
	item.DeviceRuns = deviceRuns
	return item, nil
}

// AddRows 为运行中的计划任务实例追加分区-排。
func (s *Service) AddRows(ctx context.Context, planRunID string, req AddRowsRequest) (Run, error) {
	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return Run{}, err
	}
	if run.Status != RunStatusPending && run.Status != RunStatusRunning {
		return Run{}, ErrPlanRunNotActive
	}

	definition, err := s.GetDefinition(ctx, run.PlanDefID)
	if err != nil {
		return Run{}, err
	}

	nextRows := normalizePlanRows(req.Rows)
	if len(nextRows) == 0 {
		return Run{}, ErrPlanRowsRequired
	}
	if err := s.appendRunRows(ctx, run.PlanRunID, definition, nextRows); err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, planRunID)
}

// RemoveRow 把某个分区-排从运行中的计划任务实例中移除。
func (s *Service) RemoveRow(ctx context.Context, planRunID string, zoneID string, rowID string) (Run, error) {
	zoneID = strings.TrimSpace(zoneID)
	rowID = strings.TrimSpace(rowID)
	if zoneID == "" || rowID == "" {
		return Run{}, ErrPlanRowsRequired
	}

	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return Run{}, err
	}
	if run.Status != RunStatusPending && run.Status != RunStatusRunning {
		return Run{}, ErrPlanRunNotActive
	}

	targets := make([]DeviceRun, 0)
	for _, item := range run.DeviceRuns {
		if item.ZoneID == zoneID && item.RowID == rowID {
			targets = append(targets, item)
		}
	}
	if len(targets) == 0 {
		return s.GetRun(ctx, planRunID)
	}

	definition, err := s.GetDefinition(ctx, run.PlanDefID)
	if err != nil {
		return Run{}, err
	}

	for _, deviceRun := range targets {
		switch definition.TargetType {
		case TargetTypeScript:
			if deviceRun.Status == DeviceRunStatusPending || deviceRun.Status == DeviceRunStatusRunning {
				if err := s.stopScriptPlanDevice(ctx, definition, run, deviceRun, "row_remove"); err != nil {
					return Run{}, err
				}
			}
		case TargetTypeWorkflow:
			if deviceRun.Status == DeviceRunStatusPending || deviceRun.Status == DeviceRunStatusRunning {
				if err := s.stopWorkflowPlanDevice(ctx, definition, run, deviceRun, "row_remove"); err != nil {
					return Run{}, err
				}
			}
		default:
			return Run{}, ErrPlanTargetTypeUnsupported
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM plan_device_runs WHERE id = ?`, deviceRun.PlanDeviceRunID); err != nil {
			return Run{}, fmt.Errorf("delete plan device run by row: %w", err)
		}
		if err := s.appendEvent(ctx, planRunID, run.PlanDefID, deviceRun.DeviceID, EventTypePlanDeviceRemoved, "整排已从计划任务实例中移除", map[string]any{
			"source":             "center",
			"plan_device_run_id": deviceRun.PlanDeviceRunID,
			"reason":             "row_remove",
			"zone_id":            zoneID,
			"row_id":             rowID,
		}); err != nil {
			return Run{}, err
		}
	}

	if err := s.refreshRunStatus(ctx, planRunID); err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, planRunID)
}

// StopRun 停止整个计划任务实例。
func (s *Service) StopRun(ctx context.Context, planRunID string) (Run, error) {
	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return Run{}, err
	}
	if run.Status != RunStatusPending && run.Status != RunStatusRunning {
		return run, nil
	}

	definition, err := s.GetDefinition(ctx, run.PlanDefID)
	if err != nil {
		return Run{}, err
	}

	for _, item := range run.DeviceRuns {
		if item.Status != DeviceRunStatusPending && item.Status != DeviceRunStatusRunning {
			continue
		}
		switch definition.TargetType {
		case TargetTypeScript:
			if err := s.stopScriptPlanDevice(ctx, definition, run, item, "plan_stop"); err != nil {
				return Run{}, err
			}
		case TargetTypeWorkflow:
			if err := s.stopWorkflowPlanDevice(ctx, definition, run, item, "plan_stop"); err != nil {
				return Run{}, err
			}
		default:
			return Run{}, ErrPlanTargetTypeUnsupported
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_runs
SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE id = ?`,
		RunStatusStopped,
		now,
		now,
		planRunID,
	); err != nil {
		return Run{}, fmt.Errorf("stop plan run: %w", err)
	}

	if err := s.appendEvent(ctx, planRunID, run.PlanDefID, "", EventTypePlanRunStopped, "计划任务实例已停止", map[string]any{
		"source": "center",
	}); err != nil {
		return Run{}, err
	}

	return s.GetRun(ctx, planRunID)
}

func (s *Service) DeleteDefinition(ctx context.Context, planDefID string) error {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return ErrPlanDefinitionNotFound
	}

	if _, err := s.GetDefinition(ctx, planDefID); err != nil {
		return err
	}

	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM plan_runs
WHERE id = ?
  AND status IN (?, ?)`,
		planDefID,
		RunStatusPending,
		RunStatusRunning,
	)

	var activeCount int
	if err := row.Scan(&activeCount); err != nil {
		return fmt.Errorf("count active plan runs: %w", err)
	}
	if activeCount > 0 {
		return ErrPlanDefinitionRunning
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
UPDATE plan_defs
SET deleted_at = ?, updated_at = ?
WHERE id = ?
  AND deleted_at = ''`,
		now,
		now,
		planDefID,
	)
	if err != nil {
		return fmt.Errorf("soft delete plan definition: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("plan definition rows affected: %w", err)
	}
	if affected == 0 {
		return ErrPlanDefinitionNotFound
	}
	return nil
}

func (s *Service) DeleteRun(ctx context.Context, planRunID string) error {
	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return err
	}
	if run.Status == RunStatusPending || run.Status == RunStatusRunning {
		return ErrPlanRunDeleteNotAllowed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete plan run tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM plan_events
WHERE id = ?`, planRunID); err != nil {
		return fmt.Errorf("delete plan events: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM plan_device_runs
WHERE id = ?`, planRunID); err != nil {
		return fmt.Errorf("delete plan device runs: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM plan_runs
WHERE id = ?`, planRunID); err != nil {
		return fmt.Errorf("delete plan run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete plan run tx: %w", err)
	}
	tx = nil
	return nil
}

func (s *Service) UpdateDefinitionRows(ctx context.Context, planDefID string, req UpdateDefinitionRowsRequest) (Definition, error) {
	_, err := s.GetDefinition(ctx, planDefID)
	if err != nil {
		return Definition{}, err
	}

	nextRows := normalizePlanRows(req.Rows)
	if len(nextRows) == 0 {
		return Definition{}, ErrPlanRowsRequired
	}

	if err := s.replaceDefinitionRows(ctx, planDefID, nextRows); err != nil {
		return Definition{}, err
	}

	return s.GetDefinition(ctx, planDefID)
}

func (s *Service) replaceDefinitionRows(ctx context.Context, planDefID string, rows []PlanRowBinding) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace definition rows tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM plan_definition_rows WHERE plan_def_id = ?`, planDefID); err != nil {
		return fmt.Errorf("clear plan definition rows: %w", err)
	}
	for _, row := range rows {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_definition_rows (plan_def_id, zone_id, row_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`,
			planDefID, row.ZoneID, row.RowID, now, now); err != nil {
			return fmt.Errorf("insert plan definition row: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE plan_defs SET updated_at = ? WHERE id = ?`, now, planDefID); err != nil {
		return fmt.Errorf("touch plan definition: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace definition rows tx: %w", err)
	}
	tx = nil
	return nil
}

// ListEvents 返回指定计划任务实例的事件列表。
func (s *Service) ListEvents(ctx context.Context, planRunID string) ([]Event, error) {
	planRunID = strings.TrimSpace(planRunID)
	if planRunID == "" {
		return nil, ErrPlanRunNotFound
	}
	if _, err := s.GetRun(ctx, planRunID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, plan_run_id, plan_def_id, device_id, event_type, message, extra_json, created_at
FROM plan_events
WHERE plan_run_id = ?
ORDER BY id ASC`, planRunID)
	if err != nil {
		return nil, fmt.Errorf("query plan events: %w", err)
	}
	defer rows.Close()

	items := make([]Event, 0)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan events: %w", err)
	}
	return items, nil
}

func (s *Service) appendRunRows(ctx context.Context, planRunID string, definition Definition, rows []PlanRowBinding) error {
	rows = normalizePlanRows(rows)
	if len(rows) == 0 {
		return nil
	}
	existing, err := s.listDeviceRunsByPlanRun(ctx, planRunID)
	if err != nil {
		return err
	}
	existsBySlot := make(map[string]struct{}, len(existing))
	existsByRow := make(map[string]struct{})
	for _, item := range existing {
		if strings.TrimSpace(item.SlotID) != "" {
			existsBySlot[item.SlotID] = struct{}{}
		}
		existsByRow[item.ZoneID+":"+item.RowID] = struct{}{}
	}
	targets, err := s.expandRowsToTargets(ctx, rows)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return ErrPlanRowsRequired
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, target := range targets {
		if _, ok := existsBySlot[target.SlotID]; ok {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id,
    status, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', '', ?, ?)`,
			planRunID,
			definition.PlanDefID,
			target.ZoneID,
			target.RowID,
			target.SlotID,
			target.DeviceID,
			definition.TargetType,
			func() string {
				if definition.TargetType == TargetTypeScript {
					return scriptTargetRef(definition)
				}
				return definition.TargetWorkflowDefID
			}(),
			DeviceRunStatusPending,
			now,
			now,
		); err != nil {
			return fmt.Errorf("insert plan run target: %w", err)
		}
	}
	return nil
}

func (s *Service) startScriptPlanTargets(ctx context.Context, definition Definition, run Run, targets []planDeviceTarget) error {
	if s.tasks == nil || s.dispatcher == nil {
		return fmt.Errorf("task creator or dispatcher is not configured")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	dispatchItems := make([]scriptPlanDispatchItem, 0, len(targets))
	for _, target := range targets {
		deviceRun, err := s.getDeviceRunByPlanAndDevice(ctx, run.PlanRunID, target.DeviceID)
		if err != nil {
			return err
		}
		reachable, err := s.isDeviceReachableForPlanStart(ctx, target.DeviceID)
		if err != nil {
			return err
		}
		if !reachable {
			if err := s.deferPlanDeviceStart(ctx, definition, run, deviceRun, now, false); err != nil {
				return err
			}
			continue
		}
		if s.devices != nil {
			if err := s.devices.EnsureExecutionReady(ctx, target.DeviceID); err != nil {
				return err
			}
		}
		createdTask, err := s.tasks.Create(ctx, task.CreateRequest{DeviceID: target.DeviceID, ScriptName: definition.TargetScriptName, ScriptVersion: definition.TargetScriptVersion})
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE tasks SET plan_run_id = ?, plan_device_run_id = ?, task_source_type = 'plan_script' WHERE id = ?`, run.PlanRunID, deviceRun.PlanDeviceRunID, createdTask.TaskID); err != nil {
			return err
		}
		dispatchItems = append(dispatchItems, scriptPlanDispatchItem{
			taskID:          createdTask.TaskID,
			deviceID:        target.DeviceID,
			planDeviceRunID: deviceRun.PlanDeviceRunID,
		})
	}
	if err := s.dispatchScriptPlanTasks(ctx, definition, run, dispatchItems); err != nil {
		return err
	}
	return s.refreshRunStatus(ctx, run.PlanRunID)
}

func (s *Service) startWorkflowPlanTargets(ctx context.Context, definition Definition, run Run, targets []planDeviceTarget) error {
	if s.workflows == nil || s.dispatcher == nil {
		return fmt.Errorf("workflow runner or dispatcher is not configured")
	}
	workflowDefinition, err := s.workflows.GetDefinition(ctx, definition.TargetWorkflowDefID)
	if err != nil {
		return err
	}
	entryNodeID, err := findWorkflowEntryNodeID(workflowDefinition)
	if err != nil {
		return err
	}
	sessionPayloadTemplate, err := buildWorkflowSessionPayloadTemplate(workflowDefinition, entryNodeID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, target := range targets {
		deviceRun, err := s.getDeviceRunByPlanAndDevice(ctx, run.PlanRunID, target.DeviceID)
		if err != nil {
			return err
		}
		reachable, err := s.isDeviceReachableForPlanStart(ctx, target.DeviceID)
		if err != nil {
			return err
		}
		if !reachable {
			if err := s.deferPlanDeviceStart(ctx, definition, run, deviceRun, now, false); err != nil {
				return err
			}
			continue
		}
		if s.devices != nil {
			if err := s.devices.EnsureExecutionReady(ctx, target.DeviceID); err != nil {
				return err
			}
		}
		payload := sessionPayloadTemplate
		payload.WorkflowSessionID = deviceRun.PlanDeviceRunID
		payload.PlanRunID = run.PlanRunID
		payload.PlanDeviceRunID = deviceRun.PlanDeviceRunID
		payload.DeviceID = target.DeviceID
		payload.WorkflowDefID = definition.TargetWorkflowDefID
		if err := s.dispatcher.StartWorkflowSession(ctx, payload); err != nil {
			if errors.Is(err, dispatch.ErrDeviceNotConnected) {
				if err := s.deferPlanDeviceStart(ctx, definition, run, deviceRun, now, false); err != nil {
					return err
				}
				continue
			}
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET status = ?, current_node_id = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, updated_at = ? WHERE id = ?`, DeviceRunStatusRunning, entryNodeID, now, now, deviceRun.PlanDeviceRunID); err != nil {
			return err
		}
		if err := s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, target.DeviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{"source": "center", "plan_device_run_id": deviceRun.PlanDeviceRunID, "workflow_def_id": definition.TargetWorkflowDefID, "workflow_node_id": entryNodeID}); err != nil {
			return err
		}
	}
	return s.refreshRunStatus(ctx, run.PlanRunID)
}

func (s *Service) ensureDevicesAvailable(ctx context.Context, targetType string, currentPlanRunID string, deviceIDs []string) ([]DeviceBusyDetail, error) {
	details, err := s.collectBusyDevices(ctx, targetType, currentPlanRunID, deviceIDs)
	if err != nil {
		return nil, err
	}
	for _, deviceID := range deviceIDs {
		reachable, err := s.isDeviceReachableForPlanStart(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		if !reachable || s.devices == nil {
			continue
		}
		if err := s.devices.EnsureExecutionReady(ctx, deviceID); err != nil {
			return nil, err
		}
	}
	return details, nil
}

func (s *Service) collectBusyDevices(ctx context.Context, targetType string, currentPlanRunID string, deviceIDs []string) ([]DeviceBusyDetail, error) {
	details := make([]DeviceBusyDetail, 0)
	for _, deviceID := range deviceIDs {
		detail, err := s.inspectDeviceBusy(ctx, targetType, currentPlanRunID, deviceID)
		if err != nil {
			return nil, err
		}
		if detail != nil {
			details = append(details, *detail)
		}
	}
	return details, nil
}

func (s *Service) stopScriptPlanDevice(ctx context.Context, definition Definition, run Run, deviceRun DeviceRun, reason string) error {
	if _, _, err := s.lookupTaskByPlanDeviceRun(ctx, deviceRun.PlanDeviceRunID); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
	SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
	WHERE id = ?`,
		DeviceRunStatusStopped,
		now,
		now,
		deviceRun.PlanDeviceRunID,
	); err != nil {
		return fmt.Errorf("stop script plan device run: %w", err)
	}

	return nil
}

func (s *Service) stopWorkflowPlanDevice(ctx context.Context, definition Definition, run Run, deviceRun DeviceRun, reason string) error {
	if s.dispatcher == nil {
		return fmt.Errorf("workflow dispatcher is not configured")
	}
	if err := s.dispatcher.StopWorkflowSession(ctx, protocol.StopWorkflowSessionPayload{
		WorkflowSessionID: deviceRun.PlanDeviceRunID,
		PlanRunID:         run.PlanRunID,
		PlanDeviceRunID:   deviceRun.PlanDeviceRunID,
		WorkflowDefID:     definition.TargetWorkflowDefID,
		DeviceID:          deviceRun.DeviceID,
		Reason:            strings.TrimSpace(reason),
		Extra: map[string]any{
			"source": "center",
		},
	}); err != nil {
		return fmt.Errorf("dispatch stop workflow session: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE id = ?`,
		DeviceRunStatusStopped,
		now,
		now,
		deviceRun.PlanDeviceRunID,
	); err != nil {
		return fmt.Errorf("stop workflow plan device run: %w", err)
	}
	return nil
}

// HandleTaskResult 在单脚本计划任务收到任务结果后推进计划任务设备态。
func (s *Service) HandleTaskResult(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id AS task_id, device_id, plan_run_id, plan_device_run_id, status, result_message
FROM tasks
WHERE id = ?`, taskID)

	var (
		dbTaskID        string
		deviceID        string
		planRunID       string
		planDeviceRunID string
		taskStatus      string
		resultMessage   string
	)
	if err := row.Scan(&dbTaskID, &deviceID, &planRunID, &planDeviceRunID, &taskStatus, &resultMessage); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("query task for plan result hook: %w", err)
	}
	if planRunID == "" || planDeviceRunID == "" {
		return nil
	}

	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return err
	}
	definition, err := s.GetDefinition(ctx, run.PlanDefID)
	if err != nil {
		return err
	}
	if definition.TargetType != TargetTypeScript {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	nextStatus := DeviceRunStatusFailed
	runStatus := RunStatusFailed
	if taskStatus == task.StatusSuccess {
		nextStatus = DeviceRunStatusSuccess
		runStatus = RunStatusSuccess
	}

	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, finished_at = ?, last_error = ?, updated_at = ?
WHERE id = ?`,
		nextStatus,
		now,
		condString(nextStatus == DeviceRunStatusFailed, resultMessage, ""),
		now,
		planDeviceRunID,
	); err != nil {
		return fmt.Errorf("update plan device run by task result: %w", err)
	}

	if err := s.appendEvent(ctx, planRunID, definition.PlanDefID, deviceID, EventTypePlanDeviceCompleted, "设备计划任务执行已结束", map[string]any{
		"source":             "center",
		"plan_device_run_id": planDeviceRunID,
		"task_id":            taskID,
		"status":             runStatus,
		"result_message":     resultMessage,
	}); err != nil {
		return err
	}

	return s.refreshRunStatus(ctx, planRunID)
}

// HandleWorkflowSessionAck 处理设备回传的 workflow_session_ack，并直接更新计划任务设备运行态。
func (s *Service) HandleWorkflowSessionAck(ctx context.Context, payload protocol.WorkflowSessionAckPayload, requestID string, deviceID string) error {
	run, deviceRun, err := s.resolveWorkflowSessionDeviceRun(ctx, payload.PlanRunID, payload.PlanDeviceRunID)
	if err != nil {
		return err
	}
	if run.PlanRunID == "" || deviceRun.PlanDeviceRunID == "" {
		return nil
	}

	deviceID = firstNonEmpty(strings.TrimSpace(deviceID), deviceRun.DeviceID)
	status := strings.TrimSpace(payload.Status)
	if status == "" {
		status = "ok"
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "设备已收到工作流会话"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, updated_at = ?
WHERE id = ?`,
		DeviceRunStatusRunning,
		now,
		now,
		deviceRun.PlanDeviceRunID,
	); err != nil {
		return fmt.Errorf("mark plan workflow device run running on session ack: %w", err)
	}

	if err := s.appendEvent(ctx, run.PlanRunID, run.PlanDefID, deviceID, EventTypePlanDeviceStarted, message, map[string]any{
		"source":             "agent",
		"request_id":         requestID,
		"status":             status,
		"plan_device_run_id": deviceRun.PlanDeviceRunID,
	}); err != nil {
		return err
	}
	return s.refreshRunStatus(ctx, run.PlanRunID)
}

// HandleWorkflowSessionEvent 处理设备回传的 workflow_session_event，并直接写入计划任务事件域。
func (s *Service) HandleWorkflowSessionEvent(ctx context.Context, payload protocol.WorkflowSessionEventPayload, requestID string, deviceID string) error {
	run, deviceRun, err := s.resolveWorkflowSessionDeviceRun(ctx, payload.PlanRunID, payload.PlanDeviceRunID)
	if err != nil {
		return err
	}
	if run.PlanRunID == "" || deviceRun.PlanDeviceRunID == "" {
		return nil
	}

	deviceID = firstNonEmpty(strings.TrimSpace(deviceID), deviceRun.DeviceID)
	workflowNodeID := strings.TrimSpace(payload.WorkflowNodeID)
	eventType := strings.TrimSpace(payload.EventType)
	if eventType == "" {
		eventType = "workflow_session_event"
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "工作流会话事件"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if workflowNodeID != "" {
		if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET current_node_id = ?, updated_at = ?
WHERE id = ?`,
			workflowNodeID,
			now,
			deviceRun.PlanDeviceRunID,
		); err != nil {
			return fmt.Errorf("update plan workflow device current node by event: %w", err)
		}
	}

	extra := map[string]any{
		"source":             "agent",
		"request_id":         requestID,
		"status":             strings.TrimSpace(payload.Status),
		"step_name":          strings.TrimSpace(payload.StepName),
		"workflow_node_id":   workflowNodeID,
		"plan_device_run_id": deviceRun.PlanDeviceRunID,
	}
	for key, value := range payload.Extra {
		extra[key] = value
	}
	return s.appendEvent(ctx, run.PlanRunID, run.PlanDefID, deviceID, eventType, message, extra)
}

// HandleWorkflowSessionResult 处理设备回传的 workflow_session_result，并直接更新计划任务设备运行态。
func (s *Service) HandleWorkflowSessionResult(ctx context.Context, payload protocol.WorkflowSessionResultPayload, requestID string, deviceID string) error {
	run, deviceRun, err := s.resolveWorkflowSessionDeviceRun(ctx, payload.PlanRunID, payload.PlanDeviceRunID)
	if err != nil {
		return err
	}
	if run.PlanRunID == "" || deviceRun.PlanDeviceRunID == "" {
		return nil
	}

	deviceID = firstNonEmpty(strings.TrimSpace(deviceID), deviceRun.DeviceID)
	workflowNodeID := strings.TrimSpace(payload.WorkflowNodeID)
	nextStatus := DeviceRunStatusFailed
	switch strings.TrimSpace(payload.Status) {
	case RunStatusSuccess:
		nextStatus = DeviceRunStatusSuccess
	case RunStatusStopped:
		nextStatus = DeviceRunStatusStopped
	}

	lastError := strings.TrimSpace(payload.ResultMessage)
	if nextStatus == DeviceRunStatusSuccess || nextStatus == DeviceRunStatusStopped {
		lastError = ""
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, current_node_id = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END,
    last_error = ?, updated_at = ?
WHERE id = ?`,
		nextStatus,
		workflowNodeID,
		now,
		lastError,
		now,
		deviceRun.PlanDeviceRunID,
	); err != nil {
		return fmt.Errorf("update plan workflow device run by session result: %w", err)
	}

	if err := s.appendEvent(ctx, run.PlanRunID, run.PlanDefID, deviceID, EventTypePlanDeviceCompleted, "设备计划任务执行已结束", map[string]any{
		"source":             "agent",
		"request_id":         requestID,
		"status":             strings.TrimSpace(payload.Status),
		"result_code":        strings.TrimSpace(payload.ResultCode),
		"result_message":     strings.TrimSpace(payload.ResultMessage),
		"workflow_node_id":   workflowNodeID,
		"plan_device_run_id": deviceRun.PlanDeviceRunID,
	}); err != nil {
		return err
	}
	return s.refreshRunStatus(ctx, run.PlanRunID)
}

func (s *Service) refreshRunStatus(ctx context.Context, planRunID string) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT status, finished_at
FROM plan_device_runs
WHERE plan_run_id = ?`, planRunID)
	if err != nil {
		return fmt.Errorf("query plan device runs for status refresh: %w", err)
	}
	defer rows.Close()

	total := 0
	pendingCount := 0
	runningCount := 0
	successCount := 0
	failedCount := 0
	stoppedCount := 0
	lastFinishedAt := ""

	for rows.Next() {
		total += 1
		var status string
		var finishedAt string
		if err := rows.Scan(&status, &finishedAt); err != nil {
			return fmt.Errorf("scan plan device run status: %w", err)
		}
		switch status {
		case DeviceRunStatusPending:
			pendingCount += 1
		case DeviceRunStatusRunning:
			runningCount += 1
		case DeviceRunStatusSuccess:
			successCount += 1
		case DeviceRunStatusFailed:
			failedCount += 1
		case DeviceRunStatusStopped:
			stoppedCount += 1
		}
		if finishedAt > lastFinishedAt {
			lastFinishedAt = finishedAt
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate plan device run statuses: %w", err)
	}
	if total == 0 {
		return nil
	}

	nextStatus := RunStatusRunning
	finishedAt := ""
	switch {
	case pendingCount > 0 || runningCount > 0:
		if pendingCount == total {
			nextStatus = RunStatusPending
		} else {
			nextStatus = RunStatusRunning
		}
	case failedCount > 0:
		nextStatus = RunStatusFailed
		finishedAt = lastFinishedAt
	case successCount == total:
		nextStatus = RunStatusSuccess
		finishedAt = lastFinishedAt
	case stoppedCount == total:
		nextStatus = RunStatusStopped
		finishedAt = lastFinishedAt
	default:
		nextStatus = RunStatusStopped
		finishedAt = lastFinishedAt
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE plan_runs
SET status = ?, finished_at = ?, updated_at = ?
WHERE id = ?`,
		nextStatus,
		finishedAt,
		now,
		planRunID,
	); err != nil {
		return fmt.Errorf("update plan run status: %w", err)
	}

	if nextStatus == RunStatusSuccess || nextStatus == RunStatusFailed || nextStatus == RunStatusStopped {
		run, err := s.GetRun(ctx, planRunID)
		if err != nil {
			return err
		}
		if err := s.appendEvent(ctx, planRunID, run.PlanDefID, "", EventTypePlanRunCompleted, "计划任务实例已结束", map[string]any{
			"source": "center",
			"status": nextStatus,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) syncWorkflowPlanRun(ctx context.Context, planRunID string) error {
	_ = ctx
	_ = planRunID
	return nil
}

func (s *Service) inspectDeviceBusy(ctx context.Context, targetType string, currentPlanRunID string, deviceID string) (*DeviceBusyDetail, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT p.id AS plan_run_id, d.status
FROM plan_device_runs d
JOIN plan_runs p
  ON p.id = d.plan_run_id
WHERE d.device_id = ?
  AND d.status IN (?, ?)
  AND p.id <> ?
ORDER BY p.id DESC
LIMIT 1`,
		deviceID,
		DeviceRunStatusPending,
		DeviceRunStatusRunning,
		currentPlanRunID,
	)

	var (
		planRunID string
		status    string
	)
	if err := row.Scan(&planRunID, &status); err == nil {
		return &DeviceBusyDetail{
			DeviceID:      deviceID,
			OccupancyType: "plan",
			Message:       "设备已被其他计划任务实例占用",
			TaskStatus:    status,
		}, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query busy plan device run: %w", err)
	}

	return nil, nil
}

// GetDeviceBusyDetail 返回某台设备当前是否被计划任务占用。
func (s *Service) GetDeviceBusyDetail(ctx context.Context, deviceID string) (*DeviceBusyDetail, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	return s.inspectDeviceBusy(ctx, "", "", strings.TrimSpace(deviceID))
}

func (s *Service) listDeviceRunsByPlanRun(ctx context.Context, planRunID string) ([]DeviceRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status,
       current_node_id, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
FROM plan_device_runs
WHERE plan_run_id = ?
ORDER BY id ASC`, planRunID)
	if err != nil {
		return nil, fmt.Errorf("query plan device runs: %w", err)
	}
	defer rows.Close()

	items := make([]DeviceRun, 0)
	for rows.Next() {
		item, err := scanDeviceRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan device runs: %w", err)
	}
	return items, nil
}

func (s *Service) getDeviceRunByPlanAndDevice(ctx context.Context, planRunID string, deviceID string) (DeviceRun, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status,
       current_node_id, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
FROM plan_device_runs
WHERE plan_run_id = ?
  AND device_id = ?
ORDER BY id DESC
LIMIT 1`,
		planRunID,
		deviceID,
	)
	return scanDeviceRun(row)
}

func (s *Service) getDeviceRunByID(ctx context.Context, planDeviceRunID string) (DeviceRun, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status,
       current_node_id, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
FROM plan_device_runs
WHERE id = ?`,
		planDeviceRunID,
	)
	return scanDeviceRun(row)
}

func (s *Service) lookupTaskByPlanDeviceRun(ctx context.Context, planDeviceRunID string) (string, string, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS task_id, status
FROM tasks
WHERE plan_device_run_id = ?
ORDER BY id DESC
LIMIT 1`, planDeviceRunID)
	var taskID string
	var status string
	if err := row.Scan(&taskID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", nil
		}
		return "", "", fmt.Errorf("query task by plan_device_run_id: %w", err)
	}
	return taskID, status, nil
}

func (s *Service) resolveWorkflowSessionDeviceRun(ctx context.Context, planRunID string, planDeviceRunID string) (Run, DeviceRun, error) {
	planRunID = strings.TrimSpace(planRunID)
	planDeviceRunID = strings.TrimSpace(planDeviceRunID)

	if planRunID == "" || planDeviceRunID == "" {
		return Run{}, DeviceRun{}, nil
	}

	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		if errors.Is(err, ErrPlanRunNotFound) {
			return Run{}, DeviceRun{}, nil
		}
		return Run{}, DeviceRun{}, err
	}
	deviceRun, err := s.getDeviceRunByID(ctx, planDeviceRunID)
	if err != nil {
		if errors.Is(err, ErrPlanDeviceRunNotFound) {
			return Run{}, DeviceRun{}, nil
		}
		return Run{}, DeviceRun{}, err
	}
	return run, deviceRun, nil
}

func findWorkflowEntryNodeID(definition workflow.Definition) (string, error) {
	if len(definition.Nodes) == 0 {
		return "", fmt.Errorf("workflow definition has no nodes")
	}
	return strings.TrimSpace(definition.Nodes[0].NodeID), nil
}

func buildWorkflowSessionPayloadTemplate(definition workflow.Definition, entryNodeID string) (protocol.StartWorkflowSessionPayload, error) {
	nodeSnapshots := make([]protocol.WorkflowNodeSnapshot, 0, len(definition.Nodes))
	edgeSnapshots := make([]protocol.WorkflowEdgeSnapshot, 0, len(definition.Edges))
	scriptManifest := make([]protocol.WorkflowScriptManifest, 0)
	seenScripts := make(map[string]struct{})

	for _, node := range definition.Nodes {
		nodeSnapshots = append(nodeSnapshots, protocol.WorkflowNodeSnapshot{
			NodeID:        node.NodeID,
			NodeType:      node.NodeType,
			NodeName:      node.NodeName,
			ScriptName:    node.ScriptName,
			ScriptVersion: node.ScriptVersion,
			MaxIterations: node.MaxIterations,
			Position:      node.Position,
		})
		if node.NodeType == workflow.NodeTypeScript {
			key := node.ScriptName + "@" + node.ScriptVersion
			if _, exists := seenScripts[key]; !exists {
				seenScripts[key] = struct{}{}
				scriptManifest = append(scriptManifest, protocol.WorkflowScriptManifest{
					ScriptName:    node.ScriptName,
					ScriptVersion: node.ScriptVersion,
				})
			}
		}
	}

	for _, edge := range definition.Edges {
		edgeSnapshots = append(edgeSnapshots, protocol.WorkflowEdgeSnapshot{
			FromNodeID: edge.FromNodeID,
			ToNodeID:   edge.ToNodeID,
			EdgeType:   edge.EdgeType,
		})
	}

	return protocol.StartWorkflowSessionPayload{
		WorkflowName: definition.WorkflowName,
		EntryNodeID:  entryNodeID,
		DefinitionSnapshot: protocol.WorkflowDefinitionSnapshot{
			Nodes: nodeSnapshots,
			Edges: edgeSnapshots,
		},
		ScriptManifest: scriptManifest,
		RuntimePolicy: map[string]any{
			"event_mode": "key_events",
		},
	}, nil
}

func (s *Service) appendEvent(ctx context.Context, planRunID string, planDefID string, deviceID string, eventType string, message string, extra map[string]any) error {
	if extra == nil {
		extra = map[string]any{}
	}
	// device_id 列是 INTEGER，实例级事件没有具体设备，写入 0 避免存入空串。
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "0"
	}
	body, err := json.Marshal(extra)
	if err != nil {
		return fmt.Errorf("marshal plan event extra: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO plan_events (
    plan_run_id, plan_def_id, device_id, event_type, message, extra_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		planRunID,
		planDefID,
		deviceID,
		eventType,
		message,
		string(body),
		now,
	); err != nil {
		return fmt.Errorf("insert plan event: %w", err)
	}
	return nil
}

func (s *Service) hasRunOnDate(ctx context.Context, planDefID string, runDate string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM plan_runs
WHERE plan_def_id = ?
  AND run_date = ?`,
		planDefID,
		runDate,
	)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("count plan runs by date: %w", err)
	}
	return count > 0, nil
}

func (s *Service) dispatcherAssign(ctx context.Context, taskID string) error {
	if s.dispatcher == nil {
		return fmt.Errorf("task dispatcher is not configured")
	}
	if _, err := s.dispatcher.AssignTask(ctx, taskID); err != nil {
		if errors.Is(err, dispatch.ErrDeviceNotConnected) {
			return err
		}
		return fmt.Errorf("assign plan task: %w", err)
	}
	return nil
}

func (s *Service) dispatchScriptPlanTasks(ctx context.Context, definition Definition, run Run, items []scriptPlanDispatchItem) error {
	if len(items) == 0 {
		return nil
	}

	workerCount := s.startFanout
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(items) {
		workerCount = len(items)
	}

	taskCh := make(chan scriptPlanDispatchItem)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for item := range taskCh {
			if err := s.dispatcherAssign(ctx, item.taskID); err != nil {
				if errors.Is(err, dispatch.ErrDeviceNotConnected) {
					if cleanupErr := s.deleteTaskRecord(ctx, item.taskID); cleanupErr != nil {
						select {
						case errCh <- cleanupErr:
						default:
						}
						continue
					}
					deviceRun, lookupErr := s.getDeviceRunByID(ctx, item.planDeviceRunID)
					if lookupErr != nil {
						select {
						case errCh <- lookupErr:
						default:
						}
						continue
					}
					if deferErr := s.deferPlanDeviceStart(ctx, definition, run, deviceRun, time.Now().UTC().Format(time.RFC3339), false); deferErr != nil {
						select {
						case errCh <- deferErr:
						default:
						}
					}
					continue
				}
				select {
				case errCh <- err:
				default:
				}
				continue
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET status = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, updated_at = ? WHERE id = ?`, DeviceRunStatusRunning, now, now, item.planDeviceRunID); err != nil {
				select {
				case errCh <- err:
				default:
				}
				continue
			}
			if err := s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, item.deviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{
				"source":             "center",
				"plan_device_run_id": item.planDeviceRunID,
				"task_id":            item.taskID,
				"script_name":        definition.TargetScriptName,
				"script_version":     definition.TargetScriptVersion,
			}); err != nil {
				select {
				case errCh <- err:
				default:
				}
			}
		}
	}

	wg.Add(workerCount)
	for index := 0; index < workerCount; index++ {
		go worker()
	}

	for _, item := range items {
		select {
		case <-ctx.Done():
			close(taskCh)
			wg.Wait()
			return ctx.Err()
		case taskCh <- item:
		}
	}
	close(taskCh)
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (s *Service) isDeviceReachableForPlanStart(ctx context.Context, deviceID string) (bool, error) {
	if s.devices == nil {
		return true, nil
	}
	current, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(current.Status) != "online" {
		return false, nil
	}
	if s.dispatcher != nil && !s.dispatcher.HasDeviceConnection(deviceID) {
		return false, nil
	}
	return true, nil
}

func (s *Service) deferPlanDeviceStart(ctx context.Context, definition Definition, run Run, deviceRun DeviceRun, nowText string, includeRetryTimestamp bool) error {
	nextRetryText := ""
	if definition.ScheduleType == ScheduleTypeDaily {
		nextRetryText = nowText
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE plan_device_runs SET next_retry_at = ?, updated_at = ? WHERE id = ?`, nextRetryText, nowText, deviceRun.PlanDeviceRunID); err != nil {
		return fmt.Errorf("update plan device next retry: %w", err)
	}
	message := "设备不在线未启动"
	if includeRetryTimestamp {
		message = nowText + " 设备离线未启动"
	}
	extra := map[string]any{
		"source":             "center",
		"plan_device_run_id": deviceRun.PlanDeviceRunID,
	}
	if nextRetryText != "" {
		extra["retry_at"] = nextRetryText
	}
	return s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, deviceRun.DeviceID, EventTypePlanDeviceStarted, message, extra)
}

func (s *Service) deleteTaskRecord(ctx context.Context, taskID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete undispatched task tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM task_events WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete undispatched task events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, taskID); err != nil {
		return fmt.Errorf("delete undispatched task: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete undispatched task tx: %w", err)
	}
	tx = nil
	return nil
}

func (s *Service) tryAcquireStarting(planDefID string) bool {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return false
	}

	s.startMu.Lock()
	defer s.startMu.Unlock()
	if _, exists := s.starting[planDefID]; exists {
		return false
	}
	s.starting[planDefID] = struct{}{}
	return true
}

func (s *Service) releaseStarting(planDefID string) {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return
	}

	s.startMu.Lock()
	defer s.startMu.Unlock()
	delete(s.starting, planDefID)
}

func normalizeCreateDefinitionRequest(req CreateDefinitionRequest) CreateDefinitionRequest {
	req.PlanName = strings.TrimSpace(req.PlanName)
	req.Description = strings.TrimSpace(req.Description)
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.TargetScriptName = strings.TrimSpace(req.TargetScriptName)
	req.TargetScriptVersion = strings.TrimSpace(req.TargetScriptVersion)
	req.TargetWorkflowDefID = strings.TrimSpace(req.TargetWorkflowDefID)
	req.ScheduleType = strings.TrimSpace(req.ScheduleType)
	req.DailyStartTime = strings.TrimSpace(req.DailyStartTime)
	req.DailyDeadlineTime = strings.TrimSpace(req.DailyDeadlineTime)
	req.Status = strings.TrimSpace(req.Status)
	if req.ScheduleType == "" {
		req.ScheduleType = ScheduleTypeOnce
	}
	if req.Status == "" {
		req.Status = StatusEnabled
	}
	return req
}

func validateDefinitionRequest(req CreateDefinitionRequest) error {
	if req.PlanName == "" {
		return ErrPlanNameRequired
	}
	switch req.TargetType {
	case TargetTypeScript:
		if req.TargetScriptName == "" {
			return ErrPlanTargetScriptNameRequired
		}
	case TargetTypeWorkflow:
		if req.TargetWorkflowDefID == "" {
			return ErrPlanTargetWorkflowDefIDRequired
		}
	default:
		return ErrPlanTargetTypeUnsupported
	}
	switch req.ScheduleType {
	case ScheduleTypeOnce, ScheduleTypeDaily:
	default:
		return ErrPlanScheduleTypeUnsupported
	}
	if !isDailyTimeValid(req.DailyStartTime) {
		return ErrPlanDailyStartTimeInvalid
	}
	if !isDailyTimeValid(req.DailyDeadlineTime) {
		return ErrPlanDailyDeadlineTimeInvalid
	}
	return nil
}

func scriptTargetRef(definition Definition) string {
	return strings.TrimSpace(definition.TargetScriptName) + "@" + strings.TrimSpace(definition.TargetScriptVersion)
}

func mapWorkflowStatus(status string) string {
	switch strings.TrimSpace(status) {
	case RunStatusPending:
		return DeviceRunStatusPending
	case RunStatusRunning:
		return DeviceRunStatusRunning
	case RunStatusSuccess:
		return DeviceRunStatusSuccess
	case RunStatusFailed:
		return DeviceRunStatusFailed
	case RunStatusStopped:
		return DeviceRunStatusStopped
	default:
		return DeviceRunStatusPending
	}
}

func containsDeviceID(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func condString(ok bool, left string, right string) string {
	if ok {
		return left
	}
	return right
}

type runScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner runScanner) (Run, error) {
	var item Run
	var planRunID int64
	var planDefID int64
	if err := scanner.Scan(
		&planRunID,
		&planDefID,
		&item.PlanName,
		&item.TargetType,
		&item.TargetRefID,
		&item.RunDate,
		&item.Status,
		&item.StartedAt,
		&item.FinishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Run{}, ErrPlanRunNotFound
		}
		return Run{}, fmt.Errorf("scan plan run: %w", err)
	}
	item.PlanRunID = strconv.FormatInt(planRunID, 10)
	item.PlanDefID = strconv.FormatInt(planDefID, 10)
	return item, nil
}

type deviceRunScanner interface {
	Scan(dest ...any) error
}

func scanDeviceRun(scanner deviceRunScanner) (DeviceRun, error) {
	var item DeviceRun
	var planDeviceRunID int64
	var planRunID int64
	var planDefID int64
	var zoneID int64
	var rowID int64
	var slotID int64
	var deviceID int64
	if err := scanner.Scan(
		&planDeviceRunID,
		&planRunID,
		&planDefID,
		&zoneID,
		&rowID,
		&slotID,
		&deviceID,
		&item.TargetType,
		&item.TargetRefID,
		&item.Status,
		&item.CurrentNodeID,
		&item.NextRetryAt,
		&item.StartedAt,
		&item.FinishedAt,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeviceRun{}, ErrPlanDeviceRunNotFound
		}
		return DeviceRun{}, fmt.Errorf("scan plan device run: %w", err)
	}
	item.PlanDeviceRunID = strconv.FormatInt(planDeviceRunID, 10)
	item.PlanRunID = strconv.FormatInt(planRunID, 10)
	item.PlanDefID = strconv.FormatInt(planDefID, 10)
	item.ZoneID = strconv.FormatInt(zoneID, 10)
	item.RowID = strconv.FormatInt(rowID, 10)
	item.SlotID = strconv.FormatInt(slotID, 10)
	item.DeviceID = strconv.FormatInt(deviceID, 10)
	return item, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

type scriptPlanDispatchItem struct {
	taskID          string
	deviceID        string
	planDeviceRunID string
}

func scanEvent(scanner eventScanner) (Event, error) {
	var item Event
	var planRunID int64
	var planDefID int64
	var deviceIDStr string
	var extraJSON string
	if err := scanner.Scan(
		&item.PlanEventID,
		&planRunID,
		&planDefID,
		&deviceIDStr,
		&item.EventType,
		&item.Message,
		&extraJSON,
		&item.CreatedAt,
	); err != nil {
		return Event{}, fmt.Errorf("scan plan event: %w", err)
	}

	item.Extra = map[string]any{}
	if strings.TrimSpace(extraJSON) != "" {
		if err := json.Unmarshal([]byte(extraJSON), &item.Extra); err != nil {
			return Event{}, fmt.Errorf("decode plan event extra: %w", err)
		}
	}
	item.PlanRunID = strconv.FormatInt(planRunID, 10)
	item.PlanDefID = strconv.FormatInt(planDefID, 10)
	// device_id 列声明为 INTEGER，但历史数据中实例级事件写入过空串，
	// SQLite 动态类型会原样保留，直接 Scan 成 int64 会报错，因此按字符串读出再容错解析。
	deviceID, _ := strconv.ParseInt(strings.TrimSpace(deviceIDStr), 10, 64)
	item.DeviceID = strconv.FormatInt(deviceID, 10)
	return item, nil
}

func (s *Service) listRetryableTargets(ctx context.Context, now time.Time) ([]DeviceRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status,
       current_node_id, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
FROM plan_device_runs
WHERE status = ?
  AND next_retry_at <> ''
  AND next_retry_at <= ?
ORDER BY id ASC`,
		DeviceRunStatusPending,
		now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("query retryable plan targets: %w", err)
	}
	defer rows.Close()

	items := make([]DeviceRun, 0)
	for rows.Next() {
		item, err := scanDeviceRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retryable plan targets: %w", err)
	}
	return items, nil
}

func isDailyTimeValid(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	_, err := time.Parse("15:04:05", value)
	return err == nil
}

func isDailyStartReached(startTime string, now time.Time) bool {
	startTime = strings.TrimSpace(startTime)
	if startTime == "" {
		return false
	}
	parsed, err := time.Parse("15:04:05", startTime)
	if err != nil {
		return false
	}
	localNow := now.In(time.Local)
	startAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	return !localNow.Before(startAt)
}

func isDailyDeadlineReached(deadlineTime string, now time.Time) bool {
	deadlineTime = strings.TrimSpace(deadlineTime)
	if deadlineTime == "" {
		return false
	}
	parsed, err := time.Parse("15:04:05", deadlineTime)
	if err != nil {
		return false
	}
	localNow := now.In(time.Local)
	deadlineAt := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	return !localNow.Before(deadlineAt)
}

func shouldApplyDailyAdditionsImmediately(definition Definition, run Run, now time.Time) bool {
	if definition.ScheduleType != ScheduleTypeDaily {
		return true
	}
	if run.Status != RunStatusPending && run.Status != RunStatusRunning {
		return false
	}
	if !isSameLocalRunDate(run.RunDate, now) {
		return false
	}
	if isDailyDeadlineReached(definition.DailyDeadlineTime, now) {
		return false
	}
	return true
}

func isManualStartAllowed(definition Definition, now time.Time) bool {
	if definition.ScheduleType != ScheduleTypeDaily {
		return true
	}
	startTime := strings.TrimSpace(definition.DailyStartTime)
	if startTime == "" {
		return true
	}
	return !isDailyStartReached(startTime, now)
}

func isSameLocalRunDate(runDate string, now time.Time) bool {
	return strings.TrimSpace(runDate) == now.In(time.Local).Format("2006-01-02")
}

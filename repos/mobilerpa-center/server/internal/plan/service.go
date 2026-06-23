package plan

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
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
	// ErrPlanDeviceIDsRequired 表示缺少设备集合。
	ErrPlanDeviceIDsRequired = errors.New("plan device_ids are required")
	// ErrPlanDailyStartTimeInvalid 表示每日开始时间格式不合法。
	ErrPlanDailyStartTimeInvalid = errors.New("plan daily_start_time is invalid")
	// ErrPlanDailyDeadlineTimeInvalid 表示每日截止时间格式不合法。
	ErrPlanDailyDeadlineTimeInvalid = errors.New("plan daily_deadline_time is invalid")
	ErrPlanDefinitionRunning       = errors.New("plan definition has active runs")
	ErrPlanTodayAlreadyStarted     = errors.New("plan today already started")
	ErrPlanRunDeleteNotAllowed     = errors.New("plan run delete not allowed")
)

// DeviceBusyDetail 描述计划任务启动时发现的设备占用情况。
type DeviceBusyDetail struct {
	DeviceID           string `json:"device_id"`
	OccupancyType      string `json:"occupancy_type"`
	WorkflowDefID      string `json:"workflow_def_id"`
	WorkflowInstanceID string `json:"workflow_instance_id"`
	WorkflowRunID      string `json:"workflow_run_id"`
	TaskID             string `json:"task_id"`
	TaskStatus         string `json:"task_status"`
	Message            string `json:"message"`
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
	DeviceIDs           []string `json:"device_ids"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
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
	DeviceID        string `json:"device_id"`
	TargetType      string `json:"target_type"`
	TargetRefID     string `json:"target_ref_id"`
	Status          string `json:"status"`
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
	DeviceIDs           []string `json:"device_ids"`
}

type UpdateDefinitionDevicesRequest struct {
	DeviceIDs []string `json:"device_ids"`
}

// StartRequest 描述启动计划任务时的请求。
type StartRequest struct {
	DeviceIDs []string `json:"device_ids"`
	Manual    bool     `json:"-"`
}

// AddDevicesRequest 描述追加设备时的请求。
type AddDevicesRequest struct {
	DeviceIDs []string `json:"device_ids"`
}

// TaskCreator 定义计划任务调度单脚本时需要的最小任务创建能力。
type TaskCreator interface {
	Create(ctx context.Context, req task.CreateRequest) (task.Task, error)
}

// TaskDispatcher 定义计划任务下发单脚本任务时需要的能力。
type TaskDispatcher interface {
	AssignTask(ctx context.Context, taskID string) (task.Task, error)
}

// WorkflowRunner 定义计划任务调度工作流时依赖的最小工作流能力。
type WorkflowRunner interface {
	Start(ctx context.Context, workflowDefID string, req workflow.StartRequest) (workflow.Instance, error)
	AddDevices(ctx context.Context, workflowDefID string, req workflow.AddDevicesRequest) (workflow.Instance, error)
	StopDefinition(ctx context.Context, workflowDefID string, workflowInstanceID string) (workflow.Instance, error)
	StopRunByDevice(ctx context.Context, workflowDefID string, workflowInstanceID string, deviceID string) (workflow.Run, error)
	GetInstance(ctx context.Context, workflowInstanceID string) (workflow.Instance, error)
	GetDeviceBusyDetail(ctx context.Context, deviceID string) (*workflow.DeviceBusyDetail, error)
	GetRunByTaskID(ctx context.Context, taskID string) (workflow.Run, error)
}

// Service 负责计划任务定义、实例与统一调度外壳。
type Service struct {
	db         *sql.DB
	devices    *device.Service
	tasks      TaskCreator
	dispatcher TaskDispatcher
	workflows  WorkflowRunner
	startFanout int
	startMu     sync.Mutex
	starting    map[string]struct{}
}

// NewService 创建计划任务服务。
func NewService(db *sql.DB, devices *device.Service, tasks TaskCreator, dispatcher TaskDispatcher, workflows WorkflowRunner) *Service {
	return &Service{
		db:         db,
		devices:    devices,
		tasks:      tasks,
		dispatcher: dispatcher,
		workflows:  workflows,
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

// CreateDefinition 创建计划任务定义。
func (s *Service) CreateDefinition(ctx context.Context, req CreateDefinitionRequest) (Definition, error) {
	req = normalizeCreateDefinitionRequest(req)
	if err := validateDefinitionRequest(req); err != nil {
		return Definition{}, err
	}

	cleanDeviceIDs := uniqueDeviceIDs(req.DeviceIDs)
	if len(cleanDeviceIDs) == 0 {
		return Definition{}, ErrPlanDeviceIDsRequired
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

	for position, deviceID := range cleanDeviceIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_devices (
    plan_def_id, device_id, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?)`,
			planDefID,
			deviceID,
			position+1,
			now,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert plan device %s: %w", deviceID, err)
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

	deviceIDsByPlan, err := s.listDeviceIDsByPlan(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].DeviceIDs = deviceIDsByPlan[items[index].PlanDefID]
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

	deviceIDsByPlan, err := s.listDeviceIDsByPlan(ctx)
	if err != nil {
		return Definition{}, err
	}
	item.DeviceIDs = deviceIDsByPlan[item.PlanDefID]
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

	deviceIDs := uniqueDeviceIDs(req.DeviceIDs)
	if len(deviceIDs) == 0 {
		deviceIDs = append(deviceIDs, definition.DeviceIDs...)
	}
	if len(deviceIDs) == 0 {
		return Run{}, ErrPlanDeviceIDsRequired
	}

	busyDetails, err := s.ensureDevicesAvailable(ctx, definition.TargetType, "", deviceIDs)
	if err != nil {
		return Run{}, err
	}
	if len(busyDetails) > 0 {
		return Run{}, &DeviceBusyError{Details: busyDetails}
	}

	if definition.TargetType == TargetTypeWorkflow {
		return s.startWorkflowPlanRun(ctx, definition, deviceIDs)
	}
	return s.startScriptPlanRun(ctx, definition, deviceIDs)
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
			if errors.Is(err, ErrPlanDeviceIDsRequired) {
				continue
			}
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
		if errors.Is(err, ErrPlanDeviceIDsRequired) {
			return "", nil
		}
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

// AddDevices 为运行中的计划任务实例追加设备。
func (s *Service) AddDevices(ctx context.Context, planRunID string, req AddDevicesRequest) (Run, error) {
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

	deviceIDs := uniqueDeviceIDs(req.DeviceIDs)
	if len(deviceIDs) == 0 {
		return Run{}, ErrPlanDeviceIDsRequired
	}

	exists := make(map[string]struct{}, len(run.DeviceRuns))
	for _, item := range run.DeviceRuns {
		exists[item.DeviceID] = struct{}{}
	}

	filtered := make([]string, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if _, ok := exists[deviceID]; ok {
			continue
		}
		filtered = append(filtered, deviceID)
	}
	if len(filtered) == 0 {
		return run, nil
	}

	busyDetails, err := s.ensureDevicesAvailable(ctx, definition.TargetType, planRunID, filtered)
	if err != nil {
		return Run{}, err
	}
	if len(busyDetails) > 0 {
		return Run{}, &DeviceBusyError{Details: busyDetails}
	}

	switch definition.TargetType {
	case TargetTypeScript:
		if err := s.addScriptPlanDevices(ctx, definition, run, filtered); err != nil {
			return Run{}, err
		}
	case TargetTypeWorkflow:
		if err := s.addWorkflowPlanDevices(ctx, definition, run, filtered); err != nil {
			return Run{}, err
		}
	default:
		return Run{}, ErrPlanTargetTypeUnsupported
	}

	if err := s.syncDefinitionDevices(ctx, run.PlanDefID, filtered, nil); err != nil {
		return Run{}, err
	}

	return s.GetRun(ctx, planRunID)
}

// RemoveDevice 把某设备从运行中的计划任务实例中移除。
func (s *Service) RemoveDevice(ctx context.Context, planRunID string, deviceID string) (Run, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Run{}, ErrPlanDeviceRunNotFound
	}

	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return Run{}, err
	}
	if run.Status != RunStatusPending && run.Status != RunStatusRunning {
		return Run{}, ErrPlanRunNotActive
	}

	deviceRun, err := s.getDeviceRunByPlanAndDevice(ctx, planRunID, deviceID)
	if err != nil {
		return Run{}, err
	}

	definition, err := s.GetDefinition(ctx, run.PlanDefID)
	if err != nil {
		return Run{}, err
	}

	if deviceRun.Status == DeviceRunStatusSuccess || deviceRun.Status == DeviceRunStatusFailed || deviceRun.Status == DeviceRunStatusStopped {
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := s.db.ExecContext(ctx, `
DELETE FROM plan_device_runs
WHERE id = ?`,
			deviceRun.PlanDeviceRunID,
		); err != nil {
			return Run{}, fmt.Errorf("delete completed plan device run: %w", err)
		}
		if err := s.appendEvent(ctx, planRunID, run.PlanDefID, deviceID, EventTypePlanDeviceRemoved, "设备已从计划任务实例中移除", map[string]any{
			"source":             "center",
			"plan_device_run_id": deviceRun.PlanDeviceRunID,
			"reason":             "manual_remove",
			"removed_after_done": true,
			"removed_at":         now,
		}); err != nil {
			return Run{}, err
		}
		if err := s.refreshRunStatus(ctx, planRunID); err != nil {
			return Run{}, err
		}
		if err := s.syncDefinitionDevices(ctx, run.PlanDefID, nil, []string{deviceID}); err != nil {
			return Run{}, err
		}
		return s.GetRun(ctx, planRunID)
	}

	switch definition.TargetType {
	case TargetTypeScript:
		if err := s.stopScriptPlanDevice(ctx, definition, run, deviceRun, "manual_remove"); err != nil {
			return Run{}, err
		}
	case TargetTypeWorkflow:
		if err := s.stopWorkflowPlanDevice(ctx, definition, run, deviceRun, "manual_remove"); err != nil {
			return Run{}, err
		}
	default:
		return Run{}, ErrPlanTargetTypeUnsupported
	}

	if err := s.appendEvent(ctx, planRunID, run.PlanDefID, deviceID, EventTypePlanDeviceRemoved, "设备已从计划任务实例中移除", map[string]any{
		"source":             "center",
		"plan_device_run_id": deviceRun.PlanDeviceRunID,
		"reason":             "manual_remove",
	}); err != nil {
		return Run{}, err
	}
	if err := s.refreshRunStatus(ctx, planRunID); err != nil {
		return Run{}, err
	}
	if err := s.syncDefinitionDevices(ctx, run.PlanDefID, nil, []string{deviceID}); err != nil {
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

func (s *Service) UpdateDefinitionDevices(ctx context.Context, planDefID string, req UpdateDefinitionDevicesRequest) (Definition, error) {
	definition, err := s.GetDefinition(ctx, planDefID)
	if err != nil {
		return Definition{}, err
	}

	nextDeviceIDs := uniqueDeviceIDs(req.DeviceIDs)
	if len(nextDeviceIDs) == 0 {
		return Definition{}, ErrPlanDeviceIDsRequired
	}

	currentSet := make(map[string]struct{}, len(definition.DeviceIDs))
	nextSet := make(map[string]struct{}, len(nextDeviceIDs))
	for _, deviceID := range definition.DeviceIDs {
		currentSet[deviceID] = struct{}{}
	}
	for _, deviceID := range nextDeviceIDs {
		nextSet[deviceID] = struct{}{}
	}

	additions := make([]string, 0)
	removals := make([]string, 0)
	for _, deviceID := range nextDeviceIDs {
		if _, exists := currentSet[deviceID]; !exists {
			additions = append(additions, deviceID)
		}
	}
	for _, deviceID := range definition.DeviceIDs {
		if _, exists := nextSet[deviceID]; !exists {
			removals = append(removals, deviceID)
		}
	}

	if err := s.syncDefinitionDevices(ctx, planDefID, additions, removals); err != nil {
		return Definition{}, err
	}

	activeRuns, err := s.ListRuns(ctx, planDefID)
	if err != nil {
		return Definition{}, err
	}
	now := time.Now()
	for _, run := range activeRuns {
		if run.Status != RunStatusPending && run.Status != RunStatusRunning {
			continue
		}

		runCurrent := make(map[string]struct{}, len(run.DeviceRuns))
		for _, deviceRun := range run.DeviceRuns {
			runCurrent[deviceRun.DeviceID] = struct{}{}
		}

		runAdditions := make([]string, 0)
		for _, deviceID := range nextDeviceIDs {
			if _, exists := runCurrent[deviceID]; !exists {
				runAdditions = append(runAdditions, deviceID)
			}
		}

		runRemovals := make([]string, 0)
		for _, deviceRun := range run.DeviceRuns {
			if _, exists := nextSet[deviceRun.DeviceID]; !exists {
				runRemovals = append(runRemovals, deviceRun.DeviceID)
			}
		}

		if len(runAdditions) > 0 && shouldApplyDailyAdditionsImmediately(definition, run, now) {
			if _, err := s.AddDevices(ctx, run.PlanRunID, AddDevicesRequest{DeviceIDs: runAdditions}); err != nil {
				return Definition{}, err
			}
		}
		for _, deviceID := range runRemovals {
			if _, err := s.RemoveDevice(ctx, run.PlanRunID, deviceID); err != nil {
				return Definition{}, err
			}
		}
	}

	return s.GetDefinition(ctx, planDefID)
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

func (s *Service) startScriptPlanRun(ctx context.Context, definition Definition, deviceIDs []string) (Run, error) {
	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	runDate := now.In(time.Local).Format("2006-01-02")

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
		scriptTargetRef(definition),
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

	for _, deviceID := range deviceIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
			planRunID,
			definition.PlanDefID,
			deviceID,
			definition.TargetType,
			scriptTargetRef(definition),
			DeviceRunStatusRunning,
			nowText,
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
		"target_ref":  scriptTargetRef(definition),
	}); err != nil {
		return Run{}, err
	}

	if err := s.addScriptPlanDevices(ctx, definition, Run{PlanRunID: planRunID, PlanDefID: definition.PlanDefID}, deviceIDs); err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, planRunID)
}

func (s *Service) startWorkflowPlanRun(ctx context.Context, definition Definition, deviceIDs []string) (Run, error) {
	if s.workflows == nil {
		return Run{}, fmt.Errorf("workflow runner is not configured")
	}

	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	runDate := now.In(time.Local).Format("2006-01-02")

	instance, err := s.workflows.Start(ctx, definition.TargetWorkflowDefID, workflow.StartRequest{
		DeviceIDs: deviceIDs,
	})
	if err != nil {
		return Run{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, fmt.Errorf("begin workflow plan run tx: %w", err)
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
		instance.WorkflowInstanceID,
		runDate,
		definition.TargetType,
		RunStatusRunning,
		nowText,
		nowText,
		nowText,
	)
	if err != nil {
		return Run{}, fmt.Errorf("insert workflow plan run: %w", err)
	}
	insertedPlanRunID, err := result.LastInsertId()
	if err != nil {
		return Run{}, fmt.Errorf("read inserted workflow plan run id: %w", err)
	}
	planRunID := strconv.FormatInt(insertedPlanRunID, 10)

	for _, deviceRun := range instance.DeviceRuns {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			planRunID,
			definition.PlanDefID,
			deviceRun.DeviceID,
			definition.TargetType,
			deviceRun.WorkflowRunID,
			mapWorkflowStatus(deviceRun.Status),
			firstNonEmpty(deviceRun.StartedAt, nowText),
			deviceRun.FinishedAt,
			deviceRun.LastError,
			nowText,
			nowText,
		); err != nil {
			return Run{}, fmt.Errorf("insert workflow plan device run: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE workflow_instances
SET plan_run_id = ?
WHERE id = ?`,
		planRunID,
		instance.WorkflowInstanceID,
	); err != nil {
		return Run{}, fmt.Errorf("bind workflow instance to plan run: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Run{}, fmt.Errorf("commit workflow plan run tx: %w", err)
	}
	tx = nil

	if err := s.appendEvent(ctx, planRunID, definition.PlanDefID, "", EventTypePlanRunStarted, "计划任务实例已启动", map[string]any{
		"source":               "center",
		"target_type":          definition.TargetType,
		"workflow_instance_id": instance.WorkflowInstanceID,
		"workflow_def_id":      definition.TargetWorkflowDefID,
	}); err != nil {
		return Run{}, err
	}

	for _, deviceRun := range instance.DeviceRuns {
		if err := s.appendEvent(ctx, planRunID, definition.PlanDefID, deviceRun.DeviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{
			"source":               "center",
			"workflow_run_id":      deviceRun.WorkflowRunID,
			"workflow_instance_id": instance.WorkflowInstanceID,
		}); err != nil {
			return Run{}, err
		}
	}

	if err := s.syncWorkflowPlanRun(ctx, planRunID); err != nil {
		return Run{}, err
	}
	return s.GetRun(ctx, planRunID)
}

func (s *Service) addScriptPlanDevices(ctx context.Context, definition Definition, run Run, deviceIDs []string) error {
	if s.tasks == nil || s.dispatcher == nil {
		return fmt.Errorf("task creator or dispatcher is not configured")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	dispatchItems := make([]scriptPlanDispatchItem, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceRun, err := s.getDeviceRunByPlanAndDevice(ctx, run.PlanRunID, deviceID)
		if err != nil {
			return err
		}

		if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END, updated_at = ?
WHERE id = ?`,
			DeviceRunStatusRunning,
			now,
			now,
			deviceRun.PlanDeviceRunID,
		); err != nil {
			return fmt.Errorf("update script plan device run started: %w", err)
		}

		createdTask, err := s.tasks.Create(ctx, task.CreateRequest{
			DeviceID:      deviceID,
			ScriptName:    definition.TargetScriptName,
			ScriptVersion: definition.TargetScriptVersion,
		})
		if err != nil {
			return fmt.Errorf("create plan script task: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET plan_run_id = ?, plan_device_run_id = ?, task_source_type = 'plan_script'
WHERE id = ?`,
			run.PlanRunID,
			deviceRun.PlanDeviceRunID,
			createdTask.TaskID,
		); err != nil {
			return fmt.Errorf("bind plan script task metadata: %w", err)
		}

		if err := s.appendEvent(ctx, run.PlanRunID, definition.PlanDefID, deviceID, EventTypePlanDeviceStarted, "设备已开始执行计划任务", map[string]any{
			"source":             "center",
			"plan_device_run_id": deviceRun.PlanDeviceRunID,
			"task_id":            createdTask.TaskID,
			"script_name":        definition.TargetScriptName,
			"script_version":     definition.TargetScriptVersion,
		}); err != nil {
			return err
		}

		dispatchItems = append(dispatchItems, scriptPlanDispatchItem{taskID: createdTask.TaskID})
	}
	return s.dispatchScriptPlanTasks(ctx, dispatchItems)
}

func (s *Service) addWorkflowPlanDevices(ctx context.Context, definition Definition, run Run, deviceIDs []string) error {
	if s.workflows == nil {
		return fmt.Errorf("workflow runner is not configured")
	}

	instance, err := s.workflows.AddDevices(ctx, definition.TargetWorkflowDefID, workflow.AddDevicesRequest{
		WorkflowInstanceID: run.TargetRefID,
		DeviceIDs:          deviceIDs,
	})
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, workflowRun := range instance.DeviceRuns {
		if !containsDeviceID(deviceIDs, workflowRun.DeviceID) {
			continue
		}
		result, err := s.db.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			run.PlanRunID,
			run.PlanDefID,
			workflowRun.DeviceID,
			definition.TargetType,
			workflowRun.WorkflowRunID,
			mapWorkflowStatus(workflowRun.Status),
			firstNonEmpty(workflowRun.StartedAt, now),
			workflowRun.FinishedAt,
			workflowRun.LastError,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("insert appended workflow plan device run: %w", err)
		}
		insertedPlanDeviceRunID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read inserted appended workflow plan device run id: %w", err)
		}
		planDeviceRunID := strconv.FormatInt(insertedPlanDeviceRunID, 10)

		if err := s.appendEvent(ctx, run.PlanRunID, run.PlanDefID, workflowRun.DeviceID, EventTypePlanDeviceAdded, "设备已追加到计划任务实例", map[string]any{
			"source":               "center",
			"plan_device_run_id":   planDeviceRunID,
			"workflow_run_id":      workflowRun.WorkflowRunID,
			"workflow_instance_id": instance.WorkflowInstanceID,
		}); err != nil {
			return err
		}
	}
	if err := s.syncWorkflowPlanRun(ctx, run.PlanRunID); err != nil {
		return err
	}
	return nil
}

func (s *Service) stopScriptPlanDevice(ctx context.Context, definition Definition, run Run, deviceRun DeviceRun, reason string) error {
	taskID, taskStatus, err := s.lookupTaskByPlanDeviceRun(ctx, deviceRun.PlanDeviceRunID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if taskID != "" && (taskStatus == task.StatusAssigned || taskStatus == task.StatusRunning) {
		if taskService, ok := s.tasks.(*task.Service); ok {
			if _, err := taskService.StopManualTask(ctx, taskID, "计划任务移除设备"); err != nil && !errors.Is(err, task.ErrTaskManualStopNotAllowed) {
				return err
			}
		}
	}

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
	if s.workflows == nil {
		return fmt.Errorf("workflow runner is not configured")
	}

	workflowInstanceID := strings.TrimSpace(run.TargetRefID)
	if workflowInstanceID == "" {
		row := s.db.QueryRowContext(ctx, `
SELECT id
FROM workflow_instances
WHERE plan_run_id = ?
ORDER BY id DESC
LIMIT 1`, run.PlanRunID)
		if err := row.Scan(&workflowInstanceID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query workflow instance by plan run: %w", err)
		}
		workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	}

	if workflowInstanceID != "" {
		if _, err := s.workflows.StopRunByDevice(ctx, definition.TargetWorkflowDefID, workflowInstanceID, deviceRun.DeviceID); err != nil && !errors.Is(err, workflow.ErrWorkflowRunNotFound) {
			return err
		}
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
	if err := s.syncWorkflowPlanRun(ctx, run.PlanRunID); err != nil {
		return err
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

// SyncWorkflowRunByTask 在工作流任务结果推进后，把工作流实例状态回写到计划任务实例。
func (s *Service) SyncWorkflowRunByTask(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || s.workflows == nil {
		return nil
	}

	row := s.db.QueryRowContext(ctx, `
SELECT plan_run_id, workflow_instance_id
FROM tasks
WHERE id = ?`,
		taskID,
	)

	var planRunID string
	var workflowInstanceID string
	if err := row.Scan(&planRunID, &workflowInstanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("query workflow plan task metadata: %w", err)
	}
	planRunID = strings.TrimSpace(planRunID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	if workflowInstanceID == "" {
		return nil
	}
	if planRunID == "" {
		instanceRow := s.db.QueryRowContext(ctx, `
SELECT plan_run_id
FROM workflow_instances
WHERE id = ?`,
			workflowInstanceID,
		)
		if err := instanceRow.Scan(&planRunID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("query plan run by workflow instance: %w", err)
		}
		planRunID = strings.TrimSpace(planRunID)
	}
	if planRunID == "" {
		return nil
	}
	return s.syncWorkflowPlanRun(ctx, planRunID)
}

// SyncWorkflowRunBySession 在工作流会话结果回传后，把工作流实例状态回写到计划任务实例。
func (s *Service) SyncWorkflowRunBySession(ctx context.Context, workflowRunID string) error {
	workflowRunID = strings.TrimSpace(workflowRunID)
	if workflowRunID == "" {
		return nil
	}

	row := s.db.QueryRowContext(ctx, `
SELECT workflow_instance_id
FROM workflow_runs
WHERE id = ?`,
		workflowRunID,
	)

	var workflowInstanceID string
	if err := row.Scan(&workflowInstanceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("query workflow instance by workflow run: %w", err)
	}
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	if workflowInstanceID == "" {
		return nil
	}

	instanceRow := s.db.QueryRowContext(ctx, `
SELECT plan_run_id
FROM workflow_instances
WHERE id = ?`,
		workflowInstanceID,
	)

	var planRunID string
	if err := instanceRow.Scan(&planRunID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("query plan run by workflow instance: %w", err)
	}
	planRunID = strings.TrimSpace(planRunID)
	if planRunID == "" {
		return nil
	}

	return s.syncWorkflowPlanRun(ctx, planRunID)
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
	run, err := s.GetRun(ctx, planRunID)
	if err != nil {
		return err
	}
	if run.TargetType != TargetTypeWorkflow || s.workflows == nil || strings.TrimSpace(run.TargetRefID) == "" {
		return nil
	}

	workflowInstance, err := s.workflows.GetInstance(ctx, run.TargetRefID)
	if err != nil {
		return err
	}

	existingMap := make(map[string]DeviceRun, len(run.DeviceRuns))
	for _, item := range run.DeviceRuns {
		existingMap[item.DeviceID] = item
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, workflowRun := range workflowInstance.DeviceRuns {
		existing, exists := existingMap[workflowRun.DeviceID]
		if !exists {
			continue
		}
		nextStatus := mapWorkflowStatus(workflowRun.Status)
		lastError := workflowRun.LastError
		if _, err := s.db.ExecContext(ctx, `
UPDATE plan_device_runs
SET status = ?, target_ref_id = ?, started_at = CASE WHEN started_at = '' THEN ? ELSE started_at END,
    finished_at = ?, last_error = ?, updated_at = ?
WHERE id = ?`,
			nextStatus,
			workflowRun.WorkflowRunID,
			firstNonEmpty(workflowRun.StartedAt, now),
			workflowRun.FinishedAt,
			lastError,
			now,
			existing.PlanDeviceRunID,
		); err != nil {
			return fmt.Errorf("sync workflow plan device run: %w", err)
		}

		if nextStatus == DeviceRunStatusSuccess || nextStatus == DeviceRunStatusFailed || nextStatus == DeviceRunStatusStopped {
			if err := s.appendEvent(ctx, planRunID, run.PlanDefID, workflowRun.DeviceID, EventTypePlanDeviceCompleted, "设备计划任务执行已结束", map[string]any{
				"source":             "center",
				"plan_device_run_id": existing.PlanDeviceRunID,
				"workflow_run_id":    workflowRun.WorkflowRunID,
				"status":             nextStatus,
				"last_error":         lastError,
			}); err != nil {
				return err
			}
		}
	}

	return s.refreshRunStatus(ctx, planRunID)
}

func (s *Service) ensureDevicesAvailable(ctx context.Context, targetType string, currentPlanRunID string, deviceIDs []string) ([]DeviceBusyDetail, error) {
	details := make([]DeviceBusyDetail, 0)
	for _, deviceID := range deviceIDs {
		if s.devices != nil {
			if err := s.devices.EnsureExecutionReady(ctx, deviceID); err != nil {
				return nil, err
			}
		}
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

	if targetType == TargetTypeWorkflow && s.workflows != nil {
		workflowBusy, err := s.workflows.GetDeviceBusyDetail(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		if workflowBusy != nil {
			return &DeviceBusyDetail{
				DeviceID:           workflowBusy.DeviceID,
				OccupancyType:      workflowBusy.OccupancyType,
				WorkflowDefID:      workflowBusy.WorkflowDefID,
				WorkflowInstanceID: workflowBusy.WorkflowInstanceID,
				WorkflowRunID:      workflowBusy.WorkflowRunID,
				TaskID:             workflowBusy.TaskID,
				TaskStatus:         workflowBusy.TaskStatus,
				Message:            workflowBusy.Message,
			}, nil
		}
	}

	if targetType == TargetTypeScript {
		row = s.db.QueryRowContext(ctx, `
SELECT id AS task_id, status
FROM tasks
WHERE device_id = ?
  AND task_source_type = 'manual'
  AND status IN (?, ?)
ORDER BY task_id DESC
LIMIT 1`,
			deviceID,
			task.StatusAssigned,
			task.StatusRunning,
		)
		var taskID string
		var taskStatus string
		if err := row.Scan(&taskID, &taskStatus); err == nil {
			return &DeviceBusyDetail{
				DeviceID:      deviceID,
				OccupancyType: "manual_task",
				TaskID:        taskID,
				TaskStatus:    taskStatus,
				Message:       "设备已被手工任务占用",
			}, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("query busy manual task: %w", err)
		}
	}
	return nil, nil
}

// GetDeviceBusyDetail 返回某台设备当前是否被计划任务、工作流或手工任务占用。
func (s *Service) GetDeviceBusyDetail(ctx context.Context, deviceID string) (*DeviceBusyDetail, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	return s.inspectDeviceBusy(ctx, "", "", strings.TrimSpace(deviceID))
}

func (s *Service) listDeviceIDsByPlan(ctx context.Context) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT plan_def_id, device_id
FROM plan_devices
ORDER BY plan_def_id ASC, position ASC, device_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query plan devices: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var planDefID string
		var deviceID string
		if err := rows.Scan(&planDefID, &deviceID); err != nil {
			return nil, fmt.Errorf("scan plan device: %w", err)
		}
		result[planDefID] = append(result[planDefID], deviceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate plan devices: %w", err)
	}
	return result, nil
}

func (s *Service) listDeviceRunsByPlanRun(ctx context.Context, planRunID string) ([]DeviceRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status,
       started_at, finished_at, last_error, created_at, updated_at
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
SELECT id AS plan_device_run_id, plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status,
       started_at, finished_at, last_error, created_at, updated_at
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

func (s *Service) appendEvent(ctx context.Context, planRunID string, planDefID string, deviceID string, eventType string, message string, extra map[string]any) error {
	if extra == nil {
		extra = map[string]any{}
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

func (s *Service) syncDefinitionDevices(ctx context.Context, planDefID string, additions []string, removals []string) error {
	planDefID = strings.TrimSpace(planDefID)
	if planDefID == "" {
		return nil
	}

	additions = uniqueDeviceIDs(additions)
	removals = uniqueDeviceIDs(removals)
	if len(additions) == 0 && len(removals) == 0 {
		return nil
	}

	currentMap, err := s.listDeviceIDsByPlan(ctx)
	if err != nil {
		return err
	}
	current := append([]string(nil), currentMap[planDefID]...)

	next := make([]string, 0, len(current)+len(additions))
	seen := make(map[string]struct{}, len(current)+len(additions))
	removeSet := make(map[string]struct{}, len(removals))
	for _, deviceID := range removals {
		removeSet[deviceID] = struct{}{}
	}

	for _, deviceID := range current {
		if _, removed := removeSet[deviceID]; removed {
			continue
		}
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		next = append(next, deviceID)
	}
	for _, deviceID := range additions {
		if _, removed := removeSet[deviceID]; removed {
			continue
		}
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		next = append(next, deviceID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync plan devices tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM plan_devices
WHERE plan_def_id = ?`, planDefID); err != nil {
		return fmt.Errorf("clear plan devices: %w", err)
	}

	for index, deviceID := range next {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO plan_devices (
    plan_def_id, device_id, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?)`,
			planDefID,
			deviceID,
			index+1,
			now,
			now,
		); err != nil {
			return fmt.Errorf("reinsert plan device %s: %w", deviceID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE plan_defs
SET updated_at = ?
WHERE id = ?`,
		now,
		planDefID,
	); err != nil {
		return fmt.Errorf("touch plan definition updated_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync plan devices tx: %w", err)
	}
	tx = nil
	return nil
}

func (s *Service) dispatcherAssign(ctx context.Context, taskID string) error {
	if s.dispatcher == nil {
		return fmt.Errorf("task dispatcher is not configured")
	}
	if _, err := s.dispatcher.AssignTask(ctx, taskID); err != nil {
		return fmt.Errorf("assign plan task: %w", err)
	}
	return nil
}

func (s *Service) dispatchScriptPlanTasks(ctx context.Context, items []scriptPlanDispatchItem) error {
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

	taskCh := make(chan string)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for taskID := range taskCh {
			if err := s.dispatcherAssign(ctx, taskID); err != nil {
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
		case taskCh <- item.taskID:
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
	if len(req.DeviceIDs) == 0 {
		return ErrPlanDeviceIDsRequired
	}
	if !isDailyTimeValid(req.DailyStartTime) {
		return ErrPlanDailyStartTimeInvalid
	}
	if !isDailyTimeValid(req.DailyDeadlineTime) {
		return ErrPlanDailyDeadlineTimeInvalid
	}
	return nil
}

func uniqueDeviceIDs(deviceIDs []string) []string {
	result := make([]string, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, item := range deviceIDs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func scriptTargetRef(definition Definition) string {
	return strings.TrimSpace(definition.TargetScriptName) + "@" + strings.TrimSpace(definition.TargetScriptVersion)
}

func mapWorkflowStatus(status string) string {
	switch strings.TrimSpace(status) {
	case workflow.RunStatusPending:
		return DeviceRunStatusPending
	case workflow.RunStatusRunning:
		return DeviceRunStatusRunning
	case workflow.RunStatusSuccess:
		return DeviceRunStatusSuccess
	case workflow.RunStatusFailed:
		return DeviceRunStatusFailed
	case workflow.RunStatusStopped:
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
	var deviceID int64
	if err := scanner.Scan(
		&planDeviceRunID,
		&planRunID,
		&planDefID,
		&deviceID,
		&item.TargetType,
		&item.TargetRefID,
		&item.Status,
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
	item.DeviceID = strconv.FormatInt(deviceID, 10)
	return item, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

type scriptPlanDispatchItem struct {
	taskID string
}

func scanEvent(scanner eventScanner) (Event, error) {
	var item Event
	var planRunID int64
	var planDefID int64
	var deviceID int64
	var extraJSON string
	if err := scanner.Scan(
		&item.PlanEventID,
		&planRunID,
		&planDefID,
		&deviceID,
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
	item.DeviceID = strconv.FormatInt(deviceID, 10)
	return item, nil
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


package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// TopicTasks 表示任务事件统一归属到 tasks 主题。
	TopicTasks = "tasks"

	// StatusPending 表示任务已创建，等待后续下发。
	StatusPending = "pending"
	// StatusAssigned 表示任务已经下发到设备，等待设备确认。
	StatusAssigned = "assigned"
	// StatusRunning 表示任务已经开始执行。
	StatusRunning = "running"
	// StatusSuccess 表示任务执行成功。
	StatusSuccess = "success"
	// StatusFailed 表示任务执行失败。
	StatusFailed = "failed"

	// EventTypeTaskCreated 表示任务创建事件。
	EventTypeTaskCreated = "task_created"
	// EventTypeTaskAssigned 表示任务下发事件。
	EventTypeTaskAssigned = "task_assigned"
	// EventTypeTaskAck 表示设备确认收到任务事件。
	EventTypeTaskAck = "task_ack"
	// EventTypeTaskRunning 表示设备开始执行任务事件。
	EventTypeTaskRunning = "task_running"
	// EventTypeTaskProgress 表示设备上报关键步骤事件。
	EventTypeTaskProgress = "task_progress"
	// EventTypeTaskResult 表示设备回传任务执行结果事件。
	EventTypeTaskResult = "task_result"
)

var (
	// ErrTaskNotFound 表示任务不存在。
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskDeviceNotFound 表示任务目标设备不存在。
	ErrTaskDeviceNotFound = errors.New("task device not found")
	// ErrTaskDeviceRequired 表示缺少设备标识。
	ErrTaskDeviceRequired = errors.New("device_id is required")
	// ErrTaskScriptNameRequired 表示缺少脚本名称。
	ErrTaskScriptNameRequired = errors.New("script_name is required")
	// ErrTaskPriorityInvalid 表示优先级非法。
	ErrTaskPriorityInvalid = errors.New("priority must be greater than or equal to 0")
	// ErrTaskScheduledAtInvalid 表示计划执行时间格式非法。
	ErrTaskScheduledAtInvalid = errors.New("scheduled_at must be RFC3339 format")
	// ErrTaskAlreadyAssigned 表示任务已经进入下发后的状态，不能重复下发。
	ErrTaskAlreadyAssigned = errors.New("task already assigned")
	// ErrTaskDeleteNotAllowed 表示当前任务仍在执行中，不允许删除。
	ErrTaskDeleteNotAllowed = errors.New("task delete not allowed")
)

// Service 负责任务创建、状态推进与任务事件记录。
type Service struct {
	db *sql.DB
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// CreateRequest 描述创建任务时接收的请求体。
type CreateRequest struct {
	// DeviceID 是任务目标设备标识。
	DeviceID string `json:"device_id"`
	// ScriptName 是待执行脚本名称。
	ScriptName string `json:"script_name"`
	// ScriptVersion 是期望执行的脚本版本。
	ScriptVersion string `json:"script_version"`
	// Params 是透传给脚本执行入口的参数。
	Params map[string]any `json:"params"`
	// Priority 是任务优先级，值越大优先级越高。
	Priority int `json:"priority"`
	// ScheduledAt 是计划执行时间，当前阶段先做记录。
	ScheduledAt string `json:"scheduled_at"`
}

// Task 表示中心服务返回的任务记录。
type Task struct {
	// TaskID 是中心生成的任务标识。
	TaskID string `json:"task_id"`
	// DeviceID 是目标设备标识。
	DeviceID string `json:"device_id"`
	// WorkflowNodeID 是关联的工作流节点标识。
	WorkflowNodeID string `json:"workflow_node_id"`
	// TaskSourceType 表示任务来源，例如 plan_script 或 workflow_session。
	TaskSourceType string `json:"task_source_type"`
	// ScriptName 是脚本名称。
	ScriptName string `json:"script_name"`
	// ScriptVersion 是脚本版本。
	ScriptVersion string `json:"script_version"`
	// Params 是执行参数。
	Params map[string]any `json:"params"`
	// Status 是任务当前状态。
	Status string `json:"status"`
	// Priority 是任务优先级。
	Priority int `json:"priority"`
	// RetryCount 是当前已记录的重试次数。
	RetryCount int `json:"retry_count"`
	// CurrentStep 是当前步骤名称。
	CurrentStep string `json:"current_step"`
	// ResultCode 是结果码。
	ResultCode string `json:"result_code"`
	// ResultMessage 是结果摘要。
	ResultMessage string `json:"result_message"`
	// ScheduledAt 是计划执行时间。
	ScheduledAt string `json:"scheduled_at"`
	// StartedAt 是开始执行时间。
	StartedAt string `json:"started_at"`
	// FinishedAt 是结束执行时间。
	FinishedAt string `json:"finished_at"`
	// CreatedAt 是创建时间。
	CreatedAt string `json:"created_at"`
	// UpdatedAt 是最后更新时间。
	UpdatedAt string `json:"updated_at"`
}

// Event 表示任务事件记录。
type Event struct {
	// TaskEventID 是任务事件的业务主键字段。
	TaskEventID int64 `json:"task_event_id"`
	// Topic 是事件所属主题。
	Topic string `json:"topic"`
	// TaskID 是关联任务标识。
	TaskID string `json:"task_id"`
	// DeviceID 是关联设备标识。
	DeviceID string `json:"device_id"`
	// EventType 是事件类型。
	EventType string `json:"event_type"`
	// TaskStatus 是事件发生后的任务状态。
	TaskStatus string `json:"task_status"`
	// StepName 是事件关联步骤名称。
	StepName string `json:"step_name"`
	// Message 是给人工查看的摘要。
	Message string `json:"message"`
	// Extra 是保留给扩展使用的事件附加字段。
	Extra map[string]any `json:"extra"`
	// CreatedAt 是事件创建时间。
	CreatedAt string `json:"created_at"`
}

// AckPayload 描述设备回传的 task_ack 载荷。
type AckPayload struct {
	// TaskID 是已收到的任务标识。
	TaskID string `json:"task_id"`
	// Status 是确认状态，当前要求为 ok。
	Status string `json:"status"`
	// Message 是补充说明。
	Message string `json:"message"`
}

// ResultPayload 描述设备回传的 task_result 载荷。
type ResultPayload struct {
	// TaskID 是任务标识。
	TaskID string `json:"task_id"`
	// Status 是执行结果状态，支持 success 或 failed。
	Status string `json:"status"`
	// ResultCode 是执行结果码。
	ResultCode string `json:"result_code"`
	// ResultMessage 是执行结果摘要。
	ResultMessage string `json:"result_message"`
	// StepName 是当前步骤名。
	StepName string `json:"step_name"`
	// Extra 是执行结果扩展信息。
	Extra map[string]any `json:"extra"`
}

// ProgressPayload 描述设备回传的 task_progress 载荷。
type ProgressPayload struct {
	// TaskID 是任务标识。
	TaskID string `json:"task_id"`
	// Status 是当前步骤状态，支持 running、success 或 failed。
	Status string `json:"status"`
	// StepName 是步骤名称。
	StepName string `json:"step_name"`
	// Message 是步骤说明。
	Message string `json:"message"`
	// Extra 是步骤附加信息。
	Extra map[string]any `json:"extra"`
}

// NewService 使用数据库连接创建任务服务。
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create 创建单设备任务，并同步写入 task_created 事件。
func (s *Service) Create(ctx context.Context, req CreateRequest) (Task, error) {
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ScriptName = strings.TrimSpace(req.ScriptName)
	req.ScriptVersion = strings.TrimSpace(req.ScriptVersion)
	req.ScheduledAt = strings.TrimSpace(req.ScheduledAt)

	if req.DeviceID == "" {
		return Task{}, ErrTaskDeviceRequired
	}
	if req.ScriptName == "" {
		return Task{}, ErrTaskScriptNameRequired
	}
	if req.Priority < 0 {
		return Task{}, ErrTaskPriorityInvalid
	}
	if req.ScheduledAt != "" {
		if _, err := time.Parse(time.RFC3339, req.ScheduledAt); err != nil {
			return Task{}, ErrTaskScheduledAtInvalid
		}
	}

	exists, err := s.deviceExists(ctx, req.DeviceID)
	if err != nil {
		return Task{}, err
	}
	if !exists {
		return Task{}, ErrTaskDeviceNotFound
	}

	paramsJSON, err := marshalJSONObject(req.Params)
	if err != nil {
		return Task{}, fmt.Errorf("marshal task params: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin create task tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO tasks (
    device_id, script_name, script_version, params_json, status, priority,
    retry_count, current_step, result_code, result_message, scheduled_at, started_at,
    finished_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, 0, '', '', '', ?, '', '', ?, ?)`,
		req.DeviceID,
		req.ScriptName,
		req.ScriptVersion,
		paramsJSON,
		StatusPending,
		req.Priority,
		req.ScheduledAt,
		now,
		now,
	)
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return Task{}, fmt.Errorf("read inserted task id: %w", err)
	}
	taskID := strconv.FormatInt(insertedID, 10)

	extra := map[string]any{
		"topic":          TopicTasks,
		"event_type":     EventTypeTaskCreated,
		"task_status":    StatusPending,
		"script_name":    req.ScriptName,
		"script_version": req.ScriptVersion,
		"source":         "center",
	}
	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskID,
		DeviceID:   req.DeviceID,
		EventType:  EventTypeTaskCreated,
		TaskStatus: StatusPending,
		Message:    "任务已创建，等待后续下发",
		Extra:      extra,
		CreatedAt:  now,
	}, tx); err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit create task tx: %w", err)
	}
	tx = nil

	return s.GetByID(ctx, taskID)
}

// List 返回任务列表，支持按任务来源过滤，默认按任务序号倒序排列。
func (s *Service) List(ctx context.Context, sourceType string) ([]Task, error) {
	sourceType = strings.TrimSpace(sourceType)

	query := `
SELECT id, device_id, script_name, script_version, params_json, status, priority,
       workflow_node_id, task_source_type,
       retry_count, current_step, result_code, result_message, scheduled_at, started_at,
       finished_at, created_at, updated_at
FROM tasks`
	args := make([]any, 0, 1)
	if sourceType != "" {
		query += `
WHERE task_source_type = ?`
		args = append(args, sourceType)
	}
	query += `
ORDER BY id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0)
	for rows.Next() {
		taskItem, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, taskItem)
	}

	return tasks, rows.Err()
}

// GetByID 根据任务标识返回单个任务。
func (s *Service) GetByID(ctx context.Context, taskID string) (Task, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Task{}, ErrTaskNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id, device_id, script_name, script_version, params_json, status, priority,
       workflow_node_id, task_source_type,
       retry_count, current_step, result_code, result_message, scheduled_at, started_at,
       finished_at, created_at, updated_at
FROM tasks
WHERE id = ?`, taskID)

	taskItem, err := scanTask(row)
	if err != nil {
		return Task{}, err
	}
	return taskItem, nil
}

// Delete 删除一个已结束的任务及其事件记录。
func (s *Service) Delete(ctx context.Context, taskID string) error {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	if taskItem.Status == StatusPending || taskItem.Status == StatusAssigned || taskItem.Status == StatusRunning {
		return ErrTaskDeleteNotAllowed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete task tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM task_events
WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task events: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM tasks
WHERE id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete task tx: %w", err)
	}
	tx = nil
	return nil
}

// ListEvents 返回指定任务的事件列表。
func (s *Service) ListEvents(ctx context.Context, taskID string) ([]Event, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, ErrTaskNotFound
	}
	if _, err := s.GetByID(ctx, taskID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, task_id, device_id, event_type, step_name, message, extra_json, created_at
FROM task_events
WHERE task_id = ?
ORDER BY id ASC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query task events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// MarkAssigned 把任务推进到 assigned，并记录任务下发事件。
func (s *Service) MarkAssigned(ctx context.Context, taskID string, requestID string) (Task, error) {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	if taskItem.Status != StatusPending {
		return Task{}, ErrTaskAlreadyAssigned
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.updateTaskState(ctx, taskID, StatusAssigned, "", "", "", taskItem.StartedAt, taskItem.FinishedAt, now); err != nil {
		return Task{}, err
	}

	extra := map[string]any{
		"topic":       TopicTasks,
		"event_type":  EventTypeTaskAssigned,
		"task_status": StatusAssigned,
		"request_id":  requestID,
		"source":      "center",
	}
	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskItem.TaskID,
		DeviceID:   taskItem.DeviceID,
		EventType:  EventTypeTaskAssigned,
		TaskStatus: StatusAssigned,
		Message:    "任务已下发到设备，等待设备确认",
		Extra:      extra,
		CreatedAt:  now,
	}, nil); err != nil {
		return Task{}, err
	}

	return s.GetByID(ctx, taskID)
}

// MarkAcknowledged 处理设备回传的 task_ack，并保持任务处于 assigned 状态。
func (s *Service) MarkAcknowledged(ctx context.Context, taskID string, payload AckPayload, requestID string) (Task, error) {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	if requestID != "" {
		exists, err := s.eventExistsByRequestID(ctx, taskID, EventTypeTaskAck, requestID)
		if err != nil {
			return Task{}, err
		}
		if exists {
			return taskItem, nil
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "设备已收到任务"
	}
	if payload.Status == "" {
		payload.Status = "ok"
	}

	if err := s.updateTaskState(ctx, taskID, StatusAssigned, taskItem.CurrentStep, taskItem.ResultCode, taskItem.ResultMessage, taskItem.StartedAt, taskItem.FinishedAt, now); err != nil {
		return Task{}, err
	}

	extra := map[string]any{
		"topic":       TopicTasks,
		"event_type":  EventTypeTaskAck,
		"task_status": StatusAssigned,
		"request_id":  requestID,
		"ack_status":  payload.Status,
		"source":      "agent",
	}
	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskItem.TaskID,
		DeviceID:   taskItem.DeviceID,
		EventType:  EventTypeTaskAck,
		TaskStatus: StatusAssigned,
		Message:    message,
		Extra:      extra,
		CreatedAt:  now,
	}, nil); err != nil {
		return Task{}, err
	}

	return s.GetByID(ctx, taskID)
}

// MarkRunning 把任务推进到 running，并记录开始执行事件。
func (s *Service) MarkRunning(ctx context.Context, taskID string, requestID string, stepName string, message string) (Task, error) {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	if requestID != "" {
		exists, err := s.eventExistsByRequestID(ctx, taskID, EventTypeTaskRunning, requestID)
		if err != nil {
			return Task{}, err
		}
		if exists {
			return taskItem, nil
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	stepName = strings.TrimSpace(stepName)
	message = strings.TrimSpace(message)
	if message == "" {
		message = "设备已开始执行任务"
	}

	startedAt := taskItem.StartedAt
	if startedAt == "" {
		startedAt = now
	}
	if err := s.updateTaskState(ctx, taskID, StatusRunning, stepName, taskItem.ResultCode, taskItem.ResultMessage, startedAt, taskItem.FinishedAt, now); err != nil {
		return Task{}, err
	}

	extra := map[string]any{
		"topic":       TopicTasks,
		"event_type":  EventTypeTaskRunning,
		"task_status": StatusRunning,
		"request_id":  requestID,
		"source":      "agent",
	}
	if stepName != "" {
		extra["step_name"] = stepName
	}
	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskItem.TaskID,
		DeviceID:   taskItem.DeviceID,
		EventType:  EventTypeTaskRunning,
		TaskStatus: StatusRunning,
		StepName:   stepName,
		Message:    message,
		Extra:      extra,
		CreatedAt:  now,
	}, nil); err != nil {
		return Task{}, err
	}

	return s.GetByID(ctx, taskID)
}

// MarkProgress 处理设备回传的 task_progress，并写入关键步骤事件。
func (s *Service) MarkProgress(ctx context.Context, taskID string, payload ProgressPayload, requestID string) (Task, error) {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	if requestID != "" {
		exists, err := s.eventExistsByRequestID(ctx, taskID, EventTypeTaskProgress, requestID)
		if err != nil {
			return Task{}, err
		}
		if exists {
			return taskItem, nil
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	stepName := strings.TrimSpace(payload.StepName)
	progressStatus := strings.TrimSpace(payload.Status)
	if progressStatus == "" {
		progressStatus = StatusRunning
	}

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		if stepName != "" {
			message = "任务执行中：" + stepName
		} else {
			message = "任务执行中"
		}
	}

	startedAt := taskItem.StartedAt
	if startedAt == "" {
		startedAt = now
	}
	if err := s.updateTaskState(ctx, taskID, StatusRunning, stepName, taskItem.ResultCode, taskItem.ResultMessage, startedAt, taskItem.FinishedAt, now); err != nil {
		return Task{}, err
	}

	extra := map[string]any{
		"topic":           TopicTasks,
		"event_type":      EventTypeTaskProgress,
		"task_status":     StatusRunning,
		"request_id":      requestID,
		"progress_status": progressStatus,
		"source":          "agent",
	}
	if stepName != "" {
		extra["step_name"] = stepName
	}
	if payload.Extra != nil {
		extra["progress_extra"] = payload.Extra
	}

	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskItem.TaskID,
		DeviceID:   taskItem.DeviceID,
		EventType:  EventTypeTaskProgress,
		TaskStatus: StatusRunning,
		StepName:   stepName,
		Message:    message,
		Extra:      extra,
		CreatedAt:  now,
	}, nil); err != nil {
		return Task{}, err
	}

	return s.GetByID(ctx, taskID)
}

// MarkResult 处理设备回传的 task_result，并推进任务到 success 或 failed。
func (s *Service) MarkResult(ctx context.Context, taskID string, payload ResultPayload, requestID string) (Task, error) {
	taskItem, err := s.GetByID(ctx, taskID)
	if err != nil {
		return Task{}, err
	}

	if requestID != "" {
		exists, err := s.eventExistsByRequestID(ctx, taskID, EventTypeTaskResult, requestID)
		if err != nil {
			return Task{}, err
		}
		if exists {
			return taskItem, nil
		}
	}

	status := strings.TrimSpace(payload.Status)
	if status != StatusSuccess {
		status = StatusFailed
	}

	now := time.Now().UTC().Format(time.RFC3339)
	stepName := strings.TrimSpace(payload.StepName)
	resultCode := strings.TrimSpace(payload.ResultCode)
	resultMessage := strings.TrimSpace(payload.ResultMessage)
	if resultMessage == "" {
		if status == StatusSuccess {
			resultMessage = "任务执行成功"
		} else {
			resultMessage = "任务执行失败"
		}
	}

	startedAt := taskItem.StartedAt
	if startedAt == "" {
		startedAt = now
	}
	if err := s.updateTaskState(ctx, taskID, status, stepName, resultCode, resultMessage, startedAt, now, now); err != nil {
		return Task{}, err
	}

	extra := map[string]any{
		"topic":          TopicTasks,
		"event_type":     EventTypeTaskResult,
		"task_status":    status,
		"request_id":     requestID,
		"result_code":    resultCode,
		"result_message": resultMessage,
		"source":         "agent",
	}
	if payload.Extra != nil {
		extra["result_extra"] = payload.Extra
	}
	if stepName != "" {
		extra["step_name"] = stepName
	}
	if err := s.appendEvent(ctx, Event{
		Topic:      TopicTasks,
		TaskID:     taskItem.TaskID,
		DeviceID:   taskItem.DeviceID,
		EventType:  EventTypeTaskResult,
		TaskStatus: status,
		StepName:   stepName,
		Message:    resultMessage,
		Extra:      extra,
		CreatedAt:  now,
	}, nil); err != nil {
		return Task{}, err
	}

	return s.GetByID(ctx, taskID)
}

func (s *Service) updateTaskState(
	ctx context.Context,
	taskID string,
	status string,
	currentStep string,
	resultCode string,
	resultMessage string,
	startedAt string,
	finishedAt string,
	updatedAt string,
) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET status = ?,
    current_step = ?,
    result_code = ?,
    result_message = ?,
    started_at = ?,
    finished_at = ?,
    updated_at = ?
WHERE id = ?`,
		status,
		currentStep,
		resultCode,
		resultMessage,
		startedAt,
		finishedAt,
		updatedAt,
		taskID,
	); err != nil {
		return fmt.Errorf("update task state: %w", err)
	}
	return nil
}

func (s *Service) eventExistsByRequestID(ctx context.Context, taskID string, eventType string, requestID string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT 1
FROM task_events
WHERE task_id = ?
  AND event_type = ?
  AND json_extract(extra_json, '$.request_id') = ?
LIMIT 1`,
		taskID,
		eventType,
		requestID,
	)

	var one int
	switch err := row.Scan(&one); {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("query task event exists: %w", err)
	}
}

func (s *Service) deviceExists(ctx context.Context, deviceID string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT 1 FROM devices WHERE id = ? LIMIT 1`, deviceID)

	var one int
	switch err := row.Scan(&one); {
	case err == nil:
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	default:
		return false, fmt.Errorf("query device exists: %w", err)
	}
}

func (s *Service) appendEvent(ctx context.Context, event Event, executor sqlExecutor) error {
	extraJSON, err := marshalJSONObject(event.Extra)
	if err != nil {
		return fmt.Errorf("marshal task event extra: %w", err)
	}

	if executor == nil {
		executor = s.db
	}

	if _, err := executor.ExecContext(ctx, `
INSERT INTO task_events (
    task_id, device_id, event_type, step_name, message, extra_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.TaskID,
		event.DeviceID,
		event.EventType,
		event.StepName,
		event.Message,
		extraJSON,
		event.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert task event: %w", err)
	}

	return nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (Task, error) {
	var (
		taskItem   Task
		paramsJSON string
		taskID     int64
		deviceID   int64
	)
	err := scanner.Scan(
		&taskID,
		&deviceID,
		&taskItem.ScriptName,
		&taskItem.ScriptVersion,
		&paramsJSON,
		&taskItem.Status,
		&taskItem.Priority,
		&taskItem.WorkflowNodeID,
		&taskItem.TaskSourceType,
		&taskItem.RetryCount,
		&taskItem.CurrentStep,
		&taskItem.ResultCode,
		&taskItem.ResultMessage,
		&taskItem.ScheduledAt,
		&taskItem.StartedAt,
		&taskItem.FinishedAt,
		&taskItem.CreatedAt,
		&taskItem.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, err
	}
	taskItem.TaskID = strconv.FormatInt(taskID, 10)
	taskItem.DeviceID = strconv.FormatInt(deviceID, 10)

	params, err := parseJSONObject(paramsJSON)
	if err != nil {
		return Task{}, fmt.Errorf("parse task params_json: %w", err)
	}
	taskItem.Params = params

	return taskItem, nil
}

func scanEvent(scanner taskScanner) (Event, error) {
	var (
		event     Event
		taskID    int64
		deviceID  int64
		extraJSON string
	)
	if err := scanner.Scan(
		&event.TaskEventID,
		&taskID,
		&deviceID,
		&event.EventType,
		&event.StepName,
		&event.Message,
		&extraJSON,
		&event.CreatedAt,
	); err != nil {
		return Event{}, err
	}
	event.TaskID = strconv.FormatInt(taskID, 10)
	event.DeviceID = strconv.FormatInt(deviceID, 10)

	extra, err := parseJSONObject(extraJSON)
	if err != nil {
		return Event{}, fmt.Errorf("parse task event extra_json: %w", err)
	}
	event.Extra = extra
	event.Topic = TopicTasks
	if status, ok := extra["task_status"].(string); ok {
		event.TaskStatus = status
	}

	return event, nil
}

func marshalJSONObject(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func parseJSONObject(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}

	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]any{}, nil
	}
	return value, nil
}

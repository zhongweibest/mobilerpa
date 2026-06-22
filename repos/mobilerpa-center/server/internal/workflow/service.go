package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
)

const (
	// DefinitionStatusDraft 表示工作流定义仍处于草稿状态。
	DefinitionStatusDraft = "draft"
	// DefinitionStatusActive 表示工作流定义已经可以启动运行。
	DefinitionStatusActive = "active"

	// RunStatusPending 表示运行实例已经创建，等待首个节点启动。
	RunStatusPending = "pending"
	// RunStatusRunning 表示运行实例正在执行。
	RunStatusRunning = "running"
	// RunStatusSuccess 表示运行实例已经成功结束。
	RunStatusSuccess = "success"
	// RunStatusFailed 表示运行实例已经失败结束。
	RunStatusFailed = "failed"
	// RunStatusStopped 表示运行实例已被人工停止。
	RunStatusStopped = "stopped"

	// NodeTypeScript 表示脚本执行节点。
	NodeTypeScript = "script"
	// NodeTypeLoop 表示循环控制节点。
	NodeTypeLoop = "loop"
	// NodeTypeStop 表示终止节点。
	NodeTypeStop = "stop"

	// EdgeTypeNext 表示普通顺序边。
	EdgeTypeNext = "next"
	// EdgeTypeLoopBody 表示循环体边。
	EdgeTypeLoopBody = "loop_body"
	// EdgeTypeLoopExit 表示循环退出边。
	EdgeTypeLoopExit = "loop_exit"

	// EventTypeWorkflowRunStarted 表示工作流运行实例已启动。
	EventTypeWorkflowRunStarted = "workflow_run_started"
	// EventTypeWorkflowStepStarted 表示工作流某个步骤开始执行。
	EventTypeWorkflowStepStarted = "workflow_step_started"
	// EventTypeWorkflowStepProgress 表示工作流某个步骤回传了关键进度。
	EventTypeWorkflowStepProgress = "workflow_step_progress"
	// EventTypeWorkflowStepSucceeded 表示工作流某个步骤执行成功。
	EventTypeWorkflowStepSucceeded = "workflow_step_succeeded"
	// EventTypeWorkflowStepFailed 表示工作流某个步骤执行失败。
	EventTypeWorkflowStepFailed = "workflow_step_failed"
	// EventTypeWorkflowLoopCompleted 表示某个循环节点完成了一轮。
	EventTypeWorkflowLoopCompleted = "workflow_loop_completed"
	// EventTypeWorkflowDeviceStopped 表示某台设备的工作流被手动停止。
	EventTypeWorkflowDeviceStopped = "workflow_device_stopped"
	// EventTypeWorkflowRunCompleted 表示工作流运行实例完成。
	EventTypeWorkflowRunCompleted = "workflow_run_completed"
	// EventTypeWorkflowRunFailed 表示工作流运行实例失败结束。
	EventTypeWorkflowRunFailed = "workflow_run_failed"
)

var (
	// ErrWorkflowDefinitionNotFound 表示工作流定义不存在。
	ErrWorkflowDefinitionNotFound = errors.New("workflow definition not found")
	// ErrWorkflowDefinitionNameRequired 表示缺少工作流名称。
	ErrWorkflowDefinitionNameRequired = errors.New("workflow_name is required")
	// ErrWorkflowDefinitionNodesRequired 表示工作流节点列表为空。
	ErrWorkflowDefinitionNodesRequired = errors.New("workflow nodes are required")
	// ErrWorkflowNodeIDRequired 表示缺少节点标识。
	ErrWorkflowNodeIDRequired = errors.New("workflow node_id is required")
	// ErrWorkflowNodeTypeUnsupported 表示节点类型不支持。
	ErrWorkflowNodeTypeUnsupported = errors.New("workflow node_type is unsupported")
	// ErrWorkflowScriptNameRequired 表示脚本节点缺少脚本名。
	ErrWorkflowScriptNameRequired = errors.New("workflow script_name is required")
	// ErrWorkflowDeviceIDsRequired 表示启动或追加设备时未传设备。
	ErrWorkflowDeviceIDsRequired = errors.New("device_ids are required")
	// ErrWorkflowDeviceBusy 表示设备已被其他主工作流占用。
	ErrWorkflowDeviceBusy = errors.New("device already running another workflow")
	// ErrWorkflowAnotherActive 表示已有其他主工作流在运行。
	ErrWorkflowAnotherActive = errors.New("another workflow is running")
	// ErrWorkflowRunNotFound 表示设备运行实例不存在。
	ErrWorkflowRunNotFound = errors.New("workflow run not found")
	// ErrWorkflowInstanceNotFound 表示工作流实例不存在。
	ErrWorkflowInstanceNotFound = errors.New("workflow instance not found")
	// ErrWorkflowInstanceNotActive 表示当前没有可追加设备的活动工作流实例。
	ErrWorkflowInstanceNotActive = errors.New("workflow instance not active")
	ErrWorkflowDefinitionRunning = errors.New("workflow definition still has active runs")
	ErrWorkflowInstanceDeleteNotAllowed = errors.New("workflow instance delete not allowed")
)

// DeviceBusyDetail 描述某台设备被任务或工作流占用的原因。
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

// DeviceBusyError 表示一组设备中存在被占用的设备。
type DeviceBusyError struct {
	Details []DeviceBusyDetail
}

func (e *DeviceBusyError) Error() string {
	return ErrWorkflowDeviceBusy.Error()
}

func (e *DeviceBusyError) Unwrap() error {
	return ErrWorkflowDeviceBusy
}

// TaskDispatcher 定义工作流触发任务下发所需的最小能力。
type TaskDispatcher interface {
	AssignTask(ctx context.Context, taskID string) (task.Task, error)
}

// Definition 表示工作流定义。
type Definition struct {
	WorkflowDefID string `json:"workflow_def_id"`
	WorkflowName  string `json:"workflow_name"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Node 表示工作流节点定义。
type Node struct {
	WorkflowDefID string `json:"workflow_def_id"`
	NodeID        string `json:"node_id"`
	NodeType      string `json:"node_type"`
	NodeName      string `json:"node_name"`
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
	MaxIterations int    `json:"max_iterations"`
	Position      int    `json:"position"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Edge 表示工作流节点之间的连线关系。
type Edge struct {
	WorkflowDefID string `json:"workflow_def_id"`
	FromNodeID    string `json:"from_node_id"`
	ToNodeID      string `json:"to_node_id"`
	EdgeType      string `json:"edge_type"`
	CreatedAt     string `json:"created_at"`
}

// Run 表示单设备工作流运行实例。
type Run struct {
	WorkflowRunID string `json:"workflow_run_id"`
	WorkflowInstanceID string `json:"workflow_instance_id"`
	WorkflowDefID string `json:"workflow_def_id"`
	DeviceID      string `json:"device_id"`
	Status        string `json:"status"`
	CurrentNodeID string `json:"current_node_id"`
	CurrentTaskID string `json:"current_task_id"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	LastError     string `json:"last_error"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Instance 表示一个工作流实例，实例下可挂多台设备执行。
type Instance struct {
	WorkflowInstanceID string `json:"workflow_instance_id"`
	WorkflowDefID      string `json:"workflow_def_id"`
	WorkflowName       string `json:"workflow_name"`
	Status             string `json:"status"`
	StartedAt          string `json:"started_at"`
	FinishedAt         string `json:"finished_at"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	DeviceRuns         []Run  `json:"device_runs"`
}

// Event 表示工作流运行域中的标准化事件记录。
type Event struct {
	WorkflowEventID   int64          `json:"workflow_event_id"`
	WorkflowInstanceID string    `json:"workflow_instance_id"`
	WorkflowRunID string         `json:"workflow_run_id"`
	WorkflowDefID string         `json:"workflow_def_id"`
	DeviceID      string         `json:"device_id"`
	NodeID        string         `json:"node_id"`
	EventType     string         `json:"event_type"`
	Message       string         `json:"message"`
	Extra         map[string]any `json:"extra"`
	CreatedAt     string         `json:"created_at"`
}

// StepProgressPayload 描述设备回传的 workflow_step_progress 载荷。
type StepProgressPayload struct {
	WorkflowRunID string         `json:"workflow_run_id"`
	WorkflowNodeID string        `json:"workflow_node_id"`
	TaskID        string         `json:"task_id"`
	Status        string         `json:"status"`
	StepName      string         `json:"step_name"`
	Message       string         `json:"message"`
	Extra         map[string]any `json:"extra"`
}

// CreateDefinitionRequest 描述创建工作流定义的请求。
type CreateDefinitionRequest struct {
	WorkflowName string `json:"workflow_name"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Nodes        []Node `json:"nodes"`
	Edges        []Edge `json:"edges"`
}

// StartRequest 描述启动工作流时的请求。
type StartRequest struct {
	DeviceIDs []string `json:"device_ids"`
}

// AddDevicesRequest 描述运行中追加设备的请求。
type AddDevicesRequest struct {
	WorkflowInstanceID string   `json:"workflow_instance_id"`
	DeviceIDs          []string `json:"device_ids"`
}

// Service 负责工作流定义、运行实例与编排推进。
type Service struct {
	db         *sql.DB
	devices    *device.Service
	tasks      *task.Service
	dispatcher TaskDispatcher
}

// NewService 创建工作流服务。
func NewService(db *sql.DB, devices *device.Service, tasks *task.Service, dispatcher TaskDispatcher) *Service {
	return &Service{
		db:         db,
		devices:    devices,
		tasks:      tasks,
		dispatcher: dispatcher,
	}
}

// CreateDefinition 创建新的工作流定义。
func (s *Service) CreateDefinition(ctx context.Context, req CreateDefinitionRequest) (Definition, error) {
	req.WorkflowName = strings.TrimSpace(req.WorkflowName)
	req.Description = strings.TrimSpace(req.Description)
	req.Status = strings.TrimSpace(req.Status)
	if req.WorkflowName == "" {
		return Definition{}, ErrWorkflowDefinitionNameRequired
	}
	if len(req.Nodes) == 0 {
		return Definition{}, ErrWorkflowDefinitionNodesRequired
	}
	if req.Status == "" {
		req.Status = DefinitionStatusDraft
	}

	for index := range req.Nodes {
		req.Nodes[index].NodeID = strings.TrimSpace(req.Nodes[index].NodeID)
		req.Nodes[index].NodeType = strings.TrimSpace(req.Nodes[index].NodeType)
		req.Nodes[index].NodeName = strings.TrimSpace(req.Nodes[index].NodeName)
		req.Nodes[index].ScriptName = strings.TrimSpace(req.Nodes[index].ScriptName)
		req.Nodes[index].ScriptVersion = strings.TrimSpace(req.Nodes[index].ScriptVersion)
		if req.Nodes[index].NodeID == "" {
			return Definition{}, ErrWorkflowNodeIDRequired
		}
		switch req.Nodes[index].NodeType {
		case NodeTypeScript:
			if req.Nodes[index].ScriptName == "" {
				return Definition{}, ErrWorkflowScriptNameRequired
			}
		case NodeTypeLoop, NodeTypeStop:
		default:
			return Definition{}, ErrWorkflowNodeTypeUnsupported
		}
		req.Nodes[index].Position = index + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Definition{}, fmt.Errorf("begin workflow definition tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO workflow_defs (
    workflow_name, description, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?)`,
		req.WorkflowName,
		req.Description,
		req.Status,
		now,
		now,
	)
	if err != nil {
		return Definition{}, fmt.Errorf("insert workflow definition: %w", err)
	}

	insertedID, err := result.LastInsertId()
	if err != nil {
		return Definition{}, fmt.Errorf("read inserted workflow definition id: %w", err)
	}
	workflowDefID := strconv.FormatInt(insertedID, 10)

	for _, node := range req.Nodes {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_nodes (
    workflow_def_id, node_id, node_type, node_name, script_name, script_version,
    max_iterations, position, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			workflowDefID,
			node.NodeID,
			node.NodeType,
			node.NodeName,
			node.ScriptName,
			node.ScriptVersion,
			node.MaxIterations,
			node.Position,
			now,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert workflow node %s: %w", node.NodeID, err)
		}
	}

	for _, edge := range req.Edges {
		edge.FromNodeID = strings.TrimSpace(edge.FromNodeID)
		edge.ToNodeID = strings.TrimSpace(edge.ToNodeID)
		edge.EdgeType = strings.TrimSpace(edge.EdgeType)
		if edge.EdgeType == "" {
			edge.EdgeType = EdgeTypeNext
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO workflow_edges (
    workflow_def_id, from_node_id, to_node_id, edge_type, created_at
) VALUES (?, ?, ?, ?, ?)`,
			workflowDefID,
			edge.FromNodeID,
			edge.ToNodeID,
			edge.EdgeType,
			now,
		); err != nil {
			return Definition{}, fmt.Errorf("insert workflow edge %s -> %s: %w", edge.FromNodeID, edge.ToNodeID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Definition{}, fmt.Errorf("commit workflow definition tx: %w", err)
	}
	tx = nil

	return s.GetDefinition(ctx, workflowDefID)
}

// ListDefinitions 返回工作流定义列表。
func (s *Service) ListDefinitions(ctx context.Context) ([]Definition, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS workflow_def_id, workflow_name, description, status, created_at, updated_at
FROM workflow_defs
ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query workflow definitions: %w", err)
	}
	defer rows.Close()

	items := make([]Definition, 0)
	for rows.Next() {
		var item Definition
		if err := rows.Scan(
			&item.WorkflowDefID,
			&item.WorkflowName,
			&item.Description,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow definition: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow definitions: %w", err)
	}

	if len(items) == 0 {
		return items, nil
	}

	workflowDefIDs := make([]string, 0, len(items))
	for _, item := range items {
		workflowDefIDs = append(workflowDefIDs, item.WorkflowDefID)
	}

	nodesByDefinition, err := s.listNodesByDefinitions(ctx, workflowDefIDs)
	if err != nil {
		return nil, fmt.Errorf("list workflow definition nodes: %w", err)
	}
	edgesByDefinition, err := s.listEdgesByDefinitions(ctx, workflowDefIDs)
	if err != nil {
		return nil, fmt.Errorf("list workflow definition edges: %w", err)
	}

	for index := range items {
		items[index].Nodes = nodesByDefinition[items[index].WorkflowDefID]
		items[index].Edges = edgesByDefinition[items[index].WorkflowDefID]
	}

	return items, nil
}

// GetDefinition 返回单个工作流定义及其节点、边。
func (s *Service) GetDefinition(ctx context.Context, workflowDefID string) (Definition, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return Definition{}, ErrWorkflowDefinitionNotFound
	}

	var item Definition
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_def_id, workflow_name, description, status, created_at, updated_at
FROM workflow_defs
WHERE id = ?`,
		workflowDefID,
	)
	if err := row.Scan(
		&item.WorkflowDefID,
		&item.WorkflowName,
		&item.Description,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Definition{}, ErrWorkflowDefinitionNotFound
		}
		return Definition{}, fmt.Errorf("get workflow definition: %w", err)
	}

	nodes, err := s.listNodes(ctx, workflowDefID)
	if err != nil {
		return Definition{}, err
	}
	edges, err := s.listEdges(ctx, workflowDefID)
	if err != nil {
		return Definition{}, err
	}

	item.Nodes = nodes
	item.Edges = edges
	return item, nil
}

// Start 启动工作流，并为选中的设备创建运行实例。
func (s *Service) Start(ctx context.Context, workflowDefID string, req StartRequest) (Instance, error) {
	return s.addRuns(ctx, workflowDefID, req.DeviceIDs, "")
}

// AddDevices 为正在运行的工作流追加设备。
func (s *Service) AddDevices(ctx context.Context, workflowDefID string, req AddDevicesRequest) (Instance, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	req.WorkflowInstanceID = strings.TrimSpace(req.WorkflowInstanceID)
	if workflowDefID == "" {
		return Instance{}, ErrWorkflowDefinitionNotFound
	}
	if req.WorkflowInstanceID == "" {
		return Instance{}, ErrWorkflowInstanceNotFound
	}

	instance, err := s.GetInstance(ctx, req.WorkflowInstanceID)
	if err != nil {
		return Instance{}, err
	}
	if instance.WorkflowDefID != workflowDefID {
		return Instance{}, ErrWorkflowInstanceNotFound
	}
	if instance.Status != RunStatusPending && instance.Status != RunStatusRunning {
		return Instance{}, ErrWorkflowInstanceNotActive
	}
	return s.addRuns(ctx, workflowDefID, req.DeviceIDs, instance.WorkflowInstanceID)
}

// StopDefinition 停止某个工作流下的全部未结束运行实例。
func (s *Service) StopDefinition(ctx context.Context, workflowDefID string, workflowInstanceID string) (Instance, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	if workflowDefID == "" {
		return Instance{}, ErrWorkflowDefinitionNotFound
	}
	if workflowInstanceID == "" {
		return Instance{}, ErrWorkflowInstanceNotFound
	}

	instance, err := s.GetInstance(ctx, workflowInstanceID)
	if err != nil {
		return Instance{}, err
	}
	if instance.WorkflowDefID != workflowDefID {
		return Instance{}, ErrWorkflowInstanceNotFound
	}
	if instance.Status != RunStatusPending && instance.Status != RunStatusRunning {
		return Instance{}, ErrWorkflowInstanceNotActive
	}
	activeRuns := instance.DeviceRuns

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE workflow_instance_id = ?
  AND status IN (?, ?)`,
		RunStatusStopped,
		now,
		now,
		instance.WorkflowInstanceID,
		RunStatusPending,
		RunStatusRunning,
	); err != nil {
		return Instance{}, fmt.Errorf("stop workflow definition runs: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_instances
SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE workflow_instance_id = ?`,
		RunStatusStopped,
		now,
		now,
		instance.WorkflowInstanceID,
	); err != nil {
		return Instance{}, fmt.Errorf("stop workflow instance: %w", err)
	}

	for _, run := range activeRuns {
		if run.Status != RunStatusPending && run.Status != RunStatusRunning {
			continue
		}
		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, run.CurrentNodeID, EventTypeWorkflowDeviceStopped, "设备工作流已手动停止", map[string]any{
			"source": "center",
			"reason": "definition_stop",
		}); err != nil {
			return Instance{}, err
		}
	}

	return s.GetInstance(ctx, instance.WorkflowInstanceID)
}

// StopRunByDevice 停止某个设备的工作流运行实例。
func (s *Service) StopRunByDevice(ctx context.Context, workflowDefID string, workflowInstanceID string, deviceID string) (Run, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	deviceID = strings.TrimSpace(deviceID)
	if workflowDefID == "" || workflowInstanceID == "" || deviceID == "" {
		return Run{}, ErrWorkflowRunNotFound
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE workflow_def_id = ?
  AND workflow_instance_id = ?
  AND device_id = ?
  AND status IN (?, ?)`,
		RunStatusStopped,
		now,
		now,
		workflowDefID,
		workflowInstanceID,
		deviceID,
		RunStatusPending,
		RunStatusRunning,
	)
	if err != nil {
		return Run{}, fmt.Errorf("stop workflow run by device: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Run{}, fmt.Errorf("workflow run rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return Run{}, ErrWorkflowRunNotFound
	}

	run, err := s.getRunByDeviceInInstance(ctx, workflowDefID, workflowInstanceID, deviceID)
	if err != nil {
		return Run{}, err
	}

	if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, workflowDefID, deviceID, "", EventTypeWorkflowDeviceStopped, "设备工作流已手动停止", map[string]any{
		"source": "center",
		"reason": "device_stop",
	}); err != nil {
		return Run{}, err
	}

	if err := s.refreshInstanceStatus(ctx, run.WorkflowInstanceID); err != nil {
		return Run{}, err
	}
	return s.getRunByDeviceInInstance(ctx, workflowDefID, workflowInstanceID, deviceID)
}

// ListRuns 返回某个工作流定义下的运行实例列表。
func (s *Service) ListRuns(ctx context.Context, workflowDefID string) ([]Run, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return nil, ErrWorkflowDefinitionNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE workflow_def_id = ?
  ORDER BY id ASC`, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("query workflow runs: %w", err)
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
		return nil, fmt.Errorf("iterate workflow runs: %w", err)
	}
	return items, nil
}

// ListInstances 返回工作流实例列表。
func (s *Service) ListInstances(ctx context.Context, workflowDefID string) ([]Instance, error) {
	query := `
SELECT id AS workflow_instance_id, workflow_def_id, workflow_name, status, started_at, finished_at, created_at, updated_at
FROM workflow_instances`
	args := make([]any, 0, 1)
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID != "" {
		query += `
WHERE workflow_def_id = ?`
		args = append(args, workflowDefID)
	}
	query += `
ORDER BY id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow instances: %w", err)
	}
	defer rows.Close()

	items := make([]Instance, 0)
	for rows.Next() {
		item, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow instances: %w", err)
	}

	for index := range items {
		runs, err := s.listRunsByInstance(ctx, items[index].WorkflowInstanceID)
		if err != nil {
			return nil, err
		}
		items[index].DeviceRuns = runs
	}

	return items, nil
}

// GetInstance 返回单个工作流实例详情。
func (s *Service) GetInstance(ctx context.Context, workflowInstanceID string) (Instance, error) {
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	if workflowInstanceID == "" {
		return Instance{}, ErrWorkflowInstanceNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_instance_id, workflow_def_id, workflow_name, status, started_at, finished_at, created_at, updated_at
FROM workflow_instances
WHERE id = ?`, workflowInstanceID)

	item, err := scanInstance(row)
	if err != nil {
		return Instance{}, err
	}
	runs, err := s.listRunsByInstance(ctx, workflowInstanceID)
	if err != nil {
		return Instance{}, err
	}
	item.DeviceRuns = runs
	return item, nil
}

// GetRunByTaskID 根据任务 ID 返回对应的工作流运行记录。
func (s *Service) GetRunByTaskID(ctx context.Context, taskID string) (Run, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return Run{}, ErrWorkflowRunNotFound
	}

	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE current_task_id = ?
ORDER BY id DESC
LIMIT 1`, taskID)
	return scanRun(row)
}

// ListEvents 返回指定工作流定义下的工作流运行事件，可按运行实例过滤。
func (s *Service) ListEvents(ctx context.Context, workflowDefID string, workflowRunID string) ([]Event, error) {
	workflowDefID = strings.TrimSpace(workflowDefID)
	workflowRunID = strings.TrimSpace(workflowRunID)
	if workflowDefID == "" {
		return nil, ErrWorkflowDefinitionNotFound
	}
	if workflowRunID != "" {
		if _, err := s.getRunByID(ctx, workflowRunID); err != nil {
			return nil, err
		}
	}

	query := `
SELECT id, workflow_instance_id, workflow_run_id, workflow_def_id, device_id, node_id, event_type, message, extra_json, created_at
FROM workflow_events
WHERE workflow_def_id = ?`
	args := []any{workflowDefID}
	if workflowRunID != "" {
		query += `
  AND workflow_run_id = ?`
		args = append(args, workflowRunID)
	}
	query += `
ORDER BY id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow events: %w", err)
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
		return nil, fmt.Errorf("iterate workflow events: %w", err)
	}
	return items, nil
}

// HandleTaskResult 把任务结果推进到对应工作流运行实例。
func (s *Service) HandleTaskResult(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}

	taskInfo, err := s.lookupWorkflowTask(ctx, taskID)
	if err != nil {
		return err
	}
	if taskInfo.WorkflowRunID == "" || taskInfo.WorkflowDefID == "" {
		return nil
	}

	run, err := s.getRunByID(ctx, taskInfo.WorkflowRunID)
	if err != nil {
		return err
	}
	if run.Status != RunStatusRunning && run.Status != RunStatusPending {
		return nil
	}

	if taskInfo.Status == task.StatusSuccess {
		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, taskInfo.WorkflowNodeID, EventTypeWorkflowStepSucceeded, "工作流步骤执行成功", map[string]any{
			"task_id":        taskInfo.TaskID,
			"result_message": taskInfo.ResultMessage,
			"source":         "center",
		}); err != nil {
			return err
		}
		return s.advanceRunAfterNode(ctx, run, taskInfo.WorkflowNodeID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, last_error = ?, finished_at = CASE WHEN finished_at = '' THEN ? ELSE finished_at END, updated_at = ?
WHERE id = ?`,
		RunStatusFailed,
		taskInfo.ResultMessage,
		now,
		now,
		run.WorkflowRunID,
	); err != nil {
		return fmt.Errorf("mark workflow run failed: %w", err)
	}

	if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, taskInfo.WorkflowNodeID, EventTypeWorkflowStepFailed, "工作流步骤执行失败", map[string]any{
		"task_id":        taskInfo.TaskID,
		"result_message": taskInfo.ResultMessage,
		"source":         "center",
	}); err != nil {
		return err
	}
	if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, taskInfo.WorkflowNodeID, EventTypeWorkflowRunFailed, "工作流运行失败结束", map[string]any{
		"task_id":        taskInfo.TaskID,
		"result_message": taskInfo.ResultMessage,
		"source":         "center",
	}); err != nil {
		return err
	}
	return s.refreshInstanceStatus(ctx, run.WorkflowInstanceID)
}

// HandleStepProgress 处理设备回传的 workflow_step_progress，并写入工作流事件域。
func (s *Service) HandleStepProgress(ctx context.Context, payload StepProgressPayload, requestID string, deviceID string) error {
	payload.WorkflowRunID = strings.TrimSpace(payload.WorkflowRunID)
	payload.WorkflowNodeID = strings.TrimSpace(payload.WorkflowNodeID)
	payload.TaskID = strings.TrimSpace(payload.TaskID)
	payload.Status = strings.TrimSpace(payload.Status)
	payload.StepName = strings.TrimSpace(payload.StepName)
	payload.Message = strings.TrimSpace(payload.Message)
	deviceID = strings.TrimSpace(deviceID)

	if payload.WorkflowRunID == "" {
		return ErrWorkflowRunNotFound
	}

	run, err := s.getRunByID(ctx, payload.WorkflowRunID)
	if err != nil {
		return err
	}

	if payload.WorkflowNodeID == "" {
		payload.WorkflowNodeID = run.CurrentNodeID
	}
	if deviceID == "" {
		deviceID = run.DeviceID
	}

	message := payload.Message
	if message == "" {
		if payload.StepName != "" {
			message = "工作流步骤执行中：" + payload.StepName
		} else {
			message = "工作流步骤执行中"
		}
	}
	if payload.Status == "" {
		payload.Status = "running"
	}

	extra := map[string]any{
		"source":     "agent",
		"request_id": requestID,
		"task_id":    payload.TaskID,
		"status":     payload.Status,
		"step_name":  payload.StepName,
	}
	for key, value := range payload.Extra {
		extra[key] = value
	}

	return s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, deviceID, payload.WorkflowNodeID, EventTypeWorkflowStepProgress, message, extra)
}

func (s *Service) addRuns(ctx context.Context, workflowDefID string, deviceIDs []string, workflowInstanceID string) (Instance, error) {
	definition, err := s.GetDefinition(ctx, workflowDefID)
	if err != nil {
		return Instance{}, err
	}

	cleanDeviceIDs := make([]string, 0, len(deviceIDs))
	for _, item := range deviceIDs {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		cleanDeviceIDs = append(cleanDeviceIDs, item)
	}
	if len(cleanDeviceIDs) == 0 {
		return Instance{}, ErrWorkflowDeviceIDsRequired
	}

	if workflowInstanceID == "" {
		if err := s.ensureNoOtherActiveWorkflow(ctx, workflowDefID); err != nil {
			return Instance{}, err
		}
	}

	headNode, err := s.findHeadNode(ctx, workflowDefID)
	if err != nil {
		return Instance{}, err
	}

	busyDetails := make([]DeviceBusyDetail, 0)
	for _, deviceID := range cleanDeviceIDs {
		detail, err := s.inspectDeviceAvailability(ctx, workflowDefID, workflowInstanceID, deviceID)
		if err != nil {
			return Instance{}, err
		}
		if detail != nil {
			busyDetails = append(busyDetails, *detail)
			continue
		}
		if s.devices != nil {
			if err := s.devices.EnsureExecutionReady(ctx, deviceID); err != nil {
				return Instance{}, err
			}
		}
	}
	if len(busyDetails) > 0 {
		return Instance{}, &DeviceBusyError{Details: busyDetails}
	}

	if workflowInstanceID == "" {
		workflowInstanceID, err = s.createInstance(ctx, definition, headNode)
		if err != nil {
			return Instance{}, err
		}
	}

	for _, deviceID := range cleanDeviceIDs {
		run, err := s.createRun(ctx, workflowInstanceID, definition, headNode, deviceID)
		if err != nil {
			return Instance{}, err
		}

		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, headNode.NodeID, EventTypeWorkflowRunStarted, "工作流运行已启动", map[string]any{
			"source":          "center",
			"workflow_name":   definition.WorkflowName,
			"start_node_id":   headNode.NodeID,
			"start_node_type": headNode.NodeType,
		}); err != nil {
			return Instance{}, err
		}

		if err := s.startRunFromNode(ctx, run.WorkflowRunID, headNode.NodeID); err != nil {
			return Instance{}, err
		}
	}

	return s.GetInstance(ctx, workflowInstanceID)
}

func (s *Service) createInstance(ctx context.Context, definition Definition, headNode Node) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_instances (
    workflow_def_id, workflow_name, status, started_at, finished_at, created_at, updated_at
) VALUES (?, ?, ?, ?, '', ?, ?)`,
		definition.WorkflowDefID,
		definition.WorkflowName,
		RunStatusRunning,
		now,
		now,
		now,
	)
	if err != nil {
		return "", fmt.Errorf("insert workflow instance: %w", err)
	}
	insertedID, err := result.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("read inserted workflow instance id: %w", err)
	}
	instanceID := strconv.FormatInt(insertedID, 10)
	return instanceID, nil
}

func (s *Service) createRun(ctx context.Context, workflowInstanceID string, definition Definition, headNode Node, deviceID string) (Run, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_runs (
    workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
    started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, 0, ?, '', '', ?, ?)`,
		workflowInstanceID,
		definition.WorkflowDefID,
		deviceID,
		RunStatusPending,
		headNode.NodeID,
		now,
		now,
		now,
	)
	if err != nil {
		return Run{}, fmt.Errorf("insert workflow run: %w", err)
	}
	insertedID, err := result.LastInsertId()
	if err != nil {
		return Run{}, fmt.Errorf("read inserted workflow run id: %w", err)
	}
	runID := strconv.FormatInt(insertedID, 10)

	return s.getRunByID(ctx, runID)
}

func (s *Service) startRunFromNode(ctx context.Context, workflowRunID string, nodeID string) error {
	run, err := s.getRunByID(ctx, workflowRunID)
	if err != nil {
		return err
	}

	node, err := s.getNode(ctx, run.WorkflowDefID, nodeID)
	if err != nil {
		return err
	}

	switch node.NodeType {
	case NodeTypeScript:
		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, node.NodeID, EventTypeWorkflowStepStarted, "工作流步骤开始执行", map[string]any{
			"source":         "center",
			"node_name":      node.NodeName,
			"script_name":    node.ScriptName,
			"script_version": node.ScriptVersion,
		}); err != nil {
			return err
		}

		createReq := task.CreateRequest{
			DeviceID:      run.DeviceID,
			ScriptName:    node.ScriptName,
			ScriptVersion: node.ScriptVersion,
		}
		createdTask, err := s.tasks.Create(ctx, createReq)
		if err != nil {
			return fmt.Errorf("create workflow task: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET workflow_instance_id = ?, workflow_run_id = ?, workflow_node_id = ?, task_source_type = 'workflow'
WHERE id = ?`,
			run.WorkflowInstanceID,
			run.WorkflowRunID,
			node.NodeID,
			createdTask.TaskID,
		); err != nil {
			return fmt.Errorf("bind workflow task metadata: %w", err)
		}

		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, current_node_id = ?, current_task_id = ?, updated_at = ?
WHERE id = ?`,
			RunStatusRunning,
			node.NodeID,
			createdTask.TaskID,
			now,
			run.WorkflowRunID,
		); err != nil {
			return fmt.Errorf("update workflow run current task: %w", err)
		}

		if _, err := s.dispatcher.AssignTask(ctx, createdTask.TaskID); err != nil {
			return fmt.Errorf("assign workflow task: %w", err)
		}
		return nil

	case NodeTypeLoop:
		return s.advanceRunAfterLoopNode(ctx, run, node)

	case NodeTypeStop:
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, current_node_id = ?, current_task_id = '', finished_at = ?, updated_at = ?
WHERE id = ?`,
			RunStatusSuccess,
			node.NodeID,
			now,
			now,
			run.WorkflowRunID,
		); err != nil {
			return fmt.Errorf("finish workflow run on stop node: %w", err)
		}

		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, node.NodeID, EventTypeWorkflowRunCompleted, "工作流运行已完成", map[string]any{
			"source":    "center",
			"stop_node": node.NodeID,
		}); err != nil {
			return err
		}
		return s.refreshInstanceStatus(ctx, run.WorkflowInstanceID)

	default:
		return ErrWorkflowNodeTypeUnsupported
	}
}

func (s *Service) advanceRunAfterNode(ctx context.Context, run Run, completedNodeID string) error {
	nextNodeID, err := s.getNextNodeID(ctx, run.WorkflowDefID, completedNodeID, EdgeTypeNext)
	if err != nil {
		return err
	}
	if nextNodeID == "" {
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := s.db.ExecContext(ctx, `
UPDATE workflow_runs
SET status = ?, current_task_id = '', finished_at = ?, updated_at = ?
WHERE id = ?`,
			RunStatusSuccess,
			now,
			now,
			run.WorkflowRunID,
		); err != nil {
			return fmt.Errorf("finish workflow run without next node: %w", err)
		}

		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, completedNodeID, EventTypeWorkflowRunCompleted, "工作流运行已完成", map[string]any{
			"source":          "center",
			"completed_node":  completedNodeID,
			"completion_mode": "no_next_node",
		}); err != nil {
			return err
		}
		return s.refreshInstanceStatus(ctx, run.WorkflowInstanceID)
	}
	return s.startRunFromNode(ctx, run.WorkflowRunID, nextNodeID)
}

func (s *Service) advanceRunAfterLoopNode(ctx context.Context, run Run, loopNode Node) error {
	counter, err := s.getLoopCounter(ctx, run.WorkflowRunID, loopNode.NodeID)
	if err != nil {
		return err
	}
	var targetEdgeType string
	if loopNode.MaxIterations > 0 && counter >= loopNode.MaxIterations {
		targetEdgeType = EdgeTypeLoopExit
	} else {
		counter += 1

		if err := s.saveLoopCounter(ctx, run.WorkflowRunID, loopNode.NodeID, counter); err != nil {
			return err
		}

		if err := s.appendEvent(ctx, run.WorkflowInstanceID, run.WorkflowRunID, run.WorkflowDefID, run.DeviceID, loopNode.NodeID, EventTypeWorkflowLoopCompleted, "工作流循环节点已完成一轮", map[string]any{
			"source":         "center",
			"loop_node_id":   loopNode.NodeID,
			"loop_node_name": loopNode.NodeName,
			"counter":        counter,
			"max_iterations": loopNode.MaxIterations,
		}); err != nil {
			return err
		}

		targetEdgeType = EdgeTypeLoopBody
	}

	nextNodeID, err := s.getNextNodeID(ctx, run.WorkflowDefID, loopNode.NodeID, targetEdgeType)
	if err != nil {
		return err
	}
	if nextNodeID == "" && targetEdgeType != EdgeTypeNext {
		nextNodeID, err = s.getNextNodeID(ctx, run.WorkflowDefID, loopNode.NodeID, EdgeTypeNext)
		if err != nil {
			return err
		}
	}
	if nextNodeID == "" {
		return nil
	}
	return s.startRunFromNode(ctx, run.WorkflowRunID, nextNodeID)
}

func (s *Service) ensureNoOtherActiveWorkflow(ctx context.Context, workflowDefID string) error {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT workflow_def_id
FROM workflow_instances
WHERE status IN (?, ?)`,
		RunStatusPending,
		RunStatusRunning,
	)
	if err != nil {
		return fmt.Errorf("query active workflows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var activeDefinitionID string
		if err := rows.Scan(&activeDefinitionID); err != nil {
			return fmt.Errorf("scan active workflow definition: %w", err)
		}
		if activeDefinitionID != workflowDefID {
			return ErrWorkflowAnotherActive
		}
	}
	return rows.Err()
}

func (s *Service) ensureDeviceAvailable(ctx context.Context, workflowDefID string, workflowInstanceID string, deviceID string) error {
	detail, err := s.inspectDeviceAvailability(ctx, workflowDefID, workflowInstanceID, deviceID)
	if err != nil {
		return err
	}
	if detail != nil {
		return &DeviceBusyError{Details: []DeviceBusyDetail{*detail}}
	}
	return nil
}

func (s *Service) inspectDeviceAvailability(ctx context.Context, workflowDefID string, workflowInstanceID string, deviceID string) (*DeviceBusyDetail, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, status
FROM workflow_runs
WHERE device_id = ?
  AND status IN (?, ?)
ORDER BY id DESC
LIMIT 1`,
		deviceID,
		RunStatusPending,
		RunStatusRunning,
	)

	var workflowRunID string
	var activeInstanceID string
	var activeDefinitionID string
	var activeStatus string
	switch err := row.Scan(&workflowRunID, &activeInstanceID, &activeDefinitionID, &activeStatus); {
	case err == nil:
		return &DeviceBusyDetail{
			DeviceID:           deviceID,
			OccupancyType:      "workflow",
			WorkflowDefID:      activeDefinitionID,
			WorkflowInstanceID: activeInstanceID,
			WorkflowRunID:      workflowRunID,
			TaskStatus:         activeStatus,
			Message:            "设备当前被工作流占用",
		}, nil
	case errors.Is(err, sql.ErrNoRows):
	default:
		return nil, fmt.Errorf("query device active workflow: %w", err)
	}

	if strings.TrimSpace(workflowInstanceID) != "" {
		row = s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM workflow_runs
WHERE workflow_instance_id = ?
  AND device_id = ?`,
			workflowInstanceID,
			deviceID,
		)
		var instanceRunCount int
		if err := row.Scan(&instanceRunCount); err != nil {
			return nil, fmt.Errorf("query device workflow instance history: %w", err)
		}
		if instanceRunCount > 0 {
			return &DeviceBusyDetail{
				DeviceID:           deviceID,
				OccupancyType:      "workflow_instance_history",
				WorkflowDefID:      workflowDefID,
				WorkflowInstanceID: workflowInstanceID,
				Message:            "设备已经执行过当前工作流实例，不能重复追加",
			}, nil
		}
	}

	row = s.db.QueryRowContext(ctx, `
SELECT id AS task_id, status
FROM tasks
WHERE device_id = ?
  AND task_source_type = 'manual'
  AND status IN (?, ?)
ORDER BY id DESC
LIMIT 1`,
		deviceID,
		task.StatusAssigned,
		task.StatusRunning,
	)
	var taskID string
	var taskStatus string
	switch err := row.Scan(&taskID, &taskStatus); {
	case err == nil:
		return &DeviceBusyDetail{
			DeviceID:      deviceID,
			OccupancyType: "manual_task",
			TaskID:        taskID,
			TaskStatus:    taskStatus,
			Message:       "设备当前被手工任务占用",
		}, nil
	case errors.Is(err, sql.ErrNoRows):
	default:
		return nil, fmt.Errorf("query device manual task occupancy: %w", err)
	}

	return nil, nil
}

// GetDeviceBusyDetail 返回某台设备当前是否被工作流或手工任务占用。
func (s *Service) GetDeviceBusyDetail(ctx context.Context, deviceID string) (*DeviceBusyDetail, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	return s.inspectDeviceAvailability(ctx, "", "", strings.TrimSpace(deviceID))
}

func (s *Service) listActiveRunsByInstance(ctx context.Context, workflowInstanceID string) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE workflow_instance_id = ?
  AND status IN (?, ?)
ORDER BY id ASC`,
		workflowInstanceID,
		RunStatusPending,
		RunStatusRunning,
	)
	if err != nil {
		return nil, fmt.Errorf("query active workflow runs by instance: %w", err)
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
		return nil, fmt.Errorf("iterate active workflow runs by instance: %w", err)
	}
	return items, nil
}

func (s *Service) findHeadNode(ctx context.Context, workflowDefID string) (Node, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT n.workflow_def_id, n.node_id, n.node_type, n.node_name, n.script_name, n.script_version,
       n.max_iterations, n.position, n.created_at, n.updated_at
FROM workflow_nodes n
WHERE n.workflow_def_id = ?
  AND NOT EXISTS (
      SELECT 1
      FROM workflow_edges e
      WHERE e.workflow_def_id = n.workflow_def_id
        AND e.to_node_id = n.node_id
  )
ORDER BY n.position ASC
LIMIT 1`, workflowDefID)
	if err != nil {
		return Node{}, fmt.Errorf("query workflow head node: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return scanNode(rows)
	}
	if err := rows.Err(); err != nil {
		return Node{}, fmt.Errorf("iterate workflow head node: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id = ?
ORDER BY position ASC
LIMIT 1`, workflowDefID)
	return scanNode(row)
}

func (s *Service) getNode(ctx context.Context, workflowDefID string, nodeID string) (Node, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id = ? AND node_id = ?`,
		workflowDefID,
		nodeID,
	)
	return scanNode(row)
}

func (s *Service) getNextNodeID(ctx context.Context, workflowDefID string, fromNodeID string, edgeType string) (string, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT to_node_id
FROM workflow_edges
WHERE workflow_def_id = ?
  AND from_node_id = ?
  AND edge_type = ?
ORDER BY id ASC
LIMIT 1`,
		workflowDefID,
		fromNodeID,
		edgeType,
	)

	var nextNodeID string
	switch err := row.Scan(&nextNodeID); {
	case err == nil:
		return nextNodeID, nil
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	default:
		return "", fmt.Errorf("query next workflow node: %w", err)
	}
}

func (s *Service) getLoopCounter(ctx context.Context, workflowRunID string, nodeID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT context_json
FROM workflow_contexts
WHERE workflow_run_id = ?
  AND node_id = ?`,
		workflowRunID,
		nodeID,
	)

	var contextJSON string
	switch err := row.Scan(&contextJSON); {
	case err == nil:
		var payload struct {
			Iteration int `json:"iteration"`
		}
		if err := json.Unmarshal([]byte(contextJSON), &payload); err != nil {
			return 0, fmt.Errorf("parse workflow context json: %w", err)
		}
		return payload.Iteration, nil
	case errors.Is(err, sql.ErrNoRows):
		return 0, nil
	default:
		return 0, fmt.Errorf("query workflow loop context: %w", err)
	}
}

func (s *Service) saveLoopCounter(ctx context.Context, workflowRunID string, nodeID string, counter int) error {
	contextJSON, err := json.Marshal(map[string]any{
		"iteration": counter,
	})
	if err != nil {
		return fmt.Errorf("marshal workflow loop context: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_contexts (
    workflow_run_id, node_id, context_json, updated_at
) VALUES (?, ?, ?, ?)
ON CONFLICT(workflow_run_id, node_id)
DO UPDATE SET context_json = excluded.context_json, updated_at = excluded.updated_at`,
		workflowRunID,
		nodeID,
		string(contextJSON),
		now,
	); err != nil {
		return fmt.Errorf("save workflow loop context: %w", err)
	}
	return nil
}

func (s *Service) listNodes(ctx context.Context, workflowDefID string) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id = ?
ORDER BY position ASC`, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("query workflow nodes: %w", err)
	}
	defer rows.Close()

	items := make([]Node, 0)
	for rows.Next() {
		item, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow nodes: %w", err)
	}
	return items, nil
}

func (s *Service) listNodesByDefinitions(ctx context.Context, workflowDefIDs []string) (map[string][]Node, error) {
	result := make(map[string][]Node, len(workflowDefIDs))
	if len(workflowDefIDs) == 0 {
		return result, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(workflowDefIDs)), ",")
	args := make([]any, 0, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		args = append(args, workflowDefID)
		result[workflowDefID] = []Node{}
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT workflow_def_id, node_id, node_type, node_name, script_name, script_version,
       max_iterations, position, created_at, updated_at
FROM workflow_nodes
WHERE workflow_def_id IN (%s)
ORDER BY workflow_def_id ASC, position ASC`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow nodes by definitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item Node
		if err := rows.Scan(
			&item.WorkflowDefID,
			&item.NodeID,
			&item.NodeType,
			&item.NodeName,
			&item.ScriptName,
			&item.ScriptVersion,
			&item.MaxIterations,
			&item.Position,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow node by definitions: %w", err)
		}
		result[item.WorkflowDefID] = append(result[item.WorkflowDefID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow nodes by definitions: %w", err)
	}

	return result, nil
}

func (s *Service) listEdges(ctx context.Context, workflowDefID string) ([]Edge, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workflow_def_id, from_node_id, to_node_id, edge_type, created_at
FROM workflow_edges
WHERE workflow_def_id = ?
ORDER BY id ASC`, workflowDefID)
	if err != nil {
		return nil, fmt.Errorf("query workflow edges: %w", err)
	}
	defer rows.Close()

	items := make([]Edge, 0)
	for rows.Next() {
		var item Edge
		if err := rows.Scan(
			&item.WorkflowDefID,
			&item.FromNodeID,
			&item.ToNodeID,
			&item.EdgeType,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow edge: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow edges: %w", err)
	}
	return items, nil
}

func (s *Service) listEdgesByDefinitions(ctx context.Context, workflowDefIDs []string) (map[string][]Edge, error) {
	result := make(map[string][]Edge, len(workflowDefIDs))
	if len(workflowDefIDs) == 0 {
		return result, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(workflowDefIDs)), ",")
	args := make([]any, 0, len(workflowDefIDs))
	for _, workflowDefID := range workflowDefIDs {
		args = append(args, workflowDefID)
		result[workflowDefID] = []Edge{}
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
SELECT workflow_def_id, from_node_id, to_node_id, edge_type, created_at
FROM workflow_edges
WHERE workflow_def_id IN (%s)
ORDER BY workflow_def_id ASC, id ASC`, placeholders), args...)
	if err != nil {
		return nil, fmt.Errorf("query workflow edges by definitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item Edge
		if err := rows.Scan(
			&item.WorkflowDefID,
			&item.FromNodeID,
			&item.ToNodeID,
			&item.EdgeType,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow edge by definitions: %w", err)
		}
		result[item.WorkflowDefID] = append(result[item.WorkflowDefID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow edges by definitions: %w", err)
	}

	return result, nil
}

func (s *Service) getRunByID(ctx context.Context, workflowRunID string) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE id = ?`, workflowRunID)
	return scanRun(row)
}

func (s *Service) getRunByDevice(ctx context.Context, workflowDefID string, deviceID string) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE workflow_def_id = ?
  AND device_id = ?
ORDER BY id DESC
LIMIT 1`,
		workflowDefID,
		deviceID,
	)
	return scanRun(row)
}

func (s *Service) getRunByDeviceInInstance(ctx context.Context, workflowDefID string, workflowInstanceID string, deviceID string) (Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE workflow_def_id = ?
  AND workflow_instance_id = ?
  AND device_id = ?
ORDER BY id DESC
LIMIT 1`,
		workflowDefID,
		workflowInstanceID,
		deviceID,
	)
	return scanRun(row)
}

func (s *Service) listRunsByInstance(ctx context.Context, workflowInstanceID string) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id AS workflow_run_id, workflow_instance_id, workflow_def_id, device_id, status, current_node_id, current_task_id,
       started_at, finished_at, last_error, created_at, updated_at
FROM workflow_runs
WHERE workflow_instance_id = ?
ORDER BY id DESC`, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("query workflow runs by instance: %w", err)
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
		return nil, fmt.Errorf("iterate workflow runs by instance: %w", err)
	}
	return items, nil
}

func (s *Service) getLatestActiveInstanceByDefinition(ctx context.Context, workflowDefID string) (Instance, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id AS workflow_instance_id, workflow_def_id, workflow_name, status, started_at, finished_at, created_at, updated_at
FROM workflow_instances
WHERE workflow_def_id = ?
  AND status IN (?, ?)
ORDER BY id DESC
LIMIT 1`,
		workflowDefID,
		RunStatusPending,
		RunStatusRunning,
	)
	instance, err := scanInstance(row)
	if err != nil {
		if errors.Is(err, ErrWorkflowInstanceNotFound) {
			return Instance{}, ErrWorkflowInstanceNotFound
		}
		return Instance{}, err
	}
	runs, err := s.listRunsByInstance(ctx, instance.WorkflowInstanceID)
	if err != nil {
		return Instance{}, err
	}
	instance.DeviceRuns = runs
	return instance, nil
}

func (s *Service) lookupWorkflowTask(ctx context.Context, taskID string) (struct {
	TaskID         string
	WorkflowRunID  string
	WorkflowNodeID string
	WorkflowDefID  string
	Status         string
	ResultMessage  string
}, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT t.id AS task_id, t.workflow_run_id, t.workflow_node_id, COALESCE(r.workflow_def_id, 0), t.status, t.result_message
FROM tasks t
LEFT JOIN workflow_runs r
  ON r.id = t.workflow_run_id
WHERE t.id = ?`, taskID)

	var item struct {
		TaskID         string
		WorkflowRunID  string
		WorkflowNodeID string
		WorkflowDefID  string
		Status         string
		ResultMessage  string
	}
	if err := row.Scan(
		&item.TaskID,
		&item.WorkflowRunID,
		&item.WorkflowNodeID,
		&item.WorkflowDefID,
		&item.Status,
		&item.ResultMessage,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, nil
		}
		return item, fmt.Errorf("query workflow task: %w", err)
	}
	return item, nil
}

func (s *Service) appendEvent(ctx context.Context, workflowInstanceID string, workflowRunID string, workflowDefID string, deviceID string, nodeID string, eventType string, message string, extra map[string]any) error {
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	workflowRunID = strings.TrimSpace(workflowRunID)
	workflowDefID = strings.TrimSpace(workflowDefID)
	deviceID = strings.TrimSpace(deviceID)
	nodeID = strings.TrimSpace(nodeID)
	eventType = strings.TrimSpace(eventType)
	message = strings.TrimSpace(message)
	if workflowDefID == "" || deviceID == "" || eventType == "" {
		return nil
	}

	if extra == nil {
		extra = map[string]any{}
	}
	extraJSON, err := json.Marshal(extra)
	if err != nil {
		return fmt.Errorf("marshal workflow event extra: %w", err)
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
INSERT INTO workflow_events (
    workflow_instance_id, workflow_run_id, workflow_def_id, device_id, node_id, event_type, message, extra_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workflowInstanceID,
		workflowRunID,
		workflowDefID,
		deviceID,
		nodeID,
		eventType,
		message,
		string(extraJSON),
		createdAt,
	); err != nil {
		return fmt.Errorf("insert workflow event: %w", err)
	}
	return nil
}

func (s *Service) refreshInstanceStatus(ctx context.Context, workflowInstanceID string) error {
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	if workflowInstanceID == "" {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT status, finished_at
FROM workflow_runs
WHERE workflow_instance_id = ?`, workflowInstanceID)
	if err != nil {
		return fmt.Errorf("query workflow instance run statuses: %w", err)
	}
	defer rows.Close()

	total := 0
	runningCount := 0
	pendingCount := 0
	failedCount := 0
	successCount := 0
	stoppedCount := 0
	lastFinishedAt := ""
	for rows.Next() {
		total += 1
		var status string
		var finishedAt string
		if err := rows.Scan(&status, &finishedAt); err != nil {
			return fmt.Errorf("scan workflow instance run status: %w", err)
		}
		switch status {
		case RunStatusRunning:
			runningCount += 1
		case RunStatusPending:
			pendingCount += 1
		case RunStatusFailed:
			failedCount += 1
		case RunStatusSuccess:
			successCount += 1
		case RunStatusStopped:
			stoppedCount += 1
		}
		if finishedAt > lastFinishedAt {
			lastFinishedAt = finishedAt
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate workflow instance run statuses: %w", err)
	}
	if total == 0 {
		return nil
	}

	nextStatus := RunStatusRunning
	finishedAt := ""
	switch {
	case runningCount > 0 || pendingCount > 0:
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
UPDATE workflow_instances
SET status = ?, finished_at = ?, updated_at = ?
WHERE id = ?`,
		nextStatus,
		finishedAt,
		now,
		workflowInstanceID,
	); err != nil {
		return fmt.Errorf("update workflow instance status: %w", err)
	}
	return nil
}

func (s *Service) DeleteDefinition(ctx context.Context, workflowDefID string) error {
	workflowDefID = strings.TrimSpace(workflowDefID)
	if workflowDefID == "" {
		return ErrWorkflowDefinitionNotFound
	}

	if _, err := s.GetDefinition(ctx, workflowDefID); err != nil {
		return err
	}

	activeCount := 0
	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM workflow_runs
WHERE workflow_def_id = ?
  AND status IN (?, ?)`,
		workflowDefID,
		RunStatusPending,
		RunStatusRunning,
	)
	if err := row.Scan(&activeCount); err != nil {
		return fmt.Errorf("count active workflow runs: %w", err)
	}
	if activeCount > 0 {
		return ErrWorkflowDefinitionRunning
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow delete tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_contexts
WHERE workflow_run_id IN (
    SELECT workflow_run_id
    FROM workflow_runs
    WHERE workflow_def_id = ?
)`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow contexts: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_events
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow events: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_edges
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow edges: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_nodes
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow nodes: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_runs
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow runs: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_instances
WHERE workflow_def_id = ?`, workflowDefID); err != nil {
		return fmt.Errorf("delete workflow instances: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
DELETE FROM workflow_defs
WHERE workflow_def_id = ?`, workflowDefID)
	if err != nil {
		return fmt.Errorf("delete workflow definition: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("workflow definition rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrWorkflowDefinitionNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow delete tx: %w", err)
	}
	tx = nil
	return nil
}

func (s *Service) DeleteInstance(ctx context.Context, workflowInstanceID string) error {
	instance, err := s.GetInstance(ctx, workflowInstanceID)
	if err != nil {
		return err
	}
	if instance.Status == RunStatusPending || instance.Status == RunStatusRunning {
		return ErrWorkflowInstanceDeleteNotAllowed
	}

	row := s.db.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM plan_runs
WHERE target_ref_id = ?
  AND target_type = ?`,
		workflowInstanceID,
		"workflow",
	)
	var referencedCount int
	if err := row.Scan(&referencedCount); err != nil {
		return fmt.Errorf("count plan run references by workflow instance: %w", err)
	}
	if referencedCount > 0 {
		return ErrWorkflowInstanceDeleteNotAllowed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin workflow instance delete tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_contexts
WHERE workflow_run_id IN (
    SELECT workflow_run_id
    FROM workflow_runs
    WHERE workflow_instance_id = ?
)`, workflowInstanceID); err != nil {
		return fmt.Errorf("delete workflow instance contexts: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_events
WHERE workflow_instance_id = ?`, workflowInstanceID); err != nil {
		return fmt.Errorf("delete workflow instance events: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_runs
WHERE workflow_instance_id = ?`, workflowInstanceID); err != nil {
		return fmt.Errorf("delete workflow instance runs: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
DELETE FROM workflow_instances
WHERE workflow_instance_id = ?`, workflowInstanceID); err != nil {
		return fmt.Errorf("delete workflow instance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit workflow instance delete tx: %w", err)
	}
	tx = nil
	return nil
}

type runScanner interface {
	Scan(dest ...any) error
}

func scanRun(scanner runScanner) (Run, error) {
	var item Run
	if err := scanner.Scan(
		&item.WorkflowRunID,
		&item.WorkflowInstanceID,
		&item.WorkflowDefID,
		&item.DeviceID,
		&item.Status,
		&item.CurrentNodeID,
		&item.CurrentTaskID,
		&item.StartedAt,
		&item.FinishedAt,
		&item.LastError,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Run{}, ErrWorkflowRunNotFound
		}
		return Run{}, fmt.Errorf("scan workflow run: %w", err)
	}
	return item, nil
}

type instanceScanner interface {
	Scan(dest ...any) error
}

func scanInstance(scanner instanceScanner) (Instance, error) {
	var item Instance
	if err := scanner.Scan(
		&item.WorkflowInstanceID,
		&item.WorkflowDefID,
		&item.WorkflowName,
		&item.Status,
		&item.StartedAt,
		&item.FinishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Instance{}, ErrWorkflowInstanceNotFound
		}
		return Instance{}, fmt.Errorf("scan workflow instance: %w", err)
	}
	return item, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var item Event
	var extraJSON string
	if err := scanner.Scan(
		&item.WorkflowEventID,
		&item.WorkflowInstanceID,
		&item.WorkflowRunID,
		&item.WorkflowDefID,
		&item.DeviceID,
		&item.NodeID,
		&item.EventType,
		&item.Message,
		&extraJSON,
		&item.CreatedAt,
	); err != nil {
		return Event{}, fmt.Errorf("scan workflow event: %w", err)
	}

	item.Extra = map[string]any{}
	if strings.TrimSpace(extraJSON) != "" {
		if err := json.Unmarshal([]byte(extraJSON), &item.Extra); err != nil {
			return Event{}, fmt.Errorf("decode workflow event extra: %w", err)
		}
	}
	return item, nil
}

type nodeScanner interface {
	Scan(dest ...any) error
}

func scanNode(scanner nodeScanner) (Node, error) {
	var item Node
	if err := scanner.Scan(
		&item.WorkflowDefID,
		&item.NodeID,
		&item.NodeType,
		&item.NodeName,
		&item.ScriptName,
		&item.ScriptVersion,
		&item.MaxIterations,
		&item.Position,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Node{}, ErrWorkflowDefinitionNotFound
		}
		return Node{}, fmt.Errorf("scan workflow node: %w", err)
	}
	return item, nil
}

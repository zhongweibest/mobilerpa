package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

var (
	// ErrDeviceNotConnected 表示目标设备当前没有可用的 WebSocket 连接。
	ErrDeviceNotConnected = errors.New("device not connected")
)

// DeviceConn 封装单条设备 WebSocket 连接，并保证写操作串行化。
type DeviceConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewDeviceConn 创建带写锁保护的设备连接包装。
func NewDeviceConn(conn *websocket.Conn) *DeviceConn {
	return &DeviceConn{conn: conn}
}

// WriteJSON 在单条连接上串行写入 JSON 消息。
func (c *DeviceConn) WriteJSON(message protocol.Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.conn.WriteJSON(message)
}

// Matches 判断当前包装连接是否指向指定底层连接。
func (c *DeviceConn) Matches(conn *websocket.Conn) bool {
	return c != nil && c.conn == conn
}

// RawConn 返回底层 WebSocket 连接。
func (c *DeviceConn) RawConn() *websocket.Conn {
	if c == nil {
		return nil
	}
	return c.conn
}

// Service 负责管理在线连接、任务下发与任务回执处理。
type Service struct {
	tasks           *task.Service
	taskResultHooks []func(ctx context.Context, taskID string) error

	mu    sync.RWMutex
	conns map[string]*DeviceConn
}

// NewService 创建任务下发服务。
func NewService(tasks *task.Service) *Service {
	return &Service{
		tasks:           tasks,
		taskResultHooks: make([]func(ctx context.Context, taskID string) error, 0),
		conns:           make(map[string]*DeviceConn),
	}
}

// AddTaskResultHook 注册任务结果处理后的附加回调。
func (s *Service) AddTaskResultHook(hook func(ctx context.Context, taskID string) error) {
	if hook == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskResultHooks = append(s.taskResultHooks, hook)
}

// RegisterDeviceConn 注册设备在线连接，并返回当前生效的连接包装。
func (s *Service) RegisterDeviceConn(deviceID string, conn *websocket.Conn) *DeviceConn {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.conns[deviceID]
	if ok && current.Matches(conn) {
		return current
	}

	wrapped := NewDeviceConn(conn)
	s.conns[deviceID] = wrapped
	return wrapped
}

// UnregisterDeviceConn 注销设备在线连接。
func (s *Service) UnregisterDeviceConn(deviceID string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.conns[deviceID]
	if !ok {
		return
	}
	if current.Matches(conn) {
		delete(s.conns, deviceID)
	}
}

// AssignTask 向指定设备发送 assign_task 消息，并推进任务状态。
func (s *Service) AssignTask(ctx context.Context, taskID string) (task.Task, error) {
	taskItem, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return task.Task{}, err
	}

	conn, ok := s.getConn(taskItem.DeviceID)
	if !ok {
		return task.Task{}, ErrDeviceNotConnected
	}

	requestID := fmt.Sprintf("assign-%s-%d", taskID, time.Now().UnixNano())
	message := protocol.Envelope{
		Type:      protocol.MessageTypeAssignTask,
		RequestID: requestID,
		DeviceID:  taskItem.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id":          taskItem.TaskID,
			"workflow_node_id": taskItem.WorkflowNodeID,
			"script_name":      taskItem.ScriptName,
			"script_version":   taskItem.ScriptVersion,
			"params":           taskItem.Params,
			"priority":         taskItem.Priority,
			"scheduled_at":     taskItem.ScheduledAt,
			"task_status":      taskItem.Status,
			"dispatch_source":  "center",
		},
	}

	log.Printf("dispatch assign_task start: device_id=%s task_id=%s request_id=%s", taskItem.DeviceID, taskItem.TaskID, requestID)
	if err := conn.WriteJSON(message); err != nil {
		s.UnregisterDeviceConn(taskItem.DeviceID, conn.RawConn())
		return task.Task{}, fmt.Errorf("write assign_task: %w", err)
	}
	log.Printf("dispatch assign_task done: device_id=%s task_id=%s request_id=%s", taskItem.DeviceID, taskItem.TaskID, requestID)

	return s.tasks.MarkAssigned(ctx, taskID, requestID)
}

// SyncScript 向指定设备发送 sync_script 消息，要求设备自行从中心同步脚本版本。
func (s *Service) SyncScript(_ context.Context, deviceID string, scriptName string, scriptVersion string, force bool) error {
	deviceID = strings.TrimSpace(deviceID)
	scriptName = strings.TrimSpace(scriptName)
	scriptVersion = strings.TrimSpace(scriptVersion)

	if deviceID == "" {
		return ErrDeviceNotConnected
	}

	conn, ok := s.getConn(deviceID)
	if !ok {
		return ErrDeviceNotConnected
	}

	requestID := fmt.Sprintf("sync-script-%s-%d", deviceID, time.Now().UnixNano())
	message := protocol.Envelope{
		Type:      protocol.MessageTypeSyncScript,
		RequestID: requestID,
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"script_name":    scriptName,
			"script_version": scriptVersion,
			"force":          force,
			"source":         "center",
		},
	}

	log.Printf("dispatch sync_script start: device_id=%s script=%s@%s request_id=%s", deviceID, scriptName, scriptVersion, requestID)
	if err := conn.WriteJSON(message); err != nil {
		s.UnregisterDeviceConn(deviceID, conn.RawConn())
		return fmt.Errorf("write sync_script: %w", err)
	}
	log.Printf("dispatch sync_script done: device_id=%s script=%s@%s request_id=%s", deviceID, scriptName, scriptVersion, requestID)

	return nil
}

// StartWorkflowSession 向指定设备下发一次工作流会话。
func (s *Service) StartWorkflowSession(_ context.Context, payload protocol.StartWorkflowSessionPayload) error {
	deviceID := strings.TrimSpace(payload.DeviceID)
	if deviceID == "" {
		return ErrDeviceNotConnected
	}

	conn, ok := s.getConn(deviceID)
	if !ok {
		return ErrDeviceNotConnected
	}

	sessionKey := strings.TrimSpace(payload.PlanDeviceRunID)
	requestID := fmt.Sprintf("start-workflow-session-%s-%d", sessionKey, time.Now().UnixNano())
	message := protocol.Envelope{
		Type:      protocol.MessageTypeStartWorkflowSession,
		RequestID: requestID,
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	log.Printf(
		"dispatch start_workflow_session start: device_id=%s plan_run_id=%s plan_device_run_id=%s request_id=%s",
		deviceID,
		payload.PlanRunID,
		payload.PlanDeviceRunID,
		requestID,
	)
	if err := conn.WriteJSON(message); err != nil {
		s.UnregisterDeviceConn(deviceID, conn.RawConn())
		return fmt.Errorf("write start_workflow_session: %w", err)
	}
	log.Printf(
		"dispatch start_workflow_session done: device_id=%s plan_run_id=%s plan_device_run_id=%s request_id=%s",
		deviceID,
		payload.PlanRunID,
		payload.PlanDeviceRunID,
		requestID,
	)
	return nil
}

// StopWorkflowSession 向指定设备发送 stop_workflow_session 消息。
func (s *Service) StopWorkflowSession(_ context.Context, payload protocol.StopWorkflowSessionPayload) error {
	deviceID := strings.TrimSpace(payload.DeviceID)
	if deviceID == "" {
		return ErrDeviceNotConnected
	}

	conn, ok := s.getConn(deviceID)
	if !ok {
		return ErrDeviceNotConnected
	}

	sessionKey := strings.TrimSpace(payload.PlanDeviceRunID)
	requestID := fmt.Sprintf("stop-workflow-session-%s-%d", sessionKey, time.Now().UnixNano())
	message := protocol.Envelope{
		Type:      protocol.MessageTypeStopWorkflowSession,
		RequestID: requestID,
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
	}

	log.Printf(
		"dispatch stop_workflow_session start: device_id=%s plan_run_id=%s plan_device_run_id=%s request_id=%s",
		deviceID,
		payload.PlanRunID,
		payload.PlanDeviceRunID,
		requestID,
	)
	if err := conn.WriteJSON(message); err != nil {
		s.UnregisterDeviceConn(deviceID, conn.RawConn())
		return fmt.Errorf("write stop_workflow_session: %w", err)
	}
	log.Printf(
		"dispatch stop_workflow_session done: device_id=%s plan_run_id=%s plan_device_run_id=%s request_id=%s",
		deviceID,
		payload.PlanRunID,
		payload.PlanDeviceRunID,
		requestID,
	)
	return nil
}

// HandleTaskAck 处理设备回传的 task_ack 消息。
func (s *Service) HandleTaskAck(ctx context.Context, envelope protocol.Envelope) (task.Task, error) {
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		return task.Task{}, fmt.Errorf("marshal task_ack payload: %w", err)
	}

	var payload task.AckPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return task.Task{}, fmt.Errorf("decode task_ack payload: %w", err)
	}

	payload.TaskID = strings.TrimSpace(payload.TaskID)
	if payload.TaskID == "" {
		return task.Task{}, task.ErrTaskNotFound
	}
	if payload.Status == "" {
		payload.Status = "ok"
	}

	return s.tasks.MarkAcknowledged(ctx, payload.TaskID, payload, envelope.RequestID)
}

// HandleTaskResult 处理设备回传的 task_result 消息。
func (s *Service) HandleTaskResult(ctx context.Context, envelope protocol.Envelope) (task.Task, error) {
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		return task.Task{}, fmt.Errorf("marshal task_result payload: %w", err)
	}

	var payload task.ResultPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return task.Task{}, fmt.Errorf("decode task_result payload: %w", err)
	}

	payload.TaskID = strings.TrimSpace(payload.TaskID)
	if payload.TaskID == "" {
		return task.Task{}, task.ErrTaskNotFound
	}

	taskItem, err := s.tasks.MarkResult(ctx, payload.TaskID, payload, envelope.RequestID)
	if err != nil {
		return task.Task{}, err
	}

	hooks := s.getTaskResultHooks()
	for _, hook := range hooks {
		if err := hook(ctx, payload.TaskID); err != nil {
			return task.Task{}, err
		}
	}

	return taskItem, nil
}

// HandleTaskProgress 处理设备回传的 task_progress 消息。
func (s *Service) HandleTaskProgress(ctx context.Context, envelope protocol.Envelope) (task.Task, error) {
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		return task.Task{}, fmt.Errorf("marshal task_progress payload: %w", err)
	}

	var payload task.ProgressPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return task.Task{}, fmt.Errorf("decode task_progress payload: %w", err)
	}

	payload.TaskID = strings.TrimSpace(payload.TaskID)
	if payload.TaskID == "" {
		return task.Task{}, task.ErrTaskNotFound
	}

	log.Printf(
		"task_progress received: device_id=%s task_id=%s request_id=%s step_name=%s status=%s message=%s",
		strings.TrimSpace(envelope.DeviceID),
		payload.TaskID,
		strings.TrimSpace(envelope.RequestID),
		strings.TrimSpace(payload.StepName),
		strings.TrimSpace(payload.Status),
		strings.TrimSpace(payload.Message),
	)

	return s.tasks.MarkProgress(ctx, payload.TaskID, payload, envelope.RequestID)
}

// MarkTaskRunning 把任务推进到运行中状态。
func (s *Service) MarkTaskRunning(ctx context.Context, taskID string, requestID string, stepName string, message string) (task.Task, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return task.Task{}, task.ErrTaskNotFound
	}
	return s.tasks.MarkRunning(ctx, taskID, requestID, stepName, message)
}

func (s *Service) getConn(deviceID string) (*DeviceConn, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, ok := s.conns[deviceID]
	return conn, ok
}

func (s *Service) getTaskResultHooks() []func(ctx context.Context, taskID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hooks := make([]func(ctx context.Context, taskID string) error, 0, len(s.taskResultHooks))
	hooks = append(hooks, s.taskResultHooks...)
	return hooks
}

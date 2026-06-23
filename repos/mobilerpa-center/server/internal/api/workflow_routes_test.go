package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/discovery"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/settings"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/internal/ws"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

func readMessages(t *testing.T, conn *websocket.Conn, count int) []protocol.Envelope {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()

	items := make([]protocol.Envelope, 0, count)
	for len(items) < count {
		var msg protocol.Envelope
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("read websocket message: %v", err)
		}
		items = append(items, msg)
	}
	return items
}

func findAssignTaskMessage(t *testing.T, messages []protocol.Envelope) protocol.Envelope {
	t.Helper()
	for _, msg := range messages {
		if msg.Type == protocol.MessageTypeAssignTask {
			return msg
		}
	}
	t.Fatalf("assign_task message not found")
	return protocol.Envelope{}
}

func findAckFor(t *testing.T, messages []protocol.Envelope, requestID string, messageType string) protocol.Envelope {
	t.Helper()
	for _, msg := range messages {
		if msg.Type != "ack" {
			continue
		}
		payload, ok := msg.Payload.(map[string]any)
		if !ok {
			continue
		}
		if msg.RequestID == requestID && payload["message_type"] == messageType {
			return msg
		}
	}
	t.Fatalf("ack for request_id=%s message_type=%s not found", requestID, messageType)
	return protocol.Envelope{}
}

func TestWorkflowCreateStartAndAdvance(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-workflow-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	settingsService := settings.NewService(db)
	workflowService := workflow.NewService(db, deviceService, taskService, dispatchService)
	dispatchService.AddTaskResultHook(workflowService.HandleTaskResult)
	wsHandler := ws.NewHandler(deviceService, dispatchService, workflowService)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, settingsService, workflowService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerDevice := func(agentUUID string) string {
		t.Helper()
		body := map[string]any{
			"agent_uuid":  agentUUID,
			"device_name": agentUUID,
			"brand":       "Google",
			"model":       "Pixel 8",
		}
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal register body: %v", err)
		}
		resp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("register device request: %v", err)
		}
		defer resp.Body.Close()
		var payload struct {
			Data struct {
				DeviceID string `json:"device_id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode register response: %v", err)
		}
		return payload.Data.DeviceID
	}

	deviceID := registerDevice("agent-workflow-001")

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "workflow-hello-001",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"agent_uuid": "agent-workflow-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), deviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        time.Now().UTC().Format(time.RFC3339),
		Message:                          "test ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	createWorkflowBody := map[string]any{
		"workflow_name": "示例工作流",
		"status":        "active",
		"nodes": []map[string]any{
			{
				"node_id":        "node_a",
				"node_type":      "script",
				"node_name":      "步骤A",
				"script_name":    "shoppe_sync",
				"script_version": "v0.1.0",
			},
			{
				"node_id":        "node_b",
				"node_type":      "script",
				"node_name":      "步骤B",
				"script_name":    "shoppe_sync",
				"script_version": "v0.1.1",
			},
			{
				"node_id":   "node_stop",
				"node_type": "stop",
				"node_name": "结束",
			},
		},
		"edges": []map[string]any{
			{"from_node_id": "node_a", "to_node_id": "node_b", "edge_type": "next"},
			{"from_node_id": "node_b", "to_node_id": "node_stop", "edge_type": "next"},
		},
	}
	createWorkflowRaw, err := json.Marshal(createWorkflowBody)
	if err != nil {
		t.Fatalf("marshal workflow body: %v", err)
	}

	createWorkflowResp, err := http.Post(server.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createWorkflowRaw))
	if err != nil {
		t.Fatalf("create workflow request: %v", err)
	}
	defer createWorkflowResp.Body.Close()

	var createWorkflowPayload struct {
		Status string              `json:"status"`
		Data   workflow.Definition `json:"data"`
	}
	if err := json.NewDecoder(createWorkflowResp.Body).Decode(&createWorkflowPayload); err != nil {
		t.Fatalf("decode workflow response: %v", err)
	}
	if createWorkflowPayload.Data.WorkflowDefID == "" {
		t.Fatalf("expected workflow definition id")
	}

	startBody := map[string]any{
		"device_ids": []string{deviceID},
	}
	startRaw, err := json.Marshal(startBody)
	if err != nil {
		t.Fatalf("marshal workflow start body: %v", err)
	}

	startResp, err := http.Post(server.URL+"/api/v1/workflows/"+createWorkflowPayload.Data.WorkflowDefID+"/start", "application/json", bytes.NewReader(startRaw))
	if err != nil {
		t.Fatalf("start workflow request: %v", err)
	}
	defer startResp.Body.Close()
	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected workflow start status: %d", startResp.StatusCode)
	}

	var firstAssign protocol.Envelope
	if err := conn.ReadJSON(&firstAssign); err != nil {
		t.Fatalf("read first assign_task: %v", err)
	}
	if firstAssign.Type != protocol.MessageTypeAssignTask && firstAssign.Type != protocol.MessageTypeStartWorkflowSession {
		t.Fatalf("unexpected first websocket message type: %s", firstAssign.Type)
	}

	if firstAssign.Type == protocol.MessageTypeStartWorkflowSession {
		return
	}

	firstAssignPayloadBytes, err := json.Marshal(firstAssign.Payload)
	if err != nil {
		t.Fatalf("marshal first assign payload: %v", err)
	}
	var firstAssignPayload struct {
		TaskID        string `json:"task_id"`
		ScriptVersion string `json:"script_version"`
	}
	if err := json.Unmarshal(firstAssignPayloadBytes, &firstAssignPayload); err != nil {
		t.Fatalf("decode first assign payload: %v", err)
	}
	if firstAssignPayload.ScriptVersion != "v0.1.0" {
		t.Fatalf("unexpected first script version: %s", firstAssignPayload.ScriptVersion)
	}

	firstAck := protocol.Envelope{
		Type:      protocol.MessageTypeTaskAck,
		RequestID: "workflow-task-ack-001",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id": firstAssignPayload.TaskID,
			"status":  "ok",
			"message": "task ack ok",
		},
	}
	if err := conn.WriteJSON(firstAck); err != nil {
		t.Fatalf("write first task ack: %v", err)
	}
	var ackResp protocol.Envelope
	if err := conn.ReadJSON(&ackResp); err != nil {
		t.Fatalf("read first task ack response: %v", err)
	}

	runsBeforeProgressResp, err := http.Get(server.URL + "/api/v1/workflows/" + createWorkflowPayload.Data.WorkflowDefID + "/runs")
	if err != nil {
		t.Fatalf("get workflow runs before progress request: %v", err)
	}
	defer runsBeforeProgressResp.Body.Close()
	if runsBeforeProgressResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected workflow runs before progress status: %d", runsBeforeProgressResp.StatusCode)
	}

	var runsBeforeProgressPayload struct {
		Status string         `json:"status"`
		Data   []workflow.Run `json:"data"`
	}
	if err := json.NewDecoder(runsBeforeProgressResp.Body).Decode(&runsBeforeProgressPayload); err != nil {
		t.Fatalf("decode workflow runs before progress response: %v", err)
	}
	if len(runsBeforeProgressPayload.Data) != 1 {
		t.Fatalf("unexpected workflow run count before progress: %d", len(runsBeforeProgressPayload.Data))
	}
	workflowRunID := runsBeforeProgressPayload.Data[0].WorkflowRunID

	firstProgress := protocol.Envelope{
		Type:      protocol.MessageTypeWorkflowStepProgress,
		RequestID: "workflow-step-progress-001",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"workflow_run_id":  workflowRunID,
			"workflow_node_id": "node_a",
			"task_id":          firstAssignPayload.TaskID,
			"status":           "running",
			"step_name":        "OPEN_APP",
			"message":          "工作流步骤执行中：准备打开 QQ",
			"extra": map[string]any{
				"source": "agent",
			},
		},
	}
	if err := conn.WriteJSON(firstProgress); err != nil {
		t.Fatalf("write workflow_step_progress: %v", err)
	}
	var progressAck protocol.Envelope
	if err := conn.ReadJSON(&progressAck); err != nil {
		t.Fatalf("read workflow_step_progress ack: %v", err)
	}

	firstResult := protocol.Envelope{
		Type:      protocol.MessageTypeTaskResult,
		RequestID: "workflow-task-result-001",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id":        firstAssignPayload.TaskID,
			"status":         "success",
			"result_code":    "OK",
			"result_message": "step a done",
			"step_name":      "STEP_A",
		},
	}
	if err := conn.WriteJSON(firstResult); err != nil {
		t.Fatalf("write first task result: %v", err)
	}
	firstResultMessages := readMessages(t, conn, 2)
	_ = findAckFor(t, firstResultMessages, "workflow-task-result-001", protocol.MessageTypeTaskResult)
	secondAssign := findAssignTaskMessage(t, firstResultMessages)
	if secondAssign.Type != protocol.MessageTypeAssignTask {
		t.Fatalf("unexpected second websocket message type: %s", secondAssign.Type)
	}

	secondAssignPayloadBytes, err := json.Marshal(secondAssign.Payload)
	if err != nil {
		t.Fatalf("marshal second assign payload: %v", err)
	}
	var secondAssignPayload struct {
		TaskID        string `json:"task_id"`
		ScriptVersion string `json:"script_version"`
	}
	if err := json.Unmarshal(secondAssignPayloadBytes, &secondAssignPayload); err != nil {
		t.Fatalf("decode second assign payload: %v", err)
	}
	if secondAssignPayload.ScriptVersion != "v0.1.1" {
		t.Fatalf("unexpected second script version: %s", secondAssignPayload.ScriptVersion)
	}

	secondAck := protocol.Envelope{
		Type:      protocol.MessageTypeTaskAck,
		RequestID: "workflow-task-ack-002",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id": secondAssignPayload.TaskID,
			"status":  "ok",
			"message": "task ack ok",
		},
	}
	if err := conn.WriteJSON(secondAck); err != nil {
		t.Fatalf("write second task ack: %v", err)
	}
	var secondAckResp protocol.Envelope
	if err := conn.ReadJSON(&secondAckResp); err != nil {
		t.Fatalf("read second task ack response: %v", err)
	}

	secondResult := protocol.Envelope{
		Type:      protocol.MessageTypeTaskResult,
		RequestID: "workflow-task-result-002",
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id":        secondAssignPayload.TaskID,
			"status":         "success",
			"result_code":    "OK",
			"result_message": "step b done",
			"step_name":      "STEP_B",
		},
	}
	if err := conn.WriteJSON(secondResult); err != nil {
		t.Fatalf("write second task result: %v", err)
	}
	secondResultMessages := readMessages(t, conn, 1)
	_ = findAckFor(t, secondResultMessages, "workflow-task-result-002", protocol.MessageTypeTaskResult)

	runsResp, err := http.Get(server.URL + "/api/v1/workflows/" + createWorkflowPayload.Data.WorkflowDefID + "/runs")
	if err != nil {
		t.Fatalf("get workflow runs request: %v", err)
	}
	defer runsResp.Body.Close()
	if runsResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected workflow runs status: %d", runsResp.StatusCode)
	}

	var runsPayload struct {
		Status string         `json:"status"`
		Data   []workflow.Run `json:"data"`
	}
	if err := json.NewDecoder(runsResp.Body).Decode(&runsPayload); err != nil {
		t.Fatalf("decode workflow runs response: %v", err)
	}
	if len(runsPayload.Data) != 1 {
		t.Fatalf("unexpected workflow run count: %d", len(runsPayload.Data))
	}
	if runsPayload.Data[0].Status != workflow.RunStatusSuccess {
		t.Fatalf("unexpected workflow run status: %s", runsPayload.Data[0].Status)
	}

	eventsResp, err := http.Get(server.URL + "/api/v1/workflows/" + createWorkflowPayload.Data.WorkflowDefID + "/events?workflow_run_id=" + workflowRunID)
	if err != nil {
		t.Fatalf("get workflow events request: %v", err)
	}
	defer eventsResp.Body.Close()
	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected workflow events status: %d", eventsResp.StatusCode)
	}

	var eventsPayload struct {
		Status string           `json:"status"`
		Data   []workflow.Event `json:"data"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsPayload); err != nil {
		t.Fatalf("decode workflow events response: %v", err)
	}
	if len(eventsPayload.Data) == 0 {
		t.Fatalf("expected workflow events")
	}

	eventTypes := make(map[string]bool)
	for _, item := range eventsPayload.Data {
		eventTypes[item.EventType] = true
	}
	if !eventTypes[workflow.EventTypeWorkflowRunStarted] {
		t.Fatalf("expected workflow_run_started event")
	}
	if !eventTypes[workflow.EventTypeWorkflowStepStarted] {
		t.Fatalf("expected workflow_step_started event")
	}
	if !eventTypes[workflow.EventTypeWorkflowStepProgress] {
		t.Fatalf("expected workflow_step_progress event")
	}
	if !eventTypes[workflow.EventTypeWorkflowStepSucceeded] {
		t.Fatalf("expected workflow_step_succeeded event")
	}
	if !eventTypes[workflow.EventTypeWorkflowRunCompleted] {
		t.Fatalf("expected workflow_run_completed event")
	}
}


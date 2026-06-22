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
	"github.com/mobilerpa/mobilerpa-center/server/internal/plan"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/internal/ws"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

func TestPlanCreateListAndGet(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-plan-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	planService := plan.NewService(db, deviceService, taskService, dispatchService, nil)
	dispatchService.AddTaskResultHook(planService.HandleTaskResult)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, planService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	createBody := map[string]any{
		"plan_name":             "每日脚本计划",
		"target_type":           "script",
		"target_script_name":    "open_wechat",
		"target_script_version": "v0.1.0",
		"schedule_type":         "daily",
		"daily_start_time":      "09:00:00",
		"daily_deadline_time":   "23:00:00",
		"status":                "enabled",
		"device_ids":            []string{"dev_000001", "dev_000002"},
	}

	rawBody, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	createResp, err := http.Post(server.URL+"/api/v1/plans", "application/json", bytes.NewReader(rawBody))
	if err != nil {
		t.Fatalf("create plan request: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create status: %d", createResp.StatusCode)
	}

	var createPayload struct {
		Status string          `json:"status"`
		Data   plan.Definition `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	if createPayload.Status != "ok" {
		t.Fatalf("unexpected create status payload: %s", createPayload.Status)
	}
	if createPayload.Data.PlanDefID == "" {
		t.Fatalf("expected plan_def_id")
	}
	if createPayload.Data.TargetType != "script" {
		t.Fatalf("unexpected target_type: %s", createPayload.Data.TargetType)
	}

	listResp, err := http.Get(server.URL + "/api/v1/plans")
	if err != nil {
		t.Fatalf("list plans request: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %d", listResp.StatusCode)
	}

	var listPayload struct {
		Status string `json:"status"`
		Data   struct {
			Items []plan.Definition `json:"items"`
			Total int               `json:"total"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	if listPayload.Data.Total != 1 {
		t.Fatalf("unexpected list total: %d", listPayload.Data.Total)
	}
	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("unexpected list items length: %d", len(listPayload.Data.Items))
	}

	getResp, err := http.Get(server.URL + "/api/v1/plans/" + createPayload.Data.PlanDefID)
	if err != nil {
		t.Fatalf("get plan request: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get status: %d", getResp.StatusCode)
	}

	var getPayload struct {
		Status string          `json:"status"`
		Data   plan.Definition `json:"data"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&getPayload); err != nil {
		t.Fatalf("decode get response: %v", err)
	}

	if getPayload.Data.PlanDefID != createPayload.Data.PlanDefID {
		t.Fatalf("unexpected plan_def_id: %s", getPayload.Data.PlanDefID)
	}
	if len(getPayload.Data.DeviceIDs) != 2 {
		t.Fatalf("unexpected device ids: %#v", getPayload.Data.DeviceIDs)
	}
}

func TestPlanStartScriptRun(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-plan-run-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	planService := plan.NewService(db, deviceService, taskService, dispatchService, nil)
	dispatchService.AddTaskResultHook(planService.HandleTaskResult)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, planService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-plan-001",
		"device_name": "plan-device",
		"brand":       "Google",
		"model":       "Pixel",
	}
	rawRegisterBody, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}

	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(rawRegisterBody))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer registerResp.Body.Close()

	var registerPayload struct {
		Data struct {
			DeviceID string `json:"device_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), registerPayload.Data.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        "2026-06-20T00:00:00Z",
		Message:                          "ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "plan-hello-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: 1,
		Payload: map[string]any{
			"agent_uuid": "agent-plan-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	assignTaskDone := make(chan error, 1)
	go func() {
		var envelope protocol.Envelope
		assignTaskDone <- conn.ReadJSON(&envelope)
	}()

	createBody := map[string]any{
		"plan_name":             "脚本计划任务",
		"target_type":           "script",
		"target_script_name":    "open_wechat",
		"target_script_version": "v0.1.0",
		"schedule_type":         "once",
		"status":                "enabled",
		"device_ids":            []string{registerPayload.Data.DeviceID},
	}
	rawCreateBody, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}
	createResp, err := http.Post(server.URL+"/api/v1/plans", "application/json", bytes.NewReader(rawCreateBody))
	if err != nil {
		t.Fatalf("create plan request: %v", err)
	}
	defer createResp.Body.Close()

	var createPayload struct {
		Data plan.Definition `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	startResp, err := http.Post(server.URL+"/api/v1/plans/"+createPayload.Data.PlanDefID+"/start", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("start plan request: %v", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start status: %d", startResp.StatusCode)
	}

	var startPayload struct {
		Status string   `json:"status"`
		Data   plan.Run `json:"data"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("decode start response: %v", err)
	}

	if startPayload.Status != "ok" {
		t.Fatalf("unexpected start payload status: %s", startPayload.Status)
	}
	if startPayload.Data.PlanRunID == "" {
		t.Fatalf("expected plan_run_id")
	}
	if len(startPayload.Data.DeviceRuns) != 1 {
		t.Fatalf("unexpected device runs: %d", len(startPayload.Data.DeviceRuns))
	}
	if err := <-assignTaskDone; err != nil {
		t.Fatalf("read assign_task message: %v", err)
	}
}

func TestPlanStartWorkflowRun(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-plan-workflow-run-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	workflowService := workflow.NewService(db, deviceService, taskService, dispatchService)
	planService := plan.NewService(db, deviceService, taskService, dispatchService, workflowService)
	dispatchService.AddTaskResultHook(planService.HandleTaskResult)
	dispatchService.AddTaskResultHook(workflowService.HandleTaskResult)
	dispatchService.AddTaskResultHook(planService.SyncWorkflowRunByTask)
	wsHandler := ws.NewHandler(deviceService, dispatchService, workflowService)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, planService, workflowService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-plan-wf-001",
		"device_name": "plan-workflow-device",
		"brand":       "Google",
		"model":       "Pixel",
	}
	rawRegisterBody, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}
	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(rawRegisterBody))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer registerResp.Body.Close()

	var registerPayload struct {
		Data struct {
			DeviceID string `json:"device_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), registerPayload.Data.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        "2026-06-20T00:00:00Z",
		Message:                          "ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "plan-workflow-hello-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: 1,
		Payload: map[string]any{
			"agent_uuid": "agent-plan-wf-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	assignTaskDone := make(chan protocol.Envelope, 1)
	go func() {
		var envelope protocol.Envelope
		_ = conn.ReadJSON(&envelope)
		assignTaskDone <- envelope
	}()

	createWorkflowBody := map[string]any{
		"workflow_name": "计划任务工作流",
		"status":        "active",
		"nodes": []map[string]any{
			{
				"node_id":        "node_a",
				"node_type":      "script",
				"node_name":      "步骤A",
				"script_name":    "open_wechat",
				"script_version": "v0.1.0",
			},
			{
				"node_id":   "node_stop",
				"node_type": "stop",
				"node_name": "结束",
			},
		},
		"edges": []map[string]any{
			{
				"from_node_id": "node_a",
				"to_node_id":   "node_stop",
				"edge_type":    "next",
			},
		},
	}
	rawCreateWorkflowBody, err := json.Marshal(createWorkflowBody)
	if err != nil {
		t.Fatalf("marshal workflow create body: %v", err)
	}
	createWorkflowResp, err := http.Post(server.URL+"/api/v1/workflows", "application/json", bytes.NewReader(rawCreateWorkflowBody))
	if err != nil {
		t.Fatalf("create workflow request: %v", err)
	}
	defer createWorkflowResp.Body.Close()

	var createWorkflowPayload struct {
		Data workflow.Definition `json:"data"`
	}
	if err := json.NewDecoder(createWorkflowResp.Body).Decode(&createWorkflowPayload); err != nil {
		t.Fatalf("decode workflow create response: %v", err)
	}

	createPlanBody := map[string]any{
		"plan_name":              "工作流计划任务",
		"target_type":            "workflow",
		"target_workflow_def_id": createWorkflowPayload.Data.WorkflowDefID,
		"schedule_type":          "once",
		"status":                 "enabled",
		"device_ids":             []string{registerPayload.Data.DeviceID},
	}
	rawCreatePlanBody, err := json.Marshal(createPlanBody)
	if err != nil {
		t.Fatalf("marshal plan create body: %v", err)
	}
	createPlanResp, err := http.Post(server.URL+"/api/v1/plans", "application/json", bytes.NewReader(rawCreatePlanBody))
	if err != nil {
		t.Fatalf("create plan request: %v", err)
	}
	defer createPlanResp.Body.Close()

	var createPlanPayload struct {
		Data plan.Definition `json:"data"`
	}
	if err := json.NewDecoder(createPlanResp.Body).Decode(&createPlanPayload); err != nil {
		t.Fatalf("decode plan create response: %v", err)
	}

	startResp, err := http.Post(server.URL+"/api/v1/plans/"+createPlanPayload.Data.PlanDefID+"/start", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("start workflow plan request: %v", err)
	}
	defer startResp.Body.Close()

	if startResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected start workflow plan status: %d", startResp.StatusCode)
	}

	var startPayload struct {
		Status string   `json:"status"`
		Data   plan.Run `json:"data"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&startPayload); err != nil {
		t.Fatalf("decode workflow plan start response: %v", err)
	}

	if startPayload.Data.TargetType != "workflow" {
		t.Fatalf("unexpected target_type: %s", startPayload.Data.TargetType)
	}
	if startPayload.Data.TargetRefID == "" {
		t.Fatalf("expected workflow instance id as target_ref_id")
	}
	assignTaskMessage := <-assignTaskDone
	if assignTaskMessage.Type != protocol.MessageTypeAssignTask {
		t.Fatalf("unexpected message type: %s", assignTaskMessage.Type)
	}

	payload, ok := assignTaskMessage.Payload.(map[string]any)
	if !ok {
		t.Fatalf("unexpected assign_task payload type")
	}
	taskID, _ := payload["task_id"].(string)
	workflowRunID, _ := payload["workflow_run_id"].(string)
	workflowNodeID, _ := payload["workflow_node_id"].(string)
	if taskID == "" || workflowRunID == "" || workflowNodeID == "" {
		t.Fatalf("unexpected workflow assign_task payload: %#v", payload)
	}

	resultMessage := protocol.Envelope{
		Type:      protocol.MessageTypeTaskResult,
		RequestID: "plan-workflow-result-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: 2,
		Payload: map[string]any{
			"task_id":          taskID,
			"status":           "success",
			"result_code":      "ok",
			"result_message":   "ok",
			"step_name":        "DONE",
			"workflow_run_id":  workflowRunID,
			"workflow_node_id": workflowNodeID,
		},
	}
	if err := conn.WriteJSON(resultMessage); err != nil {
		t.Fatalf("write task_result: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	runResp, err := http.Get(server.URL + "/api/v1/plans/" + createPlanPayload.Data.PlanDefID + "/runs/" + startPayload.Data.PlanRunID)
	if err != nil {
		t.Fatalf("get workflow plan run request: %v", err)
	}
	defer runResp.Body.Close()

	var runPayload struct {
		Data plan.Run `json:"data"`
	}
	if err := json.NewDecoder(runResp.Body).Decode(&runPayload); err != nil {
		t.Fatalf("decode workflow plan run response: %v", err)
	}
	if runPayload.Data.Status == "" {
		t.Fatalf("expected plan run status")
	}
	if runPayload.Data.Status != plan.RunStatusRunning && runPayload.Data.Status != plan.RunStatusSuccess {
		t.Fatalf("unexpected workflow plan run status: %s", runPayload.Data.Status)
	}
	if len(runPayload.Data.DeviceRuns) != 1 {
		t.Fatalf("unexpected workflow plan device runs: %d", len(runPayload.Data.DeviceRuns))
	}
	if runPayload.Data.DeviceRuns[0].Status != plan.DeviceRunStatusRunning && runPayload.Data.DeviceRuns[0].Status != plan.DeviceRunStatusSuccess {
		t.Fatalf("unexpected workflow plan device run status: %s", runPayload.Data.DeviceRuns[0].Status)
	}
}

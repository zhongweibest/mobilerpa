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
	"github.com/mobilerpa/mobilerpa-center/server/internal/discovery"
	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/script"
	"github.com/mobilerpa/mobilerpa-center/server/internal/settings"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/ws"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

func newTestScriptService(t *testing.T) *script.Service {
	t.Helper()
	return script.NewService(nil, filepath.Join("..", "..", "..", "mobilerpa-agent", "agent", "scripts"))
}

func TestRegisterAndListDevices(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	healthResp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected healthz status: %d", healthResp.StatusCode)
	}

	registerBody := map[string]any{
		"agent_uuid":  "agent-test-001",
		"device_name": "Pixel Test",
		"brand":       "Google",
		"model":       "Pixel 8",
		"android_id":  "android-test-001",
		"adb_serial":  "adb-test-001",
	}

	bodyBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}

	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer registerResp.Body.Close()

	if registerResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected register status: %d", registerResp.StatusCode)
	}

	var registerPayload struct {
		Status string `json:"status"`
		Data   struct {
			DeviceID   string `json:"device_id"`
			BindStatus string `json:"bind_status"`
			Status     string `json:"status"`
		} `json:"data"`
	}

	if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	if registerPayload.Status != "ok" {
		t.Fatalf("unexpected register payload status: %s", registerPayload.Status)
	}

	if registerPayload.Data.DeviceID != "1" {
		t.Fatalf("unexpected device id: %s", registerPayload.Data.DeviceID)
	}

	listResp, err := http.Get(server.URL + "/api/v1/devices")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list status: %d", listResp.StatusCode)
	}

	var listPayload struct {
		Status string `json:"status"`
		Data   struct {
			Items    []device.Device `json:"items"`
			Total    int             `json:"total"`
			Page     int             `json:"page"`
			PageSize int             `json:"page_size"`
		} `json:"data"`
	}

	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	if listPayload.Status != "ok" {
		t.Fatalf("unexpected list payload status: %s", listPayload.Status)
	}

	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("unexpected device count: %d", len(listPayload.Data.Items))
	}

	if listPayload.Data.Items[0].AgentUUID != "agent-test-001" {
		t.Fatalf("unexpected agent uuid: %s", listPayload.Data.Items[0].AgentUUID)
	}

	getResp, err := http.Get(server.URL + "/api/v1/devices/" + registerPayload.Data.DeviceID)
	if err != nil {
		t.Fatalf("get device request: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get device status: %d", getResp.StatusCode)
	}

	var getPayload struct {
		Status string        `json:"status"`
		Data   device.Device `json:"data"`
	}

	if err := json.NewDecoder(getResp.Body).Decode(&getPayload); err != nil {
		t.Fatalf("decode get device response: %v", err)
	}

	if getPayload.Status != "ok" {
		t.Fatalf("unexpected get device payload status: %s", getPayload.Status)
	}

	if getPayload.Data.DeviceID != registerPayload.Data.DeviceID {
		t.Fatalf("unexpected get device id: %s", getPayload.Data.DeviceID)
	}

	if getPayload.Data.AgentUUID != "agent-test-001" {
		t.Fatalf("unexpected get device agent uuid: %s", getPayload.Data.AgentUUID)
	}

	missingResp, err := http.Get(server.URL + "/api/v1/devices/dev_missing")
	if err != nil {
		t.Fatalf("missing device request: %v", err)
	}
	defer missingResp.Body.Close()

	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected missing device status: %d", missingResp.StatusCode)
	}

	var missingPayload struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}

	if err := json.NewDecoder(missingResp.Body).Decode(&missingPayload); err != nil {
		t.Fatalf("decode missing device response: %v", err)
	}

	if missingPayload.Error != "device_not_found" {
		t.Fatalf("unexpected missing device error: %s", missingPayload.Error)
	}
}

func TestDeleteOfflineDeviceAndRejectOnlineDevice(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-delete-device-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerOfflineBody := map[string]any{
		"agent_uuid":  "agent-delete-offline-001",
		"device_name": "Offline Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}
	registerOfflineBytes, err := json.Marshal(registerOfflineBody)
	if err != nil {
		t.Fatalf("marshal offline device body: %v", err)
	}

	registerOfflineResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerOfflineBytes))
	if err != nil {
		t.Fatalf("register offline device request: %v", err)
	}
	defer registerOfflineResp.Body.Close()

	var offlinePayload struct {
		Data struct {
			DeviceID string `json:"device_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(registerOfflineResp.Body).Decode(&offlinePayload); err != nil {
		t.Fatalf("decode offline register response: %v", err)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/devices/"+offlinePayload.Data.DeviceID, nil)
	if err != nil {
		t.Fatalf("new delete offline request: %v", err)
	}

	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete offline device request: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected delete offline status: %d", deleteResp.StatusCode)
	}

	var deletePayload struct {
		Status string `json:"status"`
		Data   struct {
			DeviceID string `json:"device_id"`
			Deleted  bool   `json:"deleted"`
		} `json:"data"`
	}
	if err := json.NewDecoder(deleteResp.Body).Decode(&deletePayload); err != nil {
		t.Fatalf("decode delete offline response: %v", err)
	}
	if deletePayload.Status != "ok" {
		t.Fatalf("unexpected delete offline payload status: %s", deletePayload.Status)
	}
	if !deletePayload.Data.Deleted {
		t.Fatalf("expected deleted=true for offline device")
	}

	getDeletedResp, err := http.Get(server.URL + "/api/v1/devices/" + offlinePayload.Data.DeviceID)
	if err != nil {
		t.Fatalf("get deleted device request: %v", err)
	}
	defer getDeletedResp.Body.Close()

	if getDeletedResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected deleted device query status: %d", getDeletedResp.StatusCode)
	}

	registerOnlineBody := map[string]any{
		"agent_uuid":  "agent-delete-online-001",
		"device_name": "Online Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}
	registerOnlineBytes, err := json.Marshal(registerOnlineBody)
	if err != nil {
		t.Fatalf("marshal online device body: %v", err)
	}

	registerOnlineResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerOnlineBytes))
	if err != nil {
		t.Fatalf("register online device request: %v", err)
	}
	defer registerOnlineResp.Body.Close()

	var onlinePayload struct {
		Data struct {
			DeviceID string `json:"device_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(registerOnlineResp.Body).Decode(&onlinePayload); err != nil {
		t.Fatalf("decode online register response: %v", err)
	}

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "hello-delete-online-001",
		DeviceID:  onlinePayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"agent_uuid": "agent-delete-online-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), onlinePayload.Data.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        time.Now().UTC().Format(time.RFC3339),
		Message:                          "test ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	deleteOnlineReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/devices/"+onlinePayload.Data.DeviceID, nil)
	if err != nil {
		t.Fatalf("new delete online request: %v", err)
	}

	deleteOnlineResp, err := http.DefaultClient.Do(deleteOnlineReq)
	if err != nil {
		t.Fatalf("delete online device request: %v", err)
	}
	defer deleteOnlineResp.Body.Close()

	if deleteOnlineResp.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected delete online status: %d", deleteOnlineResp.StatusCode)
	}

	var deleteOnlinePayload struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(deleteOnlineResp.Body).Decode(&deleteOnlinePayload); err != nil {
		t.Fatalf("decode delete online response: %v", err)
	}
	if deleteOnlinePayload.Error != "device_online_cannot_delete" {
		t.Fatalf("unexpected delete online error: %s", deleteOnlinePayload.Error)
	}
}

func TestCreateListTasksAndEvents(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-task-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-task-001",
		"device_name": "Task Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}

	bodyBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}

	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	defer registerResp.Body.Close()

	var registerPayload struct {
		Status string `json:"status"`
		Data   struct {
			DeviceID string `json:"device_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
		t.Fatalf("decode register response: %v", err)
	}

	createBody := map[string]any{
		"device_id":      registerPayload.Data.DeviceID,
		"script_name":    "shoppe_sync",
		"script_version": "v0.1.0",
		"priority":       3,
		"params": map[string]any{
			"shop_id": "shop-001",
			"mode":    "dry-run",
		},
		"scheduled_at": "2026-06-12T10:30:00Z",
	}

	createBodyBytes, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	createResp, err := http.Post(server.URL+"/api/v1/tasks", "application/json", bytes.NewReader(createBodyBytes))
	if err != nil {
		t.Fatalf("create task request: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create task status: %d", createResp.StatusCode)
	}

	var createPayload struct {
		Status string    `json:"status"`
		Data   task.Task `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}

	if createPayload.Status != "ok" {
		t.Fatalf("unexpected create task payload status: %s", createPayload.Status)
	}
	if createPayload.Data.TaskID != "1" {
		t.Fatalf("unexpected task id: %s", createPayload.Data.TaskID)
	}
	if createPayload.Data.Status != task.StatusPending {
		t.Fatalf("unexpected task status: %s", createPayload.Data.Status)
	}
	if createPayload.Data.Params["shop_id"] != "shop-001" {
		t.Fatalf("unexpected task params: %#v", createPayload.Data.Params)
	}

	listResp, err := http.Get(server.URL + "/api/v1/tasks")
	if err != nil {
		t.Fatalf("list tasks request: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list tasks status: %d", listResp.StatusCode)
	}

	var listPayload struct {
		Status string `json:"status"`
		Data   struct {
			Items    []task.Task `json:"items"`
			Total    int         `json:"total"`
			Page     int         `json:"page"`
			PageSize int         `json:"page_size"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list tasks response: %v", err)
	}

	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("unexpected task count: %d", len(listPayload.Data.Items))
	}
	if listPayload.Data.Items[0].TaskID != createPayload.Data.TaskID {
		t.Fatalf("unexpected listed task id: %s", listPayload.Data.Items[0].TaskID)
	}

	eventsResp, err := http.Get(server.URL + "/api/v1/tasks/" + createPayload.Data.TaskID + "/events")
	if err != nil {
		t.Fatalf("list task events request: %v", err)
	}
	defer eventsResp.Body.Close()

	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list task events status: %d", eventsResp.StatusCode)
	}

	var eventsPayload struct {
		Status string       `json:"status"`
		Data   []task.Event `json:"data"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsPayload); err != nil {
		t.Fatalf("decode task events response: %v", err)
	}

	if len(eventsPayload.Data) != 1 {
		t.Fatalf("unexpected task event count: %d", len(eventsPayload.Data))
	}
	if eventsPayload.Data[0].EventType != task.EventTypeTaskCreated {
		t.Fatalf("unexpected task event type: %s", eventsPayload.Data[0].EventType)
	}
	if eventsPayload.Data[0].TaskStatus != task.StatusPending {
		t.Fatalf("unexpected task event status: %s", eventsPayload.Data[0].TaskStatus)
	}
	if eventsPayload.Data[0].Topic != task.TopicTasks {
		t.Fatalf("unexpected task event topic: %s", eventsPayload.Data[0].Topic)
	}

	missingEventsResp, err := http.Get(server.URL + "/api/v1/tasks/task_missing/events")
	if err != nil {
		t.Fatalf("missing task events request: %v", err)
	}
	defer missingEventsResp.Body.Close()

	if missingEventsResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected missing task events status: %d", missingEventsResp.StatusCode)
	}
}

func TestAssignTaskAndTaskAck(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-dispatch-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-dispatch-001",
		"device_name": "Dispatch Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}

	registerBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}

	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerBytes))
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

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "hello-test-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"agent_uuid": "agent-dispatch-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), registerPayload.Data.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        time.Now().UTC().Format(time.RFC3339),
		Message:                          "test ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	createBody := map[string]any{
		"device_id":   registerPayload.Data.DeviceID,
		"script_name": "shoppe_sync",
		"priority":    1,
		"params": map[string]any{
			"shop_id": "shop-ack-001",
		},
	}

	createBytes, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	createResp, err := http.Post(server.URL+"/api/v1/tasks", "application/json", bytes.NewReader(createBytes))
	if err != nil {
		t.Fatalf("create task request: %v", err)
	}
	defer createResp.Body.Close()

	var createPayload struct {
		Data task.Task `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}

	assignBody := map[string]any{
		"task_id": createPayload.Data.TaskID,
		"action":  "assign",
	}
	assignBytes, err := json.Marshal(assignBody)
	if err != nil {
		t.Fatalf("marshal assign body: %v", err)
	}

	assignReq, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/tasks", bytes.NewReader(assignBytes))
	if err != nil {
		t.Fatalf("new assign request: %v", err)
	}
	assignReq.Header.Set("Content-Type", "application/json")

	assignResp, err := http.DefaultClient.Do(assignReq)
	if err != nil {
		t.Fatalf("assign task request: %v", err)
	}
	defer assignResp.Body.Close()

	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected assign task status: %d", assignResp.StatusCode)
	}

	var assignedPayload struct {
		Status string    `json:"status"`
		Data   task.Task `json:"data"`
	}
	if err := json.NewDecoder(assignResp.Body).Decode(&assignedPayload); err != nil {
		t.Fatalf("decode assign task response: %v", err)
	}
	if assignedPayload.Data.Status != task.StatusAssigned {
		t.Fatalf("unexpected assigned task status: %s", assignedPayload.Data.Status)
	}

	var assignMessage protocol.Envelope
	if err := conn.ReadJSON(&assignMessage); err != nil {
		t.Fatalf("read assign_task message: %v", err)
	}
	if assignMessage.Type != protocol.MessageTypeAssignTask {
		t.Fatalf("unexpected websocket message type: %s", assignMessage.Type)
	}

	var assignPayload struct {
		TaskID string `json:"task_id"`
	}
	assignPayloadBytes, err := json.Marshal(assignMessage.Payload)
	if err != nil {
		t.Fatalf("marshal assign payload: %v", err)
	}
	if err := json.Unmarshal(assignPayloadBytes, &assignPayload); err != nil {
		t.Fatalf("decode assign payload: %v", err)
	}
	if assignPayload.TaskID != createPayload.Data.TaskID {
		t.Fatalf("unexpected assigned task id: %s", assignPayload.TaskID)
	}

	taskAck := protocol.Envelope{
		Type:      protocol.MessageTypeTaskAck,
		RequestID: "task-ack-test-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id": createPayload.Data.TaskID,
			"status":  "ok",
			"message": "task ack ok",
		},
	}
	if err := conn.WriteJSON(taskAck); err != nil {
		t.Fatalf("write task_ack: %v", err)
	}

	var taskAckResp protocol.Envelope
	if err := conn.ReadJSON(&taskAckResp); err != nil {
		t.Fatalf("read task_ack ack: %v", err)
	}
	if taskAckResp.Type != "ack" {
		t.Fatalf("unexpected task_ack response type: %s", taskAckResp.Type)
	}

	if err := conn.WriteJSON(taskAck); err != nil {
		t.Fatalf("write duplicate task_ack: %v", err)
	}

	var duplicateTaskAckResp protocol.Envelope
	if err := conn.ReadJSON(&duplicateTaskAckResp); err != nil {
		t.Fatalf("read duplicate task_ack ack: %v", err)
	}
	if duplicateTaskAckResp.Type != "ack" {
		t.Fatalf("unexpected duplicate task_ack response type: %s", duplicateTaskAckResp.Type)
	}
	duplicatePayload, ok := duplicateTaskAckResp.Payload.(map[string]any)
	if !ok {
		t.Fatalf("unexpected duplicate task_ack payload type: %T", duplicateTaskAckResp.Payload)
	}
	if duplicatePayload["status"] != "ok" {
		t.Fatalf("unexpected duplicate task_ack response status: %v", duplicatePayload["status"])
	}

	eventsResp, err := http.Get(server.URL + "/api/v1/tasks/" + createPayload.Data.TaskID + "/events")
	if err != nil {
		t.Fatalf("list task events request: %v", err)
	}
	defer eventsResp.Body.Close()

	var eventsPayload struct {
		Data []task.Event `json:"data"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsPayload); err != nil {
		t.Fatalf("decode task events response: %v", err)
	}
	if len(eventsPayload.Data) != 4 {
		t.Fatalf("expected exactly 4 task events after duplicate task_ack, got %d", len(eventsPayload.Data))
	}
	if eventsPayload.Data[1].EventType != task.EventTypeTaskAssigned {
		t.Fatalf("unexpected second task event type: %s", eventsPayload.Data[1].EventType)
	}
	if eventsPayload.Data[2].EventType != task.EventTypeTaskAck {
		t.Fatalf("unexpected third task event type: %s", eventsPayload.Data[2].EventType)
	}
	if got := eventsPayload.Data[2].Extra["request_id"]; got != "task-ack-test-001" {
		t.Fatalf("unexpected task_ack request_id: %v", got)
	}
	if eventsPayload.Data[3].EventType != task.EventTypeTaskRunning {
		t.Fatalf("unexpected fourth task event type: %s", eventsPayload.Data[3].EventType)
	}
}

func TestTaskResultUpdatesTaskStatus(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-task-result-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-result-001",
		"device_name": "Result Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}

	registerBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}

	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerBytes))
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

	wsURL := "ws" + server.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	hello := protocol.Envelope{
		Type:      protocol.MessageTypeHello,
		RequestID: "hello-result-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"agent_uuid": "agent-result-001",
		},
	}
	if err := conn.WriteJSON(hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	var helloAck protocol.Envelope
	if err := conn.ReadJSON(&helloAck); err != nil {
		t.Fatalf("read hello ack: %v", err)
	}

	if err := deviceService.UpdateExecutionProfile(t.Context(), registerPayload.Data.DeviceID, device.ExecutionProfile{
		AccessibilityStatus:              "enabled",
		ForegroundServiceStatus:          "enabled",
		BatteryOptimizationIgnoredStatus: "enabled",
		CheckedAt:                        time.Now().UTC().Format(time.RFC3339),
		Message:                          "test ready",
	}); err != nil {
		t.Fatalf("update execution profile: %v", err)
	}

	createBody := map[string]any{
		"device_id":      registerPayload.Data.DeviceID,
		"script_name":    "shoppe_sync",
		"script_version": "v0.1.0",
		"priority":       1,
		"params": map[string]any{
			"should_fail": false,
		},
	}

	createBytes, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	createResp, err := http.Post(server.URL+"/api/v1/tasks", "application/json", bytes.NewReader(createBytes))
	if err != nil {
		t.Fatalf("create task request: %v", err)
	}
	defer createResp.Body.Close()

	var createPayload struct {
		Data task.Task `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create task response: %v", err)
	}

	assignBody := map[string]any{
		"task_id": createPayload.Data.TaskID,
		"action":  "assign",
	}
	assignBytes, err := json.Marshal(assignBody)
	if err != nil {
		t.Fatalf("marshal assign body: %v", err)
	}

	assignReq, err := http.NewRequest(http.MethodPatch, server.URL+"/api/v1/tasks", bytes.NewReader(assignBytes))
	if err != nil {
		t.Fatalf("new assign request: %v", err)
	}
	assignReq.Header.Set("Content-Type", "application/json")

	assignResp, err := http.DefaultClient.Do(assignReq)
	if err != nil {
		t.Fatalf("assign task request: %v", err)
	}
	defer assignResp.Body.Close()

	var assignMessage protocol.Envelope
	if err := conn.ReadJSON(&assignMessage); err != nil {
		t.Fatalf("read assign_task message: %v", err)
	}

	taskAck := protocol.Envelope{
		Type:      protocol.MessageTypeTaskAck,
		RequestID: "task-ack-result-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id": createPayload.Data.TaskID,
			"status":  "ok",
			"message": "task ack ok",
		},
	}
	if err := conn.WriteJSON(taskAck); err != nil {
		t.Fatalf("write task_ack: %v", err)
	}

	var taskAckResp protocol.Envelope
	if err := conn.ReadJSON(&taskAckResp); err != nil {
		t.Fatalf("read task_ack ack: %v", err)
	}

	taskProgress := protocol.Envelope{
		Type:      protocol.MessageTypeTaskProgress,
		RequestID: "task-progress-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id":   createPayload.Data.TaskID,
			"status":    "running",
			"step_name": "CHECK_HOME",
			"message":   "任务执行中：正在检查首页状态",
			"extra": map[string]any{
				"source": "agent",
			},
		},
	}
	if err := conn.WriteJSON(taskProgress); err != nil {
		t.Fatalf("write task_progress: %v", err)
	}

	var taskProgressResp protocol.Envelope
	if err := conn.ReadJSON(&taskProgressResp); err != nil {
		t.Fatalf("read task_progress ack: %v", err)
	}
	if taskProgressResp.Type != "ack" {
		t.Fatalf("unexpected task_progress response type: %s", taskProgressResp.Type)
	}

	taskResult := protocol.Envelope{
		Type:      protocol.MessageTypeTaskResult,
		RequestID: "task-result-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id":         createPayload.Data.TaskID,
			"status":          "success",
			"result_code":     "OK",
			"result_message":  "script done",
			"step_name":       "RUN_SCRIPT_ENTRY",
			"extra": map[string]any{
				"mode": "builtin",
			},
		},
	}
	if err := conn.WriteJSON(taskResult); err != nil {
		t.Fatalf("write task_result: %v", err)
	}

	var taskResultResp protocol.Envelope
	if err := conn.ReadJSON(&taskResultResp); err != nil {
		t.Fatalf("read task_result ack: %v", err)
	}
	if taskResultResp.Type != "ack" {
		t.Fatalf("unexpected task_result response type: %s", taskResultResp.Type)
	}

	taskResp, err := http.Get(server.URL + "/api/v1/tasks")
	if err != nil {
		t.Fatalf("list tasks request: %v", err)
	}
	defer taskResp.Body.Close()

	var taskListPayload struct {
		Data struct {
			Items []task.Task `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(taskResp.Body).Decode(&taskListPayload); err != nil {
		t.Fatalf("decode task list response: %v", err)
	}

	if len(taskListPayload.Data.Items) != 1 {
		t.Fatalf("unexpected task count: %d", len(taskListPayload.Data.Items))
	}
	if taskListPayload.Data.Items[0].Status != task.StatusSuccess {
		t.Fatalf("unexpected task status after task_result: %s", taskListPayload.Data.Items[0].Status)
	}
	if taskListPayload.Data.Items[0].ResultCode != "OK" {
		t.Fatalf("unexpected task result code: %s", taskListPayload.Data.Items[0].ResultCode)
	}

	eventsResp, err := http.Get(server.URL + "/api/v1/tasks/" + createPayload.Data.TaskID + "/events")
	if err != nil {
		t.Fatalf("list task events request: %v", err)
	}
	defer eventsResp.Body.Close()

	var eventsPayload struct {
		Data []task.Event `json:"data"`
	}
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsPayload); err != nil {
		t.Fatalf("decode task events response: %v", err)
	}

	if len(eventsPayload.Data) != 6 {
		t.Fatalf("expected 6 task events after task_result, got %d", len(eventsPayload.Data))
	}
	if eventsPayload.Data[3].EventType != task.EventTypeTaskRunning {
		t.Fatalf("unexpected running event type: %s", eventsPayload.Data[3].EventType)
	}
	if eventsPayload.Data[4].EventType != task.EventTypeTaskProgress {
		t.Fatalf("unexpected progress event type: %s", eventsPayload.Data[4].EventType)
	}
	if eventsPayload.Data[4].StepName != "CHECK_HOME" {
		t.Fatalf("unexpected progress step name: %s", eventsPayload.Data[4].StepName)
	}
	if eventsPayload.Data[5].EventType != task.EventTypeTaskResult {
		t.Fatalf("unexpected result event type: %s", eventsPayload.Data[5].EventType)
	}
	if eventsPayload.Data[5].TaskStatus != task.StatusSuccess {
		t.Fatalf("unexpected result event task status: %s", eventsPayload.Data[5].TaskStatus)
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []string{"http://localhost:5173"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}

func TestCORSPreflightRejectsUnknownOrigin(t *testing.T) {
	t.Parallel()

	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), []string{"http://localhost:5173"})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestDiscoveryDeployAgentRequiresEndpoints(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-discovery-deploy-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	body := []byte(`{"adb_endpoints":[],"center_base_url":"http://127.0.0.1:8080","reset_config":false,"run_agent":true}`)
	resp, err := http.Post(server.URL+"/api/v1/discovery/agent-deployments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("deploy agents request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected deploy agents status: %d", resp.StatusCode)
	}

	var payload struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode deploy agents response: %v", err)
	}

	if payload.Error != "adb_endpoints_required" {
		t.Fatalf("unexpected deploy agents error: %s", payload.Error)
	}
}

func TestDiscoveryControlAgentRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-discovery-action-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	body := []byte(`{"adb_endpoint":"192.168.0.120:37123","action":"restart"}`)
	resp, err := http.Post(server.URL+"/api/v1/discovery/agent-actions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("control agent request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected control agent status: %d", resp.StatusCode)
	}

	var payload struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode control agent response: %v", err)
	}

	if payload.Error != "agent_action_unsupported" {
		t.Fatalf("unexpected control agent error: %s", payload.Error)
	}
}

func TestDiscoveryPairRequiresHostPortAndCode(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-discovery-pair-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	testCases := []struct {
		name          string
		body          string
		expectedError string
	}{
		{
			name:          "missing host",
			body:          `{"host":"","port":"37123","pair_code":"123456"}`,
			expectedError: "pair_host_required",
		},
		{
			name:          "missing port",
			body:          `{"host":"192.168.0.120","port":"","pair_code":"123456"}`,
			expectedError: "pair_port_required",
		},
		{
			name:          "missing pair code",
			body:          `{"host":"192.168.0.120","port":"37123","pair_code":""}`,
			expectedError: "pair_code_required",
		},
	}

	for _, item := range testCases {
		t.Run(item.name, func(t *testing.T) {
			resp, err := http.Post(server.URL+"/api/v1/discovery/pair", "application/json", bytes.NewReader([]byte(item.body)))
			if err != nil {
				t.Fatalf("pair device request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("unexpected pair status: %d", resp.StatusCode)
			}

			var payload struct {
				Status string `json:"status"`
				Error  string `json:"error"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				t.Fatalf("decode pair device response: %v", err)
			}
			if payload.Error != item.expectedError {
				t.Fatalf("unexpected pair device error: %s", payload.Error)
			}
		})
	}
}

func TestDiscoverySettingsRoundTrip(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-settings-test.db")
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, settingsService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	saveBody := []byte(`{"center_base_url":"http://192.168.0.155:28080"}`)
	saveReq, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/settings/discovery", bytes.NewReader(saveBody))
	if err != nil {
		t.Fatalf("new save settings request: %v", err)
	}
	saveReq.Header.Set("Content-Type", "application/json")

	saveResp, err := http.DefaultClient.Do(saveReq)
	if err != nil {
		t.Fatalf("save settings request: %v", err)
	}
	defer saveResp.Body.Close()
	if saveResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected save settings status: %d", saveResp.StatusCode)
	}

	getResp, err := http.Get(server.URL + "/api/v1/settings/discovery")
	if err != nil {
		t.Fatalf("get settings request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected get settings status: %d", getResp.StatusCode)
	}

	var payload struct {
		Data struct {
			CenterBaseURL string `json:"center_base_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode get settings response: %v", err)
	}
	if payload.Data.CenterBaseURL != "http://192.168.0.155:28080" {
		t.Fatalf("unexpected center_base_url: %s", payload.Data.CenterBaseURL)
	}
}




package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/discovery"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/script"
	"github.com/mobilerpa/mobilerpa-center/server/internal/settings"
	"github.com/mobilerpa/mobilerpa-center/server/internal/software"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

func TestScalarDocsAndOpenAPIDocument(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-docs-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	openAPIResp, err := http.Get(server.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("openapi request: %v", err)
	}
	defer openAPIResp.Body.Close()

	if openAPIResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected openapi status: %d", openAPIResp.StatusCode)
	}

	var openAPIPayload map[string]any
	if err := json.NewDecoder(openAPIResp.Body).Decode(&openAPIPayload); err != nil {
		t.Fatalf("decode openapi response: %v", err)
	}
	if openAPIPayload["openapi"] != "3.0.3" {
		t.Fatalf("unexpected openapi version: %#v", openAPIPayload["openapi"])
	}

	scalarResp, err := http.Get(server.URL + "/scalar")
	if err != nil {
		t.Fatalf("scalar request: %v", err)
	}
	defer scalarResp.Body.Close()

	if scalarResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected scalar status: %d", scalarResp.StatusCode)
	}

	bodyBytes := new(bytes.Buffer)
	if _, err := bodyBytes.ReadFrom(scalarResp.Body); err != nil {
		t.Fatalf("read scalar response: %v", err)
	}
	body := bodyBytes.String()
	if !strings.Contains(body, "@scalar/api-reference") {
		t.Fatalf("scalar page should include scalar script")
	}
	if !strings.Contains(body, "/openapi.json") {
		t.Fatalf("scalar page should reference openapi.json")
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

func TestListAllDevices(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-list-all-devices.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	for index := 1; index <= 2; index++ {
		registerBody := map[string]any{
			"agent_uuid":  "agent-list-all-" + strconv.Itoa(index),
			"device_name": "Device " + strconv.Itoa(index),
			"brand":       "Google",
			"model":       "Pixel 8",
		}
		bodyBytes, err := json.Marshal(registerBody)
		if err != nil {
			t.Fatalf("marshal register body: %v", err)
		}
		resp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("register request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected register status: %d", resp.StatusCode)
		}
	}

	resp, err := http.Get(server.URL + "/api/v1/devices/all")
	if err != nil {
		t.Fatalf("list all devices request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list all devices status: %d", resp.StatusCode)
	}

	var payload struct {
		Status string          `json:"status"`
		Data   []device.Device `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode list all devices response: %v", err)
	}

	if payload.Status != "ok" {
		t.Fatalf("unexpected list all devices payload status: %s", payload.Status)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("unexpected list all devices count: %d", len(payload.Data))
	}
}

func TestListAllSoftware(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-list-all-software.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	softwareRoot := filepath.Join(t.TempDir(), "software-library")
	softwareService := software.NewService(db, softwareRoot)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, softwareService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	for index := 1; index <= 2; index++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		if err := writer.WriteField("software_name", "软件"+strconv.Itoa(index)); err != nil {
			t.Fatalf("write software_name: %v", err)
		}
		if err := writer.WriteField("description", "描述"+strconv.Itoa(index)); err != nil {
			t.Fatalf("write description: %v", err)
		}
		part, err := writer.CreateFormFile("file", "pkg"+strconv.Itoa(index)+".apk")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte("apk-bytes")); err != nil {
			t.Fatalf("write file body: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close writer: %v", err)
		}

		resp, err := http.Post(server.URL+"/api/v1/software", writer.FormDataContentType(), body)
		if err != nil {
			t.Fatalf("create software request: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected create software status: %d", resp.StatusCode)
		}
	}

	resp, err := http.Get(server.URL + "/api/v1/software/all")
	if err != nil {
		t.Fatalf("list all software request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list all software status: %d", resp.StatusCode)
	}

	var payload struct {
		Status string             `json:"status"`
		Data   []software.Package `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode list all software response: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("unexpected list all software payload status: %s", payload.Status)
	}
	if len(payload.Data) != 2 {
		t.Fatalf("unexpected list all software count: %d", len(payload.Data))
	}
}

func TestLocationNodeRoutes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-device-slot-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-slot-route-001",
		"device_name": "Slot Route Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}
	registerBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}
	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerBytes))
	if err != nil {
		t.Fatalf("register device request: %v", err)
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

	createZoneResp, err := http.Post(server.URL+"/api/v1/location-nodes", "application/json", bytes.NewReader([]byte(`{"node_type":"zone","node_name":"A区"}`)))
	if err != nil {
		t.Fatalf("create zone request: %v", err)
	}
	defer createZoneResp.Body.Close()
	if createZoneResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create zone status: %d", createZoneResp.StatusCode)
	}

	var createZonePayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(createZoneResp.Body).Decode(&createZonePayload); err != nil {
		t.Fatalf("decode create zone response: %v", err)
	}
	createRowBody, err := json.Marshal(map[string]any{
		"parent_id": createZonePayload.Data.NodeID,
		"node_type": "row",
		"node_name": "第2排",
	})
	if err != nil {
		t.Fatalf("marshal create row body: %v", err)
	}
	createRowResp, err := http.Post(server.URL+"/api/v1/location-nodes", "application/json", bytes.NewReader(createRowBody))
	if err != nil {
		t.Fatalf("create row request: %v", err)
	}
	defer createRowResp.Body.Close()

	var createRowPayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(createRowResp.Body).Decode(&createRowPayload); err != nil {
		t.Fatalf("decode create row response: %v", err)
	}

	createSlotBody, err := json.Marshal(map[string]any{
		"parent_id": createRowPayload.Data.NodeID,
		"node_type": "slot",
		"node_name": "08",
	})
	if err != nil {
		t.Fatalf("marshal create slot body: %v", err)
	}
	createSlotResp, err := http.Post(server.URL+"/api/v1/location-nodes", "application/json", bytes.NewReader(createSlotBody))
	if err != nil {
		t.Fatalf("create slot request: %v", err)
	}
	defer createSlotResp.Body.Close()
	if createSlotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create slot status: %d", createSlotResp.StatusCode)
	}

	var createSlotPayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(createSlotResp.Body).Decode(&createSlotPayload); err != nil {
		t.Fatalf("decode create slot response: %v", err)
	}
	if createSlotPayload.Data.PathText != "A区-第2排-08" {
		t.Fatalf("unexpected created slot path: %#v", createSlotPayload.Data)
	}

	listSlotsResp, err := http.Get(server.URL + "/api/v1/location-nodes")
	if err != nil {
		t.Fatalf("list location nodes request: %v", err)
	}
	defer listSlotsResp.Body.Close()
	if listSlotsResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list slots status: %d", listSlotsResp.StatusCode)
	}

	var listSlotsPayload struct {
		Data []device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(listSlotsResp.Body).Decode(&listSlotsPayload); err != nil {
		t.Fatalf("decode list slots response: %v", err)
	}
	if len(listSlotsPayload.Data) != 3 {
		t.Fatalf("unexpected location node count: %d", len(listSlotsPayload.Data))
	}

	bindBodyBytes, err := json.Marshal(map[string]any{
		"device_id": registerPayload.Data.DeviceID,
	})
	if err != nil {
		t.Fatalf("marshal bind body: %v", err)
	}
	bindResp, err := http.Post(server.URL+"/api/v1/location-nodes/"+createSlotPayload.Data.NodeID+"/bind", "application/json", bytes.NewReader(bindBodyBytes))
	if err != nil {
		t.Fatalf("bind slot request: %v", err)
	}
	defer bindResp.Body.Close()
	if bindResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bind slot status: %d", bindResp.StatusCode)
	}

	var bindPayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(bindResp.Body).Decode(&bindPayload); err != nil {
		t.Fatalf("decode bind slot response: %v", err)
	}
	if bindPayload.Data.DeviceID != registerPayload.Data.DeviceID {
		t.Fatalf("unexpected bound slot device id: %s", bindPayload.Data.DeviceID)
	}

	deviceResp, err := http.Get(server.URL + "/api/v1/devices/" + registerPayload.Data.DeviceID)
	if err != nil {
		t.Fatalf("get device request: %v", err)
	}
	defer deviceResp.Body.Close()

	var devicePayload struct {
		Data device.Device `json:"data"`
	}
	if err := json.NewDecoder(deviceResp.Body).Decode(&devicePayload); err != nil {
		t.Fatalf("decode device response: %v", err)
	}
	if devicePayload.Data.PhysicalSlot != "A区-第2排-08" {
		t.Fatalf("unexpected physical slot after bind: %s", devicePayload.Data.PhysicalSlot)
	}
	if devicePayload.Data.SlotZone != "A区" || devicePayload.Data.SlotRow != "第2排" || devicePayload.Data.SlotPosition != "08" {
		t.Fatalf("unexpected split slot fields after bind: %#v", devicePayload.Data)
	}

	unbindResp, err := http.Post(server.URL+"/api/v1/location-nodes/"+createSlotPayload.Data.NodeID+"/unbind", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatalf("unbind slot request: %v", err)
	}
	defer unbindResp.Body.Close()
	if unbindResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected unbind slot status: %d", unbindResp.StatusCode)
	}

	var unbindPayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(unbindResp.Body).Decode(&unbindPayload); err != nil {
		t.Fatalf("decode unbind slot response: %v", err)
	}
	if unbindPayload.Data.DeviceID != "" {
		t.Fatalf("expected empty slot after unbind, got %q", unbindPayload.Data.DeviceID)
	}
}

func TestUpdateAndDeleteLocationNodeRoutes(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-location-node-update-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	registerBody := map[string]any{
		"agent_uuid":  "agent-route-node-001",
		"device_name": "Node Route Device",
		"brand":       "Google",
		"model":       "Pixel 8",
	}
	registerBytes, err := json.Marshal(registerBody)
	if err != nil {
		t.Fatalf("marshal register body: %v", err)
	}
	registerResp, err := http.Post(server.URL+"/api/v1/device/register", "application/json", bytes.NewReader(registerBytes))
	if err != nil {
		t.Fatalf("register device request: %v", err)
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

	createNode := func(body map[string]any) device.LocationNode {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal create node body: %v", err)
		}
		resp, err := http.Post(server.URL+"/api/v1/location-nodes", "application/json", bytes.NewReader(bodyBytes))
		if err != nil {
			t.Fatalf("create node request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected create node status: %d", resp.StatusCode)
		}
		var payload struct {
			Data device.LocationNode `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("decode create node response: %v", err)
		}
		return payload.Data
	}

	zoneA := createNode(map[string]any{"node_type": "zone", "node_name": "A区"})
	zoneB := createNode(map[string]any{"node_type": "zone", "node_name": "B区"})
	row := createNode(map[string]any{"parent_id": zoneA.NodeID, "node_type": "row", "node_name": "第1排"})
	slot := createNode(map[string]any{"parent_id": row.NodeID, "node_type": "slot", "node_name": "01"})

	bindBodyBytes, err := json.Marshal(map[string]any{
		"device_id": registerPayload.Data.DeviceID,
	})
	if err != nil {
		t.Fatalf("marshal bind body: %v", err)
	}
	bindResp, err := http.Post(server.URL+"/api/v1/location-nodes/"+slot.NodeID+"/bind", "application/json", bytes.NewReader(bindBodyBytes))
	if err != nil {
		t.Fatalf("bind slot request: %v", err)
	}
	defer bindResp.Body.Close()
	if bindResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bind slot status: %d", bindResp.StatusCode)
	}

	updateBodyBytes, err := json.Marshal(map[string]any{
		"parent_id":  zoneB.NodeID,
		"node_name":  "第9排",
		"sort_order": 9,
	})
	if err != nil {
		t.Fatalf("marshal update node body: %v", err)
	}
	updateReq, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/location-nodes/"+row.NodeID, bytes.NewReader(updateBodyBytes))
	if err != nil {
		t.Fatalf("new update location node request: %v", err)
	}
	updateReq.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("update location node request: %v", err)
	}
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected update location node status: %d", updateResp.StatusCode)
	}

	var updatePayload struct {
		Data device.LocationNode `json:"data"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updatePayload); err != nil {
		t.Fatalf("decode update location node response: %v", err)
	}
	if updatePayload.Data.ParentID != zoneB.NodeID || updatePayload.Data.PathText != "B区-第9排" {
		t.Fatalf("unexpected updated row response: %#v", updatePayload.Data)
	}

	deviceResp, err := http.Get(server.URL + "/api/v1/devices/" + registerPayload.Data.DeviceID)
	if err != nil {
		t.Fatalf("get device request after node update: %v", err)
	}
	defer deviceResp.Body.Close()

	var devicePayload struct {
		Data device.Device `json:"data"`
	}
	if err := json.NewDecoder(deviceResp.Body).Decode(&devicePayload); err != nil {
		t.Fatalf("decode device response after node update: %v", err)
	}
	if devicePayload.Data.PhysicalSlot != "B区-第9排-01" {
		t.Fatalf("unexpected physical slot after node update: %s", devicePayload.Data.PhysicalSlot)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/location-nodes/"+row.NodeID, nil)
	if err != nil {
		t.Fatalf("new delete location node request: %v", err)
	}
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("delete location node request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected delete location node status: %d", deleteResp.StatusCode)
	}

	deviceRespAfterDelete, err := http.Get(server.URL + "/api/v1/devices/" + registerPayload.Data.DeviceID)
	if err != nil {
		t.Fatalf("get device request after node delete: %v", err)
	}
	defer deviceRespAfterDelete.Body.Close()

	var devicePayloadAfterDelete struct {
		Data device.Device `json:"data"`
	}
	if err := json.NewDecoder(deviceRespAfterDelete.Body).Decode(&devicePayloadAfterDelete); err != nil {
		t.Fatalf("decode device response after node delete: %v", err)
	}
	if devicePayloadAfterDelete.Data.PhysicalSlot != "" || devicePayloadAfterDelete.Data.BindStatus != "pending" {
		t.Fatalf("unexpected device fields after node delete: %#v", devicePayloadAfterDelete.Data)
	}
}

func TestUpdateWorkflowDefinition(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-update-workflow-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	workflowService := workflow.NewService(db, deviceService, taskService, nil)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, workflowService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	createBody := map[string]any{
		"workflow_name": "原始工作流",
		"description":   "原始说明",
		"status":        "active",
		"nodes": []map[string]any{
			{"node_id": "node_1", "node_type": "script", "node_name": "打开QQ", "script_name": "qq", "script_version": "v1"},
			{"node_id": "node_2", "node_type": "stop", "node_name": "结束"},
		},
		"edges": []map[string]any{
			{"from_node_id": "node_1", "to_node_id": "node_2", "edge_type": "next"},
		},
	}
	createBytes, err := json.Marshal(createBody)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	createResp, err := http.Post(server.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createBytes))
	if err != nil {
		t.Fatalf("create workflow request: %v", err)
	}
	defer createResp.Body.Close()

	var createPayload struct {
		Data workflow.Definition `json:"data"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&createPayload); err != nil {
		t.Fatalf("decode create workflow response: %v", err)
	}

	updateBody := map[string]any{
		"workflow_name": "更新后的工作流",
		"description":   "更新后的说明",
		"status":        "draft",
		"nodes": []map[string]any{
			{"node_id": "node_1", "node_type": "script", "node_name": "打开微信", "script_name": "wechat", "script_version": "v2"},
			{"node_id": "node_2", "node_type": "stop", "node_name": "结束"},
		},
		"edges": []map[string]any{
			{"from_node_id": "node_1", "to_node_id": "node_2", "edge_type": "next"},
		},
	}
	updateBytes, err := json.Marshal(updateBody)
	if err != nil {
		t.Fatalf("marshal update body: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, server.URL+"/api/v1/workflows/"+createPayload.Data.WorkflowDefID, bytes.NewReader(updateBytes))
	if err != nil {
		t.Fatalf("new update workflow request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update workflow request: %v", err)
	}
	defer updateResp.Body.Close()

	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected update workflow status: %d", updateResp.StatusCode)
	}

	var updatePayload struct {
		Status string              `json:"status"`
		Data   workflow.Definition `json:"data"`
	}
	if err := json.NewDecoder(updateResp.Body).Decode(&updatePayload); err != nil {
		t.Fatalf("decode update workflow response: %v", err)
	}

	if updatePayload.Status != "ok" {
		t.Fatalf("unexpected update payload status: %s", updatePayload.Status)
	}
	if updatePayload.Data.WorkflowName != "更新后的工作流" {
		t.Fatalf("unexpected workflow name: %s", updatePayload.Data.WorkflowName)
	}
	if updatePayload.Data.Description != "更新后的说明" {
		t.Fatalf("unexpected workflow description: %s", updatePayload.Data.Description)
	}
	if updatePayload.Data.Status != "draft" {
		t.Fatalf("unexpected workflow status: %s", updatePayload.Data.Status)
	}
	if len(updatePayload.Data.Nodes) != 2 {
		t.Fatalf("unexpected workflow node count: %d", len(updatePayload.Data.Nodes))
	}
	if updatePayload.Data.Nodes[0].NodeName != "打开微信" {
		t.Fatalf("unexpected updated node name: %s", updatePayload.Data.Nodes[0].NodeName)
	}

	updated, err := workflowService.GetDefinition(context.Background(), createPayload.Data.WorkflowDefID)
	if err != nil {
		t.Fatalf("get updated workflow: %v", err)
	}
	if updated.WorkflowName != "更新后的工作流" {
		t.Fatalf("unexpected persisted workflow name: %s", updated.WorkflowName)
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

	createdTask, err := taskService.Create(t.Context(), task.CreateRequest{
		DeviceID:      registerPayload.Data.DeviceID,
		ScriptName:    "shoppe_sync",
		ScriptVersion: "v0.1.0",
		Priority:      3,
		Params: map[string]any{
			"shop_id": "shop-001",
			"mode":    "dry-run",
		},
		ScheduledAt: "2026-06-12T10:30:00Z",
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	var createPayload struct {
		Status string    `json:"status"`
		Data   task.Task `json:"data"`
	}
	createPayload.Status = "ok"
	createPayload.Data = createdTask

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

	listedTasks, err := taskService.List(t.Context(), "")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(listedTasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(listedTasks))
	}
	if listedTasks[0].TaskID != createPayload.Data.TaskID {
		t.Fatalf("unexpected listed task id: %s", listedTasks[0].TaskID)
	}

	events, err := taskService.ListEvents(t.Context(), createPayload.Data.TaskID)
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("unexpected task event count: %d", len(events))
	}
	if events[0].EventType != task.EventTypeTaskCreated {
		t.Fatalf("unexpected task event type: %s", events[0].EventType)
	}
	if events[0].TaskStatus != task.StatusPending {
		t.Fatalf("unexpected task event status: %s", events[0].TaskStatus)
	}
	if events[0].Topic != task.TopicTasks {
		t.Fatalf("unexpected task event topic: %s", events[0].Topic)
	}

	if _, err := taskService.ListEvents(t.Context(), "task_missing"); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("expected task not found for missing task events, got: %v", err)
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

	createdTask, err := taskService.Create(t.Context(), task.CreateRequest{
		DeviceID:   registerPayload.Data.DeviceID,
		ScriptName: "shoppe_sync",
		Priority:   1,
		Params:     map[string]any{"shop_id": "shop-ack-001"},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if _, err := dispatchService.AssignTask(t.Context(), createdTask.TaskID); err != nil {
		t.Fatalf("assign task: %v", err)
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
	if assignPayload.TaskID != createdTask.TaskID {
		t.Fatalf("unexpected assigned task id: %s", assignPayload.TaskID)
	}

	taskAck := protocol.Envelope{
		Type:      protocol.MessageTypeTaskAck,
		RequestID: "task-ack-test-001",
		DeviceID:  registerPayload.Data.DeviceID,
		Timestamp: time.Now().Unix(),
		Payload: map[string]any{
			"task_id": createdTask.TaskID,
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

	events, err := taskService.ListEvents(t.Context(), createdTask.TaskID)
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected exactly 4 task events after duplicate task_ack, got %d", len(events))
	}
	if events[1].EventType != task.EventTypeTaskAssigned {
		t.Fatalf("unexpected second task event type: %s", events[1].EventType)
	}
	if events[2].EventType != task.EventTypeTaskAck {
		t.Fatalf("unexpected third task event type: %s", events[2].EventType)
	}
	if got := events[2].Extra["request_id"]; got != "task-ack-test-001" {
		t.Fatalf("unexpected task_ack request_id: %v", got)
	}
	if events[3].EventType != task.EventTypeTaskRunning {
		t.Fatalf("unexpected fourth task event type: %s", events[3].EventType)
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

	createdTask, err := taskService.Create(t.Context(), task.CreateRequest{
		DeviceID:      registerPayload.Data.DeviceID,
		ScriptName:    "shoppe_sync",
		ScriptVersion: "v0.1.0",
		Priority:      1,
		Params:        map[string]any{"should_fail": false},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	var createPayload struct {
		Data task.Task `json:"data"`
	}
	createPayload.Data = createdTask

	if _, err := dispatchService.AssignTask(t.Context(), createdTask.TaskID); err != nil {
		t.Fatalf("assign task: %v", err)
	}

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
			"task_id":        createPayload.Data.TaskID,
			"status":         "success",
			"result_code":    "OK",
			"result_message": "script done",
			"step_name":      "RUN_SCRIPT_ENTRY",
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

	listedTasks, err := taskService.List(t.Context(), "")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(listedTasks) != 1 {
		t.Fatalf("unexpected task count: %d", len(listedTasks))
	}
	if listedTasks[0].Status != task.StatusSuccess {
		t.Fatalf("unexpected task status after task_result: %s", listedTasks[0].Status)
	}
	if listedTasks[0].ResultCode != "OK" {
		t.Fatalf("unexpected task result code: %s", listedTasks[0].ResultCode)
	}

	events, err := taskService.ListEvents(t.Context(), createPayload.Data.TaskID)
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("expected 6 task events after task_result, got %d", len(events))
	}
	if events[3].EventType != task.EventTypeTaskRunning {
		t.Fatalf("unexpected running event type: %s", events[3].EventType)
	}
	if events[4].EventType != task.EventTypeTaskProgress {
		t.Fatalf("unexpected progress event type: %s", events[4].EventType)
	}
	if events[4].StepName != "CHECK_HOME" {
		t.Fatalf("unexpected progress step name: %s", events[4].StepName)
	}
	if events[5].EventType != task.EventTypeTaskResult {
		t.Fatalf("unexpected result event type: %s", events[5].EventType)
	}
	if events[5].TaskStatus != task.StatusSuccess {
		t.Fatalf("unexpected result event task status: %s", events[5].TaskStatus)
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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

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

package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/internal/ws"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

func TestScriptUploadManifestAndDownload(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptService := script.NewService(db, filepath.Join(t.TempDir(), "script-library"))
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadURL := server.URL + "/api/v1/scripts/upload"
	uploadBody, contentType := buildUploadRequestBody(t, "shoppe_sync", "v0.1.1", "zip", map[string]string{
		"index.js":             "\"use strict\";\nmodule.exports = { run: function () { return { status: \"success\", result_code: \"OK\", result_message: \"ok\", step_name: \"START\", extra: {} }; } };\n",
		"utils/ghostmobile.js": "\"use strict\";\nmodule.exports = {};\n",
	})

	uploadResp, err := http.Post(uploadURL, contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
	}

	listResp, err := http.Get(server.URL + "/api/v1/scripts")
	if err != nil {
		t.Fatalf("list scripts request: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list scripts status: %d", listResp.StatusCode)
	}

	var listPayload struct {
		Status string `json:"status"`
		Data   struct {
			Items []script.ScriptSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list scripts response: %v", err)
	}
	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("unexpected script count: %d", len(listPayload.Data.Items))
	}
	if len(listPayload.Data.Items[0].Versions) != 1 {
		t.Fatalf("unexpected version count: %d", len(listPayload.Data.Items[0].Versions))
	}

	manifestResp, err := http.Get(server.URL + "/api/v1/scripts/shoppe_sync/versions/v0.1.1")
	if err != nil {
		t.Fatalf("manifest request: %v", err)
	}
	defer manifestResp.Body.Close()

	if manifestResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected manifest status: %d", manifestResp.StatusCode)
	}

	var manifestPayload struct {
		Status string          `json:"status"`
		Data   script.Manifest `json:"data"`
	}
	if err := json.NewDecoder(manifestResp.Body).Decode(&manifestPayload); err != nil {
		t.Fatalf("decode manifest response: %v", err)
	}
	if manifestPayload.Data.EntryFile != "index.js" {
		t.Fatalf("unexpected manifest entry_file: %s", manifestPayload.Data.EntryFile)
	}
	if manifestPayload.Data.SourceType != "zip" {
		t.Fatalf("unexpected manifest source_type: %s", manifestPayload.Data.SourceType)
	}
	if len(manifestPayload.Data.Files) != 2 {
		t.Fatalf("unexpected manifest files count: %d", len(manifestPayload.Data.Files))
	}

	downloadResp, err := http.Get(server.URL + "/api/v1/script/download?script_name=shoppe_sync&script_version=v0.1.1&relative_path=index.js")
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected download status: %d", downloadResp.StatusCode)
	}
	if got := downloadResp.Header.Get("X-Script-Relative-Path"); got != "index.js" {
		t.Fatalf("unexpected relative path header: %s", got)
	}
}

func TestScriptUploadSupportsSingleRootDirectoryZip(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-rootdir-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptService := script.NewService(db, filepath.Join(t.TempDir(), "script-library"))
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadURL := server.URL + "/api/v1/scripts/upload"
	uploadBody, contentType := buildUploadRequestBody(t, "shoppe_sync", "v0.1.2", "zip", map[string]string{
		"shoppe_sync_v0.1.2/index.js":         "\"use strict\";\nmodule.exports = { run: function () { return { status: \"success\" }; } };\n",
		"shoppe_sync_v0.1.2/utils/common.js":  "\"use strict\";\nmodule.exports = {};\n",
		"shoppe_sync_v0.1.2/config/config.js": "\"use strict\";\nmodule.exports = {};\n",
		"shoppe_sync_v0.1.2/index_debug.js":   "\"use strict\";\n",
	})

	uploadResp, err := http.Post(uploadURL, contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
	}

	manifestResp, err := http.Get(server.URL + "/api/v1/scripts/shoppe_sync/versions/v0.1.2")
	if err != nil {
		t.Fatalf("manifest request: %v", err)
	}
	defer manifestResp.Body.Close()
	if manifestResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected manifest status: %d", manifestResp.StatusCode)
	}

	var manifestPayload struct {
		Status string          `json:"status"`
		Data   script.Manifest `json:"data"`
	}
	if err := json.NewDecoder(manifestResp.Body).Decode(&manifestPayload); err != nil {
		t.Fatalf("decode manifest response: %v", err)
	}
	if manifestPayload.Data.EntryFile != "index.js" {
		t.Fatalf("unexpected manifest entry_file: %s", manifestPayload.Data.EntryFile)
	}
	if len(manifestPayload.Data.Files) != 4 {
		t.Fatalf("unexpected manifest files count: %d", len(manifestPayload.Data.Files))
	}
}

func TestScriptUploadFailsWithoutIndexJS(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptService := script.NewService(db, filepath.Join(t.TempDir(), "script-library"))
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadURL := server.URL + "/api/v1/scripts/upload"
	uploadBody, contentType := buildUploadRequestBody(t, "broken_script", "v0.0.1", "zip", map[string]string{
		"main.js": "\"use strict\";\n",
	})

	uploadResp, err := http.Post(uploadURL, contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(uploadResp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode upload error response: %v", err)
	}
	if payload["error"] != "cannot_find_index_js" {
		t.Fatalf("unexpected upload error: %v", payload["error"])
	}
}

func TestScriptUploadForceReplaceKeepsWorkingWhenOldVersionDirExists(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-force-replace.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptRoot := filepath.Join(t.TempDir(), "script-library")
	scriptService := script.NewService(db, scriptRoot)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadBody, contentType := buildUploadRequestBody(t, "open_douyin", "0.1.2", "zip", map[string]string{
		"index.js": "\"use strict\";\nmodule.exports = { version: \"first\" };\n",
	})
	uploadResp, err := http.Post(server.URL+"/api/v1/scripts/upload", contentType, uploadBody)
	if err != nil {
		t.Fatalf("first upload request: %v", err)
	}
	uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected first upload status: %d", uploadResp.StatusCode)
	}

	forceBody, forceContentType := buildUploadRequestBodyWithForce(t, "open_douyin", "0.1.2", "zip", true, map[string]string{
		"index.js": "\"use strict\";\nmodule.exports = { version: \"second\" };\n",
	})
	forceResp, err := http.Post(server.URL+"/api/v1/scripts/upload", forceContentType, forceBody)
	if err != nil {
		t.Fatalf("force upload request: %v", err)
	}
	defer forceResp.Body.Close()
	if forceResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected force upload status: %d", forceResp.StatusCode)
	}
	var forcePayload struct {
		Status string              `json:"status"`
		Data   script.UploadResult `json:"data"`
	}
	if err := json.NewDecoder(forceResp.Body).Decode(&forcePayload); err != nil {
		t.Fatalf("decode force upload response: %v", err)
	}
	if forcePayload.Data.StoredPath == "" {
		t.Fatalf("expected stored_path in force upload response")
	}

	downloadResp, err := http.Get(server.URL + "/api/v1/script/download?script_name=open_douyin&script_version=0.1.2&relative_path=index.js")
	if err != nil {
		t.Fatalf("download request after force upload: %v", err)
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected download status after force upload: %d", downloadResp.StatusCode)
	}

	content, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if !strings.Contains(string(content), "second") {
		t.Fatalf("expected updated script content, got: %s", string(content))
	}

	if _, err := os.Stat(forcePayload.Data.StoredPath); err != nil {
		t.Fatalf("stat updated stored path: %v", err)
	}
}

func TestDeployScriptToAllOnlineDevices(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-deploy-all.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptService := script.NewService(db, filepath.Join(t.TempDir(), "script-library"))
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	deviceIDs := make([]string, 0, 2)
	conns := make([]*websocket.Conn, 0, 2)
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	for _, suffix := range []string{"001", "002"} {
		registerBody := map[string]any{
			"agent_uuid":  "agent-script-all-" + suffix,
			"device_name": "Script Device " + suffix,
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

		var registerPayload struct {
			Data struct {
				DeviceID string `json:"device_id"`
			} `json:"data"`
		}
		if err := json.NewDecoder(registerResp.Body).Decode(&registerPayload); err != nil {
			registerResp.Body.Close()
			t.Fatalf("decode register response: %v", err)
		}
		registerResp.Body.Close()

		deviceIDs = append(deviceIDs, registerPayload.Data.DeviceID)

		wsURL := "ws" + server.URL[len("http"):] + "/ws"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial websocket: %v", err)
		}
		conns = append(conns, conn)

		hello := protocol.Envelope{
			Type:      protocol.MessageTypeHello,
			RequestID: "hello-script-all-" + suffix,
			DeviceID:  registerPayload.Data.DeviceID,
			Timestamp: time.Now().Unix(),
			Payload: map[string]any{
				"agent_uuid": "agent-script-all-" + suffix,
			},
		}
		if err := conn.WriteJSON(hello); err != nil {
			t.Fatalf("write hello: %v", err)
		}

		var helloAck protocol.Envelope
		if err := conn.ReadJSON(&helloAck); err != nil {
			t.Fatalf("read hello ack: %v", err)
		}
	}

	deployBody := map[string]any{
		"script_name":    "shoppe_sync",
		"script_version": "v0.1.2",
		"force":          true,
	}
	deployBytes, err := json.Marshal(deployBody)
	if err != nil {
		t.Fatalf("marshal deploy body: %v", err)
	}

	deployResp, err := http.Post(server.URL+"/api/v1/scripts/deploy-all", "application/json", bytes.NewReader(deployBytes))
	if err != nil {
		t.Fatalf("deploy-all request: %v", err)
	}
	defer deployResp.Body.Close()
	if deployResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected deploy-all status: %d", deployResp.StatusCode)
	}

	var deployPayload struct {
		Status string `json:"status"`
		Data   struct {
			TotalOnlineDevices int `json:"total_online_devices"`
			SuccessCount       int `json:"success_count"`
			FailureCount       int `json:"failure_count"`
			Results            []struct {
				DeviceID string `json:"device_id"`
				Status   string `json:"status"`
				Message  string `json:"message"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.NewDecoder(deployResp.Body).Decode(&deployPayload); err != nil {
		t.Fatalf("decode deploy-all response: %v", err)
	}
	if deployPayload.Data.TotalOnlineDevices != 2 {
		t.Fatalf("unexpected total online devices: %d", deployPayload.Data.TotalOnlineDevices)
	}
	if deployPayload.Data.SuccessCount != 2 {
		t.Fatalf("unexpected success count: %d", deployPayload.Data.SuccessCount)
	}
	if deployPayload.Data.FailureCount != 0 {
		t.Fatalf("unexpected failure count: %d", deployPayload.Data.FailureCount)
	}

	for index, conn := range conns {
		var syncMessage protocol.Envelope
		if err := conn.ReadJSON(&syncMessage); err != nil {
			t.Fatalf("read sync_script message: %v", err)
		}
		if syncMessage.Type != protocol.MessageTypeSyncScript {
			t.Fatalf("unexpected message type: %s", syncMessage.Type)
		}
		if syncMessage.DeviceID != deviceIDs[index] {
			t.Fatalf("unexpected device id: %s", syncMessage.DeviceID)
		}
	}
}

func TestDeleteScriptVersion(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-delete-version.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptRoot := filepath.Join(t.TempDir(), "script-library")
	scriptService := script.NewService(db, scriptRoot)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadBody, contentType := buildUploadRequestBody(t, "shoppe_sync", "v0.1.3", "zip", map[string]string{
		"index.js": "\"use strict\";\nmodule.exports = {};\n",
	})
	uploadResp, err := http.Post(server.URL+"/api/v1/scripts/upload", contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	var uploadPayload struct {
		Status string              `json:"status"`
		Data   script.UploadResult `json:"data"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploadPayload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
	}

	versionDir := uploadPayload.Data.StoredPath
	if _, err := os.Stat(versionDir); err != nil {
		t.Fatalf("stat uploaded version dir: %v", err)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/scripts/shoppe_sync/versions/v0.1.3", nil)
	if err != nil {
		t.Fatalf("new delete version request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete version request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected delete version status: %d", resp.StatusCode)
	}

	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Fatalf("expected version dir deleted, stat err=%v", err)
	}

	manifestResp, err := http.Get(server.URL + "/api/v1/scripts/shoppe_sync/versions/v0.1.3")
	if err != nil {
		t.Fatalf("manifest request after delete: %v", err)
	}
	defer manifestResp.Body.Close()
	if manifestResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected manifest status after delete: %d", manifestResp.StatusCode)
	}
}

func TestDeleteScript(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-delete.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptRoot := filepath.Join(t.TempDir(), "script-library")
	scriptService := script.NewService(db, scriptRoot)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	for _, version := range []string{"v0.1.1", "v0.1.2"} {
		uploadBody, contentType := buildUploadRequestBody(t, "shoppe_sync", version, "zip", map[string]string{
			"index.js": "\"use strict\";\nmodule.exports = {};\n",
		})
		uploadResp, err := http.Post(server.URL+"/api/v1/scripts/upload", contentType, uploadBody)
		if err != nil {
			t.Fatalf("upload request: %v", err)
		}
		uploadResp.Body.Close()
		if uploadResp.StatusCode != http.StatusOK {
			t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
		}
	}

	scriptDir := filepath.Join(scriptRoot, "shoppe_sync")
	if _, err := os.Stat(scriptDir); err != nil {
		t.Fatalf("stat uploaded script dir: %v", err)
	}

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/scripts/shoppe_sync", nil)
	if err != nil {
		t.Fatalf("new delete script request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete script request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected delete script status: %d", resp.StatusCode)
	}

	if _, err := os.Stat(scriptDir); !os.IsNotExist(err) {
		t.Fatalf("expected script dir deleted, stat err=%v", err)
	}

	listResp, err := http.Get(server.URL + "/api/v1/scripts")
	if err != nil {
		t.Fatalf("list scripts request after delete: %v", err)
	}
	defer listResp.Body.Close()

	var listPayload struct {
		Data struct {
			Items []script.ScriptSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list scripts response: %v", err)
	}
	if len(listPayload.Data.Items) != 0 {
		t.Fatalf("expected no scripts after delete, got %d", len(listPayload.Data.Items))
	}
}

func TestDeleteScriptVersionNotFound(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-delete-notfound.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptService := script.NewService(db, filepath.Join(t.TempDir(), "script-library"))
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, nil)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/scripts/shoppe_sync/versions/v9.9.9", nil)
	if err != nil {
		t.Fatalf("new delete version request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete version request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected delete version status: %d", resp.StatusCode)
	}
}

func TestListScriptsIncludesWorkflowReferencesAndBlocksDelete(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "center-script-workflow-reference.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, "adb", filepath.Join("..", "..", "..", "mobilerpa-agent", "agent"), "http://127.0.0.1:8080", "")
	scriptRoot := filepath.Join(t.TempDir(), "script-library")
	scriptService := script.NewService(db, scriptRoot)
	workflowService := workflow.NewService(db, deviceService, taskService, dispatchService)
	wsHandler := ws.NewHandler(deviceService, dispatchService, nil, workflowService)

	mux := http.NewServeMux()
	RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, workflowService, wsHandler)

	server := httptest.NewServer(WithCORS(mux, []string{"http://localhost:5173", "http://127.0.0.1:5173"}))
	defer server.Close()

	uploadBody, contentType := buildUploadRequestBody(t, "open_qq", "v0.1.0", "zip", map[string]string{
		"index.js": "\"use strict\";\nmodule.exports = {};\n",
	})
	uploadResp, err := http.Post(server.URL+"/api/v1/scripts/upload", contentType, uploadBody)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected upload status: %d", uploadResp.StatusCode)
	}

	createWorkflowBody := map[string]any{
		"workflow_name": "引用 open_qq 的工作流",
		"description":   "",
		"status":        "active",
		"nodes": []map[string]any{
			{
				"node_id":        "node_a",
				"node_type":      "script",
				"node_name":      "步骤A",
				"script_name":    "open_qq",
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
	createWorkflowBytes, err := json.Marshal(createWorkflowBody)
	if err != nil {
		t.Fatalf("marshal workflow body: %v", err)
	}
	createWorkflowResp, err := http.Post(server.URL+"/api/v1/workflows", "application/json", bytes.NewReader(createWorkflowBytes))
	if err != nil {
		t.Fatalf("create workflow request: %v", err)
	}
	defer createWorkflowResp.Body.Close()
	if createWorkflowResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected create workflow status: %d", createWorkflowResp.StatusCode)
	}

	listResp, err := http.Get(server.URL + "/api/v1/scripts")
	if err != nil {
		t.Fatalf("list scripts request: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected list scripts status: %d", listResp.StatusCode)
	}

	var listPayload struct {
		Status string `json:"status"`
		Data   struct {
			Items []script.ScriptSummary `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&listPayload); err != nil {
		t.Fatalf("decode list scripts response: %v", err)
	}
	if len(listPayload.Data.Items) != 1 {
		t.Fatalf("unexpected script count: %d", len(listPayload.Data.Items))
	}
	if len(listPayload.Data.Items[0].Versions) != 1 {
		t.Fatalf("unexpected version count: %d", len(listPayload.Data.Items[0].Versions))
	}
	if len(listPayload.Data.Items[0].Versions[0].WorkflowReferences) != 1 {
		t.Fatalf("unexpected workflow reference count: %d", len(listPayload.Data.Items[0].Versions[0].WorkflowReferences))
	}
	if listPayload.Data.Items[0].Versions[0].WorkflowReferences[0].WorkflowName != "引用 open_qq 的工作流" {
		t.Fatalf("unexpected workflow reference name: %s", listPayload.Data.Items[0].Versions[0].WorkflowReferences[0].WorkflowName)
	}

	deleteVersionReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/scripts/open_qq/versions/v0.1.0", nil)
	if err != nil {
		t.Fatalf("new delete version request: %v", err)
	}
	deleteVersionResp, err := http.DefaultClient.Do(deleteVersionReq)
	if err != nil {
		t.Fatalf("delete version request: %v", err)
	}
	defer deleteVersionResp.Body.Close()
	if deleteVersionResp.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected delete version status: %d", deleteVersionResp.StatusCode)
	}
	var deleteVersionPayload struct {
		Error              string                     `json:"error"`
		WorkflowReferences []script.WorkflowReference `json:"workflow_references"`
	}
	if err := json.NewDecoder(deleteVersionResp.Body).Decode(&deleteVersionPayload); err != nil {
		t.Fatalf("decode delete version response: %v", err)
	}
	if deleteVersionPayload.Error != "script_version_referenced_by_workflows" {
		t.Fatalf("unexpected delete version error: %s", deleteVersionPayload.Error)
	}
	if len(deleteVersionPayload.WorkflowReferences) != 1 {
		t.Fatalf("unexpected delete version workflow references: %d", len(deleteVersionPayload.WorkflowReferences))
	}

	deleteScriptReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/v1/scripts/open_qq", nil)
	if err != nil {
		t.Fatalf("new delete script request: %v", err)
	}
	deleteScriptResp, err := http.DefaultClient.Do(deleteScriptReq)
	if err != nil {
		t.Fatalf("delete script request: %v", err)
	}
	defer deleteScriptResp.Body.Close()
	if deleteScriptResp.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected delete script status: %d", deleteScriptResp.StatusCode)
	}
	var deleteScriptPayload struct {
		Error              string                     `json:"error"`
		WorkflowReferences []script.WorkflowReference `json:"workflow_references"`
	}
	if err := json.NewDecoder(deleteScriptResp.Body).Decode(&deleteScriptPayload); err != nil {
		t.Fatalf("decode delete script response: %v", err)
	}
	if deleteScriptPayload.Error != "script_referenced_by_workflows" {
		t.Fatalf("unexpected delete script error: %s", deleteScriptPayload.Error)
	}
	if len(deleteScriptPayload.WorkflowReferences) != 1 {
		t.Fatalf("unexpected delete script workflow references: %d", len(deleteScriptPayload.WorkflowReferences))
	}
}

func buildUploadRequestBody(t *testing.T, scriptName string, scriptVersion string, sourceType string, files map[string]string) (*bytes.Buffer, string) {
	return buildUploadRequestBodyWithForce(t, scriptName, scriptVersion, sourceType, false, files)
}

func buildUploadRequestBodyWithForce(t *testing.T, scriptName string, scriptVersion string, sourceType string, force bool, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()

	zipBytes := buildZipBytes(t, files)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("script_name", scriptName); err != nil {
		t.Fatalf("write script_name: %v", err)
	}
	if err := writer.WriteField("script_version", scriptVersion); err != nil {
		t.Fatalf("write script_version: %v", err)
	}
	if err := writer.WriteField("source_type", sourceType); err != nil {
		t.Fatalf("write source_type: %v", err)
	}
	if err := writer.WriteField("force", strconv.FormatBool(force)); err != nil {
		t.Fatalf("write force: %v", err)
	}

	part, err := writer.CreateFormFile("file", scriptName+"-"+scriptVersion+".zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(zipBytes); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}

func buildZipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	zipWriter := zip.NewWriter(buf)
	for path, content := range files {
		entry, err := zipWriter.Create(strings.TrimSpace(path))
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return buf.Bytes()
}

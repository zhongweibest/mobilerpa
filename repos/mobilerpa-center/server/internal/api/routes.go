package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/discovery"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/plan"
	"github.com/mobilerpa/mobilerpa-center/server/internal/script"
	"github.com/mobilerpa/mobilerpa-center/server/internal/settings"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
)

// RegisterRoutes 注册中心服务当前可用的 HTTP 路由。
// 为兼容历史测试调用，最后参数同时支持：
// 1. 仅传入 wsHandler
// 2. 传入 scripts, wsHandler
func RegisterRoutes(mux *http.ServeMux, devices *device.Service, tasks *task.Service, dispatcher *dispatch.Service, discoveryService *discovery.Service, extras ...any) {
	scripts := script.NewService(nil, "./data/scripts")
	systemSettings := settings.NewService(nil)
	plans := plan.NewService(nil, nil, nil, nil, nil)
	workflows := workflow.NewService(nil, nil, nil, nil)
	var wsHandler http.Handler

	for _, extra := range extras {
		switch value := extra.(type) {
		case *script.Service:
			if value != nil {
				scripts = value
			}
		case *settings.Service:
			if value != nil {
				systemSettings = value
			}
		case *plan.Service:
			if value != nil {
				plans = value
			}
		case http.Handler:
			wsHandler = value
		case http.HandlerFunc:
			wsHandler = value
		case *workflow.Service:
			if value != nil {
				workflows = value
			}
		}
	}

	if wsHandler == nil {
		panic("RegisterRoutes requires wsHandler")
	}

	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/api/v1/device/register", registerDevice(devices))
	mux.HandleFunc("/api/v1/devices", listDevices(devices, plans))
	mux.HandleFunc("/api/v1/devices/", getDevice(devices, tasks, workflows, plans))
	mux.HandleFunc("/api/v1/discovery/devices", listDiscoveredDevices(discoveryService))
	mux.HandleFunc("/api/v1/discovery/agent-deployments", deployAgents(discoveryService))
	mux.HandleFunc("/api/v1/discovery/agent-actions", controlAgent(discoveryService))
	mux.HandleFunc("/api/v1/discovery/pair", pairDevice(discoveryService))
	mux.HandleFunc("/api/v1/settings/discovery", discoverySettings(systemSettings))
	mux.HandleFunc("/api/v1/plans", plansCollection(plans))
	mux.HandleFunc("/api/v1/plans/", planSubResources(plans))
	mux.HandleFunc("/api/v1/scripts/deploy", deployScript(scripts, dispatcher))
	mux.HandleFunc("/api/v1/scripts/deploy-all", deployScriptToAllOnlineDevices(devices, dispatcher))
	mux.HandleFunc("/api/v1/scripts/upload", uploadScript(scripts))
	mux.HandleFunc("/api/v1/scripts", listScripts(scripts))
	mux.HandleFunc("/api/v1/scripts/", scriptsSubResources(scripts))
	mux.HandleFunc("/api/v1/tasks", tasksCollection(devices, tasks, dispatcher, workflows))
	mux.HandleFunc("/api/v1/tasks/", taskSubResources(tasks))
	mux.HandleFunc("/api/v1/workflows", workflowsCollection(workflows))
	mux.HandleFunc("/api/v1/workflows/", workflowSubResources(workflows))
	mux.HandleFunc("/api/v1/device/upload", notImplemented("device upload"))
	mux.HandleFunc("/api/v1/script/manifest", scriptManifest(scripts))
	mux.HandleFunc("/api/v1/script/download", scriptDownload(scripts))
	mux.Handle("/ws", wsHandler)
}

func workflowsCollection(workflows *workflow.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req workflow.CreateDefinitionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			result, err := workflows.CreateDefinition(ctx, req)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})

		case http.MethodGet:
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			if strings.TrimSpace(r.URL.Query().Get("view")) == "instances" {
				result, err := workflows.ListInstances(ctx, "")
				if err != nil {
					writeWorkflowError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"status": "ok",
					"data":   result,
				})
				return
			}

			page, pageSize, err := parsePagination(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			result, err := workflows.ListDefinitions(ctx)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   paginateItems(result, page, pageSize),
			})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost},
			})
		}
	}
}

func plansCollection(plans *plan.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req plan.CreateDefinitionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			result, err := plans.CreateDefinition(ctx, req)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})

		case http.MethodGet:
			page, pageSize, err := parsePagination(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			if strings.TrimSpace(r.URL.Query().Get("view")) == "runs" {
				result, err := plans.ListRuns(ctx, "")
				if err != nil {
					writePlanError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"status": "ok",
					"data":   paginateItems(result, page, pageSize),
				})
				return
			}

			result, err := plans.ListDefinitions(ctx)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   paginateItems(result, page, pageSize),
			})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost},
			})
		}
	}
}

func planSubResources(plans *plan.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/plans/")
		trimmed = strings.Trim(trimmed, "/")
		if trimmed == "" {
			writeError(w, http.StatusNotFound, "plan_resource_not_found")
			return
		}

		parts := strings.Split(trimmed, "/")
		planDefID := strings.TrimSpace(parts[0])
		if planDefID == "" {
			writeError(w, http.StatusNotFound, "plan_resource_not_found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if len(parts) == 1 && r.Method == http.MethodGet {
			result, err := plans.GetDefinition(ctx, planDefID)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			if err := plans.DeleteDefinition(ctx, planDefID); err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"plan_def_id": planDefID,
					"deleted":     true,
				},
			})
			return
		}

		if len(parts) == 2 && parts[1] == "devices" && r.Method == http.MethodPut {
			var req plan.UpdateDefinitionDevicesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := plans.UpdateDefinitionDevices(ctx, planDefID, req)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodGet {
			result, err := plans.ListRuns(ctx, planDefID)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "start" && r.Method == http.MethodPost {
			var req plan.StartRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			req.Manual = true
			result, err := plans.Start(ctx, planDefID, req)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 3 && parts[1] == "runs" && r.Method == http.MethodGet {
			result, err := plans.GetRun(ctx, parts[2])
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "events" && r.Method == http.MethodGet {
			result, err := plans.ListEvents(ctx, parts[2])
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "stop" && r.Method == http.MethodPost {
			result, err := plans.StopRun(ctx, parts[2])
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 3 && parts[1] == "runs" && r.Method == http.MethodDelete {
			if err := plans.DeleteRun(ctx, parts[2]); err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"plan_def_id": planDefID,
					"plan_run_id": parts[2],
					"deleted":     true,
				},
			})
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "devices" && r.Method == http.MethodPost {
			var req plan.AddDevicesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := plans.AddDevices(ctx, parts[2], req)
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 5 && parts[1] == "runs" && parts[3] == "devices" && r.Method == http.MethodDelete {
			result, err := plans.RemoveDevice(ctx, parts[2], parts[4])
			if err != nil {
				writePlanError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		writeError(w, http.StatusNotFound, "plan_resource_not_found")
	}
}

func workflowSubResources(workflows *workflow.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
		trimmed = strings.Trim(trimmed, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "workflow_resource_not_found")
			return
		}

		workflowDefID := strings.TrimSpace(parts[0])
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		if len(parts) == 1 && r.Method == http.MethodGet {
			result, err := workflows.GetDefinition(ctx, workflowDefID)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			if err := workflows.DeleteDefinition(ctx, workflowDefID); err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"workflow_def_id": workflowDefID,
					"deleted":         true,
				},
			})
			return
		}

		if len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodGet {
			result, err := workflows.ListRuns(ctx, workflowDefID)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "instances" && r.Method == http.MethodGet {
			result, err := workflows.ListInstances(ctx, workflowDefID)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "events" && r.Method == http.MethodGet {
			workflowRunID := strings.TrimSpace(r.URL.Query().Get("workflow_run_id"))
			result, err := workflows.ListEvents(ctx, workflowDefID, workflowRunID)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "start" && r.Method == http.MethodPost {
			var req workflow.StartRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			result, err := workflows.Start(ctx, workflowDefID, req)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 4 && parts[1] == "instances" && parts[3] == "devices" && r.Method == http.MethodPost {
			var req workflow.AddDevicesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			req.WorkflowInstanceID = strings.TrimSpace(parts[2])

			result, err := workflows.AddDevices(ctx, workflowDefID, req)
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 4 && parts[1] == "instances" && parts[3] == "stop" && r.Method == http.MethodPost {
			result, err := workflows.StopDefinition(ctx, workflowDefID, parts[2])
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 3 && parts[1] == "instances" && r.Method == http.MethodDelete {
			if err := workflows.DeleteInstance(ctx, parts[2]); err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"workflow_def_id":      workflowDefID,
					"workflow_instance_id": parts[2],
					"deleted":              true,
				},
			})
			return
		}

		if len(parts) == 6 && parts[1] == "instances" && parts[3] == "devices" && parts[5] == "stop" && r.Method == http.MethodPost {
			result, err := workflows.StopRunByDevice(ctx, workflowDefID, parts[2], parts[4])
			if err != nil {
				writeWorkflowError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		writeError(w, http.StatusNotFound, "workflow_resource_not_found")
	}
}

func scriptsSubResources(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/scripts/")
		trimmed = strings.Trim(trimmed, "/")
		if trimmed == "" {
			writeError(w, http.StatusNotFound, "script_resource_not_found")
			return
		}

		parts := strings.Split(trimmed, "/")
		if len(parts) == 1 {
			deleteScript(scripts).ServeHTTP(w, r)
			return
		}

		getScriptVersion(scripts).ServeHTTP(w, r)
	}
}

type pageResult[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func parsePagination(r *http.Request) (int, int, error) {
	page := 1
	pageSize := 10

	if rawPage := strings.TrimSpace(r.URL.Query().Get("page")); rawPage != "" {
		value, err := strconv.Atoi(rawPage)
		if err != nil || value <= 0 {
			return 0, 0, fmt.Errorf("invalid_page")
		}
		page = value
	}

	if rawPageSize := strings.TrimSpace(r.URL.Query().Get("page_size")); rawPageSize != "" {
		value, err := strconv.Atoi(rawPageSize)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid_page_size")
		}
		switch value {
		case 10, 20, 30, 50, 100:
			pageSize = value
		default:
			return 0, 0, fmt.Errorf("invalid_page_size")
		}
	}

	return page, pageSize, nil
}

func paginateItems[T any](items []T, page int, pageSize int) pageResult[T] {
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	pagedItems := make([]T, 0, end-start)
	if start < end {
		pagedItems = append(pagedItems, items[start:end]...)
	}

	return pageResult[T]{
		Items:    pagedItems,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

func discoverySettings(service *settings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			result, err := service.GetDiscoverySettings(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodPut:
			var req settings.DiscoverySettings
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			result, err := service.SaveDiscoverySettings(ctx, req)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
		}
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "mobilerpa-center",
	})
}

func registerDevice(devices *device.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req device.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := devices.Register(ctx, req, r)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
				status = http.StatusRequestTimeout
			case strings.Contains(err.Error(), "agent_uuid is required"):
				status = http.StatusBadRequest
			}

			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func listDevices(devices *device.Service, plans *plan.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		page, pageSize, err := parsePagination(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		results, err := devices.List(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		items := make([]map[string]any, 0, len(results))
		for _, item := range results {
			deviceItem := map[string]any{
				"device_id":                             item.DeviceID,
				"agent_uuid":                            item.AgentUUID,
				"device_name":                           item.DeviceName,
				"physical_slot":                         item.PhysicalSlot,
				"group_name":                            item.GroupName,
				"status":                                item.Status,
				"bind_status":                           item.BindStatus,
				"ip":                                    item.IP,
				"brand":                                 item.Brand,
				"model":                                 item.Model,
				"android_id":                            item.AndroidID,
				"adb_serial":                            item.ADBSerial,
				"current_task_id":                       item.CurrentTaskID,
				"current_step":                          item.CurrentStep,
				"last_error":                            item.LastError,
				"accessibility_status":                  item.AccessibilityStatus,
				"foreground_service_status":             item.ForegroundServiceStatus,
				"battery_optimization_ignored_status":   item.BatteryOptimizationIgnoredStatus,
				"env_checked_at":                        item.EnvCheckedAt,
				"env_check_message":                     item.EnvCheckMessage,
				"last_heartbeat_at":                     item.LastHeartbeatAt,
				"created_at":                            item.CreatedAt,
				"updated_at":                            item.UpdatedAt,
				"occupancy":                             nil,
			}
			if plans != nil {
				busyDetail, err := plans.GetDeviceBusyDetail(ctx, item.DeviceID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				deviceItem["occupancy"] = busyDetail
			}
			items = append(items, deviceItem)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   paginateItems(items, page, pageSize),
		})
	}
}

func getDevice(devices *device.Service, tasks *task.Service, workflows *workflow.Service, plans *plan.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
		trimmed = strings.Trim(trimmed, "/")
		parts := strings.Split(trimmed, "/")
		deviceID := strings.TrimSpace(parts[0])
		if deviceID == "" {
			writeError(w, http.StatusNotFound, "device_not_found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if len(parts) == 2 && parts[1] == "occupancy" && r.Method == http.MethodGet {
			deviceItem, err := devices.GetByID(ctx, deviceID)
			if err != nil {
				writeDeviceError(w, err)
				return
			}

			var busyDetail any
			if plans != nil {
				planBusyDetail, err := plans.GetDeviceBusyDetail(ctx, deviceID)
				if err != nil {
					writePlanError(w, err)
					return
				}
				busyDetail = planBusyDetail
			} else {
				workflowBusyDetail, err := workflows.GetDeviceBusyDetail(ctx, deviceID)
				if err != nil {
					writeWorkflowError(w, err)
					return
				}
				busyDetail = workflowBusyDetail
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"device_id":       deviceID,
					"device_status":   deviceItem.Status,
					"current_task_id": deviceItem.CurrentTaskID,
					"current_step":    deviceItem.CurrentStep,
					"last_error":      deviceItem.LastError,
					"occupancy":       busyDetail,
				},
			})
			return
		}

		if len(parts) == 3 && parts[1] == "manual-tasks" && parts[2] != "" && r.Method == http.MethodPost {
			if tasks == nil {
				writeError(w, http.StatusNotFound, "task_resource_not_found")
				return
			}

			result, err := tasks.StopManualTask(ctx, parts[2], "人工结束手工任务")
			if err != nil {
				writeTaskError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		switch r.Method {
		case http.MethodGet:
			if len(parts) != 1 {
				writeError(w, http.StatusNotFound, "device_not_found")
				return
			}
			result, err := devices.GetByID(ctx, deviceID)
			if err != nil {
				writeDeviceError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodDelete:
			if len(parts) != 1 {
				writeError(w, http.StatusNotFound, "device_not_found")
				return
			}
			if err := devices.DeleteByID(ctx, deviceID); err != nil {
				writeDeviceError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"device_id": deviceID,
					"deleted":   true,
				},
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodDelete},
			})
		}
	}
}

func listDiscoveredDevices(discoveryService *discovery.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		page, pageSize, err := parsePagination(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		results, err := discoveryService.ListDevices(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   paginateItems(results, page, pageSize),
		})
	}
}

func deployAgents(discoveryService *discovery.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req discovery.DeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()

		results, err := discoveryService.DeployAgent(ctx, req)
		if err != nil {
			writeDiscoveryError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   results,
		})
	}
}

func controlAgent(discoveryService *discovery.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req discovery.AgentActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := discoveryService.ControlAgent(ctx, req)
		if err != nil {
			writeDiscoveryError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func pairDevice(discoveryService *discovery.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req discovery.PairRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := discoveryService.PairDevice(ctx, req)
		if err != nil {
			writeDiscoveryError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func tasksCollection(devices *device.Service, tasks *task.Service, dispatcher *dispatch.Service, workflows *workflow.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req task.CreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			if workflows != nil {
				busyDetail, err := workflows.GetDeviceBusyDetail(ctx, req.DeviceID)
				if err != nil {
					writeWorkflowError(w, err)
					return
				}
				if busyDetail != nil {
					writeWorkflowError(w, &workflow.DeviceBusyError{Details: []workflow.DeviceBusyDetail{*busyDetail}})
					return
				}
			}

			result, err := tasks.Create(ctx, req)
			if err != nil {
				writeTaskError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodGet:
			page, pageSize, err := parsePagination(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			results, err := tasks.List(ctx, strings.TrimSpace(r.URL.Query().Get("source_type")))
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   paginateItems(results, page, pageSize),
			})
		case http.MethodPatch:
			var req struct {
				TaskID string `json:"task_id"`
				Action string `json:"action"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}

			if strings.TrimSpace(req.Action) != "assign" {
				writeError(w, http.StatusBadRequest, "unsupported_task_action")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			taskItem, err := tasks.GetByID(ctx, req.TaskID)
			if err != nil {
				writeTaskError(w, err)
				return
			}

			if err := devices.EnsureExecutionReady(ctx, taskItem.DeviceID); err != nil {
				writeDispatchError(w, err)
				return
			}
			if workflows != nil {
				busyDetail, err := workflows.GetDeviceBusyDetail(ctx, taskItem.DeviceID)
				if err != nil {
					writeWorkflowError(w, err)
					return
				}
				if busyDetail != nil && !(busyDetail.OccupancyType == "manual_task" && busyDetail.TaskID == taskItem.TaskID) {
					writeWorkflowError(w, &workflow.DeviceBusyError{Details: []workflow.DeviceBusyDetail{*busyDetail}})
					return
				}
			}

			result, err := dispatcher.AssignTask(ctx, req.TaskID)
			if err != nil {
				writeDispatchError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost, http.MethodPatch},
			})
		}
	}
}

func taskSubResources(tasks *task.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
		trimmed = strings.Trim(trimmed, "/")
		parts := strings.Split(trimmed, "/")
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if len(parts) == 2 && parts[0] != "" && parts[1] == "events" && r.Method == http.MethodGet {
			results, err := tasks.ListEvents(ctx, parts[0])
			if err != nil {
				writeTaskError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   results,
			})
			return
		}

		if len(parts) == 2 && parts[0] != "" && parts[1] == "terminate" && r.Method == http.MethodPost {
			result, err := tasks.StopManualTask(ctx, parts[0], "人工结束手工任务")
			if err != nil {
				writeTaskError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodDelete {
			if err := tasks.Delete(ctx, parts[0]); err != nil {
				writeTaskError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"task_id": parts[0],
					"deleted": true,
				},
			})
			return
		}

		writeError(w, http.StatusNotFound, "task_resource_not_found")
	}
}

func scriptManifest(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := scripts.GetManifest(ctx, r.URL.Query().Get("script_name"), r.URL.Query().Get("script_version"))
		if err != nil {
			writeScriptError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func uploadScript(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		if err := r.ParseMultipartForm(64 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart_form")
			return
		}

		fileHeader, err := firstFileHeader(r.MultipartForm.File["file"])
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := scripts.UploadZip(ctx, script.UploadRequest{
			ScriptName:    r.FormValue("script_name"),
			ScriptVersion: r.FormValue("script_version"),
			SourceType:    r.FormValue("source_type"),
			Force:         parseBooleanFormValue(r.FormValue("force")),
			FileHeader:    fileHeader,
		})
		if err != nil {
			writeScriptError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func parseBooleanFormValue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func listScripts(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		page, pageSize, err := parsePagination(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := scripts.ListScripts(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   paginateItems(result, page, pageSize),
		})
	}
}

func deleteScript(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			methodNotAllowed(w, http.MethodDelete)
			return
		}

		scriptName := strings.TrimPrefix(r.URL.Path, "/api/v1/scripts/")
		scriptName = strings.Trim(scriptName, "/")
		if scriptName == "" || strings.Contains(scriptName, "/") {
			writeError(w, http.StatusNotFound, "script_resource_not_found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := scripts.DeleteScript(ctx, scriptName); err != nil {
			writeScriptError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data": map[string]any{
				"script_name": scriptName,
				"deleted":     true,
			},
		})
	}
}

func deployScript(_ *script.Service, dispatcher *dispatch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req struct {
			DeviceID      string `json:"device_id"`
			ScriptName    string `json:"script_name"`
			ScriptVersion string `json:"script_version"`
			Force         bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := dispatcher.SyncScript(ctx, req.DeviceID, req.ScriptName, req.ScriptVersion, req.Force); err != nil {
			writeDiscoveryError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data": map[string]any{
				"device_id":      strings.TrimSpace(req.DeviceID),
				"script_name":    strings.TrimSpace(req.ScriptName),
				"script_version": strings.TrimSpace(req.ScriptVersion),
				"force":          req.Force,
				"accepted":       true,
			},
		})
	}
}

func deployScriptToAllOnlineDevices(deviceService *device.Service, dispatcher *dispatch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req struct {
			ScriptName    string `json:"script_name"`
			ScriptVersion string `json:"script_version"`
			Force         bool   `json:"force"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		devices, err := deviceService.ListOnline(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		type itemResult struct {
			DeviceID string `json:"device_id"`
			Status   string `json:"status"`
			Message  string `json:"message"`
		}

		results := make([]itemResult, 0, len(devices))
		successCount := 0

		for _, item := range devices {
			if err := dispatcher.SyncScript(ctx, item.DeviceID, req.ScriptName, req.ScriptVersion, req.Force); err != nil {
				results = append(results, itemResult{
					DeviceID: item.DeviceID,
					Status:   "error",
					Message:  err.Error(),
				})
				continue
			}

			successCount += 1
			results = append(results, itemResult{
				DeviceID: item.DeviceID,
				Status:   "ok",
				Message:  "sync_script_dispatched",
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data": map[string]any{
				"total_online_devices": len(devices),
				"success_count":        successCount,
				"failure_count":        len(devices) - successCount,
				"results":              results,
			},
		})
	}
}

func getScriptVersion(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/scripts/")
		trimmed = strings.Trim(trimmed, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) != 3 || strings.TrimSpace(parts[0]) == "" || parts[1] != "versions" || strings.TrimSpace(parts[2]) == "" {
			writeError(w, http.StatusNotFound, "script_resource_not_found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		switch r.Method {
		case http.MethodGet:
			result, err := scripts.GetManifest(ctx, parts[0], parts[2])
			if err != nil {
				writeScriptError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodDelete:
			if err := scripts.DeleteVersion(ctx, parts[0], parts[2]); err != nil {
				writeScriptError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"script_name":    parts[0],
					"script_version": parts[2],
					"deleted":        true,
				},
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodDelete},
			})
		}
	}
}

func scriptDownload(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		relativePath := strings.TrimSpace(r.URL.Query().Get("relative_path"))
		if relativePath == "" {
			relativePath = "index.js"
		}

		result, err := scripts.GetFile(ctx, r.URL.Query().Get("script_name"), r.URL.Query().Get("script_version"), relativePath)
		if err != nil {
			writeScriptError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Script-Name", result.Manifest.ScriptName)
		w.Header().Set("X-Script-Version", result.Manifest.ScriptVersion)
		w.Header().Set("X-Script-Checksum-SHA256", result.Manifest.ChecksumSHA256)
		w.Header().Set("X-Script-Relative-Path", relativePath)
		http.ServeFile(w, r, result.Path)
	}
}

func firstFileHeader(files []*multipart.FileHeader) (*multipart.FileHeader, error) {
	if len(files) == 0 || files[0] == nil {
		return nil, errors.New("script_file_required")
	}
	return files[0], nil
}

func writeTaskError(w http.ResponseWriter, err error) {
	if writeExecutionReadyError(w, err) {
		return
	}

	status := http.StatusInternalServerError
	message := err.Error()

	switch {
	case errors.Is(err, task.ErrTaskNotFound):
		status = http.StatusNotFound
		message = "task_not_found"
	case errors.Is(err, task.ErrTaskDeviceNotFound):
		status = http.StatusNotFound
		message = "device_not_found"
	case errors.Is(err, task.ErrTaskDeviceRequired):
		status = http.StatusBadRequest
		message = "device_id_required"
	case errors.Is(err, task.ErrTaskScriptNameRequired):
		status = http.StatusBadRequest
		message = "script_name_required"
	case errors.Is(err, task.ErrTaskPriorityInvalid):
		status = http.StatusBadRequest
		message = "invalid_priority"
	case errors.Is(err, task.ErrTaskScheduledAtInvalid):
		status = http.StatusBadRequest
		message = "invalid_scheduled_at"
	case errors.Is(err, task.ErrTaskAlreadyAssigned):
		status = http.StatusConflict
		message = "task_already_assigned"
	case errors.Is(err, task.ErrTaskManualStopNotAllowed):
		status = http.StatusConflict
		message = "task_manual_stop_not_allowed"
	case errors.Is(err, task.ErrTaskDeleteNotAllowed):
		status = http.StatusConflict
		message = "task_delete_not_allowed"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status = http.StatusRequestTimeout
	}

	writeError(w, status, message)
}

func writeDeviceError(w http.ResponseWriter, err error) {
	if writeExecutionReadyError(w, err) {
		return
	}

	status := http.StatusInternalServerError
	message := err.Error()

	switch {
	case errors.Is(err, device.ErrDeviceNotFound):
		status = http.StatusNotFound
		message = "device_not_found"
	case errors.Is(err, device.ErrDeviceOnline):
		status = http.StatusConflict
		message = "device_online_cannot_delete"
	case errors.Is(err, device.ErrDeviceExecutionProfileUnknown):
		status = http.StatusConflict
		message = "device_execution_profile_unknown"
	case errors.Is(err, device.ErrDeviceAccessibilityRequired):
		status = http.StatusConflict
		message = "device_accessibility_required"
	case errors.Is(err, device.ErrDeviceForegroundServiceRequired):
		status = http.StatusConflict
		message = "device_foreground_service_required"
	case errors.Is(err, device.ErrDeviceBatteryOptimizationRequired):
		status = http.StatusConflict
		message = "device_battery_optimization_required"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		status = http.StatusRequestTimeout
	}

	writeError(w, status, message)
}

func writeDispatchError(w http.ResponseWriter, err error) {
	if writeExecutionReadyError(w, err) {
		return
	}

	switch {
	case errors.Is(err, dispatch.ErrDeviceNotConnected):
		writeError(w, http.StatusConflict, "device_not_connected")
	case errors.Is(err, task.ErrTaskAlreadyAssigned):
		writeError(w, http.StatusConflict, "task_already_assigned")
	default:
		writeTaskError(w, err)
	}
}

func writeDiscoveryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, discovery.ErrADBEndpointRequired):
		writeError(w, http.StatusBadRequest, "adb_endpoints_required")
	case errors.Is(err, discovery.ErrCenterBaseURLRequired):
		writeError(w, http.StatusBadRequest, "center_base_url_required")
	case errors.Is(err, discovery.ErrAgentActionRequired):
		writeError(w, http.StatusBadRequest, "agent_action_required")
	case errors.Is(err, discovery.ErrAgentActionUnsupported):
		writeError(w, http.StatusBadRequest, "agent_action_unsupported")
	case errors.Is(err, discovery.ErrPairHostRequired):
		writeError(w, http.StatusBadRequest, "pair_host_required")
	case errors.Is(err, discovery.ErrPairPortRequired):
		writeError(w, http.StatusBadRequest, "pair_port_required")
	case errors.Is(err, discovery.ErrPairCodeRequired):
		writeError(w, http.StatusBadRequest, "pair_code_required")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_timeout")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeScriptError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, script.ErrScriptNameRequired):
		writeError(w, http.StatusBadRequest, "script_name_required")
	case errors.Is(err, script.ErrScriptVersionRequired):
		writeError(w, http.StatusBadRequest, "script_version_required")
	case errors.Is(err, script.ErrScriptVersionNotFound):
		writeError(w, http.StatusNotFound, "script_version_not_found")
	case errors.Is(err, script.ErrScriptNotFound):
		writeError(w, http.StatusNotFound, "script_not_found")
	case errors.Is(err, script.ErrScriptPathUnsafe):
		writeError(w, http.StatusBadRequest, "script_path_unsafe")
	case errors.Is(err, script.ErrScriptSourceTypeUnsupported):
		writeError(w, http.StatusBadRequest, "script_source_type_unsupported")
	case errors.Is(err, script.ErrScriptVersionAlreadyExists):
		writeError(w, http.StatusConflict, "script_version_already_exists")
	case errors.Is(err, script.ErrScriptVersionReferenced):
		var refErr *script.ReferenceConflictError
		if errors.As(err, &refErr) {
			writeErrorWithDetails(w, http.StatusConflict, "script_version_referenced_by_workflows", map[string]any{
				"script_name":         refErr.ScriptName,
				"script_version":      refErr.ScriptVersion,
				"workflow_references": refErr.References,
			})
			return
		}
		writeError(w, http.StatusConflict, "script_version_referenced_by_workflows")
	case errors.Is(err, script.ErrScriptReferenced):
		var refErr *script.ReferenceConflictError
		if errors.As(err, &refErr) {
			writeErrorWithDetails(w, http.StatusConflict, "script_referenced_by_workflows", map[string]any{
				"script_name":         refErr.ScriptName,
				"workflow_references": refErr.References,
			})
			return
		}
		writeError(w, http.StatusConflict, "script_referenced_by_workflows")
	case errors.Is(err, script.ErrScriptEntryNotFound):
		writeError(w, http.StatusBadRequest, "cannot_find_index_js")
	case errors.Is(err, script.ErrScriptUploadEmpty):
		writeError(w, http.StatusBadRequest, "script_upload_empty")
	case errors.Is(err, script.ErrScriptRepositoryUnavailable):
		writeError(w, http.StatusServiceUnavailable, "script_repository_unavailable")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_timeout")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeExecutionReadyError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, device.ErrDeviceExecutionProfileUnknown):
		writeErrorWithDetails(w, http.StatusConflict, "设备执行环境校验失败", map[string]any{
			"reason": "设备尚未上报执行环境状态，请先启动 Agent 并等待环境检查完成",
		})
		return true
	case errors.Is(err, device.ErrDeviceAccessibilityRequired):
		writeErrorWithDetails(w, http.StatusConflict, "设备执行环境校验失败", map[string]any{
			"reason": "设备未开启无障碍服务，请先在手机上开启 AutoJs6 无障碍权限",
		})
		return true
	case errors.Is(err, device.ErrDeviceForegroundServiceRequired):
		writeErrorWithDetails(w, http.StatusConflict, "设备执行环境校验失败", map[string]any{
			"reason": "设备未开启前台服务或通知保活，请先确认 Agent 前台服务正常运行",
		})
		return true
	case errors.Is(err, device.ErrDeviceBatteryOptimizationRequired):
		writeErrorWithDetails(w, http.StatusConflict, "设备执行环境校验失败", map[string]any{
			"reason": "设备未忽略电量优化，请先关闭系统对 Agent 或 AutoJs6 的电量限制",
		})
		return true
	default:
		return false
	}
}

func writeWorkflowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workflow.ErrWorkflowDefinitionNotFound):
		writeError(w, http.StatusNotFound, "workflow_definition_not_found")
	case errors.Is(err, workflow.ErrWorkflowInstanceNotFound):
		writeError(w, http.StatusNotFound, "workflow_instance_not_found")
	case errors.Is(err, workflow.ErrWorkflowInstanceNotActive):
		writeError(w, http.StatusConflict, "workflow_instance_not_active")
	case errors.Is(err, workflow.ErrWorkflowRunNotFound):
		writeError(w, http.StatusNotFound, "workflow_run_not_found")
	case errors.Is(err, workflow.ErrWorkflowDefinitionNameRequired):
		writeError(w, http.StatusBadRequest, "workflow_name_required")
	case errors.Is(err, workflow.ErrWorkflowDefinitionNodesRequired):
		writeError(w, http.StatusBadRequest, "workflow_nodes_required")
	case errors.Is(err, workflow.ErrWorkflowNodeIDRequired):
		writeError(w, http.StatusBadRequest, "workflow_node_id_required")
	case errors.Is(err, workflow.ErrWorkflowNodeTypeUnsupported):
		writeError(w, http.StatusBadRequest, "workflow_node_type_unsupported")
	case errors.Is(err, workflow.ErrWorkflowScriptNameRequired):
		writeError(w, http.StatusBadRequest, "workflow_script_name_required")
	case errors.Is(err, workflow.ErrWorkflowDeviceIDsRequired):
		writeError(w, http.StatusBadRequest, "workflow_device_ids_required")
	case errors.Is(err, workflow.ErrWorkflowDeviceBusy):
		var busyErr *workflow.DeviceBusyError
		if errors.As(err, &busyErr) {
			writeErrorWithDetails(w, http.StatusConflict, "workflow_device_busy", map[string]any{
				"busy_devices": busyErr.Details,
			})
			return
		}
		writeError(w, http.StatusConflict, "workflow_device_busy")
	case errors.Is(err, workflow.ErrWorkflowAnotherActive):
		writeError(w, http.StatusConflict, "workflow_another_active")
	case errors.Is(err, workflow.ErrWorkflowDefinitionRunning):
		writeError(w, http.StatusConflict, "workflow_definition_running")
	case errors.Is(err, workflow.ErrWorkflowInstanceDeleteNotAllowed):
		writeErrorWithDetails(w, http.StatusConflict, "工作流实例暂不允许删除", map[string]any{
			"reason": "只有执行成功或执行失败的工作流实例允许删除，运行中、待执行或已停止实例请勿删除",
		})
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_timeout")
	default:
		if writeExecutionReadyError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writePlanError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, plan.ErrPlanDefinitionNotFound):
		writeError(w, http.StatusNotFound, "plan_definition_not_found")
	case errors.Is(err, plan.ErrPlanRunNotFound):
		writeError(w, http.StatusNotFound, "plan_run_not_found")
	case errors.Is(err, plan.ErrPlanDeviceRunNotFound):
		writeError(w, http.StatusNotFound, "plan_device_run_not_found")
	case errors.Is(err, plan.ErrPlanRunNotActive):
		writeError(w, http.StatusConflict, "plan_run_not_active")
	case errors.Is(err, plan.ErrPlanNameRequired):
		writeError(w, http.StatusBadRequest, "plan_name_required")
	case errors.Is(err, plan.ErrPlanTargetTypeUnsupported):
		writeError(w, http.StatusBadRequest, "plan_target_type_unsupported")
	case errors.Is(err, plan.ErrPlanScheduleTypeUnsupported):
		writeError(w, http.StatusBadRequest, "plan_schedule_type_unsupported")
	case errors.Is(err, plan.ErrPlanTargetScriptNameRequired):
		writeError(w, http.StatusBadRequest, "plan_target_script_name_required")
	case errors.Is(err, plan.ErrPlanTargetWorkflowDefIDRequired):
		writeError(w, http.StatusBadRequest, "plan_target_workflow_def_id_required")
	case errors.Is(err, plan.ErrPlanDeviceIDsRequired):
		writeError(w, http.StatusBadRequest, "plan_device_ids_required")
	case errors.Is(err, plan.ErrPlanDailyStartTimeInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_start_time_invalid")
	case errors.Is(err, plan.ErrPlanDailyDeadlineTimeInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_deadline_time_invalid")
	case errors.Is(err, plan.ErrPlanDefinitionRunning):
		writeError(w, http.StatusConflict, "plan_definition_running")
	case errors.Is(err, plan.ErrPlanRunDeleteNotAllowed):
		writeError(w, http.StatusConflict, "plan_run_delete_not_allowed")
	case errors.Is(err, plan.ErrPlanTodayAlreadyStarted):
		writeError(w, http.StatusConflict, "plan_today_already_started")
	case strings.Contains(err.Error(), "plan device busy"):
		var busyErr *plan.DeviceBusyError
		if errors.As(err, &busyErr) {
			writeErrorWithDetails(w, http.StatusConflict, "plan_device_busy", map[string]any{
				"busy_devices": busyErr.Details,
			})
			return
		}
		writeError(w, http.StatusConflict, "plan_device_busy")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeError(w, http.StatusRequestTimeout, "request_timeout")
	default:
		if writeExecutionReadyError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func notImplemented(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"status": "not_implemented",
			"name":   name,
		})
	}
}

func methodNotAllowed(w http.ResponseWriter, method string) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
		"status":          "method_not_allowed",
		"expected_method": method,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"status": "error",
		"error":  message,
	})
}

func writeErrorWithDetails(w http.ResponseWriter, status int, message string, details map[string]any) {
	payload := map[string]any{
		"status": "error",
		"error":  message,
	}
	for key, value := range details {
		payload[key] = value
	}
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

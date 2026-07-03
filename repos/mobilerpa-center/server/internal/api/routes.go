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
	"github.com/mobilerpa/mobilerpa-center/server/internal/software"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
)

type createScriptNameRequest struct {
	ScriptName string `json:"script_name"`
}

type deployScriptRequest struct {
	DeviceID      string `json:"device_id"`
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
	Force         bool   `json:"force"`
}

type deployScriptAllRequest struct {
	ScriptName    string `json:"script_name"`
	ScriptVersion string `json:"script_version"`
	Force         bool   `json:"force"`
}

type deployScriptItemResult struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

type deployScriptAllResponse struct {
	TotalOnlineDevices int                      `json:"total_online_devices"`
	SuccessCount       int                      `json:"success_count"`
	FailureCount       int                      `json:"failure_count"`
	Results            []deployScriptItemResult `json:"results"`
}

// RegisterRoutes 注册中心服务当前可用的 HTTP 路由。
// 为兼容历史测试调用，最后参数同时支持：
// 1. 仅传入 wsHandler
// 2. 传入 scripts, wsHandler
func RegisterRoutes(mux *http.ServeMux, devices *device.Service, tasks *task.Service, dispatcher *dispatch.Service, discoveryService *discovery.Service, extras ...any) {
	scripts := script.NewService(nil, "./data/scripts")
	systemSettings := settings.NewService(nil)
	softwareService := software.NewService(nil, "./data/software")
	plans := plan.NewService(nil, nil, nil, nil, nil, nil)
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
		case *software.Service:
			if value != nil {
				softwareService = value
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
	mux.HandleFunc("/openapi.json", openAPIDocument())
	mux.HandleFunc("/scalar", scalarDocs())
	mux.HandleFunc("/api/v1/device/register", registerDevice(devices))
	mux.HandleFunc("/api/v1/devices", listDevices(devices, plans))
	mux.HandleFunc("/api/v1/devices/all", listAllDevices(devices))
	mux.HandleFunc("/api/v1/devices/", getDevice(devices, tasks, workflows, plans))
	mux.HandleFunc("/api/v1/location-nodes", locationNodesCollection(devices))
	mux.HandleFunc("/api/v1/location-nodes/", locationNodeSubResources(devices))
	mux.HandleFunc("/api/v1/discovery/devices", listDiscoveredDevices(discoveryService))
	mux.HandleFunc("/api/v1/discovery/agent-deployments", deployAgents(discoveryService))
	mux.HandleFunc("/api/v1/discovery/agent-actions", controlAgent(discoveryService))
	mux.HandleFunc("/api/v1/discovery/software-installs", installSoftware(discoveryService, softwareService))
	mux.HandleFunc("/api/v1/discovery/software-installs/", softwareInstallSubResources(discoveryService))
	mux.HandleFunc("/api/v1/discovery/pair", pairDevice(discoveryService))
	mux.HandleFunc("/api/v1/settings/discovery", discoverySettings(systemSettings))
	mux.HandleFunc("/api/v1/settings/plans/daily-retry", planDailyRetrySettings(systemSettings))
	mux.HandleFunc("/api/v1/plans", plansCollection(plans))
	mux.HandleFunc("/api/v1/plans/", planSubResources(plans))
	mux.HandleFunc("/api/v1/script-names", scriptNamesCollection(scripts))
	mux.HandleFunc("/api/v1/scripts/deploy", deployScript(scripts, dispatcher))
	mux.HandleFunc("/api/v1/scripts/deploy-all", deployScriptToAllOnlineDevices(devices, dispatcher))
	mux.HandleFunc("/api/v1/scripts/upload", uploadScript(scripts))
	mux.HandleFunc("/api/v1/scripts", listScripts(scripts))
	mux.HandleFunc("/api/v1/scripts/", scriptsSubResources(scripts))
	mux.HandleFunc("/api/v1/software", softwareCollection(softwareService))
	mux.HandleFunc("/api/v1/software/all", listAllSoftware(softwareService))
	mux.HandleFunc("/api/v1/software/", softwareSubResources(softwareService))
	mux.HandleFunc("/api/v1/workflows", workflowsCollection(workflows))
	mux.HandleFunc("/api/v1/workflows/", workflowSubResources(workflows))
	mux.HandleFunc("/api/v1/device/upload", notImplemented("device upload"))
	mux.HandleFunc("/api/v1/script/manifest", scriptManifest(scripts))
	mux.HandleFunc("/api/v1/script/download", scriptDownload(scripts))
	mux.Handle("/ws", wsHandler)
}

// workflowsCollection godoc
// @Summary 分页获取或创建工作流定义
// @Tags Workflows
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Success 200 {object} device.RegisterResult
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/workflows [get]
// @Router /api/v1/workflows [post]
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

// plansCollection godoc
// @Summary 分页获取或创建计划任务
// @Tags Plans
// @Accept json
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Param target_type query string false "目标类型"
// @Param schedule_type query string false "调度类型"
// @Param plan_def_id query string false "计划任务定义 ID"
// @Param plan_name query string false "计划任务名称"
// @Param view query string false "当值为 runs 时返回实例列表"
// @Success 200 {array} plan.Definition
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans [get]
// @Router /api/v1/plans [post]
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
				result, err := plans.ListRuns(ctx, plan.RunListFilter{
					PlanDefID: strings.TrimSpace(r.URL.Query().Get("plan_def_id")),
					PlanName:  strings.TrimSpace(r.URL.Query().Get("plan_name")),
				})
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

			result, err := plans.ListDefinitions(ctx, plan.DefinitionListFilter{
				TargetType:   strings.TrimSpace(r.URL.Query().Get("target_type")),
				ScheduleType: strings.TrimSpace(r.URL.Query().Get("schedule_type")),
			})
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
			handlePlanDefinitionGet(ctx, w, plans, planDefID)
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			handlePlanDefinitionDelete(ctx, w, plans, planDefID)
			return
		}

		if len(parts) == 2 && parts[1] == "rows" && r.Method == http.MethodPut {
			handlePlanDefinitionRowsUpdate(ctx, w, r, plans, planDefID)
			return
		}

		if len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPut {
			handlePlanDefinitionStatusUpdate(ctx, w, r, plans, planDefID)
			return
		}

		if len(parts) == 2 && parts[1] == "runs" && r.Method == http.MethodGet {
			handlePlanRunsList(ctx, w, plans, planDefID)
			return
		}

		if len(parts) == 2 && parts[1] == "start" && r.Method == http.MethodPost {
			handlePlanStart(ctx, w, r, plans, planDefID)
			return
		}

		if len(parts) == 3 && parts[1] == "runs" && r.Method == http.MethodGet {
			handlePlanRunGet(ctx, w, plans, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "events" && r.Method == http.MethodGet {
			handlePlanRunEventsList(ctx, w, plans, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "stop" && r.Method == http.MethodPost {
			handlePlanRunStop(ctx, w, plans, parts[2])
			return
		}

		if len(parts) == 6 && parts[1] == "runs" && parts[3] == "device-runs" && parts[5] == "stop" && r.Method == http.MethodPost {
			handlePlanDeviceRunStop(ctx, w, plans, parts[2], parts[4])
			return
		}

		if len(parts) == 3 && parts[1] == "runs" && r.Method == http.MethodDelete {
			handlePlanRunDelete(ctx, w, plans, planDefID, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "runs" && parts[3] == "rows" && r.Method == http.MethodPost {
			handlePlanRunRowsAdd(ctx, w, r, plans, parts[2])
			return
		}

		if len(parts) == 6 && parts[1] == "runs" && parts[3] == "rows" && r.Method == http.MethodDelete {
			handlePlanRunRowDelete(ctx, w, plans, parts[2], parts[4], parts[5])
			return
		}

		writeError(w, http.StatusNotFound, "plan_resource_not_found")
	}
}

// handlePlanDefinitionGet godoc
// @Summary 获取计划任务定义详情
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Success 200 {object} plan.Definition
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id} [get]
func handlePlanDefinitionGet(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planDefID string) {
	result, err := plans.GetDefinition(ctx, planDefID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanDefinitionDelete godoc
// @Summary 删除计划任务定义
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id} [delete]
func handlePlanDefinitionDelete(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planDefID string) {
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
}

// handlePlanDefinitionRowsUpdate godoc
// @Summary 更新计划任务默认排选择
// @Tags Plans
// @Accept json
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param body body plan.UpdateDefinitionRowsRequest true "默认排选择"
// @Success 200 {object} plan.Definition
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/rows [put]
func handlePlanDefinitionRowsUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, plans *plan.Service, planDefID string) {
	var req plan.UpdateDefinitionRowsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	result, err := plans.UpdateDefinitionRows(ctx, planDefID, req)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanDefinitionStatusUpdate godoc
// @Summary 更新计划任务启用状态
// @Tags Plans
// @Accept json
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param body body plan.UpdateDefinitionStatusRequest true "状态请求"
// @Success 200 {object} plan.Definition
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/status [put]
func handlePlanDefinitionStatusUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, plans *plan.Service, planDefID string) {
	var req plan.UpdateDefinitionStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	result, err := plans.UpdateDefinitionStatus(ctx, planDefID, req)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanRunsList godoc
// @Summary 获取计划任务实例列表
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Success 200 {array} plan.Run
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs [get]
func handlePlanRunsList(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planDefID string) {
	result, err := plans.ListRuns(ctx, plan.RunListFilter{PlanDefID: planDefID})
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanStart godoc
// @Summary 手动启动计划任务
// @Tags Plans
// @Accept json
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param body body plan.StartRequest true "启动请求"
// @Success 200 {object} plan.Run
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/start [post]
func handlePlanStart(ctx context.Context, w http.ResponseWriter, r *http.Request, plans *plan.Service, planDefID string) {
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
}

// handlePlanRunGet godoc
// @Summary 获取单个计划任务实例
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Success 200 {object} plan.Run
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id} [get]
func handlePlanRunGet(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planRunID string) {
	result, err := plans.GetRun(ctx, planRunID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanRunEventsList godoc
// @Summary 获取计划任务实例事件列表
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Success 200 {array} plan.Event
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id}/events [get]
func handlePlanRunEventsList(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planRunID string) {
	result, err := plans.ListEvents(ctx, planRunID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanRunStop godoc
// @Summary 停止计划任务实例
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Success 200 {object} plan.Run
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id}/stop [post]
func handlePlanRunStop(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planRunID string) {
	result, err := plans.StopRun(ctx, planRunID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanDeviceRunStop godoc
// @Summary 停止计划任务实例中的单个设备
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Param plan_device_run_id path string true "计划任务设备运行 ID"
// @Success 200 {object} plan.Run
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id}/device-runs/{plan_device_run_id}/stop [post]
func handlePlanDeviceRunStop(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planRunID string, planDeviceRunID string) {
	result, err := plans.StopDeviceRun(ctx, planRunID, planDeviceRunID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanRunDelete godoc
// @Summary 删除计划任务实例
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id} [delete]
func handlePlanRunDelete(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planDefID string, planRunID string) {
	if err := plans.DeleteRun(ctx, planRunID); err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data": map[string]any{
			"plan_def_id": planDefID,
			"plan_run_id": planRunID,
			"deleted":     true,
		},
	})
}

// handlePlanRunRowsAdd godoc
// @Summary 向计划任务实例追加排
// @Tags Plans
// @Accept json
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Param body body plan.AddRowsRequest true "追加排请求"
// @Success 200 {object} plan.Run
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id}/rows [post]
func handlePlanRunRowsAdd(ctx context.Context, w http.ResponseWriter, r *http.Request, plans *plan.Service, planRunID string) {
	var req plan.AddRowsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	result, err := plans.AddRows(ctx, planRunID, req)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handlePlanRunRowDelete godoc
// @Summary 从计划任务实例移除排
// @Tags Plans
// @Produce json
// @Param plan_def_id path string true "计划任务定义 ID"
// @Param plan_run_id path string true "计划任务实例 ID"
// @Param zone_id path string true "分区 ID"
// @Param row_id path string true "排号 ID"
// @Success 200 {object} plan.Run
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/plans/{plan_def_id}/runs/{plan_run_id}/rows/{zone_id}/{row_id} [delete]
func handlePlanRunRowDelete(ctx context.Context, w http.ResponseWriter, plans *plan.Service, planRunID string, zoneID string, rowID string) {
	result, err := plans.RemoveRow(ctx, planRunID, zoneID, rowID)
	if err != nil {
		writePlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
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
			handleWorkflowDefinitionGet(ctx, w, workflows, workflowDefID)
			return
		}

		if len(parts) == 1 && r.Method == http.MethodPut {
			handleWorkflowDefinitionUpdate(ctx, w, r, workflows, workflowDefID)
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			handleWorkflowDefinitionDelete(ctx, w, workflows, workflowDefID)
			return
		}

		writeError(w, http.StatusNotFound, "workflow_resource_not_found")
	}
}

// handleWorkflowDefinitionGet godoc
// @Summary 获取工作流定义详情
// @Tags Workflows
// @Produce json
// @Param workflow_def_id path string true "工作流定义 ID"
// @Success 200 {object} workflow.Definition
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/workflows/{workflow_def_id} [get]
func handleWorkflowDefinitionGet(ctx context.Context, w http.ResponseWriter, workflows *workflow.Service, workflowDefID string) {
	result, err := workflows.GetDefinition(ctx, workflowDefID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handleWorkflowDefinitionUpdate godoc
// @Summary 更新工作流定义
// @Tags Workflows
// @Accept json
// @Produce json
// @Param workflow_def_id path string true "工作流定义 ID"
// @Param body body workflow.UpdateDefinitionRequest true "工作流定义更新请求"
// @Success 200 {object} workflow.Definition
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/workflows/{workflow_def_id} [put]
func handleWorkflowDefinitionUpdate(ctx context.Context, w http.ResponseWriter, r *http.Request, workflows *workflow.Service, workflowDefID string) {
	var req workflow.UpdateDefinitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	result, err := workflows.UpdateDefinition(ctx, workflowDefID, req)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handleWorkflowDefinitionDelete godoc
// @Summary 删除工作流定义
// @Tags Workflows
// @Produce json
// @Param workflow_def_id path string true "工作流定义 ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/workflows/{workflow_def_id} [delete]
func handleWorkflowDefinitionDelete(ctx context.Context, w http.ResponseWriter, workflows *workflow.Service, workflowDefID string) {
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
			handleScriptDelete(scripts).ServeHTTP(w, r)
			return
		}

		handleScriptVersionSubResources(scripts).ServeHTTP(w, r)
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

func planDailyRetrySettings(service *settings.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		switch r.Method {
		case http.MethodGet:
			result, err := service.GetPlanDailyRetrySettings(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodPut:
			var req settings.PlanDailyRetrySettings
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := service.SavePlanDailyRetrySettings(ctx, req)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPut},
			})
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

// listDevices godoc
// @Summary 分页获取设备列表
// @Tags Devices
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Param slot_zone query string false "分区 ID"
// @Param slot_row query string false "排号 ID"
// @Param slot_position query string false "槽位 ID"
// @Success 200 {array} device.LocationNode
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/devices [get]
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

		results, err := devices.List(ctx, device.DeviceListFilter{
			SlotZoneID:     strings.TrimSpace(r.URL.Query().Get("slot_zone")),
			SlotRowID:      strings.TrimSpace(r.URL.Query().Get("slot_row")),
			SlotPositionID: strings.TrimSpace(r.URL.Query().Get("slot_position")),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		items := make([]map[string]any, 0, len(results))
		for _, item := range results {
			deviceItem := map[string]any{
				"device_id":                           item.DeviceID,
				"agent_uuid":                          item.AgentUUID,
				"device_name":                         item.DeviceName,
				"physical_slot":                       item.PhysicalSlot,
				"group_name":                          item.GroupName,
				"slot_zone_id":                        item.SlotZoneID,
				"slot_row_id":                         item.SlotRowID,
				"slot_position_id":                    item.SlotPositionID,
				"slot_zone":                           item.SlotZone,
				"slot_row":                            item.SlotRow,
				"slot_position":                       item.SlotPosition,
				"status":                              item.Status,
				"bind_status":                         item.BindStatus,
				"ip":                                  item.IP,
				"brand":                               item.Brand,
				"model":                               item.Model,
				"android_id":                          item.AndroidID,
				"adb_serial":                          item.ADBSerial,
				"device_link_sn":                      item.DeviceLinkSN,
				"current_task_id":                     item.CurrentTaskID,
				"current_step":                        item.CurrentStep,
				"last_error":                          item.LastError,
				"accessibility_status":                item.AccessibilityStatus,
				"foreground_service_status":           item.ForegroundServiceStatus,
				"battery_optimization_ignored_status": item.BatteryOptimizationIgnoredStatus,
				"env_checked_at":                      item.EnvCheckedAt,
				"env_check_message":                   item.EnvCheckMessage,
				"last_heartbeat_at":                   item.LastHeartbeatAt,
				"created_at":                          item.CreatedAt,
				"updated_at":                          item.UpdatedAt,
				"occupancy":                           nil,
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

// listAllDevices godoc
// @Summary 获取全量设备列表
// @Tags Devices
// @Produce json
// @Param slot_zone query string false "分区 ID"
// @Param slot_row query string false "排号 ID"
// @Param slot_position query string false "槽位 ID"
// @Success 200 {object} device.LocationNode
// @Failure 500 {object} map[string]any
// @Router /api/v1/devices/all [get]
func listAllDevices(devices *device.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		results, err := devices.List(ctx, device.DeviceListFilter{
			SlotZoneID:     strings.TrimSpace(r.URL.Query().Get("slot_zone")),
			SlotRowID:      strings.TrimSpace(r.URL.Query().Get("slot_row")),
			SlotPositionID: strings.TrimSpace(r.URL.Query().Get("slot_position")),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		items := make([]map[string]any, 0, len(results))
		for _, item := range results {
			items = append(items, map[string]any{
				"device_id":                           item.DeviceID,
				"agent_uuid":                          item.AgentUUID,
				"device_name":                         item.DeviceName,
				"physical_slot":                       item.PhysicalSlot,
				"group_name":                          item.GroupName,
				"slot_zone_id":                        item.SlotZoneID,
				"slot_row_id":                         item.SlotRowID,
				"slot_position_id":                    item.SlotPositionID,
				"slot_zone":                           item.SlotZone,
				"slot_row":                            item.SlotRow,
				"slot_position":                       item.SlotPosition,
				"status":                              item.Status,
				"bind_status":                         item.BindStatus,
				"ip":                                  item.IP,
				"brand":                               item.Brand,
				"model":                               item.Model,
				"android_id":                          item.AndroidID,
				"adb_serial":                          item.ADBSerial,
				"device_link_sn":                      item.DeviceLinkSN,
				"current_task_id":                     item.CurrentTaskID,
				"current_step":                        item.CurrentStep,
				"last_error":                          item.LastError,
				"accessibility_status":                item.AccessibilityStatus,
				"foreground_service_status":           item.ForegroundServiceStatus,
				"battery_optimization_ignored_status": item.BatteryOptimizationIgnoredStatus,
				"env_checked_at":                      item.EnvCheckedAt,
				"env_check_message":                   item.EnvCheckMessage,
				"last_heartbeat_at":                   item.LastHeartbeatAt,
				"created_at":                          item.CreatedAt,
				"updated_at":                          item.UpdatedAt,
				"occupancy":                           nil,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   items,
		})
	}
}

// getDevice godoc
// @Summary 获取设备详情或删除离线设备
// @Tags Devices
// @Produce json
// @Param device_id path string true "设备 ID"
// @Success 200 {object} device.Device
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/devices/{device_id} [get]
// @Router /api/v1/devices/{device_id} [delete]
// @Router /api/v1/devices/{device_id}/occupancy [get]
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

// locationNodesCollection godoc
// @Summary 获取或创建位置节点
// @Tags Location
// @Accept json
// @Produce json
// @Success 200 {array} device.LocationNode
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/location-nodes [get]
// @Router /api/v1/location-nodes [post]
func locationNodesCollection(devices *device.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		switch r.Method {
		case http.MethodGet:
			result, err := devices.ListLocationNodes(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodPost:
			var req device.CreateLocationNodeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := devices.CreateLocationNode(ctx, req)
			if err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost},
			})
		}
	}
}

// locationNodeSubResources godoc
// @Summary 位置节点子资源
// @Tags Location
// @Accept json
// @Produce json
// @Param node_id path string true "节点 ID"
// @Success 200 {object} device.LocationNode
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/location-nodes/{node_id} [get]
// @Router /api/v1/location-nodes/{node_id} [put]
// @Router /api/v1/location-nodes/{node_id} [delete]
// @Router /api/v1/location-nodes/{node_id}/bind [post]
// @Router /api/v1/location-nodes/{node_id}/unbind [post]
func locationNodeSubResources(devices *device.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/location-nodes/")
		trimmed = strings.Trim(trimmed, "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			writeError(w, http.StatusNotFound, "location_node_not_found")
			return
		}

		nodeID := strings.TrimSpace(parts[0])
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if len(parts) == 1 && r.Method == http.MethodGet {
			result, err := devices.GetLocationNodeByID(ctx, nodeID)
			if err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodPut {
			var req device.UpdateLocationNodeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := devices.UpdateLocationNode(ctx, nodeID, req)
			if err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 1 && r.Method == http.MethodDelete {
			if err := devices.DeleteLocationNode(ctx, nodeID); err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data": map[string]any{
					"node_id": nodeID,
					"deleted": true,
				},
			})
			return
		}

		if len(parts) == 2 && parts[1] == "bind" && r.Method == http.MethodPost {
			var req device.BindLocationNodeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			result, err := devices.BindDeviceToLocationNode(ctx, nodeID, req)
			if err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		if len(parts) == 2 && parts[1] == "unbind" && r.Method == http.MethodPost {
			result, err := devices.UnbindLocationNode(ctx, nodeID)
			if err != nil {
				writeDeviceError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
			return
		}

		writeError(w, http.StatusNotFound, "location_node_resource_not_found")
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

func installSoftware(discoveryService *discovery.Service, softwareService *software.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req discovery.SoftwareInstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		packages := make([]software.Package, 0, len(req.SoftwareIDs))
		for _, softwareID := range req.SoftwareIDs {
			pkg, err := softwareService.Get(ctx, softwareID)
			if err != nil {
				writeSoftwareError(w, err)
				return
			}
			packages = append(packages, pkg)
		}

		result, err := discoveryService.StartSoftwareInstall(ctx, req, packages)
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

func softwareInstallSubResources(discoveryService *discovery.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		jobID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/discovery/software-installs/"), "/")
		if jobID == "" || strings.Contains(jobID, "/") {
			writeError(w, http.StatusNotFound, "software_install_job_not_found")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := discoveryService.GetSoftwareInstallJob(ctx, jobID)
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

// scriptNamesCollection godoc
// @Summary 获取或新增脚本名称
// @Tags Scripts
// @Accept json
// @Produce json
// @Success 200 {array} script.ScriptNameRecord
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/script-names [get]
// @Router /api/v1/script-names [post]
func scriptNamesCollection(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			result, err := scripts.ListScriptNames(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		case http.MethodPost:
			var req createScriptNameRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			result, err := scripts.CreateScriptName(ctx, req.ScriptName)
			if err != nil {
				writeScriptError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost},
			})
		}
	}
}

// @Summary 向单台设备下发脚本版本
// @Tags Scripts
// @Accept json
// @Produce json
// @Param payload body deployScriptRequest true "下发请求"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/deploy [post]
func deployScript(_ *script.Service, dispatcher *dispatch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req deployScriptRequest
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

// uploadScript godoc
// @Summary 上传脚本版本压缩包
// @Tags Scripts
// @Accept multipart/form-data
// @Produce json
// @Success 200 {object} script.UploadResult
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/upload [post]
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

// @Summary 向全部在线设备下发脚本版本
// @Tags Scripts
// @Accept json
// @Produce json
// @Param payload body deployScriptAllRequest true "批量下发请求"
// @Success 200 {object} deployScriptAllResponse
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/deploy-all [post]
func deployScriptToAllOnlineDevices(deviceService *device.Service, dispatcher *dispatch.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		var req deployScriptAllRequest
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

		results := make([]deployScriptItemResult, 0, len(devices))
		successCount := 0

		for _, item := range devices {
			if err := dispatcher.SyncScript(ctx, item.DeviceID, req.ScriptName, req.ScriptVersion, req.Force); err != nil {
				results = append(results, deployScriptItemResult{
					DeviceID: item.DeviceID,
					Status:   "error",
					Message:  err.Error(),
				})
				continue
			}

			successCount += 1
			results = append(results, deployScriptItemResult{
				DeviceID: item.DeviceID,
				Status:   "ok",
				Message:  "sync_script_dispatched",
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data": deployScriptAllResponse{
				TotalOnlineDevices: len(devices),
				SuccessCount:       successCount,
				FailureCount:       len(devices) - successCount,
				Results:            results,
			},
		})
	}
}

// listScripts godoc
// @Summary 分页获取脚本列表
// @Tags Scripts
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Success 200 {array} script.ScriptSummary
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts [get]
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

// softwareCollection godoc
// @Summary 分页获取或新增软件
// @Tags Software
// @Accept multipart/form-data
// @Produce json
// @Param page query int false "页码"
// @Param page_size query int false "分页大小"
// @Success 200 {array} software.Package
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/software [get]
// @Router /api/v1/software [post]
func softwareCollection(softwareService *software.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			page, pageSize, err := parsePagination(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}

			result, err := softwareService.List(ctx)
			if err != nil {
				writeSoftwareError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   paginateItems(result, page, pageSize),
			})

		case http.MethodPost:
			if err := r.ParseMultipartForm(128 << 20); err != nil {
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

			result, err := softwareService.Create(ctx, software.CreateRequest{
				SoftwareName: r.FormValue("software_name"),
				Description:  r.FormValue("description"),
				FileHeader:   fileHeader,
			})
			if err != nil {
				writeSoftwareError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"data":   result,
			})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPost},
			})
		}
	}
}

// listAllSoftware godoc
// @Summary 获取全量软件列表
// @Tags Software
// @Produce json
// @Success 200 {array} software.Package
// @Failure 500 {object} map[string]any
// @Router /api/v1/software/all [get]
func listAllSoftware(softwareService *software.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		result, err := softwareService.List(ctx)
		if err != nil {
			writeSoftwareError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"data":   result,
		})
	}
}

func softwareSubResources(softwareService *software.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/software/"), "/")
		softwareID := trimmed
		isDownload := false
		if strings.HasSuffix(trimmed, "/download") {
			softwareID = strings.TrimSuffix(trimmed, "/download")
			isDownload = true
		}
		if softwareID == "" || strings.Contains(softwareID, "/") {
			writeError(w, http.StatusNotFound, "software_not_found")
			return
		}

		switch r.Method {
		case http.MethodGet:
			if isDownload {
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				handleSoftwareDownload(ctx, w, r, softwareService, softwareID)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			handleSoftwareGet(ctx, w, softwareService, softwareID)

		case http.MethodPut:
			if isDownload {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			handleSoftwareUpdate(w, r, softwareService, softwareID)

		case http.MethodDelete:
			if isDownload {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			handleSoftwareDelete(ctx, w, softwareService, softwareID)

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			})
		}
	}
}

// handleSoftwareGet godoc
// @Summary 获取软件详情
// @Tags Software
// @Produce json
// @Param software_id path string true "软件 ID"
// @Success 200 {object} software.Package
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/software/{software_id} [get]
func handleSoftwareGet(ctx context.Context, w http.ResponseWriter, softwareService *software.Service, softwareID string) {
	result, err := softwareService.Get(ctx, softwareID)
	if err != nil {
		writeSoftwareError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handleSoftwareUpdate godoc
// @Summary 更新软件信息
// @Tags Software
// @Accept multipart/form-data
// @Produce json
// @Param software_id path string true "软件 ID"
// @Param software_name formData string false "软件名称"
// @Param description formData string false "软件描述"
// @Param file formData file false "软件包文件"
// @Success 200 {object} software.Package
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/software/{software_id} [put]
func handleSoftwareUpdate(w http.ResponseWriter, r *http.Request, softwareService *software.Service, softwareID string) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart_form")
		return
	}

	var fileHeader *multipart.FileHeader
	if r.MultipartForm != nil && len(r.MultipartForm.File["file"]) > 0 {
		fileHeader = r.MultipartForm.File["file"][0]
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := softwareService.Update(ctx, softwareID, software.UpdateRequest{
		SoftwareName: r.FormValue("software_name"),
		Description:  r.FormValue("description"),
		FileHeader:   fileHeader,
	})
	if err != nil {
		writeSoftwareError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handleSoftwareDelete godoc
// @Summary 删除软件
// @Tags Software
// @Produce json
// @Param software_id path string true "软件 ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/software/{software_id} [delete]
func handleSoftwareDelete(ctx context.Context, w http.ResponseWriter, softwareService *software.Service, softwareID string) {
	if err := softwareService.Delete(ctx, softwareID); err != nil {
		writeSoftwareError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data": map[string]any{
			"software_id": softwareID,
			"deleted":     true,
		},
	})
}

// handleSoftwareDownload godoc
// @Summary 下载软件包
// @Tags Software
// @Produce application/octet-stream
// @Param software_id path string true "软件 ID"
// @Success 200 {file} binary
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/software/{software_id}/download [get]
func handleSoftwareDownload(ctx context.Context, w http.ResponseWriter, r *http.Request, softwareService *software.Service, softwareID string) {
	result, err := softwareService.Get(ctx, softwareID)
	if err != nil {
		writeSoftwareError(w, err)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.PackageFileName))
	w.Header().Set("X-Software-ID", result.SoftwareID)
	w.Header().Set("X-Software-Name", result.SoftwareName)
	http.ServeFile(w, r, result.PackageStoragePath)
}

// handleScriptVersionSubResources godoc
// @Summary 脚本版本子资源分发器
func handleScriptVersionSubResources(scripts *script.Service) http.HandlerFunc {
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
			handleScriptVersionGet(ctx, w, scripts, parts[0], parts[2])
		case http.MethodDelete:
			handleScriptVersionDelete(ctx, w, scripts, parts[0], parts[2])
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodGet, http.MethodDelete},
			})
		}
	}
}

// handleScriptDelete godoc
// @Summary 删除脚本名称及其全部版本
// @Tags Scripts
// @Produce json
// @Param script_name path string true "脚本名称"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/{script_name} [delete]
func handleScriptDelete(scripts *script.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/scripts/")
		scriptName := strings.Trim(trimmed, "/")
		if scriptName == "" || strings.Contains(scriptName, "/") {
			writeError(w, http.StatusNotFound, "script_resource_not_found")
			return
		}
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"status":           "method_not_allowed",
				"expected_methods": []string{http.MethodDelete},
			})
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

// handleScriptVersionGet godoc
// @Summary 获取脚本版本详情
// @Tags Scripts
// @Produce json
// @Param script_name path string true "脚本名称"
// @Param script_version path string true "脚本版本"
// @Success 200 {object} script.Manifest
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/{script_name}/versions/{script_version} [get]
func handleScriptVersionGet(ctx context.Context, w http.ResponseWriter, scripts *script.Service, scriptName string, scriptVersion string) {
	result, err := scripts.GetManifest(ctx, scriptName, scriptVersion)
	if err != nil {
		writeScriptError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data":   result,
	})
}

// handleScriptVersionDelete godoc
// @Summary 删除脚本版本
// @Tags Scripts
// @Produce json
// @Param script_name path string true "脚本名称"
// @Param script_version path string true "脚本版本"
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
// @Failure 500 {object} map[string]any
// @Router /api/v1/scripts/{script_name}/versions/{script_version} [delete]
func handleScriptVersionDelete(ctx context.Context, w http.ResponseWriter, scripts *script.Service, scriptName string, scriptVersion string) {
	if err := scripts.DeleteVersion(ctx, scriptName, scriptVersion); err != nil {
		writeScriptError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"data": map[string]any{
			"script_name":    scriptName,
			"script_version": scriptVersion,
			"deleted":        true,
		},
	})
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
	case errors.Is(err, device.ErrLocationNodeNotFound):
		status = http.StatusNotFound
		message = "location_node_not_found"
	case errors.Is(err, device.ErrLocationNodeOccupied):
		status = http.StatusConflict
		message = "location_node_occupied"
	case errors.Is(err, device.ErrLocationNodeFieldsRequired):
		status = http.StatusBadRequest
		message = "location_node_fields_required"
	case errors.Is(err, device.ErrLocationNodeParentRequired):
		status = http.StatusBadRequest
		message = "location_node_parent_required"
	case errors.Is(err, device.ErrLocationNodeTypeUnsupported):
		status = http.StatusBadRequest
		message = "location_node_type_unsupported"
	case errors.Is(err, device.ErrLocationNodeHierarchyInvalid):
		status = http.StatusConflict
		message = "location_node_hierarchy_invalid"
	case strings.Contains(err.Error(), "UNIQUE constraint failed"):
		status = http.StatusConflict
		message = "location_node_duplicate"
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
	case errors.Is(err, discovery.ErrSoftwareIDRequired):
		writeError(w, http.StatusBadRequest, "software_id_required")
	case errors.Is(err, discovery.ErrInstallJobNotFound):
		writeError(w, http.StatusNotFound, "software_install_job_not_found")
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

func writeSoftwareError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, software.ErrSoftwareNameRequired):
		writeError(w, http.StatusBadRequest, "software_name_required")
	case errors.Is(err, software.ErrSoftwareFileRequired):
		writeError(w, http.StatusBadRequest, "software_file_required")
	case errors.Is(err, software.ErrSoftwareNotFound):
		writeError(w, http.StatusNotFound, "software_not_found")
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
	case errors.Is(err, plan.ErrPlanDailyStartTimeInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_start_time_invalid")
	case errors.Is(err, plan.ErrPlanDailyDeadlineTimeInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_deadline_time_invalid")
	case errors.Is(err, plan.ErrPlanStatusInvalid):
		writeError(w, http.StatusBadRequest, "plan_status_invalid")
	case errors.Is(err, plan.ErrPlanRetryPolicyModeInvalid):
		writeError(w, http.StatusBadRequest, "plan_retry_policy_mode_invalid")
	case errors.Is(err, plan.ErrPlanRetryIntervalInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_retry_interval_seconds_invalid")
	case errors.Is(err, plan.ErrPlanRetryStopWindowInvalid):
		writeError(w, http.StatusBadRequest, "plan_daily_retry_stop_before_deadline_minutes_invalid")
	case errors.Is(err, plan.ErrPlanDefinitionDisabled):
		writeError(w, http.StatusConflict, "plan_definition_disabled")
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

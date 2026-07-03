package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/api"
	"github.com/mobilerpa/mobilerpa-center/server/internal/config"
	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/discovery"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/plan"
	"github.com/mobilerpa/mobilerpa-center/server/internal/script"
	"github.com/mobilerpa/mobilerpa-center/server/internal/settings"
	"github.com/mobilerpa/mobilerpa-center/server/internal/software"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/internal/ws"
)

// App 负责装配配置、存储、业务服务和 HTTP 服务。
type App struct {
	cfg            config.Config
	server         *http.Server
	database       *sql.DB
	devices        *device.Service
	tasks          *task.Service
	dispatch       *dispatch.Service
	discovery      *discovery.Service
	scripts        *script.Service
	settings       *settings.Service
	software       *software.Service
	plans          *plan.Service
	workflows      *workflow.Service
	planStartQueue chan string
	planStopQueue  chan string
}

// New 创建中心服务应用，并初始化运行所需依赖。
func New() (*App, error) {
	cfg := config.Load()
	api.SetDocsAuthConfig(cfg)

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	deviceService := device.NewService(db)
	taskService := task.NewService(db)
	dispatchService := dispatch.NewService(taskService)
	discoveryService := discovery.NewService(db, cfg.ADBPath, cfg.AgentRootPath, cfg.CenterBaseURL, cfg.ToolkitPath)
	scriptService := script.NewService(db, cfg.ScriptRootPath)
	settingsService := settings.NewService(db)
	softwareService := software.NewService(db, cfg.SoftwareRootPath)
	workflowService := workflow.NewService(db, deviceService, taskService, dispatchService)
	planService := plan.NewService(db, deviceService, taskService, dispatchService, workflowService, settingsService)
	planService.SetStartFanout(cfg.PlanStartFanout)
	dispatchService.AddTaskResultHook(planService.HandleTaskResult)
	wsHandler := ws.NewHandler(deviceService, dispatchService, planService, workflowService)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, deviceService, taskService, dispatchService, discoveryService, scriptService, settingsService, softwareService, planService, workflowService, wsHandler)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: api.WithCORS(mux, cfg.CORSAllowedOrigins),
	}

	return &App{
		cfg:            cfg,
		server:         srv,
		database:       db,
		devices:        deviceService,
		tasks:          taskService,
		dispatch:       dispatchService,
		discovery:      discoveryService,
		scripts:        scriptService,
		settings:       settingsService,
		software:       softwareService,
		plans:          planService,
		workflows:      workflowService,
		planStartQueue: make(chan string, 256),
		planStopQueue:  make(chan string, 256),
	}, nil
}

// Run 启动中心服务的 HTTP 服务。
func (a *App) Run() error {
	log.Printf(
		"mobilerpa-center listening on %s with db %s, heartbeat_interval=%s, offline_timeout=%s, offline_scan_interval=%s, plan_scan_interval=%s",
		a.cfg.HTTPAddr,
		a.cfg.DBPath,
		a.cfg.HeartbeatInterval,
		a.cfg.DeviceOfflineTimeout,
		a.cfg.DeviceOfflineScanInterval,
		a.cfg.PlanScanInterval,
	)
	go a.runOfflineScanner()
	a.startPlanWorkers()
	go a.runPlanScheduler()
	return a.server.ListenAndServe()
}

func (a *App) runOfflineScanner() {
	ticker := time.NewTicker(a.cfg.DeviceOfflineScanInterval)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-a.cfg.DeviceOfflineTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		deviceIDs, err := a.devices.MarkStaleOffline(ctx, cutoff)
		cancel()
		if err != nil {
			log.Printf("scan stale devices: %v", err)
			continue
		}
		if len(deviceIDs) == 0 {
			continue
		}

		log.Printf("marked %d stale devices offline: %v", len(deviceIDs), deviceIDs)
	}
}

func (a *App) runPlanScheduler() {
	ticker := time.NewTicker(a.cfg.PlanScanInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		startCtx, startCancel := context.WithTimeout(context.Background(), 10*time.Second)
		dueDefinitionIDs, err := a.plans.ListDueDefinitionIDs(startCtx, now)
		startCancel()
		if err != nil {
			log.Printf("scan plan auto starts: %v", err)
		} else {
			for _, planDefID := range dueDefinitionIDs {
				select {
				case a.planStartQueue <- planDefID:
				default:
					log.Printf("plan start queue is full, skip enqueue plan_def_id=%s", planDefID)
				}
			}
		}

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		expiredPlanRunIDs, err := a.plans.ListExpiredRunIDs(stopCtx, now)
		stopCancel()
		if err != nil {
			log.Printf("scan plan deadlines: %v", err)
		} else {
			for _, planRunID := range expiredPlanRunIDs {
				select {
				case a.planStopQueue <- planRunID:
				default:
					log.Printf("plan stop queue is full, skip enqueue plan_run_id=%s", planRunID)
				}
			}
		}

		retryCtx, retryCancel := context.WithTimeout(context.Background(), 10*time.Second)
		retriedRuns, err := a.plans.RetryDueTargets(retryCtx, now, a.cfg.PlanRetryInterval)
		retryCancel()
		if err != nil {
			log.Printf("scan plan retries: %v", err)
			continue
		}
		if len(retriedRuns) > 0 {
			log.Printf("retry scan processed plan runs: %v", retriedRuns)
		}
	}
}

func (a *App) startPlanWorkers() {
	workerCount := a.cfg.PlanStartWorkers
	if workerCount <= 0 {
		workerCount = 1
	}

	for index := 0; index < workerCount; index++ {
		go a.runPlanStartWorker()
		go a.runPlanStopWorker()
	}
}

func (a *App) runPlanStartWorker() {
	for planDefID := range a.planStartQueue {
		now := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		planRunID, err := a.plans.AutoStartDefinition(ctx, planDefID, now)
		cancel()
		if err != nil {
			log.Printf("async auto start plan %s: %v", planDefID, err)
			continue
		}
		if planRunID != "" {
			log.Printf("auto started plan run asynchronously: plan_def_id=%s plan_run_id=%s", planDefID, planRunID)
		}
	}
}

func (a *App) runPlanStopWorker() {
	for planRunID := range a.planStopQueue {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		stoppedPlanRunID, err := a.plans.AutoStopRun(ctx, planRunID)
		cancel()
		if err != nil {
			log.Printf("async auto stop plan run %s: %v", planRunID, err)
			continue
		}
		if stoppedPlanRunID != "" {
			log.Printf("stopped plan run asynchronously by deadline: %s", stoppedPlanRunID)
		}
	}
}

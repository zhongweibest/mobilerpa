package plan

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

func TestWorkflowSessionResultKeepsStoppedDeviceNotBusy(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-workflow-session-stop-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()

	taskSvc := task.NewService(db)
	dispatchSvc := dispatch.NewService(taskSvc)
	deviceSvc := device.NewService(db)
	workflowSvc := workflow.NewService(db, deviceSvc, taskSvc, dispatchSvc)
	planSvc := NewService(db, nil, taskSvc, dispatchSvc, workflowSvc)

	now := time.Now().UTC().Format(time.RFC3339)

	workflowDef, err := workflowSvc.CreateDefinition(ctx, workflow.CreateDefinitionRequest{
		WorkflowName: "工作流停止态保护",
		Status:       workflow.DefinitionStatusActive,
		Nodes: []workflow.Node{
			{
				NodeID:        "node_1",
				NodeType:      workflow.NodeTypeScript,
				NodeName:      "脚本节点",
				ScriptName:    "demo_script",
				ScriptVersion: "v1",
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow definition: %v", err)
	}

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "工作流停止态保护",
		TargetType:          TargetTypeWorkflow,
		TargetWorkflowDefID: workflowDef.WorkflowDefID,
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "1"},
		},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	result, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_ref_id, run_date, target_type, status, started_at, finished_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		definition.PlanDefID, "1", "2026-06-29", TargetTypeWorkflow, RunStatusStopped, now, now, now, now,
	)
	if err != nil {
		t.Fatalf("seed plan run: %v", err)
	}
	insertedPlanRunID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted plan run id: %v", err)
	}
	planRunID := strconv.FormatInt(insertedPlanRunID, 10)

	result, err = db.ExecContext(ctx, `
INSERT INTO plan_device_runs (plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, started_at, finished_at, last_error, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		planRunID, definition.PlanDefID, "1", TargetTypeWorkflow, "1", DeviceRunStatusStopped, now, now, now, now,
	)
	if err != nil {
		t.Fatalf("seed plan device run: %v", err)
	}
	insertedPlanDeviceRunID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read inserted plan device run id: %v", err)
	}
	planDeviceRunID := strconv.FormatInt(insertedPlanDeviceRunID, 10)

	if err := planSvc.HandleWorkflowSessionResult(ctx, protocol.WorkflowSessionResultPayload{
		PlanRunID:       planRunID,
		PlanDeviceRunID: planDeviceRunID,
		Status:          RunStatusStopped,
		WorkflowNodeID:  "node_1",
		ResultMessage:   "stopped by test",
	}, "req-stop-1", "1"); err != nil {
		t.Fatalf("handle workflow session result: %v", err)
	}

	row := db.QueryRowContext(ctx, `
SELECT status, current_node_id, last_error
FROM plan_device_runs
WHERE id = ?`, planDeviceRunID)

	var status string
	var currentNodeID string
	var lastError string
	if err := row.Scan(&status, &currentNodeID, &lastError); err != nil {
		t.Fatalf("query plan device run status: %v", err)
	}
	if status != DeviceRunStatusStopped {
		t.Fatalf("expected stopped status after session result, got %q", status)
	}
	if currentNodeID != "node_1" {
		t.Fatalf("expected current node updated to node_1, got %q", currentNodeID)
	}
	if lastError != "" {
		t.Fatalf("expected stopped result to clear last_error, got %q", lastError)
	}

	busy, err := planSvc.GetDeviceBusyDetail(ctx, "1")
	if err != nil {
		t.Fatalf("get device busy detail: %v", err)
	}
	if busy != nil {
		t.Fatalf("expected stopped device not busy after workflow session stop, got %#v", busy)
	}
}

func TestCreateAndListDefinitions(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-service-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db, nil, nil, nil, nil)
	ctx := t.Context()

	created, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "每日微信计划",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "open_wechat",
		TargetScriptVersion: "v0.1.0",
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:59:00",
		Status:              StatusEnabled,
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "1"},
			{ZoneID: "1", RowID: "2"},
			{ZoneID: "1", RowID: "2"},
		},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	if created.PlanDefID == "" {
		t.Fatalf("expected plan_def_id")
	}
	if len(created.Rows) != 2 {
		t.Fatalf("unexpected rows: %#v", created.Rows)
	}

	items, err := service.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list plan definitions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected plan definition count: %d", len(items))
	}
	if items[0].PlanName != "每日微信计划" {
		t.Fatalf("unexpected plan name: %s", items[0].PlanName)
	}
	if items[0].DailyStartTime != "09:00:00" {
		t.Fatalf("unexpected daily_start_time: %s", items[0].DailyStartTime)
	}
}

func TestPlanDailyStartAndStopSelection(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-service-schedule-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db, nil, nil, nil, nil)
	ctx := t.Context()

	created, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "每日脚本计划",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "open_wechat",
		TargetScriptVersion: "v0.1.0",
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:00:00",
		Status:              StatusEnabled,
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "1"},
		},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	runDate := "2026-06-20"
	result, err := db.ExecContext(ctx, `
	INSERT INTO plan_runs (
    plan_def_id, target_ref_id, run_date, target_type, status, started_at, finished_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)`,
		created.PlanDefID,
		"open_wechat@v0.1.0",
		runDate,
		TargetTypeScript,
		RunStatusRunning,
		"2026-06-20T09:00:00Z",
		"2026-06-20T09:00:00Z",
		"2026-06-20T09:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert running plan run: %v", err)
	}
	insertedPlanRunID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read running plan run id: %v", err)
	}
	planRunID := "1"
	planRunID = strconv.FormatInt(insertedPlanRunID, 10)

	result, err = db.ExecContext(ctx, `
	INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, '', '', ?, ?)`,
		planRunID,
		created.PlanDefID,
		"1",
		TargetTypeScript,
		"open_wechat@v0.1.0",
		DeviceRunStatusRunning,
		"2026-06-20T09:00:00Z",
		"2026-06-20T09:00:00Z",
		"2026-06-20T09:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert running plan device run: %v", err)
	}

	started, err := service.StartDueDefinitions(ctx, time.Date(2026, 6, 20, 8, 59, 59, 0, time.Local))
	if err != nil {
		t.Fatalf("start due definitions before time: %v", err)
	}
	if len(started) != 0 {
		t.Fatalf("expected no started runs before daily start, got %v", started)
	}

	stopped, err := service.StopExpiredRuns(ctx, time.Date(2026, 6, 20, 22, 59, 59, 0, time.Local))
	if err != nil {
		t.Fatalf("stop expired runs before deadline: %v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("expected no stopped runs before deadline, got %v", stopped)
	}

	stopped, err = service.StopExpiredRuns(ctx, time.Date(2026, 6, 20, 23, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("stop expired runs at deadline: %v", err)
	}
	if len(stopped) != 1 || stopped[0] != planRunID {
		t.Fatalf("unexpected stopped runs: %v", stopped)
	}

	started, err = service.StartDueDefinitions(ctx, time.Date(2026, 6, 20, 23, 30, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("start due definitions after deadline: %v", err)
	}
	if len(started) != 0 {
		t.Fatalf("expected no started runs after daily deadline, got %v", started)
	}
}

func TestDeleteDefinitionAndUpdateRows(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-service-update-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db, nil, nil, nil, nil)
	ctx := t.Context()

	created, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "删除与改设备测试",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "open_wechat",
		TargetScriptVersion: "v0.1.0",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "1"},
			{ZoneID: "1", RowID: "2"},
		},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	updated, err := service.UpdateDefinitionRows(ctx, created.PlanDefID, UpdateDefinitionRowsRequest{
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "2"},
			{ZoneID: "1", RowID: "3"},
		},
	})
	if err != nil {
		t.Fatalf("update plan definition rows: %v", err)
	}
	if len(updated.Rows) != 2 || updated.Rows[0].RowID != "2" || updated.Rows[1].RowID != "3" {
		t.Fatalf("unexpected updated rows: %#v", updated.Rows)
	}

	if err := service.DeleteDefinition(ctx, created.PlanDefID); err != nil {
		t.Fatalf("delete plan definition: %v", err)
	}

	items, err := service.ListDefinitions(ctx)
	if err != nil {
		t.Fatalf("list plan definitions after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected zero visible definitions after delete, got %d", len(items))
	}
}

func TestDailyManualStartAndDeferredDeviceSync(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-service-daily-rules-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db, nil, nil, nil, nil)
	ctx := t.Context()

	created, err := service.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "daily 规则测试",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "open_wechat",
		TargetScriptVersion: "v0.1.0",
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:00:00",
		Status:              StatusEnabled,
		Rows: []PlanRowBinding{
			{ZoneID: "1", RowID: "1"},
		},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	now := time.Now()
	if isManualStartAllowed(created, time.Date(now.Year(), now.Month(), now.Day(), 8, 59, 0, 0, time.Local)) != true {
		t.Fatalf("expected manual start allowed before daily_start_time")
	}
	if isManualStartAllowed(created, time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.Local)) != false {
		t.Fatalf("expected manual start blocked at daily_start_time")
	}

	run := Run{
		PlanRunID: "plr_test",
		PlanDefID: created.PlanDefID,
		RunDate:   now.In(time.Local).Format("2006-01-02"),
		Status:    RunStatusRunning,
	}

	if !shouldApplyDailyAdditionsImmediately(created, run, time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.Local)) {
		t.Fatalf("expected additions applied immediately before deadline")
	}
	if shouldApplyDailyAdditionsImmediately(created, run, time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, time.Local)) {
		t.Fatalf("expected additions deferred at deadline")
	}
	if shouldApplyDailyAdditionsImmediately(created, Run{
		PlanRunID: "plr_old",
		PlanDefID: created.PlanDefID,
		RunDate:   "2000-01-01",
		Status:    RunStatusStopped,
	}, time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.Local)) {
		t.Fatalf("expected additions deferred for inactive or non-today run")
	}
}

// TestScriptPlanBlocksOnOtherPlanRun 验证脚本型计划任务启动时会避让其他未结束的计划设备运行。
func TestScriptPlanBlocksOnOtherPlanRun(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-script-plan-busy-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()

	taskSvc := task.NewService(db)
	dispatchSvc := dispatch.NewService(taskSvc)
	planSvc := NewService(db, nil, taskSvc, dispatchSvc, nil)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_ref_id, run_date, target_type, status, started_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"other-plan", "demo-script@v1", "2026-06-29", TargetTypeScript, RunStatusRunning, now, now, now,
	); err != nil {
		t.Fatalf("seed plan run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_device_runs (plan_run_id, plan_def_id, device_id, target_type, target_ref_id, status, current_node_id, started_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "other-plan", "2", TargetTypeScript, "demo-script@v1", DeviceRunStatusPending, "", now, now, now,
	); err != nil {
		t.Fatalf("seed plan device run: %v", err)
	}

	busy, err := planSvc.ensureDevicesAvailable(ctx, TargetTypeScript, "", []string{"2"})
	if err != nil {
		t.Fatalf("ensureDevicesAvailable: %v", err)
	}
	if len(busy) == 0 {
		t.Fatalf("期望脚本型计划任务被目标设备上的其他计划设备运行拦下，实际未拦")
	}
	if busy[0].OccupancyType != "plan" {
		t.Fatalf("期望 OccupancyType=plan，实际=%q", busy[0].OccupancyType)
	}

	// 对照：未被占用的设备不应被拦。
	busyOther, err := planSvc.ensureDevicesAvailable(ctx, TargetTypeScript, "", []string{"999"})
	if err != nil {
		t.Fatalf("ensureDevicesAvailable other device: %v", err)
	}
	if len(busyOther) != 0 {
		t.Fatalf("期望未占用设备不被拦，实际 busy=%#v", busyOther)
	}
}

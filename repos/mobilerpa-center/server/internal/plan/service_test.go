package plan

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
	"github.com/mobilerpa/mobilerpa-center/server/internal/task"
	"github.com/mobilerpa/mobilerpa-center/server/internal/workflow"
	"github.com/mobilerpa/mobilerpa-center/server/pkg/protocol"
)

type stubPlanDispatcher struct {
	hasConnection bool
	assignCalls   int
}

func (s *stubPlanDispatcher) AssignTask(_ context.Context, _ string) (task.Task, error) {
	s.assignCalls += 1
	if !s.hasConnection {
		return task.Task{}, dispatch.ErrDeviceNotConnected
	}
	return task.Task{}, nil
}

func (s *stubPlanDispatcher) StartWorkflowSession(_ context.Context, payload protocol.StartWorkflowSessionPayload) error {
	if !s.hasConnection && strings.TrimSpace(payload.DeviceID) != "" {
		return dispatch.ErrDeviceNotConnected
	}
	return nil
}

func (s *stubPlanDispatcher) StopWorkflowSession(_ context.Context, payload protocol.StopWorkflowSessionPayload) error {
	if !s.hasConnection && strings.TrimSpace(payload.DeviceID) != "" {
		return dispatch.ErrDeviceNotConnected
	}
	return nil
}

func (s *stubPlanDispatcher) HasDeviceConnection(_ string) bool {
	return s.hasConnection
}

func seedBoundRowDevice(t *testing.T, ctx context.Context, db *sql.DB, deviceStatus string) (string, string, string) {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339)
	deviceResult, err := db.ExecContext(ctx, `
INSERT INTO devices (
    agent_uuid, device_name, physical_slot, group_name, slot_zone, slot_row, slot_position, status, bind_status, ip,
    brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
    accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
    last_heartbeat_at, created_at, updated_at
) VALUES (?, ?, '', '', '', '', '', ?, 'bound', '', '', '', '', '', 0, '', '', 'enabled', 'enabled', 'enabled', ?, '', ?, ?, ?)`,
		"agent-"+strconv.FormatInt(time.Now().UnixNano(), 10),
		"test-device",
		deviceStatus,
		now,
		now,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	deviceIDInt, err := deviceResult.LastInsertId()
	if err != nil {
		t.Fatalf("device last insert id: %v", err)
	}
	deviceID := strconv.FormatInt(deviceIDInt, 10)

	zoneResult, err := db.ExecContext(ctx, `INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at) VALUES (0, 'zone', 'A区', 0, 1, ?, ?)`, now, now)
	if err != nil {
		t.Fatalf("insert zone: %v", err)
	}
	zoneIDInt, err := zoneResult.LastInsertId()
	if err != nil {
		t.Fatalf("zone last insert id: %v", err)
	}
	zoneID := strconv.FormatInt(zoneIDInt, 10)

	rowResult, err := db.ExecContext(ctx, `INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at) VALUES (?, 'row', '第1排', 0, 1, ?, ?)`, zoneID, now, now)
	if err != nil {
		t.Fatalf("insert row: %v", err)
	}
	rowIDInt, err := rowResult.LastInsertId()
	if err != nil {
		t.Fatalf("row last insert id: %v", err)
	}
	rowID := strconv.FormatInt(rowIDInt, 10)

	if _, err := db.ExecContext(ctx, `INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at) VALUES (?, 'slot', '01', ?, 1, ?, ?)`, rowID, deviceID, now, now); err != nil {
		t.Fatalf("insert slot: %v", err)
	}

	return deviceID, zoneID, rowID
}

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
	planSvc := NewService(db, nil, taskSvc, dispatchSvc, workflowSvc, nil)

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

func TestStopRunAllowsDisconnectedWorkflowDevice(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-stop-run-disconnected-workflow.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	workflowSvc := workflow.NewService(db, nil, nil, nil)
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, workflowSvc, nil)

	deviceID, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "online")
	workflowDef, err := workflowSvc.CreateDefinition(ctx, workflow.CreateDefinitionRequest{
		WorkflowName: "停止离线工作流设备",
		Nodes: []workflow.Node{
			{
				NodeID:       "node_1",
				NodeType:     "script",
				NodeName:     "打开QQ",
				ScriptName:   "qq",
				ScriptVersion:"v1",
			},
		},
	})
	if err != nil {
		t.Fatalf("create workflow definition: %v", err)
	}

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "停止离线工作流实例",
		TargetType:          TargetTypeWorkflow,
		TargetWorkflowDefID: workflowDef.WorkflowDefID,
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:00:00",
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_ref_id, run_date, target_type, status, started_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		definition.PlanDefID, workflowDef.WorkflowDefID, "2026-07-01", TargetTypeWorkflow, RunStatusRunning, now, now, now,
	)
	if err != nil {
		t.Fatalf("insert plan run: %v", err)
	}
	planRunIDInt, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("plan run last insert id: %v", err)
	}
	planRunID := strconv.FormatInt(planRunIDInt, 10)

	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_device_runs (
    plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id,
    status, current_node_id, next_retry_at, started_at, finished_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, '', '', ?, ?)`,
		planRunID, definition.PlanDefID, zoneID, rowID, "1", deviceID, TargetTypeWorkflow, workflowDef.WorkflowDefID,
		DeviceRunStatusRunning, "node_1", now, now, now,
	); err != nil {
		t.Fatalf("insert plan device run: %v", err)
	}

	run, err := planSvc.StopRun(ctx, planRunID)
	if err != nil {
		t.Fatalf("stop run should allow disconnected workflow device: %v", err)
	}
	if run.Status != RunStatusStopped {
		t.Fatalf("expected stopped run, got %q", run.Status)
	}
	if len(run.DeviceRuns) != 1 || run.DeviceRuns[0].Status != DeviceRunStatusStopped {
		t.Fatalf("expected stopped device run, got %#v", run.DeviceRuns)
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

	service := NewService(db, nil, nil, nil, nil, nil)
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

	service := NewService(db, nil, nil, nil, nil, nil)
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

	service := NewService(db, nil, nil, nil, nil, nil)
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

	service := NewService(db, nil, nil, nil, nil, nil)
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

	manualBeforeStart := time.Date(2026, 7, 1, 8, 59, 0, 0, time.Local)
	manualAtStart := time.Date(2026, 7, 1, 9, 0, 0, 0, time.Local)
	created.StatusUpdatedAt = time.Date(2026, 7, 1, 8, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
	if isManualStartAllowed(created, manualBeforeStart) != true {
		t.Fatalf("expected manual start allowed before daily_start_time")
	}
	if isManualStartAllowed(created, manualAtStart) != false {
		t.Fatalf("expected manual start blocked at daily_start_time when task should auto start")
	}
	created.StatusUpdatedAt = time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
	if isManualStartAllowed(created, time.Date(2026, 7, 1, 10, 1, 0, 0, time.Local)) != true {
		t.Fatalf("expected manual start allowed after enabling task later than daily_start_time")
	}

	run := Run{
		PlanRunID: "plr_test",
		PlanDefID: created.PlanDefID,
		RunDate:   manualBeforeStart.In(time.Local).Format("2006-01-02"),
		Status:    RunStatusRunning,
	}

	if !shouldApplyDailyAdditionsImmediately(created, run, time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)) {
		t.Fatalf("expected additions applied immediately before deadline")
	}
	if shouldApplyDailyAdditionsImmediately(created, run, time.Date(2026, 7, 1, 23, 0, 0, 0, time.Local)) {
		t.Fatalf("expected additions deferred at deadline")
	}
	if shouldApplyDailyAdditionsImmediately(created, Run{
		PlanRunID: "plr_old",
		PlanDefID: created.PlanDefID,
		RunDate:   "2000-01-01",
		Status:    RunStatusStopped,
	}, time.Date(2026, 7, 1, 10, 0, 0, 0, time.Local)) {
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
	planSvc := NewService(db, nil, taskSvc, dispatchSvc, nil, nil)

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

func TestStartPlanFailsBeforeCreatingRunWhenTargetDeviceBusy(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-start-busy-precheck-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: true}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	deviceID, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "online")
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_ref_id, run_date, target_type, status, started_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"other-plan", "demo_script@v1", "2026-07-01", TargetTypeScript, RunStatusRunning, now, now, now,
	); err != nil {
		t.Fatalf("seed busy plan run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_device_runs (plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "other-plan", zoneID, rowID, 1, deviceID, TargetTypeScript, "demo_script@v1", DeviceRunStatusRunning, now, now,
	); err != nil {
		t.Fatalf("seed busy plan device run: %v", err)
	}

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "占用前置校验",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	_, err = planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err == nil {
		t.Fatalf("expected busy precheck to block plan start")
	}
	var busyErr *DeviceBusyError
	if !errors.As(err, &busyErr) {
		t.Fatalf("expected DeviceBusyError, got %T %v", err, err)
	}
	if len(busyErr.Details) == 0 || busyErr.Details[0].DeviceID != deviceID {
		t.Fatalf("unexpected busy details: %#v", busyErr.Details)
	}

	runs, err := planSvc.ListRuns(ctx, definition.PlanDefID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected no new run created when busy precheck fails, got %d", len(runs))
	}
	if dispatcherStub.assignCalls != 0 {
		t.Fatalf("expected no dispatch attempt when busy precheck fails, got %d", dispatcherStub.assignCalls)
	}
}

func TestStartOncePlanSkipsOfflineDeviceWithoutError(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-start-once-offline-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	_, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "offline")

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "一次性离线跳过",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("start once plan should not fail on offline device: %v", err)
	}
	if dispatcherStub.assignCalls != 0 {
		t.Fatalf("expected offline device not dispatched, got assignCalls=%d", dispatcherStub.assignCalls)
	}
	if run.Status != RunStatusSuccess {
		t.Fatalf("expected run status success when only offline targets remain, got %q", run.Status)
	}
	if len(run.DeviceRuns) != 1 {
		t.Fatalf("expected one device run, got %d", len(run.DeviceRuns))
	}
	if run.DeviceRuns[0].Status != DeviceRunStatusFailed {
		t.Fatalf("expected offline skipped device run failed, got %q", run.DeviceRuns[0].Status)
	}
	if run.DeviceRuns[0].NextRetryAt != "" {
		t.Fatalf("expected once plan not to schedule retry, got %q", run.DeviceRuns[0].NextRetryAt)
	}

	events, err := planSvc.ListEvents(ctx, run.PlanRunID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundOfflineEvent := false
	for _, item := range events {
		if item.DeviceID == run.DeviceRuns[0].DeviceID && item.Message == "设备不在线未启动" {
			foundOfflineEvent = true
			break
		}
	}
	if !foundOfflineEvent {
		t.Fatalf("expected offline skip event, got %#v", events)
	}

	tasks, err := taskSvc.List(ctx, "")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected no tasks created for offline once plan, got %d", len(tasks))
	}
}

func TestStartOncePlanDoesNotValidateOfflineDeviceExecutionProfile(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-start-once-offline-no-env-check-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	deviceID, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "offline")
	if _, err := db.ExecContext(ctx, `
UPDATE devices
SET accessibility_status = 'disabled', foreground_service_status = 'disabled', battery_optimization_ignored_status = 'disabled'
WHERE id = ?`, deviceID); err != nil {
		t.Fatalf("update offline device execution profile: %v", err)
	}

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "离线设备不做环境校验",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("offline device should be skipped before execution-profile validation: %v", err)
	}
	if len(run.DeviceRuns) != 1 || run.DeviceRuns[0].DeviceID != deviceID {
		t.Fatalf("unexpected run device runs: %#v", run.DeviceRuns)
	}
	if run.DeviceRuns[0].Status != DeviceRunStatusFailed {
		t.Fatalf("expected offline skipped device run failed, got %q", run.DeviceRuns[0].Status)
	}
}

func TestStartOncePlanSkipsDisconnectedOnlineDeviceWithoutError(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-start-once-disconnected-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	_, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "online")

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "一次性连接缺失跳过",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("start once plan should not fail on disconnected online device: %v", err)
	}
	if dispatcherStub.assignCalls != 0 {
		t.Fatalf("expected disconnected device not dispatched, got assignCalls=%d", dispatcherStub.assignCalls)
	}
	if run.Status != RunStatusSuccess {
		t.Fatalf("expected run status success when only disconnected targets remain, got %q", run.Status)
	}
	if len(run.DeviceRuns) != 1 || run.DeviceRuns[0].NextRetryAt != "" {
		t.Fatalf("expected disconnected device run without retry, got %#v", run.DeviceRuns)
	}
	if run.DeviceRuns[0].Status != DeviceRunStatusFailed {
		t.Fatalf("expected disconnected skipped device run failed, got %q", run.DeviceRuns[0].Status)
	}
}

func TestRetryDailyPlanKeepsDisconnectedDeviceAsDeferred(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-retry-disconnected-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	deviceID, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "online")

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "按天循环连接缺失重试",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:59:59",
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{})
	if err != nil {
		t.Fatalf("start daily plan should not fail on disconnected device: %v", err)
	}
	if len(run.DeviceRuns) != 1 {
		t.Fatalf("expected one device run, got %d", len(run.DeviceRuns))
	}
	if run.DeviceRuns[0].NextRetryAt == "" {
		t.Fatalf("expected daily deferred device to keep retry timestamp")
	}
	retryAt, err := time.Parse(time.RFC3339, run.DeviceRuns[0].NextRetryAt)
	if err != nil {
		t.Fatalf("parse next_retry_at: %v", err)
	}
	retryDelay := retryAt.Sub(time.Now().UTC())
	if retryDelay < 30*time.Second || retryDelay > 90*time.Second {
		t.Fatalf("expected retry timestamp about 1 minute later, got %s", run.DeviceRuns[0].NextRetryAt)
	}

	processedRuns, err := planSvc.RetryDueTargets(ctx, time.Now().Add(30*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("retry due targets should not fail on disconnected device: %v", err)
	}
	if len(processedRuns) != 0 {
		t.Fatalf("unexpected processed runs: %v", processedRuns)
	}

	events, err := planSvc.ListEvents(ctx, run.PlanRunID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundRetryEvent := false
	for _, item := range events {
		if item.DeviceID == deviceID && strings.Contains(item.Message, "设备离线未启动") {
			foundRetryEvent = true
		}
	}
	if !foundRetryEvent {
		if len(events) != 2 {
			t.Fatalf("expected only initial offline event before retry time, got %#v", events)
		}
	}

	processedRuns, err = planSvc.RetryDueTargets(ctx, retryAt.Add(2*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("retry due targets after retry timestamp should not fail: %v", err)
	}
	if len(processedRuns) != 1 || processedRuns[0] != run.PlanRunID {
		t.Fatalf("unexpected processed runs after retry timestamp: %v", processedRuns)
	}

	events, err = planSvc.ListEvents(ctx, run.PlanRunID)
	if err != nil {
		t.Fatalf("list events after retry: %v", err)
	}
	foundRetryEvent = false
	for _, item := range events {
		if item.DeviceID == deviceID && strings.Contains(item.Message, "设备离线未启动") && strings.Contains(item.Message, "T") {
			foundRetryEvent = true
		}
	}
	if !foundRetryEvent {
		t.Fatalf("expected delayed retry offline event, got %#v", events)
	}
}

func TestOnceRunBecomesSuccessWhenOnlyOfflinePendingTargetsRemain(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-once-success-with-offline-pending.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	dispatchSvc := dispatch.NewService(taskSvc)
	planSvc := NewService(db, nil, taskSvc, dispatchSvc, nil, nil)

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_ref_id, run_date, target_type, status, started_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"1", "demo_script@v1", "2026-07-01", TargetTypeScript, RunStatusRunning, now, now, now,
	); err != nil {
		t.Fatalf("seed plan run: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO plan_device_runs (plan_run_id, plan_def_id, zone_id, row_id, slot_id, device_id, target_type, target_ref_id, status, next_retry_at, started_at, finished_at, last_error, created_at, updated_at)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', ?, ?)`,
		1, "1", "6", "7", "8", "2", TargetTypeScript, "demo_script@v1", DeviceRunStatusSuccess, "", now, now, now, now,
		1, "1", "6", "7", "9", "1", TargetTypeScript, "demo_script@v1", DeviceRunStatusPending, "", now, now,
	); err != nil {
		t.Fatalf("seed plan device runs: %v", err)
	}

	if err := planSvc.refreshRunStatus(ctx, "1"); err != nil {
		t.Fatalf("refreshRunStatus: %v", err)
	}

	run, err := planSvc.GetRun(ctx, "1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if run.Status != RunStatusSuccess {
		t.Fatalf("expected run success, got %q", run.Status)
	}
	if run.FinishedAt == "" {
		t.Fatalf("expected finished_at populated")
	}

	events, err := planSvc.ListEvents(ctx, "1")
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundCompleted := false
	for _, item := range events {
		if item.EventType == EventTypePlanRunCompleted {
			foundCompleted = true
			break
		}
	}
	if !foundCompleted {
		t.Fatalf("expected plan completed event, got %#v", events)
	}
}

func TestStartOncePlanSkippedOfflineDeviceDoesNotBlockNextStart(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-start-once-offline-not-busy-after-finish.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: false}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	deviceID, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "offline")

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "一次性离线不占用后续启动",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	firstRun, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("start first once plan: %v", err)
	}
	if firstRun.Status != RunStatusSuccess {
		t.Fatalf("expected first run success, got %q", firstRun.Status)
	}
	if len(firstRun.DeviceRuns) != 1 || firstRun.DeviceRuns[0].Status != DeviceRunStatusFailed {
		t.Fatalf("expected failed skipped device run, got %#v", firstRun.DeviceRuns)
	}

	busy, err := planSvc.GetDeviceBusyDetail(ctx, deviceID)
	if err != nil {
		t.Fatalf("get device busy detail: %v", err)
	}
	if busy != nil {
		t.Fatalf("expected skipped offline device not busy after run completion, got %#v", busy)
	}

	secondRun, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("start second once plan should not be blocked by skipped offline device: %v", err)
	}
	if secondRun.Status != RunStatusSuccess {
		t.Fatalf("expected second run success, got %q", secondRun.Status)
	}
}

func TestAddRowsToOnceRunMarksOfflineNewTargetsFailed(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-add-rows-once-offline-failed.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: true}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	onlineDeviceID, zoneID, rowIDOnline := seedBoundRowDevice(t, ctx, db, "online")

	now := time.Now().UTC().Format(time.RFC3339)
	offlineDeviceResult, err := db.ExecContext(ctx, `
INSERT INTO devices (
    agent_uuid, device_name, physical_slot, group_name, slot_zone, slot_row, slot_position, status, bind_status, ip,
    brand, model, android_id, adb_serial, current_task_id, current_step, last_error,
    accessibility_status, foreground_service_status, battery_optimization_ignored_status, env_checked_at, env_check_message,
    last_heartbeat_at, created_at, updated_at
) VALUES (?, ?, '', '', '', '', '', 'offline', 'bound', '', '', '', '', '', 0, '', '', 'enabled', 'enabled', 'enabled', ?, '', ?, ?, ?)`,
		"agent-"+strconv.FormatInt(time.Now().UnixNano(), 10),
		"test-device-offline",
		now,
		now,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert offline device: %v", err)
	}
	offlineDeviceIDInt, err := offlineDeviceResult.LastInsertId()
	if err != nil {
		t.Fatalf("offline device last insert id: %v", err)
	}
	offlineDeviceID := strconv.FormatInt(offlineDeviceIDInt, 10)

	offlineRowResult, err := db.ExecContext(ctx, `INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at) VALUES (?, 'row', '第2排', 0, 2, ?, ?)`, zoneID, now, now)
	if err != nil {
		t.Fatalf("insert offline row: %v", err)
	}
	offlineRowIDInt, err := offlineRowResult.LastInsertId()
	if err != nil {
		t.Fatalf("offline row last insert id: %v", err)
	}
	rowIDOffline := strconv.FormatInt(offlineRowIDInt, 10)

	if _, err := db.ExecContext(ctx, `INSERT INTO location_nodes (parent_id, node_type, node_name, device_id, sort_order, created_at, updated_at) VALUES (?, 'slot', '02', ?, 2, ?, ?)`, rowIDOffline, offlineDeviceID, now, now); err != nil {
		t.Fatalf("insert offline slot: %v", err)
	}

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "一次性运行中追加排离线收敛",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeOnce,
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowIDOnline}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{Manual: true})
	if err != nil {
		t.Fatalf("start once plan: %v", err)
	}
	if len(run.DeviceRuns) != 1 || run.DeviceRuns[0].DeviceID != onlineDeviceID {
		t.Fatalf("expected initial online device run, got %#v", run.DeviceRuns)
	}

	run, err = planSvc.AddRows(ctx, run.PlanRunID, AddRowsRequest{
		Rows: []PlanRowBinding{{ZoneID: zoneID, RowID: rowIDOffline}},
	})
	if err != nil {
		t.Fatalf("add rows: %v", err)
	}

	foundOffline := false
	for _, item := range run.DeviceRuns {
		if item.DeviceID != offlineDeviceID {
			continue
		}
		foundOffline = true
		if item.Status != DeviceRunStatusFailed {
			t.Fatalf("expected added offline device run failed, got %q", item.Status)
		}
		if item.NextRetryAt != "" {
			t.Fatalf("expected added offline device run without retry, got %q", item.NextRetryAt)
		}
	}
	if !foundOffline {
		t.Fatalf("expected offline device run added, got %#v", run.DeviceRuns)
	}

	events, err := planSvc.ListEvents(ctx, run.PlanRunID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundOfflineEvent := false
	foundAppendRowEvent := false
	for _, item := range events {
		if item.DeviceID == offlineDeviceID && item.Message == "设备不在线未启动" {
			foundOfflineEvent = true
		}
		if item.DeviceID == "0" && item.EventType == EventTypePlanDeviceAdded && item.Message == "整排已追加到计划任务实例" {
			foundAppendRowEvent = true
		}
	}
	if !foundOfflineEvent {
		t.Fatalf("expected added offline device event, got %#v", events)
	}
	if !foundAppendRowEvent {
		t.Fatalf("expected instance row-added event, got %#v", events)
	}
}

func TestRemoveRowAppendsPlanRunLevelEvent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "plan-remove-row-run-level-event.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := t.Context()
	taskSvc := task.NewService(db)
	deviceSvc := device.NewService(db)
	dispatcherStub := &stubPlanDispatcher{hasConnection: true}
	planSvc := NewService(db, deviceSvc, taskSvc, dispatcherStub, nil, nil)

	_, zoneID, rowID := seedBoundRowDevice(t, ctx, db, "online")

	definition, err := planSvc.CreateDefinition(ctx, CreateDefinitionRequest{
		PlanName:            "移除排实例事件",
		TargetType:          TargetTypeScript,
		TargetScriptName:    "demo_script",
		TargetScriptVersion: "v1",
		ScheduleType:        ScheduleTypeDaily,
		DailyStartTime:      "09:00:00",
		DailyDeadlineTime:   "23:00:00",
		Status:              StatusEnabled,
		Rows:                []PlanRowBinding{{ZoneID: zoneID, RowID: rowID}},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	run, err := planSvc.Start(ctx, definition.PlanDefID, StartRequest{})
	if err != nil {
		t.Fatalf("start once plan: %v", err)
	}

	if _, err := planSvc.RemoveRow(ctx, run.PlanRunID, zoneID, rowID); err != nil {
		t.Fatalf("remove row: %v", err)
	}

	events, err := planSvc.ListEvents(ctx, run.PlanRunID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	foundRemovedRowEvent := false
	for _, item := range events {
		if item.DeviceID == "0" && item.EventType == EventTypePlanDeviceRemoved && item.Message == "整排已从计划任务实例中移除" {
			foundRemovedRowEvent = true
			break
		}
	}
	if !foundRemovedRowEvent {
		t.Fatalf("expected plan-run-level row removed event, got %#v", events)
	}
}

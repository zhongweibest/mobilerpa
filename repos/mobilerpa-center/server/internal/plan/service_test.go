package plan

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
)

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
		DeviceIDs:           []string{"dev_000001", "dev_000002", "dev_000001"},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	if created.PlanDefID == "" {
		t.Fatalf("expected plan_def_id")
	}
	if len(created.DeviceIDs) != 2 {
		t.Fatalf("unexpected device ids: %#v", created.DeviceIDs)
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
		DeviceIDs:           []string{"dev_000001"},
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

func TestDeleteDefinitionAndUpdateDevices(t *testing.T) {
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
		DeviceIDs:           []string{"dev_000001", "dev_000002"},
	})
	if err != nil {
		t.Fatalf("create plan definition: %v", err)
	}

	updated, err := service.UpdateDefinitionDevices(ctx, created.PlanDefID, UpdateDefinitionDevicesRequest{
		DeviceIDs: []string{"dev_000002", "dev_000003"},
	})
	if err != nil {
		t.Fatalf("update plan definition devices: %v", err)
	}
	if len(updated.DeviceIDs) != 2 || updated.DeviceIDs[0] != "dev_000002" || updated.DeviceIDs[1] != "dev_000003" {
		t.Fatalf("unexpected updated device ids: %#v", updated.DeviceIDs)
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
		DeviceIDs:           []string{"dev_000001"},
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

package plan

import (
    "path/filepath"
    "testing"
    "time"

    "github.com/mobilerpa/mobilerpa-center/server/internal/dispatch"
    "github.com/mobilerpa/mobilerpa-center/server/internal/storage"
    "github.com/mobilerpa/mobilerpa-center/server/internal/task"
)

// TestListEventsWithEmptyDeviceID 验证 scanEvent 能容错处理 device_id 为空串的历史脏数据。
// 实例级事件（启动/停止/结束）没有具体设备，旧版代码向 INTEGER 列写入了空串，
// SQLite 动态类型原样保留，直接 Scan int64 会报 invalid syntax。修复后按字符串读出再解析。
func TestListEventsWithEmptyDeviceID(t *testing.T) {
    t.Parallel()

    dbPath := filepath.Join(t.TempDir(), "plan-events-scan-test.db")
    db, err := storage.Open(dbPath)
    if err != nil {
        t.Fatalf("open db: %v", err)
    }
    defer db.Close()

    ctx := t.Context()
    now := time.Now().UTC().Format(time.RFC3339)

    // 插入一条计划任务实例。
    if _, err := db.ExecContext(ctx, `
INSERT INTO plan_runs (plan_def_id, target_type, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)`,
        1, "script", "running", now, now,
    ); err != nil {
        t.Fatalf("seed plan run: %v", err)
    }

    // 三条事件：空串（实例级旧脏数据）、0（规范化后）、5（正常设备级）。
    events := []struct{ deviceID string }{{""}, {"0"}, {"5"}}
    for _, e := range events {
        if _, err := db.ExecContext(ctx, `
INSERT INTO plan_events (plan_run_id, plan_def_id, device_id, event_type, message, extra_json, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
            1, 1, e.deviceID, "test_event", "msg", "{}", now,
        ); err != nil {
            t.Fatalf("seed plan event: %v", err)
        }
    }

    taskSvc := task.NewService(db)
    dispatchSvc := dispatch.NewService(taskSvc)
    planSvc := NewService(db, nil, taskSvc, dispatchSvc, nil, nil)

    items, err := planSvc.ListEvents(ctx, "1")
    if err != nil {
        t.Fatalf("ListEvents should tolerate empty-string device_id: %v", err)
    }
    if len(items) != 3 {
        t.Fatalf("expected 3 events, got %d", len(items))
    }
    // 空串和 "0" 都应被解析为 "0"，"5" 保持不变。
    expected := []string{"0", "0", "5"}
    for i, item := range items {
        if item.DeviceID != expected[i] {
            t.Fatalf("event %d device_id: want %q, got %q", i, expected[i], item.DeviceID)
        }
    }
}

package storage

import (
	"context"
	"path/filepath"
	"testing"
)

// TestOpenCreatesCurrentSchema 验证当前清库重建后的关键表结构会被正确创建。
func TestOpenCreatesCurrentSchema(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "schema.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	checks := []struct {
		table  string
		column string
	}{
		{table: "devices", column: "id"},
		{table: "tasks", column: "id"},
		{table: "workflow_defs", column: "id"},
		{table: "plan_defs", column: "id"},
		{table: "plan_runs", column: "id"},
		{table: "plan_device_runs", column: "id"},
		{table: "script_versions", column: "storage_type"},
		{table: "script_versions", column: "source_type"},
	}

	for _, item := range checks {
		exists, err := columnExists(ctx, db, item.table, item.column)
		if err != nil {
			t.Fatalf("check %s.%s: %v", item.table, item.column, err)
		}
		if !exists {
			t.Fatalf("column not found: %s.%s", item.table, item.column)
		}
	}
}

func TestOpenMigratesPlanDeviceRunsColumns(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := db.Exec(`
DROP TABLE plan_device_runs;
CREATE TABLE plan_device_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_run_id INTEGER NOT NULL,
    plan_def_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    target_type TEXT NOT NULL,
    target_ref_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
)`); err != nil {
		db.Close()
		t.Fatalf("prepare legacy plan_device_runs: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer db.Close()

	for _, column := range []string{"zone_id", "row_id", "slot_id", "current_node_id", "next_retry_at"} {
		exists, err := columnExists(context.Background(), db, "plan_device_runs", column)
		if err != nil {
			t.Fatalf("check migrated column %s: %v", column, err)
		}
		if !exists {
			t.Fatalf("column not found after migration: plan_device_runs.%s", column)
		}
	}
}

func TestOpenDropsLegacyADBSerialColumn(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "legacy-devices.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := db.Exec(`
DROP TABLE devices;
CREATE TABLE devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_uuid TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT '',
    physical_slot TEXT NOT NULL DEFAULT '',
    group_name TEXT NOT NULL DEFAULT '',
    slot_zone_id TEXT NOT NULL DEFAULT '',
    slot_row_id TEXT NOT NULL DEFAULT '',
    slot_position_id TEXT NOT NULL DEFAULT '',
    slot_zone TEXT NOT NULL DEFAULT '',
    slot_row TEXT NOT NULL DEFAULT '',
    slot_position TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'offline',
    bind_status TEXT NOT NULL DEFAULT 'pending',
    ip TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    android_id TEXT NOT NULL DEFAULT '',
    adb_serial TEXT NOT NULL DEFAULT '',
    device_link_sn TEXT NOT NULL DEFAULT '',
    current_task_id INTEGER NOT NULL DEFAULT 0,
    current_step TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    accessibility_status TEXT NOT NULL DEFAULT 'unknown',
    foreground_service_status TEXT NOT NULL DEFAULT 'unknown',
    battery_optimization_ignored_status TEXT NOT NULL DEFAULT 'unknown',
    env_checked_at TEXT NOT NULL DEFAULT '',
    env_check_message TEXT NOT NULL DEFAULT '',
    last_heartbeat_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
)`); err != nil {
		db.Close()
		t.Fatalf("prepare legacy devices: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer db.Close()

	exists, err := columnExists(context.Background(), db, "devices", "adb_serial")
	if err != nil {
		t.Fatalf("check removed column adb_serial: %v", err)
	}
	if exists {
		t.Fatalf("column still exists after migration: devices.adb_serial")
	}
}

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

func TestOpenMigratesPlanDeviceRunsCurrentNodeID(t *testing.T) {
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

	exists, err := columnExists(context.Background(), db, "plan_device_runs", "current_node_id")
	if err != nil {
		t.Fatalf("check migrated column: %v", err)
	}
	if !exists {
		t.Fatalf("column not found after migration: plan_device_runs.current_node_id")
	}
}

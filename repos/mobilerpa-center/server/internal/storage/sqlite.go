package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// 初始表结构定义中心服务的 SQLite 表结构。
const schema = `
CREATE TABLE IF NOT EXISTS devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_uuid TEXT NOT NULL UNIQUE,
    device_name TEXT NOT NULL DEFAULT '',
    physical_slot TEXT NOT NULL DEFAULT '',
    group_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'offline',
    bind_status TEXT NOT NULL DEFAULT 'pending',
    ip TEXT NOT NULL DEFAULT '',
    brand TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    android_id TEXT NOT NULL DEFAULT '',
    adb_serial TEXT NOT NULL DEFAULT '',
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
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    plan_run_id INTEGER NOT NULL DEFAULT 0,
    plan_device_run_id INTEGER NOT NULL DEFAULT 0,
    workflow_instance_id INTEGER NOT NULL DEFAULT 0,
    workflow_run_id INTEGER NOT NULL DEFAULT 0,
    workflow_node_id TEXT NOT NULL DEFAULT '',
    task_source_type TEXT NOT NULL DEFAULT 'manual',
    script_name TEXT NOT NULL,
    script_version TEXT NOT NULL DEFAULT '',
    params_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    current_step TEXT NOT NULL DEFAULT '',
    result_code TEXT NOT NULL DEFAULT '',
    result_message TEXT NOT NULL DEFAULT '',
    scheduled_at TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS task_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    step_name TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    extra_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS script_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    script_name TEXT NOT NULL,
    version TEXT NOT NULL,
    entry_file TEXT NOT NULL,
    checksum TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    storage_type TEXT NOT NULL DEFAULT 'directory',
    source_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'dev',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS uploaded_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL DEFAULT 0,
    file_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS daily_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_date TEXT NOT NULL,
    device_id INTEGER NOT NULL,
    task_total INTEGER NOT NULL DEFAULT 0,
    success_total INTEGER NOT NULL DEFAULT 0,
    failed_total INTEGER NOT NULL DEFAULT 0,
    duration_sec INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS system_settings (
    setting_key TEXT PRIMARY KEY,
    setting_value TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_defs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'draft',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_def_id INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    node_type TEXT NOT NULL,
    node_name TEXT NOT NULL DEFAULT '',
    script_name TEXT NOT NULL DEFAULT '',
    script_version TEXT NOT NULL DEFAULT '',
    max_iterations INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (workflow_def_id, node_id)
);

CREATE TABLE IF NOT EXISTS workflow_edges (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_def_id INTEGER NOT NULL,
    from_node_id TEXT NOT NULL,
    to_node_id TEXT NOT NULL,
    edge_type TEXT NOT NULL DEFAULT 'next',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_instances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_run_id INTEGER NOT NULL DEFAULT 0,
    workflow_def_id INTEGER NOT NULL,
    workflow_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_instance_id INTEGER NOT NULL DEFAULT 0,
    workflow_def_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    current_node_id TEXT NOT NULL DEFAULT '',
    current_task_id INTEGER NOT NULL DEFAULT 0,
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_instance_id INTEGER NOT NULL DEFAULT 0,
    workflow_run_id INTEGER NOT NULL,
    workflow_def_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    node_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    extra_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workflow_contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_run_id INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    context_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL,
    UNIQUE (workflow_run_id, node_id)
);

CREATE TABLE IF NOT EXISTS plan_defs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL,
    target_script_name TEXT NOT NULL DEFAULT '',
    target_script_version TEXT NOT NULL DEFAULT '',
    target_workflow_def_id INTEGER NOT NULL DEFAULT 0,
    schedule_type TEXT NOT NULL DEFAULT 'once',
    daily_start_time TEXT NOT NULL DEFAULT '',
    daily_deadline_time TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'enabled',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS plan_devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_def_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (plan_def_id, device_id)
);

CREATE TABLE IF NOT EXISTS plan_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_def_id INTEGER NOT NULL,
    target_ref_id TEXT NOT NULL DEFAULT '',
    run_date TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS plan_device_runs (
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
);

CREATE TABLE IF NOT EXISTS plan_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_run_id INTEGER NOT NULL,
    plan_def_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL DEFAULT 0,
    event_type TEXT NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    extra_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS plan_contexts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_run_id INTEGER NOT NULL,
    device_id INTEGER NOT NULL,
    context_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL,
    UNIQUE (plan_run_id, device_id)
);
`

// Open 打开数据库并初始化最新表结构。
func Open(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := initSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func initSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	return nil
}

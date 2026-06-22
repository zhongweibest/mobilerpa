package storage

import (
	"context"
	"database/sql"
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
		{table: "workflow_runs", column: "id"},
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

func columnExists(ctx context.Context, db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			valueType string
			notNull   int
			defaultV  any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &valueType, &notNull, &defaultV, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

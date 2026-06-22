package task

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mobilerpa/mobilerpa-center/server/internal/device"
	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
)

func newTestTaskService(t *testing.T) (*Service, *device.Service) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "task-service-test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return NewService(db), device.NewService(db)
}

func registerTaskTestDevice(t *testing.T, ctx context.Context, devices *device.Service, agentUUID string) string {
	t.Helper()

	req := httptest.NewRequest("POST", "http://127.0.0.1/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	item, err := devices.Register(ctx, device.RegisterRequest{
		AgentUUID:  agentUUID,
		DeviceName: agentUUID,
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	return item.DeviceID
}

func TestCreateUsesNextMaxTaskIDAfterDeletion(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	service, devices := newTestTaskService(t)
	deviceID := registerTaskTestDevice(t, ctx, devices, "agent-task-001")

	firstTask, err := service.Create(ctx, CreateRequest{
		DeviceID:      deviceID,
		ScriptName:    "open_xiaohongshu",
		ScriptVersion: "v0.1.0",
		Priority:      1,
	})
	if err != nil {
		t.Fatalf("create first task: %v", err)
	}
	if firstTask.TaskID != "1" {
		t.Fatalf("unexpected first task id: %s", firstTask.TaskID)
	}

	if _, err := service.db.ExecContext(ctx, `DELETE FROM task_events WHERE task_id = ?`, firstTask.TaskID); err != nil {
		t.Fatalf("delete first task events: %v", err)
	}
	if _, err := service.db.ExecContext(ctx, `DELETE FROM tasks WHERE id = ?`, firstTask.TaskID); err != nil {
		t.Fatalf("delete first task: %v", err)
	}

	secondTask, err := service.Create(ctx, CreateRequest{
		DeviceID:      deviceID,
		ScriptName:    "open_douyin",
		ScriptVersion: "v0.1.0",
		Priority:      1,
	})
	if err != nil {
		t.Fatalf("create second task: %v", err)
	}
	if secondTask.TaskID != "2" {
		t.Fatalf("unexpected second task id: %s", secondTask.TaskID)
	}
}

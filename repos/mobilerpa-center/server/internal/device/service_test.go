package device

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mobilerpa/mobilerpa-center/server/internal/storage"
)

func TestMarkStaleOffline(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "device-service.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db)
	req := httptest.NewRequest("POST", "/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	result, err := service.Register(context.Background(), RegisterRequest{
		AgentUUID:  "agent-stale-001",
		DeviceName: "Pixel Test",
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	staleSeenAt := time.Now().Add(-2 * time.Minute)
	becameOnline, err := service.MarkOnline(context.Background(), result.DeviceID, staleSeenAt)
	if err != nil {
		t.Fatalf("mark online stale device: %v", err)
	}
	if !becameOnline {
		t.Fatalf("expected stale device to become online on first heartbeat")
	}

	freshReq := httptest.NewRequest("POST", "/api/v1/device/register", nil)
	freshReq.RemoteAddr = "127.0.0.1:12346"
	freshResult, err := service.Register(context.Background(), RegisterRequest{
		AgentUUID:  "agent-fresh-001",
		DeviceName: "Pixel Fresh",
		Brand:      "Google",
		Model:      "Pixel 8",
	}, freshReq)
	if err != nil {
		t.Fatalf("register fresh device: %v", err)
	}

	freshSeenAt := time.Now()
	becameOnline, err = service.MarkOnline(context.Background(), freshResult.DeviceID, freshSeenAt)
	if err != nil {
		t.Fatalf("mark online fresh device: %v", err)
	}
	if !becameOnline {
		t.Fatalf("expected fresh device to become online on first heartbeat")
	}

	becameOnline, err = service.MarkOnline(context.Background(), freshResult.DeviceID, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatalf("mark online fresh device second time: %v", err)
	}
	if becameOnline {
		t.Fatalf("expected second heartbeat to keep device online without reporting a new online transition")
	}

	marked, err := service.MarkStaleOffline(context.Background(), time.Now().Add(-90*time.Second))
	if err != nil {
		t.Fatalf("mark stale offline: %v", err)
	}

	if len(marked) != 1 || marked[0] != result.DeviceID {
		t.Fatalf("unexpected marked devices: %#v", marked)
	}

	staleDevice, err := service.GetByID(context.Background(), result.DeviceID)
	if err != nil {
		t.Fatalf("get stale device: %v", err)
	}
	if staleDevice.Status != "offline" {
		t.Fatalf("unexpected stale device status: %s", staleDevice.Status)
	}
	if staleDevice.LastHeartbeatAt == "" {
		t.Fatalf("expected stale device last heartbeat to be preserved")
	}

	freshDevice, err := service.GetByID(context.Background(), freshResult.DeviceID)
	if err != nil {
		t.Fatalf("get fresh device: %v", err)
	}
	if freshDevice.Status != "online" {
		t.Fatalf("unexpected fresh device status: %s", freshDevice.Status)
	}
}

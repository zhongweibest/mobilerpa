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

func TestCreateBindAndUnbindLocationNode(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "device-slot-service.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db)
	req := httptest.NewRequest("POST", "/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	deviceRecord, err := service.Register(context.Background(), RegisterRequest{
		AgentUUID:  "agent-slot-001",
		DeviceName: "Slot Device",
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	zone, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		NodeType: "zone",
		NodeName: "A区",
	})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	row, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: zone.NodeID,
		NodeType: "row",
		NodeName: "第1排",
	})
	if err != nil {
		t.Fatalf("create row: %v", err)
	}
	slot, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: row.NodeID,
		NodeType: "slot",
		NodeName: "01",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}
	if slot.DeviceID != "" {
		t.Fatalf("expected new slot node to be empty, got %q", slot.DeviceID)
	}

	boundSlot, err := service.BindDeviceToLocationNode(context.Background(), slot.NodeID, BindLocationNodeRequest{
		DeviceID: deviceRecord.DeviceID,
	})
	if err != nil {
		t.Fatalf("bind location node: %v", err)
	}
	if boundSlot.DeviceID != deviceRecord.DeviceID {
		t.Fatalf("unexpected bound location node device id: %s", boundSlot.DeviceID)
	}

	boundDevice, err := service.GetByID(context.Background(), deviceRecord.DeviceID)
	if err != nil {
		t.Fatalf("get bound device: %v", err)
	}
	if boundDevice.PhysicalSlot != "A区-第1排-01" {
		t.Fatalf("unexpected physical slot: %s", boundDevice.PhysicalSlot)
	}
	if boundDevice.SlotZone != "A区" || boundDevice.SlotRow != "第1排" || boundDevice.SlotPosition != "01" {
		t.Fatalf("unexpected split slot fields: %#v", boundDevice)
	}
	if boundDevice.BindStatus != "bound" {
		t.Fatalf("unexpected bind status: %s", boundDevice.BindStatus)
	}

	unboundSlot, err := service.UnbindLocationNode(context.Background(), slot.NodeID)
	if err != nil {
		t.Fatalf("unbind location node: %v", err)
	}
	if unboundSlot.DeviceID != "" {
		t.Fatalf("expected slot to be empty after unbind, got %q", unboundSlot.DeviceID)
	}

	unboundDevice, err := service.GetByID(context.Background(), deviceRecord.DeviceID)
	if err != nil {
		t.Fatalf("get unbound device: %v", err)
	}
	if unboundDevice.PhysicalSlot != "" || unboundDevice.SlotZone != "" || unboundDevice.SlotRow != "" || unboundDevice.SlotPosition != "" {
		t.Fatalf("expected cleared slot fields after unbind: %#v", unboundDevice)
	}
	if unboundDevice.BindStatus != "pending" {
		t.Fatalf("unexpected bind status after unbind: %s", unboundDevice.BindStatus)
	}
}

func TestUpdateLocationNodeMovesRowAndRefreshesBoundDevice(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "device-slot-update-service.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db)
	req := httptest.NewRequest("POST", "/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	deviceRecord, err := service.Register(context.Background(), RegisterRequest{
		AgentUUID:  "agent-slot-update-001",
		DeviceName: "Slot Update Device",
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	zoneA, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{NodeType: "zone", NodeName: "A区"})
	if err != nil {
		t.Fatalf("create zone A: %v", err)
	}
	zoneB, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{NodeType: "zone", NodeName: "B区"})
	if err != nil {
		t.Fatalf("create zone B: %v", err)
	}
	row, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: zoneA.NodeID,
		NodeType: "row",
		NodeName: "第1排",
	})
	if err != nil {
		t.Fatalf("create row: %v", err)
	}
	slot, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: row.NodeID,
		NodeType: "slot",
		NodeName: "01",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	if _, err := service.BindDeviceToLocationNode(context.Background(), slot.NodeID, BindLocationNodeRequest{
		DeviceID: deviceRecord.DeviceID,
	}); err != nil {
		t.Fatalf("bind location node: %v", err)
	}

	updatedRow, err := service.UpdateLocationNode(context.Background(), row.NodeID, UpdateLocationNodeRequest{
		ParentID:  zoneB.NodeID,
		NodeName:  "第9排",
		SortOrder: 12,
	})
	if err != nil {
		t.Fatalf("update row: %v", err)
	}

	if updatedRow.ParentID != zoneB.NodeID {
		t.Fatalf("unexpected updated row parent id: %s", updatedRow.ParentID)
	}
	if updatedRow.ZoneName != "B区" || updatedRow.RowName != "第9排" || updatedRow.PathText != "B区-第9排" {
		t.Fatalf("unexpected updated row path info: %#v", updatedRow)
	}
	if updatedRow.SortOrder != 12 {
		t.Fatalf("unexpected updated row sort order: %d", updatedRow.SortOrder)
	}

	updatedSlot, err := service.GetLocationNodeByID(context.Background(), slot.NodeID)
	if err != nil {
		t.Fatalf("get updated slot: %v", err)
	}
	if updatedSlot.ZoneName != "B区" || updatedSlot.RowName != "第9排" || updatedSlot.PathText != "B区-第9排-01" {
		t.Fatalf("unexpected updated slot path: %#v", updatedSlot)
	}

	boundDevice, err := service.GetByID(context.Background(), deviceRecord.DeviceID)
	if err != nil {
		t.Fatalf("get bound device after row move: %v", err)
	}
	if boundDevice.PhysicalSlot != "B区-第9排-01" {
		t.Fatalf("unexpected physical slot after row move: %s", boundDevice.PhysicalSlot)
	}
	if boundDevice.SlotZone != "B区" || boundDevice.SlotRow != "第9排" || boundDevice.SlotPosition != "01" {
		t.Fatalf("unexpected split slot fields after row move: %#v", boundDevice)
	}
}

func TestDeleteLocationNodeRemovesSubtreeAndClearsBoundDevice(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "device-slot-delete-service.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	service := NewService(db)
	req := httptest.NewRequest("POST", "/api/v1/device/register", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	deviceRecord, err := service.Register(context.Background(), RegisterRequest{
		AgentUUID:  "agent-slot-delete-001",
		DeviceName: "Slot Delete Device",
		Brand:      "Google",
		Model:      "Pixel 8",
	}, req)
	if err != nil {
		t.Fatalf("register device: %v", err)
	}

	zone, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{NodeType: "zone", NodeName: "A区"})
	if err != nil {
		t.Fatalf("create zone: %v", err)
	}
	row, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: zone.NodeID,
		NodeType: "row",
		NodeName: "第1排",
	})
	if err != nil {
		t.Fatalf("create row: %v", err)
	}
	slot, err := service.CreateLocationNode(context.Background(), CreateLocationNodeRequest{
		ParentID: row.NodeID,
		NodeType: "slot",
		NodeName: "01",
	})
	if err != nil {
		t.Fatalf("create slot: %v", err)
	}

	if _, err := service.BindDeviceToLocationNode(context.Background(), slot.NodeID, BindLocationNodeRequest{
		DeviceID: deviceRecord.DeviceID,
	}); err != nil {
		t.Fatalf("bind location node: %v", err)
	}

	if err := service.DeleteLocationNode(context.Background(), row.NodeID); err != nil {
		t.Fatalf("delete row subtree: %v", err)
	}

	if _, err := service.GetLocationNodeByID(context.Background(), row.NodeID); err == nil {
		t.Fatalf("expected deleted row to be missing")
	}
	if _, err := service.GetLocationNodeByID(context.Background(), slot.NodeID); err == nil {
		t.Fatalf("expected deleted slot to be missing")
	}

	boundDevice, err := service.GetByID(context.Background(), deviceRecord.DeviceID)
	if err != nil {
		t.Fatalf("get device after subtree delete: %v", err)
	}
	if boundDevice.PhysicalSlot != "" || boundDevice.SlotZone != "" || boundDevice.SlotRow != "" || boundDevice.SlotPosition != "" {
		t.Fatalf("expected slot fields cleared after subtree delete: %#v", boundDevice)
	}
	if boundDevice.BindStatus != "pending" {
		t.Fatalf("unexpected bind status after subtree delete: %s", boundDevice.BindStatus)
	}
}

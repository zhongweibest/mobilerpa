import type {
  BindLocationNodeRequest,
  CreateLocationNodeRequest,
  DeviceOccupancyDetail,
  DeviceRecord,
  LocationNodeRecord,
  UpdateLocationNodeRequest
} from "../types/device";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";
import { requestJSON } from "./http";

export function fetchDevices(query: PaginationQuery): Promise<PaginatedResult<DeviceRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  if (query.slot_zone?.trim()) {
    searchParams.set("slot_zone", query.slot_zone.trim());
  }
  if (query.slot_row?.trim()) {
    searchParams.set("slot_row", query.slot_row.trim());
  }
  if (query.slot_position?.trim()) {
    searchParams.set("slot_position", query.slot_position.trim());
  }
  return requestJSON<PaginatedResult<DeviceRecord>>(`/api/v1/devices?${searchParams.toString()}`);
}

export function fetchAllDevices(): Promise<DeviceRecord[]> {
  return requestJSON<DeviceRecord[]>("/api/v1/devices/all");
}

export function fetchDeviceByID(deviceID: string): Promise<DeviceRecord> {
  return requestJSON<DeviceRecord>(`/api/v1/devices/${encodeURIComponent(deviceID)}`);
}

export function deleteDevice(deviceID: string): Promise<{ device_id: string; deleted: boolean }> {
  return requestJSON<{ device_id: string; deleted: boolean }>(`/api/v1/devices/${encodeURIComponent(deviceID)}`, {
    method: "DELETE"
  });
}

export function fetchDeviceOccupancy(deviceID: string): Promise<DeviceOccupancyDetail> {
  return requestJSON<DeviceOccupancyDetail>(`/api/v1/devices/${encodeURIComponent(deviceID)}/occupancy`);
}

export function fetchLocationNodes(): Promise<LocationNodeRecord[]> {
  return requestJSON<LocationNodeRecord[]>("/api/v1/location-nodes");
}

export function createLocationNode(payload: CreateLocationNodeRequest): Promise<LocationNodeRecord> {
  return requestJSON<LocationNodeRecord>("/api/v1/location-nodes", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function updateLocationNode(nodeID: string, payload: UpdateLocationNodeRequest): Promise<LocationNodeRecord> {
  return requestJSON<LocationNodeRecord>(`/api/v1/location-nodes/${encodeURIComponent(nodeID)}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function deleteLocationNode(nodeID: string): Promise<{ node_id: string; deleted: boolean }> {
  return requestJSON<{ node_id: string; deleted: boolean }>(`/api/v1/location-nodes/${encodeURIComponent(nodeID)}`, {
    method: "DELETE"
  });
}

export function bindLocationNode(nodeID: string, payload: BindLocationNodeRequest): Promise<LocationNodeRecord> {
  return requestJSON<LocationNodeRecord>(`/api/v1/location-nodes/${encodeURIComponent(nodeID)}/bind`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function unbindLocationNode(nodeID: string): Promise<LocationNodeRecord> {
  return requestJSON<LocationNodeRecord>(`/api/v1/location-nodes/${encodeURIComponent(nodeID)}/unbind`, {
    method: "POST"
  });
}

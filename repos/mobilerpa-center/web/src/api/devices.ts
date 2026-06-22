import type { DeviceOccupancyDetail, DeviceRecord } from "../types/device";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";
import { requestJSON } from "./http";

export function fetchDevices(query: PaginationQuery): Promise<PaginatedResult<DeviceRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<DeviceRecord>>(`/api/v1/devices?${searchParams.toString()}`);
}

export function deleteDevice(deviceID: string): Promise<{ device_id: string; deleted: boolean }> {
  return requestJSON<{ device_id: string; deleted: boolean }>(`/api/v1/devices/${encodeURIComponent(deviceID)}`, {
    method: "DELETE"
  });
}

export function fetchDeviceOccupancy(deviceID: string): Promise<DeviceOccupancyDetail> {
  return requestJSON<DeviceOccupancyDetail>(`/api/v1/devices/${encodeURIComponent(deviceID)}/occupancy`);
}

import type { DeviceBindStatus, DeviceRecord, DeviceStatus } from "../types/device";

export function normalizeDeviceStatus(status: string): DeviceStatus {
  if (status === "online" || status === "offline") {
    return status;
  }
  return "unknown";
}

export function normalizeBindStatus(bindStatus: string): DeviceBindStatus {
  if (bindStatus === "pending" || bindStatus === "bound") {
    return bindStatus;
  }
  return "unknown";
}

export function getDeviceDisplayName(device: DeviceRecord): string {
  if (device.device_name.trim() !== "") {
    return device.device_name;
  }
  return device.device_id;
}

export function formatDateTime(value: string): string {
  if (value.trim() === "") {
    return "暂无";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return date.toLocaleString("zh-CN", {
    hour12: false
  });
}

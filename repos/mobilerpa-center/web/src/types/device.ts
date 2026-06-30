export type DeviceStatus = "online" | "offline" | "unknown";
export type DeviceBindStatus = "pending" | "bound" | "unknown";

export interface DeviceRecord {
  device_id: string;
  agent_uuid: string;
  device_name: string;
  physical_slot: string;
  slot_zone: string;
  slot_row: string;
  slot_position: string;
  group_name: string;
  status: string;
  bind_status: string;
  ip: string;
  brand: string;
  model: string;
  android_id: string;
  adb_serial: string;
  current_task_id: string;
  current_step: string;
  last_error: string;
  accessibility_status: string;
  foreground_service_status: string;
  battery_optimization_ignored_status: string;
  env_checked_at: string;
  env_check_message: string;
  last_heartbeat_at: string;
  created_at: string;
  updated_at: string;
  occupancy: DeviceOccupancyRecord | null;
}

export interface DeviceOccupancyRecord {
  device_id: string;
  occupancy_type: string;
  task_id: string;
  task_status: string;
  message: string;
}

export interface DeviceOccupancyDetail {
  device_id: string;
  device_status: string;
  current_task_id: string;
  current_step: string;
  last_error: string;
  occupancy: DeviceOccupancyRecord | null;
}

export interface LocationNodeRecord {
  node_id: string;
  parent_id: string;
  node_type: string;
  node_name: string;
  device_id: string;
  sort_order: number;
  created_at: string;
  updated_at: string;
  zone_name: string;
  row_name: string;
  slot_name: string;
  path_text: string;
}

export interface CreateLocationNodeRequest {
  parent_id: string;
  node_type: string;
  node_name: string;
  sort_order?: number;
}

export interface UpdateLocationNodeRequest {
  parent_id: string;
  node_name: string;
  sort_order?: number;
}

export interface BindLocationNodeRequest {
  device_id: string;
}

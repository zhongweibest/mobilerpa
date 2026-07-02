export interface DeviceInfo {
  device_name: string;
  brand: string;
  model: string;
  android_id: string;
  adb_serial: string;
}

export interface WebSocketConfig {
  enabled?: boolean;
  heartbeat_interval_ms?: number;
  reconnect_enabled?: boolean;
  reconnect_initial_delay_ms?: number;
  reconnect_max_delay_ms?: number;
  reconnect_backoff_multiplier?: number;
}

export interface LastRegisterState {
  status?: string;
  device_id?: string;
  bind_status?: string;
  register_status?: string;
  registered_at?: string;
}

export interface AgentConfig {
  center_base_url: string;
  agent_uuid: string;
  device_id: string;
  device_link_sn?: string;
  device?: Partial<DeviceInfo>;
  websocket?: WebSocketConfig;
  last_register?: LastRegisterState;
  created_at?: string;
  updated_at?: string;
}

export interface BootstrapConfig {
  center_base_url?: string;
  device_link_sn?: string;
  websocket?: WebSocketConfig;
}

export interface AgentCLIOptions {
  center: string;
  config: string;
  dryRun: boolean;
  skipWS: boolean;
}

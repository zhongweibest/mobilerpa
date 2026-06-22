export interface DiscoveredDevice {
  service_name: string;
  service_type: string;
  adb_endpoint: string;
  device_name: string;
  device_id: string;
  source: string;
  connection_kind: string;
  connected: boolean;
  connectable: boolean;
  last_error: string;
}

export interface AgentDeploymentRequest {
  adb_endpoints: string[];
  center_base_url: string;
  reset_config: boolean;
  run_agent: boolean;
}

export interface AgentDeploymentResult {
  adb_endpoint: string;
  connected: boolean;
  pushed: boolean;
  started: boolean;
  status: string;
  message: string;
}

export interface AgentActionRequest {
  adb_endpoint: string;
  action: "start" | "stop" | "disconnect";
}

export interface AgentActionResult {
  adb_endpoint: string;
  action: "start" | "stop" | "disconnect";
  status: string;
  message: string;
}

export interface PairDeviceRequest {
  host: string;
  port: string;
  pair_code: string;
}

export interface PairDeviceResult {
  adb_endpoint: string;
  status: string;
  message: string;
}

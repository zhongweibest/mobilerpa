export interface RegisterPayload {
  agent_uuid: string;
  device_name: string;
  brand: string;
  model: string;
  android_id: string;
  adb_serial: string;
  device_link_sn: string;
}

export interface RegisterResponseData {
  device_id?: string;
  bind_status?: string;
  status?: string;
}

export interface RegisterResponse {
  status?: string;
  data?: RegisterResponseData;
}

export interface TaskSummary {
  task_id: string;
  script_name: string;
  script_version: string;
  priority: number;
  params: Record<string, unknown>;
}

export interface TaskRunnerContext {
  task_id: string;
  script_name: string;
  script_version: string;
  priority: number;
  params: Record<string, unknown>;
  device_id: string;
  agent_uuid: string;
  center_base_url: string;
}

export interface TaskProgress {
  task_id?: string;
  status: string;
  step_name: string;
  message: string;
  extra?: Record<string, unknown>;
}

export interface TaskResult {
  status: string;
  result_code: string;
  result_message: string;
  step_name: string;
  extra?: Record<string, unknown>;
}

export interface WorkflowSessionRefs {
  plan_run_id: string;
  plan_device_run_id: string;
}

export interface WorkflowNode {
  node_id?: string;
  node_name?: string;
  node_type?: string;
  script_name?: string;
  script_version?: string;
  max_iterations?: number;
}

export interface WorkflowEdge {
  from_node_id?: string;
  to_node_id?: string;
  edge_type?: string;
}

export interface WorkflowDefinitionSnapshot {
  nodes?: WorkflowNode[];
  edges?: WorkflowEdge[];
}

export interface WorkflowScriptManifestItem {
  script_name?: string;
  script_version?: string;
}

export interface WorkflowSessionPayload {
  workflow_session_id?: string;
  workflow_def_id?: string;
  workflow_name?: string;
  entry_node_id?: string;
  plan_run_id?: string;
  plan_device_run_id?: string;
  script_manifest?: WorkflowScriptManifestItem[];
  definition_snapshot?: WorkflowDefinitionSnapshot;
}

export interface WorkflowSessionResult {
  plan_run_id: string;
  plan_device_run_id: string;
  workflow_node_id: string;
  status: string;
  result_code: string;
  result_message: string;
  extra?: Record<string, unknown>;
}

export interface WebSocketEnvelope<TPayload> {
  type: string;
  request_id: string;
  device_id: string;
  timestamp: number;
  payload: TPayload;
}

export interface AckPayload {
  message_type?: string;
  status?: string;
}

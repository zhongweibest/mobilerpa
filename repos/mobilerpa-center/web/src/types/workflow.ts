export interface WorkflowNodeRecord {
  workflow_def_id: string;
  node_id: string;
  node_type: string;
  node_name: string;
  script_name: string;
  script_version: string;
  max_iterations: number;
  position: number;
  created_at: string;
  updated_at: string;
}

export interface WorkflowEdgeRecord {
  workflow_def_id: string;
  from_node_id: string;
  to_node_id: string;
  edge_type: string;
  created_at: string;
}

export interface WorkflowDefinitionRecord {
  workflow_def_id: string;
  workflow_name: string;
  description: string;
  status: string;
  nodes: WorkflowNodeRecord[];
  edges: WorkflowEdgeRecord[];
  created_at: string;
  updated_at: string;
}

export interface WorkflowRunRecord {
  workflow_run_id: string;
  workflow_instance_id: string;
  workflow_def_id: string;
  device_id: string;
  status: string;
  current_node_id: string;
  current_task_id: string;
  started_at: string;
  finished_at: string;
  last_error: string;
  created_at: string;
  updated_at: string;
}

export interface WorkflowInstanceRecord {
  workflow_instance_id: string;
  workflow_def_id: string;
  workflow_name: string;
  status: string;
  started_at: string;
  finished_at: string;
  created_at: string;
  updated_at: string;
  device_runs: WorkflowRunRecord[];
}

export interface WorkflowRunSummary {
  total: number;
  pending: number;
  running: number;
  success: number;
  failed: number;
  stopped: number;
}

export interface WorkflowEventRecord {
  id: number;
  workflow_instance_id: string;
  workflow_run_id: string;
  workflow_def_id: string;
  device_id: string;
  node_id: string;
  event_type: string;
  message: string;
  extra: Record<string, unknown>;
  created_at: string;
}

export interface CreateWorkflowRequest {
  workflow_name: string;
  description: string;
  status: string;
  nodes: Array<{
    node_id: string;
    node_type: string;
    node_name: string;
    script_name?: string;
    script_version?: string;
    max_iterations?: number;
  }>;
  edges: Array<{
    from_node_id: string;
    to_node_id: string;
    edge_type: string;
  }>;
}

export interface WorkflowStartRequest {
  device_ids: string[];
}

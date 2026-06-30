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

export interface UpdateWorkflowRequest extends CreateWorkflowRequest {}

export interface WorkflowStartRequest {
  device_ids: string[];
}

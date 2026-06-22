export interface TaskRecord {
  task_id: string;
  device_id: string;
  workflow_run_id: string;
  workflow_node_id: string;
  task_source_type: string;
  script_name: string;
  script_version: string;
  params: Record<string, unknown>;
  status: string;
  priority: number;
  retry_count: number;
  current_step: string;
  result_code: string;
  result_message: string;
  scheduled_at: string;
  started_at: string;
  finished_at: string;
  created_at: string;
  updated_at: string;
}

export interface TaskEventRecord {
  id: number;
  topic: string;
  task_id: string;
  device_id: string;
  event_type: string;
  task_status: string;
  step_name: string;
  message: string;
  extra: Record<string, unknown>;
  created_at: string;
}

export interface CreateTaskRequest {
  device_id: string;
  script_name: string;
  script_version: string;
  priority: number;
  scheduled_at?: string;
  params: Record<string, unknown>;
}

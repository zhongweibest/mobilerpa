export interface PlanDefinitionRecord {
  plan_def_id: string;
  plan_name: string;
  description: string;
  target_type: string;
  target_script_name: string;
  target_script_version: string;
  target_workflow_def_id: string;
  schedule_type: string;
  daily_start_time: string;
  daily_deadline_time: string;
  status: string;
  rows: PlanRowBinding[];
  created_at: string;
  updated_at: string;
}

export interface PlanRowBinding {
  plan_definition_row_id?: string;
  plan_def_id?: string;
  zone_id: string;
  row_id: string;
  zone_name?: string;
  row_name?: string;
  created_at?: string;
  updated_at?: string;
}

export interface PlanRowTargetRecord {
  zone_id: string;
  row_id: string;
  zone_name?: string;
  row_name?: string;
  slot_count?: number;
  device_count?: number;
}

export interface PlanDeviceRunRecord {
  plan_device_run_id: string;
  plan_run_id: string;
  plan_def_id: string;
  zone_id: string;
  row_id: string;
  slot_id: string;
  device_id: string;
  target_type: string;
  target_ref_id: string;
  status: string;
  started_at: string;
  finished_at: string;
  last_error: string;
  created_at: string;
  updated_at: string;
}

export interface PlanRunRecord {
  plan_run_id: string;
  plan_def_id: string;
  plan_name: string;
  target_type: string;
  target_ref_id: string;
  run_date: string;
  status: string;
  started_at: string;
  finished_at: string;
  created_at: string;
  updated_at: string;
  device_runs: PlanDeviceRunRecord[];
}

export interface PlanEventRecord {
  id: number;
  plan_run_id: string;
  plan_def_id: string;
  device_id: string;
  event_type: string;
  message: string;
  extra: Record<string, unknown>;
  created_at: string;
}

export interface CreatePlanRequest {
  plan_name: string;
  description: string;
  target_type: string;
  target_script_name?: string;
  target_script_version?: string;
  target_workflow_def_id?: string;
  schedule_type: string;
  daily_start_time?: string;
  daily_deadline_time?: string;
  status: string;
  rows: PlanRowBinding[];
}

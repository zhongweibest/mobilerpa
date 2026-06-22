import { requestJSON } from "./http";
import type { WorkflowDefinitionRecord, WorkflowRunRecord, WorkflowEventRecord, WorkflowInstanceRecord, CreateWorkflowRequest } from "../types/workflow";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";

export function fetchWorkflows(query: PaginationQuery): Promise<PaginatedResult<WorkflowDefinitionRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<WorkflowDefinitionRecord>>(`/api/v1/workflows?${searchParams.toString()}`);
}

export function fetchAllWorkflowInstances(): Promise<WorkflowInstanceRecord[]> {
  return requestJSON<WorkflowInstanceRecord[]>("/api/v1/workflows?view=instances");
}

export function fetchWorkflowDetail(workflowDefID: string): Promise<WorkflowDefinitionRecord> {
  return requestJSON<WorkflowDefinitionRecord>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}`);
}

export function createWorkflow(payload: CreateWorkflowRequest): Promise<WorkflowDefinitionRecord> {
  return requestJSON<WorkflowDefinitionRecord>("/api/v1/workflows", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function fetchWorkflowRuns(workflowDefID: string): Promise<WorkflowRunRecord[]> {
  return requestJSON<WorkflowRunRecord[]>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}/runs`);
}

export function fetchWorkflowInstances(workflowDefID: string): Promise<WorkflowInstanceRecord[]> {
  return requestJSON<WorkflowInstanceRecord[]>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}/instances`);
}

export function fetchWorkflowEvents(workflowDefID: string, workflowRunID: string): Promise<WorkflowEventRecord[]> {
  const searchParams = new URLSearchParams({
    workflow_run_id: workflowRunID
  });
  return requestJSON<WorkflowEventRecord[]>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}/events?${searchParams.toString()}`);
}

export function startWorkflow(workflowDefID: string, deviceIDs: string[]): Promise<WorkflowInstanceRecord> {
  return requestJSON<WorkflowInstanceRecord>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}/start`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      device_ids: deviceIDs
    })
  });
}

export function addWorkflowDevices(workflowDefID: string, workflowInstanceID: string, deviceIDs: string[]): Promise<WorkflowInstanceRecord> {
  return requestJSON<WorkflowInstanceRecord>(
    `/api/v1/workflows/${encodeURIComponent(workflowDefID)}/instances/${encodeURIComponent(workflowInstanceID)}/devices`,
    {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      device_ids: deviceIDs
    })
    }
  );
}

export function stopWorkflow(workflowDefID: string, workflowInstanceID: string): Promise<WorkflowInstanceRecord> {
  return requestJSON<WorkflowInstanceRecord>(
    `/api/v1/workflows/${encodeURIComponent(workflowDefID)}/instances/${encodeURIComponent(workflowInstanceID)}/stop`,
    {
    method: "POST"
    }
  );
}

export function deleteWorkflowInstance(
  workflowDefID: string,
  workflowInstanceID: string
): Promise<{ workflow_def_id: string; workflow_instance_id: string; deleted: boolean }> {
  return requestJSON<{ workflow_def_id: string; workflow_instance_id: string; deleted: boolean }>(
    `/api/v1/workflows/${encodeURIComponent(workflowDefID)}/instances/${encodeURIComponent(workflowInstanceID)}`,
    {
      method: "DELETE"
    }
  );
}

export function stopWorkflowDevice(workflowDefID: string, workflowInstanceID: string, deviceID: string): Promise<WorkflowRunRecord> {
  return requestJSON<WorkflowRunRecord>(
    `/api/v1/workflows/${encodeURIComponent(workflowDefID)}/instances/${encodeURIComponent(workflowInstanceID)}/devices/${encodeURIComponent(deviceID)}/stop`,
    {
    method: "POST"
    }
  );
}

export function deleteWorkflow(workflowDefID: string): Promise<{ workflow_def_id: string; deleted: boolean }> {
  return requestJSON<{ workflow_def_id: string; deleted: boolean }>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}`, {
    method: "DELETE"
  });
}

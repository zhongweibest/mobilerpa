import { requestJSON } from "./http";
import type { WorkflowDefinitionRecord, CreateWorkflowRequest, UpdateWorkflowRequest } from "../types/workflow";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";

export function fetchWorkflows(query: PaginationQuery): Promise<PaginatedResult<WorkflowDefinitionRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<WorkflowDefinitionRecord>>(`/api/v1/workflows?${searchParams.toString()}`);
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

export function updateWorkflow(workflowDefID: string, payload: UpdateWorkflowRequest): Promise<WorkflowDefinitionRecord> {
  return requestJSON<WorkflowDefinitionRecord>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function deleteWorkflow(workflowDefID: string): Promise<{ workflow_def_id: string; deleted: boolean }> {
  return requestJSON<{ workflow_def_id: string; deleted: boolean }>(`/api/v1/workflows/${encodeURIComponent(workflowDefID)}`, {
    method: "DELETE"
  });
}

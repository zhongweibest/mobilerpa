import { requestJSON } from "./http";
import type { CreateTaskRequest, TaskEventRecord, TaskRecord } from "../types/task";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";

export function fetchTasks(query: PaginationQuery): Promise<PaginatedResult<TaskRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size),
    source_type: "manual"
  });
  return requestJSON<PaginatedResult<TaskRecord>>(`/api/v1/tasks?${searchParams.toString()}`);
}

export function createTask(payload: CreateTaskRequest): Promise<TaskRecord> {
  return requestJSON<TaskRecord>("/api/v1/tasks", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function assignTask(taskID: string): Promise<TaskRecord> {
  return requestJSON<TaskRecord>("/api/v1/tasks", {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      task_id: taskID,
      action: "assign"
    })
  });
}

export function fetchTaskEvents(taskID: string): Promise<TaskEventRecord[]> {
  return requestJSON<TaskEventRecord[]>(`/api/v1/tasks/${encodeURIComponent(taskID)}/events`);
}

export function terminateTask(taskID: string): Promise<TaskRecord> {
  return requestJSON<TaskRecord>(`/api/v1/tasks/${encodeURIComponent(taskID)}/terminate`, {
    method: "POST"
  });
}

export function deleteTask(taskID: string): Promise<{ task_id: string; deleted: boolean }> {
  return requestJSON<{ task_id: string; deleted: boolean }>(`/api/v1/tasks/${encodeURIComponent(taskID)}`, {
    method: "DELETE"
  });
}

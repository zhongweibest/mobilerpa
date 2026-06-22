import type { PaginatedResult, PaginationQuery } from "../types/pagination";
import type { CreatePlanRequest, PlanDefinitionRecord, PlanEventRecord, PlanRunRecord, UpdatePlanDevicesRequest } from "../types/plan";
import { requestJSON } from "./http";

export function fetchPlans(query: PaginationQuery): Promise<PaginatedResult<PlanDefinitionRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<PlanDefinitionRecord>>(`/api/v1/plans?${searchParams.toString()}`);
}

export function fetchPlanRuns(query: PaginationQuery): Promise<PaginatedResult<PlanRunRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size),
    view: "runs"
  });
  return requestJSON<PaginatedResult<PlanRunRecord>>(`/api/v1/plans?${searchParams.toString()}`);
}

export function createPlan(payload: CreatePlanRequest): Promise<PlanDefinitionRecord> {
  return requestJSON<PlanDefinitionRecord>("/api/v1/plans", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function deletePlan(planDefID: string): Promise<{ plan_def_id: string; deleted: boolean }> {
  return requestJSON<{ plan_def_id: string; deleted: boolean }>(`/api/v1/plans/${encodeURIComponent(planDefID)}`, {
    method: "DELETE"
  });
}

export function updatePlanDevices(planDefID: string, payload: UpdatePlanDevicesRequest): Promise<PlanDefinitionRecord> {
  return requestJSON<PlanDefinitionRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/devices`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function startPlan(planDefID: string, deviceIDs: string[]): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/start`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      device_ids: deviceIDs
    })
  });
}

export function fetchPlanRun(planDefID: string, planRunID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}`);
}

export function fetchPlanEvents(planDefID: string, planRunID: string): Promise<PlanEventRecord[]> {
  return requestJSON<PlanEventRecord[]>(`/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/events`);
}

export function stopPlanRun(planDefID: string, planRunID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/stop`, {
    method: "POST"
  });
}

export function deletePlanRun(planDefID: string, planRunID: string): Promise<{ plan_def_id: string; plan_run_id: string; deleted: boolean }> {
  return requestJSON<{ plan_def_id: string; plan_run_id: string; deleted: boolean }>(
    `/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}`,
    {
      method: "DELETE"
    }
  );
}

export function addPlanDevices(planDefID: string, planRunID: string, deviceIDs: string[]): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/devices`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      device_ids: deviceIDs
    })
  });
}

export function removePlanDevice(planDefID: string, planRunID: string, deviceID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(
    `/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/devices/${encodeURIComponent(deviceID)}`,
    {
      method: "DELETE"
    }
  );
}

import type { PaginatedResult, PaginationQuery } from "../types/pagination";
import type { CreatePlanRequest, PlanDefinitionRecord, PlanEventRecord, PlanRunRecord, PlanRowBinding } from "../types/plan";
import { requestJSON } from "./http";

export function fetchPlans(query: PaginationQuery): Promise<PaginatedResult<PlanDefinitionRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  if (query.target_type?.trim()) {
    searchParams.set("target_type", query.target_type.trim());
  }
  if (query.schedule_type?.trim()) {
    searchParams.set("schedule_type", query.schedule_type.trim());
  }
  return requestJSON<PaginatedResult<PlanDefinitionRecord>>(`/api/v1/plans?${searchParams.toString()}`);
}

export function fetchPlanRuns(query: PaginationQuery): Promise<PaginatedResult<PlanRunRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size),
    view: "runs"
  });
  if (query.plan_def_id?.trim()) {
    searchParams.set("plan_def_id", query.plan_def_id.trim());
  }
  if (query.plan_name?.trim()) {
    searchParams.set("plan_name", query.plan_name.trim());
  }
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

export function updatePlanRows(planDefID: string, payload: { rows: PlanRowBinding[] }): Promise<PlanDefinitionRecord> {
  const requestInit: RequestInit = {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  };
  return requestJSON<PlanDefinitionRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/rows`, requestInit).catch((error: Error) => {
    if (error.message.includes("plan_resource_not_found") || error.message.includes("request_failed:404")) {
      throw new Error("计划任务接口尚未更新，请重启中心服务后重试");
    }
    throw error;
  });
}

export function startPlan(planDefID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/start`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({})
  });
}

export function updatePlanStatus(planDefID: string, status: string): Promise<PlanDefinitionRecord> {
  return requestJSON<PlanDefinitionRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/status`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ status })
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

export function stopPlanDeviceRun(planDefID: string, planRunID: string, planDeviceRunID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(
    `/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/device-runs/${encodeURIComponent(planDeviceRunID)}/stop`,
    {
      method: "POST"
    }
  );
}

export function deletePlanRun(planDefID: string, planRunID: string): Promise<{ plan_def_id: string; plan_run_id: string; deleted: boolean }> {
  return requestJSON<{ plan_def_id: string; plan_run_id: string; deleted: boolean }>(
    `/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}`,
    {
      method: "DELETE"
    }
  );
}

export function addPlanRows(planDefID: string, planRunID: string, rows: PlanRowBinding[]): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(`/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/rows`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ rows })
  });
}

export function removePlanRow(planDefID: string, planRunID: string, zoneID: string, rowID: string): Promise<PlanRunRecord> {
  return requestJSON<PlanRunRecord>(
    `/api/v1/plans/${encodeURIComponent(planDefID)}/runs/${encodeURIComponent(planRunID)}/rows/${encodeURIComponent(zoneID)}/${encodeURIComponent(rowID)}`,
    {
      method: "DELETE"
    }
  );
}

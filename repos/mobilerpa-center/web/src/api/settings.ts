import { requestJSON } from "./http";

export interface DiscoverySettings {
  center_base_url: string;
}

export interface PlanDailyRetrySettings {
  plan_daily_retry_enabled: boolean;
  plan_daily_retry_interval_seconds: number;
  plan_daily_retry_stop_before_deadline_minutes: number;
}

export function fetchDiscoverySettings(): Promise<DiscoverySettings> {
  return requestJSON<DiscoverySettings>("/api/v1/settings/discovery");
}

export function saveDiscoverySettings(payload: DiscoverySettings): Promise<DiscoverySettings> {
  return requestJSON<DiscoverySettings>("/api/v1/settings/discovery", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function fetchPlanDailyRetrySettings(): Promise<PlanDailyRetrySettings> {
  return requestJSON<PlanDailyRetrySettings>("/api/v1/settings/plans/daily-retry");
}

export function savePlanDailyRetrySettings(payload: PlanDailyRetrySettings): Promise<PlanDailyRetrySettings> {
  return requestJSON<PlanDailyRetrySettings>("/api/v1/settings/plans/daily-retry", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

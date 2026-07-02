import { requestJSON } from "./http";
import type {
  AgentActionRequest,
  AgentActionResult,
  AgentDeploymentRequest,
  AgentDeploymentResult,
  DiscoveredDevice,
  PairDeviceRequest,
  PairDeviceResult,
  SoftwareInstallJob,
  SoftwareInstallRequest
} from "../types/discovery";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";

export function fetchDiscoveredDevices(query: PaginationQuery): Promise<PaginatedResult<DiscoveredDevice>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<DiscoveredDevice>>(`/api/v1/discovery/devices?${searchParams.toString()}`);
}

export function deployAgents(payload: AgentDeploymentRequest): Promise<AgentDeploymentResult[]> {
  return requestJSON<AgentDeploymentResult[]>("/api/v1/discovery/agent-deployments", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function controlAgent(payload: AgentActionRequest): Promise<AgentActionResult> {
  return requestJSON<AgentActionResult>("/api/v1/discovery/agent-actions", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function pairDevice(payload: PairDeviceRequest): Promise<PairDeviceResult> {
  return requestJSON<PairDeviceResult>("/api/v1/discovery/pair", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function startSoftwareInstall(payload: SoftwareInstallRequest): Promise<SoftwareInstallJob> {
  return requestJSON<SoftwareInstallJob>("/api/v1/discovery/software-installs", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export function fetchSoftwareInstallJob(jobID: string): Promise<SoftwareInstallJob> {
  return requestJSON<SoftwareInstallJob>(`/api/v1/discovery/software-installs/${encodeURIComponent(jobID)}`);
}

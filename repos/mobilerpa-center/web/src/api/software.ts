import type { PaginatedResult, PaginationQuery } from "../types/pagination";
import type { CreateSoftwareRequest, SoftwarePackageRecord, UpdateSoftwareRequest } from "../types/software";
import { getApiBaseUrl, requestJSON } from "./http";

export function fetchSoftware(query: PaginationQuery): Promise<PaginatedResult<SoftwarePackageRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<SoftwarePackageRecord>>(`/api/v1/software?${searchParams.toString()}`);
}

export function fetchAllSoftware(): Promise<SoftwarePackageRecord[]> {
  return requestJSON<SoftwarePackageRecord[]>("/api/v1/software/all");
}

export async function createSoftware(payload: CreateSoftwareRequest): Promise<SoftwarePackageRecord> {
  const formData = new FormData();
  formData.append("software_name", payload.software_name);
  formData.append("description", payload.description);
  formData.append("file", payload.file);
  return requestJSON<SoftwarePackageRecord>("/api/v1/software", {
    method: "POST",
    body: formData
  });
}

export async function updateSoftware(payload: UpdateSoftwareRequest): Promise<SoftwarePackageRecord> {
  const formData = new FormData();
  formData.append("software_name", payload.software_name);
  formData.append("description", payload.description);
  if (payload.file) {
    formData.append("file", payload.file);
  }
  return requestJSON<SoftwarePackageRecord>(`/api/v1/software/${encodeURIComponent(payload.software_id)}`, {
    method: "PUT",
    body: formData
  });
}

export async function deleteSoftware(softwareID: string): Promise<void> {
  await requestJSON(`/api/v1/software/${encodeURIComponent(softwareID)}`, {
    method: "DELETE"
  });
}

export function buildSoftwareDownloadUrl(softwareID: string): string {
  return `${getApiBaseUrl()}/api/v1/software/${encodeURIComponent(softwareID)}/download`;
}

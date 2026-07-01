import { requestJSON } from "./http";
import type { CreateScriptNameRequest, DeployAllScriptsRequest, DeployScriptRequest, ScriptManifestRecord, ScriptNameRecord, ScriptRecord, UploadScriptRequest } from "../types/script";
import type { PaginatedResult, PaginationQuery } from "../types/pagination";

export async function fetchScripts(query: PaginationQuery): Promise<PaginatedResult<ScriptRecord>> {
  const searchParams = new URLSearchParams({
    page: String(query.page),
    page_size: String(query.page_size)
  });
  return requestJSON<PaginatedResult<ScriptRecord>>(`/api/v1/scripts?${searchParams.toString()}`);
}

export async function fetchScriptNames(): Promise<ScriptNameRecord[]> {
  try {
    return await requestJSON<ScriptNameRecord[]>("/api/v1/script-names");
  } catch (error) {
    const message = error instanceof Error ? error.message : "";
    if (!message.includes("request_failed:404")) {
      throw error;
    }
    const fallback = await fetchScripts({ page: 1, page_size: 100 });
    return fallback.items.map((item) => ({
      script_name: item.script_name,
      created_at: "",
      updated_at: ""
    }));
  }
}

export async function fetchScriptVersion(scriptName: string, scriptVersion: string): Promise<ScriptManifestRecord> {
  return requestJSON<ScriptManifestRecord>(`/api/v1/scripts/${encodeURIComponent(scriptName)}/versions/${encodeURIComponent(scriptVersion)}`);
}

export async function uploadScript(payload: UploadScriptRequest): Promise<void> {
  const formData = new FormData();
  formData.append("script_name", payload.script_name);
  formData.append("script_version", payload.script_version);
  formData.append("source_type", payload.source_type);
  formData.append("force", payload.force ? "true" : "false");
  formData.append("file", payload.file);

  await requestJSON("/api/v1/scripts/upload", {
    method: "POST",
    body: formData
  });
}

export async function createScriptName(payload: CreateScriptNameRequest): Promise<ScriptNameRecord> {
  return requestJSON<ScriptNameRecord>("/api/v1/script-names", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export async function deployScript(payload: DeployScriptRequest): Promise<void> {
  await requestJSON("/api/v1/scripts/deploy", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export async function deployScriptToAll(payload: DeployAllScriptsRequest): Promise<void> {
  await requestJSON("/api/v1/scripts/deploy-all", {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(payload)
  });
}

export async function deleteScriptVersion(scriptName: string, scriptVersion: string): Promise<void> {
  await requestJSON(`/api/v1/scripts/${encodeURIComponent(scriptName)}/versions/${encodeURIComponent(scriptVersion)}`, {
    method: "DELETE"
  });
}

export async function deleteScript(scriptName: string): Promise<void> {
  await requestJSON(`/api/v1/scripts/${encodeURIComponent(scriptName)}`, {
    method: "DELETE"
  });
}

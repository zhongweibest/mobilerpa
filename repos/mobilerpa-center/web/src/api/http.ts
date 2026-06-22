export interface ApiEnvelope<T> {
  status: string;
  data: T;
}

const DEFAULT_PRODUCTION_BASE_URL = "http://127.0.0.1:28080";

export function getApiBaseUrl(): string {
  const envUrl = import.meta.env.VITE_API_BASE_URL;
  if (typeof envUrl === "string" && envUrl.trim() !== "") {
    return envUrl.trim().replace(/\/+$/, "");
  }
  if (import.meta.env.DEV) {
    return "";
  }
  return DEFAULT_PRODUCTION_BASE_URL;
}

export async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${getApiBaseUrl()}${path}`, init);
  if (!response.ok) {
    let errorCode = `request_failed:${response.status}`;
    let errorPayload: Record<string, unknown> | null = null;
    try {
      const payload = (await response.json()) as { error?: string; status?: string };
      errorPayload = payload as unknown as Record<string, unknown>;
      if (typeof payload.error === "string" && payload.error.trim() !== "") {
        errorCode = payload.error.trim();
      }
    } catch (_error) {
      // 忽略非 JSON 错误响应，保留默认错误码。
    }
    const err = new Error(errorCode) as Error & { details?: Record<string, unknown> | null };
    err.details = errorPayload;
    throw err;
  }

  const payload = (await response.json()) as ApiEnvelope<T>;
  return payload.data;
}

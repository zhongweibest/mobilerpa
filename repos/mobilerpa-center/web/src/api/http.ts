export interface ApiEnvelope<T> {
  status: string;
  data: T;
}

const DEFAULT_PRODUCTION_BASE_URL = "http://127.0.0.1:28080";

function buildErrorMessage(errorCode: string, errorPayload: Record<string, unknown> | null): string {
  const reason = errorPayload && typeof errorPayload.reason === "string" ? errorPayload.reason.trim() : "";
  if (reason !== "") {
    return `${errorCode}：${reason}`;
  }
  if (errorCode === "plan_device_busy" && errorPayload && Array.isArray(errorPayload.busy_devices) && errorPayload.busy_devices.length > 0) {
    const firstDetail = errorPayload.busy_devices[0];
    if (firstDetail && typeof firstDetail === "object") {
      const detailRecord = firstDetail as Record<string, unknown>;
      const deviceID = typeof detailRecord.device_id === "string" ? detailRecord.device_id.trim() : "";
      const message = typeof detailRecord.message === "string" ? detailRecord.message.trim() : "";
      const occupancyType = typeof detailRecord.occupancy_type === "string" ? detailRecord.occupancy_type.trim() : "";
      const taskStatus = typeof detailRecord.task_status === "string" ? detailRecord.task_status.trim() : "";
      const parts = [deviceID ? `设备 ${deviceID}` : "", message, occupancyType ? `占用类型：${occupancyType}` : "", taskStatus ? `状态：${taskStatus}` : ""]
        .filter((item) => item !== "")
        .join("，");
      if (parts !== "") {
        return `${errorCode}：${parts}`;
      }
    }
  }
  return errorCode;
}

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
    const err = new Error(buildErrorMessage(errorCode, errorPayload)) as Error & { details?: Record<string, unknown> | null };
    err.details = errorPayload;
    throw err;
  }

  const payload = (await response.json()) as ApiEnvelope<T>;
  return payload.data;
}

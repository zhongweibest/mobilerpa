import { requestJSON } from "./http";

export interface DiscoverySettings {
  center_base_url: string;
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

import * as runtime from "./runtime";
import type { RegisterPayload, RegisterResponse } from "../types/protocol";

type JSONValue = Record<string, unknown>;
type NodeResponse = {
  statusCode?: number;
  on: (eventName: string, listener: (...args: any[]) => void) => void;
};

function nodeRequire(moduleName: string): any {
  return require(moduleName);
}

function parseJSONResponse(statusCode: number, text: string): JSONValue {
  let body: JSONValue = {};
  try {
    body = text ? (JSON.parse(text) as JSONValue) : {};
  } catch (_error) {
    throw new Error("中心服务返回了非 JSON 响应：" + text);
  }

  if (statusCode < 200 || statusCode >= 300) {
    throw new Error("中心服务请求失败，状态码=" + statusCode + "，响应=" + text);
  }

  return body;
}

function formatErrorMessage(error: unknown): string {
  if (!error) {
    return "unknown_error";
  }
  if (typeof error === "object" && error !== null) {
    const maybeError = error as { stack?: unknown; message?: unknown };
    if (maybeError.stack) {
      return String(maybeError.stack);
    }
    if (maybeError.message) {
      return String(maybeError.message);
    }
  }
  return String(error);
}

function trimBaseURL(baseURL: string): string {
  return String(baseURL || "").replace(/\/+$/, "");
}

function createNodeClient(url: URL): any {
  return url.protocol === "https:" ? nodeRequire("https") : nodeRequire("http");
}

function nodeGetJSON(url: string): Promise<JSONValue> {
  return new Promise(function resolveJSON(resolve, reject) {
    const target = new URL(url);
    const client = createNodeClient(target);
    const request = client.request({
      hostname: target.hostname,
      port: target.port || (target.protocol === "https:" ? 443 : 80),
      path: target.pathname + target.search,
      method: "GET"
    }, function onResponse(response: NodeResponse) {
      const chunks: Buffer[] = [];
      response.on("data", function onData(chunk: Buffer) {
        chunks.push(chunk);
      });
      response.on("end", function onEnd() {
        try {
          resolve(parseJSONResponse(response.statusCode || 200, Buffer.concat(chunks).toString("utf8")));
        } catch (error) {
          reject(error);
        }
      });
    });
    request.on("error", reject);
    request.end();
  });
}

function nodeDownloadText(url: string): Promise<string> {
  return new Promise(function resolveText(resolve, reject) {
    const target = new URL(url);
    const client = createNodeClient(target);
    const request = client.request({
      hostname: target.hostname,
      port: target.port || (target.protocol === "https:" ? 443 : 80),
      path: target.pathname + target.search,
      method: "GET"
    }, function onResponse(response: NodeResponse) {
      const chunks: Buffer[] = [];
      response.on("data", function onData(chunk: Buffer) {
        chunks.push(chunk);
      });
      response.on("end", function onEnd() {
        if ((response.statusCode || 200) < 200 || (response.statusCode || 200) >= 300) {
          reject(new Error("download_failed:" + response.statusCode));
          return;
        }
        resolve(Buffer.concat(chunks).toString("utf8"));
      });
    });
    request.on("error", reject);
    request.end();
  });
}

function nodeDownloadBuffer(url: string): Promise<Buffer> {
  return new Promise(function resolveBuffer(resolve, reject) {
    const target = new URL(url);
    const client = createNodeClient(target);
    const request = client.request({
      hostname: target.hostname,
      port: target.port || (target.protocol === "https:" ? 443 : 80),
      path: target.pathname + target.search,
      method: "GET"
    }, function onResponse(response: NodeResponse) {
      const chunks: Buffer[] = [];
      response.on("data", function onData(chunk: Buffer) {
        chunks.push(chunk);
      });
      response.on("end", function onEnd() {
        if ((response.statusCode || 200) < 200 || (response.statusCode || 200) >= 300) {
          reject(new Error("download_failed:" + response.statusCode));
          return;
        }
        resolve(Buffer.concat(chunks));
      });
    });
    request.on("error", reject);
    request.end();
  });
}

function autoJsGetJSON(url: string): JSONValue {
  try {
    const response = http.get(url);
    const statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
    const body = response.body && typeof response.body.string === "function"
      ? response.body.string()
      : String(response.body || "");
    return parseJSONResponse(statusCode, body);
  } catch (error) {
    throw new Error("AutoJs6 请求中心服务失败，url=" + url + "，error=" + formatErrorMessage(error));
  }
}

function autoJsDownloadText(url: string): string {
  try {
    const response = http.get(url);
    const statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
    const body = response.body && typeof response.body.string === "function"
      ? response.body.string()
      : String(response.body || "");
    if (statusCode < 200 || statusCode >= 300) {
      throw new Error("download_failed:" + statusCode + ":" + body);
    }
    return body;
  } catch (error) {
    throw new Error("AutoJs6 下载中心文件失败，url=" + url + "，error=" + formatErrorMessage(error));
  }
}

function autoJsDownloadBytes(url: string): unknown {
  try {
    const response = http.get(url);
    const statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
    if (statusCode < 200 || statusCode >= 300) {
      const errorBody = response.body && typeof response.body.string === "function"
        ? response.body.string()
        : String(response.body || "");
      throw new Error("download_failed:" + statusCode + ":" + errorBody);
    }

    if (response.body && typeof response.body.bytes === "function") {
      return response.body.bytes();
    }

    if (response.body && typeof response.body.string === "function") {
      return response.body.string();
    }

    return String(response.body || "");
  } catch (error) {
    throw new Error("AutoJs6 下载中心文件失败，url=" + url + "，error=" + formatErrorMessage(error));
  }
}

function autoJsPostJSON(url: string, payload: RegisterPayload): JSONValue {
  try {
    const response = http.postJson(url, payload);
    const statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
    const body = response.body && typeof response.body.string === "function"
      ? response.body.string()
      : String(response.body || "");
    return parseJSONResponse(statusCode, body);
  } catch (error) {
    throw new Error("AutoJs6 请求中心服务失败，url=" + url + "，error=" + formatErrorMessage(error));
  }
}

function nodePostJSON(url: string, payload: RegisterPayload): Promise<JSONValue> {
  return new Promise(function resolveJSON(resolve, reject) {
    const target = new URL(url);
    const data = JSON.stringify(payload);
    const client = createNodeClient(target);
    const request = client.request({
      hostname: target.hostname,
      port: target.port || (target.protocol === "https:" ? 443 : 80),
      path: target.pathname + target.search,
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Content-Length": Buffer.byteLength(data)
      }
    }, function onResponse(response: NodeResponse) {
      const chunks: Buffer[] = [];
      response.on("data", function onData(chunk: Buffer) {
        chunks.push(chunk);
      });
      response.on("end", function onEnd() {
        try {
          resolve(parseJSONResponse(response.statusCode || 200, Buffer.concat(chunks).toString("utf8")));
        } catch (error) {
          reject(error);
        }
      });
    });

    request.on("error", reject);
    request.write(data);
    request.end();
  });
}

function registerDevice(centerBaseURL: string, payload: RegisterPayload): RegisterResponse | Promise<RegisterResponse> {
  const url = trimBaseURL(centerBaseURL) + "/api/v1/device/register";
  if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
    return autoJsPostJSON(url, payload) as RegisterResponse;
  }
  return nodePostJSON(url, payload) as Promise<RegisterResponse>;
}

function getScriptManifest(centerBaseURL: string, scriptName: string, scriptVersion: string): JSONValue | Promise<JSONValue> {
  const url = trimBaseURL(centerBaseURL)
    + "/api/v1/script/manifest?script_name=" + encodeURIComponent(scriptName)
    + "&script_version=" + encodeURIComponent(scriptVersion);
  if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
    return autoJsGetJSON(url);
  }
  return nodeGetJSON(url);
}

function downloadScript(centerBaseURL: string, scriptName: string, scriptVersion: string): string | Promise<string> {
  return downloadScriptFile(centerBaseURL, scriptName, scriptVersion, "index.js");
}

function downloadScriptFile(
  centerBaseURL: string,
  scriptName: string,
  scriptVersion: string,
  relativePath: string
): string | Promise<string> {
  const filePath = String(relativePath || "index.js");
  const url = trimBaseURL(centerBaseURL)
    + "/api/v1/script/download?script_name=" + encodeURIComponent(scriptName)
    + "&script_version=" + encodeURIComponent(scriptVersion)
    + "&relative_path=" + encodeURIComponent(filePath);
  if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
    return autoJsDownloadText(url);
  }
  return nodeDownloadText(url);
}

function downloadScriptFileBytes(
  centerBaseURL: string,
  scriptName: string,
  scriptVersion: string,
  relativePath: string
): unknown | Promise<Buffer> {
  const filePath = String(relativePath || "index.js");
  const url = trimBaseURL(centerBaseURL)
    + "/api/v1/script/download?script_name=" + encodeURIComponent(scriptName)
    + "&script_version=" + encodeURIComponent(scriptVersion)
    + "&relative_path=" + encodeURIComponent(filePath);
  if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
    return autoJsDownloadBytes(url);
  }
  return nodeDownloadBuffer(url);
}

export {
  registerDevice,
  getScriptManifest,
  downloadScript,
  downloadScriptFile,
  downloadScriptFileBytes
};

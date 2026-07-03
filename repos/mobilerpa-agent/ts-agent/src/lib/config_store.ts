import * as runtime from "./runtime";
import type { AgentConfig, BootstrapConfig, WebSocketConfig } from "../types/agent";

interface ConfigStoreOptions {
  configPath?: string;
  bootstrapPath?: string;
  stopSignalPath?: string;
}

interface ConfigStore {
  configPath: string;
  bootstrapPath: string;
  stopSignalPath: string;
  exists(): boolean;
  bootstrapExists(): boolean;
  loadBootstrap(): BootstrapConfig;
  load(): AgentConfig;
  save(config: AgentConfig): void;
}

function isNodeRuntime(): boolean {
  return runtime.isNodeRuntime();
}

function nodeRequire(moduleName: string): any {
  return require(moduleName);
}

function joinPath(...parts: Array<string | undefined>): string {
  const filtered = parts.filter(Boolean) as string[];
  if (isNodeRuntime()) {
    const path = nodeRequire("path");
    return path.join(...filtered);
  }
  return normalizePath(filtered.join("/"));
}

function resolveAgentRootPath(): string {
  if (isNodeRuntime()) {
    const path = nodeRequire("path");
    return path.resolve(typeof __dirname !== "undefined" ? joinPath(__dirname, "..", "..") : ".");
  }

  if (typeof files !== "undefined" && typeof files.path === "function") {
    return normalizePath(String(files.path(".") || "."));
  }

  if (typeof __dirname !== "undefined") {
    return normalizePath(joinPath(__dirname, "..", ".."));
  }

  return ".";
}

function normalizePath(input: string): string {
  const raw = String(input || "").replace(/\\/g, "/");
  const absolute = raw.charAt(0) === "/";
  const segments = raw.split("/");
  const stack: string[] = [];

  for (let index = 0; index < segments.length; index += 1) {
    if (!segments[index] || segments[index] === ".") {
      continue;
    }
    if (segments[index] === "..") {
      if (stack.length > 0 && stack[stack.length - 1] !== "..") {
        stack.pop();
      } else if (!absolute) {
        stack.push("..");
      }
      continue;
    }
    stack.push(segments[index]);
  }

  let output = stack.join("/");
  if (absolute) {
    output = "/" + output;
  }
  return output || (absolute ? "/" : ".");
}

function dirname(filePath: string): string {
  if (isNodeRuntime()) {
    const path = nodeRequire("path");
    return path.dirname(filePath);
  }
  const normalized = String(filePath).replace(/\\/g, "/");
  const index = normalized.lastIndexOf("/");
  return index >= 0 ? normalized.slice(0, index) : ".";
}

function defaultConfigPath(): string {
  const root = resolveAgentRootPath();
  return joinPath(root, "runtime", "config.json");
}

function defaultBootstrapPath(): string {
  const root = resolveAgentRootPath();
  return joinPath(root, "runtime", "bootstrap.json");
}

function defaultStopSignalPath(): string {
  const root = resolveAgentRootPath();
  return joinPath(root, "runtime", "stop.signal");
}

function defaultRuntimeLockPath(): string {
  return joinPath(resolveRuntimeStateRootPath(), "agent.lock.json");
}

function defaultHeartbeatLeasePath(): string {
  return joinPath(resolveRuntimeStateRootPath(), "heartbeat.lease.json");
}

function resolveRuntimeStateRootPath(): string {
  if (isNodeRuntime()) {
    return joinPath(resolveAgentRootPath(), "runtime");
  }

  const agentRoot = resolveAgentRootPath();
  const parentRoot = dirname(agentRoot);
  return joinPath(parentRoot, ".mobilerpa-agent-runtime");
}

function exists(filePath: string): boolean {
  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    return fs.existsSync(filePath);
  }
  return typeof files !== "undefined" && files.exists(filePath);
}

function readText(filePath: string): string {
  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    return fs.readFileSync(filePath, "utf8");
  }
  return files.read(filePath);
}

function writeText(filePath: string, content: string): void {
  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    fs.mkdirSync(dirname(filePath), { recursive: true });
    fs.writeFileSync(filePath, content, "utf8");
    return;
  }

  if (typeof files !== "undefined") {
    files.createWithDirs(filePath);
    files.write(filePath, content);
  }
}

function createEmptyConfig(): AgentConfig {
  const now = runtime.nowISOString();
  return {
    center_base_url: "http://127.0.0.1:18080",
    agent_uuid: "",
    device_id: "",
    device_link_sn: "",
    device: {},
    websocket: {
      enabled: true,
      heartbeat_interval_ms: 30000,
      heartbeat_scheduler: "executor",
      reconnect_enabled: true,
      reconnect_initial_delay_ms: 3000,
      reconnect_max_delay_ms: 60000,
      reconnect_backoff_multiplier: 2
    },
    keep_alive: {
      wake_screen_before_task: true,
      wake_screen_cooldown_seconds: 30,
      ws_watchdog_interval_seconds: 20,
      ws_silence_timeout_seconds: 120
    },
    last_register: {},
    created_at: now,
    updated_at: now
  };
}

function normalizeWebSocketConfig(input?: WebSocketConfig, fallback?: WebSocketConfig): WebSocketConfig {
  const source = input || {};
  const base = fallback || {};
  return {
    enabled: source.enabled === false ? false : base.enabled !== false,
    heartbeat_interval_ms: source.heartbeat_interval_ms || base.heartbeat_interval_ms || 30000,
    heartbeat_scheduler: source.heartbeat_scheduler || base.heartbeat_scheduler || "executor",
    reconnect_enabled: source.reconnect_enabled === false ? false : base.reconnect_enabled !== false,
    reconnect_initial_delay_ms: source.reconnect_initial_delay_ms || base.reconnect_initial_delay_ms || 3000,
    reconnect_max_delay_ms: source.reconnect_max_delay_ms || base.reconnect_max_delay_ms || 60000,
    reconnect_backoff_multiplier: source.reconnect_backoff_multiplier || base.reconnect_backoff_multiplier || 2
  };
}

function normalizeConfig(raw?: Partial<AgentConfig>): AgentConfig {
  const base = createEmptyConfig();
  const input = raw || {};
  return {
    center_base_url: input.center_base_url || base.center_base_url,
    agent_uuid: input.agent_uuid || "",
    device_id: input.device_id || "",
    device_link_sn: input.device_link_sn || "",
    device: input.device || {},
    websocket: normalizeWebSocketConfig(input.websocket, base.websocket),
    keep_alive: {
      wake_screen_before_task: input.keep_alive && input.keep_alive.wake_screen_before_task === false ? false : base.keep_alive && base.keep_alive.wake_screen_before_task !== false,
      wake_screen_cooldown_seconds: Number((input.keep_alive && input.keep_alive.wake_screen_cooldown_seconds) || (base.keep_alive && base.keep_alive.wake_screen_cooldown_seconds) || 30),
      ws_watchdog_interval_seconds: Number((input.keep_alive && input.keep_alive.ws_watchdog_interval_seconds) || (base.keep_alive && base.keep_alive.ws_watchdog_interval_seconds) || 20),
      ws_silence_timeout_seconds: Number((input.keep_alive && input.keep_alive.ws_silence_timeout_seconds) || (base.keep_alive && base.keep_alive.ws_silence_timeout_seconds) || 120)
    },
    last_register: input.last_register || {},
    created_at: input.created_at || base.created_at,
    updated_at: input.updated_at || base.updated_at
  };
}

function createConfigStore(options?: ConfigStoreOptions): ConfigStore {
  const configPath = options && options.configPath ? options.configPath : defaultConfigPath();
  const bootstrapPath = options && options.bootstrapPath ? options.bootstrapPath : defaultBootstrapPath();
  const stopSignalPath = options && options.stopSignalPath ? options.stopSignalPath : defaultStopSignalPath();

  return {
    configPath,
    bootstrapPath,
    stopSignalPath,
    exists(): boolean {
      return exists(configPath);
    },
    bootstrapExists(): boolean {
      return exists(bootstrapPath);
    },
    loadBootstrap(): BootstrapConfig {
      if (!exists(bootstrapPath)) {
        return {};
      }

      const text = readText(bootstrapPath);
      if (!text || !text.trim()) {
        return {};
      }

      return JSON.parse(text) as BootstrapConfig;
    },
    load(): AgentConfig {
      if (!exists(configPath)) {
        return createEmptyConfig();
      }

      const text = readText(configPath);
      if (!text || !text.trim()) {
        return createEmptyConfig();
      }

      return normalizeConfig(JSON.parse(text) as Partial<AgentConfig>);
    },
    save(config: AgentConfig): void {
      const nextConfig = normalizeConfig(config);
      nextConfig.updated_at = runtime.nowISOString();
      writeText(configPath, JSON.stringify(nextConfig, null, 2));
    }
  };
}

export {
  defaultConfigPath,
  defaultBootstrapPath,
  defaultStopSignalPath,
  defaultRuntimeLockPath,
  defaultHeartbeatLeasePath,
  createConfigStore
};

import * as runtime from "./lib/runtime";
import * as configStore from "./lib/config_store";
import * as centerClient from "./lib/center_client";
import * as wsClient from "./lib/ws_client";

import type { AgentCLIOptions, AgentConfig, BootstrapConfig, DeviceInfo } from "./types/agent";
import type { RegisterPayload, RegisterResponse } from "./types/protocol";
import type { IntervalHandle, LoggerLike } from "./types/runtime";

const STOP_SIGNAL_CHECK_INTERVAL_MS = 2000;
const RUNTIME_LOCK_STALE_MS = 2 * 60 * 1000;

interface RuntimeLockState {
  engine_id?: string;
  acquired_at?: string;
  updated_at?: string;
}

interface RuntimeLockHandle {
  alreadyRunning: boolean;
  path: string;
  release: () => void;
}

interface ConfigStoreLike {
  configPath: string;
  bootstrapPath: string;
  stopSignalPath: string;
  exists(): boolean;
  bootstrapExists(): boolean;
  loadBootstrap(): BootstrapConfig;
  load(): AgentConfig;
  save(config: AgentConfig): void;
}

interface WebSocketLike {
  close?: () => void;
}

interface StopSignalMonitorHandle {
  path: string;
  close: () => void;
}

function parseCLIArgs(args: string[]): AgentCLIOptions {
  const result: AgentCLIOptions = {
    center: "",
    config: "",
    dryRun: false,
    skipWS: false
  };

  for (let index = 0; index < args.length; index += 1) {
    const item = args[index];
    if (item === "--center") {
      result.center = args[index + 1] || "";
      index += 1;
    } else if (item === "--config") {
      result.config = args[index + 1] || "";
      index += 1;
    } else if (item === "--dry-run") {
      result.dryRun = true;
    } else if (item === "--skip-ws") {
      result.skipWS = true;
    }
  }

  return result;
}

function buildRegisterPayload(agentUUID: string, deviceInfo: DeviceInfo, deviceLinkSN?: string): RegisterPayload {
  return {
    agent_uuid: agentUUID,
    device_name: deviceInfo.device_name,
    brand: deviceInfo.brand,
    model: deviceInfo.model,
    android_id: deviceInfo.android_id,
    adb_serial: deviceInfo.adb_serial,
    device_link_sn: String(deviceLinkSN || "")
  };
}

function shallowCopy<T extends Record<string, any>>(input?: T): T {
  const output = {} as T;
  const source = input || ({} as T);

  for (const key in source) {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      output[key] = source[key];
    }
  }

  return output;
}

function mergeBootstrapConfig(config: AgentConfig, bootstrap?: BootstrapConfig): AgentConfig {
  const nextConfig = shallowCopy(config);
  const source = bootstrap || {};

  if (source.center_base_url) {
    nextConfig.center_base_url = source.center_base_url;
  }
  if (source.device_link_sn) {
    nextConfig.device_link_sn = source.device_link_sn;
  }

  if (source.websocket) {
    nextConfig.websocket = shallowCopy(nextConfig.websocket || {});
    if (Object.prototype.hasOwnProperty.call(source.websocket, "enabled")) {
      nextConfig.websocket.enabled = source.websocket.enabled;
    }
    if (source.websocket.heartbeat_interval_ms) {
      nextConfig.websocket.heartbeat_interval_ms = source.websocket.heartbeat_interval_ms;
    }
    if (Object.prototype.hasOwnProperty.call(source.websocket, "reconnect_enabled")) {
      nextConfig.websocket.reconnect_enabled = source.websocket.reconnect_enabled;
    }
    if (source.websocket.reconnect_initial_delay_ms) {
      nextConfig.websocket.reconnect_initial_delay_ms = source.websocket.reconnect_initial_delay_ms;
    }
    if (source.websocket.reconnect_max_delay_ms) {
      nextConfig.websocket.reconnect_max_delay_ms = source.websocket.reconnect_max_delay_ms;
    }
    if (source.websocket.reconnect_backoff_multiplier) {
      nextConfig.websocket.reconnect_backoff_multiplier = source.websocket.reconnect_backoff_multiplier;
    }
  }

  return nextConfig;
}

function mergeRegisterResult(config: AgentConfig, response: RegisterResponse): AgentConfig {
  const data = response && response.data ? response.data : {};
  const nextConfig = shallowCopy(config);

  nextConfig.device_id = data.device_id || nextConfig.device_id || "";
  nextConfig.last_register = {
    status: response.status || "",
    device_id: nextConfig.device_id,
    bind_status: data.bind_status || "",
    register_status: data.status || "",
    registered_at: runtime.nowISOString()
  };

  return nextConfig;
}

function isPromiseLike<T = unknown>(value: unknown): value is Promise<T> {
  return !!(value && typeof (value as Promise<T>).then === "function");
}

function getStopSignalPath(store?: Partial<ConfigStoreLike>): string {
  if (store && store.stopSignalPath) {
    return store.stopSignalPath;
  }
  return configStore.defaultStopSignalPath();
}

function getRuntimeLockPath(store?: { runtimeLockPath?: string }): string {
  if (store && store.runtimeLockPath) {
    return store.runtimeLockPath;
  }
  return configStore.defaultRuntimeLockPath();
}

function parseRuntimeLock(text: string): RuntimeLockState {
  if (!text || !String(text).trim()) {
    return {};
  }

  try {
    return JSON.parse(String(text)) as RuntimeLockState;
  } catch (_error) {
    return {};
  }
}

function getCurrentEngineID(): string {
  try {
    if (typeof engines !== "undefined" && typeof engines.myEngine === "function") {
      const current = engines.myEngine();
      if (current && current.id) {
        return String(current.id);
      }
      if (current && typeof current.toString === "function") {
        return String(current.toString());
      }
    }
  } catch (_error) {
    return "";
  }

  return "";
}

function isEngineAlive(engineID: string): boolean {
  if (!engineID) {
    return false;
  }

  try {
    if (typeof engines === "undefined" || typeof engines.all !== "function") {
      return false;
    }

    const list = engines.all();
    for (let index = 0; index < list.length; index += 1) {
      if (String(list[index].id) === String(engineID)) {
        return true;
      }
    }
  } catch (_error) {
    return false;
  }

  return false;
}

function acquireRuntimeLock(store: ConfigStoreLike, logger: LoggerLike): RuntimeLockHandle {
  const lockPath = getRuntimeLockPath(store as unknown as { runtimeLockPath?: string });
  const currentEngineID = getCurrentEngineID();
  const nowISO = runtime.nowISOString();
  const existing = parseRuntimeLock(runtime.readTextFile(lockPath));
  const existingUpdatedAt = Date.parse(existing.updated_at || "");
  const isStale = !existingUpdatedAt || (Date.now() - existingUpdatedAt) > RUNTIME_LOCK_STALE_MS;

  if (existing.engine_id && existing.engine_id !== currentEngineID && isEngineAlive(existing.engine_id) && !isStale) {
    return {
      alreadyRunning: true,
      path: lockPath,
      release(): void {}
    };
  }

  runtime.writeTextFile(lockPath, JSON.stringify({
    engine_id: currentEngineID,
    acquired_at: nowISO,
    updated_at: nowISO
  }, null, 2));
  logger.info("已获取 Agent 运行锁：" + lockPath);

  return {
    alreadyRunning: false,
    path: lockPath,
    release(): void {
      const latest = parseRuntimeLock(runtime.readTextFile(lockPath));
      if (!latest.engine_id || latest.engine_id === currentEngineID) {
        runtime.removeFileIfExists(lockPath);
        logger.info("已释放 Agent 运行锁：" + lockPath);
      }
    }
  };
}

function clearStaleStopSignal(store: ConfigStoreLike, logger: LoggerLike): void {
  const stopSignalPath = getStopSignalPath(store);
  if (!runtime.fileExists(stopSignalPath)) {
    return;
  }

  runtime.removeFileIfExists(stopSignalPath);
  logger.info("检测到遗留停止信号，启动前已清理：" + stopSignalPath);
}

function startStopSignalMonitor(options?: {
  store?: ConfigStoreLike;
  websocket?: WebSocketLike | null;
  logger?: LoggerLike;
}): StopSignalMonitorHandle | null {
  if (runtime.isNodeRuntime()) {
    return null;
  }

  const monitorOptions = options || {};
  const logger = monitorOptions.logger || runtime.createLogger();
  const websocket = monitorOptions.websocket;
  const stopSignalPath = getStopSignalPath(monitorOptions.store);
  let monitor: IntervalHandle | null = null;
  let stopped = false;

  function shutdownAgent(): void {
    if (stopped) {
      return;
    }
    stopped = true;

    logger.info("检测到 stop.signal，Agent 即将优雅退出：" + stopSignalPath);
    runtime.removeFileIfExists(stopSignalPath);

    if (monitor) {
      monitor.cancel();
      monitor = null;
    }

    if (websocket && typeof websocket.close === "function") {
      websocket.close();
    }

    logger.info("Agent 已停止心跳与重连，准备退出当前脚本。");
    runtime.exitProcess(0);
  }

  monitor = runtime.startInterval(function watchStopSignal() {
    if (runtime.fileExists(stopSignalPath)) {
      shutdownAgent();
    }
  }, STOP_SIGNAL_CHECK_INTERVAL_MS);

  return {
    path: stopSignalPath,
    close(): void {
      stopped = true;
      if (monitor) {
        monitor.cancel();
        monitor = null;
      }
    }
  };
}

function main(cliOptions?: Partial<AgentCLIOptions>): Record<string, unknown> | Promise<Record<string, unknown>> {
  const logger = runtime.createLogger();
  const options = cliOptions || {};
  const store = configStore.createConfigStore({ configPath: options.config }) as ConfigStoreLike;
  const runtimeLock = acquireRuntimeLock(store, logger);

  if (runtimeLock.alreadyRunning) {
    logger.info("检测到 Agent 已在运行，当前启动请求直接结束：" + runtimeLock.path);
    return {
      status: "already_running",
      runtime_lock: runtimeLock.path
    };
  }

  try {
    const configExists = store.exists();
    let config = store.load();

    if (store.bootstrapExists()) {
      config = mergeBootstrapConfig(config, store.loadBootstrap());
      if (configExists) {
        logger.info("检测到 bootstrap 配置，已刷新中心地址与连接参数，并保留本地身份配置。");
      } else {
        logger.info("检测到 bootstrap 配置，已用于初始化本地 config.json。");
      }
    }

    clearStaleStopSignal(store, logger);

    if (options.center) {
      config.center_base_url = options.center;
    }

    const deviceInfo = runtime.collectDeviceInfo(config.device);
    config.device = deviceInfo;

    if (!config.agent_uuid) {
      config.agent_uuid = runtime.createStableAgentUUID(deviceInfo);
      logger.info("已根据稳定指纹生成 agent_uuid：" + config.agent_uuid);
    }

    store.save(config);

    const payload = buildRegisterPayload(config.agent_uuid, deviceInfo, config.device_link_sn);
    logger.info("Agent 配置文件：" + store.configPath);
    logger.info("Agent 引导文件：" + store.bootstrapPath);
    logger.info("Agent 运行锁文件：" + runtimeLock.path);
    logger.info("中心服务地址：" + config.center_base_url);

    if (options.dryRun) {
      logger.info("当前为 dry-run，仅验证配置和注册载荷，不请求中心服务。");
      runtimeLock.release();
      return {
        status: "dry_run",
        config_path: store.configPath,
        agent_uuid: config.agent_uuid,
        device_id: config.device_id || "",
        register_payload: payload
      };
    }

    logger.info("开始向中心服务注册设备。");
    const response = centerClient.registerDevice(config.center_base_url, payload);
    if (isPromiseLike<RegisterResponse>(response)) {
      return response.then(function onResolved(resolvedResponse) {
        return finishRegister(config, store, resolvedResponse, logger, options, runtimeLock);
      }).catch(function onRejected(error) {
        runtimeLock.release();
        throw error;
      });
    }

    return finishRegister(config, store, response, logger, options, runtimeLock);
  } catch (error) {
    runtimeLock.release();
    throw error;
  }
}

function finishRegister(
  config: AgentConfig,
  store: ConfigStoreLike,
  response: RegisterResponse,
  logger: LoggerLike,
  options: Partial<AgentCLIOptions>,
  runtimeLock: RuntimeLockHandle
): Record<string, unknown> {
  const nextConfig = mergeRegisterResult(config, response);
  let websocketResult: WebSocketLike | null = null;
  let stopSignalMonitor: StopSignalMonitorHandle | null = null;

  store.save(nextConfig);
  logger.info("设备注册完成，device_id=" + nextConfig.device_id);

  if (shouldStartWebSocket(nextConfig, options)) {
    websocketResult = wsClient.connect({
      centerBaseURL: nextConfig.center_base_url,
      deviceID: nextConfig.device_id,
      agentUUID: nextConfig.agent_uuid,
      deviceLinkSN: nextConfig.device_link_sn || "",
      heartbeatIntervalMS: getHeartbeatIntervalMS(nextConfig),
      reconnectEnabled: getReconnectEnabled(nextConfig),
      reconnectInitialDelayMS: getReconnectInitialDelayMS(nextConfig),
      reconnectMaxDelayMS: getReconnectMaxDelayMS(nextConfig),
      reconnectBackoffMultiplier: getReconnectBackoffMultiplier(nextConfig),
      onAssignTask(taskSummary) {
        logger.info("Agent 已接收任务摘要：" + JSON.stringify(taskSummary));
      },
      logger
    }) as WebSocketLike;

    stopSignalMonitor = startStopSignalMonitor({
      store,
      websocket: websocketResult,
      logger
    });
  } else {
    logger.info("已跳过 WebSocket 连接。");
  }

  return {
    status: "registered",
    config_path: store.configPath,
    agent_uuid: nextConfig.agent_uuid,
    device_id: nextConfig.device_id,
    websocket: websocketResult,
    stop_signal_monitor: stopSignalMonitor,
    runtime_lock: runtimeLock ? runtimeLock.path : "",
    response
  };
}

function shouldStartWebSocket(config: AgentConfig, options?: Partial<AgentCLIOptions>): boolean {
  const websocketConfig = config.websocket || {};

  if (options && options.skipWS) {
    return false;
  }
  if (options && options.dryRun) {
    return false;
  }
  if (websocketConfig.enabled === false) {
    return false;
  }
  if (runtime.isNodeRuntime()) {
    return false;
  }

  return !!config.device_id;
}

function getHeartbeatIntervalMS(config: AgentConfig): number {
  const websocketConfig = config.websocket || {};
  const interval = Number(websocketConfig.heartbeat_interval_ms || 30000);
  if (!interval || interval < 1000) {
    return 30000;
  }
  return interval;
}

function getReconnectEnabled(config: AgentConfig): boolean {
  const websocketConfig = config.websocket || {};
  return websocketConfig.reconnect_enabled !== false;
}

function getReconnectInitialDelayMS(config: AgentConfig): number {
  const websocketConfig = config.websocket || {};
  const value = Number(websocketConfig.reconnect_initial_delay_ms || 3000);
  if (!value || value < 1000) {
    return 3000;
  }
  return value;
}

function getReconnectMaxDelayMS(config: AgentConfig): number {
  const websocketConfig = config.websocket || {};
  const value = Number(websocketConfig.reconnect_max_delay_ms || 60000);
  if (!value || value < 1000) {
    return 60000;
  }
  return value;
}

function getReconnectBackoffMultiplier(config: AgentConfig): number {
  const websocketConfig = config.websocket || {};
  const value = Number(websocketConfig.reconnect_backoff_multiplier || 2);
  if (!value || value < 1) {
    return 2;
  }
  return value;
}

function run(): void {
  const args = runtime.isNodeRuntime() ? parseCLIArgs(process.argv.slice(2)) : {};
  const logger = runtime.createLogger();

  try {
    const result = main(args);
    if (isPromiseLike<Record<string, unknown>>(result)) {
      result.then(function onResolved(resolvedResult) {
        logger.info("Agent 启动结果：" + JSON.stringify(resolvedResult));
      }).catch(function onRejected(error) {
        logRunError(error, logger);
      });
      return;
    }

    logger.info("Agent 启动结果：" + JSON.stringify(result));
  } catch (error) {
    logRunError(error, logger);
  }
}

function logRunError(error: unknown, logger: LoggerLike): void {
  if (error && typeof error === "object" && "message" in error && (error as { message?: string }).message === "agent_instance_already_running") {
    logger.info("检测到 Agent 已在运行，当前启动请求直接结束。");
    return;
  }

  const hasMessage = !!(error && typeof error === "object" && "message" in error && (error as { message?: unknown }).message);
  const hasStack = !!(error && typeof error === "object" && "stack" in error && (error as { stack?: unknown }).stack);

  if (hasMessage) {
    logger.error(String((error as { message?: unknown }).message));
  }
  if (hasStack) {
    logger.error(String((error as { stack?: unknown }).stack));
  }
  if (!hasMessage && !hasStack) {
    logger.error(String(error));
  }

  if (runtime.isNodeRuntime()) {
    process.exitCode = 1;
  }
}

if (runtime.isNodeRuntime()) {
  if (require.main === module) {
    run();
  }
} else {
  run();
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    main,
    parseCLIArgs,
    buildRegisterPayload
  };
}

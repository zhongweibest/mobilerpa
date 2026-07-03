import * as runtime from "./runtime";
import * as taskRunner from "./task_runner";
import * as workflowSessionRunner from "./workflow_session_runner";
import * as heartbeatScheduler from "./heartbeat_scheduler";

import type { LoggerLike } from "../types/runtime";
import type { HeartbeatSchedulerHandle } from "./heartbeat_scheduler";
import type {
  AckPayload,
  TaskProgress,
  TaskResult,
  TaskSummary,
  WebSocketEnvelope,
  WorkflowSessionPayload,
  WorkflowSessionRefs,
  WorkflowSessionResult
} from "../types/protocol";

interface WSClientOptions {
  centerBaseURL: string;
  deviceID: string;
  agentUUID?: string;
  deviceLinkSN?: string;
  heartbeatLeasePath?: string;
  heartbeatIntervalMS?: number;
  heartbeatScheduler?: string;
  pingIntervalMS?: number;
  watchdogIntervalMS?: number;
  silenceTimeoutMS?: number;
  reconnectEnabled?: boolean;
  reconnectInitialDelayMS?: number;
  reconnectMaxDelayMS?: number;
  reconnectBackoffMultiplier?: number;
  onAssignTask?: (taskSummary: TaskSummary) => void;
  logger?: LoggerLike;
}

interface WebSocketHandle {
  url: string;
  skipped?: boolean;
  close?: () => void;
}

type GenericMessage = WebSocketEnvelope<Record<string, unknown>> & {
  payload?: Record<string, unknown> & AckPayload;
};

interface ProgressDedupState {
  key: string;
  at: number;
}

interface SessionFlagStore {
  put(key: string): void;
  remove(key: string): void;
  has(key: string): boolean;
}

function trimBaseURL(baseURL: string): string {
  return String(baseURL || "").replace(/\/+$/, "");
}

function buildWebSocketURL(centerBaseURL: string): string {
  const base = trimBaseURL(centerBaseURL);
  if (base.indexOf("https://") === 0) {
    return "wss://" + base.slice("https://".length) + "/ws";
  }
  if (base.indexOf("http://") === 0) {
    return "ws://" + base.slice("http://".length) + "/ws";
  }
  if (base.indexOf("ws://") === 0 || base.indexOf("wss://") === 0) {
    return base.replace(/\/+$/, "") + "/ws";
  }
  return "ws://" + base + "/ws";
}

function createRequestID(prefix?: string): string {
  return String(prefix || "request") + "-" + Date.now() + "-" + Math.floor(Math.random() * 100000);
}

function createEnvelope<TPayload>(type: string, requestID: string, deviceID: string, payload?: TPayload): WebSocketEnvelope<TPayload | Record<string, never>> {
  return {
    type,
    request_id: requestID,
    device_id: deviceID,
    timestamp: Math.floor(Date.now() / 1000),
    payload: payload || {}
  };
}

function isOkAck(message: GenericMessage | null | undefined, messageType: string): boolean {
  return !!(
    message &&
    message.type === "ack" &&
    message.payload &&
    message.payload.message_type === messageType &&
    message.payload.status === "ok"
  );
}

function javaType(name: string): any {
  if (typeof Java !== "undefined" && typeof Java.type === "function") {
    return Java.type(name);
  }

  const parts = String(name || "").split(".");
  let current = Packages;

  for (let index = 0; index < parts.length; index += 1) {
    current = current[parts[index]];
  }

  return current;
}

function createWebSocketListener(callbacks: Record<string, unknown>): any {
  const WebSocketListener = javaType("okhttp3.WebSocketListener");
  if (typeof JavaAdapter === "function") {
    return new JavaAdapter(WebSocketListener, callbacks);
  }
  return new WebSocketListener(callbacks);
}

function createRunnable(runCallback: () => void): any {
  const Runnable = javaType("java.lang.Runnable");
  if (typeof JavaAdapter === "function") {
    return new JavaAdapter(Runnable, {
      run: runCallback
    });
  }
  return new Runnable({
    run: runCallback
  });
}

function createSessionFlagStore(): SessionFlagStore {
  if (typeof java !== "undefined") {
    try {
      const ConcurrentHashMap = javaType("java.util.concurrent.ConcurrentHashMap");
      const map = new ConcurrentHashMap();
      return {
        put(key: string): void {
          map.put(String(key || ""), true);
        },
        remove(key: string): void {
          map.remove(String(key || ""));
        },
        has(key: string): boolean {
          return map.containsKey(String(key || ""));
        }
      };
    } catch (_error) {
      // 回退到普通对象存储。
    }
  }

  const state: Record<string, boolean> = {};
  return {
    put(key: string): void {
      state[String(key || "")] = true;
    },
    remove(key: string): void {
      delete state[String(key || "")];
    },
    has(key: string): boolean {
      return state[String(key || "")] === true;
    }
  };
}

function buildTaskSummary(taskPayload?: Record<string, unknown>): TaskSummary {
  const payload = taskPayload || {};
  return {
    task_id: String(payload.task_id || ""),
    script_name: String(payload.script_name || ""),
    script_version: String(payload.script_version || ""),
    priority: Number(payload.priority || 0),
    params: (payload.params || {}) as Record<string, unknown>
  };
}

function buildWorkflowSessionRefs(payload?: Record<string, unknown>): WorkflowSessionRefs {
  const sessionPayload = payload || {};
  return {
    plan_run_id: String(sessionPayload.plan_run_id || ""),
    plan_device_run_id: String(sessionPayload.plan_device_run_id || "")
  };
}

function connectAutoJs(options: WSClientOptions): WebSocketHandle {
  const logger = options.logger || runtime.createLogger();
  const OkHttpClient = javaType("okhttp3.OkHttpClient");
  const Request = javaType("okhttp3.Request");
  const TimeUnit = javaType("java.util.concurrent.TimeUnit");
  const Executors = javaType("java.util.concurrent.Executors");
  let heartbeatHandle: HeartbeatSchedulerHandle | null = null;
  let reconnectExecutor: any = null;
  let reconnectFuture: any = null;
  const wsURL = buildWebSocketURL(options.centerBaseURL);
  const heartbeatIntervalMS = Number(options.heartbeatIntervalMS || 30000);
  const heartbeatSchedulerMode = String(options.heartbeatScheduler || "executor");
  const heartbeatLeasePath = String(options.heartbeatLeasePath || "");
  const pingIntervalMS = Number(options.pingIntervalMS || Math.min(heartbeatIntervalMS, 20000));
  const watchdogIntervalMS = Number(options.watchdogIntervalMS || 20000);
  const silenceTimeoutMS = Number(options.silenceTimeoutMS || 120000);
  const reconnectEnabled = options.reconnectEnabled !== false;
  let reconnectInitialDelayMS = Number(options.reconnectInitialDelayMS || 3000);
  let reconnectMaxDelayMS = Number(options.reconnectMaxDelayMS || 60000);
  let reconnectBackoffMultiplier = Number(options.reconnectBackoffMultiplier || 2);
  const deviceID = String(options.deviceID || "");
  const agentUUID = String(options.agentUUID || "");
  const deviceLinkSN = String(options.deviceLinkSN || "");
  const onAssignTask = typeof options.onAssignTask === "function" ? options.onAssignTask : null;
  let heartbeatStarted = false;
  let heartbeatGeneration = 0;
  let heartbeatLeaseToken = "";
  let reconnectAttempt = 0;
  let reconnectScheduledGeneration = 0;
  let connectGeneration = 0;
  let closedGeneration = 0;
  let intentionallyClosed = false;
  let lastReceiveAt = Date.now();
  let lastSendAt = Date.now();
  let taskExecuting = false;
  let workflowSessionExecuting = false;
  const workflowStopFlags = createSessionFlagStore();
  const workflowResultSentFlags = createSessionFlagStore();
  let currentWorkflowRunID = "";
  let lastTaskProgressState: ProgressDedupState | null = null;
  let lastWorkflowEventState: ProgressDedupState | null = null;
  let watchdogExecutor: any = null;
  let watchdogFuture: any = null;
  logger.info("WebSocket 心跳调度模式：" + heartbeatSchedulerMode);
  const client = new OkHttpClient.Builder()
    .readTimeout(0, TimeUnit.MILLISECONDS)
    .pingInterval(Math.max(5000, pingIntervalMS), TimeUnit.MILLISECONDS)
    .build();
  let socket: any = null;
  const activeHeartbeatScheduler = heartbeatScheduler.createHeartbeatScheduler(heartbeatSchedulerMode, logger);

  function shouldSkipDuplicateEvent(
    cache: ProgressDedupState | null,
    key: string,
    dedupWindowMS: number
  ): boolean {
    if (!cache) {
      return false;
    }
    return cache.key === key && (Date.now() - cache.at) <= dedupWindowMS;
  }

  function send(type: string, requestID: string, payload: Record<string, unknown>): void {
    if (!socket) {
      throw new Error("websocket_not_connected");
    }
    const message = createEnvelope(type, requestID, deviceID, payload);
    const text = JSON.stringify(message);
    logger.info("发送 WebSocket 消息：" + type + "，request_id=" + requestID);
    if (socket.send(text) === false) {
      throw new Error("websocket_send_failed");
    }
    lastSendAt = Date.now();
  }

  function readHeartbeatLeaseToken(): string {
    if (!heartbeatLeasePath) {
      return "";
    }

    try {
      const text = runtime.readTextFile(heartbeatLeasePath);
      if (!text || !String(text).trim()) {
        return "";
      }
      const payload = JSON.parse(String(text)) as { token?: string };
      return String(payload.token || "");
    } catch (_error) {
      return "";
    }
  }

  function writeHeartbeatLeaseToken(token: string): void {
    if (!heartbeatLeasePath) {
      return;
    }

    runtime.writeTextFile(heartbeatLeasePath, JSON.stringify({
      token: String(token || ""),
      updated_at: runtime.nowISOString()
    }, null, 2));
  }

  function clearHeartbeatLeaseToken(token: string): void {
    if (!heartbeatLeasePath) {
      return;
    }

    const currentToken = readHeartbeatLeaseToken();
    if (!currentToken || currentToken === String(token || "")) {
      runtime.removeFileIfExists(heartbeatLeasePath);
    }
  }

  function isHeartbeatLeaseActive(token: string): boolean {
    if (!heartbeatLeasePath) {
      return true;
    }
    return readHeartbeatLeaseToken() === String(token || "");
  }

  function sendHeartbeat(source: string): void {
    logger.info(
      "准备发送心跳，source="
      + String(source || "")
      + "，connect_generation="
      + connectGeneration
      + "，heartbeat_generation="
      + heartbeatGeneration
    );
    send("heartbeat", createRequestID("agent-heartbeat"), {
      agent_uuid: agentUUID,
      device_link_sn: deviceLinkSN,
      execution_profile: runtime.collectExecutionProfile()
    });
  }

  function sendTaskAck(taskPayload?: Record<string, unknown>): void {
    const summary = buildTaskSummary(taskPayload);
    send("task_ack", createRequestID("agent-task-ack"), {
      task_id: summary.task_id,
      status: "ok",
      message: "Agent 已收到任务，准备执行",
      script_name: summary.script_name,
      script_version: summary.script_version
    });
    logger.info("已发送 task_ack：" + summary.task_id);
  }

  function sendTaskResult(taskSummary: Partial<TaskSummary>, result: Partial<TaskResult>): void {
    const summary = taskSummary || {};
    const payload = result || {};
    send("task_result", createRequestID("agent-task-result"), {
      task_id: String(summary.task_id || ""),
      status: String(payload.status || "failed"),
      result_code: String(payload.result_code || ""),
      result_message: String(payload.result_message || ""),
      step_name: String(payload.step_name || ""),
      extra: payload.extra || {}
    });
    logger.info("已发送 task_result：" + String(summary.task_id || "") + " -> " + String(payload.status || "failed"));
  }

  function sendTaskProgress(taskSummary: Partial<TaskSummary>, progress: Partial<TaskProgress>): void {
    const summary = taskSummary || {};
    const payload = progress || {};
    const eventKey = JSON.stringify({
      task_id: String(summary.task_id || ""),
      status: String(payload.status || "running"),
      step_name: String(payload.step_name || ""),
      message: String(payload.message || ""),
      extra: payload.extra || {}
    });

    if (shouldSkipDuplicateEvent(lastTaskProgressState, eventKey, 1500)) {
      logger.info("跳过重复 task_progress：" + String(summary.task_id || "") + " -> " + String(payload.step_name || ""));
      return;
    }

    send("task_progress", createRequestID("agent-task-progress"), {
      task_id: String(summary.task_id || ""),
      status: String(payload.status || "running"),
      step_name: String(payload.step_name || ""),
      message: String(payload.message || ""),
      extra: payload.extra || {}
    });
    lastTaskProgressState = {
      key: eventKey,
      at: Date.now()
    };
    logger.info("已发送 task_progress：" + String(summary.task_id || "") + " -> " + String(payload.step_name || ""));
  }

  function sendUnifiedProgress(taskSummary: Partial<TaskSummary>, progress: Partial<TaskProgress>): void {
    sendTaskProgress(taskSummary, progress);
  }

  function sendScriptSyncAck(syncPayload?: Record<string, unknown>): void {
    const payload = syncPayload || {};
    send("script_sync_ack", createRequestID("agent-script-sync-ack"), {
      script_name: String(payload.script_name || ""),
      script_version: String(payload.script_version || ""),
      status: "ok",
      message: "Agent 已收到脚本同步指令"
    });
    logger.info("已发送 script_sync_ack：" + String(payload.script_name || "") + "@" + String(payload.script_version || ""));
  }

  function sendScriptSyncResult(syncPayload: Record<string, unknown> | undefined, result: Partial<TaskResult>): void {
    const payload = syncPayload || {};
    const summary = result || {};
    send("script_sync_result", createRequestID("agent-script-sync-result"), {
      script_name: String(payload.script_name || ""),
      script_version: String(payload.script_version || ""),
      status: String(summary.status || "failed"),
      result_code: String(summary.result_code || ""),
      result_message: String(summary.result_message || ""),
      extra: summary.extra || {}
    });
    logger.info("已发送 script_sync_result：" + String(payload.script_name || "") + "@" + String(payload.script_version || "") + " -> " + String(summary.status || "failed"));
  }

  function sendWorkflowSessionAck(sessionPayload?: Record<string, unknown>): void {
    const payload = sessionPayload || {};
    const refs = buildWorkflowSessionRefs(payload);
    send("workflow_session_ack", createRequestID("agent-workflow-session-ack"), {
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      status: "ok",
      message: "Agent 已收到工作流会话"
    });
    logger.info("已发送 workflow_session_ack：" + String(refs.plan_device_run_id || ""));
  }

  function sendWorkflowSessionEvent(eventPayload?: Record<string, unknown>): void {
    const payload = eventPayload || {};
    const refs = buildWorkflowSessionRefs(payload);
    const eventKey = JSON.stringify({
      plan_device_run_id: refs.plan_device_run_id,
      workflow_node_id: String(payload.workflow_node_id || ""),
      event_type: String(payload.event_type || ""),
      status: String(payload.status || "running"),
      step_name: String(payload.step_name || ""),
      message: String(payload.message || ""),
      extra: payload.extra || {}
    });

    if (shouldSkipDuplicateEvent(lastWorkflowEventState, eventKey, 1500)) {
      logger.info("跳过重复 workflow_session_event：" + String(refs.plan_device_run_id || "") + " -> " + String(payload.step_name || ""));
      return;
    }

    send("workflow_session_event", createRequestID("agent-workflow-session-event"), {
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      workflow_node_id: String(payload.workflow_node_id || ""),
      event_type: String(payload.event_type || ""),
      status: String(payload.status || "running"),
      step_name: String(payload.step_name || ""),
      message: String(payload.message || ""),
      extra: payload.extra || {}
    });
    logger.info(
      "已发送 workflow_session_event："
      + String(refs.plan_device_run_id || "")
      + " -> "
      + String(payload.event_type || "")
      + " / "
      + String(payload.status || "running")
      + " / "
      + String(payload.message || "")
    );
    lastWorkflowEventState = {
      key: eventKey,
      at: Date.now()
    };
  }

  function sendWorkflowSessionResult(resultPayload: Partial<WorkflowSessionResult> & Record<string, unknown>): void {
    const payload = resultPayload || {};
    const refs = buildWorkflowSessionRefs(payload);
    const sessionKey = String(refs.plan_device_run_id || "");

    if (sessionKey && workflowResultSentFlags.has(sessionKey)) {
      logger.warn("工作流结果已发送，忽略重复 workflow_session_result：" + sessionKey + " -> " + String(payload.status || "failed"));
      return;
    }

    send("workflow_session_result", createRequestID("agent-workflow-session-result"), {
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      workflow_node_id: String(payload.workflow_node_id || ""),
      status: String(payload.status || "failed"),
      result_code: String(payload.result_code || ""),
      result_message: String(payload.result_message || ""),
      extra: payload.extra || {}
    });
    logger.info(
      "已发送 workflow_session_result："
      + String(refs.plan_device_run_id || "")
      + " -> "
      + String(payload.status || "failed")
      + " / "
      + String(payload.result_message || "")
      + " / extra="
      + JSON.stringify(payload.extra || {})
    );

    if (sessionKey) {
      workflowResultSentFlags.put(sessionKey);
    }
  }

  function markWorkflowStopRequested(sessionKey: string): void {
    const nextKey = String(sessionKey || "").trim();
    if (!nextKey) {
      return;
    }
    workflowStopFlags.put(nextKey);
  }

  function clearWorkflowStopRequested(sessionKey: string): void {
    const nextKey = String(sessionKey || "").trim();
    if (!nextKey) {
      return;
    }
    workflowStopFlags.remove(nextKey);
  }

  function clearWorkflowResultSent(sessionKey: string): void {
    const nextKey = String(sessionKey || "").trim();
    if (!nextKey) {
      return;
    }
    workflowResultSentFlags.remove(nextKey);
  }

  function isWorkflowStopRequested(sessionKey: string): boolean {
    const nextKey = String(sessionKey || "").trim();
    if (!nextKey) {
      return false;
    }
    return workflowStopFlags.has(nextKey);
  }

  function stopHeartbeat(): void {
    heartbeatGeneration += 1;
    if (heartbeatHandle) {
      logger.info(
        "停止心跳调度器，mode="
        + heartbeatHandle.kind
        + "，scheduler_id="
        + heartbeatHandle.schedulerID
        + "，next_generation="
        + heartbeatGeneration
      );
      heartbeatHandle.cancel();
      heartbeatHandle = null;
    }
    clearHeartbeatLeaseToken(heartbeatLeaseToken);
    heartbeatLeaseToken = "";
    heartbeatStarted = false;
  }

  function stopReconnect(): void {
    if (reconnectFuture) {
      reconnectFuture.cancel(false);
      reconnectFuture = null;
    }
    if (reconnectExecutor) {
      reconnectExecutor.shutdownNow();
      reconnectExecutor = null;
    }
    reconnectScheduledGeneration = 0;
  }

  function stopWatchdog(): void {
    if (watchdogFuture) {
      watchdogFuture.cancel(false);
      watchdogFuture = null;
    }
    if (watchdogExecutor) {
      watchdogExecutor.shutdownNow();
      watchdogExecutor = null;
    }
  }

  function resetReconnectState(): void {
    reconnectAttempt = 0;
    stopReconnect();
  }

  function sanitizeReconnectDelay(value: number, fallback: number): number {
    const nextValue = Number(value || fallback);
    if (!nextValue || nextValue < 1000) {
      return fallback;
    }
    return nextValue;
  }

  reconnectInitialDelayMS = sanitizeReconnectDelay(reconnectInitialDelayMS, 3000);
  reconnectMaxDelayMS = sanitizeReconnectDelay(reconnectMaxDelayMS, 60000);
  if (reconnectMaxDelayMS < reconnectInitialDelayMS) {
    reconnectMaxDelayMS = reconnectInitialDelayMS;
  }
  if (!reconnectBackoffMultiplier || reconnectBackoffMultiplier < 1) {
    reconnectBackoffMultiplier = 2;
  }

  function getReconnectDelayMS(attempt: number): number {
    let delay = reconnectInitialDelayMS;
    const currentAttempt = Number(attempt || 1);
    let index = 1;

    while (index < currentAttempt) {
      delay = Math.min(reconnectMaxDelayMS, delay * reconnectBackoffMultiplier);
      index += 1;
    }

    return Math.min(delay, reconnectMaxDelayMS);
  }

  function closeSocketQuietly(closeCode?: number, closeReason?: string): void {
    if (!socket) {
      return;
    }
    try {
      socket.close(closeCode || 1000, closeReason || "close");
    } catch (error) {
      logger.warn("WebSocket 关闭时出现异常：" + String(error));
    }
  }

  function startWatchdog(): void {
    const safeWatchdogIntervalMS = Math.max(5000, watchdogIntervalMS);
    const safeSilenceTimeoutMS = Math.max(safeWatchdogIntervalMS * 2, silenceTimeoutMS);

    stopWatchdog();
    watchdogExecutor = Executors.newSingleThreadScheduledExecutor();
    watchdogFuture = watchdogExecutor.scheduleAtFixedRate(createRunnable(function watchdogTask() {
      if (intentionallyClosed) {
        return;
      }

      const now = Date.now();
      const silenceMS = now - Math.max(lastReceiveAt, lastSendAt);

      if (!socket) {
        logger.warn("WebSocket watchdog 检测到连接对象缺失，准备触发重连。");
        scheduleReconnect("watchdog_missing_socket");
        return;
      }

      if (silenceMS >= safeSilenceTimeoutMS) {
        logger.warn("WebSocket watchdog 检测到连接静默超时，准备主动重连，silence_ms=" + silenceMS);
        closeSocketQuietly(1001, "watchdog silence timeout");
        handleSocketClosed(connectGeneration, "watchdog_silence_timeout " + silenceMS);
      }
    }), safeWatchdogIntervalMS, safeWatchdogIntervalMS, TimeUnit.MILLISECONDS);

    logger.info(
      "已启动 WebSocket watchdog，interval_ms="
      + safeWatchdogIntervalMS
      + "，silence_timeout_ms="
      + safeSilenceTimeoutMS
      + "，ping_interval_ms="
      + Math.max(5000, pingIntervalMS)
    );
  }

  function scheduleReconnect(reason: string): void {
    if (!reconnectEnabled) {
      logger.warn("WebSocket 自动重连已禁用，原因：" + String(reason || ""));
      return;
    }
    if (intentionallyClosed) {
      logger.info("WebSocket 为主动关闭，不进入自动重连。");
      return;
    }
    if (reconnectFuture) {
      return;
    }
    if (reconnectScheduledGeneration === connectGeneration) {
      return;
    }

    reconnectScheduledGeneration = connectGeneration;
    reconnectAttempt += 1;
    const delayMS = getReconnectDelayMS(reconnectAttempt);
    const scheduledGeneration = connectGeneration;
    logger.warn("WebSocket 连接中断，准备第 " + reconnectAttempt + " 次重连，delay=" + delayMS + "ms，原因：" + String(reason || ""));

    reconnectExecutor = Executors.newSingleThreadScheduledExecutor();
    reconnectFuture = reconnectExecutor.schedule(createRunnable(function reconnectTask() {
      reconnectFuture = null;
      if (reconnectExecutor) {
        reconnectExecutor.shutdownNow();
        reconnectExecutor = null;
      }
      if (intentionallyClosed) {
        logger.info("WebSocket 在重连等待期间被主动关闭，取消重连。");
        return;
      }
      if (scheduledGeneration !== connectGeneration) {
        logger.info("WebSocket 重连任务代际已过期，取消执行：scheduled_generation=" + scheduledGeneration + "，current_generation=" + connectGeneration);
        return;
      }
      logger.info("WebSocket 开始执行第 " + reconnectAttempt + " 次重连：" + wsURL);
      openSocket();
    }), delayMS, TimeUnit.MILLISECONDS);
  }

  function handleSocketClosed(expectedGeneration: number, reason: string): void {
    if (expectedGeneration !== connectGeneration) {
      return;
    }
    if (closedGeneration === expectedGeneration) {
      return;
    }
    closedGeneration = expectedGeneration;
    stopHeartbeat();
    socket = null;
    scheduleReconnect(reason);
  }

  function startHeartbeat(expectedGeneration: number): void {
    if (heartbeatStarted) {
      return;
    }
    heartbeatStarted = true;
    heartbeatGeneration += 1;
    const currentHeartbeatGeneration = heartbeatGeneration;
    heartbeatLeaseToken = String(connectGeneration) + ":" + String(currentHeartbeatGeneration) + ":" + String(Date.now());
    writeHeartbeatLeaseToken(heartbeatLeaseToken);
    sendHeartbeat("hello_ack");
    heartbeatHandle = activeHeartbeatScheduler.start({
      intervalMS: heartbeatIntervalMS,
      logger,
      onTick: function heartbeatTask() {
      logger.info(
        "心跳调度器触发，mode="
        + (heartbeatHandle ? heartbeatHandle.kind : activeHeartbeatScheduler.kind)
        + "，scheduler_id="
        + (heartbeatHandle ? heartbeatHandle.schedulerID : "")
        + "，connect_generation="
        + connectGeneration
        + "，expected_generation="
        + expectedGeneration
        + "，heartbeat_generation="
        + currentHeartbeatGeneration
      );
      if (!heartbeatStarted || currentHeartbeatGeneration !== heartbeatGeneration || expectedGeneration !== connectGeneration) {
        return;
      }
      if (!isHeartbeatLeaseActive(heartbeatLeaseToken)) {
        logger.warn(
          "检测到心跳租约已失效，停止旧心跳链路，scheduler_id="
          + (heartbeatHandle ? heartbeatHandle.schedulerID : "")
          + "，connect_generation="
          + connectGeneration
          + "，heartbeat_generation="
          + currentHeartbeatGeneration
        );
        stopHeartbeat();
        return;
      }
      try {
        sendHeartbeat("scheduler_tick");
      } catch (error) {
        logger.error("WebSocket heartbeat 发送失败：" + String(error));
        closeSocketQuietly(1001, "heartbeat send failed");
        handleSocketClosed(expectedGeneration, "heartbeat_send_failed " + String(error));
      }
      }
    });
    logger.info(
      "已启动心跳调度器，requested_mode="
      + heartbeatSchedulerMode
      + "，actual_mode="
      + heartbeatHandle.kind
      + "，scheduler_id="
      + heartbeatHandle.schedulerID
      + "，lease_token="
      + heartbeatLeaseToken
      + "，connect_generation="
      + connectGeneration
      + "，heartbeat_generation="
      + currentHeartbeatGeneration
      + "，interval_ms="
      + heartbeatIntervalMS
    );
  }

  function executeTask(taskSummary: TaskSummary): void {
    if (taskExecuting) {
      logger.warn("当前已有任务在执行，忽略新的 assign_task：" + String(taskSummary.task_id || ""));
      return;
    }

    taskExecuting = true;
    try {
      sendUnifiedProgress(taskSummary, {
        status: "running",
        step_name: "INIT_TASK",
        message: "任务执行中：初始化任务执行上下文"
      });

      const result = taskRunner.runTask(taskSummary, {
        deviceID,
        agentUUID,
        centerBaseURL: options.centerBaseURL,
        logger,
        onProgress(progress) {
          sendUnifiedProgress(taskSummary, progress);
        }
      });
      sendTaskResult(taskSummary, result);
    } catch (error) {
      logger.error("本地任务执行失败：" + String(error));
      sendUnifiedProgress(taskSummary, {
        status: "failed",
        step_name: "RUN_TASK",
        message: "任务执行中：执行器抛出异常",
        extra: {
          error: String(error)
        }
      });
      sendTaskResult(taskSummary, {
        status: "failed",
        result_code: "RUNNER_EXCEPTION",
        result_message: String(error),
        step_name: "RUN_TASK",
        extra: {
          mode: "builtin"
        }
      });
    } finally {
      taskExecuting = false;
    }
  }

  function executeScriptSync(syncPayload?: Record<string, unknown>): void {
    const payload = syncPayload || {};
    try {
      const result = taskRunner.syncScriptVersion(String(payload.script_name || ""), String(payload.script_version || ""), {
        centerBaseURL: options.centerBaseURL,
        force: payload.force === true,
        logger
      });
      sendScriptSyncResult(payload, {
        status: "success",
        result_code: "OK",
        result_message: "脚本同步成功",
        extra: result
      });
    } catch (error) {
      logger.error("脚本同步失败：" + String(error));
      sendScriptSyncResult(payload, {
        status: "failed",
        result_code: "SCRIPT_SYNC_FAILED",
        result_message: String(error),
        extra: {}
      });
    }
  }

  function executeWorkflowSession(sessionPayload?: Record<string, unknown>): void {
    const payload = (sessionPayload || {}) as WorkflowSessionPayload & Record<string, unknown>;
    const refs = buildWorkflowSessionRefs(payload);
    const sessionKey = String(refs.plan_device_run_id || "");

    if (workflowSessionExecuting) {
      logger.warn("当前已有工作流会话在执行，忽略新的 start_workflow_session: " + String(sessionKey || ""));
      return;
    }

    workflowSessionExecuting = true;
    currentWorkflowRunID = sessionKey;
    clearWorkflowStopRequested(sessionKey);
    clearWorkflowResultSent(sessionKey);
    try {
      const result = workflowSessionRunner.runSession(payload, {
        deviceID,
        agentUUID,
        centerBaseURL: options.centerBaseURL,
        logger,
        isCancelled() {
          return isWorkflowStopRequested(sessionKey);
        },
        sendEvent(eventPayload) {
          sendWorkflowSessionEvent(eventPayload);
        }
      });
      sendWorkflowSessionResult({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String((result && result.workflow_node_id) || ""),
        status: String((result && result.status) || "failed"),
        result_code: String((result && result.result_code) || ""),
        result_message: String((result && result.result_message) || ""),
        extra: (result && result.extra) || {}
      });
    } catch (error) {
      logger.error("本地工作流会话执行失败：" + String(error));
      sendWorkflowSessionResult({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: "",
        status: "failed",
        result_code: "WORKFLOW_SESSION_EXCEPTION",
        result_message: String(error),
        extra: {}
      });
    } finally {
      clearWorkflowStopRequested(sessionKey);
      currentWorkflowRunID = "";
      workflowSessionExecuting = false;
    }
  }

  function scheduleTaskExecution(taskSummary: TaskSummary): void {
    runtime.runAsync(function runTaskAsync() {
      executeTask(taskSummary);
    });
  }

  function scheduleScriptSync(syncPayload?: Record<string, unknown>): void {
    runtime.runAsync(function runScriptSyncAsync() {
      executeScriptSync(syncPayload);
    });
  }

  function scheduleWorkflowSession(sessionPayload?: Record<string, unknown>): void {
    runtime.runAsync(function runWorkflowAsync() {
      executeWorkflowSession(sessionPayload);
    });
  }

  function handleAssignTask(message: GenericMessage): void {
    const payload = message && message.payload ? message.payload : {};
    const summary = buildTaskSummary(payload);
    logger.info("收到 assign_task：" + JSON.stringify(summary));
    if (onAssignTask) {
      try {
        onAssignTask(summary);
      } catch (error) {
        logger.warn("assign_task 回调执行失败：" + String(error));
      }
    }
    sendTaskAck(payload);
    scheduleTaskExecution(summary);
  }

  function handleSyncScript(message: GenericMessage): void {
    const payload = message && message.payload ? message.payload : {};
    logger.info("收到 sync_script：" + JSON.stringify(payload));
    sendScriptSyncAck(payload);
    scheduleScriptSync(payload);
  }

  function handleStartWorkflowSession(message: GenericMessage): void {
    const payload = message && message.payload ? message.payload : {};
    const refs = buildWorkflowSessionRefs(payload);
    logger.info("收到 start_workflow_session：" + JSON.stringify({
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      workflow_def_id: payload.workflow_def_id || "",
      entry_node_id: payload.entry_node_id || ""
    }));
    sendWorkflowSessionAck(payload);
    scheduleWorkflowSession(payload);
  }

  function handleStopWorkflowSession(message: GenericMessage): void {
    const payload = message && message.payload ? message.payload : {};
    const refs = buildWorkflowSessionRefs(payload);
    const sessionKey = String(refs.plan_device_run_id || "");
    logger.info("收到 stop_workflow_session：" + JSON.stringify({
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      reason: String(payload.reason || "")
    }));
    markWorkflowStopRequested(sessionKey);
    if (!workflowSessionExecuting || currentWorkflowRunID !== sessionKey) {
      sendWorkflowSessionResult({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: "",
        status: "stopped",
        result_code: "WORKFLOW_SESSION_STOPPED",
        result_message: "工作流会话已停止",
        extra: {
          reason: String(payload.reason || "")
        }
      });
      clearWorkflowStopRequested(sessionKey);
    }
  }

  function createListener(expectedGeneration: number): any {
    return createWebSocketListener({
      onOpen(_webSocket: any, _response: any) {
        if (expectedGeneration !== connectGeneration) {
          _webSocket.close(1000, "stale generation");
          return;
        }
        socket = _webSocket;
        closedGeneration = 0;
        lastReceiveAt = Date.now();
        logger.info("WebSocket 已连接：" + wsURL);
        try {
          send("hello", createRequestID("agent-hello"), {
            agent_uuid: agentUUID,
            device_link_sn: deviceLinkSN,
            execution_profile: runtime.collectExecutionProfile()
          });
        } catch (error) {
          logger.error("WebSocket hello 发送失败：" + String(error));
          closeSocketQuietly(1001, "hello send failed");
          handleSocketClosed(expectedGeneration, "hello_send_failed " + String(error));
        }
      },
      onMessage(_webSocket: any, text: unknown) {
        if (expectedGeneration !== connectGeneration) {
          return;
        }
        lastReceiveAt = Date.now();
        logger.info("收到 WebSocket 消息：" + String(text));
        try {
          const message = JSON.parse(String(text)) as GenericMessage;
          if (isOkAck(message, "hello")) {
            logger.info("WebSocket hello 已确认。");
            resetReconnectState();
            startHeartbeat(expectedGeneration);
          } else if (isOkAck(message, "heartbeat")) {
            logger.info("WebSocket heartbeat 已确认。");
          } else if (isOkAck(message, "task_ack")) {
            logger.info("WebSocket task_ack 已确认。");
          } else if (isOkAck(message, "task_progress")) {
            logger.info("WebSocket task_progress 已确认。");
          } else if (isOkAck(message, "script_sync_ack")) {
            logger.info("WebSocket script_sync_ack 已确认。");
          } else if (isOkAck(message, "script_sync_result")) {
            logger.info("WebSocket script_sync_result 已确认。");
          } else if (isOkAck(message, "task_result")) {
            logger.info("WebSocket task_result 已确认。");
          } else if (isOkAck(message, "workflow_session_ack")) {
            logger.info("WebSocket workflow_session_ack 已确认。");
          } else if (isOkAck(message, "workflow_session_event")) {
            logger.info("WebSocket workflow_session_event 已确认。");
          } else if (isOkAck(message, "workflow_session_result")) {
            logger.info("WebSocket workflow_session_result 已确认。");
          } else if (message && message.type === "assign_task") {
            handleAssignTask(message);
          } else if (message && message.type === "start_workflow_session") {
            handleStartWorkflowSession(message);
          } else if (message && message.type === "stop_workflow_session") {
            handleStopWorkflowSession(message);
          } else if (message && message.type === "sync_script") {
            handleSyncScript(message);
          }
        } catch (error) {
          logger.warn("WebSocket 消息解析失败：" + String(error));
        }
      },
      onClosing(_webSocket: any, code: number, reason: unknown) {
        if (expectedGeneration !== connectGeneration) {
          return;
        }
        logger.warn("WebSocket 正在关闭，code=" + code + "，reason=" + String(reason));
        _webSocket.close(code, reason);
      },
      onClosed(_webSocket: any, code: number, reason: unknown) {
        if (expectedGeneration !== connectGeneration) {
          return;
        }
        logger.warn("WebSocket 已关闭，code=" + code + "，reason=" + String(reason));
        handleSocketClosed(expectedGeneration, "closed code=" + code + " reason=" + String(reason));
      },
      onFailure(_webSocket: any, throwable: unknown, _response: any) {
        if (expectedGeneration !== connectGeneration) {
          return;
        }
        logger.error("WebSocket 连接失败：" + String(throwable));
        handleSocketClosed(expectedGeneration, "failure " + String(throwable));
      }
    });
  }

  function openSocket(): void {
    const request = new Request.Builder().url(wsURL).build();
    connectGeneration += 1;
    closedGeneration = 0;
    socket = null;
    lastReceiveAt = Date.now();
    lastSendAt = Date.now();
    logger.info("开始连接 WebSocket：" + wsURL + "，connect_generation=" + connectGeneration);
    client.newWebSocket(request, createListener(connectGeneration));
  }

  startWatchdog();
  openSocket();

  return {
    url: wsURL,
    close(): void {
      intentionallyClosed = true;
      stopHeartbeat();
      stopReconnect();
      stopWatchdog();
      if (socket) {
        socket.close(1000, "agent close");
      }
      socket = null;
    }
  };
}

function connect(options: WSClientOptions): WebSocketHandle {
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const config = options || ({} as WSClientOptions);

  if (!config.deviceID) {
    throw new Error("WebSocket 连接缺少 device_id。");
  }

  if (runtime.isAutoJsRuntime()) {
    return connectAutoJs(config);
  }

  logger.warn("当前运行环境不是 AutoJs6，已跳过 WebSocket 长连接。");
  return {
    url: buildWebSocketURL(config.centerBaseURL),
    skipped: true
  };
}

export {
  buildWebSocketURL,
  createEnvelope,
  connect,
  createSessionFlagStore,
  buildTaskSummary,
  buildWorkflowSessionRefs
};

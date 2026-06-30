"use strict";

var runtime = require("./runtime");
var taskRunner = require("./task_runner");
var workflowSessionRunner = require("./workflow_session_runner");

function trimBaseURL(baseURL) {
    return String(baseURL || "").replace(/\/+$/, "");
}

function buildWebSocketURL(centerBaseURL) {
    var base = trimBaseURL(centerBaseURL);
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

function createRequestID(prefix) {
    return String(prefix || "request") + "-" + Date.now() + "-" + Math.floor(Math.random() * 100000);
}

function createEnvelope(type, requestID, deviceID, payload) {
    return {
        type: type,
        request_id: requestID,
        device_id: deviceID,
        timestamp: Math.floor(Date.now() / 1000),
        payload: payload || {}
    };
}

function isOkAck(message, messageType) {
    return !!(message &&
        message.type === "ack" &&
        message.payload &&
        message.payload.message_type === messageType &&
        message.payload.status === "ok");
}

function javaType(name) {
    if (typeof Java !== "undefined" && typeof Java.type === "function") {
        return Java.type(name);
    }

    var parts = String(name || "").split(".");
    var current = Packages;
    var index = 0;

    for (index = 0; index < parts.length; index += 1) {
        current = current[parts[index]];
    }

    return current;
}

function createWebSocketListener(callbacks) {
    var WebSocketListener = javaType("okhttp3.WebSocketListener");
    if (typeof JavaAdapter === "function") {
        return new JavaAdapter(WebSocketListener, callbacks);
    }
    return new WebSocketListener(callbacks);
}

function createRunnable(runCallback) {
    var Runnable = javaType("java.lang.Runnable");
    if (typeof JavaAdapter === "function") {
        return new JavaAdapter(Runnable, {
            run: runCallback
        });
    }
    return new Runnable({
        run: runCallback
    });
}

function buildTaskSummary(taskPayload) {
    var payload = taskPayload || {};
    return {
        task_id: String(payload.task_id || ""),
        script_name: String(payload.script_name || ""),
        script_version: String(payload.script_version || ""),
        priority: Number(payload.priority || 0),
        params: payload.params || {}
    };
}

function buildWorkflowSessionRefs(payload) {
    var sessionPayload = payload || {};
    return {
        plan_run_id: String(sessionPayload.plan_run_id || ""),
        plan_device_run_id: String(sessionPayload.plan_device_run_id || "")
    };
}

function connectAutoJs(options) {
    var logger = options.logger || runtime.createLogger();
    var OkHttpClient = javaType("okhttp3.OkHttpClient");
    var Request = javaType("okhttp3.Request");
    var TimeUnit = javaType("java.util.concurrent.TimeUnit");
    var Executors = javaType("java.util.concurrent.Executors");
    var heartbeatExecutor = null;
    var heartbeatFuture = null;
    var reconnectExecutor = null;
    var reconnectFuture = null;
    var wsURL = buildWebSocketURL(options.centerBaseURL);
    var heartbeatIntervalMS = Number(options.heartbeatIntervalMS || 30000);
    var reconnectEnabled = options.reconnectEnabled !== false;
    var reconnectInitialDelayMS = Number(options.reconnectInitialDelayMS || 3000);
    var reconnectMaxDelayMS = Number(options.reconnectMaxDelayMS || 60000);
    var reconnectBackoffMultiplier = Number(options.reconnectBackoffMultiplier || 2);
    var deviceID = String(options.deviceID || "");
    var agentUUID = String(options.agentUUID || "");
    var onAssignTask = typeof options.onAssignTask === "function" ? options.onAssignTask : null;
    var heartbeatStarted = false;
    var reconnectAttempt = 0;
    var connectGeneration = 0;
    var closedGeneration = 0;
    var intentionallyClosed = false;
    var taskExecuting = false;
    var workflowSessionExecuting = false;
    var workflowStopFlags = {};
    var currentWorkflowRunID = "";
    var client = new OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .build();
    var socket = null;

    function send(type, requestID, payload) {
        if (!socket) {
            throw new Error("websocket_not_connected");
        }
        var message = createEnvelope(type, requestID, deviceID, payload);
        var text = JSON.stringify(message);
        logger.info("发送 WebSocket 消息：" + type + "，request_id=" + requestID);
        if (socket.send(text) === false) {
            throw new Error("websocket_send_failed");
        }
    }

    function sendHeartbeat() {
        send("heartbeat", createRequestID("agent-heartbeat"), {
            agent_uuid: agentUUID,
            execution_profile: runtime.collectExecutionProfile()
        });
    }

    function sendTaskAck(taskPayload) {
        var summary = buildTaskSummary(taskPayload);
        send("task_ack", createRequestID("agent-task-ack"), {
            task_id: summary.task_id,
            status: "ok",
            message: "Agent 已收到任务，准备执行",
            script_name: summary.script_name,
            script_version: summary.script_version
        });
        logger.info("已发送 task_ack：" + summary.task_id);
    }

    function sendTaskResult(taskSummary, result) {
        var summary = taskSummary || {};
        var payload = result || {};
        send("task_result", createRequestID("agent-task-result"), {
            task_id: String(summary.task_id || ""),
            status: String(payload.status || "failed"),
            result_code: String(payload.result_code || ""),
            result_message: String(payload.result_message || ""),
            step_name: String(payload.step_name || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 task_result：" + summary.task_id + " -> " + String(payload.status || "failed"));
    }

    function sendTaskProgress(taskSummary, progress) {
        var summary = taskSummary || {};
        var payload = progress || {};
        send("task_progress", createRequestID("agent-task-progress"), {
            task_id: String(summary.task_id || ""),
            status: String(payload.status || "running"),
            step_name: String(payload.step_name || ""),
            message: String(payload.message || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 task_progress：" + summary.task_id + " -> " + String(payload.step_name || ""));
    }

    function sendUnifiedProgress(taskSummary, progress) {
        sendTaskProgress(taskSummary, progress);
    }

    function sendScriptSyncAck(syncPayload) {
        var payload = syncPayload || {};
        send("script_sync_ack", createRequestID("agent-script-sync-ack"), {
            script_name: String(payload.script_name || ""),
            script_version: String(payload.script_version || ""),
            status: "ok",
            message: "Agent 已收到脚本同步指令"
        });
        logger.info("已发送 script_sync_ack：" + String(payload.script_name || "") + "@" + String(payload.script_version || ""));
    }

    function sendScriptSyncResult(syncPayload, result) {
        var payload = syncPayload || {};
        var summary = result || {};
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

    function sendWorkflowSessionAck(sessionPayload) {
        var payload = sessionPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
        send("workflow_session_ack", createRequestID("agent-workflow-session-ack"), {
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            status: "ok",
            message: "Agent 已收到工作流会话"
        });
        logger.info("已发送 workflow_session_ack：" + String(refs.plan_device_run_id || ""));
    }

    function sendWorkflowSessionEvent(eventPayload) {
        var payload = eventPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
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
    }

    function sendWorkflowSessionResult(resultPayload) {
        var payload = resultPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
        send("workflow_session_result", createRequestID("agent-workflow-session-result"), {
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(payload.workflow_node_id || ""),
            status: String(payload.status || "failed"),
            result_code: String(payload.result_code || ""),
            result_message: String(payload.result_message || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 workflow_session_result：" + String(refs.plan_device_run_id || "") + " -> " + String(payload.status || "failed"));
    }

    function markWorkflowStopRequested(sessionKey) {
        sessionKey = String(sessionKey || "").trim();
        if (!sessionKey) {
            return;
        }
        workflowStopFlags[sessionKey] = true;
    }

    function clearWorkflowStopRequested(sessionKey) {
        sessionKey = String(sessionKey || "").trim();
        if (!sessionKey) {
            return;
        }
        delete workflowStopFlags[sessionKey];
    }

    function isWorkflowStopRequested(sessionKey) {
        sessionKey = String(sessionKey || "").trim();
        if (!sessionKey) {
            return false;
        }
        return workflowStopFlags[sessionKey] === true;
    }

    function stopHeartbeat() {
        if (heartbeatFuture) {
            heartbeatFuture.cancel(false);
            heartbeatFuture = null;
        }
        if (heartbeatExecutor) {
            heartbeatExecutor.shutdownNow();
            heartbeatExecutor = null;
        }
        heartbeatStarted = false;
    }

    function stopReconnect() {
        if (reconnectFuture) {
            reconnectFuture.cancel(false);
            reconnectFuture = null;
        }
        if (reconnectExecutor) {
            reconnectExecutor.shutdownNow();
            reconnectExecutor = null;
        }
    }

    function resetReconnectState() {
        reconnectAttempt = 0;
        stopReconnect();
    }

    function sanitizeReconnectDelay(value, fallback) {
        var nextValue = Number(value || fallback);
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

    function getReconnectDelayMS(attempt) {
        var delay = reconnectInitialDelayMS;
        var currentAttempt = Number(attempt || 1);
        var index = 1;

        while (index < currentAttempt) {
            delay = Math.min(reconnectMaxDelayMS, delay * reconnectBackoffMultiplier);
            index += 1;
        }

        return Math.min(delay, reconnectMaxDelayMS);
    }

    function closeSocketQuietly(closeCode, closeReason) {
        if (!socket) {
            return;
        }
        try {
            socket.close(closeCode || 1000, closeReason || "close");
        } catch (error) {
            logger.warn("WebSocket 关闭时出现异常：" + String(error));
        }
    }

    function scheduleReconnect(reason) {
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

        reconnectAttempt += 1;
        var delayMS = getReconnectDelayMS(reconnectAttempt);
        logger.warn("WebSocket 连接中断，准备第 " + reconnectAttempt + " 次重连，delay=" + delayMS + "ms，原因：" + String(reason || ""));

        reconnectExecutor = Executors.newSingleThreadScheduledExecutor();
        reconnectFuture = reconnectExecutor.schedule(createRunnable(function () {
            reconnectFuture = null;
            if (reconnectExecutor) {
                reconnectExecutor.shutdownNow();
                reconnectExecutor = null;
            }
            if (intentionallyClosed) {
                logger.info("WebSocket 在重连等待期间被主动关闭，取消重连。");
                return;
            }
            logger.info("WebSocket 开始执行第 " + reconnectAttempt + " 次重连：" + wsURL);
            openSocket();
        }), delayMS, TimeUnit.MILLISECONDS);
    }

    function handleSocketClosed(expectedGeneration, reason) {
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

    function startHeartbeat(expectedGeneration) {
        if (heartbeatStarted) {
            return;
        }
        heartbeatStarted = true;
        sendHeartbeat();
        heartbeatExecutor = Executors.newSingleThreadScheduledExecutor();
        heartbeatFuture = heartbeatExecutor.scheduleAtFixedRate(createRunnable(function () {
            try {
                sendHeartbeat();
            } catch (error) {
                logger.error("WebSocket heartbeat 发送失败：" + String(error));
                closeSocketQuietly(1001, "heartbeat send failed");
                handleSocketClosed(expectedGeneration, "heartbeat_send_failed " + String(error));
            }
        }), heartbeatIntervalMS, heartbeatIntervalMS, TimeUnit.MILLISECONDS);
    }

    function executeTask(taskSummary) {
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

            var result = taskRunner.runTask(taskSummary, {
                deviceID: deviceID,
                agentUUID: agentUUID,
                centerBaseURL: options.centerBaseURL,
                logger: logger,
                onProgress: function (progress) {
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

    function executeScriptSync(syncPayload) {
        var payload = syncPayload || {};
        try {
            var result = taskRunner.syncScriptVersion(payload.script_name, payload.script_version, {
                centerBaseURL: options.centerBaseURL,
                force: payload.force === true,
                logger: logger
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

    function executeWorkflowSession(sessionPayload) {
        var payload = sessionPayload || {};
        var result = null;
        var refs = buildWorkflowSessionRefs(payload);
        var sessionKey = String(refs.plan_device_run_id || "");

        if (workflowSessionExecuting) {
            logger.warn("当前已有工作流会话在执行，忽略新的 start_workflow_session: " + String(sessionKey || ""));
            return;
        }

        workflowSessionExecuting = true;
        currentWorkflowRunID = sessionKey;
        clearWorkflowStopRequested(sessionKey);
        try {
            result = workflowSessionRunner.runSession(payload, {
                deviceID: deviceID,
                agentUUID: agentUUID,
                centerBaseURL: options.centerBaseURL,
                logger: logger,
                isCancelled: function () {
                    return isWorkflowStopRequested(sessionKey);
                },
                sendEvent: function (eventPayload) {
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
                extra: {}
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

    function scheduleTaskExecution(taskSummary) {
        runtime.runAsync(function () {
            executeTask(taskSummary);
        });
    }

    function scheduleScriptSync(syncPayload) {
        runtime.runAsync(function () {
            executeScriptSync(syncPayload);
        });
    }

    function scheduleWorkflowSession(sessionPayload) {
        runtime.runAsync(function () {
            executeWorkflowSession(sessionPayload);
        });
    }

    function handleAssignTask(message) {
        var payload = message && message.payload ? message.payload : {};
        var summary = buildTaskSummary(payload);
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

    function handleSyncScript(message) {
        var payload = message && message.payload ? message.payload : {};
        logger.info("收到 sync_script：" + JSON.stringify(payload));
        sendScriptSyncAck(payload);
        scheduleScriptSync(payload);
    }

    function handleStartWorkflowSession(message) {
        var payload = message && message.payload ? message.payload : {};
        var refs = buildWorkflowSessionRefs(payload);
        logger.info("收到 start_workflow_session：" + JSON.stringify({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_def_id: payload.workflow_def_id || "",
            entry_node_id: payload.entry_node_id || ""
        }));
        sendWorkflowSessionAck(payload);
        scheduleWorkflowSession(payload);
    }

    function handleStopWorkflowSession(message) {
        var payload = message && message.payload ? message.payload : {};
        var refs = buildWorkflowSessionRefs(payload);
        var sessionKey = String(refs.plan_device_run_id || "");
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

    function createListener(expectedGeneration) {
        return createWebSocketListener({
            onOpen: function (_webSocket, _response) {
                if (expectedGeneration !== connectGeneration) {
                    _webSocket.close(1000, "stale generation");
                    return;
                }
                socket = _webSocket;
                closedGeneration = 0;
                logger.info("WebSocket 已连接：" + wsURL);
                try {
                    send("hello", createRequestID("agent-hello"), {
                        agent_uuid: agentUUID,
                        execution_profile: runtime.collectExecutionProfile()
                    });
                } catch (error) {
                    logger.error("WebSocket hello 发送失败：" + String(error));
                    closeSocketQuietly(1001, "hello send failed");
                    handleSocketClosed(expectedGeneration, "hello_send_failed " + String(error));
                }
            },
            onMessage: function (_webSocket, text) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.info("收到 WebSocket 消息：" + text);
                try {
                    var message = JSON.parse(String(text));
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
            onClosing: function (_webSocket, code, reason) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.warn("WebSocket 正在关闭，code=" + code + "，reason=" + reason);
                _webSocket.close(code, reason);
            },
            onClosed: function (_webSocket, code, reason) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.warn("WebSocket 已关闭，code=" + code + "，reason=" + reason);
                handleSocketClosed(expectedGeneration, "closed code=" + code + " reason=" + reason);
            },
            onFailure: function (_webSocket, throwable, _response) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.error("WebSocket 连接失败：" + String(throwable));
                handleSocketClosed(expectedGeneration, "failure " + String(throwable));
            }
        });
    }

    function openSocket() {
        var request = new Request.Builder().url(wsURL).build();
        connectGeneration += 1;
        closedGeneration = 0;
        socket = null;
        logger.info("开始连接 WebSocket：" + wsURL);
        client.newWebSocket(request, createListener(connectGeneration));
    }

    openSocket();

    return {
        url: wsURL,
        close: function () {
            intentionallyClosed = true;
            stopHeartbeat();
            stopReconnect();
            if (socket) {
                socket.close(1000, "agent close");
            }
            socket = null;
        }
    };
}

function connect(options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var config = options || {};

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

module.exports = {
    buildWebSocketURL: buildWebSocketURL,
    createEnvelope: createEnvelope,
    connect: connect
};

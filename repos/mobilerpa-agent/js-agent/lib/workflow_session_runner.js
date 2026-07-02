"use strict";

var runtime = require("./runtime");
var taskRunner = require("./task_runner");
var NODE_TRANSITION_DELAY_MS = 2000;

function sleepMilliseconds(durationMs) {
    var startedAt = new Date().getTime();

    while (new Date().getTime() - startedAt < durationMs) {
        java.lang.Thread.sleep(100);
    }
}

function buildSessionRefs(sessionPayload) {
    return {
        plan_run_id: String((sessionPayload && sessionPayload.plan_run_id) || ""),
        plan_device_run_id: String((sessionPayload && sessionPayload.plan_device_run_id) || "")
    };
}

function pauseBeforeNextNode(sessionPayload, fromNodeID, toNodeID, options) {
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function () {};
    var isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function () { return false; };
    var refs = buildSessionRefs(sessionPayload);

    if (!toNodeID || isCancelled()) {
        return;
    }

    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(fromNodeID || ""),
        event_type: "workflow_session_progress",
        status: "running",
        step_name: "NODE_TRANSITION_WAIT",
        message: "节点切换前等待 2 秒",
        extra: {
            from_node_id: String(fromNodeID || ""),
            to_node_id: String(toNodeID || ""),
            delay_ms: NODE_TRANSITION_DELAY_MS
        }
    });

    sleepMilliseconds(NODE_TRANSITION_DELAY_MS);
}

function syncSessionScripts(sessionPayload, options) {
    var manifests = sessionPayload && sessionPayload.script_manifest ? sessionPayload.script_manifest : [];
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function () {};
    var centerBaseURL = String((options && options.centerBaseURL) || "");
    var refs = buildSessionRefs(sessionPayload);
    var index = 0;
    var item = null;

    if (!centerBaseURL || manifests.length === 0) {
        return;
    }

    for (index = 0; index < manifests.length; index += 1) {
        item = manifests[index] || {};
        sendEvent({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: "",
            event_type: "workflow_session_progress",
            status: "running",
            step_name: "SYNC_SCRIPT",
            message: "工作流执行前校验脚本版本",
            extra: {
                script_name: String(item.script_name || ""),
                script_version: String(item.script_version || "")
            }
        });

        taskRunner.ensureScriptVersion(item.script_name, item.script_version, {
            centerBaseURL: centerBaseURL,
            logger: logger,
            force: false
        });
    }
}

function buildNodeMap(snapshot) {
    var map = {};
    var nodes = snapshot && snapshot.nodes ? snapshot.nodes : [];
    var index = 0;

    for (index = 0; index < nodes.length; index += 1) {
        map[String(nodes[index].node_id || "")] = nodes[index];
    }

    return map;
}

function buildEdgeMap(snapshot) {
    var map = {};
    var edges = snapshot && snapshot.edges ? snapshot.edges : [];
    var index = 0;
    var key = "";

    for (index = 0; index < edges.length; index += 1) {
        key = String(edges[index].from_node_id || "") + "::" + String(edges[index].edge_type || "next");
        map[key] = String(edges[index].to_node_id || "");
    }

    return map;
}

function getNextNodeID(edgeMap, fromNodeID, edgeType) {
    return String(edgeMap[String(fromNodeID || "") + "::" + String(edgeType || "next")] || "");
}

function createTaskSummaryFromNode(sessionPayload, node) {
    var refs = buildSessionRefs(sessionPayload);
    return {
        task_id: "workflow-session-" + String(refs.plan_device_run_id || "") + "-" + String(node.node_id || ""),
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(node.node_id || ""),
        script_name: String(node.script_name || ""),
        script_version: String(node.script_version || ""),
        priority: 0,
        params: {
            workflow_session_id: String(sessionPayload.workflow_session_id || ""),
            workflow_def_id: String(sessionPayload.workflow_def_id || "")
        }
    };
}

function runScriptNode(sessionPayload, node, options) {
    var config = options || {};
    var logger = config.logger || runtime.createLogger();
    var sendEvent = typeof config.sendEvent === "function" ? config.sendEvent : function () {};
    var isCancelled = typeof config.isCancelled === "function" ? config.isCancelled : function () { return false; };
    var taskSummary = createTaskSummaryFromNode(sessionPayload, node);
    var refs = buildSessionRefs(sessionPayload);
    var result = null;

    if (isCancelled()) {
        return {
            status: "stopped",
            result_code: "WORKFLOW_SESSION_STOPPED",
            result_message: "工作流会话已停止",
            workflow_node_id: String(node.node_id || "")
        };
    }

    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(node.node_id || ""),
        event_type: "workflow_step_started",
        status: "running",
        step_name: "START_NODE",
        message: "工作流步骤开始执行",
        extra: {
            node_name: String(node.node_name || ""),
            script_name: String(node.script_name || ""),
            script_version: String(node.script_version || "")
        }
    });

    result = taskRunner.runTask(taskSummary, {
        deviceID: String(config.deviceID || ""),
        agentUUID: String(config.agentUUID || ""),
        centerBaseURL: String(config.centerBaseURL || ""),
        logger: logger,
        onProgress: function (progress) {
            sendEvent({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: String(node.node_id || ""),
                event_type: "workflow_session_progress",
                status: String((progress && progress.status) || "running"),
                step_name: String((progress && progress.step_name) || ""),
                message: String((progress && progress.message) || ""),
                extra: (progress && progress.extra) || {}
            });
        }
    });

    if (result && String(result.status || "") === "success") {
        sendEvent({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(node.node_id || ""),
            event_type: "workflow_step_succeeded",
            status: "success",
            step_name: String(result.step_name || "COMPLETE"),
            message: String(result.result_message || "工作流步骤执行成功"),
            extra: {
                result_code: String(result.result_code || "")
            }
        });
        return result;
    }

    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(node.node_id || ""),
        event_type: "workflow_step_failed",
        status: "failed",
        step_name: String((result && result.step_name) || "RUN_NODE"),
        message: String((result && result.result_message) || "工作流步骤执行失败"),
        extra: {
            result_code: String((result && result.result_code) || "")
        }
    });
    return result;
}

function runSession(sessionPayload, options) {
    var snapshot = sessionPayload && sessionPayload.definition_snapshot ? sessionPayload.definition_snapshot : {};
    var nodeMap = buildNodeMap(snapshot);
    var edgeMap = buildEdgeMap(snapshot);
    var currentNodeID = String(sessionPayload.entry_node_id || "");
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function () {};
    var isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function () { return false; };
    var refs = buildSessionRefs(sessionPayload);
    var loopCounters = {};
    var node = null;
    var nextNodeID = "";
    var result = null;

    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: currentNodeID,
        event_type: "workflow_run_started",
        status: "running",
        step_name: "START_SESSION",
        message: "工作流运行已启动",
        extra: {
            workflow_name: String(sessionPayload.workflow_name || "")
        }
    });

    syncSessionScripts(sessionPayload, {
        centerBaseURL: options && options.centerBaseURL,
        logger: logger,
        sendEvent: sendEvent
    });

    while (currentNodeID) {
        if (isCancelled()) {
            return {
                status: "stopped",
                result_code: "WORKFLOW_SESSION_STOPPED",
                result_message: "工作流会话已停止",
                workflow_node_id: currentNodeID
            };
        }

        node = nodeMap[currentNodeID];
        if (!node) {
            return {
                status: "failed",
                result_code: "WORKFLOW_NODE_NOT_FOUND",
                result_message: "未找到工作流节点: " + currentNodeID,
                workflow_node_id: currentNodeID
            };
        }

        if (String(node.node_type || "") === "script") {
            result = runScriptNode(sessionPayload, node, {
                deviceID: options && options.deviceID,
                agentUUID: options && options.agentUUID,
                centerBaseURL: options && options.centerBaseURL,
                logger: logger,
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            if (!result || String(result.status || "") !== "success") {
                return {
                    status: String((result && result.status) || "failed"),
                    result_code: String((result && result.result_code) || "WORKFLOW_STEP_FAILED"),
                    result_message: String((result && result.result_message) || "工作流步骤执行失败"),
                    workflow_node_id: currentNodeID
                };
            }
            nextNodeID = getNextNodeID(edgeMap, currentNodeID, "next");
            pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            currentNodeID = nextNodeID;
            continue;
        }

        if (String(node.node_type || "") === "loop") {
            loopCounters[currentNodeID] = Number(loopCounters[currentNodeID] || 0);
            if (Number(node.max_iterations || 0) > 0 && loopCounters[currentNodeID] >= Number(node.max_iterations || 0)) {
                nextNodeID = getNextNodeID(edgeMap, currentNodeID, "loop_exit") || getNextNodeID(edgeMap, currentNodeID, "next");
                pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
                    sendEvent: sendEvent,
                    isCancelled: isCancelled
                });
                currentNodeID = nextNodeID;
                continue;
            }

            loopCounters[currentNodeID] += 1;
            sendEvent({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: currentNodeID,
                event_type: "workflow_loop_completed",
                status: "running",
                step_name: "LOOP",
                message: "工作流循环节点已完成一轮",
                extra: {
                    counter: loopCounters[currentNodeID],
                    max_iterations: Number(node.max_iterations || 0)
                }
            });
            nextNodeID = getNextNodeID(edgeMap, currentNodeID, "loop_body") || getNextNodeID(edgeMap, currentNodeID, "next");
            pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            currentNodeID = nextNodeID;
            continue;
        }

        if (String(node.node_type || "") === "stop") {
            return {
                status: "success",
                result_code: "OK",
                result_message: "工作流运行已完成",
                workflow_node_id: currentNodeID
            };
        }

        logger.error("不支持的工作流节点类型: " + String(node.node_type || ""));
        return {
            status: "failed",
            result_code: "WORKFLOW_NODE_TYPE_UNSUPPORTED",
            result_message: "不支持的工作流节点类型: " + String(node.node_type || ""),
            workflow_node_id: currentNodeID
        };
    }

    return {
        status: "success",
        result_code: "OK",
        result_message: "工作流运行已完成",
        workflow_node_id: ""
    };
}

module.exports = {
    runSession: runSession
};

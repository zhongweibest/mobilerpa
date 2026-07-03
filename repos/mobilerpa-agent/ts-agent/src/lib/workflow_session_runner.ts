import * as runtime from "./runtime";
import * as taskRunner from "./task_runner";

import type { LoggerLike } from "../types/runtime";
import type {
  TaskResult,
  TaskSummary,
  WorkflowDefinitionSnapshot,
  WorkflowNode,
  WorkflowSessionPayload,
  WorkflowSessionRefs,
  WorkflowSessionResult
} from "../types/protocol";

const NODE_TRANSITION_DELAY_MS = 2000;

interface WorkflowRunnerOptions {
  deviceID?: string;
  agentUUID?: string;
  centerBaseURL?: string;
  logger?: LoggerLike;
  sendEvent?: (payload: Record<string, unknown>) => void;
  isCancelled?: () => boolean;
}

interface WorkflowRunState {
  status: string;
  result_code: string;
  result_message: string;
  workflow_node_id: string;
  extra?: Record<string, unknown>;
}

function sleepMilliseconds(durationMS: number): void {
  const startedAt = new Date().getTime();

  while (new Date().getTime() - startedAt < durationMS) {
    java.lang.Thread.sleep(100);
  }
}

function buildSessionRefs(sessionPayload?: WorkflowSessionPayload): WorkflowSessionRefs {
  return {
    plan_run_id: String((sessionPayload && sessionPayload.plan_run_id) || ""),
    plan_device_run_id: String((sessionPayload && sessionPayload.plan_device_run_id) || "")
  };
}

function pauseBeforeNextNode(
  sessionPayload: WorkflowSessionPayload,
  fromNodeID: string,
  toNodeID: string,
  options?: WorkflowRunnerOptions
): void {
  const sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop(): void {};
  const isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled(): boolean { return false; };
  const refs = buildSessionRefs(sessionPayload);

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

function syncSessionScripts(sessionPayload: WorkflowSessionPayload, options?: WorkflowRunnerOptions): void {
  const manifests = sessionPayload && sessionPayload.script_manifest ? sessionPayload.script_manifest : [];
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop(): void {};
  const centerBaseURL = String((options && options.centerBaseURL) || "");
  const isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled(): boolean { return false; };
  const refs = buildSessionRefs(sessionPayload);

  if (!centerBaseURL || manifests.length === 0) {
    return;
  }

  for (let index = 0; index < manifests.length; index += 1) {
    if (isCancelled()) {
      return;
    }

    const item = manifests[index] || {};
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

    taskRunner.ensureScriptVersion(String(item.script_name || ""), String(item.script_version || ""), {
      centerBaseURL,
      logger,
      force: false
    });
  }
}

function buildNodeMap(snapshot?: WorkflowDefinitionSnapshot): Record<string, WorkflowNode> {
  const map: Record<string, WorkflowNode> = {};
  const nodes = snapshot && snapshot.nodes ? snapshot.nodes : [];

  for (let index = 0; index < nodes.length; index += 1) {
    map[String(nodes[index].node_id || "")] = nodes[index];
  }

  return map;
}

function buildEdgeMap(snapshot?: WorkflowDefinitionSnapshot): Record<string, string> {
  const map: Record<string, string> = {};
  const edges = snapshot && snapshot.edges ? snapshot.edges : [];

  for (let index = 0; index < edges.length; index += 1) {
    const key = String(edges[index].from_node_id || "") + "::" + String(edges[index].edge_type || "next");
    map[key] = String(edges[index].to_node_id || "");
  }

  return map;
}

function getNextNodeID(edgeMap: Record<string, string>, fromNodeID: string, edgeType: string): string {
  return String(edgeMap[String(fromNodeID || "") + "::" + String(edgeType || "next")] || "");
}

function createTaskSummaryFromNode(sessionPayload: WorkflowSessionPayload, node: WorkflowNode): TaskSummary {
  const refs = buildSessionRefs(sessionPayload);
  return {
    task_id: "workflow-session-" + String(refs.plan_device_run_id || "") + "-" + String(node.node_id || ""),
    script_name: String(node.script_name || ""),
    script_version: String(node.script_version || ""),
    priority: 0,
    params: {
      workflow_session_id: String(sessionPayload.workflow_session_id || ""),
      workflow_def_id: String(sessionPayload.workflow_def_id || ""),
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      workflow_node_id: String(node.node_id || "")
    }
  };
}

function runScriptNode(
  sessionPayload: WorkflowSessionPayload,
  node: WorkflowNode,
  options?: WorkflowRunnerOptions
): TaskResult | WorkflowRunState {
  const config = options || {};
  const logger = config.logger || runtime.createLogger();
  const sendEvent = typeof config.sendEvent === "function" ? config.sendEvent : function noop(): void {};
  const isCancelled = typeof config.isCancelled === "function" ? config.isCancelled : function neverCancelled(): boolean { return false; };
  const taskSummary = createTaskSummaryFromNode(sessionPayload, node);
  const refs = buildSessionRefs(sessionPayload);

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

  const result = taskRunner.runTask(taskSummary, {
    deviceID: String(config.deviceID || ""),
    agentUUID: String(config.agentUUID || ""),
    centerBaseURL: String(config.centerBaseURL || ""),
    logger,
    isCancelled,
    onProgress(progress) {
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

  if (isCancelled()) {
    const actualStatus = String((result && result.status) || "");
    const actualResultCode = String((result && result.result_code) || "");
    const actualResultMessage = String((result && result.result_message) || "");
    const actualStepName = String((result && result.step_name) || "");
    const stoppedMessage = actualStatus === "success"
      ? "工作流会话已停止，脚本在停止后实际执行成功"
      : "工作流会话已停止，脚本在停止后实际执行失败";

    sendEvent({
      plan_run_id: refs.plan_run_id,
      plan_device_run_id: refs.plan_device_run_id,
      workflow_node_id: String(node.node_id || ""),
      event_type: "workflow_step_stopped",
      status: "stopped",
      step_name: actualStepName || "STOPPED",
      message: stoppedMessage,
      extra: {
        actual_status: actualStatus,
        actual_result_code: actualResultCode,
        actual_result_message: actualResultMessage
      }
    });

    return {
      status: "stopped",
      result_code: "WORKFLOW_SESSION_STOPPED",
      result_message: stoppedMessage,
      workflow_node_id: String(node.node_id || ""),
      extra: {
        actual_status: actualStatus,
        actual_result_code: actualResultCode,
        actual_result_message: actualResultMessage
      }
    };
  }

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

function runSession(sessionPayload: WorkflowSessionPayload, options?: WorkflowRunnerOptions): WorkflowRunState {
  const snapshot = sessionPayload && sessionPayload.definition_snapshot ? sessionPayload.definition_snapshot : {};
  const nodeMap = buildNodeMap(snapshot);
  const edgeMap = buildEdgeMap(snapshot);
  let currentNodeID = String(sessionPayload.entry_node_id || "");
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop(): void {};
  const isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled(): boolean { return false; };
  const refs = buildSessionRefs(sessionPayload);
  const loopCounters: Record<string, number> = {};

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
    logger,
    sendEvent,
    isCancelled
  });

  if (isCancelled()) {
    return {
      status: "stopped",
      result_code: "WORKFLOW_SESSION_STOPPED",
      result_message: "工作流会话已停止",
      workflow_node_id: currentNodeID
    };
  }

  while (currentNodeID) {
    if (isCancelled()) {
      return {
        status: "stopped",
        result_code: "WORKFLOW_SESSION_STOPPED",
        result_message: "工作流会话已停止",
        workflow_node_id: currentNodeID
      };
    }

    const node = nodeMap[currentNodeID];
    if (!node) {
      return {
        status: "failed",
        result_code: "WORKFLOW_NODE_NOT_FOUND",
        result_message: "未找到工作流节点: " + currentNodeID,
        workflow_node_id: currentNodeID
      };
    }

    if (String(node.node_type || "") === "script") {
      const result = runScriptNode(sessionPayload, node, {
        deviceID: options && options.deviceID,
        agentUUID: options && options.agentUUID,
        centerBaseURL: options && options.centerBaseURL,
        logger,
        sendEvent,
        isCancelled
      });

      if (!result || String(result.status || "") !== "success") {
        return {
          status: String((result && result.status) || "failed"),
          result_code: String((result && result.result_code) || "WORKFLOW_STEP_FAILED"),
          result_message: String((result && result.result_message) || "工作流步骤执行失败"),
          workflow_node_id: currentNodeID
        };
      }

      const nextNodeID = getNextNodeID(edgeMap, currentNodeID, "next");
      pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
        sendEvent,
        isCancelled
      });
      currentNodeID = nextNodeID;
      continue;
    }

    if (String(node.node_type || "") === "loop") {
      loopCounters[currentNodeID] = Number(loopCounters[currentNodeID] || 0);
      if (Number(node.max_iterations || 0) > 0 && loopCounters[currentNodeID] >= Number(node.max_iterations || 0)) {
        const nextNodeID = getNextNodeID(edgeMap, currentNodeID, "loop_exit") || getNextNodeID(edgeMap, currentNodeID, "next");
        pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
          sendEvent,
          isCancelled
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

      const nextNodeID = getNextNodeID(edgeMap, currentNodeID, "loop_body") || getNextNodeID(edgeMap, currentNodeID, "next");
      pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
        sendEvent,
        isCancelled
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

export {
  runSession
};

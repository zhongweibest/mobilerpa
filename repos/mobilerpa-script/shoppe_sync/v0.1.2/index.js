"use strict";
"use strict";
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/index.ts
var index_exports = {};
__export(index_exports, {
  run: () => run
});
module.exports = __toCommonJS(index_exports);

// ../shared/src/index.ts
function noopReportProgress() {
}
function formatErrorMessage(error) {
  if (!error) {
    return "未知错误";
  }
  if (typeof error === "string") {
    return error;
  }
  if (typeof error === "object" && error && "message" in error) {
    return String(error.message || "未知错误");
  }
  return String(error);
}
function ensureNotCancelled(helpers, message) {
  if (helpers && typeof helpers.throwIfCancelled === "function") {
    helpers.throwIfCancelled(message);
    return;
  }
  if (helpers && typeof helpers.isCancelled === "function" && helpers.isCancelled()) {
    throw new Error(message || "任务已取消");
  }
}
function sleepSafe(durationMS, helpers) {
  if (helpers && helpers.runtime && typeof helpers.runtime.sleepMS === "function") {
    helpers.runtime.sleepMS(durationMS);
    return;
  }
  if (typeof sleep === "function") {
    sleep(durationMS);
    return;
  }
  if (typeof java !== "undefined" && java.lang && java.lang.Thread && typeof java.lang.Thread.sleep === "function") {
    java.lang.Thread.sleep(durationMS);
  }
}
function waitForPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage) {
  if (typeof currentPackage !== "function") {
    return true;
  }
  const endTime = Date.now() + timeoutMS;
  while (Date.now() < endTime) {
    ensureNotCancelled(helpers, cancelMessage || "任务已取消");
    if (currentPackage() === packageName) {
      return true;
    }
    sleepSafe(300, helpers);
  }
  return false;
}
function launchAppByPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage) {
  if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
    throw new Error("当前运行环境不支持 app.launchPackage");
  }
  ensureNotCancelled(helpers, cancelMessage || "任务已取消");
  app.launchPackage(packageName);
  if (!waitForPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage)) {
    throw new Error("等待应用启动超时");
  }
}

// src/config.ts
var GHOSTMOBILE_PACKAGE_NAME = "com.game.ghostmobile";
var APP_START_TIMEOUT_MS = 1e4;

// src/common.ts
function launchAppByPackage(packageName, helpers) {
  launchAppByPackageWithCancel(packageName, APP_START_TIMEOUT_MS, helpers, "任务已取消，停止等待极魔游戏助手启动");
}

// src/ghostmobile.ts
function launchAndStartGame(helpers) {
  ensureAccessibilityEnabled();
  launchAppByPackage(GHOSTMOBILE_PACKAGE_NAME, helpers);
  toastSafe("启动 极魔游戏助手");
  return {
    started: true,
    code: "OK",
    message: "尝试启动应用完成",
    state: "already_locked"
  };
}
function ensureAccessibilityEnabled() {
  if (typeof auto === "undefined" || !auto || !auto.service) {
    toastSafe("请先开启无障碍服务");
    throw new Error("无障碍服务未开启");
  }
}
function toastSafe(message) {
  if (typeof toast === "function") {
    toast(message);
  }
}

// src/app.ts
function startApp(helpers) {
  if (helpers && typeof helpers.reportProgress === "function") {
    helpers.reportProgress("CALL_GHOSTMOBILE", "任务执行中：准备调用极魔游戏助手启动链路", "running", {
      app_name: "极魔游戏助手",
      app_package: GHOSTMOBILE_PACKAGE_NAME
    });
  }
  return launchAndStartGame(helpers);
}

// src/index.ts
var SCRIPT_NAME = "shoppe_sync";
var SCRIPT_VERSION = "v0.1.2";
function run(context, helpers) {
  const logger = helpers && helpers.logger ? helpers.logger : console;
  const reportProgress = helpers && typeof helpers.reportProgress === "function" ? helpers.reportProgress : noopReportProgress;
  logger.info && logger.info("开始执行真实业务脚本：" + SCRIPT_NAME + "@" + SCRIPT_VERSION);
  reportProgress("INIT_APP", "任务执行中：准备启动极魔游戏助手", "running", {
    app_name: "极魔游戏助手",
    app_package: GHOSTMOBILE_PACKAGE_NAME
  });
  try {
    reportProgress("CHECK_HOME", "任务执行中：检查启动前页面状态", "running", {
      app_name: "极魔游戏助手"
    });
    const result = startApp(helpers);
    if (result && result.started) {
      logger.info && logger.info("业务脚本执行完成：已确认加速已启动");
      reportProgress("COMPLETE", "任务执行中：已确认加速已启动", "success", {
        acceleration_state: result.state
      });
      return {
        status: "success",
        result_code: "OK",
        result_message: result.message,
        step_name: "START_GHOSTMOBILE",
        extra: {
          mode: "real_script",
          script_name: SCRIPT_NAME,
          script_version: SCRIPT_VERSION,
          entry_file: "shoppe_sync/v0.1.2/index.js",
          app_name: "极魔游戏助手",
          app_package: GHOSTMOBILE_PACKAGE_NAME,
          acceleration_state: result.state
        }
      };
    }
    logger.warn && logger.warn("业务脚本执行结束，但未达到成功条件：" + result.message);
    reportProgress("START_GHOSTMOBILE", "任务执行中：未达到预期启动结果", "failed", {
      acceleration_state: result && result.state ? result.state : ""
    });
    return {
      status: "failed",
      result_code: result.code,
      result_message: result.message,
      step_name: "START_GHOSTMOBILE",
      extra: {
        mode: "real_script",
        script_name: SCRIPT_NAME,
        script_version: SCRIPT_VERSION,
        entry_file: "shoppe_sync/v0.1.2/index.js",
        app_name: "极魔游戏助手",
        app_package: GHOSTMOBILE_PACKAGE_NAME,
        acceleration_state: result.state
      }
    };
  } catch (error) {
    const message = formatErrorMessage(error);
    logger.error && logger.error("业务脚本执行异常：" + message);
    reportProgress("START_GHOSTMOBILE", "任务执行中：业务脚本执行异常", "failed", {
      error_message: message
    });
    return {
      status: "failed",
      result_code: "SCRIPT_RUNTIME_ERROR",
      result_message: message,
      step_name: "START_GHOSTMOBILE",
      extra: {
        mode: "real_script",
        script_name: SCRIPT_NAME,
        script_version: SCRIPT_VERSION,
        entry_file: "shoppe_sync/v0.1.2/index.js",
        app_name: "极魔游戏助手",
        app_package: GHOSTMOBILE_PACKAGE_NAME
      }
    };
  }
}

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
function noopThrowIfCancelled() {
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
function buildOpenAppExtra(context, config) {
  return {
    mode: "open_app",
    script_name: config.scriptName,
    script_version: config.scriptVersion,
    entry_file: config.scriptName + "/" + config.scriptVersion + "/index.js",
    app_name: config.appName,
    app_package: config.packageName,
    task_id: String(context && context.task_id ? context.task_id : "")
  };
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

// src/index.ts
var SCRIPT_NAME = "open_qq";
var SCRIPT_VERSION = "v0.1.0";
var APP_NAME = "QQ";
var PACKAGE_NAME = "com.tencent.mobileqq";
var APP_START_TIMEOUT_MS = 15e3;
function run(context, helpers) {
  const logger = helpers && helpers.logger ? helpers.logger : console;
  const reportProgress = helpers && typeof helpers.reportProgress === "function" ? helpers.reportProgress : noopReportProgress;
  const throwIfCancelled = helpers && typeof helpers.throwIfCancelled === "function" ? helpers.throwIfCancelled : noopThrowIfCancelled;
  logger.info && logger.info("开始执行脚本：" + SCRIPT_NAME + "@" + SCRIPT_VERSION);
  reportProgress("OPEN_APP", "任务执行中：准备打开 QQ", "running", {
    app_name: APP_NAME,
    app_package: PACKAGE_NAME
  });
  try {
    throwIfCancelled("任务已取消，停止打开 QQ");
    launchAppByPackageWithCancel(PACKAGE_NAME, APP_START_TIMEOUT_MS, helpers, "任务已取消，停止等待 QQ 启动");
    logger.info && logger.info("脚本执行完成：QQ 已启动");
    reportProgress("OPEN_APP", "任务执行中：QQ 已成功启动", "success", {
      app_name: APP_NAME,
      app_package: PACKAGE_NAME
    });
    return {
      status: "success",
      result_code: "OK",
      result_message: "QQ 已启动",
      step_name: "OPEN_APP",
      extra: buildOpenAppExtra(context, {
        scriptName: SCRIPT_NAME,
        scriptVersion: SCRIPT_VERSION,
        appName: APP_NAME,
        packageName: PACKAGE_NAME,
        appStartTimeoutMS: APP_START_TIMEOUT_MS
      })
    };
  } catch (error) {
    const message = formatErrorMessage(error);
    logger.error && logger.error("脚本执行失败：" + message);
    reportProgress("OPEN_APP", "任务执行中：QQ 启动失败", "failed", {
      app_name: APP_NAME,
      app_package: PACKAGE_NAME,
      error_message: message
    });
    return {
      status: "failed",
      result_code: "OPEN_APP_FAILED",
      result_message: message,
      step_name: "OPEN_APP",
      extra: buildOpenAppExtra(context, {
        scriptName: SCRIPT_NAME,
        scriptVersion: SCRIPT_VERSION,
        appName: APP_NAME,
        packageName: PACKAGE_NAME,
        appStartTimeoutMS: APP_START_TIMEOUT_MS
      })
    };
  }
}

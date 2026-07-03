import {
  buildOpenAppExtra,
  formatErrorMessage,
  launchAppByPackageWithCancel,
  noopReportProgress,
  noopThrowIfCancelled,
  type ScriptContext,
  type ScriptHelpers,
  type ScriptResult
} from "@mobilerpa-script/shared";

const SCRIPT_NAME = "open_qq";
const SCRIPT_VERSION = "v0.1.2";
const APP_NAME = "QQ";
const PACKAGE_NAME = "com.tencent.mobileqq";
const APP_START_TIMEOUT_MS = 15000;

function run(context: ScriptContext, helpers?: ScriptHelpers): ScriptResult {
  const logger = helpers && helpers.logger ? helpers.logger : console;
  const reportProgress = helpers && typeof helpers.reportProgress === "function"
    ? helpers.reportProgress
    : noopReportProgress;
  const throwIfCancelled = helpers && typeof helpers.throwIfCancelled === "function"
    ? helpers.throwIfCancelled
    : noopThrowIfCancelled;

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

export {
  run
};

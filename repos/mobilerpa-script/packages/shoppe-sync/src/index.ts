import {
  formatErrorMessage,
  noopReportProgress,
  type ScriptContext,
  type ScriptHelpers,
  type ScriptResult
} from "@mobilerpa-script/shared";
import {
  GHOSTMOBILE_PACKAGE_NAME
} from "./config";
import {
  startApp
} from "./app";

const SCRIPT_NAME = "shoppe_sync";
const SCRIPT_VERSION = "v0.1.2";

function run(context: ScriptContext, helpers?: ScriptHelpers): ScriptResult {
  const logger = helpers && helpers.logger ? helpers.logger : console;
  const reportProgress = helpers && typeof helpers.reportProgress === "function"
    ? helpers.reportProgress
    : noopReportProgress;

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

export {
  run
};

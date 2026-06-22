"use strict";

var appUtil = require("./core/app");

/**
 * 执行 `shoppe_sync@v0.1.2` 的真实业务脚本入口。
 *
 * 当前版本迁移 `projects/shoppe/main.js` 对应的最小链路，
 * 即：启动极魔游戏助手，并根据页面状态尝试开启加速。
 *
 * @param {Object} context 任务执行上下文。
 * @param {{runtime: Object, logger: Object, reportProgress: Function}} helpers 运行时辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var reportProgress = typeof (helpers && helpers.reportProgress) === "function" ? helpers.reportProgress : noopReportProgress;
    var result = null;

    logger.info("开始执行真实业务脚本：" + context.script_name + "@" + context.script_version);
    reportProgress("INIT_APP", "任务执行中：准备启动极魔游戏助手", "running", {
        app_name: "极魔游戏助手",
        app_package: "com.game.ghostmobile"
    });

    try {
        reportProgress("CHECK_HOME", "任务执行中：检查启动前页面状态", "running", {
            app_name: "极魔游戏助手"
        });

        result = appUtil.startApp(logger, reportProgress);
        if (result && result.started) {
            logger.info("业务脚本执行完成：已确认加速已启动");
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
                    entry_file: "scripts/shoppe_sync/v0.1.2/index.js",
                    app_name: "极魔游戏助手",
                    app_package: "com.game.ghostmobile",
                    acceleration_state: result.state
                }
            };
        }

        logger.warn("业务脚本执行结束，但未达到成功条件：" + result.message);
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
                entry_file: "scripts/shoppe_sync/v0.1.2/index.js",
                app_name: "极魔游戏助手",
                app_package: "com.game.ghostmobile",
                acceleration_state: result.state
            }
        };
    } catch (error) {
        logger.error("业务脚本执行异常：" + formatErrorMessage(error));
        reportProgress("START_GHOSTMOBILE", "任务执行中：业务脚本执行异常", "failed", {
            error_message: formatErrorMessage(error)
        });
        return {
            status: "failed",
            result_code: "SCRIPT_RUNTIME_ERROR",
            result_message: formatErrorMessage(error),
            step_name: "START_GHOSTMOBILE",
            extra: {
                mode: "real_script",
                entry_file: "scripts/shoppe_sync/v0.1.2/index.js",
                app_name: "极魔游戏助手",
                app_package: "com.game.ghostmobile"
            }
        };
    }
}

/**
 * 空进度上报函数。
 *
 * 用于本地直跑或旧调用方未传入 `reportProgress` 时兜底。
 */
function noopReportProgress() {}

/**
 * 提取错误文本。
 *
 * @param {*} error 原始异常对象。
 * @returns {string} 错误文本。
 */
function formatErrorMessage(error) {
    if (!error) {
        return "未知错误";
    }

    if (typeof error === "string") {
        return error;
    }

    if (error.message) {
        return String(error.message);
    }

    return String(error);
}

module.exports = {
    run: run
};

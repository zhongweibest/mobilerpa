"use strict";

var appUtil = require("./core/app");

/**
 * 执行 `shoppe_sync@v0.1.1` 的真实业务脚本入口。
 *
 * 当前版本迁移 `projects/shoppe/main.js` 对应的最小链路，
 * 即：启动极魔游戏助手，并根据页面状态尝试开启加速。
 *
 * @param {Object} context 任务执行上下文。
 * @param {{runtime: Object, logger: Object}} helpers 运行时辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var result = null;

    logger.info("开始执行真实业务脚本：" + context.script_name + "@" + context.script_version);

    try {
        result = appUtil.startApp(logger);
        if (result && result.started) {
            logger.info("业务脚本执行完成：已确认加速已启动");
            return {
                status: "success",
                result_code: "OK",
                result_message: result.message,
                step_name: "START_GHOSTMOBILE",
                extra: {
                    mode: "real_script",
                    entry_file: "scripts/shoppe_sync/v0.1.1/index.js",
                    app_name: "极魔游戏助手",
                    app_package: "com.game.ghostmobile",
                    acceleration_state: result.state
                }
            };
        }

        logger.warn("业务脚本执行结束，但未达到成功条件：" + result.message);
        return {
            status: "failed",
            result_code: result.code,
            result_message: result.message,
            step_name: "START_GHOSTMOBILE",
            extra: {
                mode: "real_script",
                entry_file: "scripts/shoppe_sync/v0.1.1/index.js",
                app_name: "极魔游戏助手",
                app_package: "com.game.ghostmobile",
                acceleration_state: result.state
            }
        };
    } catch (error) {
        logger.error("业务脚本执行异常：" + formatErrorMessage(error));
        return {
            status: "failed",
            result_code: "SCRIPT_RUNTIME_ERROR",
            result_message: formatErrorMessage(error),
            step_name: "START_GHOSTMOBILE",
            extra: {
                mode: "real_script",
                entry_file: "scripts/shoppe_sync/v0.1.1/index.js",
                app_name: "极魔游戏助手",
                app_package: "com.game.ghostmobile"
            }
        };
    }
}

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

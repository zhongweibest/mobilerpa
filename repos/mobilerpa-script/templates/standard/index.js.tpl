"use strict";

var SCRIPT_NAME = "__SCRIPT_NAME__";
var SCRIPT_VERSION = "__SCRIPT_VERSION__";

/**
 * 执行脚本主入口。
 *
 * @param {Object} context 任务上下文。
 * @param {{logger: Object, runtime: Object, reportProgress: Function}} helpers 运行辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var reportProgress = typeof (helpers && helpers.reportProgress) === "function" ? helpers.reportProgress : noopReportProgress;

    logger.info("开始执行脚本：" + SCRIPT_NAME + "@" + SCRIPT_VERSION);
    reportProgress("INIT_SCRIPT", "任务执行中：脚本已启动，准备进入业务逻辑", "running", {
        script_name: SCRIPT_NAME,
        script_version: SCRIPT_VERSION
    });

    try {
        executeBusiness(context, helpers, reportProgress);
        reportProgress("FINISH_SCRIPT", "任务执行中：脚本业务逻辑执行完成", "success", {
            script_name: SCRIPT_NAME,
            script_version: SCRIPT_VERSION
        });
        return {
            status: "success",
            result_code: "OK",
            result_message: "脚本执行成功",
            step_name: "FINISH_SCRIPT",
            extra: buildExtra(context)
        };
    } catch (error) {
        var message = formatErrorMessage(error);
        logger.error("脚本执行失败：" + message);
        reportProgress("RUN_SCRIPT", "任务执行中：脚本执行失败", "failed", {
            script_name: SCRIPT_NAME,
            script_version: SCRIPT_VERSION,
            error_message: message
        });
        return {
            status: "failed",
            result_code: "SCRIPT_EXECUTION_FAILED",
            result_message: message,
            step_name: "RUN_SCRIPT",
            extra: buildExtra(context)
        };
    }
}

/**
 * 编写实际业务逻辑。
 *
 * @param {Object} context 任务上下文。
 * @param {{logger: Object, runtime: Object, reportProgress: Function}} helpers 运行辅助对象。
 * @param {Function} reportProgress 进度上报函数。
 */
function executeBusiness(context, helpers, reportProgress) {
    var logger = helpers && helpers.logger ? helpers.logger : console;

    logger.info("请在 executeBusiness 中填充真实业务逻辑");
    reportProgress("CUSTOM_STEP", "任务执行中：这里是自定义业务步骤，请按实际业务替换", "running", {
        task_id: context && context.task_id ? context.task_id : "",
        params: context && context.params ? context.params : {}
    });

    if (typeof sleep === "function") {
        sleep(1000);
    }
}

/**
 * 构造统一附加结果。
 *
 * @param {Object} context 任务上下文。
 * @returns {Object} 附加结果。
 */
function buildExtra(context) {
    return {
        script_name: SCRIPT_NAME,
        script_version: SCRIPT_VERSION,
        entry_file: SCRIPT_NAME + "/" + SCRIPT_VERSION + "/index.js",
        task_id: context && context.task_id ? context.task_id : ""
    };
}

/**
 * 空进度上报函数。
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

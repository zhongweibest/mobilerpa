"use strict";

var script = require("./index");

/**
 * 创建调试日志对象。
 *
 * @returns {{info: Function, warn: Function, error: Function}} 调试日志对象。
 */
function createDebugLogger() {
    return {
        info: function (message) {
            writeLog("[DEBUG][INFO] " + message);
        },
        warn: function (message) {
            writeLog("[DEBUG][WARN] " + message);
        },
        error: function (message) {
            writeLog("[DEBUG][ERROR] " + message);
        }
    };
}

/**
 * 创建调试进度上报函数。
 *
 * @param {{info: Function}} logger 调试日志对象。
 * @returns {Function} 进度上报函数。
 */
function createDebugProgressReporter(logger) {
    return function reportProgress(stepName, message, status, extra) {
        logger.info("[PROGRESS] " + JSON.stringify({
            step_name: String(stepName || ""),
            message: String(message || ""),
            status: String(status || "running"),
            extra: extra || {}
        }));
    };
}

/**
 * 写入调试日志。
 *
 * @param {string} message 日志内容。
 */
function writeLog(message) {
    if (typeof log === "function") {
        log(message);
        return;
    }

    if (typeof console !== "undefined" && console.log) {
        console.log(message);
    }
}

/**
 * 构造模拟任务上下文。
 *
 * @returns {Object} 模拟任务上下文。
 */
function buildDebugContext() {
    return {
        task_id: "debug_open_weichat_v0_1_0",
        script_name: "open_weichat",
        script_version: "v0.1.0",
        priority: 3,
        params: {
            debug: true
        },
        device_id: "debug_device",
        agent_uuid: "debug_agent",
        center_base_url: ""
    };
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

    if (error.stack) {
        return String(error.stack);
    }

    if (error.message) {
        return String(error.message);
    }

    return String(error);
}

/**
 * 执行调试入口。
 */
function main() {
    var logger = createDebugLogger();
    var context = buildDebugContext();
    var result = null;

    logger.info("开始直接调试 open_weichat@v0.1.0");
    logger.info("模拟任务上下文：" + JSON.stringify(context));

    try {
        if (!script || typeof script.run !== "function") {
            throw new Error("index.js 未导出 run(context, helpers) 函数");
        }

        result = script.run(context, {
            logger: logger,
            runtime: {},
            reportProgress: createDebugProgressReporter(logger)
        });

        logger.info("调试执行结果：" + JSON.stringify(result));
        if (typeof toast === "function") {
            toast("调试完成：" + result.status + " / " + result.result_code);
        }
    } catch (error) {
        logger.error("调试执行异常：" + formatErrorMessage(error));
        if (typeof toast === "function") {
            toast("调试异常：" + formatErrorMessage(error));
        }
    }
}

main();

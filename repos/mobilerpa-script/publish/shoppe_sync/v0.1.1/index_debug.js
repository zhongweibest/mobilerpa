"use strict";

var script = require("./index");

/**
 * 当前文件用于在 AutoJs6 中直接调试 `shoppe_sync@v0.1.1`。
 *
 * 它会模拟 Agent 的 `task_runner` 调用方式，不依赖中心服务、WebSocket 或任务下发。
 * 调试时请在 AutoJs6 中直接运行本文件，而不是运行 `index.js`。
 */

/**
 * 创建调试日志对象。
 *
 * @returns {{info: Function, warn: Function, error: Function}} 日志对象。
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
        task_id: "debug_shoppe_sync_v0_1_1",
        script_name: "shoppe_sync",
        script_version: "v0.1.1",
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

    logger.info("开始直接调试 shoppe_sync@v0.1.1");
    logger.info("模拟任务上下文：" + JSON.stringify(context));

    try {
        if (!script || typeof script.run !== "function") {
            throw new Error("index.js 未导出 run(context, helpers) 函数");
        }

        result = script.run(context, {
            logger: logger,
            runtime: {}
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

"use strict";

var script = require("./index");

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

function writeLog(message) {
    if (typeof log === "function") {
        log(message);
        return;
    }
    if (typeof console !== "undefined" && console.log) {
        console.log(message);
    }
}

function buildDebugContext() {
    return {
        task_id: "debug_open_douyin_v0_1_2",
        script_name: "open_douyin",
        script_version: "v0.1.2",
        priority: 3,
        params: {
  "debug": true
},
        device_id: "debug_device",
        agent_uuid: "debug_agent",
        center_base_url: ""
    };
}

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

function main() {
    var logger = createDebugLogger();
    var context = buildDebugContext();
    var result = null;

    logger.info("开始直接调试 open_douyin@v0.1.2");
    logger.info("模拟任务上下文：" + JSON.stringify(context));

    try {
        if (!script || typeof script.run !== "function") {
            throw new Error("index.js 未导出 run(context, helpers) 函数");
        }

        result = script.run(context, {
            logger: logger,
            runtime: {},
            reportProgress: createDebugProgressReporter(logger),
            isCancelled: function () {
                return false;
            },
            throwIfCancelled: function () {}
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

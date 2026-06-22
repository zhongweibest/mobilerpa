"use strict";

var SCRIPT_NAME = "open_qq";
var SCRIPT_VERSION = "v0.1.0";
var APP_NAME = "QQ";
var PACKAGE_NAME = "com.tencent.mobileqq";
var APP_START_TIMEOUT_MS = 15000;

/**
 * 执行打开 QQ 脚本。
 *
 * @param {Object} context 任务上下文。
 * @param {{logger: Object, runtime: Object, reportProgress: Function}} helpers 运行辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var reportProgress = typeof (helpers && helpers.reportProgress) === "function" ? helpers.reportProgress : noopReportProgress;

    logger.info("开始执行脚本：" + SCRIPT_NAME + "@" + SCRIPT_VERSION);
    reportProgress("OPEN_APP", "任务执行中：准备打开 QQ", "running", {
        app_name: APP_NAME,
        app_package: PACKAGE_NAME
    });

    try {
        launchAppByPackage(PACKAGE_NAME);
        logger.info("脚本执行完成：QQ 已启动");
        reportProgress("OPEN_APP", "任务执行中：QQ 已成功启动", "success", {
            app_name: APP_NAME,
            app_package: PACKAGE_NAME
        });
        return {
            status: "success",
            result_code: "OK",
            result_message: "QQ 已启动",
            step_name: "OPEN_APP",
            extra: buildExtra(context)
        };
    } catch (error) {
        logger.error("脚本执行失败：" + formatErrorMessage(error));
        reportProgress("OPEN_APP", "任务执行中：QQ 启动失败", "failed", {
            app_name: APP_NAME,
            app_package: PACKAGE_NAME,
            error_message: formatErrorMessage(error)
        });
        return {
            status: "failed",
            result_code: "OPEN_APP_FAILED",
            result_message: formatErrorMessage(error),
            step_name: "OPEN_APP",
            extra: buildExtra(context)
        };
    }
}

/**
 * 通过包名启动应用并等待前台切换成功。
 *
 * @param {string} packageName 应用包名。
 */
function launchAppByPackage(packageName) {
    if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
        throw new Error("当前运行环境不支持 app.launchPackage");
    }

    app.launchPackage(packageName);
    if (!waitForPackageSafe(packageName, APP_START_TIMEOUT_MS)) {
        throw new Error("等待应用启动超时");
    }
}

/**
 * 安全等待指定包名进入前台。
 *
 * @param {string} packageName 应用包名。
 * @param {number} timeoutMS 超时时间。
 * @returns {boolean} 是否等待成功。
 */
function waitForPackageSafe(packageName, timeoutMS) {
    if (typeof waitForPackage === "function") {
        return waitForPackage(packageName, timeoutMS);
    }

    if (typeof currentPackage !== "function") {
        return true;
    }

    var endTime = Date.now() + timeoutMS;
    while (Date.now() < endTime) {
        if (currentPackage() === packageName) {
            return true;
        }
        sleepSafe(300);
    }
    return false;
}

/**
 * 安全执行休眠。
 *
 * @param {number} durationMS 休眠时长。
 */
function sleepSafe(durationMS) {
    if (typeof sleep === "function") {
        sleep(durationMS);
        return;
    }

    if (typeof java !== "undefined" && java.lang && java.lang.Thread && typeof java.lang.Thread.sleep === "function") {
        java.lang.Thread.sleep(durationMS);
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
        mode: "open_app",
        script_name: SCRIPT_NAME,
        script_version: SCRIPT_VERSION,
        entry_file: "open_qq/v0.1.0/index.js",
        app_name: APP_NAME,
        app_package: PACKAGE_NAME,
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

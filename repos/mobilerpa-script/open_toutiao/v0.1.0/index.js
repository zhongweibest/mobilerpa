"use strict";

var SCRIPT_NAME = "open_toutiao";
var SCRIPT_VERSION = "v0.1.0";
var APP_NAME = "今日头条";
var PACKAGE_NAME = "com.ss.android.article.news";
var APP_START_TIMEOUT_MS = 15000;

/**
 * 执行打开今日头条脚本。
 *
 * @param {Object} context 任务上下文。
 * @param {{logger: Object, runtime: Object, reportProgress: Function, isCancelled: Function, throwIfCancelled: Function}} helpers 运行辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var reportProgress = typeof (helpers && helpers.reportProgress) === "function" ? helpers.reportProgress : noopReportProgress;
    var throwIfCancelled = typeof (helpers && helpers.throwIfCancelled) === "function" ? helpers.throwIfCancelled : noopThrowIfCancelled;

    logger.info("开始执行脚本：" + SCRIPT_NAME + "@" + SCRIPT_VERSION);
    reportProgress("OPEN_APP", "任务执行中：准备打开今日头条", "running", {
        app_name: APP_NAME,
        app_package: PACKAGE_NAME
    });

    try {
        throwIfCancelled("任务已取消，停止打开今日头条");
        launchAppByPackage(PACKAGE_NAME, helpers);
        logger.info("脚本执行完成：今日头条已启动");
        reportProgress("OPEN_APP", "任务执行中：今日头条已成功启动", "success", {
            app_name: APP_NAME,
            app_package: PACKAGE_NAME
        });
        return {
            status: "success",
            result_code: "OK",
            result_message: "今日头条已启动",
            step_name: "OPEN_APP",
            extra: buildExtra(context)
        };
    } catch (error) {
        logger.error("脚本执行失败：" + formatErrorMessage(error));
        reportProgress("OPEN_APP", "任务执行中：今日头条启动失败", "failed", {
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
 * @param {{isCancelled: Function, throwIfCancelled: Function}} helpers 运行辅助对象。
 */
function launchAppByPackage(packageName, helpers) {
    if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
        throw new Error("当前运行环境不支持 app.launchPackage");
    }

    ensureNotCancelled(helpers, "任务已取消，停止打开今日头条");
    app.launchPackage(packageName);
    if (!waitForPackageSafe(packageName, APP_START_TIMEOUT_MS, helpers)) {
        throw new Error("等待应用启动超时");
    }
}

/**
 * 安全等待指定包名进入前台。
 *
 * @param {string} packageName 应用包名。
 * @param {number} timeoutMS 超时时间。
 * @param {{isCancelled: Function, throwIfCancelled: Function}} helpers 运行辅助对象。
 * @returns {boolean} 是否等待成功。
 */
function waitForPackageSafe(packageName, timeoutMS, helpers) {
    if (typeof currentPackage !== "function") {
        return true;
    }

    var endTime = Date.now() + timeoutMS;
    while (Date.now() < endTime) {
        ensureNotCancelled(helpers, "任务已取消，停止等待今日头条启动");
        if (currentPackage() === packageName) {
            return true;
        }
        sleepSafe(300);
    }
    return false;
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
        entry_file: "open_toutiao/v0.1.0/index.js",
        app_name: APP_NAME,
        app_package: PACKAGE_NAME,
        task_id: context && context.task_id ? context.task_id : ""
    };
}

/**
 * 空进度上报函数。
 */
function noopReportProgress() {}

function noopThrowIfCancelled() {}

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

"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.noopReportProgress = noopReportProgress;
exports.noopThrowIfCancelled = noopThrowIfCancelled;
exports.formatErrorMessage = formatErrorMessage;
exports.buildOpenAppExtra = buildOpenAppExtra;
exports.ensureNotCancelled = ensureNotCancelled;
exports.sleepSafe = sleepSafe;
exports.waitForPackageWithCancel = waitForPackageWithCancel;
exports.launchAppByPackageWithCancel = launchAppByPackageWithCancel;
function noopReportProgress() { }
function noopThrowIfCancelled() { }
function formatErrorMessage(error) {
    if (!error) {
        return "未知错误";
    }
    if (typeof error === "string") {
        return error;
    }
    if (typeof error === "object" && error && "message" in error) {
        return String(error.message || "未知错误");
    }
    return String(error);
}
function buildOpenAppExtra(context, config) {
    return {
        mode: "open_app",
        script_name: config.scriptName,
        script_version: config.scriptVersion,
        entry_file: config.scriptName + "/" + config.scriptVersion + "/index.js",
        app_name: config.appName,
        app_package: config.packageName,
        task_id: String(context && context.task_id ? context.task_id : "")
    };
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
function sleepSafe(durationMS, helpers) {
    if (helpers && helpers.runtime && typeof helpers.runtime.sleepMS === "function") {
        helpers.runtime.sleepMS(durationMS);
        return;
    }
    if (typeof sleep === "function") {
        sleep(durationMS);
        return;
    }
    if (typeof java !== "undefined" && java.lang && java.lang.Thread && typeof java.lang.Thread.sleep === "function") {
        java.lang.Thread.sleep(durationMS);
    }
}
function waitForPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage) {
    if (typeof currentPackage !== "function") {
        return true;
    }
    const endTime = Date.now() + timeoutMS;
    while (Date.now() < endTime) {
        ensureNotCancelled(helpers, cancelMessage || "任务已取消");
        if (currentPackage() === packageName) {
            return true;
        }
        sleepSafe(300, helpers);
    }
    return false;
}
function launchAppByPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage) {
    if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
        throw new Error("当前运行环境不支持 app.launchPackage");
    }
    ensureNotCancelled(helpers, cancelMessage || "任务已取消");
    app.launchPackage(packageName);
    if (!waitForPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage)) {
        throw new Error("等待应用启动超时");
    }
}

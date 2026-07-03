"use strict";

var config = require("../config/config");

/**
 * 通过包名启动应用，并等待进入前台。
 *
 * @param {string} packageName 应用包名。
 */
function launchAppByPackage(packageName) {
    if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
        throw new Error("当前运行环境不支持 app.launchPackage");
    }

    app.launchPackage(packageName);

    if (!waitForPackageSafe(packageName, config.appStartTimeout)) {
        throw new Error("启动失败");
    }
}

/**
 * 安全等待应用进入前台。
 *
 * @param {string} packageName 包名。
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
 * 安全执行睡眠。
 *
 * @param {number} durationMS 睡眠时长。
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

module.exports = {
    launchAppByPackage: launchAppByPackage
};

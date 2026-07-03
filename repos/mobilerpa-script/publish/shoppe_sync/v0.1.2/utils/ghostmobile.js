"use strict";

var config = require("../config/config");
var common = require("./common");

/**
 * 启动软件并根据初始化状态执行开启加速。
 *
 * @param {Object} logger 日志对象。
 * @returns {{started: boolean, code: string, message: string, state: string}} 执行结果。
 */
function launchAndStartGame(logger) {
    var appName = "极魔游戏助手";
    var statusID = "com.game.ghostmobile:id/tv_step1_status";
    var status2ID = "com.game.ghostmobile:id/tv_step2_status";
    var delayID = "com.game.ghostmobile:id/tv_step1_delay";
    var startBtnID = "com.game.ghostmobile:id/btn_takeoff";

    ensureAccessibilityEnabled();

    common.launchAppByPackage(config.ghostmobilePackageName);
    toastSafe("启动 " + appName);
    return {
            started: true,
            code: "OK",
            message: "尝试启动应用完成",
            state: "already_locked"
        };
}

/**
 * 构造失败结果。
 *
 * @param {string} code 结果码。
 * @param {string} message 结果描述。
 * @param {string} state 业务状态。
 * @returns {{started: boolean, code: string, message: string, state: string}} 失败结果。
 */
function buildFailure(code, message, state) {
    return {
        started: false,
        code: code,
        message: message,
        state: state
    };
}

/**
 * 确保无障碍服务已经开启。
 */
function ensureAccessibilityEnabled() {
    if (typeof auto === "undefined" || !auto || !auto.service) {
        toastSafe("请先开启无障碍服务");
        throw new Error("无障碍服务未开启");
    }
}

/**
 * 判断当前是否已经处于加速锁定状态。
 *
 * @param {Object} status2Node 状态节点。
 * @returns {boolean} 是否已锁定。
 */
function isAccelerationLocked(status2Node) {
    return !!(status2Node && typeof status2Node.text === "function" && status2Node.text() === "加速已锁定");
}

/**
 * 点击开始后等待结果。
 *
 * @param {number} timeoutMS 超时时间。
 * @returns {{state: string, msg: string}} 结果。
 */
function waitAfterStartClick(timeoutMS) {
    var status2ID = "com.game.ghostmobile:id/tv_step2_status";
    var alertTitleID = "com.game.ghostmobile:id/alertTitle";
    var alertOKBtnID = "android:id/button1";
    var endTime = Date.now() + timeoutMS;

    while (Date.now() < endTime) {
        var status2Node = findNodeByID(status2ID, 500);
        if (status2Node && typeof status2Node.text === "function" && status2Node.text() === "加速已锁定") {
            return {
                state: "locked",
                msg: "加速已锁定"
            };
        }

        var alertTitleNode = findNodeByID(alertTitleID, 500);
        if (alertTitleNode && typeof alertTitleNode.text === "function" && alertTitleNode.text() === "加速失败") {
            var okBtn = findNodeByID(alertOKBtnID, 1000);
            if (okBtn && typeof okBtn.click === "function") {
                okBtn.click();
            }
            return {
                state: "popup_failed",
                msg: "出现加速失败弹窗"
            };
        }

        sleepSafe(300);
    }

    return {
        state: "timeout",
        msg: "点击后未等到锁定态或弹窗态"
    };
}

/**
 * 通过资源 ID 查找节点。
 *
 * @param {string} nodeID 资源 ID。
 * @param {number} timeoutMS 查找超时。
 * @returns {Object|null} 节点对象。
 */
function findNodeByID(nodeID, timeoutMS) {
    if (typeof id !== "function") {
        throw new Error("当前运行环境不支持 id() 查询");
    }
    return id(nodeID).findOne(timeoutMS);
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

/**
 * 安全弹出提示。
 *
 * @param {string} message 提示内容。
 */
function toastSafe(message) {
    if (typeof toast === "function") {
        toast(message);
    }
}

module.exports = {
    launchAndStartGame: launchAndStartGame
};

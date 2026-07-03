"use strict";

var ghostmobile = require("../utils/ghostmobile");

/**
 * 启动业务应用。
 *
 * @param {Object} logger 日志对象。
 * @param {Function} reportProgress 进度上报函数。
 * @returns {{started: boolean, code: string, message: string, state: string}} 执行结果。
 */
function startApp(logger, reportProgress) {
    if (typeof reportProgress === "function") {
        reportProgress("CALL_GHOSTMOBILE", "任务执行中：准备调用极魔游戏助手启动链路", "running", {
            app_name: "极魔游戏助手",
            app_package: "com.game.ghostmobile"
        });
    }
    return ghostmobile.launchAndStartGame(logger);
}

module.exports = {
    startApp: startApp
};

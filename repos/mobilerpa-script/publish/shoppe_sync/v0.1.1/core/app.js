"use strict";

var ghostmobile = require("../utils/ghostmobile");

/**
 * 启动业务应用。
 *
 * @param {Object} logger 日志对象。
 * @returns {{started: boolean, code: string, message: string, state: string}} 执行结果。
 */
function startApp(logger) {
    return ghostmobile.launchAndStartGame(logger);
}

module.exports = {
    startApp: startApp
};

"use strict";

/**
 * 执行 `shoppe_sync@v0.1.0` 的最小演示脚本。
 *
 * 当前仍然不是正式业务脚本，只是把原来的内置演示执行器迁移成真实脚本文件入口，
 * 便于后续继续替换为真正的业务实现。
 *
 * @param {Object} context 任务执行上下文。
 * @param {{runtime: Object, logger: Object}} helpers 运行时辅助对象。
 * @returns {{status: string, result_code: string, result_message: string, step_name: string, extra: Object}} 执行结果。
 */
function run(context, helpers) {
    var logger = helpers && helpers.logger ? helpers.logger : console;
    var runtime = helpers && helpers.runtime ? helpers.runtime : null;
    var params = context && context.params ? context.params : {};
    var sleepMS = Number(params.sleep_ms || 0);
    var shouldFail = params.should_fail === true || params.should_fail === "true";

    if (sleepMS > 0 && runtime && typeof runtime.sleepMS === "function") {
        runtime.sleepMS(sleepMS);
    }

    if (shouldFail) {
        logger.warn("脚本按参数要求模拟失败：" + context.script_name + "@" + context.script_version);
        return {
            status: "failed",
            result_code: "SIMULATED_FAILURE",
            result_message: "按任务参数要求模拟失败",
            step_name: "RUN_SCRIPT_ENTRY",
            extra: {
                mode: "script_file",
                entry_file: "scripts/shoppe_sync/v0.1.0/index.js",
                simulated: true
            }
        };
    }

    logger.info("脚本执行完成：" + context.script_name + "@" + context.script_version);
    return {
        status: "success",
        result_code: "OK",
        result_message: "脚本入口执行成功",
        step_name: "RUN_SCRIPT_ENTRY",
        extra: {
            mode: "script_file",
            entry_file: "scripts/shoppe_sync/v0.1.0/index.js",
            simulated: true
        }
    };
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        run: run
    };
}

if (typeof globalThis !== "undefined") {
    globalThis.__mobilerpaTaskScriptExport__ = {
        run: run
    };
}

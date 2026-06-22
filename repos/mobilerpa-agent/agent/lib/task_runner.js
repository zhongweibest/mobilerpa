"use strict";

var runtime = require("./runtime");
var centerClient = require("./center_client");

function joinPath() {
    var parts = [];
    var index = 0;

    for (index = 0; index < arguments.length; index += 1) {
        if (arguments[index] === null || arguments[index] === undefined) {
            continue;
        }
        parts.push(String(arguments[index]));
    }

    return parts
        .join("/")
        .replace(/\\/g, "/")
        .replace(/\/+/g, "/")
        .replace(/\/\.\//g, "/")
        .replace(/\/[^\/]+\/\.\./g, "");
}

function resolveAgentRoot() {
    if (typeof __dirname !== "undefined") {
        return joinPath(__dirname, "..");
    }
    if (typeof files !== "undefined" && typeof files.path === "function") {
        return String(files.path(".")).replace(/\\/g, "/");
    }
    return ".";
}

function buildContext(taskSummary, options) {
    var summary = taskSummary || {};
    var config = options || {};
    return {
        task_id: String(summary.task_id || ""),
        script_name: String(summary.script_name || ""),
        script_version: String(summary.script_version || ""),
        priority: Number(summary.priority || 0),
        params: summary.params || {},
        device_id: String(config.deviceID || ""),
        agent_uuid: String(config.agentUUID || ""),
        center_base_url: String(config.centerBaseURL || "")
    };
}

function buildProgressReporter(taskSummary, options, logger) {
    var summary = taskSummary || {};
    var config = options || {};
    var report = typeof config.onProgress === "function" ? config.onProgress : null;

    if (!report && logger && typeof logger.warn === "function") {
        logger.warn("当前任务未注入 onProgress，脚本内部的关键步骤事件不会发送到中心。task_id=" + String(summary.task_id || ""));
    }

    return function reportProgress(stepName, message, status, extra) {
        if (!report) {
            return;
        }
        try {
            report({
                task_id: String(summary.task_id || ""),
                step_name: String(stepName || ""),
                status: String(status || "running"),
                message: String(message || ""),
                extra: extra || {}
            });
        } catch (error) {
            if (logger && typeof logger.warn === "function") {
                logger.warn("任务进度上报回调执行失败：" + String(error));
            }
        }
    };
}

function resolveScriptModule(context) {
    var scriptName = String(context.script_name || "").trim();
    var scriptVersion = String(context.script_version || "").trim();
    var versionDir = scriptVersion || "latest";
    var agentRoot = resolveAgentRoot();
    return {
        versionRoot: joinPath(agentRoot, "scripts", scriptName, versionDir),
        modulePath: joinPath(agentRoot, "scripts", scriptName, versionDir, "index.js"),
        manifestPath: joinPath(agentRoot, "scripts", scriptName, versionDir, "manifest.json"),
        entryLabel: "scripts/" + scriptName + "/" + versionDir + "/index.js"
    };
}

function isSafeRelativePath(value) {
    var normalized = String(value || "").replace(/\\/g, "/").trim();
    var parts = [];
    var index = 0;

    if (!normalized || normalized.indexOf("..") >= 0 || normalized.charAt(0) === "/") {
        return false;
    }

    parts = normalized.split("/");
    for (index = 0; index < parts.length; index += 1) {
        if (!parts[index] || parts[index] === "." || parts[index] === "..") {
            return false;
        }
    }

    return true;
}

function tryRequire(modulePath) {
    var fallbackExportName = "__mobilerpaTaskScriptExport__";

    try {
        if (typeof globalThis !== "undefined") {
            globalThis[fallbackExportName] = null;
        }

        var loaded = require(modulePath);
        if (loaded) {
            return loaded;
        }

        if (typeof globalThis !== "undefined" && globalThis[fallbackExportName]) {
            return globalThis[fallbackExportName];
        }

        return loaded;
    } catch (error) {
        return {
            __load_error__: error
        };
    } finally {
        if (typeof globalThis !== "undefined" && globalThis[fallbackExportName]) {
            globalThis[fallbackExportName] = null;
        }
    }
}

function downloadScriptIfNeeded(context, logger, resolved, options) {
    var syncOptions = options || {};
    var forceDownload = syncOptions.force === true;

    if (!forceDownload && runtime.fileExists(resolved.modulePath)) {
        logger.info("脚本版本已存在，跳过下载：" + context.script_name + "@" + context.script_version);
        return {
            downloaded: false,
            entryFile: "index.js",
            files: ["index.js"]
        };
    }

    if (!context.center_base_url) {
        throw new Error("missing_center_base_url");
    }

    if (forceDownload) {
        logger.info("收到强制同步请求，开始覆盖下载脚本：" + context.script_name + "@" + context.script_version);
    } else {
        logger.info("本地缺少脚本版本，开始向中心下载：" + context.script_name + "@" + context.script_version);
    }
    var manifestResp = centerClient.getScriptManifest(context.center_base_url, context.script_name, context.script_version);
    var manifest = manifestResp && manifestResp.data ? manifestResp.data : manifestResp;
    var entryFile = String((manifest && manifest.entry_file) || "index.js").trim() || "index.js";
    var files = manifest && manifest.files ? manifest.files : [];
    var index = 0;
    var relativePath = "";
    var targetPath = "";
    var fileContent = null;

    if (!isSafeRelativePath(entryFile)) {
        throw new Error("unsupported_entry_file:" + entryFile);
    }

    if (!files || files.length === 0) {
        files = [{
            relative_path: entryFile
        }];
    }

    for (index = 0; index < files.length; index += 1) {
        relativePath = String(files[index].relative_path || "").trim();
        if (!isSafeRelativePath(relativePath)) {
            throw new Error("unsafe_relative_path:" + relativePath);
        }

        targetPath = joinPath(resolved.versionRoot, relativePath);
        fileContent = centerClient.downloadScriptFile(context.center_base_url, context.script_name, context.script_version, relativePath);
        logger.info("正在下载脚本文件：" + relativePath);
        runtime.writeTextFile(targetPath, fileContent);
    }

    runtime.writeTextFile(resolved.manifestPath, JSON.stringify(manifest, null, 2));
    logger.info("脚本下载完成：" + context.script_name + "@" + context.script_version + " -> " + resolved.entryLabel);

    return {
        downloaded: true,
        entryFile: entryFile,
        files: files.map(function (item) {
            return item.relative_path;
        })
    };
}

function loadScriptModule(context, logger) {
    var resolved = resolveScriptModule(context);
    var moduleValue = null;
    var downloadResult = {
        downloaded: false,
        entryFile: "index.js"
    };

    if (!runtime.fileExists(resolved.modulePath)) {
        downloadResult = downloadScriptIfNeeded(context, logger, resolved, {
            force: false
        });
    }

    moduleValue = tryRequire(resolved.modulePath);
    return {
        scriptModule: moduleValue,
        entryLabel: resolved.entryLabel,
        downloadResult: downloadResult
    };
}

function runTask(taskSummary, options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var context = buildContext(taskSummary, options);
    var reportProgress = buildProgressReporter(taskSummary, options, logger);
    var loaded = loadScriptModule(context, logger);
    var scriptModule = loaded.scriptModule;

    reportProgress("LOAD_SCRIPT_ENTRY", "任务执行中：准备加载脚本入口", "running", {
        entry_file: loaded.entryLabel
    });

    if (!scriptModule || scriptModule.__load_error__) {
        reportProgress("LOAD_SCRIPT_ENTRY", "任务执行中：脚本入口加载失败", "failed", {
            entry_file: loaded.entryLabel,
            downloaded: loaded.downloadResult.downloaded
        });
        return {
            status: "failed",
            result_code: "SCRIPT_LOAD_FAILED",
            result_message: "脚本入口加载失败：" + context.script_name + "@" + context.script_version,
            step_name: "LOAD_SCRIPT_ENTRY",
            extra: {
                mode: "script_file",
                entry_file: loaded.entryLabel,
                downloaded: loaded.downloadResult.downloaded
            }
        };
    }

    if (typeof scriptModule.run !== "function") {
        reportProgress("LOAD_SCRIPT_ENTRY", "任务执行中：脚本入口未导出 run 函数", "failed", {
            entry_file: loaded.entryLabel,
            downloaded: loaded.downloadResult.downloaded
        });
        return {
            status: "failed",
            result_code: "SCRIPT_ENTRY_INVALID",
            result_message: "脚本入口未导出 run(context, helpers) 函数",
            step_name: "LOAD_SCRIPT_ENTRY",
            extra: {
                mode: "script_file",
                entry_file: loaded.entryLabel,
                downloaded: loaded.downloadResult.downloaded
            }
        };
    }

    logger.info("开始执行脚本入口：" + context.script_name + "@" + context.script_version + " -> " + loaded.entryLabel);
    reportProgress("RUN_SCRIPT_ENTRY", "任务执行中：开始执行脚本入口", "running", {
        entry_file: loaded.entryLabel,
        downloaded: loaded.downloadResult.downloaded
    });
    var result = scriptModule.run(context, {
        runtime: runtime,
        logger: logger,
        reportProgress: reportProgress
    });
    reportProgress(String((result && result.step_name) || "COMPLETE"), "任务执行中：" + String((result && result.result_message) || "脚本执行结束"), String((result && result.status) || "success"), {
        result_code: String((result && result.result_code) || "")
    });
    return result;
}

function syncScriptVersion(scriptName, scriptVersion, options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var context = {
        script_name: String(scriptName || "").trim(),
        script_version: String(scriptVersion || "").trim(),
        center_base_url: String(options && options.centerBaseURL ? options.centerBaseURL : "").trim()
    };
    var resolved = resolveScriptModule(context);
    var downloadResult = downloadScriptIfNeeded(context, logger, resolved, {
        force: !!(options && options.force)
    });

    return {
        entry_file: downloadResult.entryFile,
        files: downloadResult.files || [],
        entry_label: resolved.entryLabel,
        version_root: resolved.versionRoot
    };
}

module.exports = {
    runTask: runTask,
    syncScriptVersion: syncScriptVersion
};

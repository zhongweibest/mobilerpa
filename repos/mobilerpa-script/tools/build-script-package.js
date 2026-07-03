"use strict";

var esbuild = require("../node_modules/esbuild");
var fs = require("fs");
var path = require("path");
var childProcess = require("child_process");

function readJson(filePath) {
    return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function ensureDir(dirPath) {
    fs.mkdirSync(dirPath, { recursive: true });
}

function writeJson(filePath, data) {
    fs.writeFileSync(filePath, JSON.stringify(data, null, 2), "utf8");
}

function buildDebugEntry(config) {
    return `"use strict";

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
        task_id: ${JSON.stringify(config.debugTaskId)},
        script_name: ${JSON.stringify(config.scriptName)},
        script_version: ${JSON.stringify(config.scriptVersion)},
        priority: 3,
        params: ${JSON.stringify(config.debugParams || {}, null, 2)},
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

    logger.info("开始直接调试 ${config.scriptName}@${config.scriptVersion}");
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
`;
}

function buildManifest(config) {
    var files = [
        { relative_path: "index.js" },
        { relative_path: "index_debug.js" },
        { relative_path: "manifest.json" }
    ];

    if (Array.isArray(config.extraReleaseFiles)) {
        config.extraReleaseFiles.forEach(function (relativePath) {
            files.push({ relative_path: relativePath });
        });
    }

    return {
        entry_file: "index.js",
        files: files
    };
}

function rebuildZip(versionDir, scriptVersion) {
    var zipPath = path.join(versionDir, scriptVersion + ".zip");
    if (fs.existsSync(zipPath)) {
        fs.unlinkSync(zipPath);
    }

    var command = "Compress-Archive -Path (Get-ChildItem -LiteralPath '" +
        versionDir.replace(/'/g, "''") +
        "' -Force | Where-Object { $_.Name -ne '" +
        scriptVersion.replace(/'/g, "''") +
        ".zip' } | Select-Object -ExpandProperty FullName) -DestinationPath '" +
        zipPath.replace(/'/g, "''") +
        "' -Force";

    childProcess.execFileSync("powershell.exe", ["-NoProfile", "-Command", command], {
        stdio: "inherit"
    });
}

async function buildScriptPackage(packageDir) {
    var configPath = path.resolve(packageDir, "script.config.json");
    var config = readJson(configPath);
    var versionDir = path.resolve(packageDir, config.outputRoot, config.scriptVersion);
    var entryPath = path.resolve(packageDir, "src", "index.ts");
    var targetIndexPath = path.resolve(versionDir, "index.js");
    var targetDebugPath = path.resolve(versionDir, "index_debug.js");
    var targetManifestPath = path.resolve(versionDir, "manifest.json");

    ensureDir(versionDir);

    await esbuild.build({
        entryPoints: [entryPath],
        bundle: true,
        platform: "neutral",
        format: "cjs",
        target: ["es2019"],
        outfile: targetIndexPath,
        charset: "utf8",
        banner: {
            js: "\"use strict\";"
        }
    });

    fs.writeFileSync(targetDebugPath, buildDebugEntry(config), "utf8");
    writeJson(targetManifestPath, buildManifest(config));
    rebuildZip(versionDir, config.scriptVersion);

    console.log("已更新脚本产物: " + targetIndexPath);
    console.log("已更新调试入口: " + targetDebugPath);
    console.log("已更新清单文件: " + targetManifestPath);
    console.log("已更新压缩包: " + path.resolve(versionDir, config.scriptVersion + ".zip"));
}

if (require.main === module) {
    buildScriptPackage(process.cwd()).catch(function (error) {
        console.error("构建脚本失败: " + String(error && error.stack ? error.stack : error));
        process.exit(1);
    });
}

module.exports = {
    buildScriptPackage: buildScriptPackage
};

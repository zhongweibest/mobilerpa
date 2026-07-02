"use strict";

/**
 * 运行时工具模块负责屏蔽 Node.js 和 AutoJs6 的基础差异。
 */

/**
 * 设备信息用于注册接口上报。
 *
 * @typedef {Object} DeviceInfo
 * @property {string} device_name 设备可读名称。
 * @property {string} brand 设备品牌。
 * @property {string} model 设备型号。
 * @property {string} android_id Android 标识。
 * @property {string} adb_serial ADB 序列号。
 */

/**
 * 判断当前是否运行在 Node.js 环境。
 *
 * @returns {boolean} 当前运行时是否为 Node.js。
 */
function isNodeRuntime() {
    return typeof process !== "undefined" && !!(process.versions && process.versions.node);
}

/**
 * 判断当前是否运行在 AutoJs6 或兼容环境。
 *
 * @returns {boolean} 当前运行时是否具备 AutoJs 常见全局对象。
 */
function isAutoJsRuntime() {
    return typeof files !== "undefined" || typeof device !== "undefined" || typeof http !== "undefined";
}

/**
 * 创建统一日志对象。
 *
 * @returns {{info: Function, warn: Function, error: Function}} 日志对象。
 */
function createLogger() {
    var write = typeof log === "function"
        ? log
        : function (message) {
            console.log(message);
        };

    return {
        info: function (message) {
            write("[INFO] " + message);
        },
        warn: function (message) {
            write("[WARN] " + message);
        },
        error: function (message) {
            write("[ERROR] " + message);
        }
    };
}

/**
 * 获取当前 UTC 时间字符串。
 *
 * @returns {string} ISO 8601 时间字符串。
 */
function nowISOString() {
    return new Date().toISOString();
}

/**
 * 生成短随机文本。
 *
 * @param {number} length 期望长度。
 * @returns {string} 随机文本。
 */
function randomText(length) {
    var alphabet = "0123456789abcdefghijklmnopqrstuvwxyz";
    var result = "";

    if (isNodeRuntime()) {
        try {
            var crypto = require("crypto");
            var bytes = crypto.randomBytes(length);
            for (var i = 0; i < length; i += 1) {
                result += alphabet[bytes[i] % alphabet.length];
            }
            return result;
        } catch (error) {
            // 如果 Node 加密模块不可用，继续使用通用随机逻辑。
        }
    }

    for (var j = 0; j < length; j += 1) {
        result += alphabet[Math.floor(Math.random() * alphabet.length)];
    }
    return result;
}

/**
 * 把任意文本哈希为稳定十六进制字符串。
 *
 * @param {string} text 原始文本。
 * @returns {string} 哈希结果。
 */
function hashText(text) {
    var input = String(text || "");

    if (isNodeRuntime()) {
        try {
            var crypto = require("crypto");
            return crypto.createHash("sha1").update(input, "utf8").digest("hex");
        } catch (error) {
            // 继续使用通用哈希逻辑。
        }
    }

    if (isAutoJsRuntime() && typeof java !== "undefined") {
        try {
            var MessageDigest = java.security.MessageDigest;
            var StringClass = java.lang.String;
            var digest = MessageDigest.getInstance("SHA-1");
            var bytes = new StringClass(input).getBytes("UTF-8");
            var hashBytes = digest.digest(bytes);
            var result = "";
            for (var i = 0; i < hashBytes.length; i += 1) {
                var value = hashBytes[i];
                if (value < 0) {
                    value += 256;
                }
                var piece = value.toString(16);
                if (piece.length < 2) {
                    piece = "0" + piece;
                }
                result += piece;
            }
            return result;
        } catch (error) {
            // 继续使用通用哈希逻辑。
        }
    }

    var hash = 2166136261;
    for (var index = 0; index < input.length; index += 1) {
        hash ^= input.charCodeAt(index);
        hash += (hash << 1) + (hash << 4) + (hash << 7) + (hash << 8) + (hash << 24);
    }
    var normalized = (hash >>> 0).toString(16);
    while (normalized.length < 8) {
        normalized = "0" + normalized;
    }
    return normalized;
}

/**
 * 生成稳定设备指纹文本。
 *
 * @param {DeviceInfo} deviceInfo 设备信息。
 * @returns {string} 稳定指纹原文。
 */
function buildStableFingerprint(deviceInfo) {
    var info = deviceInfo || {};
    var parts = [
        "android_id=" + String(info.android_id || ""),
        "adb_serial=" + String(info.adb_serial || ""),
        "brand=" + String(info.brand || ""),
        "model=" + String(info.model || ""),
        "device_name=" + String(info.device_name || "")
    ];
    return parts.join("|");
}

/**
 * 生成随机 Agent 标识。
 *
 * @returns {string} 新的 Agent UUID。
 */
function createAgentUUID() {
    return "agent_" + Date.now().toString(36) + "_" + randomText(8);
}

/**
 * 根据设备稳定指纹生成可复用的 Agent 标识。
 *
 * @param {DeviceInfo} deviceInfo 设备信息。
 * @returns {string} 稳定 Agent UUID。
 */
function createStableAgentUUID(deviceInfo) {
    var fingerprint = buildStableFingerprint(deviceInfo);
    return "agent_" + hashText(fingerprint).slice(0, 16);
}

/**
 * 安全调用可能不存在的函数。
 *
 * @param {Function} getter 取值函数。
 * @param {string} fallback 兜底值。
 * @returns {string} 字符串结果。
 */
function safeString(getter, fallback) {
    try {
        var value = getter();
        if (value === null || value === undefined) {
            return fallback;
        }
        return String(value);
    } catch (error) {
        return fallback;
    }
}

/**
 * 从 AutoJs6 全局对象采集设备信息。
 *
 * @returns {DeviceInfo} 设备信息。
 */
function collectAutoJsDeviceInfo() {
    return {
        device_name: safeString(function () {
            return device.device || device.product || device.model;
        }, "AutoJs Device"),
        brand: safeString(function () {
            return device.brand;
        }, "unknown"),
        model: safeString(function () {
            return device.model;
        }, "unknown"),
        android_id: safeString(function () {
            return typeof device.getAndroidId === "function" ? device.getAndroidId() : "";
        }, ""),
        adb_serial: safeString(function () {
            return device.serial || "";
        }, "")
    };
}

/**
 * 从 Node.js 环境采集用于本地验证的设备信息。
 *
 * @returns {DeviceInfo} 设备信息。
 */
function collectNodeDeviceInfo() {
    var os = require("os");
    return {
        device_name: os.hostname() || "Node Agent",
        brand: "node",
        model: os.platform() + "-" + os.arch(),
        android_id: "",
        adb_serial: ""
    };
}

/**
 * 合并设备信息，允许配置文件覆盖自动采集字段。
 *
 * @param {Partial<DeviceInfo>} overrides 配置覆盖项。
 * @returns {DeviceInfo} 设备信息。
 */
function collectDeviceInfo(overrides) {
    var detected = isAutoJsRuntime() && typeof device !== "undefined"
        ? collectAutoJsDeviceInfo()
        : collectNodeDeviceInfo();
    var custom = overrides || {};

    return {
        device_name: custom.device_name || detected.device_name,
        brand: custom.brand || detected.brand,
        model: custom.model || detected.model,
        android_id: custom.android_id || detected.android_id,
        adb_serial: custom.adb_serial || detected.adb_serial
    };
}

/**
 * 判断指定文件是否存在。
 * @param {string} filePath 文件路径。
 * @returns {boolean} 文件是否存在。
 */
function fileExists(filePath) {
    if (isNodeRuntime()) {
        return require("fs").existsSync(filePath);
    }
    return typeof files !== "undefined" && files.exists(filePath);
}

/**
 * 读取文本文件，不存在时返回空字符串。
 *
 * @param {string} filePath 文件路径。
 * @returns {string} 文件内容。
 */
function readTextFile(filePath) {
    if (!fileExists(filePath)) {
        return "";
    }

    if (isNodeRuntime()) {
        return require("fs").readFileSync(filePath, "utf8");
    }

    if (typeof files !== "undefined" && typeof files.read === "function") {
        return String(files.read(filePath) || "");
    }

    return "";
}

/**
 * 确保目录存在。
 *
 * @param {string} dirPath 目录路径。
 */
function ensureDir(dirPath) {
    if (!dirPath) {
        return;
    }

    if (isNodeRuntime()) {
        require("fs").mkdirSync(dirPath, { recursive: true });
        return;
    }

    var normalizedPath = String(dirPath).replace(/\\/g, "/");

    if (typeof files !== "undefined" && typeof files.ensureDir === "function") {
        files.ensureDir(normalizedPath);
    }

    if (typeof java !== "undefined" && java.io && java.io.File) {
        var directory = new java.io.File(normalizedPath);
        if (!directory.exists()) {
            directory.mkdirs();
        }
        if (!directory.exists()) {
            throw new Error("ensure_dir_failed:" + normalizedPath);
        }
        return;
    }

    if (typeof files !== "undefined" && typeof files.exists === "function" && !files.exists(normalizedPath)) {
        throw new Error("ensure_dir_failed:" + normalizedPath);
    }
}

/**
 * 获取绝对路径，优先使用 AutoJs6 的 files.path。
 *
 * @param {string} filePath 原始路径。
 * @returns {string} 绝对路径。
 */
function resolveAbsolutePath(filePath) {
    var input = String(filePath || "");

    if (isNodeRuntime()) {
        return require("path").resolve(input);
    }

    if (typeof files !== "undefined" && typeof files.path === "function") {
        return String(files.path(input) || input).replace(/\\/g, "/");
    }

    return input.replace(/\\/g, "/");
}

/**
 * 写入文本文件。
 *
 * @param {string} filePath 文件路径。
 * @param {string} content 文本内容。
 */
function writeTextFile(filePath, content) {
    var absolutePath = resolveAbsolutePath(filePath);
    if (isNodeRuntime()) {
        ensureDir(require("path").dirname(absolutePath));
        require("fs").writeFileSync(absolutePath, String(content), "utf8");
        return;
    }

    if (typeof files !== "undefined") {
        ensureDir(String(absolutePath).replace(/[\\/][^\\/]+$/, ""));
        files.write(absolutePath, String(content));
    }
}

/**
 * 写入二进制文件。
 *
 * @param {string} filePath 文件路径。
 * @param {*} content 二进制内容或文本内容。
 */
function writeBinaryFile(filePath, content) {
    var absolutePath = resolveAbsolutePath(filePath);
    if (isNodeRuntime()) {
        ensureDir(require("path").dirname(absolutePath));
        require("fs").writeFileSync(absolutePath, content);
        return;
    }

    ensureDir(String(absolutePath).replace(/[\\/][^\\/]+$/, ""));

    if (typeof files !== "undefined" && typeof files.writeBytes === "function") {
        files.writeBytes(absolutePath, content);
        return;
    }

    if (typeof content === "string") {
        files.write(absolutePath, content);
        return;
    }

    if (typeof java !== "undefined" && java.io && java.io.FileOutputStream) {
        var output = null;
        try {
            output = new java.io.FileOutputStream(String(absolutePath));
            output.write(content);
            output.flush();
            return;
        } finally {
            if (output) {
                output.close();
            }
        }
    }

    throw new Error("write_binary_unsupported");
}

/**
 * 删除指定文件，文件不存在时忽略。
 * @param {string} filePath 文件路径。
 */
function removeFileIfExists(filePath) {
    if (!fileExists(filePath)) {
        return;
    }

    if (isNodeRuntime()) {
        require("fs").unlinkSync(filePath);
        return;
    }

    if (typeof files !== "undefined") {
        files.remove(filePath);
    }
}

/**
 * 启动轮询任务。
 * @param {Function} callback 轮询回调。
 * @param {number} intervalMS 轮询间隔，单位毫秒。
 * @returns {{cancel: Function}} 可取消句柄。
 */
function startInterval(callback, intervalMS) {
    if (isNodeRuntime()) {
        var timer = setInterval(callback, intervalMS);
        return {
            cancel: function () {
                clearInterval(timer);
            }
        };
    }

    var thread = threads.start(function () {
        while (true) {
            try {
                callback();
                sleep(intervalMS);
            } catch (error) {
                break;
            }
        }
    });

    return {
        cancel: function () {
            if (thread && thread.isAlive()) {
                thread.interrupt();
            }
        }
    };
}

/**
 * 在后台异步执行任务。
 * @param {Function} callback 后台任务回调。
 * @returns {{cancel: Function}|null} 可选控制句柄。
 */
function runAsync(callback) {
    if (isNodeRuntime()) {
        var timer = setTimeout(function () {
            callback();
        }, 0);
        return {
            cancel: function () {
                clearTimeout(timer);
            }
        };
    }

    if (typeof threads !== "undefined" && typeof threads.start === "function") {
        var thread = threads.start(function () {
            callback();
        });
        return {
            cancel: function () {
                if (thread && thread.isAlive()) {
                    thread.interrupt();
                }
            }
        };
    }

    callback();
    return null;
}

/**
 * 按毫秒休眠。
 * @param {number} milliseconds 休眠时长，单位毫秒。
 */
function sleepMS(milliseconds) {
    var duration = Number(milliseconds || 0);
    if (duration <= 0) {
        return;
    }

    if (isNodeRuntime()) {
        var end = Date.now() + duration;
        while (Date.now() < end) {
            // Node.js 验证环境仅用于最小闭环，这里允许简单阻塞实现。
        }
        return;
    }

    if (typeof sleep === "function") {
        sleep(duration);
    }
}

/**
 * 获取当前运行环境中的 Android Context。
 *
 * @returns {Object|null} Android Context。
 */
function getAndroidContext() {
    if (!isAutoJsRuntime()) {
        return null;
    }

    try {
        if (typeof context !== "undefined" && context) {
            return context;
        }
    } catch (_error) {
        // ignore
    }

    try {
        if (typeof activity !== "undefined" && activity) {
            return activity;
        }
    } catch (_error) {
        // ignore
    }

    return null;
}

/**
 * 归一化执行环境状态。
 *
 * @param {boolean|null} enabled 原始检测结果。
 * @returns {string} enabled / disabled / unknown。
 */
function normalizeExecutionStatus(enabled) {
    if (enabled === true) {
        return "enabled";
    }
    if (enabled === false) {
        return "disabled";
    }
    return "unknown";
}

/**
 * 检查无障碍服务是否已开启。
 *
 * @returns {boolean|null} 检查结果。
 */
function checkAccessibilityEnabled() {
    try {
        if (typeof auto !== "undefined" && auto && auto.service) {
            return true;
        }
        if (typeof auto !== "undefined") {
            return false;
        }
    } catch (_error) {
        return null;
    }
    return null;
}

/**
 * 检查前台服务或通知保活是否可用。
 * 当前用“通知是否可用”作为前台服务能力的代理检查。
 *
 * @returns {boolean|null} 检查结果。
 */
function checkForegroundServiceEnabled() {
    var ctx = getAndroidContext();
    if (!ctx || typeof android === "undefined") {
        return null;
    }

    try {
        if (android.app && android.app.NotificationManager && typeof android.app.NotificationManager.from === "function") {
            return !!android.app.NotificationManager.from(ctx).areNotificationsEnabled();
        }
    } catch (_error) {
        // continue
    }

    try {
        var NotificationManagerCompat = androidx.core.app.NotificationManagerCompat;
        if (NotificationManagerCompat && typeof NotificationManagerCompat.from === "function") {
            return !!NotificationManagerCompat.from(ctx).areNotificationsEnabled();
        }
    } catch (_error) {
        return null;
    }

    return null;
}

/**
 * 检查当前应用是否已放开电量优化限制。
 *
 * @returns {boolean|null} 检查结果。
 */
function checkBatteryOptimizationIgnored() {
    var ctx = getAndroidContext();
    if (!ctx || typeof android === "undefined") {
        return null;
    }

    try {
        var powerServiceName = android.content.Context.POWER_SERVICE;
        var powerManager = ctx.getSystemService(powerServiceName);
        if (!powerManager || typeof powerManager.isIgnoringBatteryOptimizations !== "function") {
            return null;
        }
        return !!powerManager.isIgnoringBatteryOptimizations(String(ctx.getPackageName()));
    } catch (_error) {
        return null;
    }
}

/**
 * 收集当前设备执行环境自检结果。
 *
 * @returns {{accessibility_status: string, foreground_service_status: string, battery_optimization_ignored_status: string, checked_at: string, message: string}} 自检结果。
 */
function collectExecutionProfile() {
    var accessibilityEnabled = checkAccessibilityEnabled();
    var foregroundServiceEnabled = checkForegroundServiceEnabled();
    var batteryOptimizationIgnored = checkBatteryOptimizationIgnored();
    var messages = [];

    messages.push("无障碍=" + normalizeExecutionStatus(accessibilityEnabled));
    messages.push("前台服务=" + normalizeExecutionStatus(foregroundServiceEnabled));
    messages.push("电量优化忽略=" + normalizeExecutionStatus(batteryOptimizationIgnored));

    return {
        accessibility_status: normalizeExecutionStatus(accessibilityEnabled),
        foreground_service_status: normalizeExecutionStatus(foregroundServiceEnabled),
        battery_optimization_ignored_status: normalizeExecutionStatus(batteryOptimizationIgnored),
        checked_at: nowISOString(),
        message: messages.join("；")
    };
}

/**
 * 退出当前脚本。
 * 在 AutoJs6 中使用 exit()；在 Node.js 中仅在显式要求时执行 process.exit。
 * @param {number} code 退出码。
 */
function exitProcess(code) {
    if (isNodeRuntime()) {
        if (typeof process !== "undefined" && typeof process.exit === "function") {
            process.exit(code || 0);
        }
        return;
    }

    if (typeof exit === "function") {
        exit();
    }
}

module.exports = {
    isNodeRuntime: isNodeRuntime,
    isAutoJsRuntime: isAutoJsRuntime,
    createLogger: createLogger,
    nowISOString: nowISOString,
    createAgentUUID: createAgentUUID,
    createStableAgentUUID: createStableAgentUUID,
    collectDeviceInfo: collectDeviceInfo,
    fileExists: fileExists,
    readTextFile: readTextFile,
    resolveAbsolutePath: resolveAbsolutePath,
    ensureDir: ensureDir,
    writeTextFile: writeTextFile,
    writeBinaryFile: writeBinaryFile,
    removeFileIfExists: removeFileIfExists,
    startInterval: startInterval,
    runAsync: runAsync,
    sleepMS: sleepMS,
    collectExecutionProfile: collectExecutionProfile,
    exitProcess: exitProcess
};

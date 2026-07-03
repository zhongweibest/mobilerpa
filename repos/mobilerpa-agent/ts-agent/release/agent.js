"use strict";
(function () {
var __bundleModules = {
  "./lib/center_client": function(module, exports, __bundleRequire) {
"use strict";
module.exports.registerDevice = registerDevice;
module.exports.getScriptManifest = getScriptManifest;
module.exports.downloadScript = downloadScript;
module.exports.downloadScriptFile = downloadScriptFile;
module.exports.downloadScriptFileBytes = downloadScriptFileBytes;
var runtime = __bundleRequire("./lib/runtime");
function nodeRequire(moduleName) {
    return require(moduleName);
}
function parseJSONResponse(statusCode, text) {
    var body = {};
    try {
        body = text ? JSON.parse(text) : {};
    }
    catch (_error) {
        throw new Error("中心服务返回了非 JSON 响应：" + text);
    }
    if (statusCode < 200 || statusCode >= 300) {
        throw new Error("中心服务请求失败，状态码=" + statusCode + "，响应=" + text);
    }
    return body;
}
function formatErrorMessage(error) {
    if (!error) {
        return "unknown_error";
    }
    if (typeof error === "object" && error !== null) {
        var maybeError = error;
        if (maybeError.stack) {
            return String(maybeError.stack);
        }
        if (maybeError.message) {
            return String(maybeError.message);
        }
    }
    return String(error);
}
function trimBaseURL(baseURL) {
    return String(baseURL || "").replace(/\/+$/, "");
}
function createNodeClient(url) {
    return url.protocol === "https:" ? nodeRequire("https") : nodeRequire("http");
}
function nodeGetJSON(url) {
    return new Promise(function resolveJSON(resolve, reject) {
        var target = new URL(url);
        var client = createNodeClient(target);
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function onResponse(response) {
            var chunks = [];
            response.on("data", function onData(chunk) {
                chunks.push(chunk);
            });
            response.on("end", function onEnd() {
                try {
                    resolve(parseJSONResponse(response.statusCode || 200, Buffer.concat(chunks).toString("utf8")));
                }
                catch (error) {
                    reject(error);
                }
            });
        });
        request.on("error", reject);
        request.end();
    });
}
function nodeDownloadText(url) {
    return new Promise(function resolveText(resolve, reject) {
        var target = new URL(url);
        var client = createNodeClient(target);
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function onResponse(response) {
            var chunks = [];
            response.on("data", function onData(chunk) {
                chunks.push(chunk);
            });
            response.on("end", function onEnd() {
                if ((response.statusCode || 200) < 200 || (response.statusCode || 200) >= 300) {
                    reject(new Error("download_failed:" + response.statusCode));
                    return;
                }
                resolve(Buffer.concat(chunks).toString("utf8"));
            });
        });
        request.on("error", reject);
        request.end();
    });
}
function nodeDownloadBuffer(url) {
    return new Promise(function resolveBuffer(resolve, reject) {
        var target = new URL(url);
        var client = createNodeClient(target);
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function onResponse(response) {
            var chunks = [];
            response.on("data", function onData(chunk) {
                chunks.push(chunk);
            });
            response.on("end", function onEnd() {
                if ((response.statusCode || 200) < 200 || (response.statusCode || 200) >= 300) {
                    reject(new Error("download_failed:" + response.statusCode));
                    return;
                }
                resolve(Buffer.concat(chunks));
            });
        });
        request.on("error", reject);
        request.end();
    });
}
function autoJsGetJSON(url) {
    try {
        var response = http.get(url);
        var statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
        var body = response.body && typeof response.body.string === "function"
            ? response.body.string()
            : String(response.body || "");
        return parseJSONResponse(statusCode, body);
    }
    catch (error) {
        throw new Error("AutoJs6 请求中心服务失败，url=" + url + "，error=" + formatErrorMessage(error));
    }
}
function autoJsDownloadText(url) {
    try {
        var response = http.get(url);
        var statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
        var body = response.body && typeof response.body.string === "function"
            ? response.body.string()
            : String(response.body || "");
        if (statusCode < 200 || statusCode >= 300) {
            throw new Error("download_failed:" + statusCode + ":" + body);
        }
        return body;
    }
    catch (error) {
        throw new Error("AutoJs6 下载中心文件失败，url=" + url + "，error=" + formatErrorMessage(error));
    }
}
function autoJsDownloadBytes(url) {
    try {
        var response = http.get(url);
        var statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
        if (statusCode < 200 || statusCode >= 300) {
            var errorBody = response.body && typeof response.body.string === "function"
                ? response.body.string()
                : String(response.body || "");
            throw new Error("download_failed:" + statusCode + ":" + errorBody);
        }
        if (response.body && typeof response.body.bytes === "function") {
            return response.body.bytes();
        }
        if (response.body && typeof response.body.string === "function") {
            return response.body.string();
        }
        return String(response.body || "");
    }
    catch (error) {
        throw new Error("AutoJs6 下载中心文件失败，url=" + url + "，error=" + formatErrorMessage(error));
    }
}
function autoJsPostJSON(url, payload) {
    try {
        var response = http.postJson(url, payload);
        var statusCode = response.statusCode || (typeof response.code === "number" ? response.code : 200);
        var body = response.body && typeof response.body.string === "function"
            ? response.body.string()
            : String(response.body || "");
        return parseJSONResponse(statusCode, body);
    }
    catch (error) {
        throw new Error("AutoJs6 请求中心服务失败，url=" + url + "，error=" + formatErrorMessage(error));
    }
}
function nodePostJSON(url, payload) {
    return new Promise(function resolveJSON(resolve, reject) {
        var target = new URL(url);
        var data = JSON.stringify(payload);
        var client = createNodeClient(target);
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Content-Length": Buffer.byteLength(data)
            }
        }, function onResponse(response) {
            var chunks = [];
            response.on("data", function onData(chunk) {
                chunks.push(chunk);
            });
            response.on("end", function onEnd() {
                try {
                    resolve(parseJSONResponse(response.statusCode || 200, Buffer.concat(chunks).toString("utf8")));
                }
                catch (error) {
                    reject(error);
                }
            });
        });
        request.on("error", reject);
        request.write(data);
        request.end();
    });
}
function registerDevice(centerBaseURL, payload) {
    var url = trimBaseURL(centerBaseURL) + "/api/v1/device/register";
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsPostJSON(url, payload);
    }
    return nodePostJSON(url, payload);
}
function getScriptManifest(centerBaseURL, scriptName, scriptVersion) {
    var url = trimBaseURL(centerBaseURL)
        + "/api/v1/script/manifest?script_name=" + encodeURIComponent(scriptName)
        + "&script_version=" + encodeURIComponent(scriptVersion);
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsGetJSON(url);
    }
    return nodeGetJSON(url);
}
function downloadScript(centerBaseURL, scriptName, scriptVersion) {
    return downloadScriptFile(centerBaseURL, scriptName, scriptVersion, "index.js");
}
function downloadScriptFile(centerBaseURL, scriptName, scriptVersion, relativePath) {
    var filePath = String(relativePath || "index.js");
    var url = trimBaseURL(centerBaseURL)
        + "/api/v1/script/download?script_name=" + encodeURIComponent(scriptName)
        + "&script_version=" + encodeURIComponent(scriptVersion)
        + "&relative_path=" + encodeURIComponent(filePath);
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsDownloadText(url);
    }
    return nodeDownloadText(url);
}
function downloadScriptFileBytes(centerBaseURL, scriptName, scriptVersion, relativePath) {
    var filePath = String(relativePath || "index.js");
    var url = trimBaseURL(centerBaseURL)
        + "/api/v1/script/download?script_name=" + encodeURIComponent(scriptName)
        + "&script_version=" + encodeURIComponent(scriptVersion)
        + "&relative_path=" + encodeURIComponent(filePath);
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsDownloadBytes(url);
    }
    return nodeDownloadBuffer(url);
}

  },
  "./lib/config_store": function(module, exports, __bundleRequire) {
"use strict";
module.exports.defaultConfigPath = defaultConfigPath;
module.exports.defaultBootstrapPath = defaultBootstrapPath;
module.exports.defaultStopSignalPath = defaultStopSignalPath;
module.exports.defaultRuntimeLockPath = defaultRuntimeLockPath;
module.exports.defaultHeartbeatLeasePath = defaultHeartbeatLeasePath;
module.exports.createConfigStore = createConfigStore;
var runtime = __bundleRequire("./lib/runtime");
function isNodeRuntime() {
    return runtime.isNodeRuntime();
}
function nodeRequire(moduleName) {
    return require(moduleName);
}
function joinPath() {
    var parts = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        parts[_i] = arguments[_i];
    }
    var filtered = parts.filter(Boolean);
    if (isNodeRuntime()) {
        var path = nodeRequire("path");
        return path.join.apply(path, filtered);
    }
    return normalizePath(filtered.join("/"));
}
function resolveAgentRootPath() {
    if (isNodeRuntime()) {
        var path = nodeRequire("path");
        return path.resolve(typeof __dirname !== "undefined" ? joinPath(__dirname, "..", "..") : ".");
    }
    if (typeof files !== "undefined" && typeof files.path === "function") {
        return normalizePath(String(files.path(".") || "."));
    }
    if (typeof __dirname !== "undefined") {
        return normalizePath(joinPath(__dirname, "..", ".."));
    }
    return ".";
}
function normalizePath(input) {
    var raw = String(input || "").replace(/\\/g, "/");
    var absolute = raw.charAt(0) === "/";
    var segments = raw.split("/");
    var stack = [];
    for (var index = 0; index < segments.length; index += 1) {
        if (!segments[index] || segments[index] === ".") {
            continue;
        }
        if (segments[index] === "..") {
            if (stack.length > 0 && stack[stack.length - 1] !== "..") {
                stack.pop();
            }
            else if (!absolute) {
                stack.push("..");
            }
            continue;
        }
        stack.push(segments[index]);
    }
    var output = stack.join("/");
    if (absolute) {
        output = "/" + output;
    }
    return output || (absolute ? "/" : ".");
}
function dirname(filePath) {
    if (isNodeRuntime()) {
        var path = nodeRequire("path");
        return path.dirname(filePath);
    }
    var normalized = String(filePath).replace(/\\/g, "/");
    var index = normalized.lastIndexOf("/");
    return index >= 0 ? normalized.slice(0, index) : ".";
}
function defaultConfigPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "config.json");
}
function defaultBootstrapPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "bootstrap.json");
}
function defaultStopSignalPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "stop.signal");
}
function defaultRuntimeLockPath() {
    return joinPath(resolveRuntimeStateRootPath(), "agent.lock.json");
}
function defaultHeartbeatLeasePath() {
    return joinPath(resolveRuntimeStateRootPath(), "heartbeat.lease.json");
}
function resolveRuntimeStateRootPath() {
    if (isNodeRuntime()) {
        return joinPath(resolveAgentRootPath(), "runtime");
    }
    var agentRoot = resolveAgentRootPath();
    var parentRoot = dirname(agentRoot);
    return joinPath(parentRoot, ".mobilerpa-agent-runtime");
}
function exists(filePath) {
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        return fs.existsSync(filePath);
    }
    return typeof files !== "undefined" && files.exists(filePath);
}
function readText(filePath) {
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        return fs.readFileSync(filePath, "utf8");
    }
    return files.read(filePath);
}
function writeText(filePath, content) {
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        fs.mkdirSync(dirname(filePath), { recursive: true });
        fs.writeFileSync(filePath, content, "utf8");
        return;
    }
    if (typeof files !== "undefined") {
        files.createWithDirs(filePath);
        files.write(filePath, content);
    }
}
function createEmptyConfig() {
    var now = runtime.nowISOString();
    return {
        center_base_url: "http://127.0.0.1:18080",
        agent_uuid: "",
        device_id: "",
        device_link_sn: "",
        device: {},
        websocket: {
            enabled: true,
            heartbeat_interval_ms: 30000,
            heartbeat_scheduler: "executor",
            reconnect_enabled: true,
            reconnect_initial_delay_ms: 3000,
            reconnect_max_delay_ms: 60000,
            reconnect_backoff_multiplier: 2
        },
        keep_alive: {
            wake_screen_before_task: true,
            wake_screen_cooldown_seconds: 30,
            ws_watchdog_interval_seconds: 20,
            ws_silence_timeout_seconds: 120
        },
        last_register: {},
        created_at: now,
        updated_at: now
    };
}
function normalizeWebSocketConfig(input, fallback) {
    var source = input || {};
    var base = fallback || {};
    return {
        enabled: source.enabled === false ? false : base.enabled !== false,
        heartbeat_interval_ms: source.heartbeat_interval_ms || base.heartbeat_interval_ms || 30000,
        heartbeat_scheduler: source.heartbeat_scheduler || base.heartbeat_scheduler || "executor",
        reconnect_enabled: source.reconnect_enabled === false ? false : base.reconnect_enabled !== false,
        reconnect_initial_delay_ms: source.reconnect_initial_delay_ms || base.reconnect_initial_delay_ms || 3000,
        reconnect_max_delay_ms: source.reconnect_max_delay_ms || base.reconnect_max_delay_ms || 60000,
        reconnect_backoff_multiplier: source.reconnect_backoff_multiplier || base.reconnect_backoff_multiplier || 2
    };
}
function normalizeConfig(raw) {
    var base = createEmptyConfig();
    var input = raw || {};
    return {
        center_base_url: input.center_base_url || base.center_base_url,
        agent_uuid: input.agent_uuid || "",
        device_id: input.device_id || "",
        device_link_sn: input.device_link_sn || "",
        device: input.device || {},
        websocket: normalizeWebSocketConfig(input.websocket, base.websocket),
        keep_alive: {
            wake_screen_before_task: input.keep_alive && input.keep_alive.wake_screen_before_task === false ? false : base.keep_alive && base.keep_alive.wake_screen_before_task !== false,
            wake_screen_cooldown_seconds: Number((input.keep_alive && input.keep_alive.wake_screen_cooldown_seconds) || (base.keep_alive && base.keep_alive.wake_screen_cooldown_seconds) || 30),
            ws_watchdog_interval_seconds: Number((input.keep_alive && input.keep_alive.ws_watchdog_interval_seconds) || (base.keep_alive && base.keep_alive.ws_watchdog_interval_seconds) || 20),
            ws_silence_timeout_seconds: Number((input.keep_alive && input.keep_alive.ws_silence_timeout_seconds) || (base.keep_alive && base.keep_alive.ws_silence_timeout_seconds) || 120)
        },
        last_register: input.last_register || {},
        created_at: input.created_at || base.created_at,
        updated_at: input.updated_at || base.updated_at
    };
}
function createConfigStore(options) {
    var configPath = options && options.configPath ? options.configPath : defaultConfigPath();
    var bootstrapPath = options && options.bootstrapPath ? options.bootstrapPath : defaultBootstrapPath();
    var stopSignalPath = options && options.stopSignalPath ? options.stopSignalPath : defaultStopSignalPath();
    return {
        configPath: configPath,
        bootstrapPath: bootstrapPath,
        stopSignalPath: stopSignalPath,
        exists: function () {
            return exists(configPath);
        },
        bootstrapExists: function () {
            return exists(bootstrapPath);
        },
        loadBootstrap: function () {
            if (!exists(bootstrapPath)) {
                return {};
            }
            var text = readText(bootstrapPath);
            if (!text || !text.trim()) {
                return {};
            }
            return JSON.parse(text);
        },
        load: function () {
            if (!exists(configPath)) {
                return createEmptyConfig();
            }
            var text = readText(configPath);
            if (!text || !text.trim()) {
                return createEmptyConfig();
            }
            return normalizeConfig(JSON.parse(text));
        },
        save: function (config) {
            var nextConfig = normalizeConfig(config);
            nextConfig.updated_at = runtime.nowISOString();
            writeText(configPath, JSON.stringify(nextConfig, null, 2));
        }
    };
}

  },
  "./lib/heartbeat_scheduler": function(module, exports, __bundleRequire) {
"use strict";
module.exports.createHeartbeatScheduler = createHeartbeatScheduler;
module.exports.normalizeSchedulerMode = normalizeSchedulerMode;
var runtime = __bundleRequire("./lib/runtime");
function javaType(name) {
    if (typeof Java !== "undefined" && typeof Java.type === "function") {
        return Java.type(name);
    }
    var parts = String(name || "").split(".");
    var current = Packages;
    for (var index = 0; index < parts.length; index += 1) {
        current = current[parts[index]];
    }
    return current;
}
function createRunnable(runCallback) {
    var Runnable = javaType("java.lang.Runnable");
    if (typeof JavaAdapter === "function") {
        return new JavaAdapter(Runnable, {
            run: runCallback
        });
    }
    return new Runnable({
        run: runCallback
    });
}
function createSchedulerID(prefix) {
    return String(prefix || "scheduler") + "-" + Date.now().toString(36) + "-" + Math.floor(Math.random() * 100000).toString(36);
}
function createIntervalHeartbeatScheduler() {
    return {
        kind: "interval",
        start: function (options) {
            var intervalMS = Math.max(1000, Number(options.intervalMS || 30000));
            var handle = runtime.startInterval(function onHeartbeatTick() {
                options.onTick();
            }, intervalMS);
            var schedulerID = createSchedulerID("interval");
            return {
                kind: "interval",
                schedulerID: schedulerID,
                cancel: function () {
                    handle.cancel();
                }
            };
        }
    };
}
function createExecutorHeartbeatScheduler() {
    return {
        kind: "executor",
        start: function (options) {
            if (!runtime.isAutoJsRuntime()) {
                return createIntervalHeartbeatScheduler().start(options);
            }
            var TimeUnit = javaType("java.util.concurrent.TimeUnit");
            var Executors = javaType("java.util.concurrent.Executors");
            var intervalMS = Math.max(1000, Number(options.intervalMS || 30000));
            var executor = Executors.newSingleThreadScheduledExecutor();
            var future = executor.scheduleAtFixedRate(createRunnable(function heartbeatTick() {
                options.onTick();
            }), intervalMS, intervalMS, TimeUnit.MILLISECONDS);
            var schedulerID = createSchedulerID("executor");
            return {
                kind: "executor",
                schedulerID: schedulerID,
                cancel: function () {
                    future.cancel(false);
                    executor.shutdownNow();
                }
            };
        }
    };
}
function normalizeSchedulerMode(mode) {
    var nextMode = String(mode || "").toLowerCase();
    if (nextMode === "interval") {
        return "interval";
    }
    return "executor";
}
function createHeartbeatScheduler(mode, logger) {
    var normalizedMode = normalizeSchedulerMode(mode);
    if (normalizedMode === "interval") {
        return createIntervalHeartbeatScheduler();
    }
    return createExecutorHeartbeatScheduler();
}

  },
  "./lib/runtime": function(module, exports, __bundleRequire) {
"use strict";
module.exports.isNodeRuntime = isNodeRuntime;
module.exports.isAutoJsRuntime = isAutoJsRuntime;
module.exports.createLogger = createLogger;
module.exports.nowISOString = nowISOString;
module.exports.createAgentUUID = createAgentUUID;
module.exports.createStableAgentUUID = createStableAgentUUID;
module.exports.collectDeviceInfo = collectDeviceInfo;
module.exports.fileExists = fileExists;
module.exports.readTextFile = readTextFile;
module.exports.resolveAbsolutePath = resolveAbsolutePath;
module.exports.ensureDir = ensureDir;
module.exports.writeTextFile = writeTextFile;
module.exports.writeBinaryFile = writeBinaryFile;
module.exports.removeFileIfExists = removeFileIfExists;
module.exports.startInterval = startInterval;
module.exports.runAsync = runAsync;
module.exports.sleepMS = sleepMS;
module.exports.getAndroidContext = getAndroidContext;
module.exports.collectExecutionProfile = collectExecutionProfile;
module.exports.exitProcess = exitProcess;
function isNodeRuntime() {
    return typeof process !== "undefined" && !!(process.versions && process.versions.node);
}
function isAutoJsRuntime() {
    return typeof files !== "undefined" || typeof device !== "undefined" || typeof http !== "undefined";
}
function nodeRequire(moduleName) {
    return require(moduleName);
}
function createLogger() {
    var write = typeof log === "function"
        ? log
        : function writeByConsole(message) {
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
function nowISOString() {
    return new Date().toISOString();
}
function randomText(length) {
    var alphabet = "0123456789abcdefghijklmnopqrstuvwxyz";
    var result = "";
    if (isNodeRuntime()) {
        try {
            var crypto_1 = nodeRequire("crypto");
            var bytes = crypto_1.randomBytes(length);
            for (var index = 0; index < length; index += 1) {
                result += alphabet[bytes[index] % alphabet.length];
            }
            return result;
        }
        catch (_error) {
            // 如果 Node 加密模块不可用，继续使用通用随机逻辑。
        }
    }
    for (var index = 0; index < length; index += 1) {
        result += alphabet[Math.floor(Math.random() * alphabet.length)];
    }
    return result;
}
function hashText(text) {
    var input = String(text || "");
    if (isNodeRuntime()) {
        try {
            var crypto_2 = nodeRequire("crypto");
            return crypto_2.createHash("sha1").update(input, "utf8").digest("hex");
        }
        catch (_error) {
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
            for (var index = 0; index < hashBytes.length; index += 1) {
                var value = hashBytes[index];
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
        }
        catch (_error) {
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
function buildStableFingerprint(deviceInfo) {
    var info = deviceInfo || {
        device_name: "",
        brand: "",
        model: "",
        android_id: "",
        adb_serial: ""
    };
    return "android_id=" + String(info.android_id || "");
}
function createAgentUUID() {
    return "agent_" + Date.now().toString(36) + "_" + randomText(8);
}
function createStableAgentUUID(deviceInfo) {
    var fingerprint = buildStableFingerprint(deviceInfo);
    return "agent_" + hashText(fingerprint).slice(0, 16);
}
function safeString(getter, fallback) {
    try {
        var value = getter();
        if (value === null || value === undefined) {
            return fallback;
        }
        return String(value);
    }
    catch (_error) {
        return fallback;
    }
}
function collectAutoJsDeviceInfo() {
    return {
        device_name: safeString(function getDeviceName() {
            return device.device || device.product || device.model;
        }, "AutoJs Device"),
        brand: safeString(function getBrand() {
            return device.brand;
        }, "unknown"),
        model: safeString(function getModel() {
            return device.model;
        }, "unknown"),
        android_id: safeString(function getAndroidID() {
            return typeof device.getAndroidId === "function" ? device.getAndroidId() : "";
        }, ""),
        adb_serial: safeString(function getADBSerial() {
            return device.serial || "";
        }, "")
    };
}
function collectNodeDeviceInfo() {
    var os = nodeRequire("os");
    return {
        device_name: os.hostname() || "Node Agent",
        brand: "node",
        model: os.platform() + "-" + os.arch(),
        android_id: "",
        adb_serial: ""
    };
}
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
function fileExists(filePath) {
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        return fs.existsSync(filePath);
    }
    return typeof files !== "undefined" && files.exists(filePath);
}
function readTextFile(filePath) {
    if (!fileExists(filePath)) {
        return "";
    }
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        return fs.readFileSync(filePath, "utf8");
    }
    if (typeof files !== "undefined" && typeof files.read === "function") {
        return String(files.read(filePath) || "");
    }
    return "";
}
function ensureDir(dirPath) {
    if (!dirPath) {
        return;
    }
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        fs.mkdirSync(dirPath, { recursive: true });
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
function resolveAbsolutePath(filePath) {
    var input = String(filePath || "");
    if (isNodeRuntime()) {
        var path = nodeRequire("path");
        return path.resolve(input);
    }
    if (typeof files !== "undefined" && typeof files.path === "function") {
        return String(files.path(input) || input).replace(/\\/g, "/");
    }
    return input.replace(/\\/g, "/");
}
function writeTextFile(filePath, content) {
    var absolutePath = resolveAbsolutePath(filePath);
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        var path = nodeRequire("path");
        ensureDir(path.dirname(absolutePath));
        fs.writeFileSync(absolutePath, String(content), "utf8");
        return;
    }
    if (typeof files !== "undefined") {
        ensureDir(String(absolutePath).replace(/[\\/][^\\/]+$/, ""));
        files.write(absolutePath, String(content));
    }
}
function writeBinaryFile(filePath, content) {
    var absolutePath = resolveAbsolutePath(filePath);
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        var path = nodeRequire("path");
        ensureDir(path.dirname(absolutePath));
        fs.writeFileSync(absolutePath, content);
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
        }
        finally {
            if (output) {
                output.close();
            }
        }
    }
    throw new Error("write_binary_unsupported");
}
function removeFileIfExists(filePath) {
    if (!fileExists(filePath)) {
        return;
    }
    if (isNodeRuntime()) {
        var fs = nodeRequire("fs");
        fs.unlinkSync(filePath);
        return;
    }
    if (typeof files !== "undefined") {
        files.remove(filePath);
    }
}
function startInterval(callback, intervalMS) {
    if (isNodeRuntime()) {
        var timer_1 = setInterval(callback, intervalMS);
        return {
            cancel: function () {
                clearInterval(timer_1);
            }
        };
    }
    var thread = threads.start(function runIntervalLoop() {
        while (true) {
            try {
                callback();
                sleep(intervalMS);
            }
            catch (_error) {
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
function runAsync(callback) {
    if (isNodeRuntime()) {
        var timer_2 = setTimeout(function runTask() {
            callback();
        }, 0);
        return {
            cancel: function () {
                clearTimeout(timer_2);
            }
        };
    }
    if (typeof threads !== "undefined" && typeof threads.start === "function") {
        var thread_1 = threads.start(function runInThread() {
            callback();
        });
        return {
            cancel: function () {
                if (thread_1 && thread_1.isAlive()) {
                    thread_1.interrupt();
                }
            }
        };
    }
    callback();
    return null;
}
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
function getAndroidContext() {
    if (!isAutoJsRuntime()) {
        return null;
    }
    try {
        if (typeof context !== "undefined" && context) {
            return context;
        }
    }
    catch (_error) {
        // ignore
    }
    try {
        if (typeof activity !== "undefined" && activity) {
            return activity;
        }
    }
    catch (_error) {
        // ignore
    }
    return null;
}
function normalizeExecutionStatus(enabled) {
    if (enabled === true) {
        return "enabled";
    }
    if (enabled === false) {
        return "disabled";
    }
    return "unknown";
}
function checkAccessibilityEnabled() {
    try {
        if (typeof auto !== "undefined" && auto && auto.service) {
            return true;
        }
        if (typeof auto !== "undefined") {
            return false;
        }
    }
    catch (_error) {
        return null;
    }
    return null;
}
function checkForegroundServiceEnabled() {
    var ctx = getAndroidContext();
    if (!ctx || typeof android === "undefined") {
        return null;
    }
    try {
        if (android.app && android.app.NotificationManager && typeof android.app.NotificationManager.from === "function") {
            return !!android.app.NotificationManager.from(ctx).areNotificationsEnabled();
        }
    }
    catch (_error) {
        // continue
    }
    try {
        var NotificationManagerCompat = androidx.core.app.NotificationManagerCompat;
        if (NotificationManagerCompat && typeof NotificationManagerCompat.from === "function") {
            return !!NotificationManagerCompat.from(ctx).areNotificationsEnabled();
        }
    }
    catch (_error) {
        return null;
    }
    return null;
}
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
    }
    catch (_error) {
        return null;
    }
}
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

  },
  "./lib/task_runner": function(module, exports, __bundleRequire) {
"use strict";
module.exports.runTask = runTask;
module.exports.syncScriptVersion = syncScriptVersion;
module.exports.ensureScriptVersion = ensureScriptVersion;
module.exports.buildContext = buildContext;
module.exports.resolveScriptModule = resolveScriptModule;
module.exports.isSafeRelativePath = isSafeRelativePath;
module.exports.normalizeModulePath = normalizeModulePath;
var runtime = __bundleRequire("./lib/runtime");
var centerClient = __bundleRequire("./lib/center_client");
function joinPath() {
    var args = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        args[_i] = arguments[_i];
    }
    var parts = [];
    for (var index = 0; index < args.length; index += 1) {
        if (args[index] === null || args[index] === undefined) {
            continue;
        }
        parts.push(String(args[index]));
    }
    return parts
        .join("/")
        .replace(/\\/g, "/")
        .replace(/\/+/g, "/")
        .replace(/\/\.\//g, "/")
        .replace(/\/[^/]+\/\.\./g, "");
}
function resolveAgentRoot() {
    if (typeof __dirname !== "undefined") {
        return joinPath(__dirname, "..", "..");
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
        params: (summary.params || {}),
        device_id: String(config.deviceID || ""),
        agent_uuid: String(config.agentUUID || ""),
        center_base_url: String(config.centerBaseURL || "")
    };
}
function buildProgressReporter(taskSummary, options, logger) {
    var summary = taskSummary || {};
    var config = options || {};
    var report = typeof config.onProgress === "function" ? config.onProgress : null;
    var lastProgress = null;
    if (!report && logger && typeof logger.warn === "function") {
        logger.warn("当前任务未注入 onProgress，脚本内部的关键步骤事件不会发送到中心。task_id=" + String(summary.task_id || ""));
    }
    return {
        reportProgress: function (stepName, message, status, extra) {
            lastProgress = {
                task_id: String(summary.task_id || ""),
                step_name: String(stepName || ""),
                status: String(status || "running"),
                message: String(message || ""),
                extra: extra || {}
            };
            if (!report) {
                return;
            }
            try {
                report(lastProgress);
            }
            catch (error) {
                if (logger && typeof logger.warn === "function") {
                    logger.warn("任务进度上报回调执行失败：" + String(error));
                }
            }
        },
        getLastProgress: function () {
            return lastProgress;
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
    if (!normalized || normalized.indexOf("..") >= 0 || normalized.charAt(0) === "/") {
        return false;
    }
    var parts = normalized.split("/");
    for (var index = 0; index < parts.length; index += 1) {
        if (!parts[index] || parts[index] === "." || parts[index] === "..") {
            return false;
        }
    }
    return true;
}
function dirnameOfPath(filePath) {
    var normalized = String(filePath || "").replace(/\\/g, "/");
    var index = normalized.lastIndexOf("/");
    if (index < 0) {
        return ".";
    }
    return normalized.slice(0, index) || "/";
}
function normalizeModulePath(filePath) {
    var normalized = String(filePath || "").replace(/\\/g, "/");
    var absolute = normalized.charAt(0) === "/";
    var parts = normalized.split("/");
    var output = [];
    for (var index = 0; index < parts.length; index += 1) {
        var part = parts[index];
        if (!part || part === ".") {
            continue;
        }
        if (part === "..") {
            if (output.length > 0) {
                output.pop();
            }
            continue;
        }
        output.push(part);
    }
    return (absolute ? "/" : "") + output.join("/");
}
function resolveAutoJsModuleFile(baseFilePath, requestPath) {
    var baseDir = dirnameOfPath(baseFilePath);
    var normalizedRequest = String(requestPath || "").replace(/\\/g, "/");
    var rawPath = normalizedRequest.indexOf("./") === 0 || normalizedRequest.indexOf("../") === 0
        ? normalizeModulePath(baseDir + "/" + normalizedRequest)
        : normalizedRequest;
    var candidates = [rawPath];
    if (!/\.js$/i.test(rawPath)) {
        candidates.push(rawPath + ".js");
        candidates.push(normalizeModulePath(rawPath + "/index.js"));
    }
    for (var index = 0; index < candidates.length; index += 1) {
        if (runtime.fileExists(candidates[index])) {
            return candidates[index];
        }
    }
    throw new Error("script_module_not_found:" + requestPath + " from " + baseFilePath);
}
function loadAutoJsModule(entryPath, globalObject) {
    var moduleCache = {};
    var fallbackExportName = "__mobilerpaTaskScriptExport__";
    var fallbackModuleExportName = "__mobilerpaTaskScriptModuleExports__";
    function executeModule(filePath) {
        var normalizedFilePath = normalizeModulePath(filePath);
        if (moduleCache[normalizedFilePath]) {
            return moduleCache[normalizedFilePath];
        }
        var sourceText = runtime.readTextFile(normalizedFilePath);
        if (!sourceText) {
            throw new Error("script_file_not_found:" + normalizedFilePath);
        }
        var moduleRecord = {
            exports: {}
        };
        moduleCache[normalizedFilePath] = moduleRecord.exports;
        var localRequire = function localRequire(requestPath) {
            if (String(requestPath || "").indexOf(".") !== 0) {
                throw new Error("unsupported_script_require:" + requestPath);
            }
            var targetFilePath = resolveAutoJsModuleFile(normalizedFilePath, requestPath);
            return executeModule(targetFilePath);
        };
        var wrapper = new Function("require", "module", "exports", sourceText + "\nreturn module.exports;");
        var result = wrapper(localRequire, moduleRecord, moduleRecord.exports);
        var exported = result || moduleRecord.exports;
        if (globalObject
            && normalizedFilePath === normalizeModulePath(entryPath)
            && (!exported || Object.keys(exported).length === 0)) {
            if (globalObject[fallbackExportName]) {
                moduleCache[normalizedFilePath] = globalObject[fallbackExportName];
            }
            else if (globalObject[fallbackModuleExportName]) {
                moduleCache[normalizedFilePath] = globalObject[fallbackModuleExportName];
            }
        }
        else {
            moduleCache[normalizedFilePath] = exported || {};
        }
        return moduleCache[normalizedFilePath];
    }
    return executeModule(entryPath);
}
function tryRequire(modulePath) {
    var fallbackExportName = "__mobilerpaTaskScriptExport__";
    var fallbackModuleExportName = "__mobilerpaTaskScriptModuleExports__";
    var globalObject = typeof globalThis !== "undefined"
        ? globalThis
        : (typeof global !== "undefined" ? global : undefined);
    try {
        if (globalObject) {
            globalObject[fallbackExportName] = null;
            globalObject[fallbackModuleExportName] = null;
        }
        if (runtime.isAutoJsRuntime()) {
            var result = loadAutoJsModule(modulePath, globalObject);
            if (result && Object.keys(result).length > 0) {
                return result;
            }
            if (globalObject && globalObject[fallbackExportName]) {
                return globalObject[fallbackExportName];
            }
            if (globalObject && globalObject[fallbackModuleExportName]) {
                return globalObject[fallbackModuleExportName];
            }
            return result || {};
        }
        var loaded = require(modulePath);
        if (loaded) {
            return loaded;
        }
        if (globalObject && globalObject[fallbackExportName]) {
            return globalObject[fallbackExportName];
        }
        return loaded;
    }
    catch (error) {
        return {
            __load_error__: error
        };
    }
    finally {
        if (globalObject && globalObject[fallbackExportName]) {
            globalObject[fallbackExportName] = null;
        }
        if (globalObject && globalObject[fallbackModuleExportName]) {
            globalObject[fallbackModuleExportName] = null;
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
    }
    else {
        logger.info("本地缺少脚本版本，开始向中心下载：" + context.script_name + "@" + context.script_version);
    }
    var manifestResp = centerClient.getScriptManifest(context.center_base_url, context.script_name, context.script_version);
    var manifest = (manifestResp && "data" in manifestResp && manifestResp.data ? manifestResp.data : manifestResp);
    var entryFile = String((manifest && manifest.entry_file) || "index.js").trim() || "index.js";
    var files = manifest && manifest.files ? manifest.files : [];
    if (!isSafeRelativePath(entryFile)) {
        throw new Error("unsupported_entry_file:" + entryFile);
    }
    if (!files || files.length === 0) {
        files = [{
                relative_path: entryFile
            }];
    }
    for (var index = 0; index < files.length; index += 1) {
        var relativePath = String(files[index].relative_path || "").trim();
        if (!isSafeRelativePath(relativePath)) {
            throw new Error("unsafe_relative_path:" + relativePath);
        }
        var targetPath = joinPath(resolved.versionRoot, relativePath);
        var fileContent = centerClient.downloadScriptFile(context.center_base_url, context.script_name, context.script_version, relativePath);
        logger.info("正在下载脚本文件：" + relativePath);
        runtime.writeTextFile(targetPath, fileContent);
    }
    runtime.writeTextFile(resolved.manifestPath, JSON.stringify(manifest, null, 2));
    logger.info("脚本下载完成：" + context.script_name + "@" + context.script_version + " -> " + resolved.entryLabel);
    return {
        downloaded: true,
        entryFile: entryFile,
        files: files.map(function mapRelativePath(item) {
            return item.relative_path;
        })
    };
}
function loadScriptModule(context, logger) {
    var resolved = resolveScriptModule(context);
    var downloadResult = {
        downloaded: false,
        entryFile: "index.js"
    };
    if (!runtime.fileExists(resolved.modulePath)) {
        downloadResult = downloadScriptIfNeeded(context, logger, resolved, {
            force: false
        });
    }
    var moduleValue = tryRequire(resolved.modulePath);
    return {
        scriptModule: moduleValue,
        entryLabel: resolved.entryLabel,
        downloadResult: downloadResult
    };
}
function ensureScriptVersion(scriptName, scriptVersion, options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var context = {
        script_name: String(scriptName || "").trim(),
        script_version: String(scriptVersion || "").trim(),
        center_base_url: String(options && options.centerBaseURL ? options.centerBaseURL : "").trim()
    };
    var resolved = resolveScriptModule(context);
    return downloadScriptIfNeeded(context, logger, resolved, {
        force: !!(options && options.force)
    });
}
function runTask(taskSummary, options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var context = buildContext(taskSummary, options);
    var progressReporter = buildProgressReporter(taskSummary, options, logger);
    var reportProgress = progressReporter.reportProgress;
    var isCancelled = options && typeof options.isCancelled === "function"
        ? options.isCancelled
        : function neverCancelled() { return false; };
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
    if (isCancelled()) {
        reportProgress("RUN_SCRIPT_ENTRY", "任务执行中：任务已取消，跳过脚本执行", "stopped", {
            entry_file: loaded.entryLabel
        });
        return {
            status: "stopped",
            result_code: "TASK_CANCELLED",
            result_message: "任务已取消",
            step_name: "RUN_SCRIPT_ENTRY",
            extra: {
                mode: "script_file",
                entry_file: loaded.entryLabel
            }
        };
    }
    function throwIfCancelled(message) {
        if (!isCancelled()) {
            return;
        }
        var error = new Error(String(message || "任务已取消"));
        error.code = "TASK_CANCELLED";
        error.isCancelled = true;
        throw error;
    }
    var result = scriptModule.run(context, {
        runtime: runtime,
        logger: logger,
        reportProgress: reportProgress,
        isCancelled: isCancelled,
        throwIfCancelled: throwIfCancelled
    });
    var lastProgress = progressReporter.getLastProgress();
    var expectedStepName = String((result && result.step_name) || "COMPLETE");
    var expectedStatus = String((result && result.status) || "success");
    var expectedMessage = "任务执行中：" + String((result && result.result_message) || "脚本执行结束");
    if (!lastProgress
        || String(lastProgress.step_name || "") !== expectedStepName
        || String(lastProgress.status || "") !== expectedStatus
        || String(lastProgress.message || "") !== expectedMessage) {
        reportProgress(expectedStepName, expectedMessage, expectedStatus, {
            result_code: String((result && result.result_code) || "")
        });
    }
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

  },
  "./lib/workflow_session_runner": function(module, exports, __bundleRequire) {
"use strict";
module.exports.runSession = runSession;
var runtime = __bundleRequire("./lib/runtime");
var taskRunner = __bundleRequire("./lib/task_runner");
var NODE_TRANSITION_DELAY_MS = 2000;
function sleepMilliseconds(durationMS) {
    var startedAt = new Date().getTime();
    while (new Date().getTime() - startedAt < durationMS) {
        java.lang.Thread.sleep(100);
    }
}
function buildSessionRefs(sessionPayload) {
    return {
        plan_run_id: String((sessionPayload && sessionPayload.plan_run_id) || ""),
        plan_device_run_id: String((sessionPayload && sessionPayload.plan_device_run_id) || "")
    };
}
function pauseBeforeNextNode(sessionPayload, fromNodeID, toNodeID, options) {
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop() { };
    var isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled() { return false; };
    var refs = buildSessionRefs(sessionPayload);
    if (!toNodeID || isCancelled()) {
        return;
    }
    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(fromNodeID || ""),
        event_type: "workflow_session_progress",
        status: "running",
        step_name: "NODE_TRANSITION_WAIT",
        message: "节点切换前等待 2 秒",
        extra: {
            from_node_id: String(fromNodeID || ""),
            to_node_id: String(toNodeID || ""),
            delay_ms: NODE_TRANSITION_DELAY_MS
        }
    });
    sleepMilliseconds(NODE_TRANSITION_DELAY_MS);
}
function syncSessionScripts(sessionPayload, options) {
    var manifests = sessionPayload && sessionPayload.script_manifest ? sessionPayload.script_manifest : [];
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop() { };
    var centerBaseURL = String((options && options.centerBaseURL) || "");
    var isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled() { return false; };
    var refs = buildSessionRefs(sessionPayload);
    if (!centerBaseURL || manifests.length === 0) {
        return;
    }
    for (var index = 0; index < manifests.length; index += 1) {
        if (isCancelled()) {
            return;
        }
        var item = manifests[index] || {};
        sendEvent({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: "",
            event_type: "workflow_session_progress",
            status: "running",
            step_name: "SYNC_SCRIPT",
            message: "工作流执行前校验脚本版本",
            extra: {
                script_name: String(item.script_name || ""),
                script_version: String(item.script_version || "")
            }
        });
        taskRunner.ensureScriptVersion(String(item.script_name || ""), String(item.script_version || ""), {
            centerBaseURL: centerBaseURL,
            logger: logger,
            force: false
        });
    }
}
function buildNodeMap(snapshot) {
    var map = {};
    var nodes = snapshot && snapshot.nodes ? snapshot.nodes : [];
    for (var index = 0; index < nodes.length; index += 1) {
        map[String(nodes[index].node_id || "")] = nodes[index];
    }
    return map;
}
function buildEdgeMap(snapshot) {
    var map = {};
    var edges = snapshot && snapshot.edges ? snapshot.edges : [];
    for (var index = 0; index < edges.length; index += 1) {
        var key = String(edges[index].from_node_id || "") + "::" + String(edges[index].edge_type || "next");
        map[key] = String(edges[index].to_node_id || "");
    }
    return map;
}
function getNextNodeID(edgeMap, fromNodeID, edgeType) {
    return String(edgeMap[String(fromNodeID || "") + "::" + String(edgeType || "next")] || "");
}
function createTaskSummaryFromNode(sessionPayload, node) {
    var refs = buildSessionRefs(sessionPayload);
    return {
        task_id: "workflow-session-" + String(refs.plan_device_run_id || "") + "-" + String(node.node_id || ""),
        script_name: String(node.script_name || ""),
        script_version: String(node.script_version || ""),
        priority: 0,
        params: {
            workflow_session_id: String(sessionPayload.workflow_session_id || ""),
            workflow_def_id: String(sessionPayload.workflow_def_id || ""),
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(node.node_id || "")
        }
    };
}
function runScriptNode(sessionPayload, node, options) {
    var config = options || {};
    var logger = config.logger || runtime.createLogger();
    var sendEvent = typeof config.sendEvent === "function" ? config.sendEvent : function noop() { };
    var isCancelled = typeof config.isCancelled === "function" ? config.isCancelled : function neverCancelled() { return false; };
    var taskSummary = createTaskSummaryFromNode(sessionPayload, node);
    var refs = buildSessionRefs(sessionPayload);
    if (isCancelled()) {
        return {
            status: "stopped",
            result_code: "WORKFLOW_SESSION_STOPPED",
            result_message: "工作流会话已停止",
            workflow_node_id: String(node.node_id || "")
        };
    }
    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(node.node_id || ""),
        event_type: "workflow_step_started",
        status: "running",
        step_name: "START_NODE",
        message: "工作流步骤开始执行",
        extra: {
            node_name: String(node.node_name || ""),
            script_name: String(node.script_name || ""),
            script_version: String(node.script_version || "")
        }
    });
    var result = taskRunner.runTask(taskSummary, {
        deviceID: String(config.deviceID || ""),
        agentUUID: String(config.agentUUID || ""),
        centerBaseURL: String(config.centerBaseURL || ""),
        logger: logger,
        isCancelled: isCancelled,
        onProgress: function (progress) {
            sendEvent({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: String(node.node_id || ""),
                event_type: "workflow_session_progress",
                status: String((progress && progress.status) || "running"),
                step_name: String((progress && progress.step_name) || ""),
                message: String((progress && progress.message) || ""),
                extra: (progress && progress.extra) || {}
            });
        }
    });
    if (isCancelled()) {
        var actualStatus = String((result && result.status) || "");
        var actualResultCode = String((result && result.result_code) || "");
        var actualResultMessage = String((result && result.result_message) || "");
        var actualStepName = String((result && result.step_name) || "");
        var stoppedMessage = actualStatus === "success"
            ? "工作流会话已停止，脚本在停止后实际执行成功"
            : "工作流会话已停止，脚本在停止后实际执行失败";
        sendEvent({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(node.node_id || ""),
            event_type: "workflow_step_stopped",
            status: "stopped",
            step_name: actualStepName || "STOPPED",
            message: stoppedMessage,
            extra: {
                actual_status: actualStatus,
                actual_result_code: actualResultCode,
                actual_result_message: actualResultMessage
            }
        });
        return {
            status: "stopped",
            result_code: "WORKFLOW_SESSION_STOPPED",
            result_message: stoppedMessage,
            workflow_node_id: String(node.node_id || ""),
            extra: {
                actual_status: actualStatus,
                actual_result_code: actualResultCode,
                actual_result_message: actualResultMessage
            }
        };
    }
    if (result && String(result.status || "") === "success") {
        sendEvent({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(node.node_id || ""),
            event_type: "workflow_step_succeeded",
            status: "success",
            step_name: String(result.step_name || "COMPLETE"),
            message: String(result.result_message || "工作流步骤执行成功"),
            extra: {
                result_code: String(result.result_code || "")
            }
        });
        return result;
    }
    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: String(node.node_id || ""),
        event_type: "workflow_step_failed",
        status: "failed",
        step_name: String((result && result.step_name) || "RUN_NODE"),
        message: String((result && result.result_message) || "工作流步骤执行失败"),
        extra: {
            result_code: String((result && result.result_code) || "")
        }
    });
    return result;
}
function runSession(sessionPayload, options) {
    var snapshot = sessionPayload && sessionPayload.definition_snapshot ? sessionPayload.definition_snapshot : {};
    var nodeMap = buildNodeMap(snapshot);
    var edgeMap = buildEdgeMap(snapshot);
    var currentNodeID = String(sessionPayload.entry_node_id || "");
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var sendEvent = options && typeof options.sendEvent === "function" ? options.sendEvent : function noop() { };
    var isCancelled = options && typeof options.isCancelled === "function" ? options.isCancelled : function neverCancelled() { return false; };
    var refs = buildSessionRefs(sessionPayload);
    var loopCounters = {};
    sendEvent({
        plan_run_id: refs.plan_run_id,
        plan_device_run_id: refs.plan_device_run_id,
        workflow_node_id: currentNodeID,
        event_type: "workflow_run_started",
        status: "running",
        step_name: "START_SESSION",
        message: "工作流运行已启动",
        extra: {
            workflow_name: String(sessionPayload.workflow_name || "")
        }
    });
    syncSessionScripts(sessionPayload, {
        centerBaseURL: options && options.centerBaseURL,
        logger: logger,
        sendEvent: sendEvent,
        isCancelled: isCancelled
    });
    if (isCancelled()) {
        return {
            status: "stopped",
            result_code: "WORKFLOW_SESSION_STOPPED",
            result_message: "工作流会话已停止",
            workflow_node_id: currentNodeID
        };
    }
    while (currentNodeID) {
        if (isCancelled()) {
            return {
                status: "stopped",
                result_code: "WORKFLOW_SESSION_STOPPED",
                result_message: "工作流会话已停止",
                workflow_node_id: currentNodeID
            };
        }
        var node = nodeMap[currentNodeID];
        if (!node) {
            return {
                status: "failed",
                result_code: "WORKFLOW_NODE_NOT_FOUND",
                result_message: "未找到工作流节点: " + currentNodeID,
                workflow_node_id: currentNodeID
            };
        }
        if (String(node.node_type || "") === "script") {
            var result = runScriptNode(sessionPayload, node, {
                deviceID: options && options.deviceID,
                agentUUID: options && options.agentUUID,
                centerBaseURL: options && options.centerBaseURL,
                logger: logger,
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            if (!result || String(result.status || "") !== "success") {
                return {
                    status: String((result && result.status) || "failed"),
                    result_code: String((result && result.result_code) || "WORKFLOW_STEP_FAILED"),
                    result_message: String((result && result.result_message) || "工作流步骤执行失败"),
                    workflow_node_id: currentNodeID
                };
            }
            var nextNodeID = getNextNodeID(edgeMap, currentNodeID, "next");
            pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            currentNodeID = nextNodeID;
            continue;
        }
        if (String(node.node_type || "") === "loop") {
            loopCounters[currentNodeID] = Number(loopCounters[currentNodeID] || 0);
            if (Number(node.max_iterations || 0) > 0 && loopCounters[currentNodeID] >= Number(node.max_iterations || 0)) {
                var nextNodeID_1 = getNextNodeID(edgeMap, currentNodeID, "loop_exit") || getNextNodeID(edgeMap, currentNodeID, "next");
                pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID_1, {
                    sendEvent: sendEvent,
                    isCancelled: isCancelled
                });
                currentNodeID = nextNodeID_1;
                continue;
            }
            loopCounters[currentNodeID] += 1;
            sendEvent({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: currentNodeID,
                event_type: "workflow_loop_completed",
                status: "running",
                step_name: "LOOP",
                message: "工作流循环节点已完成一轮",
                extra: {
                    counter: loopCounters[currentNodeID],
                    max_iterations: Number(node.max_iterations || 0)
                }
            });
            var nextNodeID = getNextNodeID(edgeMap, currentNodeID, "loop_body") || getNextNodeID(edgeMap, currentNodeID, "next");
            pauseBeforeNextNode(sessionPayload, currentNodeID, nextNodeID, {
                sendEvent: sendEvent,
                isCancelled: isCancelled
            });
            currentNodeID = nextNodeID;
            continue;
        }
        if (String(node.node_type || "") === "stop") {
            return {
                status: "success",
                result_code: "OK",
                result_message: "工作流运行已完成",
                workflow_node_id: currentNodeID
            };
        }
        logger.error("不支持的工作流节点类型: " + String(node.node_type || ""));
        return {
            status: "failed",
            result_code: "WORKFLOW_NODE_TYPE_UNSUPPORTED",
            result_message: "不支持的工作流节点类型: " + String(node.node_type || ""),
            workflow_node_id: currentNodeID
        };
    }
    return {
        status: "success",
        result_code: "OK",
        result_message: "工作流运行已完成",
        workflow_node_id: ""
    };
}

  },
  "./lib/ws_client": function(module, exports, __bundleRequire) {
"use strict";
module.exports.buildWebSocketURL = buildWebSocketURL;
module.exports.createEnvelope = createEnvelope;
module.exports.connect = connect;
module.exports.createSessionFlagStore = createSessionFlagStore;
module.exports.buildTaskSummary = buildTaskSummary;
module.exports.buildWorkflowSessionRefs = buildWorkflowSessionRefs;
var runtime = __bundleRequire("./lib/runtime");
var taskRunner = __bundleRequire("./lib/task_runner");
var workflowSessionRunner = __bundleRequire("./lib/workflow_session_runner");
var heartbeatScheduler = __bundleRequire("./lib/heartbeat_scheduler");
function trimBaseURL(baseURL) {
    return String(baseURL || "").replace(/\/+$/, "");
}
function buildWebSocketURL(centerBaseURL) {
    var base = trimBaseURL(centerBaseURL);
    if (base.indexOf("https://") === 0) {
        return "wss://" + base.slice("https://".length) + "/ws";
    }
    if (base.indexOf("http://") === 0) {
        return "ws://" + base.slice("http://".length) + "/ws";
    }
    if (base.indexOf("ws://") === 0 || base.indexOf("wss://") === 0) {
        return base.replace(/\/+$/, "") + "/ws";
    }
    return "ws://" + base + "/ws";
}
function createRequestID(prefix) {
    return String(prefix || "request") + "-" + Date.now() + "-" + Math.floor(Math.random() * 100000);
}
function createEnvelope(type, requestID, deviceID, payload) {
    return {
        type: type,
        request_id: requestID,
        device_id: deviceID,
        timestamp: Math.floor(Date.now() / 1000),
        payload: payload || {}
    };
}
function isOkAck(message, messageType) {
    return !!(message &&
        message.type === "ack" &&
        message.payload &&
        message.payload.message_type === messageType &&
        message.payload.status === "ok");
}
function javaType(name) {
    if (typeof Java !== "undefined" && typeof Java.type === "function") {
        return Java.type(name);
    }
    var parts = String(name || "").split(".");
    var current = Packages;
    for (var index = 0; index < parts.length; index += 1) {
        current = current[parts[index]];
    }
    return current;
}
function createWebSocketListener(callbacks) {
    var WebSocketListener = javaType("okhttp3.WebSocketListener");
    if (typeof JavaAdapter === "function") {
        return new JavaAdapter(WebSocketListener, callbacks);
    }
    return new WebSocketListener(callbacks);
}
function createRunnable(runCallback) {
    var Runnable = javaType("java.lang.Runnable");
    if (typeof JavaAdapter === "function") {
        return new JavaAdapter(Runnable, {
            run: runCallback
        });
    }
    return new Runnable({
        run: runCallback
    });
}
function createSessionFlagStore() {
    if (typeof java !== "undefined") {
        try {
            var ConcurrentHashMap = javaType("java.util.concurrent.ConcurrentHashMap");
            var map_1 = new ConcurrentHashMap();
            return {
                put: function (key) {
                    map_1.put(String(key || ""), true);
                },
                remove: function (key) {
                    map_1.remove(String(key || ""));
                },
                has: function (key) {
                    return map_1.containsKey(String(key || ""));
                }
            };
        }
        catch (_error) {
            // 回退到普通对象存储。
        }
    }
    var state = {};
    return {
        put: function (key) {
            state[String(key || "")] = true;
        },
        remove: function (key) {
            delete state[String(key || "")];
        },
        has: function (key) {
            return state[String(key || "")] === true;
        }
    };
}
function buildTaskSummary(taskPayload) {
    var payload = taskPayload || {};
    return {
        task_id: String(payload.task_id || ""),
        script_name: String(payload.script_name || ""),
        script_version: String(payload.script_version || ""),
        priority: Number(payload.priority || 0),
        params: (payload.params || {})
    };
}
function buildWorkflowSessionRefs(payload) {
    var sessionPayload = payload || {};
    return {
        plan_run_id: String(sessionPayload.plan_run_id || ""),
        plan_device_run_id: String(sessionPayload.plan_device_run_id || "")
    };
}
function connectAutoJs(options) {
    var logger = options.logger || runtime.createLogger();
    var OkHttpClient = javaType("okhttp3.OkHttpClient");
    var Request = javaType("okhttp3.Request");
    var TimeUnit = javaType("java.util.concurrent.TimeUnit");
    var Executors = javaType("java.util.concurrent.Executors");
    var heartbeatHandle = null;
    var reconnectExecutor = null;
    var reconnectFuture = null;
    var wsURL = buildWebSocketURL(options.centerBaseURL);
    var heartbeatIntervalMS = Number(options.heartbeatIntervalMS || 30000);
    var heartbeatSchedulerMode = String(options.heartbeatScheduler || "executor");
    var heartbeatLeasePath = String(options.heartbeatLeasePath || "");
    var pingIntervalMS = Number(options.pingIntervalMS || Math.min(heartbeatIntervalMS, 20000));
    var watchdogIntervalMS = Number(options.watchdogIntervalMS || 20000);
    var silenceTimeoutMS = Number(options.silenceTimeoutMS || 120000);
    var reconnectEnabled = options.reconnectEnabled !== false;
    var reconnectInitialDelayMS = Number(options.reconnectInitialDelayMS || 3000);
    var reconnectMaxDelayMS = Number(options.reconnectMaxDelayMS || 60000);
    var reconnectBackoffMultiplier = Number(options.reconnectBackoffMultiplier || 2);
    var deviceID = String(options.deviceID || "");
    var agentUUID = String(options.agentUUID || "");
    var deviceLinkSN = String(options.deviceLinkSN || "");
    var onAssignTask = typeof options.onAssignTask === "function" ? options.onAssignTask : null;
    var heartbeatStarted = false;
    var heartbeatGeneration = 0;
    var heartbeatLeaseToken = "";
    var reconnectAttempt = 0;
    var reconnectScheduledGeneration = 0;
    var connectGeneration = 0;
    var closedGeneration = 0;
    var intentionallyClosed = false;
    var lastReceiveAt = Date.now();
    var lastSendAt = Date.now();
    var taskExecuting = false;
    var workflowSessionExecuting = false;
    var workflowStopFlags = createSessionFlagStore();
    var workflowResultSentFlags = createSessionFlagStore();
    var currentWorkflowRunID = "";
    var lastTaskProgressState = null;
    var lastWorkflowEventState = null;
    var watchdogExecutor = null;
    var watchdogFuture = null;
    logger.info("WebSocket 心跳调度模式：" + heartbeatSchedulerMode);
    var client = new OkHttpClient.Builder()
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .pingInterval(Math.max(5000, pingIntervalMS), TimeUnit.MILLISECONDS)
        .build();
    var socket = null;
    var activeHeartbeatScheduler = heartbeatScheduler.createHeartbeatScheduler(heartbeatSchedulerMode, logger);
    function shouldSkipDuplicateEvent(cache, key, dedupWindowMS) {
        if (!cache) {
            return false;
        }
        return cache.key === key && (Date.now() - cache.at) <= dedupWindowMS;
    }
    function send(type, requestID, payload) {
        if (!socket) {
            throw new Error("websocket_not_connected");
        }
        var message = createEnvelope(type, requestID, deviceID, payload);
        var text = JSON.stringify(message);
        logger.info("发送 WebSocket 消息：" + type + "，request_id=" + requestID);
        if (socket.send(text) === false) {
            throw new Error("websocket_send_failed");
        }
        lastSendAt = Date.now();
    }
    function readHeartbeatLeaseToken() {
        if (!heartbeatLeasePath) {
            return "";
        }
        try {
            var text = runtime.readTextFile(heartbeatLeasePath);
            if (!text || !String(text).trim()) {
                return "";
            }
            var payload = JSON.parse(String(text));
            return String(payload.token || "");
        }
        catch (_error) {
            return "";
        }
    }
    function writeHeartbeatLeaseToken(token) {
        if (!heartbeatLeasePath) {
            return;
        }
        runtime.writeTextFile(heartbeatLeasePath, JSON.stringify({
            token: String(token || ""),
            updated_at: runtime.nowISOString()
        }, null, 2));
    }
    function clearHeartbeatLeaseToken(token) {
        if (!heartbeatLeasePath) {
            return;
        }
        var currentToken = readHeartbeatLeaseToken();
        if (!currentToken || currentToken === String(token || "")) {
            runtime.removeFileIfExists(heartbeatLeasePath);
        }
    }
    function isHeartbeatLeaseActive(token) {
        if (!heartbeatLeasePath) {
            return true;
        }
        return readHeartbeatLeaseToken() === String(token || "");
    }
    function sendHeartbeat(source) {
        logger.info("准备发送心跳，source="
            + String(source || "")
            + "，connect_generation="
            + connectGeneration
            + "，heartbeat_generation="
            + heartbeatGeneration);
        send("heartbeat", createRequestID("agent-heartbeat"), {
            agent_uuid: agentUUID,
            device_link_sn: deviceLinkSN,
            execution_profile: runtime.collectExecutionProfile()
        });
    }
    function sendTaskAck(taskPayload) {
        var summary = buildTaskSummary(taskPayload);
        send("task_ack", createRequestID("agent-task-ack"), {
            task_id: summary.task_id,
            status: "ok",
            message: "Agent 已收到任务，准备执行",
            script_name: summary.script_name,
            script_version: summary.script_version
        });
        logger.info("已发送 task_ack：" + summary.task_id);
    }
    function sendTaskResult(taskSummary, result) {
        var summary = taskSummary || {};
        var payload = result || {};
        send("task_result", createRequestID("agent-task-result"), {
            task_id: String(summary.task_id || ""),
            status: String(payload.status || "failed"),
            result_code: String(payload.result_code || ""),
            result_message: String(payload.result_message || ""),
            step_name: String(payload.step_name || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 task_result：" + String(summary.task_id || "") + " -> " + String(payload.status || "failed"));
    }
    function sendTaskProgress(taskSummary, progress) {
        var summary = taskSummary || {};
        var payload = progress || {};
        var eventKey = JSON.stringify({
            task_id: String(summary.task_id || ""),
            status: String(payload.status || "running"),
            step_name: String(payload.step_name || ""),
            message: String(payload.message || ""),
            extra: payload.extra || {}
        });
        if (shouldSkipDuplicateEvent(lastTaskProgressState, eventKey, 1500)) {
            logger.info("跳过重复 task_progress：" + String(summary.task_id || "") + " -> " + String(payload.step_name || ""));
            return;
        }
        send("task_progress", createRequestID("agent-task-progress"), {
            task_id: String(summary.task_id || ""),
            status: String(payload.status || "running"),
            step_name: String(payload.step_name || ""),
            message: String(payload.message || ""),
            extra: payload.extra || {}
        });
        lastTaskProgressState = {
            key: eventKey,
            at: Date.now()
        };
        logger.info("已发送 task_progress：" + String(summary.task_id || "") + " -> " + String(payload.step_name || ""));
    }
    function sendUnifiedProgress(taskSummary, progress) {
        sendTaskProgress(taskSummary, progress);
    }
    function sendScriptSyncAck(syncPayload) {
        var payload = syncPayload || {};
        send("script_sync_ack", createRequestID("agent-script-sync-ack"), {
            script_name: String(payload.script_name || ""),
            script_version: String(payload.script_version || ""),
            status: "ok",
            message: "Agent 已收到脚本同步指令"
        });
        logger.info("已发送 script_sync_ack：" + String(payload.script_name || "") + "@" + String(payload.script_version || ""));
    }
    function sendScriptSyncResult(syncPayload, result) {
        var payload = syncPayload || {};
        var summary = result || {};
        send("script_sync_result", createRequestID("agent-script-sync-result"), {
            script_name: String(payload.script_name || ""),
            script_version: String(payload.script_version || ""),
            status: String(summary.status || "failed"),
            result_code: String(summary.result_code || ""),
            result_message: String(summary.result_message || ""),
            extra: summary.extra || {}
        });
        logger.info("已发送 script_sync_result：" + String(payload.script_name || "") + "@" + String(payload.script_version || "") + " -> " + String(summary.status || "failed"));
    }
    function sendWorkflowSessionAck(sessionPayload) {
        var payload = sessionPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
        send("workflow_session_ack", createRequestID("agent-workflow-session-ack"), {
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            status: "ok",
            message: "Agent 已收到工作流会话"
        });
        logger.info("已发送 workflow_session_ack：" + String(refs.plan_device_run_id || ""));
    }
    function sendWorkflowSessionEvent(eventPayload) {
        var payload = eventPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
        var eventKey = JSON.stringify({
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(payload.workflow_node_id || ""),
            event_type: String(payload.event_type || ""),
            status: String(payload.status || "running"),
            step_name: String(payload.step_name || ""),
            message: String(payload.message || ""),
            extra: payload.extra || {}
        });
        if (shouldSkipDuplicateEvent(lastWorkflowEventState, eventKey, 1500)) {
            logger.info("跳过重复 workflow_session_event：" + String(refs.plan_device_run_id || "") + " -> " + String(payload.step_name || ""));
            return;
        }
        send("workflow_session_event", createRequestID("agent-workflow-session-event"), {
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(payload.workflow_node_id || ""),
            event_type: String(payload.event_type || ""),
            status: String(payload.status || "running"),
            step_name: String(payload.step_name || ""),
            message: String(payload.message || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 workflow_session_event："
            + String(refs.plan_device_run_id || "")
            + " -> "
            + String(payload.event_type || "")
            + " / "
            + String(payload.status || "running")
            + " / "
            + String(payload.message || ""));
        lastWorkflowEventState = {
            key: eventKey,
            at: Date.now()
        };
    }
    function sendWorkflowSessionResult(resultPayload) {
        var payload = resultPayload || {};
        var refs = buildWorkflowSessionRefs(payload);
        var sessionKey = String(refs.plan_device_run_id || "");
        if (sessionKey && workflowResultSentFlags.has(sessionKey)) {
            logger.warn("工作流结果已发送，忽略重复 workflow_session_result：" + sessionKey + " -> " + String(payload.status || "failed"));
            return;
        }
        send("workflow_session_result", createRequestID("agent-workflow-session-result"), {
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_node_id: String(payload.workflow_node_id || ""),
            status: String(payload.status || "failed"),
            result_code: String(payload.result_code || ""),
            result_message: String(payload.result_message || ""),
            extra: payload.extra || {}
        });
        logger.info("已发送 workflow_session_result："
            + String(refs.plan_device_run_id || "")
            + " -> "
            + String(payload.status || "failed")
            + " / "
            + String(payload.result_message || "")
            + " / extra="
            + JSON.stringify(payload.extra || {}));
        if (sessionKey) {
            workflowResultSentFlags.put(sessionKey);
        }
    }
    function markWorkflowStopRequested(sessionKey) {
        var nextKey = String(sessionKey || "").trim();
        if (!nextKey) {
            return;
        }
        workflowStopFlags.put(nextKey);
    }
    function clearWorkflowStopRequested(sessionKey) {
        var nextKey = String(sessionKey || "").trim();
        if (!nextKey) {
            return;
        }
        workflowStopFlags.remove(nextKey);
    }
    function clearWorkflowResultSent(sessionKey) {
        var nextKey = String(sessionKey || "").trim();
        if (!nextKey) {
            return;
        }
        workflowResultSentFlags.remove(nextKey);
    }
    function isWorkflowStopRequested(sessionKey) {
        var nextKey = String(sessionKey || "").trim();
        if (!nextKey) {
            return false;
        }
        return workflowStopFlags.has(nextKey);
    }
    function stopHeartbeat() {
        heartbeatGeneration += 1;
        if (heartbeatHandle) {
            logger.info("停止心跳调度器，mode="
                + heartbeatHandle.kind
                + "，scheduler_id="
                + heartbeatHandle.schedulerID
                + "，next_generation="
                + heartbeatGeneration);
            heartbeatHandle.cancel();
            heartbeatHandle = null;
        }
        clearHeartbeatLeaseToken(heartbeatLeaseToken);
        heartbeatLeaseToken = "";
        heartbeatStarted = false;
    }
    function stopReconnect() {
        if (reconnectFuture) {
            reconnectFuture.cancel(false);
            reconnectFuture = null;
        }
        if (reconnectExecutor) {
            reconnectExecutor.shutdownNow();
            reconnectExecutor = null;
        }
        reconnectScheduledGeneration = 0;
    }
    function stopWatchdog() {
        if (watchdogFuture) {
            watchdogFuture.cancel(false);
            watchdogFuture = null;
        }
        if (watchdogExecutor) {
            watchdogExecutor.shutdownNow();
            watchdogExecutor = null;
        }
    }
    function resetReconnectState() {
        reconnectAttempt = 0;
        stopReconnect();
    }
    function sanitizeReconnectDelay(value, fallback) {
        var nextValue = Number(value || fallback);
        if (!nextValue || nextValue < 1000) {
            return fallback;
        }
        return nextValue;
    }
    reconnectInitialDelayMS = sanitizeReconnectDelay(reconnectInitialDelayMS, 3000);
    reconnectMaxDelayMS = sanitizeReconnectDelay(reconnectMaxDelayMS, 60000);
    if (reconnectMaxDelayMS < reconnectInitialDelayMS) {
        reconnectMaxDelayMS = reconnectInitialDelayMS;
    }
    if (!reconnectBackoffMultiplier || reconnectBackoffMultiplier < 1) {
        reconnectBackoffMultiplier = 2;
    }
    function getReconnectDelayMS(attempt) {
        var delay = reconnectInitialDelayMS;
        var currentAttempt = Number(attempt || 1);
        var index = 1;
        while (index < currentAttempt) {
            delay = Math.min(reconnectMaxDelayMS, delay * reconnectBackoffMultiplier);
            index += 1;
        }
        return Math.min(delay, reconnectMaxDelayMS);
    }
    function closeSocketQuietly(closeCode, closeReason) {
        if (!socket) {
            return;
        }
        try {
            socket.close(closeCode || 1000, closeReason || "close");
        }
        catch (error) {
            logger.warn("WebSocket 关闭时出现异常：" + String(error));
        }
    }
    function startWatchdog() {
        var safeWatchdogIntervalMS = Math.max(5000, watchdogIntervalMS);
        var safeSilenceTimeoutMS = Math.max(safeWatchdogIntervalMS * 2, silenceTimeoutMS);
        stopWatchdog();
        watchdogExecutor = Executors.newSingleThreadScheduledExecutor();
        watchdogFuture = watchdogExecutor.scheduleAtFixedRate(createRunnable(function watchdogTask() {
            if (intentionallyClosed) {
                return;
            }
            var now = Date.now();
            var silenceMS = now - Math.max(lastReceiveAt, lastSendAt);
            if (!socket) {
                logger.warn("WebSocket watchdog 检测到连接对象缺失，准备触发重连。");
                scheduleReconnect("watchdog_missing_socket");
                return;
            }
            if (silenceMS >= safeSilenceTimeoutMS) {
                logger.warn("WebSocket watchdog 检测到连接静默超时，准备主动重连，silence_ms=" + silenceMS);
                closeSocketQuietly(1001, "watchdog silence timeout");
                handleSocketClosed(connectGeneration, "watchdog_silence_timeout " + silenceMS);
            }
        }), safeWatchdogIntervalMS, safeWatchdogIntervalMS, TimeUnit.MILLISECONDS);
        logger.info("已启动 WebSocket watchdog，interval_ms="
            + safeWatchdogIntervalMS
            + "，silence_timeout_ms="
            + safeSilenceTimeoutMS
            + "，ping_interval_ms="
            + Math.max(5000, pingIntervalMS));
    }
    function scheduleReconnect(reason) {
        if (!reconnectEnabled) {
            logger.warn("WebSocket 自动重连已禁用，原因：" + String(reason || ""));
            return;
        }
        if (intentionallyClosed) {
            logger.info("WebSocket 为主动关闭，不进入自动重连。");
            return;
        }
        if (reconnectFuture) {
            return;
        }
        if (reconnectScheduledGeneration === connectGeneration) {
            return;
        }
        reconnectScheduledGeneration = connectGeneration;
        reconnectAttempt += 1;
        var delayMS = getReconnectDelayMS(reconnectAttempt);
        var scheduledGeneration = connectGeneration;
        logger.warn("WebSocket 连接中断，准备第 " + reconnectAttempt + " 次重连，delay=" + delayMS + "ms，原因：" + String(reason || ""));
        reconnectExecutor = Executors.newSingleThreadScheduledExecutor();
        reconnectFuture = reconnectExecutor.schedule(createRunnable(function reconnectTask() {
            reconnectFuture = null;
            if (reconnectExecutor) {
                reconnectExecutor.shutdownNow();
                reconnectExecutor = null;
            }
            if (intentionallyClosed) {
                logger.info("WebSocket 在重连等待期间被主动关闭，取消重连。");
                return;
            }
            if (scheduledGeneration !== connectGeneration) {
                logger.info("WebSocket 重连任务代际已过期，取消执行：scheduled_generation=" + scheduledGeneration + "，current_generation=" + connectGeneration);
                return;
            }
            logger.info("WebSocket 开始执行第 " + reconnectAttempt + " 次重连：" + wsURL);
            openSocket();
        }), delayMS, TimeUnit.MILLISECONDS);
    }
    function handleSocketClosed(expectedGeneration, reason) {
        if (expectedGeneration !== connectGeneration) {
            return;
        }
        if (closedGeneration === expectedGeneration) {
            return;
        }
        closedGeneration = expectedGeneration;
        stopHeartbeat();
        socket = null;
        scheduleReconnect(reason);
    }
    function startHeartbeat(expectedGeneration) {
        if (heartbeatStarted) {
            return;
        }
        heartbeatStarted = true;
        heartbeatGeneration += 1;
        var currentHeartbeatGeneration = heartbeatGeneration;
        heartbeatLeaseToken = String(connectGeneration) + ":" + String(currentHeartbeatGeneration) + ":" + String(Date.now());
        writeHeartbeatLeaseToken(heartbeatLeaseToken);
        sendHeartbeat("hello_ack");
        heartbeatHandle = activeHeartbeatScheduler.start({
            intervalMS: heartbeatIntervalMS,
            logger: logger,
            onTick: function heartbeatTask() {
                logger.info("心跳调度器触发，mode="
                    + (heartbeatHandle ? heartbeatHandle.kind : activeHeartbeatScheduler.kind)
                    + "，scheduler_id="
                    + (heartbeatHandle ? heartbeatHandle.schedulerID : "")
                    + "，connect_generation="
                    + connectGeneration
                    + "，expected_generation="
                    + expectedGeneration
                    + "，heartbeat_generation="
                    + currentHeartbeatGeneration);
                if (!heartbeatStarted || currentHeartbeatGeneration !== heartbeatGeneration || expectedGeneration !== connectGeneration) {
                    return;
                }
                if (!isHeartbeatLeaseActive(heartbeatLeaseToken)) {
                    logger.warn("检测到心跳租约已失效，停止旧心跳链路，scheduler_id="
                        + (heartbeatHandle ? heartbeatHandle.schedulerID : "")
                        + "，connect_generation="
                        + connectGeneration
                        + "，heartbeat_generation="
                        + currentHeartbeatGeneration);
                    stopHeartbeat();
                    return;
                }
                try {
                    sendHeartbeat("scheduler_tick");
                }
                catch (error) {
                    logger.error("WebSocket heartbeat 发送失败：" + String(error));
                    closeSocketQuietly(1001, "heartbeat send failed");
                    handleSocketClosed(expectedGeneration, "heartbeat_send_failed " + String(error));
                }
            }
        });
        logger.info("已启动心跳调度器，requested_mode="
            + heartbeatSchedulerMode
            + "，actual_mode="
            + heartbeatHandle.kind
            + "，scheduler_id="
            + heartbeatHandle.schedulerID
            + "，lease_token="
            + heartbeatLeaseToken
            + "，connect_generation="
            + connectGeneration
            + "，heartbeat_generation="
            + currentHeartbeatGeneration
            + "，interval_ms="
            + heartbeatIntervalMS);
    }
    function executeTask(taskSummary) {
        if (taskExecuting) {
            logger.warn("当前已有任务在执行，忽略新的 assign_task：" + String(taskSummary.task_id || ""));
            return;
        }
        taskExecuting = true;
        try {
            sendUnifiedProgress(taskSummary, {
                status: "running",
                step_name: "INIT_TASK",
                message: "任务执行中：初始化任务执行上下文"
            });
            var result = taskRunner.runTask(taskSummary, {
                deviceID: deviceID,
                agentUUID: agentUUID,
                centerBaseURL: options.centerBaseURL,
                logger: logger,
                onProgress: function (progress) {
                    sendUnifiedProgress(taskSummary, progress);
                }
            });
            sendTaskResult(taskSummary, result);
        }
        catch (error) {
            logger.error("本地任务执行失败：" + String(error));
            sendUnifiedProgress(taskSummary, {
                status: "failed",
                step_name: "RUN_TASK",
                message: "任务执行中：执行器抛出异常",
                extra: {
                    error: String(error)
                }
            });
            sendTaskResult(taskSummary, {
                status: "failed",
                result_code: "RUNNER_EXCEPTION",
                result_message: String(error),
                step_name: "RUN_TASK",
                extra: {
                    mode: "builtin"
                }
            });
        }
        finally {
            taskExecuting = false;
        }
    }
    function executeScriptSync(syncPayload) {
        var payload = syncPayload || {};
        try {
            var result = taskRunner.syncScriptVersion(String(payload.script_name || ""), String(payload.script_version || ""), {
                centerBaseURL: options.centerBaseURL,
                force: payload.force === true,
                logger: logger
            });
            sendScriptSyncResult(payload, {
                status: "success",
                result_code: "OK",
                result_message: "脚本同步成功",
                extra: result
            });
        }
        catch (error) {
            logger.error("脚本同步失败：" + String(error));
            sendScriptSyncResult(payload, {
                status: "failed",
                result_code: "SCRIPT_SYNC_FAILED",
                result_message: String(error),
                extra: {}
            });
        }
    }
    function executeWorkflowSession(sessionPayload) {
        var payload = (sessionPayload || {});
        var refs = buildWorkflowSessionRefs(payload);
        var sessionKey = String(refs.plan_device_run_id || "");
        if (workflowSessionExecuting) {
            logger.warn("当前已有工作流会话在执行，忽略新的 start_workflow_session: " + String(sessionKey || ""));
            return;
        }
        workflowSessionExecuting = true;
        currentWorkflowRunID = sessionKey;
        clearWorkflowStopRequested(sessionKey);
        clearWorkflowResultSent(sessionKey);
        try {
            var result = workflowSessionRunner.runSession(payload, {
                deviceID: deviceID,
                agentUUID: agentUUID,
                centerBaseURL: options.centerBaseURL,
                logger: logger,
                isCancelled: function () {
                    return isWorkflowStopRequested(sessionKey);
                },
                sendEvent: function (eventPayload) {
                    sendWorkflowSessionEvent(eventPayload);
                }
            });
            sendWorkflowSessionResult({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: String((result && result.workflow_node_id) || ""),
                status: String((result && result.status) || "failed"),
                result_code: String((result && result.result_code) || ""),
                result_message: String((result && result.result_message) || ""),
                extra: (result && result.extra) || {}
            });
        }
        catch (error) {
            logger.error("本地工作流会话执行失败：" + String(error));
            sendWorkflowSessionResult({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: "",
                status: "failed",
                result_code: "WORKFLOW_SESSION_EXCEPTION",
                result_message: String(error),
                extra: {}
            });
        }
        finally {
            clearWorkflowStopRequested(sessionKey);
            currentWorkflowRunID = "";
            workflowSessionExecuting = false;
        }
    }
    function scheduleTaskExecution(taskSummary) {
        runtime.runAsync(function runTaskAsync() {
            executeTask(taskSummary);
        });
    }
    function scheduleScriptSync(syncPayload) {
        runtime.runAsync(function runScriptSyncAsync() {
            executeScriptSync(syncPayload);
        });
    }
    function scheduleWorkflowSession(sessionPayload) {
        runtime.runAsync(function runWorkflowAsync() {
            executeWorkflowSession(sessionPayload);
        });
    }
    function handleAssignTask(message) {
        var payload = message && message.payload ? message.payload : {};
        var summary = buildTaskSummary(payload);
        logger.info("收到 assign_task：" + JSON.stringify(summary));
        if (onAssignTask) {
            try {
                onAssignTask(summary);
            }
            catch (error) {
                logger.warn("assign_task 回调执行失败：" + String(error));
            }
        }
        sendTaskAck(payload);
        scheduleTaskExecution(summary);
    }
    function handleSyncScript(message) {
        var payload = message && message.payload ? message.payload : {};
        logger.info("收到 sync_script：" + JSON.stringify(payload));
        sendScriptSyncAck(payload);
        scheduleScriptSync(payload);
    }
    function handleStartWorkflowSession(message) {
        var payload = message && message.payload ? message.payload : {};
        var refs = buildWorkflowSessionRefs(payload);
        logger.info("收到 start_workflow_session：" + JSON.stringify({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            workflow_def_id: payload.workflow_def_id || "",
            entry_node_id: payload.entry_node_id || ""
        }));
        sendWorkflowSessionAck(payload);
        scheduleWorkflowSession(payload);
    }
    function handleStopWorkflowSession(message) {
        var payload = message && message.payload ? message.payload : {};
        var refs = buildWorkflowSessionRefs(payload);
        var sessionKey = String(refs.plan_device_run_id || "");
        logger.info("收到 stop_workflow_session：" + JSON.stringify({
            plan_run_id: refs.plan_run_id,
            plan_device_run_id: refs.plan_device_run_id,
            reason: String(payload.reason || "")
        }));
        markWorkflowStopRequested(sessionKey);
        if (!workflowSessionExecuting || currentWorkflowRunID !== sessionKey) {
            sendWorkflowSessionResult({
                plan_run_id: refs.plan_run_id,
                plan_device_run_id: refs.plan_device_run_id,
                workflow_node_id: "",
                status: "stopped",
                result_code: "WORKFLOW_SESSION_STOPPED",
                result_message: "工作流会话已停止",
                extra: {
                    reason: String(payload.reason || "")
                }
            });
            clearWorkflowStopRequested(sessionKey);
        }
    }
    function createListener(expectedGeneration) {
        return createWebSocketListener({
            onOpen: function (_webSocket, _response) {
                if (expectedGeneration !== connectGeneration) {
                    _webSocket.close(1000, "stale generation");
                    return;
                }
                socket = _webSocket;
                closedGeneration = 0;
                lastReceiveAt = Date.now();
                logger.info("WebSocket 已连接：" + wsURL);
                try {
                    send("hello", createRequestID("agent-hello"), {
                        agent_uuid: agentUUID,
                        device_link_sn: deviceLinkSN,
                        execution_profile: runtime.collectExecutionProfile()
                    });
                }
                catch (error) {
                    logger.error("WebSocket hello 发送失败：" + String(error));
                    closeSocketQuietly(1001, "hello send failed");
                    handleSocketClosed(expectedGeneration, "hello_send_failed " + String(error));
                }
            },
            onMessage: function (_webSocket, text) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                lastReceiveAt = Date.now();
                logger.info("收到 WebSocket 消息：" + String(text));
                try {
                    var message = JSON.parse(String(text));
                    if (isOkAck(message, "hello")) {
                        logger.info("WebSocket hello 已确认。");
                        resetReconnectState();
                        startHeartbeat(expectedGeneration);
                    }
                    else if (isOkAck(message, "heartbeat")) {
                        logger.info("WebSocket heartbeat 已确认。");
                    }
                    else if (isOkAck(message, "task_ack")) {
                        logger.info("WebSocket task_ack 已确认。");
                    }
                    else if (isOkAck(message, "task_progress")) {
                        logger.info("WebSocket task_progress 已确认。");
                    }
                    else if (isOkAck(message, "script_sync_ack")) {
                        logger.info("WebSocket script_sync_ack 已确认。");
                    }
                    else if (isOkAck(message, "script_sync_result")) {
                        logger.info("WebSocket script_sync_result 已确认。");
                    }
                    else if (isOkAck(message, "task_result")) {
                        logger.info("WebSocket task_result 已确认。");
                    }
                    else if (isOkAck(message, "workflow_session_ack")) {
                        logger.info("WebSocket workflow_session_ack 已确认。");
                    }
                    else if (isOkAck(message, "workflow_session_event")) {
                        logger.info("WebSocket workflow_session_event 已确认。");
                    }
                    else if (isOkAck(message, "workflow_session_result")) {
                        logger.info("WebSocket workflow_session_result 已确认。");
                    }
                    else if (message && message.type === "assign_task") {
                        handleAssignTask(message);
                    }
                    else if (message && message.type === "start_workflow_session") {
                        handleStartWorkflowSession(message);
                    }
                    else if (message && message.type === "stop_workflow_session") {
                        handleStopWorkflowSession(message);
                    }
                    else if (message && message.type === "sync_script") {
                        handleSyncScript(message);
                    }
                }
                catch (error) {
                    logger.warn("WebSocket 消息解析失败：" + String(error));
                }
            },
            onClosing: function (_webSocket, code, reason) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.warn("WebSocket 正在关闭，code=" + code + "，reason=" + String(reason));
                _webSocket.close(code, reason);
            },
            onClosed: function (_webSocket, code, reason) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.warn("WebSocket 已关闭，code=" + code + "，reason=" + String(reason));
                handleSocketClosed(expectedGeneration, "closed code=" + code + " reason=" + String(reason));
            },
            onFailure: function (_webSocket, throwable, _response) {
                if (expectedGeneration !== connectGeneration) {
                    return;
                }
                logger.error("WebSocket 连接失败：" + String(throwable));
                handleSocketClosed(expectedGeneration, "failure " + String(throwable));
            }
        });
    }
    function openSocket() {
        var request = new Request.Builder().url(wsURL).build();
        connectGeneration += 1;
        closedGeneration = 0;
        socket = null;
        lastReceiveAt = Date.now();
        lastSendAt = Date.now();
        logger.info("开始连接 WebSocket：" + wsURL + "，connect_generation=" + connectGeneration);
        client.newWebSocket(request, createListener(connectGeneration));
    }
    startWatchdog();
    openSocket();
    return {
        url: wsURL,
        close: function () {
            intentionallyClosed = true;
            stopHeartbeat();
            stopReconnect();
            stopWatchdog();
            if (socket) {
                socket.close(1000, "agent close");
            }
            socket = null;
        }
    };
}
function connect(options) {
    var logger = options && options.logger ? options.logger : runtime.createLogger();
    var config = options || {};
    if (!config.deviceID) {
        throw new Error("WebSocket 连接缺少 device_id。");
    }
    if (runtime.isAutoJsRuntime()) {
        return connectAutoJs(config);
    }
    logger.warn("当前运行环境不是 AutoJs6，已跳过 WebSocket 长连接。");
    return {
        url: buildWebSocketURL(config.centerBaseURL),
        skipped: true
    };
}

  },
  "./agent": function(module, exports, __bundleRequire) {
"use strict";
var runtime = __bundleRequire("./lib/runtime");
var configStore = __bundleRequire("./lib/config_store");
var centerClient = __bundleRequire("./lib/center_client");
var wsClient = __bundleRequire("./lib/ws_client");
var STOP_SIGNAL_CHECK_INTERVAL_MS = 2000;
var RUNTIME_LOCK_STALE_MS = 2 * 60 * 1000;
var FALLBACK_ENGINE_ID_PREFIX = "fallback-engine-";
function parseCLIArgs(args) {
    var result = {
        center: "",
        config: "",
        dryRun: false,
        skipWS: false
    };
    for (var index = 0; index < args.length; index += 1) {
        var item = args[index];
        if (item === "--center") {
            result.center = args[index + 1] || "";
            index += 1;
        }
        else if (item === "--config") {
            result.config = args[index + 1] || "";
            index += 1;
        }
        else if (item === "--dry-run") {
            result.dryRun = true;
        }
        else if (item === "--skip-ws") {
            result.skipWS = true;
        }
    }
    return result;
}
function buildRegisterPayload(agentUUID, deviceInfo, deviceLinkSN) {
    return {
        agent_uuid: agentUUID,
        device_name: deviceInfo.device_name,
        brand: deviceInfo.brand,
        model: deviceInfo.model,
        android_id: deviceInfo.android_id,
        adb_serial: deviceInfo.adb_serial,
        device_link_sn: String(deviceLinkSN || "")
    };
}
function shallowCopy(input) {
    var output = {};
    var source = input || {};
    for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
            output[key] = source[key];
        }
    }
    return output;
}
function normalizePreferredHeartbeatScheduler(config, logger) {
    var nextConfig = shallowCopy(config);
    var websocket = shallowCopy(nextConfig.websocket || {});
    var currentMode = String(websocket.heartbeat_scheduler || "");
    if (!currentMode || currentMode === "alarm_manager") {
        websocket.heartbeat_scheduler = "executor";
        nextConfig.websocket = websocket;
        logger.info("已将心跳调度模式迁移为 executor。");
        return nextConfig;
    }
    nextConfig.websocket = websocket;
    return nextConfig;
}
function mergeBootstrapConfig(config, bootstrap) {
    var nextConfig = shallowCopy(config);
    var source = bootstrap || {};
    if (source.center_base_url) {
        nextConfig.center_base_url = source.center_base_url;
    }
    if (source.device_link_sn) {
        nextConfig.device_link_sn = source.device_link_sn;
    }
    if (source.websocket) {
        nextConfig.websocket = shallowCopy(nextConfig.websocket || {});
        if (Object.prototype.hasOwnProperty.call(source.websocket, "enabled")) {
            nextConfig.websocket.enabled = source.websocket.enabled;
        }
        if (source.websocket.heartbeat_interval_ms) {
            nextConfig.websocket.heartbeat_interval_ms = source.websocket.heartbeat_interval_ms;
        }
        if (source.websocket.heartbeat_scheduler) {
            nextConfig.websocket.heartbeat_scheduler = source.websocket.heartbeat_scheduler;
        }
        if (Object.prototype.hasOwnProperty.call(source.websocket, "reconnect_enabled")) {
            nextConfig.websocket.reconnect_enabled = source.websocket.reconnect_enabled;
        }
        if (source.websocket.reconnect_initial_delay_ms) {
            nextConfig.websocket.reconnect_initial_delay_ms = source.websocket.reconnect_initial_delay_ms;
        }
        if (source.websocket.reconnect_max_delay_ms) {
            nextConfig.websocket.reconnect_max_delay_ms = source.websocket.reconnect_max_delay_ms;
        }
        if (source.websocket.reconnect_backoff_multiplier) {
            nextConfig.websocket.reconnect_backoff_multiplier = source.websocket.reconnect_backoff_multiplier;
        }
    }
    return nextConfig;
}
function mergeRegisterResult(config, response) {
    var data = response && response.data ? response.data : {};
    var nextConfig = shallowCopy(config);
    nextConfig.device_id = data.device_id || nextConfig.device_id || "";
    nextConfig.last_register = {
        status: response.status || "",
        device_id: nextConfig.device_id,
        bind_status: data.bind_status || "",
        register_status: data.status || "",
        registered_at: runtime.nowISOString()
    };
    return nextConfig;
}
function isPromiseLike(value) {
    return !!(value && typeof value.then === "function");
}
function getStopSignalPath(store) {
    if (store && store.stopSignalPath) {
        return store.stopSignalPath;
    }
    return configStore.defaultStopSignalPath();
}
function getRuntimeLockPath(store) {
    if (store && store.runtimeLockPath) {
        return store.runtimeLockPath;
    }
    return configStore.defaultRuntimeLockPath();
}
function parseRuntimeLock(text) {
    if (!text || !String(text).trim()) {
        return {};
    }
    try {
        return JSON.parse(String(text));
    }
    catch (_error) {
        return {};
    }
}
function getCurrentEngineID() {
    try {
        if (typeof engines !== "undefined" && typeof engines.myEngine === "function") {
            var current = engines.myEngine();
            if (current && current.id) {
                return String(current.id);
            }
            if (current && typeof current.toString === "function") {
                return String(current.toString());
            }
        }
    }
    catch (_error) {
        return "";
    }
    return "";
}
function createFallbackEngineID() {
    return FALLBACK_ENGINE_ID_PREFIX + Date.now().toString(36) + "-" + Math.floor(Math.random() * 100000).toString(36);
}
function isFallbackEngineID(engineID) {
    return String(engineID || "").indexOf(FALLBACK_ENGINE_ID_PREFIX) === 0;
}
function isEngineAlive(engineID) {
    if (!engineID) {
        return false;
    }
    if (isFallbackEngineID(engineID)) {
        return false;
    }
    try {
        if (typeof engines === "undefined" || typeof engines.all !== "function") {
            return false;
        }
        var list = engines.all();
        for (var index = 0; index < list.length; index += 1) {
            if (String(list[index].id) === String(engineID)) {
                return true;
            }
        }
    }
    catch (_error) {
        return false;
    }
    return false;
}
function acquireRuntimeLock(store, logger) {
    var lockPath = getRuntimeLockPath(store);
    var currentEngineID = getCurrentEngineID() || createFallbackEngineID();
    var nowISO = runtime.nowISOString();
    var existing = parseRuntimeLock(runtime.readTextFile(lockPath));
    var existingUpdatedAt = Date.parse(existing.updated_at || "");
    var isStale = !existingUpdatedAt || (Date.now() - existingUpdatedAt) > RUNTIME_LOCK_STALE_MS;
    var hasExistingLock = !!(existing.engine_id || existing.acquired_at || existing.updated_at);
    var shouldBlockByExistingLock = hasExistingLock && !isStale;
    if (shouldBlockByExistingLock) {
        logger.warn("检测到未过期的 Agent 运行锁，当前实例不再重复启动：" + lockPath);
        return {
            alreadyRunning: true,
            path: lockPath,
            release: function () { }
        };
    }
    runtime.writeTextFile(lockPath, JSON.stringify({
        engine_id: currentEngineID,
        acquired_at: nowISO,
        updated_at: nowISO
    }, null, 2));
    logger.info("已获取 Agent 运行锁：" + lockPath);
    return {
        alreadyRunning: false,
        path: lockPath,
        release: function () {
            var latest = parseRuntimeLock(runtime.readTextFile(lockPath));
            if (!latest.engine_id || latest.engine_id === currentEngineID) {
                runtime.removeFileIfExists(lockPath);
                logger.info("已释放 Agent 运行锁：" + lockPath);
            }
        }
    };
}
function clearStaleStopSignal(store, logger) {
    var stopSignalPath = getStopSignalPath(store);
    if (!runtime.fileExists(stopSignalPath)) {
        return;
    }
    runtime.removeFileIfExists(stopSignalPath);
    logger.info("检测到遗留停止信号，启动前已清理：" + stopSignalPath);
}
function startStopSignalMonitor(options) {
    if (runtime.isNodeRuntime()) {
        return null;
    }
    var monitorOptions = options || {};
    var logger = monitorOptions.logger || runtime.createLogger();
    var websocket = monitorOptions.websocket;
    var cleanup = monitorOptions.cleanup || null;
    var stopSignalPath = getStopSignalPath(monitorOptions.store);
    var monitor = null;
    var stopped = false;
    function shutdownAgent() {
        if (stopped) {
            return;
        }
        stopped = true;
        logger.info("检测到 stop.signal，Agent 即将优雅退出：" + stopSignalPath);
        runtime.removeFileIfExists(stopSignalPath);
        if (monitor) {
            monitor.cancel();
            monitor = null;
        }
        if (websocket && typeof websocket.close === "function") {
            websocket.close();
        }
        if (cleanup && typeof cleanup.release === "function") {
            cleanup.release();
        }
        logger.info("Agent 已停止心跳与重连，准备退出当前脚本。");
        runtime.sleepMS(300);
        runtime.exitProcess(0);
    }
    monitor = runtime.startInterval(function watchStopSignal() {
        if (runtime.fileExists(stopSignalPath)) {
            shutdownAgent();
        }
    }, STOP_SIGNAL_CHECK_INTERVAL_MS);
    return {
        path: stopSignalPath,
        close: function () {
            stopped = true;
            if (monitor) {
                monitor.cancel();
                monitor = null;
            }
        }
    };
}
function main(cliOptions) {
    var logger = runtime.createLogger();
    var options = cliOptions || {};
    var store = configStore.createConfigStore({ configPath: options.config });
    var runtimeLock = acquireRuntimeLock(store, logger);
    if (runtimeLock.alreadyRunning) {
        logger.info("检测到 Agent 已在运行，当前启动请求直接结束：" + runtimeLock.path);
        return {
            status: "already_running",
            runtime_lock: runtimeLock.path
        };
    }
    try {
        var configExists = store.exists();
        var config_1 = store.load();
        if (store.bootstrapExists()) {
            config_1 = mergeBootstrapConfig(config_1, store.loadBootstrap());
            if (configExists) {
                logger.info("检测到 bootstrap 配置，已刷新中心地址与连接参数，并保留本地身份配置。");
            }
            else {
                logger.info("检测到 bootstrap 配置，已用于初始化本地 config.json。");
            }
        }
        clearStaleStopSignal(store, logger);
        if (options.center) {
            config_1.center_base_url = options.center;
        }
        config_1 = normalizePreferredHeartbeatScheduler(config_1, logger);
        var deviceInfo = runtime.collectDeviceInfo(config_1.device);
        config_1.device = deviceInfo;
        if (!config_1.agent_uuid) {
            config_1.agent_uuid = runtime.createStableAgentUUID(deviceInfo);
            logger.info("已根据稳定指纹生成 agent_uuid：" + config_1.agent_uuid);
        }
        store.save(config_1);
        var payload = buildRegisterPayload(config_1.agent_uuid, deviceInfo, config_1.device_link_sn);
        logger.info("Agent 配置文件：" + store.configPath);
        logger.info("Agent 引导文件：" + store.bootstrapPath);
        logger.info("Agent 运行锁文件：" + runtimeLock.path);
        logger.info("中心服务地址：" + config_1.center_base_url);
        if (options.dryRun) {
            logger.info("当前为 dry-run，仅验证配置和注册载荷，不请求中心服务。");
            runtimeLock.release();
            return {
                status: "dry_run",
                config_path: store.configPath,
                agent_uuid: config_1.agent_uuid,
                device_id: config_1.device_id || "",
                register_payload: payload
            };
        }
        logger.info("开始向中心服务注册设备。");
        var response = centerClient.registerDevice(config_1.center_base_url, payload);
        if (isPromiseLike(response)) {
            return response.then(function onResolved(resolvedResponse) {
                return finishRegister(config_1, store, resolvedResponse, logger, options, runtimeLock);
            }).catch(function onRejected(error) {
                runtimeLock.release();
                throw error;
            });
        }
        return finishRegister(config_1, store, response, logger, options, runtimeLock);
    }
    catch (error) {
        runtimeLock.release();
        throw error;
    }
}
function finishRegister(config, store, response, logger, options, runtimeLock) {
    var nextConfig = mergeRegisterResult(config, response);
    var websocketResult = null;
    var stopSignalMonitor = null;
    store.save(nextConfig);
    logger.info("设备注册完成，device_id=" + nextConfig.device_id);
    if (shouldStartWebSocket(nextConfig, options)) {
        websocketResult = wsClient.connect({
            centerBaseURL: nextConfig.center_base_url,
            deviceID: nextConfig.device_id,
            agentUUID: nextConfig.agent_uuid,
            deviceLinkSN: nextConfig.device_link_sn || "",
            heartbeatLeasePath: configStore.defaultHeartbeatLeasePath(),
            heartbeatIntervalMS: getHeartbeatIntervalMS(nextConfig),
            heartbeatScheduler: getHeartbeatScheduler(nextConfig),
            pingIntervalMS: getPingIntervalMS(nextConfig),
            watchdogIntervalMS: getWSWatchdogIntervalMS(nextConfig),
            silenceTimeoutMS: getWSSilenceTimeoutMS(nextConfig),
            reconnectEnabled: getReconnectEnabled(nextConfig),
            reconnectInitialDelayMS: getReconnectInitialDelayMS(nextConfig),
            reconnectMaxDelayMS: getReconnectMaxDelayMS(nextConfig),
            reconnectBackoffMultiplier: getReconnectBackoffMultiplier(nextConfig),
            onAssignTask: function (taskSummary) {
                logger.info("Agent 已接收任务摘要：" + JSON.stringify(taskSummary));
            },
            logger: logger
        });
        stopSignalMonitor = startStopSignalMonitor({
            store: store,
            websocket: websocketResult,
            logger: logger,
            cleanup: {
                release: function () {
                    runtimeLock.release();
                }
            }
        });
    }
    else {
        logger.info("已跳过 WebSocket 连接。");
    }
    return {
        status: "registered",
        config_path: store.configPath,
        agent_uuid: nextConfig.agent_uuid,
        device_id: nextConfig.device_id,
        websocket: websocketResult,
        stop_signal_monitor: stopSignalMonitor,
        runtime_lock: runtimeLock ? runtimeLock.path : "",
        response: response
    };
}
function shouldStartWebSocket(config, options) {
    var websocketConfig = config.websocket || {};
    if (options && options.skipWS) {
        return false;
    }
    if (options && options.dryRun) {
        return false;
    }
    if (websocketConfig.enabled === false) {
        return false;
    }
    if (runtime.isNodeRuntime()) {
        return false;
    }
    return !!config.device_id;
}
function getHeartbeatIntervalMS(config) {
    var websocketConfig = config.websocket || {};
    var interval = Number(websocketConfig.heartbeat_interval_ms || 30000);
    if (!interval || interval < 1000) {
        return 30000;
    }
    return interval;
}
function getHeartbeatScheduler(config) {
    var websocketConfig = config.websocket || {};
    return String(websocketConfig.heartbeat_scheduler || "executor");
}
function getPingIntervalMS(config) {
    var heartbeatIntervalMS = getHeartbeatIntervalMS(config);
    return Math.max(5000, Math.min(heartbeatIntervalMS, 20000));
}
function getWSWatchdogIntervalMS(config) {
    var keepAliveConfig = config.keep_alive || {};
    var intervalSeconds = Number(keepAliveConfig.ws_watchdog_interval_seconds || 20);
    if (!intervalSeconds || intervalSeconds < 5) {
        return 20000;
    }
    return intervalSeconds * 1000;
}
function getWSSilenceTimeoutMS(config) {
    var keepAliveConfig = config.keep_alive || {};
    var timeoutSeconds = Number(keepAliveConfig.ws_silence_timeout_seconds || 120);
    if (!timeoutSeconds || timeoutSeconds < 10) {
        return 120000;
    }
    return timeoutSeconds * 1000;
}
function getReconnectEnabled(config) {
    var websocketConfig = config.websocket || {};
    return websocketConfig.reconnect_enabled !== false;
}
function getReconnectInitialDelayMS(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_initial_delay_ms || 3000);
    if (!value || value < 1000) {
        return 3000;
    }
    return value;
}
function getReconnectMaxDelayMS(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_max_delay_ms || 60000);
    if (!value || value < 1000) {
        return 60000;
    }
    return value;
}
function getReconnectBackoffMultiplier(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_backoff_multiplier || 2);
    if (!value || value < 1) {
        return 2;
    }
    return value;
}
function run() {
    var args = runtime.isNodeRuntime() ? parseCLIArgs(process.argv.slice(2)) : {};
    var logger = runtime.createLogger();
    try {
        var result = main(args);
        if (isPromiseLike(result)) {
            result.then(function onResolved(resolvedResult) {
                logger.info("Agent 启动结果：" + JSON.stringify(resolvedResult));
            }).catch(function onRejected(error) {
                logRunError(error, logger);
            });
            return;
        }
        logger.info("Agent 启动结果：" + JSON.stringify(result));
    }
    catch (error) {
        logRunError(error, logger);
    }
}
function logRunError(error, logger) {
    if (error && typeof error === "object" && "message" in error && error.message === "agent_instance_already_running") {
        logger.info("检测到 Agent 已在运行，当前启动请求直接结束。");
        return;
    }
    var hasMessage = !!(error && typeof error === "object" && "message" in error && error.message);
    var hasStack = !!(error && typeof error === "object" && "stack" in error && error.stack);
    if (hasMessage) {
        logger.error(String(error.message));
    }
    if (hasStack) {
        logger.error(String(error.stack));
    }
    if (!hasMessage && !hasStack) {
        logger.error(String(error));
    }
    if (runtime.isNodeRuntime()) {
        process.exitCode = 1;
    }
}
if (runtime.isNodeRuntime()) {
    if (require.main === module) {
        run();
    }
}
else {
    run();
}
if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        main: main,
        parseCLIArgs: parseCLIArgs,
        buildRegisterPayload: buildRegisterPayload,
        mergeBootstrapConfig: mergeBootstrapConfig,
        mergeRegisterResult: mergeRegisterResult,
        acquireRuntimeLock: acquireRuntimeLock,
        getHeartbeatIntervalMS: getHeartbeatIntervalMS,
        getReconnectEnabled: getReconnectEnabled,
        getReconnectInitialDelayMS: getReconnectInitialDelayMS,
        getReconnectMaxDelayMS: getReconnectMaxDelayMS,
        getReconnectBackoffMultiplier: getReconnectBackoffMultiplier,
        createFallbackEngineID: createFallbackEngineID
    };
}

  }
};
var __bundleCache = {};
function __bundleResolve(moduleName) {
  var normalized = String(moduleName || "");
  if (__bundleModules[normalized]) {
    return normalized;
  }
  if (__bundleModules[normalized + ".js"]) {
    return normalized + ".js";
  }
  throw new Error("bundle_module_not_found:" + normalized);
}
function __bundleRequire(moduleName) {
  var resolved = __bundleResolve(moduleName);
  if (__bundleCache[resolved]) {
    return __bundleCache[resolved].exports;
  }
  var module = { exports: {} };
  __bundleCache[resolved] = module;
  __bundleModules[resolved](module, module.exports, __bundleRequire);
  return module.exports;
}
__bundleRequire("./agent");
})();

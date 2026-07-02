"use strict";

var runtime = require("./runtime");

/**
 * Agent 配置对象。
 *
 * @typedef {Object} AgentConfig
 * @property {string} center_base_url 中心服务基础地址。
 * @property {string} agent_uuid Agent 本地稳定标识。
 * @property {string} device_id 中心服务返回的设备标识。
 * @property {string} device_link_sn 设备发现阶段下发的链路标识。
 * @property {Object} device 设备信息覆盖项。
 * @property {Object} websocket WebSocket 连接配置。
 * @property {Object} last_register 最近一次注册结果。
 * @property {string} created_at 配置创建时间。
 * @property {string} updated_at 配置更新时间。
 */

/**
 * 判断当前是否运行在 Node.js。
 *
 * @returns {boolean} 当前是否为 Node.js。
 */
function isNodeRuntime() {
    return runtime.isNodeRuntime();
}

/**
 * 拼接路径片段。
 *
 * @param {...string} parts 路径片段。
 * @returns {string} 拼接后的路径。
 */
function joinPath() {
    var parts = Array.prototype.slice.call(arguments).filter(Boolean);
    if (isNodeRuntime()) {
        return require("path").join.apply(null, parts);
    }
    return normalizePath(parts.join("/"));
}

/**
 * 获取 Agent 根目录，优先返回绝对路径。
 *
 * @returns {string} Agent 根目录。
 */
function resolveAgentRootPath() {
    if (isNodeRuntime()) {
        return require("path").resolve(typeof __dirname !== "undefined" ? joinPath(__dirname, "..") : ".");
    }

    if (typeof files !== "undefined" && typeof files.path === "function") {
        return normalizePath(String(files.path(".") || "."));
    }

    if (typeof __dirname !== "undefined") {
        return normalizePath(joinPath(__dirname, ".."));
    }

    return ".";
}

/**
 * 归一化路径，移除重复分隔符与简单的 "." / ".." 片段。
 *
 * @param {string} input 原始路径。
 * @returns {string} 归一化后的路径。
 */
function normalizePath(input) {
    var raw = String(input || "").replace(/\\/g, "/");
    var absolute = raw.charAt(0) === "/";
    var segments = raw.split("/");
    var stack = [];
    var i = 0;

    for (i = 0; i < segments.length; i += 1) {
        if (!segments[i] || segments[i] === ".") {
            continue;
        }
        if (segments[i] === "..") {
            if (stack.length > 0 && stack[stack.length - 1] !== "..") {
                stack.pop();
            } else if (!absolute) {
                stack.push("..");
            }
            continue;
        }
        stack.push(segments[i]);
    }

    var output = stack.join("/");
    if (absolute) {
        output = "/" + output;
    }
    return output || (absolute ? "/" : ".");
}

/**
 * 返回路径所在目录。
 *
 * @param {string} filePath 文件路径。
 * @returns {string} 目录路径。
 */
function dirname(filePath) {
    if (isNodeRuntime()) {
        return require("path").dirname(filePath);
    }
    var normalized = String(filePath).replace(/\\/g, "/");
    var index = normalized.lastIndexOf("/");
    return index >= 0 ? normalized.slice(0, index) : ".";
}

/**
 * 获取默认配置文件路径。
 *
 * @returns {string} 默认配置文件路径。
 */
function defaultConfigPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "config.json");
}

/**
 * 获取默认启动引导配置路径。
 *
 * @returns {string} 默认 bootstrap 配置路径。
 */
function defaultBootstrapPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "bootstrap.json");
}

/**
 * 获取默认停止信号文件路径。
 * @returns {string} 默认停止信号文件路径。
 */
function defaultStopSignalPath() {
    var root = resolveAgentRootPath();
    return joinPath(root, "runtime", "stop.signal");
}

/**
 * 获取默认运行锁文件路径。
 *
 * @returns {string} 默认运行锁文件路径。
 */
function defaultRuntimeLockPath() {
    return joinPath(resolveAgentRootPath(), "runtime", "agent.lock.json");
}

/**
 * 判断文件是否存在。
 *
 * @param {string} filePath 文件路径。
 * @returns {boolean} 文件是否存在。
 */
function exists(filePath) {
    if (isNodeRuntime()) {
        return require("fs").existsSync(filePath);
    }
    return typeof files !== "undefined" && files.exists(filePath);
}

/**
 * 读取文本文件。
 *
 * @param {string} filePath 文件路径。
 * @returns {string} 文件内容。
 */
function readText(filePath) {
    if (isNodeRuntime()) {
        return require("fs").readFileSync(filePath, "utf8");
    }
    return files.read(filePath);
}

/**
 * 写入文本文件，写入前会创建父目录。
 *
 * @param {string} filePath 文件路径。
 * @param {string} content 文件内容。
 */
function writeText(filePath, content) {
    if (isNodeRuntime()) {
        var fs = require("fs");
        fs.mkdirSync(dirname(filePath), { recursive: true });
        fs.writeFileSync(filePath, content, "utf8");
        return;
    }

    if (typeof files !== "undefined") {
        files.createWithDirs(filePath);
        files.write(filePath, content);
    }
}

/**
 * 创建空配置对象。
 *
 * @returns {AgentConfig} 配置对象。
 */
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
            reconnect_enabled: true,
            reconnect_initial_delay_ms: 3000,
            reconnect_max_delay_ms: 60000,
            reconnect_backoff_multiplier: 2
        },
        last_register: {},
        created_at: now,
        updated_at: now
    };
}

/**
 * 规范化配置对象字段。
 *
 * @param {Partial<AgentConfig>} raw 原始配置。
 * @returns {AgentConfig} 规范化后的配置。
 */
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
        last_register: input.last_register || {},
        created_at: input.created_at || base.created_at,
        updated_at: input.updated_at || base.updated_at
    };
}

/**
 * 规范化 WebSocket 配置。
 *
 * @param {Object} input 原始 WebSocket 配置。
 * @param {Object} fallback 默认 WebSocket 配置。
 * @returns {Object} 规范化后的 WebSocket 配置。
 */
function normalizeWebSocketConfig(input, fallback) {
    var source = input || {};
    var base = fallback || {};
    return {
        enabled: source.enabled === false ? false : base.enabled !== false,
        heartbeat_interval_ms: source.heartbeat_interval_ms || base.heartbeat_interval_ms || 30000,
        reconnect_enabled: source.reconnect_enabled === false ? false : base.reconnect_enabled !== false,
        reconnect_initial_delay_ms: source.reconnect_initial_delay_ms || base.reconnect_initial_delay_ms || 3000,
        reconnect_max_delay_ms: source.reconnect_max_delay_ms || base.reconnect_max_delay_ms || 60000,
        reconnect_backoff_multiplier: source.reconnect_backoff_multiplier || base.reconnect_backoff_multiplier || 2
    };
}

/**
 * 创建配置存储对象。
 *
 * @param {{configPath?: string}} options 配置选项。
 * @returns {{configPath: string, load: Function, save: Function}} 配置存储对象。
 */
function createConfigStore(options) {
    var configPath = options && options.configPath ? options.configPath : defaultConfigPath();
    var bootstrapPath = options && options.bootstrapPath ? options.bootstrapPath : defaultBootstrapPath();
    var stopSignalPath = options && options.stopSignalPath ? options.stopSignalPath : defaultStopSignalPath();

    return {
        configPath: configPath,
        bootstrapPath: bootstrapPath,
        stopSignalPath: stopSignalPath,

        /**
         * 判断主配置文件是否存在。
         *
         * @returns {boolean} 主配置文件是否存在。
         */
        exists: function () {
            return exists(configPath);
        },

        /**
         * 判断 bootstrap 配置文件是否存在。
         *
         * @returns {boolean} bootstrap 配置文件是否存在。
         */
        bootstrapExists: function () {
            return exists(bootstrapPath);
        },

        /**
         * 读取 bootstrap 配置。
         *
         * @returns {Object} bootstrap 配置对象。
         */
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

        /**
         * 加载本地配置。
         *
         * @returns {AgentConfig} 配置对象。
         */
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

        /**
         * 保存本地配置。
         *
         * @param {AgentConfig} config 配置对象。
         */
        save: function (config) {
            var nextConfig = normalizeConfig(config);
            nextConfig.updated_at = runtime.nowISOString();
            writeText(configPath, JSON.stringify(nextConfig, null, 2));
        }
    };
}

module.exports = {
    defaultConfigPath: defaultConfigPath,
    defaultBootstrapPath: defaultBootstrapPath,
    defaultStopSignalPath: defaultStopSignalPath,
    defaultRuntimeLockPath: defaultRuntimeLockPath,
    createConfigStore: createConfigStore
};

"use strict";

var runtime = require("./lib/runtime");
var configStore = require("./lib/config_store");
var centerClient = require("./lib/center_client");
var wsClient = require("./lib/ws_client");

var STOP_SIGNAL_CHECK_INTERVAL_MS = 2000;
var RUNTIME_LOCK_STALE_MS = 2 * 60 * 1000;

/**
 * Agent 命令行参数。
 *
 * @typedef {Object} AgentCLIOptions
 * @property {string} center 中心服务基础地址。
 * @property {string} config 配置文件路径。
 * @property {boolean} dryRun 是否仅验证配置而不请求中心服务。
 * @property {boolean} skipWS 是否跳过 WebSocket 连接。
 */

/**
 * 解析 Node.js 命令行参数。
 *
 * @param {string[]} args 原始参数列表。
 * @returns {AgentCLIOptions} 解析后的参数对象。
 */
function parseCLIArgs(args) {
    var result = {
        center: "",
        config: "",
        dryRun: false,
        skipWS: false
    };

    for (var i = 0; i < args.length; i += 1) {
        var item = args[i];
        if (item === "--center") {
            result.center = args[i + 1] || "";
            i += 1;
        } else if (item === "--config") {
            result.config = args[i + 1] || "";
            i += 1;
        } else if (item === "--dry-run") {
            result.dryRun = true;
        } else if (item === "--skip-ws") {
            result.skipWS = true;
        }
    }

    return result;
}

/**
 * 构建设备注册请求体。
 *
 * @param {string} agentUUID Agent 本地稳定标识。
 * @param {Object} deviceInfo 设备信息。
 * @returns {Object} 设备注册请求体。
 */
function buildRegisterPayload(agentUUID, deviceInfo) {
    return {
        agent_uuid: agentUUID,
        device_name: deviceInfo.device_name,
        brand: deviceInfo.brand,
        model: deviceInfo.model,
        android_id: deviceInfo.android_id,
        adb_serial: deviceInfo.adb_serial
    };
}

/**
 * 浅拷贝对象。
 *
 * @param {Object} input 原始对象。
 * @returns {Object} 拷贝后的对象。
 */
function shallowCopy(input) {
    var output = {};
    var source = input || {};
    var key = "";

    for (key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
            output[key] = source[key];
        }
    }

    return output;
}

/**
 * 将 bootstrap 配置合并到主配置。
 *
 * @param {Object} config 主配置。
 * @param {Object} bootstrap bootstrap 配置。
 * @returns {Object} 合并后的配置。
 */
function mergeBootstrapConfig(config, bootstrap) {
    var nextConfig = shallowCopy(config);
    var source = bootstrap || {};

    if (source.center_base_url) {
        nextConfig.center_base_url = source.center_base_url;
    }

    if (source.websocket) {
        nextConfig.websocket = shallowCopy(nextConfig.websocket || {});
        if (Object.prototype.hasOwnProperty.call(source.websocket, "enabled")) {
            nextConfig.websocket.enabled = source.websocket.enabled;
        }
        if (source.websocket.heartbeat_interval_ms) {
            nextConfig.websocket.heartbeat_interval_ms = source.websocket.heartbeat_interval_ms;
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

/**
 * 合并注册结果。
 *
 * @param {Object} config 注册前配置。
 * @param {Object} response 注册响应。
 * @returns {Object} 合并后的配置。
 */
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

/**
 * 判断对象是否是 Promise 风格结果。
 *
 * @param {Object} value 待判断对象。
 * @returns {boolean} 是否包含 then 方法。
 */
function isPromiseLike(value) {
    return !!(value && typeof value.then === "function");
}

/**
 * 获取停止信号文件路径。
 *
 * @param {Object} store 配置存储对象。
 * @returns {string} 停止信号文件路径。
 */
function getStopSignalPath(store) {
    if (store && store.stopSignalPath) {
        return store.stopSignalPath;
    }
    return configStore.defaultStopSignalPath();
}

/**
 * 获取运行锁文件路径。
 *
 * @param {Object} store 配置存储对象。
 * @returns {string} 运行锁文件路径。
 */
function getRuntimeLockPath(store) {
    if (store && store.runtimeLockPath) {
        return store.runtimeLockPath;
    }
    return configStore.defaultRuntimeLockPath();
}

/**
 * 解析运行锁内容。
 *
 * @param {string} text 运行锁文本。
 * @returns {Object} 运行锁对象。
 */
function parseRuntimeLock(text) {
    if (!text || !String(text).trim()) {
        return {};
    }

    try {
        return JSON.parse(String(text));
    } catch (_error) {
        return {};
    }
}

/**
 * 获取当前脚本引擎标识。
 *
 * @returns {string} 当前引擎标识。
 */
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
    } catch (_error) {
        return "";
    }

    return "";
}

/**
 * 判断指定脚本引擎是否仍然存活。
 *
 * @param {string} engineID 引擎标识。
 * @returns {boolean} 是否仍然存活。
 */
function isEngineAlive(engineID) {
    var list = null;
    var index = 0;

    if (!engineID) {
        return false;
    }

    try {
        if (typeof engines === "undefined" || typeof engines.all !== "function") {
            return false;
        }

        list = engines.all();
        for (index = 0; index < list.length; index += 1) {
            if (String(list[index].id) === String(engineID)) {
                return true;
            }
        }
    } catch (_error) {
        return false;
    }

    return false;
}

/**
 * 抢占 Agent 单实例运行锁。
 *
 * @param {Object} store 配置存储对象。
 * @param {Object} logger 日志对象。
 * @returns {{path: string, release: Function}} 运行锁句柄。
 */
function acquireRuntimeLock(store, logger) {
    var lockPath = getRuntimeLockPath(store);
    var currentEngineID = getCurrentEngineID();
    var nowISO = runtime.nowISOString();
    var existing = parseRuntimeLock(runtime.readTextFile(lockPath));
    var existingUpdatedAt = Date.parse(existing.updated_at || "");
    var isStale = !existingUpdatedAt || (Date.now() - existingUpdatedAt) > RUNTIME_LOCK_STALE_MS;

    if (existing.engine_id && existing.engine_id !== currentEngineID && isEngineAlive(existing.engine_id) && !isStale) {
        return {
            alreadyRunning: true,
            path: lockPath,
            release: function () {}
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

/**
 * 清理遗留的停止信号文件。
 *
 * @param {Object} store 配置存储对象。
 * @param {Object} logger 日志对象。
 */
function clearStaleStopSignal(store, logger) {
    var stopSignalPath = getStopSignalPath(store);
    if (!runtime.fileExists(stopSignalPath)) {
        return;
    }

    runtime.removeFileIfExists(stopSignalPath);
    logger.info("检测到遗留停止信号，启动前已清理：" + stopSignalPath);
}

/**
 * 启动停止信号监听。
 *
 * @param {{store: Object, websocket: Object, logger: Object}} options 监听参数。
 * @returns {Object|null} 监听句柄。
 */
function startStopSignalMonitor(options) {
    if (runtime.isNodeRuntime()) {
        return null;
    }

    var monitorOptions = options || {};
    var logger = monitorOptions.logger || runtime.createLogger();
    var websocket = monitorOptions.websocket;
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

        logger.info("Agent 已停止心跳与重连，准备退出当前脚本。");
        runtime.exitProcess(0);
    }

    monitor = runtime.startInterval(function () {
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

/**
 * 执行 Agent 启动与注册流程。
 *
 * @param {AgentCLIOptions} cliOptions 命令行参数。
 * @returns {Object|Promise<Object>} 执行结果。
 */
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
        var config = store.load();

        if (store.bootstrapExists()) {
            config = mergeBootstrapConfig(config, store.loadBootstrap());
            if (configExists) {
                logger.info("检测到 bootstrap 配置，已刷新中心地址与连接参数，并保留本地身份配置。");
            } else {
                logger.info("检测到 bootstrap 配置，已用于初始化本地 config.json。");
            }
        }

        clearStaleStopSignal(store, logger);

        if (options.center) {
            config.center_base_url = options.center;
        }

        var deviceInfo = runtime.collectDeviceInfo(config.device);
        config.device = deviceInfo;

        if (!config.agent_uuid) {
            config.agent_uuid = runtime.createStableAgentUUID(deviceInfo);
            logger.info("已根据稳定指纹生成 agent_uuid：" + config.agent_uuid);
        }

        store.save(config);

        var payload = buildRegisterPayload(config.agent_uuid, deviceInfo);
        logger.info("Agent 配置文件：" + store.configPath);
        logger.info("Agent 引导文件：" + store.bootstrapPath);
        logger.info("Agent 运行锁文件：" + runtimeLock.path);
        logger.info("中心服务地址：" + config.center_base_url);

        if (options.dryRun) {
            logger.info("当前为 dry-run，仅验证配置和注册载荷，不请求中心服务。");
            runtimeLock.release();
            return {
                status: "dry_run",
                config_path: store.configPath,
                agent_uuid: config.agent_uuid,
                device_id: config.device_id || "",
                register_payload: payload
            };
        }

        logger.info("开始向中心服务注册设备。");
        var response = centerClient.registerDevice(config.center_base_url, payload);
        if (isPromiseLike(response)) {
            return response.then(function (resolvedResponse) {
                return finishRegister(config, store, resolvedResponse, logger, options, runtimeLock);
            }).catch(function (error) {
                runtimeLock.release();
                throw error;
            });
        }

        return finishRegister(config, store, response, logger, options, runtimeLock);
    } catch (error) {
        runtimeLock.release();
        throw error;
    }
}

/**
 * 保存注册结果，并根据配置决定是否启动 WebSocket。
 *
 * @param {Object} config 注册前配置。
 * @param {Object} store 配置存储对象。
 * @param {Object} response 注册响应。
 * @param {Object} logger 日志对象。
 * @param {Object} options 命令行参数。
 * @param {{path: string, release: Function}} runtimeLock 运行锁句柄。
 * @returns {Object} 启动结果摘要。
 */
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
            heartbeatIntervalMS: getHeartbeatIntervalMS(nextConfig),
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
            logger: logger
        });
    } else {
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

/**
 * 判断是否需要启动 WebSocket。
 *
 * @param {Object} config 配置对象。
 * @param {Object} options 命令行参数。
 * @returns {boolean} 是否启动。
 */
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

/**
 * 获取心跳间隔。
 *
 * @param {Object} config 配置对象。
 * @returns {number} 心跳间隔，单位毫秒。
 */
function getHeartbeatIntervalMS(config) {
    var websocketConfig = config.websocket || {};
    var interval = Number(websocketConfig.heartbeat_interval_ms || 30000);
    if (!interval || interval < 1000) {
        return 30000;
    }
    return interval;
}

/**
 * 获取是否启用自动重连。
 *
 * @param {Object} config 配置对象。
 * @returns {boolean} 是否启用自动重连。
 */
function getReconnectEnabled(config) {
    var websocketConfig = config.websocket || {};
    return websocketConfig.reconnect_enabled !== false;
}

/**
 * 获取自动重连初始等待时间。
 *
 * @param {Object} config 配置对象。
 * @returns {number} 初始等待时间，单位毫秒。
 */
function getReconnectInitialDelayMS(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_initial_delay_ms || 3000);
    if (!value || value < 1000) {
        return 3000;
    }
    return value;
}

/**
 * 获取自动重连最大等待时间。
 *
 * @param {Object} config 配置对象。
 * @returns {number} 最大等待时间，单位毫秒。
 */
function getReconnectMaxDelayMS(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_max_delay_ms || 60000);
    if (!value || value < 1000) {
        return 60000;
    }
    return value;
}

/**
 * 获取自动重连退避倍数。
 *
 * @param {Object} config 配置对象。
 * @returns {number} 退避倍数。
 */
function getReconnectBackoffMultiplier(config) {
    var websocketConfig = config.websocket || {};
    var value = Number(websocketConfig.reconnect_backoff_multiplier || 2);
    if (!value || value < 1) {
        return 2;
    }
    return value;
}

/**
 * 运行主流程并输出结果。
 */
function run() {
    var args = runtime.isNodeRuntime() ? parseCLIArgs(process.argv.slice(2)) : {};
    var logger = runtime.createLogger();

    try {
        var result = main(args);
        if (isPromiseLike(result)) {
            result.then(function (resolvedResult) {
                logger.info("Agent 启动结果：" + JSON.stringify(resolvedResult));
            }).catch(function (error) {
                logRunError(error, logger);
            });
            return;
        }

        logger.info("Agent 启动结果：" + JSON.stringify(result));
    } catch (error) {
        logRunError(error, logger);
    }
}

/**
 * 输出运行错误。
 *
 * @param {Error} error 错误对象。
 * @param {Object} logger 日志对象。
 */
function logRunError(error, logger) {
    if (error && error.message === "agent_instance_already_running") {
        logger.info("检测到 Agent 已在运行，当前启动请求直接结束。");
        return;
    }

    var hasMessage = !!(error && error.message);
    var hasStack = !!(error && error.stack);

    if (hasMessage) {
        logger.error(error.message);
    }
    if (hasStack) {
        logger.error(error.stack);
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
} else {
    run();
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = {
        main: main,
        parseCLIArgs: parseCLIArgs,
        buildRegisterPayload: buildRegisterPayload
    };
}

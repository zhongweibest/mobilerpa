"use strict";

var runtime = require("./runtime");
var logger = runtime.createLogger();

/**
 * 解析中心服务返回的 JSON 响应。
 *
 * @param {number} statusCode HTTP 状态码。
 * @param {string} text 响应文本。
 * @returns {Object} 响应对象。
 */
function parseJSONResponse(statusCode, text) {
    var body = {};
    try {
        body = text ? JSON.parse(text) : {};
    } catch (error) {
        throw new Error("中心服务返回了非 JSON 响应：" + text);
    }

    if (statusCode < 200 || statusCode >= 300) {
        throw new Error("中心服务请求失败，状态码=" + statusCode + "，响应=" + text);
    }

    return body;
}

/**
 * 提取错误对象中的可读文本。
 *
 * @param {Error|Object|string} error 原始错误对象。
 * @returns {string} 可读错误文本。
 */
function formatErrorMessage(error) {
    if (!error) {
        return "unknown_error";
    }
    if (error.stack) {
        return String(error.stack);
    }
    if (error.message) {
        return String(error.message);
    }
    return String(error);
}

/**
 * 移除基础地址末尾的斜杠。
 *
 * @param {string} baseURL 中心服务基础地址。
 * @returns {string} 标准化后的基础地址。
 */
function trimBaseURL(baseURL) {
    return String(baseURL || "").replace(/\/+$/, "");
}

function nodeGetJSON(url) {
    return new Promise(function (resolve, reject) {
        var target = new URL(url);
        var client = target.protocol === "https:" ? require("https") : require("http");
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function (response) {
            var chunks = [];
            response.on("data", function (chunk) {
                chunks.push(chunk);
            });
            response.on("end", function () {
                try {
                    resolve(parseJSONResponse(response.statusCode, Buffer.concat(chunks).toString("utf8")));
                } catch (error) {
                    reject(error);
                }
            });
        });
        request.on("error", reject);
        request.end();
    });
}

function nodeDownloadText(url) {
    return new Promise(function (resolve, reject) {
        var target = new URL(url);
        var client = target.protocol === "https:" ? require("https") : require("http");
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function (response) {
            var chunks = [];
            response.on("data", function (chunk) {
                chunks.push(chunk);
            });
            response.on("end", function () {
                if (response.statusCode < 200 || response.statusCode >= 300) {
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
    return new Promise(function (resolve, reject) {
        var target = new URL(url);
        var client = target.protocol === "https:" ? require("https") : require("http");
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "GET"
        }, function (response) {
            var chunks = [];
            response.on("data", function (chunk) {
                chunks.push(chunk);
            });
            response.on("end", function () {
                if (response.statusCode < 200 || response.statusCode >= 300) {
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
    } catch (error) {
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
    } catch (error) {
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
    } catch (error) {
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
    } catch (error) {
        throw new Error("AutoJs6 请求中心服务失败，url=" + url + "，error=" + formatErrorMessage(error));
    }
}

/**
 * 向中心服务注册设备。
 *
 * @param {string} centerBaseURL 中心服务基础地址。
 * @param {Object} payload 注册请求载荷。
 * @returns {RegisterResponse|Promise<RegisterResponse>} 注册响应。
 */
function registerDevice(centerBaseURL, payload) {
    var url = trimBaseURL(centerBaseURL) + "/api/v1/device/register";
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsPostJSON(url, payload);
    }
    return nodePostJSON(url, payload);
}

function nodePostJSON(url, payload) {
    return new Promise(function (resolve, reject) {
        var target = new URL(url);
        var data = JSON.stringify(payload);
        var client = target.protocol === "https:" ? require("https") : require("http");
        var request = client.request({
            hostname: target.hostname,
            port: target.port || (target.protocol === "https:" ? 443 : 80),
            path: target.pathname + target.search,
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Content-Length": Buffer.byteLength(data)
            }
        }, function (response) {
            var chunks = [];
            response.on("data", function (chunk) {
                chunks.push(chunk);
            });
            response.on("end", function () {
                try {
                    resolve(parseJSONResponse(response.statusCode, Buffer.concat(chunks).toString("utf8")));
                } catch (error) {
                    reject(error);
                }
            });
        });

        request.on("error", reject);
        request.write(data);
        request.end();
    });
}

/**
 * 获取脚本清单。
 *
 * @param {string} centerBaseURL 中心服务基础地址。
 * @param {string} scriptName 脚本名称。
 * @param {string} scriptVersion 脚本版本。
 * @returns {Object|Promise<Object>} 脚本清单。
 */
function getScriptManifest(centerBaseURL, scriptName, scriptVersion) {
    var url = trimBaseURL(centerBaseURL) + "/api/v1/script/manifest?script_name=" + encodeURIComponent(scriptName) + "&script_version=" + encodeURIComponent(scriptVersion);
    if (runtime.isAutoJsRuntime() && typeof http !== "undefined") {
        return autoJsGetJSON(url);
    }
    return nodeGetJSON(url);
}

/**
 * 下载脚本文本。
 *
 * @param {string} centerBaseURL 中心服务基础地址。
 * @param {string} scriptName 脚本名称。
 * @param {string} scriptVersion 脚本版本。
 * @returns {string|Promise<string>} 脚本文本。
 */
function downloadScript(centerBaseURL, scriptName, scriptVersion) {
    return downloadScriptFile(centerBaseURL, scriptName, scriptVersion, "index.js");
}

/**
 * 下载指定脚本文件。
 *
 * @param {string} centerBaseURL 中心服务基础地址。
 * @param {string} scriptName 脚本名称。
 * @param {string} scriptVersion 脚本版本。
 * @param {string} relativePath 相对路径。
 * @returns {string|Promise<string>} 脚本文件内容。
 */
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

/**
 * 下载指定脚本文件的原始字节。
 *
 * @param {string} centerBaseURL 中心服务基础地址。
 * @param {string} scriptName 脚本名称。
 * @param {string} scriptVersion 脚本版本。
 * @param {string} relativePath 相对路径。
 * @returns {*|Promise<Buffer>} 原始内容。
 */
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

module.exports = {
    registerDevice: registerDevice,
    getScriptManifest: getScriptManifest,
    downloadScript: downloadScript,
    downloadScriptFile: downloadScriptFile,
    downloadScriptFileBytes: downloadScriptFileBytes
};

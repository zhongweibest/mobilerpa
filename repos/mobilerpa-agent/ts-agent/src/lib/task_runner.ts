import * as runtime from "./runtime";
import * as centerClient from "./center_client";

import type { LoggerLike } from "../types/runtime";
import type { TaskProgress, TaskResult, TaskRunnerContext, TaskSummary } from "../types/protocol";

interface TaskRunnerOptions {
  deviceID?: string;
  agentUUID?: string;
  centerBaseURL?: string;
  logger?: LoggerLike;
  onProgress?: (progress: TaskProgress) => void;
  force?: boolean;
  isCancelled?: () => boolean;
}

interface ResolvedScriptModule {
  versionRoot: string;
  modulePath: string;
  manifestPath: string;
  entryLabel: string;
}

interface DownloadResult {
  downloaded: boolean;
  entryFile: string;
  files?: string[];
}

interface LoadedScriptModule {
  scriptModule: any;
  entryLabel: string;
  downloadResult: DownloadResult;
}

interface ScriptModule {
  run?: (
    context: TaskRunnerContext,
    helpers: {
      runtime: typeof runtime;
      logger: LoggerLike;
      reportProgress: (stepName: string, message: string, status?: string, extra?: Record<string, unknown>) => void;
      isCancelled: () => boolean;
      throwIfCancelled: (message?: string) => void;
    }
  ) => TaskResult;
  __load_error__?: unknown;
}

interface ScriptManifestFile {
  relative_path: string;
}

interface ScriptManifest {
  entry_file?: string;
  files?: ScriptManifestFile[];
}

declare const global: Record<string, unknown> | undefined;

function joinPath(...args: Array<string | null | undefined>): string {
  const parts: string[] = [];

  for (let index = 0; index < args.length; index += 1) {
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

function resolveAgentRoot(): string {
  if (typeof __dirname !== "undefined") {
    return joinPath(__dirname, "..", "..");
  }
  if (typeof files !== "undefined" && typeof files.path === "function") {
    return String(files.path(".")).replace(/\\/g, "/");
  }
  return ".";
}

function buildContext(taskSummary?: Partial<TaskSummary>, options?: TaskRunnerOptions): TaskRunnerContext {
  const summary = taskSummary || {};
  const config = options || {};
  return {
    task_id: String(summary.task_id || ""),
    script_name: String(summary.script_name || ""),
    script_version: String(summary.script_version || ""),
    priority: Number(summary.priority || 0),
    params: (summary.params || {}) as Record<string, unknown>,
    device_id: String(config.deviceID || ""),
    agent_uuid: String(config.agentUUID || ""),
    center_base_url: String(config.centerBaseURL || "")
  };
}

function buildProgressReporter(
  taskSummary?: Partial<TaskSummary>,
  options?: TaskRunnerOptions,
  logger?: LoggerLike
): {
  reportProgress: (stepName: string, message: string, status?: string, extra?: Record<string, unknown>) => void;
  getLastProgress: () => TaskProgress | null;
} {
  const summary = taskSummary || {};
  const config = options || {};
  const report = typeof config.onProgress === "function" ? config.onProgress : null;
  let lastProgress: TaskProgress | null = null;

  if (!report && logger && typeof logger.warn === "function") {
    logger.warn("当前任务未注入 onProgress，脚本内部的关键步骤事件不会发送到中心。task_id=" + String(summary.task_id || ""));
  }

  return {
    reportProgress(stepName: string, message: string, status?: string, extra?: Record<string, unknown>): void {
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
      } catch (error) {
        if (logger && typeof logger.warn === "function") {
          logger.warn("任务进度上报回调执行失败：" + String(error));
        }
      }
    },
    getLastProgress(): TaskProgress | null {
      return lastProgress;
    }
  };
}

function resolveScriptModule(context: Partial<TaskRunnerContext>): ResolvedScriptModule {
  const scriptName = String(context.script_name || "").trim();
  const scriptVersion = String(context.script_version || "").trim();
  const versionDir = scriptVersion || "latest";
  const agentRoot = resolveAgentRoot();
  return {
    versionRoot: joinPath(agentRoot, "scripts", scriptName, versionDir),
    modulePath: joinPath(agentRoot, "scripts", scriptName, versionDir, "index.js"),
    manifestPath: joinPath(agentRoot, "scripts", scriptName, versionDir, "manifest.json"),
    entryLabel: "scripts/" + scriptName + "/" + versionDir + "/index.js"
  };
}

function isSafeRelativePath(value: string): boolean {
  const normalized = String(value || "").replace(/\\/g, "/").trim();
  if (!normalized || normalized.indexOf("..") >= 0 || normalized.charAt(0) === "/") {
    return false;
  }

  const parts = normalized.split("/");
  for (let index = 0; index < parts.length; index += 1) {
    if (!parts[index] || parts[index] === "." || parts[index] === "..") {
      return false;
    }
  }

  return true;
}

function dirnameOfPath(filePath: string): string {
  const normalized = String(filePath || "").replace(/\\/g, "/");
  const index = normalized.lastIndexOf("/");
  if (index < 0) {
    return ".";
  }
  return normalized.slice(0, index) || "/";
}

function normalizeModulePath(filePath: string): string {
  const normalized = String(filePath || "").replace(/\\/g, "/");
  const absolute = normalized.charAt(0) === "/";
  const parts = normalized.split("/");
  const output: string[] = [];

  for (let index = 0; index < parts.length; index += 1) {
    const part = parts[index];
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

function resolveAutoJsModuleFile(baseFilePath: string, requestPath: string): string {
  const baseDir = dirnameOfPath(baseFilePath);
  const normalizedRequest = String(requestPath || "").replace(/\\/g, "/");
  const rawPath = normalizedRequest.indexOf("./") === 0 || normalizedRequest.indexOf("../") === 0
    ? normalizeModulePath(baseDir + "/" + normalizedRequest)
    : normalizedRequest;
  const candidates = [rawPath];

  if (!/\.js$/i.test(rawPath)) {
    candidates.push(rawPath + ".js");
    candidates.push(normalizeModulePath(rawPath + "/index.js"));
  }

  for (let index = 0; index < candidates.length; index += 1) {
    if (runtime.fileExists(candidates[index])) {
      return candidates[index];
    }
  }

  throw new Error("script_module_not_found:" + requestPath + " from " + baseFilePath);
}

function loadAutoJsModule(entryPath: string, globalObject?: Record<string, unknown>): ScriptModule {
  const moduleCache: Record<string, ScriptModule> = {};
  const fallbackExportName = "__mobilerpaTaskScriptExport__";
  const fallbackModuleExportName = "__mobilerpaTaskScriptModuleExports__";

  function executeModule(filePath: string): ScriptModule {
    const normalizedFilePath = normalizeModulePath(filePath);
    if (moduleCache[normalizedFilePath]) {
      return moduleCache[normalizedFilePath];
    }

    const sourceText = runtime.readTextFile(normalizedFilePath);
    if (!sourceText) {
      throw new Error("script_file_not_found:" + normalizedFilePath);
    }

    const moduleRecord = {
      exports: {} as ScriptModule
    };
    moduleCache[normalizedFilePath] = moduleRecord.exports;

    const localRequire = function localRequire(requestPath: string): ScriptModule {
      if (String(requestPath || "").indexOf(".") !== 0) {
        throw new Error("unsupported_script_require:" + requestPath);
      }
      const targetFilePath = resolveAutoJsModuleFile(normalizedFilePath, requestPath);
      return executeModule(targetFilePath);
    };

    const wrapper = new Function(
      "require",
      "module",
      "exports",
      sourceText + "\nreturn module.exports;"
    ) as (
      require: (requestPath: string) => ScriptModule,
      module: { exports: ScriptModule },
      exports: ScriptModule
    ) => ScriptModule;

    const result = wrapper(localRequire, moduleRecord, moduleRecord.exports);
    const exported = result || moduleRecord.exports;

    if (
      globalObject
      && normalizedFilePath === normalizeModulePath(entryPath)
      && (!exported || Object.keys(exported).length === 0)
    ) {
      if (globalObject[fallbackExportName]) {
        moduleCache[normalizedFilePath] = globalObject[fallbackExportName] as ScriptModule;
      } else if (globalObject[fallbackModuleExportName]) {
        moduleCache[normalizedFilePath] = globalObject[fallbackModuleExportName] as ScriptModule;
      }
    } else {
      moduleCache[normalizedFilePath] = exported || {};
    }

    return moduleCache[normalizedFilePath];
  }

  return executeModule(entryPath);
}

function tryRequire(modulePath: string): ScriptModule {
  const fallbackExportName = "__mobilerpaTaskScriptExport__";
  const fallbackModuleExportName = "__mobilerpaTaskScriptModuleExports__";
  const globalObject = typeof globalThis !== "undefined"
    ? (globalThis as Record<string, unknown>)
    : (typeof global !== "undefined" ? global : undefined);

  try {
    if (globalObject) {
      globalObject[fallbackExportName] = null;
      globalObject[fallbackModuleExportName] = null;
    }

    if (runtime.isAutoJsRuntime()) {
      const result = loadAutoJsModule(modulePath, globalObject);

      if (result && Object.keys(result).length > 0) {
        return result;
      }

      if (globalObject && globalObject[fallbackExportName]) {
        return globalObject[fallbackExportName] as ScriptModule;
      }

      if (globalObject && globalObject[fallbackModuleExportName]) {
        return globalObject[fallbackModuleExportName] as ScriptModule;
      }

      return result || {};
    }

    const loaded = require(modulePath) as ScriptModule;
    if (loaded) {
      return loaded;
    }

    if (globalObject && globalObject[fallbackExportName]) {
      return globalObject[fallbackExportName] as ScriptModule;
    }

    return loaded;
  } catch (error) {
    return {
      __load_error__: error
    };
  } finally {
    if (globalObject && globalObject[fallbackExportName]) {
      globalObject[fallbackExportName] = null;
    }
    if (globalObject && globalObject[fallbackModuleExportName]) {
      globalObject[fallbackModuleExportName] = null;
    }
  }
}

function downloadScriptIfNeeded(
  context: TaskRunnerContext,
  logger: LoggerLike,
  resolved: ResolvedScriptModule,
  options?: TaskRunnerOptions
): DownloadResult {
  const syncOptions = options || {};
  const forceDownload = syncOptions.force === true;

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

  const manifestResp = centerClient.getScriptManifest(context.center_base_url, context.script_name, context.script_version) as {
    data?: ScriptManifest;
  } | ScriptManifest;
  const manifest = (manifestResp && "data" in manifestResp && manifestResp.data ? manifestResp.data : manifestResp) as ScriptManifest;
  const entryFile = String((manifest && manifest.entry_file) || "index.js").trim() || "index.js";
  let files = manifest && manifest.files ? manifest.files : [];

  if (!isSafeRelativePath(entryFile)) {
    throw new Error("unsupported_entry_file:" + entryFile);
  }

  if (!files || files.length === 0) {
    files = [{
      relative_path: entryFile
    }];
  }

  for (let index = 0; index < files.length; index += 1) {
    const relativePath = String(files[index].relative_path || "").trim();
    if (!isSafeRelativePath(relativePath)) {
      throw new Error("unsafe_relative_path:" + relativePath);
    }

    const targetPath = joinPath(resolved.versionRoot, relativePath);
    const fileContent = centerClient.downloadScriptFile(
      context.center_base_url,
      context.script_name,
      context.script_version,
      relativePath
    ) as string;
    logger.info("正在下载脚本文件：" + relativePath);
    runtime.writeTextFile(targetPath, fileContent);
  }

  runtime.writeTextFile(resolved.manifestPath, JSON.stringify(manifest, null, 2));
  logger.info("脚本下载完成：" + context.script_name + "@" + context.script_version + " -> " + resolved.entryLabel);

  return {
    downloaded: true,
    entryFile,
    files: files.map(function mapRelativePath(item) {
      return item.relative_path;
    })
  };
}

function loadScriptModule(context: TaskRunnerContext, logger: LoggerLike): LoadedScriptModule {
  const resolved = resolveScriptModule(context);
  let downloadResult: DownloadResult = {
    downloaded: false,
    entryFile: "index.js"
  };

  if (!runtime.fileExists(resolved.modulePath)) {
    downloadResult = downloadScriptIfNeeded(context, logger, resolved, {
      force: false
    });
  }

  const moduleValue = tryRequire(resolved.modulePath);
  return {
    scriptModule: moduleValue,
    entryLabel: resolved.entryLabel,
    downloadResult
  };
}

function ensureScriptVersion(scriptName: string, scriptVersion: string, options?: TaskRunnerOptions): DownloadResult {
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const context = {
    script_name: String(scriptName || "").trim(),
    script_version: String(scriptVersion || "").trim(),
    center_base_url: String(options && options.centerBaseURL ? options.centerBaseURL : "").trim()
  } as TaskRunnerContext;
  const resolved = resolveScriptModule(context);

  return downloadScriptIfNeeded(context, logger, resolved, {
    force: !!(options && options.force)
  });
}

function runTask(taskSummary: TaskSummary, options?: TaskRunnerOptions): TaskResult {
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const context = buildContext(taskSummary, options);
  const progressReporter = buildProgressReporter(taskSummary, options, logger);
  const reportProgress = progressReporter.reportProgress;
  const isCancelled = options && typeof options.isCancelled === "function"
    ? options.isCancelled
    : function neverCancelled(): boolean { return false; };
  const loaded = loadScriptModule(context, logger);
  const scriptModule = loaded.scriptModule;

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

  function throwIfCancelled(message?: string): void {
    if (!isCancelled()) {
      return;
    }
    const error = new Error(String(message || "任务已取消")) as Error & {
      code?: string;
      isCancelled?: boolean;
    };
    error.code = "TASK_CANCELLED";
    error.isCancelled = true;
    throw error;
  }

  const result = scriptModule.run(context, {
    runtime,
    logger,
    reportProgress,
    isCancelled,
    throwIfCancelled
  });

  const lastProgress = progressReporter.getLastProgress();
  const expectedStepName = String((result && result.step_name) || "COMPLETE");
  const expectedStatus = String((result && result.status) || "success");
  const expectedMessage = "任务执行中：" + String((result && result.result_message) || "脚本执行结束");

  if (
    !lastProgress
    || String(lastProgress.step_name || "") !== expectedStepName
    || String(lastProgress.status || "") !== expectedStatus
    || String(lastProgress.message || "") !== expectedMessage
  ) {
    reportProgress(
      expectedStepName,
      expectedMessage,
      expectedStatus,
      {
        result_code: String((result && result.result_code) || "")
      }
    );
  }

  return result;
}

function syncScriptVersion(scriptName: string, scriptVersion: string, options?: TaskRunnerOptions): {
  entry_file: string;
  files: string[];
  entry_label: string;
  version_root: string;
} {
  const logger = options && options.logger ? options.logger : runtime.createLogger();
  const context = {
    script_name: String(scriptName || "").trim(),
    script_version: String(scriptVersion || "").trim(),
    center_base_url: String(options && options.centerBaseURL ? options.centerBaseURL : "").trim()
  } as TaskRunnerContext;
  const resolved = resolveScriptModule(context);
  const downloadResult = downloadScriptIfNeeded(context, logger, resolved, {
    force: !!(options && options.force)
  });

  return {
    entry_file: downloadResult.entryFile,
    files: downloadResult.files || [],
    entry_label: resolved.entryLabel,
    version_root: resolved.versionRoot
  };
}

export {
  runTask,
  syncScriptVersion,
  ensureScriptVersion,
  buildContext,
  resolveScriptModule,
  isSafeRelativePath,
  normalizeModulePath
};

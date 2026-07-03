export interface ScriptContext {
  task_id?: string;
  params?: Record<string, unknown>;
}

export interface ScriptResult {
  status: string;
  result_code: string;
  result_message: string;
  step_name: string;
  extra: Record<string, unknown>;
}

export interface ScriptHelpers {
  logger?: {
    info?: (message: string) => void;
    warn?: (message: string) => void;
    error?: (message: string) => void;
  };
  runtime?: {
    sleepMS?: (milliseconds: number) => void;
  };
  reportProgress?: (
    stepName: string,
    message: string,
    status?: string,
    extra?: Record<string, unknown>
  ) => void;
  isCancelled?: () => boolean;
  throwIfCancelled?: (message?: string) => void;
}

export interface OpenAppConfig {
  scriptName: string;
  scriptVersion: string;
  appName: string;
  packageName: string;
  appStartTimeoutMS: number;
}

export function noopReportProgress(): void {}

export function noopThrowIfCancelled(): void {}

export function formatErrorMessage(error: unknown): string {
  if (!error) {
    return "未知错误";
  }
  if (typeof error === "string") {
    return error;
  }
  if (typeof error === "object" && error && "message" in error) {
    return String((error as { message?: unknown }).message || "未知错误");
  }
  return String(error);
}

export function buildOpenAppExtra(context: ScriptContext, config: OpenAppConfig): Record<string, unknown> {
  return {
    mode: "open_app",
    script_name: config.scriptName,
    script_version: config.scriptVersion,
    entry_file: config.scriptName + "/" + config.scriptVersion + "/index.js",
    app_name: config.appName,
    app_package: config.packageName,
    task_id: String(context && context.task_id ? context.task_id : "")
  };
}

export function ensureNotCancelled(helpers: ScriptHelpers | undefined, message: string): void {
  if (helpers && typeof helpers.throwIfCancelled === "function") {
    helpers.throwIfCancelled(message);
    return;
  }

  if (helpers && typeof helpers.isCancelled === "function" && helpers.isCancelled()) {
    throw new Error(message || "任务已取消");
  }
}

export function sleepSafe(durationMS: number, helpers?: ScriptHelpers): void {
  if (helpers && helpers.runtime && typeof helpers.runtime.sleepMS === "function") {
    helpers.runtime.sleepMS(durationMS);
    return;
  }

  if (typeof sleep === "function") {
    sleep(durationMS);
    return;
  }

  if (typeof java !== "undefined" && java.lang && java.lang.Thread && typeof java.lang.Thread.sleep === "function") {
    java.lang.Thread.sleep(durationMS);
  }
}

export function waitForPackageWithCancel(packageName: string, timeoutMS: number, helpers?: ScriptHelpers, cancelMessage?: string): boolean {
  if (typeof currentPackage !== "function") {
    return true;
  }

  const endTime = Date.now() + timeoutMS;
  while (Date.now() < endTime) {
    ensureNotCancelled(helpers, cancelMessage || "任务已取消");
    if (currentPackage() === packageName) {
      return true;
    }
    sleepSafe(300, helpers);
  }
  return false;
}

export function launchAppByPackageWithCancel(packageName: string, timeoutMS: number, helpers?: ScriptHelpers, cancelMessage?: string): void {
  if (typeof app === "undefined" || !app || typeof app.launchPackage !== "function") {
    throw new Error("当前运行环境不支持 app.launchPackage");
  }

  ensureNotCancelled(helpers, cancelMessage || "任务已取消");
  app.launchPackage(packageName);

  if (!waitForPackageWithCancel(packageName, timeoutMS, helpers, cancelMessage)) {
    throw new Error("等待应用启动超时");
  }
}

import type { DeviceInfo } from "../types/agent";
import type { IntervalHandle, LoggerLike } from "../types/runtime";

interface ExecutionProfile {
  accessibility_status: string;
  foreground_service_status: string;
  battery_optimization_ignored_status: string;
  checked_at: string;
  message: string;
}

function isNodeRuntime(): boolean {
  return typeof process !== "undefined" && !!(process.versions && process.versions.node);
}

function isAutoJsRuntime(): boolean {
  return typeof files !== "undefined" || typeof device !== "undefined" || typeof http !== "undefined";
}

function nodeRequire(moduleName: string): any {
  return require(moduleName);
}

function createLogger(): LoggerLike {
  const write = typeof log === "function"
    ? log
    : function writeByConsole(message: string): void {
        console.log(message);
      };

  return {
    info(message: string): void {
      write("[INFO] " + message);
    },
    warn(message: string): void {
      write("[WARN] " + message);
    },
    error(message: string): void {
      write("[ERROR] " + message);
    }
  };
}

function nowISOString(): string {
  return new Date().toISOString();
}

function randomText(length: number): string {
  const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz";
  let result = "";

  if (isNodeRuntime()) {
    try {
      const crypto = nodeRequire("crypto");
      const bytes = crypto.randomBytes(length);
      for (let index = 0; index < length; index += 1) {
        result += alphabet[bytes[index] % alphabet.length];
      }
      return result;
    } catch (_error) {
      // 如果 Node 加密模块不可用，继续使用通用随机逻辑。
    }
  }

  for (let index = 0; index < length; index += 1) {
    result += alphabet[Math.floor(Math.random() * alphabet.length)];
  }
  return result;
}

function hashText(text: string): string {
  const input = String(text || "");

  if (isNodeRuntime()) {
    try {
      const crypto = nodeRequire("crypto");
      return crypto.createHash("sha1").update(input, "utf8").digest("hex");
    } catch (_error) {
      // 继续使用通用哈希逻辑。
    }
  }

  if (isAutoJsRuntime() && typeof java !== "undefined") {
    try {
      const MessageDigest = java.security.MessageDigest;
      const StringClass = java.lang.String;
      const digest = MessageDigest.getInstance("SHA-1");
      const bytes = new StringClass(input).getBytes("UTF-8");
      const hashBytes = digest.digest(bytes);
      let result = "";
      for (let index = 0; index < hashBytes.length; index += 1) {
        let value = hashBytes[index];
        if (value < 0) {
          value += 256;
        }
        let piece = value.toString(16);
        if (piece.length < 2) {
          piece = "0" + piece;
        }
        result += piece;
      }
      return result;
    } catch (_error) {
      // 继续使用通用哈希逻辑。
    }
  }

  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash += (hash << 1) + (hash << 4) + (hash << 7) + (hash << 8) + (hash << 24);
  }

  let normalized = (hash >>> 0).toString(16);
  while (normalized.length < 8) {
    normalized = "0" + normalized;
  }
  return normalized;
}

function buildStableFingerprint(deviceInfo: DeviceInfo): string {
  const info = deviceInfo || {
    device_name: "",
    brand: "",
    model: "",
    android_id: ""
  };

  return "android_id=" + String(info.android_id || "");
}

function createAgentUUID(): string {
  return "agent_" + Date.now().toString(36) + "_" + randomText(8);
}

function createStableAgentUUID(deviceInfo: DeviceInfo): string {
  const fingerprint = buildStableFingerprint(deviceInfo);
  return "agent_" + hashText(fingerprint).slice(0, 16);
}

function safeString(getter: () => unknown, fallback: string): string {
  try {
    const value = getter();
    if (value === null || value === undefined) {
      return fallback;
    }
    return String(value);
  } catch (_error) {
    return fallback;
  }
}

function collectAutoJsDeviceInfo(): DeviceInfo {
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
    }, "")
  };
}

function collectNodeDeviceInfo(): DeviceInfo {
  const os = nodeRequire("os");
  return {
    device_name: os.hostname() || "Node Agent",
    brand: "node",
    model: os.platform() + "-" + os.arch(),
    android_id: ""
  };
}

function collectDeviceInfo(overrides?: Partial<DeviceInfo>): DeviceInfo {
  const detected = isAutoJsRuntime() && typeof device !== "undefined"
    ? collectAutoJsDeviceInfo()
    : collectNodeDeviceInfo();
  const custom = overrides || {};

  return {
    device_name: custom.device_name || detected.device_name,
    brand: custom.brand || detected.brand,
    model: custom.model || detected.model,
    android_id: custom.android_id || detected.android_id
  };
}

function fileExists(filePath: string): boolean {
  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    return fs.existsSync(filePath);
  }
  return typeof files !== "undefined" && files.exists(filePath);
}

function readTextFile(filePath: string): string {
  if (!fileExists(filePath)) {
    return "";
  }

  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    return fs.readFileSync(filePath, "utf8");
  }

  if (typeof files !== "undefined" && typeof files.read === "function") {
    return String(files.read(filePath) || "");
  }

  return "";
}

function ensureDir(dirPath: string): void {
  if (!dirPath) {
    return;
  }

  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    fs.mkdirSync(dirPath, { recursive: true });
    return;
  }

  const normalizedPath = String(dirPath).replace(/\\/g, "/");

  if (typeof files !== "undefined" && typeof files.ensureDir === "function") {
    files.ensureDir(normalizedPath);
  }

  if (typeof java !== "undefined" && java.io && java.io.File) {
    const directory = new java.io.File(normalizedPath);
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

function resolveAbsolutePath(filePath: string): string {
  const input = String(filePath || "");

  if (isNodeRuntime()) {
    const path = nodeRequire("path");
    return path.resolve(input);
  }

  if (typeof files !== "undefined" && typeof files.path === "function") {
    return String(files.path(input) || input).replace(/\\/g, "/");
  }

  return input.replace(/\\/g, "/");
}

function writeTextFile(filePath: string, content: string): void {
  const absolutePath = resolveAbsolutePath(filePath);

  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    const path = nodeRequire("path");
    ensureDir(path.dirname(absolutePath));
    fs.writeFileSync(absolutePath, String(content), "utf8");
    return;
  }

  if (typeof files !== "undefined") {
    ensureDir(String(absolutePath).replace(/[\\/][^\\/]+$/, ""));
    files.write(absolutePath, String(content));
  }
}

function writeBinaryFile(filePath: string, content: unknown): void {
  const absolutePath = resolveAbsolutePath(filePath);

  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    const path = nodeRequire("path");
    ensureDir(path.dirname(absolutePath));
    fs.writeFileSync(absolutePath, content as Parameters<typeof fs.writeFileSync>[1]);
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
    let output: any = null;
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

function removeFileIfExists(filePath: string): void {
  if (!fileExists(filePath)) {
    return;
  }

  if (isNodeRuntime()) {
    const fs = nodeRequire("fs");
    fs.unlinkSync(filePath);
    return;
  }

  if (typeof files !== "undefined") {
    files.remove(filePath);
  }
}

function startInterval(callback: () => void, intervalMS: number): IntervalHandle {
  if (isNodeRuntime()) {
    const timer = setInterval(callback, intervalMS);
    return {
      cancel(): void {
        clearInterval(timer);
      }
    };
  }

  const thread = threads.start(function runIntervalLoop() {
    while (true) {
      try {
        callback();
        sleep(intervalMS);
      } catch (_error) {
        break;
      }
    }
  });

  return {
    cancel(): void {
      if (thread && thread.isAlive()) {
        thread.interrupt();
      }
    }
  };
}

function runAsync(callback: () => void): IntervalHandle | null {
  if (isNodeRuntime()) {
    const timer = setTimeout(function runTask() {
      callback();
    }, 0);
    return {
      cancel(): void {
        clearTimeout(timer);
      }
    };
  }

  if (typeof threads !== "undefined" && typeof threads.start === "function") {
    const thread = threads.start(function runInThread() {
      callback();
    });
    return {
      cancel(): void {
        if (thread && thread.isAlive()) {
          thread.interrupt();
        }
      }
    };
  }

  callback();
  return null;
}

function sleepMS(milliseconds: number): void {
  const duration = Number(milliseconds || 0);
  if (duration <= 0) {
    return;
  }

  if (isNodeRuntime()) {
    const end = Date.now() + duration;
    while (Date.now() < end) {
      // Node.js 验证环境仅用于最小闭环，这里允许简单阻塞实现。
    }
    return;
  }

  if (typeof sleep === "function") {
    sleep(duration);
  }
}

function getAndroidContext(): any | null {
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

function normalizeExecutionStatus(enabled: boolean | null): string {
  if (enabled === true) {
    return "enabled";
  }
  if (enabled === false) {
    return "disabled";
  }
  return "unknown";
}

function checkAccessibilityEnabled(): boolean | null {
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

function checkForegroundServiceEnabled(): boolean | null {
  const ctx = getAndroidContext();
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
    const NotificationManagerCompat = androidx.core.app.NotificationManagerCompat;
    if (NotificationManagerCompat && typeof NotificationManagerCompat.from === "function") {
      return !!NotificationManagerCompat.from(ctx).areNotificationsEnabled();
    }
  } catch (_error) {
    return null;
  }

  return null;
}

function checkBatteryOptimizationIgnored(): boolean | null {
  const ctx = getAndroidContext();
  if (!ctx || typeof android === "undefined") {
    return null;
  }

  try {
    const powerServiceName = android.content.Context.POWER_SERVICE;
    const powerManager = ctx.getSystemService(powerServiceName);
    if (!powerManager || typeof powerManager.isIgnoringBatteryOptimizations !== "function") {
      return null;
    }
    return !!powerManager.isIgnoringBatteryOptimizations(String(ctx.getPackageName()));
  } catch (_error) {
    return null;
  }
}

function collectExecutionProfile(): ExecutionProfile {
  const accessibilityEnabled = checkAccessibilityEnabled();
  const foregroundServiceEnabled = checkForegroundServiceEnabled();
  const batteryOptimizationIgnored = checkBatteryOptimizationIgnored();
  const messages: string[] = [];

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

function exitProcess(code: number): void {
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

export {
  isNodeRuntime,
  isAutoJsRuntime,
  createLogger,
  nowISOString,
  createAgentUUID,
  createStableAgentUUID,
  collectDeviceInfo,
  fileExists,
  readTextFile,
  resolveAbsolutePath,
  ensureDir,
  writeTextFile,
  writeBinaryFile,
  removeFileIfExists,
  startInterval,
  runAsync,
  sleepMS,
  getAndroidContext,
  collectExecutionProfile,
  exitProcess
};

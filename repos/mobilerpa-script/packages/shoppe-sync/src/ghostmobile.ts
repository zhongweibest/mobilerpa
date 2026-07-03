import {
  formatErrorMessage,
  type ScriptHelpers
} from "@mobilerpa-script/shared";
import {
  GHOSTMOBILE_PACKAGE_NAME
} from "./config";
import {
  launchAppByPackage
} from "./common";

export interface GhostmobileResult {
  started: boolean;
  code: string;
  message: string;
  state: string;
}

export function launchAndStartGame(helpers?: ScriptHelpers): GhostmobileResult {
  ensureAccessibilityEnabled();
  launchAppByPackage(GHOSTMOBILE_PACKAGE_NAME, helpers);
  toastSafe("启动 极魔游戏助手");

  return {
    started: true,
    code: "OK",
    message: "尝试启动应用完成",
    state: "already_locked"
  };
}

function ensureAccessibilityEnabled(): void {
  if (typeof auto === "undefined" || !auto || !auto.service) {
    toastSafe("请先开启无障碍服务");
    throw new Error("无障碍服务未开启");
  }
}

function toastSafe(message: string): void {
  if (typeof toast === "function") {
    toast(message);
  }
}

export function buildFailure(code: string, message: string, state: string): GhostmobileResult {
  return {
    started: false,
    code,
    message: formatErrorMessage(message),
    state
  };
}

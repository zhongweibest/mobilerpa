import {
  launchAppByPackageWithCancel,
  type ScriptHelpers
} from "@mobilerpa-script/shared";
import {
  APP_START_TIMEOUT_MS
} from "./config";

export function launchAppByPackage(packageName: string, helpers?: ScriptHelpers): void {
  launchAppByPackageWithCancel(packageName, APP_START_TIMEOUT_MS, helpers, "任务已取消，停止等待极魔游戏助手启动");
}

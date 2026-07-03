import {
  type ScriptHelpers
} from "@mobilerpa-script/shared";
import {
  GHOSTMOBILE_PACKAGE_NAME
} from "./config";
import {
  launchAndStartGame,
  type GhostmobileResult
} from "./ghostmobile";

export function startApp(helpers?: ScriptHelpers): GhostmobileResult {
  if (helpers && typeof helpers.reportProgress === "function") {
    helpers.reportProgress("CALL_GHOSTMOBILE", "任务执行中：准备调用极魔游戏助手启动链路", "running", {
      app_name: "极魔游戏助手",
      app_package: GHOSTMOBILE_PACKAGE_NAME
    });
  }

  return launchAndStartGame(helpers);
}

# MobileRPA Toolkit

`mobilerpa-toolkit` 是独立于中心服务和 Agent 仓库的跨平台辅助工具。

当前提供的核心子命令有：

1. `push-center`
   供中心服务或自动化流程非交互调用，用于把 Agent 推送到指定设备。
2. `push-manual`
   供本地调试时手工选择设备或批量推送。
3. `start-agent`
   启动目标设备上的 `agent.js`。
4. `stop-agent`
   向目标设备写入停止信号，由 `agent.js` 自行优雅退出，不再强制关闭整个 AutoJs6。

## 1. 设计原则

1. 推送、启动、停止逻辑统一收敛到独立工具中，避免中心服务内嵌平台相关脚本。
2. 生产路径优先保证 Windows / Linux 都可构建、可调用。
3. “停止 Agent”只停止 `agent.js`，不直接关闭整个 AutoJs6 应用。
4. 生产环境默认只维护 `mobilerpa-toolkit`，不把 `.bat`、`.ps1` 作为正式依赖。
5. Agent 推送与脚本同步是两条独立链路，业务脚本版本下载由中心服务脚本同步能力负责。

## 2. 构建方式

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-toolkit
$env:GOCACHE="D:\dev\code\mobilerpa\.tmp\gocache"
$env:GOMODCACHE="D:\dev\code\mobilerpa\.tmp\gomodcache"
go build -o D:\dev\code\mobilerpa\repos\mobilerpa-toolkit\build\mobilerpa-toolkit.exe .\cmd\mobilerpa-toolkit
```

## 3. 命令总览

```text
mobilerpa-toolkit push-center --device <adb-device> --center-base-url <url> [--agent-root <path>] [--adb-path <path>] [--reset-config] [--no-run]
mobilerpa-toolkit push-manual [--device <adb-device> | --all] --center-base-url <url> [--agent-root <path>] [--adb-path <path>] [--reset-config] [--no-run]
mobilerpa-toolkit start-agent --device <adb-device> [--adb-path <path>] [--autojs-component <component>] [--remote-root <path>]
mobilerpa-toolkit stop-agent --device <adb-device> [--adb-path <path>] [--remote-root <path>]
```

## 4. `push-center`

适用场景：

1. 中心服务通过接口触发下发 Agent。
2. 后续 Linux 部署环境中的非交互式推送。

最小示例：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe push-center `
  --device 192.168.0.120:37123 `
  --center-base-url http://192.168.0.155:28080 `
  --agent-root D:\dev\code\mobilerpa\repos\mobilerpa-agent\ts-agent\release
```

常用参数：

- `--device`：ADB 序列号或无线调试地址。
- `--center-base-url`：写入设备 `bootstrap.json` 的中心服务地址。
- `--agent-root`：Agent 根目录，内部至少需包含 `agent.js`。工具会递归推送根目录下实际存在的文件。
- `--adb-path`：可选，默认为 `adb`。
- `--reset-config`：显式重置设备端 `config.json`。
- `--no-run`：只推送，不自动启动 Agent。

补充说明：

1. 推送完成后如果需要自动启动，工具会先清理旧的 `stop.signal`，再拉起 `agent.js`。
2. 默认不覆盖设备已有 `runtime/config.json`。
3. 每次重新推送都会刷新设备端 `runtime/bootstrap.json`，用于同步最新的 `center_base_url` 和连接参数。
4. Agent 启动时会优先保留已有 `agent_uuid`、`device_id`，同时吸收 `bootstrap.json` 中的新中心地址。
5. 工具会确保设备端存在 `/sdcard/脚本/agent/scripts/` 目录，供后续脚本同步使用，但不会在 Agent 下发阶段直接推送业务脚本。

## 5. `push-manual`

适用场景：

1. 本地开发调试。
2. 手工选择设备推送。
3. 对当前全部已连接且已授权的设备执行 `--all` 批量推送。

最小示例：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe push-manual `
  --center-base-url http://192.168.0.155:28080 `
  --agent-root D:\dev\code\mobilerpa\repos\mobilerpa-agent\ts-agent\release
```

如果不传 `--device` 和 `--all`，会进入交互式设备选择。

## 6. `start-agent`

作用：

1. 清理设备端旧的停止信号。
2. 调用 AutoJs6 打开 `/sdcard/脚本/agent/agent.js`。

最小示例：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe start-agent `
  --device 192.168.0.120:37123
```

## 7. `stop-agent`

作用：

1. 在设备端写入 `/sdcard/脚本/agent/runtime/stop.signal`。
2. 由运行中的 `agent.js` 检测到该文件后主动关闭 WebSocket、停止心跳、停止重连，并退出自身脚本。
3. 不再对 `org.autojs.autojs6` 执行 `force-stop`。

最小示例：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe stop-agent `
  --device 192.168.0.120:37123
```

## 8. 与中心服务的关系

推荐中心服务统一调用：

1. 下发：`mobilerpa-toolkit push-center`
2. 启动：`mobilerpa-toolkit start-agent`
3. 停止：`mobilerpa-toolkit stop-agent`

这样中心服务只依赖 `toolkit`，不再依赖生产路径中的 `.bat` 或 `.ps1`。

# 通过 toolkit 推送 Agent 验收补充

## 1. 适用范围

本文档用于补充手机端手工验收中与 Agent 推送相关的正式操作路径。

从当前版本开始：

1. 生产环境默认只维护 `mobilerpa-toolkit`
2. 中心服务网页下发默认只调用 `mobilerpa-toolkit`
3. `.bat`、`.ps1` 仅作为本地调试兜底，不作为正式验收主路径

## 2. 构建 toolkit

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-toolkit
$env:GOCACHE="D:\dev\code\mobilerpa\.tmp\gocache"
$env:GOMODCACHE="D:\dev\code\mobilerpa\.tmp\gomodcache"
go build -o D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe .\cmd\mobilerpa-toolkit
```

## 3. 单设备推送并启动

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe push-center `
  --device 你的设备序列号 `
  --center-base-url http://192.168.1.23:18080 `
  --agent-root D:\dev\code\mobilerpa\repos\mobilerpa-agent\agent
```

## 4. 单设备推送但不自动启动

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe push-center `
  --device 你的设备序列号 `
  --center-base-url http://192.168.1.23:18080 `
  --agent-root D:\dev\code\mobilerpa\repos\mobilerpa-agent\agent `
  --no-run
```

## 5. 首次推送重置身份配置

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe push-center `
  --device 你的设备序列号 `
  --center-base-url http://192.168.1.23:18080 `
  --agent-root D:\dev\code\mobilerpa\repos\mobilerpa-agent\agent `
  --reset-config
```

## 6. 推送后文件检查

工具会同步以下内容到手机：

1. `/sdcard/脚本/agent/agent.js`
2. `/sdcard/脚本/agent/lib/`
3. `/sdcard/脚本/agent/scripts/`
4. `/sdcard/脚本/agent/runtime/bootstrap.json`

其中任务脚本入口当前至少应包含：

```text
/sdcard/脚本/agent/scripts/shoppe_sync/v0.1.0/index.js
```

可使用 ADB 检查：

```powershell
adb -s 你的设备序列号 shell ls /sdcard/脚本/agent/scripts/shoppe_sync/v0.1.0
```

预期结果至少包含：

```text
index.js
```

## 7. 当前验收关注点

重新推送后，真机再次执行 `shoppe_sync@v0.1.0` 时应重点确认：

1. AutoJs6 不再出现 `Module "path" not found`
2. 日志出现 `开始执行脚本入口：shoppe_sync@v0.1.0 -> scripts/shoppe_sync/v0.1.0/index.js`
3. 中心侧任务最终进入 `success` 或 `failed`

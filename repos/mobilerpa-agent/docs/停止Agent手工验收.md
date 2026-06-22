# 停止 Agent 手工验收

## 1. 目标

验证以下行为：

1. “停止 Agent”只结束 `agent.js`。
2. AutoJs6 不会被整个强制退出。
3. Agent 停止前会优雅关闭 WebSocket、心跳和重连。

## 2. 前置条件

1. 真机已推送最新 Agent 代码。
2. 真机上的 Agent 已运行，并能正常发送心跳。
3. 中心服务已启动，设备当前在中心侧显示为在线。
4. 已构建 `mobilerpa-toolkit`，或可从网页“设备发现”页触发停止动作。

## 3. 构建 toolkit

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-toolkit
$env:GOCACHE="D:\dev\code\mobilerpa\.tmp\gocache"
$env:GOMODCACHE="D:\dev\code\mobilerpa\.tmp\gomodcache"
go build -o D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe .\cmd\mobilerpa-toolkit
```

## 4. 命令行停止验收

执行：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe stop-agent `
  --device 192.168.0.120:37123
```

请把 `192.168.0.120:37123` 替换成实际设备的 ADB 地址。

## 5. 网页停止验收

如果你通过网页操作，页面应调用：

```http
POST /api/v1/discovery/agent-actions
Content-Type: application/json
```

请求体示例：

```json
{
  "adb_endpoint": "192.168.0.120:37123",
  "action": "stop"
}
```

## 6. 预期结果

1. AutoJs6 日志中出现“检测到 stop.signal，Agent 即将优雅退出”。
2. AutoJs6 日志中出现“Agent 已停止心跳与重连，准备退出当前脚本”。
3. 停止后不再继续出现新的心跳确认日志。
4. AutoJs6 应用界面仍保持打开，不应直接退出到桌面。
5. 中心服务中该设备在后续会因心跳停止而转为离线。

## 7. 再次启动验收

执行：

```powershell
D:\dev\code\mobilerpa\.tmp\mobilerpa-toolkit.exe start-agent `
  --device 192.168.0.120:37123
```

预期结果：

1. 启动前会自动清理旧的 `stop.signal`。
2. `agent.js` 可以再次正常启动。
3. 不会出现“刚启动就立即退出”的现象。

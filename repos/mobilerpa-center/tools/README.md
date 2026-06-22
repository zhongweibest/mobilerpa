# 工具

## `api-smoke-test.ps1`
这个脚本用于用真实启动服务和真实 HTTP 接口调用的方式做最小验收。

默认验收内容：

1. 启动 `mobilerpa-center`
2. 轮询 `/healthz`
3. 调用 `POST /api/v1/device/register`
4. 调用 `GET /api/v1/devices`
5. 输出 JSON 结果

执行方式：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-center
powershell -ExecutionPolicy Bypass -File .\tools\api-smoke-test.ps1
```

如果需要自定义端口或数据库路径：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\api-smoke-test.ps1 `
  -ListenAddr "127.0.0.1:18081" `
  -DBPath "D:\dev\code\mobilerpa\repos\mobilerpa-center\server\data\smoke.db" `
  -AgentUUID "agent-test-002"
```

默认数据库文件会落在：

```text
D:\dev\code\mobilerpa\.tmp\mobilerpa-center-smoke.db
```

这样可以避免把临时验收数据写进仓库目录。

## `websocket-smoke-test.ps1`
这个脚本用于用真实启动服务和真实 WebSocket 连接的方式验收 `hello/heartbeat` 链路。

默认验收内容：

1. 启动 `mobilerpa-center`
2. 调用 `POST /api/v1/device/register` 注册测试设备
3. 连接 `/ws`
4. 发送 `hello` 并检查 `ack.status = ok`
5. 发送 `heartbeat` 并检查 `ack.status = ok`
6. 调用 `GET /api/v1/devices` 确认设备状态变为 `online`
7. 关闭 WebSocket 后确认设备状态变为 `offline`

执行方式：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-center
powershell -ExecutionPolicy Bypass -File .\tools\websocket-smoke-test.ps1
```

如果需要自定义端口或数据库路径：

```powershell
powershell -ExecutionPolicy Bypass -File .\tools\websocket-smoke-test.ps1 `
  -ListenAddr "127.0.0.1:18083" `
  -DBPath "D:\dev\code\mobilerpa\.tmp\mobilerpa-center-ws-smoke.db" `
  -AgentUUID "agent-ws-test-002"
```

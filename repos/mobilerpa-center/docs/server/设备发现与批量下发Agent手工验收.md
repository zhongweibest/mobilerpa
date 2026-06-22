# 设备发现与批量下发Agent手工验收

## 1. 目标

本文档用于验收 `C3-1B` 相关的服务端能力：

1. 中心服务可以发现局域网内已开启无线调试的设备。
2. 中心服务可以通过接口批量下发 Agent。
3. 每台设备都会返回独立的下发结果，便于人工排查。

## 2. 前置条件

1. 中心服务所在机器已安装 `adb`，并且命令行可以直接执行 `adb devices`。
2. 手机已完成 Android 无线调试配对。
3. 手机与中心服务机器处于同一局域网。
4. [agent.js](/D:/dev/code/mobilerpa/repos/mobilerpa-agent/agent/agent.js) 与 `lib/` 目录已存在。
5. 已构建 `mobilerpa-toolkit` 可执行文件。

## 3. 批量下发 Agent

### 3.1 Apifox

请求方法：

```text
POST
```

请求地址：

```text
http://127.0.0.1:18080/api/v1/discovery/agent-deployments
```

请求头：

```text
Content-Type: application/json
```

请求体：

```json
{
  "adb_endpoints": [
    "192.168.0.120:37123",
    "192.168.0.121:38741"
  ],
  "center_base_url": "http://192.168.0.155:18080",
  "reset_config": false,
  "run_agent": true
}
```

字段说明：

1. `adb_endpoints` 替换成发现接口返回的真实设备地址列表。
2. `center_base_url` 替换成真机可访问到的中心服务地址。
3. `reset_config=true` 表示重置手机端已有配置；如果希望保留已有 `agent_uuid` 和 `device_id`，保持 `false`。
4. `run_agent=false` 表示只推送不自动启动，便于先去手机上检查运行目录。
5. Agent 下发阶段不要求提供 `script_name` 和 `script_version`，脚本同步应走独立脚本下发接口。

预期响应示例：

```json
{
  "status": "ok",
  "data": [
    {
      "adb_endpoint": "192.168.0.120:37123",
      "connected": true,
      "pushed": true,
      "started": true,
      "status": "ok",
      "message": "agent_deployed_and_started"
    }
  ]
}
```

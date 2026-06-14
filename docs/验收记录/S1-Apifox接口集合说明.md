# S1 Apifox 接口集合说明

## 1. 目的

本文档用于说明 `S1` 阶段在 Apifox 中应整理的最小接口集合，方便后续人工验收、问题复测和新成员快速联调。

当前集合覆盖范围：

1. 中心服务健康检查
2. 设备注册
3. 设备列表查询
4. 单设备详情查询
5. WebSocket `hello/heartbeat`
6. 心跳超时自动离线验证说明

## 2. 环境约定

建议在 Apifox 中建立以下环境变量：

```text
baseUrl=http://127.0.0.1:28080
wsUrl=ws://127.0.0.1:28080/ws
deviceId=dev_000001
```

说明：

1. 如果你的中心服务不是运行在 `28080`，把 `baseUrl` 和 `wsUrl` 改成当前实际地址即可。
2. `deviceId` 用设备注册接口返回的真实值覆盖。

## 3. 建议集合结构

```text
S1-中心接入主链路
  01-健康检查
  02-设备注册
  03-设备列表
  04-单设备详情
  05-WebSocket-hello
  06-WebSocket-heartbeat
  07-心跳超时自动离线说明
```

## 4. 接口清单

### 4.1 健康检查

- 方法：`GET`
- 地址：`{{baseUrl}}/healthz`

预期结果：

```json
{
  "status": "ok",
  "service": "mobilerpa-center"
}
```

### 4.2 设备注册

- 方法：`POST`
- 地址：`{{baseUrl}}/api/v1/device/register`
- Header：`Content-Type: application/json`

请求体：

```json
{
  "agent_uuid": "agent-apifox-001",
  "device_name": "Apifox Device",
  "brand": "Google",
  "model": "Pixel 8",
  "android_id": "android-apifox-001",
  "adb_serial": "adb-apifox-001"
}
```

预期结果：

1. 返回 `status = ok`
2. `data.device_id` 不为空
3. `data.bind_status = pending`

### 4.3 设备列表

- 方法：`GET`
- 地址：`{{baseUrl}}/api/v1/devices`

预期结果：

1. 返回 `status = ok`
2. 列表中可看到刚注册的设备

### 4.4 单设备详情

- 方法：`GET`
- 地址：`{{baseUrl}}/api/v1/devices/{{deviceId}}`

预期结果：

1. 返回 `status = ok`
2. `data.device_id = {{deviceId}}`
3. 不存在的设备应返回 `404 device_not_found`

### 4.5 WebSocket `hello`

- 类型：`WebSocket`
- 地址：`{{wsUrl}}`

发送消息：

```json
{
  "type": "hello",
  "request_id": "hello-apifox-001",
  "device_id": "{{deviceId}}",
  "timestamp": 1780935112,
  "payload": {
    "agent_uuid": "agent-apifox-001"
  }
}
```

预期结果：

1. 收到 `ack`
2. `payload.message_type = hello`
3. `payload.status = ok`

### 4.6 WebSocket `heartbeat`

在同一个 WebSocket 连接中发送：

```json
{
  "type": "heartbeat",
  "request_id": "heartbeat-apifox-001",
  "device_id": "{{deviceId}}",
  "timestamp": 1780935142,
  "payload": {}
}
```

预期结果：

1. 收到 `ack`
2. `payload.message_type = heartbeat`
3. `payload.status = ok`
4. 再调用设备列表或单设备详情时，可看到 `status = online`

### 4.7 心跳超时自动离线说明

Apifox 中不强制建立独立接口，但应在集合说明中保留以下步骤：

1. 建立 WebSocket 连接
2. 发送一次 `hello`
3. 发送一次 `heartbeat`
4. 保持连接标签页打开，但不再继续发第二次 `heartbeat`
5. 超过中心服务当前 `offline timeout` 后，再调用设备详情接口

预期结果：

1. 设备状态自动变为 `offline`
2. 不依赖先收到明确的 WebSocket 断开事件

## 5. 集合维护规则

1. `S1` 阶段相关人工验收以本说明和模块手工验收文档共同为准。
2. 后续如果接口地址、示例字段或 WebSocket 说明变化，必须同步更新本说明。
3. 进入 `S2` 后，任务接口和任务消息不直接追加到本文件，而是新建 `S2` 对应的集合说明。

# Agent TS 迁移完成定义与验收清单

本文档用于定义 `mobilerpa-agent` 何时可以认定为“TypeScript 迁移完成”，以及后续真机验收时需要逐项确认什么。

## 1. 当前迁移结论

截至 2026-07-03，`mobilerpa-agent` 已完成以下迁移动作：

- 已完成 JS 与 TS 目录隔离：
  - [js-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/js-agent)
  - [ts-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent)
- 已明确 TS 版为正式维护主线：
  - [ts-agent/release/agent.js](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release/agent.js)
- 已具备完整核心链路：
  - 启动
  - 注册
  - 心跳
  - 脚本同步与下载
  - 单任务执行
  - 工作流执行
  - 工作流停止
  - 运行锁
  - stop.signal 优雅停止
- 工程命令已可用：
  - `npm run check`
  - `npm run test`
  - `npm run build`
  - `npm run release:agent`

## 2. 什么叫“迁移完成”

只有同时满足以下 4 个条件，才能认定 Agent TS 迁移完成：

1. 代码主线完成
   TS 版本已经覆盖全部正式运行能力，且不再依赖旧 `js-agent` 才能上线。

2. 构建发布完成
   正式发布入口稳定来自 [ts-agent/release](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release)。

3. 真机验收完成
   按本文档第 4 节逐项验收通过。

4. 回滚路径明确
   如 TS 版异常，可明确回退到 [js-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/js-agent)。

## 3. 当前还未完全闭环的事项

当前仍属于“代码迁移基本完成，等待最终验收收口”状态，还差以下几项：

1. 需要按本清单完成一轮正式真机验收。
2. 自动化测试刚建立骨架，覆盖率还不高，需要后续逐步补强。
3. `js-agent` 当前仍作为回滚基线保留，后续要决定长期保留还是只保留历史版本。

## 4. 真机验收清单

以下为建议按顺序执行的最终验收项。

### 4.1 启动与基础身份

1. 推送源目录明确指向：
   [ts-agent/release](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release)
2. 手机端成功启动 `agent.js`
3. 首次启动后能生成运行时配置
4. 同一台手机重复安装 AutoJs6 后，`agent_uuid` 仍保持稳定
5. 重复启动同一 Agent，不会出现多实例并发运行

验收重点：

- `runtime/config.json` 正常生成
- `runtime/agent.lock.json` 正常工作
- `agent_uuid` 基于 `ANDROID_ID` 生成且稳定

### 4.2 注册与心跳

1. 能成功向中心服务注册
2. 注册后 `device_id` 正确回写
3. WebSocket `hello` 正常
4. 心跳正常持续发送
5. 重启 Agent 后仍能继续使用已有身份配置

验收重点：

- 注册接口响应正常
- `device_id` 不丢失
- WebSocket 连接后能持续收到心跳确认

### 4.3 单任务执行

1. 下发单脚本任务
2. Agent 能自动检查本地脚本版本
3. 如本地缺失脚本，能从中心拉取 `manifest.json` 和脚本文件
4. 脚本执行成功后，能上报进度与最终结果
5. 重复执行同一脚本版本时，本地已有版本可直接复用

验收重点：

- `scripts/<script_name>/<script_version>/` 目录正确生成
- `manifest.json` 下载正常
- `index.js` 正常执行
- `task_progress` / `task_result` 正常上报

### 4.4 工作流执行

1. 下发工作流会话
2. Agent 能按 `script_manifest` 预先同步所需脚本
3. 工作流节点能顺序执行
4. 节点事件能持续上报
5. 工作流最终结果能正确回传

验收重点：

- `workflow_session_ack`
- `workflow_session_event`
- `workflow_session_result`

### 4.5 工作流停止

1. 工作流执行过程中下发停止命令
2. Agent 能感知停止请求
3. 脚本能尽快退出
4. 最终结果统一收敛为 `stopped`
5. 不允许重复发送最终结果

验收重点：

- 停止后不再继续执行后续节点
- 日志中能看到停止原因
- 最终只保留一条 `workflow_session_result`

### 4.6 Agent 主动停止

1. 通过 `stop.signal` 停止 Agent
2. Agent 能优雅关闭 WebSocket
3. 心跳停止
4. 运行锁释放
5. 再次启动可恢复正常

## 5. 上线通过标准

当以下条件全部满足时，可以正式认为 TS Agent 迁移完成：

1. `npm run check` 通过
2. `npm run test` 通过
3. `npm run build` 通过
4. 真机验收清单全部通过
5. 推送、下发、执行、停止、重启都正常

## 6. 当前正式发布口径

当前正式发布口径如下：

- 正式发布目录：
  [ts-agent/release](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release)
- 正式入口文件：
  [ts-agent/release/agent.js](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release/agent.js)
- JS 回滚基线：
  [js-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/js-agent)

## 7. 建议你接下来怎么做

你下一步可以直接按本文档第 4 节开始做真机验收。

建议顺序：

1. 先验收启动、注册、心跳
2. 再验收单任务
3. 再验收工作流
4. 最后验收工作流停止与 Agent 停止

这样可以最快定位剩余问题，不会把多个变量混在一起。

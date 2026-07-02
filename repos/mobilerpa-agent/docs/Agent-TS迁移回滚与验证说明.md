# Agent TS 迁移回滚与验证说明

## 1. 当前目录口径

当前仓库中的 Agent 已拆成两个独立目录：

- 正式维护版本：`ts-agent`
- 回滚基线版本：`js-agent`

## 2. 正式发布入口

正式发布目录：

```text
D:\dev\code\mobilerpa\repos\mobilerpa-agent\ts-agent\release
```

手机端入口文件：

```text
D:\dev\code\mobilerpa\repos\mobilerpa-agent\ts-agent\release\agent.js
```

## 3. 回滚入口

回滚目录：

```text
D:\dev\code\mobilerpa\repos\mobilerpa-agent\js-agent
```

回滚入口文件：

```text
D:\dev\code\mobilerpa\repos\mobilerpa-agent\js-agent\agent.js
```

## 4. 已完成验证

当前已经完成：

- `ts-agent` 本地 `npm run check`
- `ts-agent` 本地 `npm run build`
- `ts-agent/release/agent.js` AutoJs6 启动验证
- 设备注册验证
- WebSocket 心跳验证
- 单脚本 `assign_task -> task_ack -> task_result` 验证
- heartbeat 重复调度问题修复后复验

## 5. 建议继续验证

下一步建议继续验证：

- `sync_script`
- `start_workflow_session`
- `stop_workflow_session`
- `stop.signal`

## 6. 回滚方式

如果 TS 版本出现严重兼容问题：

1. 将发布源从 `ts-agent/release` 切回 `js-agent`
2. 重新下发 `js-agent` 目录
3. 再做最小启动验证

## 7. 回滚后的最低验证

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent\js-agent
node .\agent.js --dry-run
```

至少确认：

- 能启动
- 能生成 `agent_uuid`
- 能输出 `register_payload`
- 能正常释放运行锁

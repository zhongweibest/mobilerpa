# agent

手机端 Agent 负责：
- 注册设备
- 建立并维持 WebSocket 连接
- 接收中心下发的任务
- 确保本地存在所需脚本版本
- 启动业务脚本
- 上报进度、日志和执行结果

## 当前已实现范围

当前最小版本已经实现：

1. `agent.js` 主入口。
2. 本地配置文件读写。
3. `agent_uuid` 生成和持久化。
4. 调用中心注册接口并保存 `device_id`。
5. 在 AutoJs6 中连接中心 WebSocket。
6. 发送 `hello` 和周期 `heartbeat`。
7. 中心服务短暂中断后按退避策略自动重连。
8. 接收 `assign_task` 后运行内置最小执行器。
9. 自动回传 `task_ack` 与 `task_result`。

当前暂未实现：

1. `shoppe` 真正业务脚本完善。
2. 关键步骤进度上报。

## 本地验证命令

在电脑上可以先使用 Node.js 做最小验证：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent
node .\agent\agent.js --dry-run --config D:\dev\code\mobilerpa\.tmp\agent-runtime\config.json
```

如果中心服务已经启动，可以调用真实注册接口：

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent
node .\agent\agent.js --center http://127.0.0.1:18080 --config D:\dev\code\mobilerpa\.tmp\agent-runtime\config.json
```

电脑端运行时默认跳过 WebSocket 长连接；如果只想验证注册链路，也可以显式追加：

```powershell
node .\agent\agent.js --center http://127.0.0.1:18080 --config D:\dev\code\mobilerpa\.tmp\agent-runtime\config.json --skip-ws
```

电脑端验收建议写入：

```text
D:\dev\code\mobilerpa\.tmp\agent-runtime\config.json
```

手机端运行时默认写入 `agent/runtime/config.json`，该路径属于本地运行状态，已通过 `.gitignore` 忽略。
真机推送脚本默认只在需要时写入 `agent/runtime/bootstrap.json`，首次启动时再由 `agent.js` 自动生成正式的 `config.json`。

## 当前执行约定

当前 `C2-3` 先采用“真实脚本文件入口 + 分阶段迁移业务脚本”打通端到端流程：

1. 中心下发 `assign_task`。
2. Agent 自动回传 `task_ack`。
3. 当 `script_name=shoppe_sync` 时，Agent 按 `script_version` 加载对应版本脚本入口。
4. `v0.1.0` 仍是最小演示版本，主要用于验证任务执行与结果回传链路。
5. `v0.1.1` 已迁移 `D:\dev\code\mobilerpa\projects\shoppe\main.js` 对应的最小真实业务链路，用于启动“极魔游戏助手”并尝试开启加速。
6. 当前中心服务已支持按脚本版本目录分发完整文件清单，`v0.1.1` 可以继续使用 `require` 和 `module.exports` 组织子模块。
7. 执行结束后 Agent 自动回传 `task_result`。

这套链路用于先完成真实服务、真实 WebSocket、真实手工验收闭环；后续继续补上版本管理、调试工具和进度上报。

## 真机脚本调试入口

`shoppe_sync@v0.1.1` 提供了可直接在 AutoJs6 中运行的调试入口：

```text
/sdcard/脚本/agent/scripts/shoppe_sync/v0.1.1/index_debug.js
```

调试入口的规则：

1. `index_debug.js` 只用于本机调试，不经过中心服务、WebSocket 或任务下发。
2. `index_debug.js` 会模拟 Agent 的 `task_runner`，调用同目录下的 `index.js`。
3. 业务逻辑仍然写在 `index.js`、`core/`、`utils/`、`config/` 等模块里，调试入口不复制业务逻辑。
4. 在 AutoJs6 中直接运行 `index_debug.js` 后，可以通过 AutoJs6 控制台查看 `[DEBUG]` 日志和执行结果。

如果需要重新推送脚本后调试，建议使用 toolkit 或中心服务的 Agent 下发能力先把最新脚本同步到手机，再在 AutoJs6 中手动打开上述文件运行。

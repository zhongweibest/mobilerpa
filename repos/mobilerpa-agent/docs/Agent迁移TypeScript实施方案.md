# Agent 迁移 TypeScript 实施方案

## 1. 目标

将手机端 Agent 从纯 JavaScript 形态迁移为 TypeScript 工程，同时把 JS 版本与 TS 版本彻底拆成两个独立目录：

- `js-agent/`
- `ts-agent/`

迁移后的默认正式发布源为：

```text
ts-agent/release
```

## 2. 目录模型

### 2.1 JS 版

`js-agent/` 用于保留纯 JavaScript 基线版本，作为：

- 回滚源
- 对照源
- 紧急恢复源

### 2.2 TS 版

`ts-agent/` 是独立的 TypeScript 工程，包含：

- `src/` 源码
- `dist/` 中间编译产物
- `release/` 最终发布产物
- `runtime/` 本地运行状态
- `tools/build-agent-bundle.js` 构建后单文件打包脚本

## 3. 迁移范围

本次迁移只覆盖 Agent 核心，不覆盖业务脚本。

迁移范围包括：

- `js-agent/agent.js`
- `js-agent/lib/runtime.js`
- `js-agent/lib/config_store.js`
- `js-agent/lib/center_client.js`
- `js-agent/lib/ws_client.js`
- `js-agent/lib/task_runner.js`
- `js-agent/lib/workflow_session_runner.js`

不在迁移范围内：

- 业务脚本目录
- 中心服务协议字段
- 手机端业务脚本开放机制

## 4. 构建与发布原则

- TypeScript 编译阶段输出 `CommonJS`
- 发布阶段对入口做单文件 bundle，确保 AutoJs6 可直接执行
- 不依赖 AutoJs6 提供 `exports`、`module`、`require`
- 正式发布源只认 `ts-agent/release`
- 需要回滚时直接切到 `js-agent`

## 5. 最终目录形态

```text
repos/mobilerpa-agent/
  js-agent/
    agent.js
    lib/
    config.example.json
    README.md
  ts-agent/
    src/
    dist/
    release/
    runtime/
    tools/
      build-agent-bundle.js
    config.example.json
    package.json
    tsconfig.json
    README.md
  docs/
```

## 6. 验收阶段

### 阶段 1：工程隔离

完成标准：

- `js-agent` 与 `ts-agent` 两个目录独立存在
- TS 构建配置不再引用旧混合目录
- 文档、发布源、回滚源都切到两目录模型

### 阶段 2：基础运行链路

完成标准：

- AutoJs6 可直接启动 `ts-agent/release/agent.js`
- 设备注册正常
- WebSocket 建连正常
- heartbeat 正常且无重复调度

### 阶段 3：单脚本任务链路

完成标准：

- `assign_task`
- `task_ack`
- 脚本下载
- 脚本加载
- `task_result`

全部真机通过

### 阶段 4：工作流链路

完成标准：

- `start_workflow_session`
- `stop_workflow_session`
- 相关事件与结果上报

真机通过

## 7. 当前正式口径

- 正式维护版本：`ts-agent`
- 正式发布源：`ts-agent/release`
- 回滚基线：`js-agent`

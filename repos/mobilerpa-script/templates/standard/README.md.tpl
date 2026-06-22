# __SCRIPT_NAME__ @ __SCRIPT_VERSION__

## 目录说明

- `index.js`：供 Agent/中心任务调度执行的正式入口。
- `index_debug.js`：供 AutoJs6 真机直接运行的调试入口。

## 开发约定

1. 默认使用中文注释、中文文档、中文提交信息。
2. 正式执行入口统一导出 `run(context, helpers)`。
3. 关键步骤统一通过 `reportProgress(stepName, message, status, extra)` 上报。
4. 真机调试优先运行 `index_debug.js`，不要直接运行 `index.js`。
5. 调试通过后，再打包成 zip 上传到中心服务。

## 初始改造点

1. 在 `index.js` 的 `executeBusiness(...)` 中补充真实业务逻辑。
2. 根据业务修改 `step_name`、`message` 和 `extra` 内容。
3. 如需拆分多个模块，可继续用 `module.exports` 方式组织子文件。

# ts-agent

`ts-agent` 是 MobileRPA 手机端 Agent 的 TypeScript 独立工程。

## 目录职责

- `src/`
  TypeScript 源码
- `dist/`
  中间编译产物
- `release/`
  最终正式发布目录
- `runtime/`
  本地运行状态与配置
- `tools/build-agent-bundle.js`
  将 `tsc` 产物打包为 AutoJs6 可直接执行的单文件入口

## 发布约定

- 正式发布源目录：`ts-agent/release`
- 手机端入口文件：`ts-agent/release/agent.js`

## 常用命令

```powershell
cd D:\dev\code\mobilerpa\repos\mobilerpa-agent\ts-agent
npm run check
npm run build
node .\release\agent.js --dry-run
```

## 说明

`release/agent.js` 是最终发布入口。
`dist/agent.js` 只是构建过程中的中间产物。

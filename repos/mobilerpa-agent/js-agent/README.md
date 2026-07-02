# js-agent

`js-agent` 是 MobileRPA 手机端 Agent 的纯 JavaScript 备份版本。

## 用途

- 作为 TS 迁移前的回滚基线
- 用于对照 JS 旧实现与 TS 新实现
- 在 TS 版本出现严重兼容问题时，可作为恢复源

## 当前定位

- 不作为默认发布版本
- 默认正式发布版本为 `ts-agent/dist`

## 目录说明

- `agent.js`
  旧版主入口
- `lib/`
  旧版依赖模块
- `config.example.json`
  旧版示例配置

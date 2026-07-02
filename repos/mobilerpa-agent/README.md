# mobilerpa-agent

这是 MobileRPA 的手机端运行仓库。

## 当前结构

- [js-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/js-agent)
  纯 JavaScript 版本，作为回滚基线保留
- [ts-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent)
  TypeScript 独立工程，当前正式维护版本
- `docs/`
  迁移、验收与方案文档

## 当前发布约定

- 默认正式发布源： [ts-agent/release](D:/dev/code/mobilerpa/repos/mobilerpa-agent/ts-agent/release)
- JS 回滚基线： [js-agent](D:/dev/code/mobilerpa/repos/mobilerpa-agent/js-agent)

## 原则

- JS 版和 TS 版必须目录隔离
- 业务脚本与 Agent 核心继续解耦
- 仓库规范与设计说明默认使用中文

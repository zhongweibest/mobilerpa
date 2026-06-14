# MobileRPA 文档入口

本文档作为项目文档索引，用于快速判断“该看哪份文档、该维护哪份文档”。后续新增或调整文档时，优先保持这里的入口关系清晰。

## 使用原则

- 当前任务状态、下一步开发顺序、验收进度，以 `docs/项目看板.md` 为准。
- 长期架构、技术选型、通信方案、仓库规划，以 `docs/整体规划方案.md` 为准。
- 开发阶段、开发周期、任务拆分和验收门禁，以 `docs/开发阶段计划.md` 为准。
- 人工验收入口以 `docs/acceptance-checklists/README.md` 为索引；具体可执行验收步骤维护在对应模块的 `docs/` 目录中。
- 旧计划文档可以作为历史参考，但不再和看板并行维护任务状态。
- 项目文档和代码注释默认使用中文。

## 主文档

| 文档 | 定位 | 维护方式 |
|---|---|---|
| `docs/整体规划方案.md` | 项目整体规划方案 | 架构级、方案级变化时更新 |
| `docs/群控工作流编排方案.md` | 群控工作流编排专项方案 | 工作流模型、运行实例和编排边界变化时更新 |
| `docs/项目看板.md` | 当前执行主看板 | 每完成或启动一个功能项时更新 |
| `docs/开发阶段计划.md` | 开发阶段计划 | 开发阶段、开发周期、验收门禁变化时更新 |
| `docs/acceptance-checklists/README.md` | 验收入口索引 | 新增或调整模块验收入口时更新 |
| `docs/文档治理建议.md` | 文档治理建议 | 文档结构调整或归档策略变化时更新 |
| `docs/工作区规划.md` | 工作区轻量说明 | 仅在仓库结构变化时更新 |
| `docs/验收记录/` | 阶段验收记录目录 | 阶段收口或试运行结论产出时更新 |

## 参考文档

| 文档 | 当前定位 | 后续建议 |
|---|---|---|
| `docs/快速上线计划.md` | 快速上线策略和范围取舍说明 | 保留策略判断，不维护任务状态 |

## 归档文档

| 文档 | 归档原因 |
|---|---|
| `docs/archive/开发计划.md` | 开发顺序、依赖关系和并行原则已吸收到 `开发阶段计划.md` 与 `项目看板.md` |
| `docs/archive/快速上线角色分工排期.md` | 角色分工和并行协作原则已吸收到 `开发阶段计划.md` 与 `项目看板.md` |
| `docs/archive/acceptance-checklists-drafts/` | 未实现功能的预写验收清单已归档，后续不作为当前验收依据 |
| `repos/mobilerpa-center/docs/web/archive/前端快速上线清单.md` | 前端快速上线边界、交付节奏和后端依赖已吸收到 `前端结构说明.md` |
| `repos/mobilerpa-agent/docs/archive/手机端四周快速上线实施清单.md` | 手机端上线检查项、失败证据链、灰度回滚和值守要求已吸收到 `手机端运行时最小契约.md` |

## 子仓库文档

| 位置 | 定位 |
|---|---|
| `repos/mobilerpa-center/docs/server/中心服务架构.md` | 中心服务架构与模块边界 |
| `repos/mobilerpa-center/docs/server/中心服务开发约定.md` | 中心服务开发规范 |
| `repos/mobilerpa-center/docs/server/中心服务手工验收.md` | 中心服务手工启动与接口验收 |
| `repos/mobilerpa-center/tools/README.md` | 中心服务工具脚本说明 |
| `repos/mobilerpa-center/docs/web/前端结构说明.md` | 前端目录结构与页面范围 |
| `repos/mobilerpa-center/docs/web/前端开发约定.md` | 前端本地启动、环境配置和请求规则 |
| `repos/mobilerpa-center/docs/web/archive/前端快速上线清单.md` | 前端快速上线历史参考清单 |
| `repos/mobilerpa-agent/docs/手机端与脚本架构.md` | Agent 与脚本架构 |
| `repos/mobilerpa-agent/docs/手机端开发约定.md` | Agent 仓库开发规范 |
| `repos/mobilerpa-agent/docs/手机端运行时最小契约.md` | Agent 与业务脚本运行契约 |
| `repos/mobilerpa-agent/docs/新脚本接入模板清单.md` | 新脚本接入模板清单 |
| `repos/mobilerpa-agent/docs/archive/手机端四周快速上线实施清单.md` | 手机端快速上线历史参考清单 |

## 合并与归档方向

- `开发计划.md`、`快速上线角色分工排期.md` 的任务级内容不再单独维护，后续以 `项目看板.md` 为准。
- `快速上线计划.md` 保留为“为什么先做 4 周早期生产版”的策略说明，不作为进度表使用。
- `群控工作流编排方案.md` 保留为专项方案文档，承接 `整体规划方案.md` 中的编排层设计，不单独维护任务状态。
- 已归档文档只作为历史参考，不再维护任务状态。
- 未实现功能不提前维护完整验收清单，功能实现后再在对应模块 `docs/` 中生成可执行验收文档。

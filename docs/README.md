# MobileRPA 文档入口

本文档作为项目文档索引，用于快速判断“该看哪份文档、该维护哪份文档”。新增或调整文档时，应优先保持这里的入口关系清晰。

## 使用原则

- 长期架构、技术选型、通信方案、仓库与工作区规划，以 `docs/整体规划方案.md` 为准。
- 人工验收入口以 `docs/acceptance-checklists/README.md` 为索引；中心端正式主线验收统一维护在根目录文档中。
- 过程性计划、阶段排期和项目看板不再作为正式文档持续维护。
- 项目文档和代码注释默认使用中文。

## 主文档

| 文档 | 定位 | 维护方式 |
|---|---|---|
| `docs/整体规划方案.md` | 项目整体规划方案 | 架构级、方案级变化时更新 |
| `docs/页面与主线手工验收.md` | 中心端正式页面验收与主线联调入口 | 页面主线、后台入口或设备联动变化时更新 |
| `docs/acceptance-checklists/README.md` | 验收入口索引 | 新增或调整模块验收入口时更新 |
| `docs/验收记录/` | 阶段验收记录目录 | 阶段收口或试运行结论产出时更新 |

## 子仓库文档

| 位置 | 定位 |
|---|---|
| `repos/mobilerpa-center/docs/server/中心服务架构.md` | 中心服务架构与模块边界 |
| `repos/mobilerpa-center/docs/server/中心服务开发约定.md` | 中心服务开发规范 |
| `repos/mobilerpa-center/docs/server/中心服务补充检查手册.md` | 中心服务单独启动、接口烟测与补充排查 |
| `repos/mobilerpa-center/tools/README.md` | 中心服务工具脚本说明 |
| `repos/mobilerpa-center/docs/web/前端结构说明.md` | 前端目录结构与页面范围 |
| `repos/mobilerpa-center/docs/web/前端开发约定.md` | 前端本地启动、环境配置和请求规则 |
| `repos/mobilerpa-agent/docs/手机端与脚本架构.md` | Agent 与脚本架构 |
| `repos/mobilerpa-agent/docs/手机端开发约定.md` | Agent 仓库开发规范 |
| `repos/mobilerpa-agent/docs/手机端运行时最小契约.md` | Agent 与业务脚本运行契约 |
| `repos/mobilerpa-agent/docs/新脚本接入模板清单.md` | 新脚本接入模板清单 |

## 合并与归档方向

- 过程性计划、阶段排期和项目看板文档不再作为正式入口维护。
- 仓库与工作区规划统一维护在 `docs/整体规划方案.md` 第 5 章。
- 工作流编排主定义统一维护在 `docs/整体规划方案.md` 第 14 章。
- 中心端页面主线验收统一维护在 `docs/页面与主线手工验收.md`；模块目录只保留架构、开发约定和补充检查文档。
- 未实现功能不提前维护完整验收清单，功能实现后再在对应模块 `docs/` 中生成可执行验收文档。

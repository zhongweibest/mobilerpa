# MobileRPA 验收入口索引

本目录只作为项目级验收入口索引，不再存放具体功能验收清单。

具体可执行验收文档必须放在对应模块自己的 `docs/` 目录中，例如：

1. 中心服务相关验收：`repos/mobilerpa-center/docs/server/`
2. Agent 与脚本相关验收：`repos/mobilerpa-agent/docs/`
3. 中心前端相关验收：`repos/mobilerpa-center/docs/web/`

这样可以避免根目录验收文档和模块验收文档重复维护，也能保证验收步骤跟随真实实现一起更新。

## 验收文档原则

1. 只为已经实现或正在等待用户验收的功能提供可执行验收文档。
2. 未实现功能不提前编写完整验收清单，只在看板中保留验收目标。
3. 每份可执行验收文档都必须提供真实服务启动方式、完整接口地址、完整请求体、预期响应和常见错误排查。
4. Apifox、Postman、Invoke-RestMethod、WebSocket 工具等人工验收方式必须至少提供一种完整可复制示例。
5. 单元测试、脚本自测或我自行判断通过，不能替代用户人工验收通过。
6. 功能完成实现后，如果原有验收文档不存在，必须先补模块验收文档，再交给用户验收。

## 如何选择验收文档

选择顺序：

1. 先打开 `docs/项目看板.md`，确认当前开发周期或开发任务编号。
2. 查看当前周期下方的“验收文档”字段。
3. 如果看板标注为“待实现后生成”，说明该功能还没有真实可执行验收文档，不能按草稿清单验收。
4. 如果看板中的验收入口不清楚，先补充或确认模块验收文档，再开始验收。

## 当前可执行验收入口

| 开发周期 | 覆盖任务 | 可执行验收文档 | 说明 |
|---|---|---|---|
| `C0-2` 中心 HTTP 最小验收链路 | `P1-BE-001` 到 `P1-BE-005`、`P1-QA-001` | `repos/mobilerpa-center/docs/server/中心服务手工验收.md` | 已实现，已通过用户启动服务方式验收 |
| `C1-1` WebSocket `hello/heartbeat` 链路 | `P1-BE-006`、`P1-BE-007`、`P1-BE-008`、`P1-QA-002`、`NEXT-001` | `repos/mobilerpa-center/docs/server/中心服务手工验收.md` | 已实现，当前等待用户验收确认 |
| `C1-3` 设备列表前端入口增项：设备删除 | `P1-BE-004B`、`P1-FE-004A`、`P1-QA-004A` | `repos/mobilerpa-center/docs/server/中心服务手工验收.md`、`repos/mobilerpa-center/docs/web/前端手工验收.md` | 实现后使用服务端接口与前端页面双路径验收 |

## 待实现后生成验收文档

| 开发周期 | 预计模块验收文档位置 | 当前状态 |
|---|---|---|
| `C1-2` Agent 注册与心跳 | `repos/mobilerpa-agent/docs/手机端手工验收.md` | 待实现后生成 |
| `C1-3` 设备列表前端入口 | `repos/mobilerpa-center/docs/web/前端手工验收.md` | 待实现后生成 |
| `C1-4` 阶段验收与接口集合 | `docs/验收记录/S1-Apifox接口集合说明.md`、`docs/验收记录/S1阶段验收记录.md` | 已生成，等待用户审阅与确认 |
| `C2` 单任务下发与脚本执行 | `repos/mobilerpa-center/docs/server/中心服务手工验收.md`、`repos/mobilerpa-agent/docs/手机端手工验收.md` | 待实现后补充 |
| `C3` 最小后台与排障能力 | `repos/mobilerpa-center/docs/web/前端手工验收.md`、`repos/mobilerpa-agent/docs/手机端手工验收.md` | 待实现后补充 |
| `C4` 早期生产试运行 | `docs/验收记录/` | 待用户确认记录目录后生成 |
| `C5` 规模化与生产加固 | `docs/验收记录/` | 待用户确认记录目录后生成 |

## 已归档草稿

早期预写但尚未具备真实实现支撑的验收清单，已经归档到：

```text
docs/archive/acceptance-checklists-drafts/
```

这些文档只作为历史思路参考，不作为当前验收依据。后续功能实现时，应根据真实接口、真实页面或真机流程重新生成模块内验收文档。

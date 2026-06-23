import type { RouteRecordRaw } from "vue-router";

import { AppShell } from "../layouts/AppShell";
import { PlaceholderPage } from "../pages/common/PlaceholderPage";
import { DevicesPage } from "../pages/devices/DevicesPage";
import { DiscoveryPage } from "../pages/discovery/DiscoveryPage";
import { HomePage } from "../pages/home/HomePage";
import { PlansPage } from "../pages/plans/PlansPage";
import { PlanRunsPage } from "../pages/plans/PlanRunsPage";
import { ScriptsPage } from "../pages/scripts/ScriptsPage";
import { SettingsPage } from "../pages/settings/SettingsPage";
import { WorkflowInstancesPage } from "../pages/workflows/WorkflowInstancesPage";
import { WorkflowsPage } from "../pages/workflows/WorkflowsPage";

type AppRouteMeta = {
  title: string;
  section?: string;
  navGroup?: "main" | "workflow" | "ops";
  navOrder?: number;
  navVisible?: boolean;
  navBadge?: string;
  summary?: string;
};

function meta(input: AppRouteMeta): AppRouteMeta {
  return input;
}

export const appRoutes: RouteRecordRaw[] = [
  {
    path: "/",
    component: AppShell,
    children: [
      {
        path: "",
        name: "home",
        component: HomePage,
        meta: meta({
          title: "主页看板",
          section: "总览",
          navGroup: "main",
          navOrder: 1,
          navVisible: true,
          summary: "集中查看设备、计划任务、脚本、工作流与系统状态，作为整个平台的全局入口。"
        })
      },
      {
        path: "devices",
        name: "devices",
        component: DevicesPage,
        meta: meta({
          title: "设备列表",
          section: "设备管理",
          navGroup: "main",
          navOrder: 2,
          navVisible: true,
          summary: "查看设备在线状态、绑定状态、心跳时间、当前占用与设备级操作。"
        })
      },
      {
        path: "discovery",
        name: "discovery",
        component: DiscoveryPage,
        meta: meta({
          title: "设备发现",
          section: "设备管理",
          navGroup: "main",
          navOrder: 3,
          navVisible: true,
          summary: "发现无线调试设备，完成连接、断开、单设备下发 Agent 与批量下发 Agent。"
        })
      },
      {
        path: "plans",
        name: "plans",
        component: PlansPage,
        meta: meta({
          title: "计划任务",
          section: "计划任务",
          navGroup: "workflow",
          navOrder: 1,
          navVisible: true,
          summary: "统一管理脚本型与工作流型计划任务定义，并从这里启动新的计划任务实例。"
        })
      },
      {
        path: "plan-runs",
        name: "plan-runs",
        component: PlanRunsPage,
        meta: meta({
          title: "计划任务实例",
          section: "计划任务",
          navGroup: "workflow",
          navOrder: 2,
          navVisible: true,
          summary: "查看计划任务实例运行状态、设备执行情况与计划任务事件。"
        })
      },
      {
        path: "workflows",
        name: "workflows",
        component: WorkflowsPage,
        meta: meta({
          title: "工作流编排",
          section: "工作流",
          navGroup: "workflow",
          navOrder: 3,
          navVisible: true,
          summary: "维护工作流定义、节点关系、脚本版本绑定与编排入口。"
        })
      },
      {
        path: "workflow-instances",
        name: "workflow-instances",
        component: WorkflowInstancesPage,
        meta: meta({
          title: "工作流实例",
          section: "工作流",
          navGroup: "workflow",
          navOrder: 4,
          navVisible: true,
          summary: "兼容查看历史工作流实例、设备执行情况与事件流水，后续将逐步收口到计划任务实例。"
        })
      },
      {
        path: "scripts",
        name: "scripts",
        component: ScriptsPage,
        meta: meta({
          title: "脚本管理",
          section: "脚本能力",
          navGroup: "ops",
          navOrder: 1,
          navVisible: true,
          summary: "维护脚本名称与版本，支持上传、查看、删除与脚本下发到设备。"
        })
      },
      {
        path: "settings",
        name: "settings",
        component: SettingsPage,
        meta: meta({
          title: "系统配置",
          section: "运维配置",
          navGroup: "ops",
          navOrder: 2,
          navVisible: true,
          summary: "统一维护中心服务地址与后续系统级默认参数，作为全局配置入口。"
        })
      },
      {
        path: "bindings",
        name: "bindings",
        component: PlaceholderPage,
        meta: meta({
          title: "设备绑定",
          section: "设备管理",
          navGroup: "main",
          navOrder: 4,
          navVisible: true,
          navBadge: "规划中",
          summary: "后续在这里维护设备物理位置、分组、标签与批量绑定能力。"
        })
      },
      {
        path: "alerts",
        name: "alerts",
        component: PlaceholderPage,
        meta: meta({
          title: "异常中心",
          section: "运维配置",
          navGroup: "ops",
          navOrder: 3,
          navVisible: true,
          navBadge: "规划中",
          summary: "后续统一查看设备异常、任务异常、工作流异常与人工关注项。"
        })
      },
      {
        path: "reports",
        name: "reports",
        component: PlaceholderPage,
        meta: meta({
          title: "报表中心",
          section: "运维配置",
          navGroup: "ops",
          navOrder: 4,
          navVisible: true,
          navBadge: "规划中",
          summary: "后续承接产出统计、成功率分析、设备利用率与日报数据。"
        })
      },
      {
        path: "feed",
        name: "feed",
        component: PlaceholderPage,
        meta: meta({
          title: "消息列表",
          section: "运维配置",
          navGroup: "ops",
          navOrder: 5,
          navVisible: true,
          navBadge: "规划中",
          summary: "后续汇总设备、任务、脚本、工作流与异常消息订阅流。"
        })
      }
    ]
  }
];

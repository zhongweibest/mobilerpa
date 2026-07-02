import type { RouteRecordRaw } from "vue-router";

import { AppShell } from "../layouts/AppShell";
import { BindingsPage } from "../pages/bindings/BindingsPage";
import { DevicesPage } from "../pages/devices/DevicesPage";
import { DiscoveryPage } from "../pages/discovery/DiscoveryPage";
import { HomePage } from "../pages/home/HomePage";
import { PlansPage } from "../pages/plans/PlansPage";
import { PlanRunsPage } from "../pages/plans/PlanRunsPage";
import { ScriptsPage } from "../pages/scripts/ScriptsPage";
import { SettingsPage } from "../pages/settings/SettingsPage";
import { SoftwarePage } from "../pages/software/SoftwarePage";
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
          navOrder: 3,
          navVisible: true,
          summary: "统一维护中心服务地址与后续系统级默认参数，作为全局配置入口。"
        })
      },
      {
        path: "software",
        name: "software",
        component: SoftwarePage,
        meta: meta({
          title: "软件管理",
          section: "软件能力",
          navGroup: "ops",
          navOrder: 2,
          navVisible: true,
          summary: "维护软件名称、描述与安装包文件，支持新增、编辑和删除。"
        })
      },
      {
        path: "bindings",
        name: "bindings",
        component: BindingsPage,
        meta: meta({
          title: "设备绑定",
          section: "设备管理",
          navGroup: "main",
          navOrder: 4,
          navVisible: true,
          summary: "维护分区、排号、槽位号三级位置模型，并在槽位级别完成设备绑定。"
        })
      }
    ]
  }
];

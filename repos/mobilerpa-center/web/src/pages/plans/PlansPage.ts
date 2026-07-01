// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTimePicker,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, nextTick, onMounted, reactive, ref, watch } from "vue";

import { useNoticesStore } from "../../stores/notices";
import { usePlansStore } from "../../stores/plans";
import { useScriptsStore } from "../../stores/scripts";
import { useWorkflowsStore } from "../../stores/workflows";
import type { LocationNodeRecord } from "../../types/device";
import type { PlanDefinitionRecord, PlanRowBinding } from "../../types/plan";
import type { WorkflowDefinitionRecord } from "../../types/workflow";
import { formatDateTime } from "../../utils/device";
import { fetchLocationNodes } from "../../api/devices";

const PAGE_SIZES = [10, 20, 30, 50, 100];
type LocationRowGroup = {
  zone_id: string;
  zone_name: string;
  row_id: string;
  row_name: string;
  slot_count: number;
  device_count: number;
};

type LocationRowTreeNode = {
  node_key: string;
  node_type: "zone" | "row";
  zone_id: string;
  zone_name: string;
  row_id: string;
  row_name: string;
  slot_count: number;
  device_count: number;
  children?: LocationRowTreeNode[];
};

function renderPlanStatus(status: string) {
  const normalized = (status || "").trim();
  let type: "success" | "danger" | "warning" | "info" = "info";
  let label = normalized || "unknown";

  if (normalized === "enabled") {
    type = "success";
    label = "启用";
  } else if (normalized === "disabled") {
    type = "danger";
    label = "停用";
  }

  return h(ElTag, { type, effect: "light" }, () => label);
}

function resolveScheduleLabel(value: string) {
  return value === "daily" ? "按天循环（自动调度）" : "一次性（手工启动）";
}

function resolveTargetTypeLabel(value: string) {
  return value === "workflow" ? "工作流" : "脚本";
}

function resolveStartButtonLabel(item: PlanDefinitionRecord) {
  return item.schedule_type === "daily" ? "立即启动" : "启动";
}

function parseTodayTime(value: string) {
  const trimmed = (value || "").trim();
  if (!/^\d{2}:\d{2}:\d{2}$/.test(trimmed)) {
    return null;
  }
  const [hours, minutes, seconds] = trimmed.split(":").map((item) => Number(item));
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate(), hours, minutes, seconds, 0);
}

function isManualStartEnabled(item: PlanDefinitionRecord) {
  if (item.schedule_type !== "daily") {
    return true;
  }
  const startAt = parseTodayTime(item.daily_start_time || "");
  if (!startAt) {
    return true;
  }
  return Date.now() < startAt.getTime();
}

function resolveStartDisabledReason(item: PlanDefinitionRecord) {
  if (item.schedule_type !== "daily") {
    return "";
  }
  return isManualStartEnabled(item) ? "" : "已到当天开始时间，当前只能等待自动调度启动";
}

function resolveWorkflowName(workflows: WorkflowDefinitionRecord[], workflowDefID: string) {
  const matched = workflows.find((item) => item.workflow_def_id === workflowDefID);
  if (matched && matched.workflow_name.trim() !== "") {
    return matched.workflow_name;
  }
  return workflowDefID || "未匹配工作流";
}

function resolvePlanTargetLabel(item: PlanDefinitionRecord, workflows: WorkflowDefinitionRecord[]) {
  if (item.target_type === "workflow") {
    return resolveWorkflowName(workflows, item.target_workflow_def_id);
  }

  const scriptName = (item.target_script_name || "").trim();
  const scriptVersion = (item.target_script_version || "").trim();
  if (scriptName === "") {
    return "未匹配脚本";
  }
  return scriptVersion === "" ? scriptName : `${scriptName}@${scriptVersion}`;
}

function buildRowLabel(binding: PlanRowBinding) {
  const zoneName = (binding.zone_name || binding.zone_id || "").trim();
  const rowName = (binding.row_name || binding.row_id || "").trim();
  return `${zoneName}-${rowName}`;
}

function normalizeNodeName(value: string) {
  return (value || "").trim();
}

function buildLocationRowGroups(nodes: LocationNodeRecord[]): LocationRowGroup[] {
  const zoneMap = new Map<string, LocationNodeRecord>();
  const rowMap = new Map<string, LocationNodeRecord>();
  const slotMap = new Map<string, LocationNodeRecord[]>();

  for (const node of nodes) {
    if (node.node_type === "zone") {
      zoneMap.set(node.node_id, node);
    } else if (node.node_type === "row") {
      rowMap.set(node.node_id, node);
    } else if (node.node_type === "slot") {
      const list = slotMap.get(node.parent_id) || [];
      list.push(node);
      slotMap.set(node.parent_id, list);
    }
  }

  const sortNodes = (list: LocationNodeRecord[]) =>
    [...list].sort((left, right) => {
      const sortDelta = Number(left.sort_order || 0) - Number(right.sort_order || 0);
      if (sortDelta !== 0) {
        return sortDelta;
      }
      return normalizeNodeName(left.node_name).localeCompare(normalizeNodeName(right.node_name), "zh-CN");
    });

  const groups: LocationRowGroup[] = [];
  for (const zone of sortNodes(Array.from(zoneMap.values()))) {
    const rows = sortNodes(Array.from(rowMap.values()).filter((row) => row.parent_id === zone.node_id));
    for (const row of rows) {
      groups.push({
        zone_id: zone.node_id,
        zone_name: zone.node_name || zone.node_id,
        row_id: row.node_id,
        row_name: row.node_name || row.node_id,
        slot_count: sortNodes(slotMap.get(row.node_id) || []).length,
        device_count: sortNodes(slotMap.get(row.node_id) || []).filter((slot) => (slot.device_id || "").trim() !== "").length
      });
    }
  }

  return groups;
}

function buildLocationRowTree(nodes: LocationNodeRecord[]): LocationRowTreeNode[] {
  const groups = buildLocationRowGroups(nodes);
  const zoneMap = new Map<string, LocationRowTreeNode>();

  for (const item of groups) {
    const zoneNode =
      zoneMap.get(item.zone_id) ||
      {
        node_key: `zone:${item.zone_id}`,
        node_type: "zone" as const,
        zone_id: item.zone_id,
        zone_name: item.zone_name,
        row_id: "",
        row_name: "",
        slot_count: 0,
        device_count: 0,
        children: []
      };

    zoneNode.children?.push({
      node_key: `row:${item.zone_id}:${item.row_id}`,
      node_type: "row",
      zone_id: item.zone_id,
      zone_name: item.zone_name,
      row_id: item.row_id,
      row_name: item.row_name,
      slot_count: item.slot_count,
      device_count: item.device_count
    });
    zoneNode.slot_count += item.slot_count;
    zoneNode.device_count += item.device_count;
    zoneMap.set(item.zone_id, zoneNode);
  }

  return Array.from(zoneMap.values());
}

export const PlansPage = defineComponent({
  name: "PlansPage",
  setup() {
    const plansStore = usePlansStore();
    const scriptsStore = useScriptsStore();
    const workflowsStore = useWorkflowsStore();
    const noticesStore = useNoticesStore();

    const { plans, total, page, pageSize, loading, creating, deletingPlanID, startingPlanID, mutatingDevices, errorMessage } = storeToRefs(plansStore);
    const { scripts } = storeToRefs(scriptsStore);
    const { workflows } = storeToRefs(workflowsStore);

    const createDialogVisible = ref(false);
    const startDialogVisible = ref(false);
    const updateRowsDialogVisible = ref(false);
    const detailDialogVisible = ref(false);
    const supportingDataWarning = ref("");
    const selectedPlan = ref<PlanDefinitionRecord | null>(null);
    const locationNodes = ref<LocationNodeRecord[]>([]);
    const createRowsTableRef = ref();
    const updateRowsTableRef = ref();

    const createForm = reactive({
      plan_name: "",
      description: "",
      target_type: "script",
      target_script_name: "",
      target_script_version: "",
      target_workflow_def_id: "",
      schedule_type: "once",
      daily_start_time: "",
      daily_deadline_time: "",
      status: "enabled",
      rows: [] as PlanRowBinding[]
    });

    const startForm = reactive({
      plan_def_id: "",
      plan_name: "",
      schedule_type: "once",
      rows: [] as PlanRowBinding[]
    });

    const updateRowsForm = reactive({
      plan_def_id: "",
      plan_name: "",
      rows: [] as PlanRowBinding[]
    });

    const availableRowTree = computed(() => buildLocationRowTree(locationNodes.value));
    const startRowTree = computed(() => {
      const selectedKeys = new Set(startForm.rows.map((item) => `${item.zone_id}:${item.row_id}`));
      return availableRowTree.value
        .map((zone) => {
          const children = (zone.children || []).filter((item) => selectedKeys.has(`${item.zone_id}:${item.row_id}`));
          if (children.length === 0) {
            return null;
          }
          return {
            ...zone,
            slot_count: children.reduce((total, item) => total + Number(item.slot_count || 0), 0),
            device_count: children.reduce((total, item) => total + Number(item.device_count || 0), 0),
            children
          };
        })
        .filter(Boolean) as LocationRowTreeNode[];
    });
    const detailRowTree = computed(() => {
      const selectedKeys = new Set((selectedPlan.value?.rows || []).map((item) => `${item.zone_id}:${item.row_id}`));
      return availableRowTree.value
        .map((zone) => {
          const children = (zone.children || []).filter((item) => selectedKeys.has(`${item.zone_id}:${item.row_id}`));
          if (children.length === 0) {
            return null;
          }
          return {
            ...zone,
            slot_count: children.reduce((total, item) => total + Number(item.slot_count || 0), 0),
            device_count: children.reduce((total, item) => total + Number(item.device_count || 0), 0),
            children
          };
        })
        .filter(Boolean) as LocationRowTreeNode[];
    });
    const availableScriptVersions = computed(() => {
      const selectedScript = scripts.value.find((item) => item.script_name === createForm.target_script_name);
      return selectedScript?.versions || [];
    });

    async function loadSupportingData() {
      const warnings: string[] = [];

      try {
        await scriptsStore.loadScripts();
      } catch (_error) {
        warnings.push("脚本列表加载失败");
      }

      try {
        await workflowsStore.loadWorkflows();
      } catch (_error) {
        warnings.push("工作流列表加载失败");
      }

      try {
        locationNodes.value = await fetchLocationNodes();
      } catch (_error) {
        warnings.push("位置树加载失败");
      }

      supportingDataWarning.value = warnings.join("；");

      if (scripts.value.length > 0 && createForm.target_script_name === "") {
        createForm.target_script_name = scripts.value[0]?.script_name || "";
        createForm.target_script_version = scripts.value[0]?.versions?.[0]?.script_version || "";
      }
      if (workflows.value.length > 0 && createForm.target_workflow_def_id === "") {
        createForm.target_workflow_def_id = workflows.value[0]?.workflow_def_id || "";
      }
    }

    async function loadPageData() {
      await plansStore.loadPlans();
      await loadSupportingData();
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(
      supportingDataWarning,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.warning(`计划任务辅助数据加载不完整：${value}`, 5000);
        }
      }
    );

    watch(
      errorMessage,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          if (loading.value) {
            noticesStore.error(`计划任务加载失败：${value}`, 5000);
          }
        }
      }
    );

    function resetCreateForm() {
      createForm.plan_name = "";
      createForm.description = "";
      createForm.target_type = "script";
      createForm.target_script_name = scripts.value[0]?.script_name || "";
      createForm.target_script_version = scripts.value[0]?.versions?.[0]?.script_version || "";
      createForm.target_workflow_def_id = workflows.value[0]?.workflow_def_id || "";
      createForm.schedule_type = "once";
      createForm.daily_start_time = "";
      createForm.daily_deadline_time = "";
      createForm.status = "enabled";
      createForm.rows = [];
    }

    function openCreateDialog() {
      resetCreateForm();
      createDialogVisible.value = true;
      void nextTick(() => {
        syncSelectedRows(createRowsTableRef.value, createForm.rows);
      });
    }

    function openStartDialog(planItem: PlanDefinitionRecord) {
      if (!isManualStartEnabled(planItem)) {
        noticesStore.warning(resolveStartDisabledReason(planItem), 5000);
        return;
      }
      startForm.plan_def_id = planItem.plan_def_id;
      startForm.plan_name = planItem.plan_name;
      startForm.schedule_type = planItem.schedule_type;
      startForm.rows = [...(planItem.rows || [])];
      startDialogVisible.value = true;
    }

    function handleCreateRowSelectionChange(items: LocationRowTreeNode[]) {
      createForm.rows = items
        .filter((item) => item.node_type === "row")
        .map((item) => ({
          zone_id: item.zone_id,
          row_id: item.row_id,
          zone_name: item.zone_name,
          row_name: item.row_name
        }));
    }

    function openUpdateRowsDialog(planItem: PlanDefinitionRecord) {
      updateRowsForm.plan_def_id = planItem.plan_def_id;
      updateRowsForm.plan_name = planItem.plan_name;
      updateRowsForm.rows = [...(planItem.rows || [])];
      updateRowsDialogVisible.value = true;
      void nextTick(() => {
        syncSelectedRows(updateRowsTableRef.value, updateRowsForm.rows);
      });
    }

    function handleUpdateRowSelectionChange(items: LocationRowTreeNode[]) {
      updateRowsForm.rows = items
        .filter((item) => item.node_type === "row")
        .map((item) => ({
          zone_id: item.zone_id,
          row_id: item.row_id,
          zone_name: item.zone_name,
          row_name: item.row_name
        }));
    }

    function openDetailDialog(planItem: PlanDefinitionRecord) {
      selectedPlan.value = planItem;
      detailDialogVisible.value = true;
    }

    function syncSelectedRows(tableInstance: any, rows: PlanRowBinding[]) {
      if (!tableInstance) {
        return;
      }
      const selectedKeys = new Set(rows.map((item) => `${item.zone_id}:${item.row_id}`));
      tableInstance.clearSelection();
      for (const zone of availableRowTree.value) {
        for (const child of zone.children || []) {
          if (selectedKeys.has(`${child.zone_id}:${child.row_id}`)) {
            tableInstance.toggleRowSelection(child, true);
          }
        }
      }
    }

    async function submitCreatePlan() {
      if (createForm.rows.length === 0) {
        noticesStore.warning("请至少选择一个分区-排", 5000);
        return;
      }
      try {
        await plansStore.submitPlan({
          plan_name: createForm.plan_name.trim(),
          description: createForm.description.trim(),
          target_type: createForm.target_type,
          target_script_name: createForm.target_type === "script" ? createForm.target_script_name : "",
          target_script_version: createForm.target_type === "script" ? createForm.target_script_version : "",
          target_workflow_def_id: createForm.target_type === "workflow" ? createForm.target_workflow_def_id : "",
          schedule_type: createForm.schedule_type,
          daily_start_time: createForm.schedule_type === "daily" ? createForm.daily_start_time.trim() : "",
          daily_deadline_time: createForm.schedule_type === "daily" ? createForm.daily_deadline_time.trim() : "",
          status: createForm.status,
          rows: createForm.rows
        });
        createDialogVisible.value = false;
        noticesStore.success("计划任务已创建", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务创建失败", 5000);
      }
    }

    async function submitStartPlan() {
      try {
        await plansStore.triggerStartPlan(startForm.plan_def_id);
        startDialogVisible.value = false;
        noticesStore.success("计划任务已启动", 3000);
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务启动失败", 5000);
      }
    }

    async function submitUpdateRows() {
      if (updateRowsForm.rows.length === 0) {
        noticesStore.warning("请至少保留一个分区-排", 5000);
        return;
      }
      try {
        await plansStore.updateDefinitionRows(updateRowsForm.plan_def_id, updateRowsForm.rows);
        updateRowsDialogVisible.value = false;
        noticesStore.success("计划任务绑定排号已更新", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务绑定排号更新失败", 5000);
      }
    }

    async function handleDeletePlan(planItem: PlanDefinitionRecord) {
      try {
        await ElMessageBox.confirm(
          `确认删除计划任务“${planItem.plan_name}”吗？如果该计划任务仍有运行中的实例，将不允许删除。`,
          "删除计划任务",
          {
            type: "warning",
            confirmButtonText: "确认删除",
            cancelButtonText: "取消"
          }
        );
      } catch (_error) {
        return;
      }

      try {
        await plansStore.removePlan(planItem.plan_def_id);
        noticesStore.success("计划任务已删除", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务删除失败", 5000);
      }
    }

    return () =>
      h("section", { class: "app-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(ElButton, { type: "primary", onClick: openCreateDialog }, () => "创建计划任务"),
          h(ElButton, { loading: loading.value, onClick: () => void loadPageData() }, () => "刷新")
        ]),
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            default: () =>
              plans.value.length === 0 && !loading.value
                ? h(ElEmpty, { description: "暂无计划任务定义" })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: plans.value, stripe: true, class: "tasks-table", tableLayout: "auto", height: "100%" },
                        {
                          default: () => [
                            h(ElTableColumn, { prop: "plan_name", label: "计划任务名称", minWidth: 180 }),
                            h(ElTableColumn, {
                              prop: "target_type",
                              label: "目标类型",
                              width: 120,
                              formatter: (_row: unknown, _column: unknown, value: string) => resolveTargetTypeLabel(value)
                            }),
                            h(ElTableColumn, {
                              label: "目标内容",
                              minWidth: 220,
                              formatter: (row: PlanDefinitionRecord) => resolvePlanTargetLabel(row, workflows.value)
                            }),
                            h(ElTableColumn, {
                              prop: "schedule_type",
                              label: "调度类型",
                              width: 180,
                              formatter: (_row: unknown, _column: unknown, value: string) => resolveScheduleLabel(value)
                            }),
                            h(ElTableColumn, {
                              label: "绑定排数",
                              width: 120,
                              formatter: (row: PlanDefinitionRecord) => String((row.rows || []).length)
                            }),
                            h(
                              ElTableColumn,
                              {
                                prop: "status",
                                label: "计划状态",
                                width: 120
                              },
                              {
                                default: (scope: { row: PlanDefinitionRecord }) => renderPlanStatus(scope.row.status)
                              }
                            ),
                            h(ElTableColumn, {
                              prop: "updated_at",
                              label: "更新时间",
                              minWidth: 180,
                              formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                            }),
                            h(
                              ElTableColumn,
                              {
                                label: "操作",
                                minWidth: 220,
                                fixed: "right"
                              },
                              {
                                default: (scope: { row: PlanDefinitionRecord }) =>
                                  h("div", { class: "table-actions table-actions--nowrap" }, [
                                    h(
                                      ElButton,
                                      {
                                        link: true,
                                        type: "primary",
                                        disabled: !isManualStartEnabled(scope.row),
                                        loading: startingPlanID.value === scope.row.plan_def_id,
                                        title: resolveStartDisabledReason(scope.row),
                                        onClick: () => openStartDialog(scope.row)
                                      },
                                      () => resolveStartButtonLabel(scope.row)
                                    ),
                                    h(
                                      ElDropdown,
                                      {
                                        trigger: "click"
                                      },
                                      {
                                        default: () => h(ElButton, { link: true, type: "primary" }, () => "更多"),
                                        dropdown: () =>
                                          h(
                                            ElDropdownMenu,
                                            null,
                                            {
                                              default: () => [
                                                h(
                                                  ElDropdownItem,
                                                  {
                                                    key: "change_rows",
                                                    onClick: () => openUpdateRowsDialog(scope.row)
                                                  },
                                                  () => "变更选择"
                                                ),
                                                h(
                                                  ElDropdownItem,
                                                  {
                                                    key: "view",
                                                    onClick: () => openDetailDialog(scope.row)
                                                  },
                                                  () => "查看"
                                                ),
                                                h(
                                                  ElDropdownItem,
                                                  {
                                                    key: "delete",
                                                    onClick: () => {
                                                      void handleDeletePlan(scope.row);
                                                    }
                                                  },
                                                  () => (deletingPlanID.value === scope.row.plan_def_id ? "删除中..." : "删除")
                                                )
                                              ]
                                            }
                                          )
                                      }
                                    )
                                  ])
                              }
                            )
                          ]
                        }
                      )
                    ]),
                    h(
                      "div",
                      { class: "page-pagination" },
                      h(ElPagination, {
                        background: true,
                        currentPage: page.value,
                        pageSize: pageSize.value,
                        pageSizes: PAGE_SIZES,
                        total: total.value,
                        layout: "total, sizes, prev, pager, next, jumper",
                        "onUpdate:currentPage": (value: number) => {
                          void plansStore.changePage(value);
                        },
                        "onUpdate:pageSize": (value: number) => {
                          void plansStore.changePageSize(value);
                        }
                      })
                    )
                  ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: createDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (createDialogVisible.value = value),
            title: "创建计划任务",
            width: "720px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "计划任务名称" }, () =>
                  h(ElInput, {
                    modelValue: createForm.plan_name,
                    "onUpdate:modelValue": (value: string) => (createForm.plan_name = value),
                    placeholder: "请输入计划任务名称"
                  })
                ),
                h(ElFormItem, { label: "说明" }, () =>
                  h(ElInput, {
                    modelValue: createForm.description,
                    "onUpdate:modelValue": (value: string) => (createForm.description = value),
                    type: "textarea",
                    rows: 3
                  })
                ),
                h(ElFormItem, { label: "目标类型" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: createForm.target_type,
                      "onUpdate:modelValue": (value: string) => (createForm.target_type = value)
                    },
                    () => [h(ElOption, { label: "脚本", value: "script" }), h(ElOption, { label: "工作流", value: "workflow" })]
                  )
                ),
                createForm.target_type === "script"
                  ? h(ElFormItem, { label: "脚本名称" }, () =>
                      h(
                        ElSelect,
                        {
                          modelValue: createForm.target_script_name,
                          "onUpdate:modelValue": (value: string) => {
                            createForm.target_script_name = value;
                            const selected = scripts.value.find((item) => item.script_name === value);
                            createForm.target_script_version = selected?.versions?.[0]?.script_version || "";
                          }
                        },
                        () => scripts.value.map((item) => h(ElOption, { key: item.script_name, label: item.script_name, value: item.script_name }))
                      )
                    )
                  : h(ElFormItem, { label: "工作流名称" }, () =>
                      h(
                        ElSelect,
                        {
                          modelValue: createForm.target_workflow_def_id,
                          "onUpdate:modelValue": (value: string) => (createForm.target_workflow_def_id = value)
                        },
                        () => workflows.value.map((item) => h(ElOption, { key: item.workflow_def_id, label: item.workflow_name, value: item.workflow_def_id }))
                      )
                    ),
                createForm.target_type === "script"
                  ? h(ElFormItem, { label: "脚本版本" }, () =>
                      h(
                        ElSelect,
                        {
                          modelValue: createForm.target_script_version,
                          "onUpdate:modelValue": (value: string) => (createForm.target_script_version = value)
                        },
                        () =>
                          availableScriptVersions.value.map((item: { script_version: string }) =>
                            h(ElOption, { key: item.script_version, label: item.script_version, value: item.script_version })
                          )
                      )
                    )
                  : null,
                h(ElFormItem, { label: "调度类型" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: createForm.schedule_type,
                      "onUpdate:modelValue": (value: string) => (createForm.schedule_type = value)
                    },
                    () => [h(ElOption, { label: "一次性（手工启动）", value: "once" }), h(ElOption, { label: "按天循环（自动调度）", value: "daily" })]
                  )
                ),
                createForm.schedule_type === "daily"
                  ? h("div", { class: "dialog-grid" }, [
                      h(ElFormItem, { label: "每日开始时间" }, () =>
                        h(ElTimePicker, {
                          modelValue: createForm.daily_start_time,
                          "onUpdate:modelValue": (value: string) => (createForm.daily_start_time = value),
                          placeholder: "选择开始时间",
                          format: "HH:mm:ss",
                          valueFormat: "HH:mm:ss",
                          clearable: true
                        })
                      ),
                      h(ElFormItem, { label: "每日截止时间" }, () =>
                        h(ElTimePicker, {
                          modelValue: createForm.daily_deadline_time,
                          "onUpdate:modelValue": (value: string) => (createForm.daily_deadline_time = value),
                          placeholder: "选择结束时间",
                          format: "HH:mm:ss",
                          valueFormat: "HH:mm:ss",
                          clearable: true
                        })
                      )
                    ])
                  : null,
                h(ElFormItem, { label: "计划状态" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: createForm.status,
                      "onUpdate:modelValue": (value: string) => (createForm.status = value)
                    },
                    () => [h(ElOption, { label: "启用", value: "enabled" }), h(ElOption, { label: "停用", value: "disabled" })]
                  )
                ),
                h(ElFormItem, { label: "绑定排号" }, () =>
                  h(
                    ElTable,
                    {
                      data: availableRowTree.value,
                      ref: createRowsTableRef,
                      stripe: true,
                      border: true,
                      height: "260px",
                      rowKey: "node_key",
                      defaultExpandAll: true,
                      treeProps: {
                        children: "children",
                        checkStrictly: false
                      },
                      onSelectionChange: handleCreateRowSelectionChange
                    },
                    {
                      default: () => [
                        h(ElTableColumn, { type: "selection", width: 60, reserveSelection: true }),
                        h(ElTableColumn, {
                          label: "分区 / 排号",
                          minWidth: 240,
                          formatter: (row: LocationRowTreeNode) => (row.node_type === "zone" ? row.zone_name : row.row_name)
                        }),
                        h(ElTableColumn, {
                          prop: "slot_count",
                          label: "槽位数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        }),
                        h(ElTableColumn, {
                          prop: "device_count",
                          label: "设备数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        })
                      ]
                    }
                  )
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (createDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: creating.value, onClick: () => void submitCreatePlan() }, () => "确认创建")
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: startDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (startDialogVisible.value = value),
            title: startForm.plan_name ? `启动计划任务：${startForm.plan_name}` : "启动计划任务",
            width: "720px",
            closeOnClickModal: false
          },
          {
            default: () => [
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "分区-排" }, () =>
                  h(
                    ElTable,
                    {
                      data: startRowTree.value,
                      stripe: true,
                      border: true,
                      height: "260px",
                      rowKey: "node_key",
                      defaultExpandAll: true,
                      treeProps: {
                        children: "children"
                      }
                    },
                    {
                      default: () => [
                        h(ElTableColumn, {
                          label: "分区 / 排号",
                          minWidth: 240,
                          formatter: (row: LocationRowTreeNode) => (row.node_type === "zone" ? row.zone_name : row.row_name)
                        }),
                        h(ElTableColumn, {
                          prop: "slot_count",
                          label: "槽位数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        }),
                        h(ElTableColumn, {
                          prop: "device_count",
                          label: "设备数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        })
                      ]
                    }
                  )
                )
              ])
            ],
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (startDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: startingPlanID.value === startForm.plan_def_id, onClick: () => void submitStartPlan() }, () => "启动")
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: updateRowsDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (updateRowsDialogVisible.value = value),
            title: updateRowsForm.plan_name ? `变更选择：${updateRowsForm.plan_name}` : "变更选择",
            width: "720px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "分区-排" }, () =>
                  h(
                    ElTable,
                    {
                      data: availableRowTree.value,
                      ref: updateRowsTableRef,
                      stripe: true,
                      border: true,
                      height: "260px",
                      rowKey: "node_key",
                      defaultExpandAll: true,
                      treeProps: {
                        children: "children",
                        checkStrictly: false
                      },
                      onSelectionChange: handleUpdateRowSelectionChange
                    },
                    {
                      default: () => [
                        h(ElTableColumn, { type: "selection", width: 60, reserveSelection: true }),
                        h(ElTableColumn, {
                          label: "分区 / 排号",
                          minWidth: 240,
                          formatter: (row: LocationRowTreeNode) => (row.node_type === "zone" ? row.zone_name : row.row_name)
                        }),
                        h(ElTableColumn, {
                          prop: "slot_count",
                          label: "槽位数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        }),
                        h(ElTableColumn, {
                          prop: "device_count",
                          label: "设备数",
                          width: 100,
                          formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                        })
                      ]
                    }
                  )
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (updateRowsDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: mutatingDevices.value, onClick: () => void submitUpdateRows() }, () => "保存")
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: detailDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (detailDialogVisible.value = value),
            title: selectedPlan.value ? `计划任务详情：${selectedPlan.value.plan_name}` : "计划任务详情",
            width: "860px"
          },
          {
            default: () =>
              selectedPlan.value
                ? h(ElDescriptions, { column: 2, border: true }, () => [
                    h(ElDescriptionsItem, { label: "计划任务 ID" }, () => selectedPlan.value?.plan_def_id || ""),
                    h(ElDescriptionsItem, { label: "计划任务名称" }, () => selectedPlan.value?.plan_name || ""),
                    h(ElDescriptionsItem, { label: "目标类型" }, () => resolveTargetTypeLabel(selectedPlan.value?.target_type || "")),
                    h(ElDescriptionsItem, { label: "目标内容" }, () => resolvePlanTargetLabel(selectedPlan.value, workflows.value)),
                    h(ElDescriptionsItem, { label: "调度类型" }, () => resolveScheduleLabel(selectedPlan.value?.schedule_type || "")),
                    h(ElDescriptionsItem, { label: "计划状态" }, () => renderPlanStatus(selectedPlan.value?.status || "")),
                    h(ElDescriptionsItem, { label: "每日开始时间" }, () => selectedPlan.value?.daily_start_time || "未设置"),
                    h(ElDescriptionsItem, { label: "每日截止时间" }, () => selectedPlan.value?.daily_deadline_time || "未设置"),
                    h(ElDescriptionsItem, { label: "立即启动规则", span: 2 }, () =>
                      selectedPlan.value?.schedule_type === "daily"
                        ? "仅在当天开始时间之前允许手工立即启动；开始后由自动调度接管。"
                        : "一次性计划任务通过手工启动执行。"
                    ),
                    h(ElDescriptionsItem, { label: "创建时间" }, () => formatDateTime(selectedPlan.value?.created_at || "")),
                    h(ElDescriptionsItem, { label: "更新时间" }, () => formatDateTime(selectedPlan.value?.updated_at || "")),
                    h(ElDescriptionsItem, { label: "说明", span: 2 }, () => selectedPlan.value?.description || "暂无"),
                    h(ElDescriptionsItem, { label: "绑定排号", span: 2 }, () =>
                      (selectedPlan.value?.rows || []).length > 0
                        ? h(
                            ElTable,
                            {
                              data: detailRowTree.value,
                              stripe: true,
                              border: true,
                              height: "260px",
                              rowKey: "node_key",
                              defaultExpandAll: true,
                              treeProps: {
                                children: "children"
                              }
                            },
                            {
                              default: () => [
                                h(ElTableColumn, {
                                  label: "分区 / 排号",
                                  minWidth: 240,
                                  formatter: (row: LocationRowTreeNode) => (row.node_type === "zone" ? row.zone_name : row.row_name)
                                }),
                                h(ElTableColumn, {
                                  prop: "slot_count",
                                  label: "槽位数",
                                  width: 100,
                                  formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                                }),
                                h(ElTableColumn, {
                                  prop: "device_count",
                                  label: "设备数",
                                  width: 100,
                                  formatter: (_row: unknown, _column: unknown, value: number) => String(value || 0)
                                })
                              ]
                            }
                          )
                        : "暂无"
                    )
                  ])
                : null,
            footer: () => h(ElButton, { onClick: () => (detailDialogVisible.value = false) }, () => "关闭")
          }
        ),
        null
      ]);
  }
});


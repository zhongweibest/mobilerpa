// @ts-nocheck
import {
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
  ElMessage,
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
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { fetchDevices } from "../../api/devices";
import { useNoticesStore } from "../../stores/notices";
import { usePlansStore } from "../../stores/plans";
import { useScriptsStore } from "../../stores/scripts";
import { useWorkflowsStore } from "../../stores/workflows";
import type { DeviceRecord } from "../../types/device";
import type { PlanDefinitionRecord } from "../../types/plan";
import type { WorkflowDefinitionRecord } from "../../types/workflow";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];
const DEVICE_SELECTOR_PAGE_SIZE = 100;

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
    const detailDialogVisible = ref(false);
    const devicesDialogVisible = ref(false);
    const supportingDataWarning = ref("");
    const selectableDevices = ref<DeviceRecord[]>([]);
    const selectedPlan = ref<PlanDefinitionRecord | null>(null);

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
      device_ids: [] as string[]
    });

    const startForm = reactive({
      plan_def_id: "",
      plan_name: "",
      schedule_type: "once",
      device_ids: [] as string[]
    });

    const devicesForm = reactive({
      plan_def_id: "",
      plan_name: "",
      device_ids: [] as string[]
    });

    const availableScriptVersions = computed(() => {
      const selectedScript = scripts.value.find((item) => item.script_name === createForm.target_script_name);
      return selectedScript?.versions || [];
    });

    const onlineDevices = computed(() => selectableDevices.value.filter((item) => item.status === "online"));

    function getDeviceLabel(deviceID: string) {
      const matched = selectableDevices.value.find((item) => item.device_id === deviceID);
      if (!matched) {
        return deviceID;
      }
      return `${matched.device_name || matched.device_id} (${matched.device_id})`;
    }

    async function loadSelectableDevices() {
      const allItems: DeviceRecord[] = [];
      let nextPage = 1;
      let totalCount = 0;

      do {
        const result = await fetchDevices({
          page: nextPage,
          page_size: DEVICE_SELECTOR_PAGE_SIZE
        });
        totalCount = result.total;
        allItems.push(...result.items);
        nextPage += 1;
      } while (allItems.length < totalCount);

      selectableDevices.value = allItems;
      return selectableDevices.value;
    }

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
        await loadSelectableDevices();
      } catch (_error) {
        warnings.push("设备列表加载失败");
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
          noticesStore.error(`计划任务加载失败：${value}`, 5000);
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
      createForm.device_ids = [];
    }

    function openCreateDialog() {
      resetCreateForm();
      createDialogVisible.value = true;
    }

    function openStartDialog(planItem: PlanDefinitionRecord) {
      if (!isManualStartEnabled(planItem)) {
        ElMessage.warning(resolveStartDisabledReason(planItem));
        return;
      }
      startForm.plan_def_id = planItem.plan_def_id;
      startForm.plan_name = planItem.plan_name;
      startForm.schedule_type = planItem.schedule_type;
      startForm.device_ids = [...(planItem.device_ids || [])];
      startDialogVisible.value = true;
    }

    function openDetailDialog(planItem: PlanDefinitionRecord) {
      selectedPlan.value = planItem;
      detailDialogVisible.value = true;
    }

    function openDevicesDialog(planItem: PlanDefinitionRecord) {
      selectedPlan.value = planItem;
      devicesForm.plan_def_id = planItem.plan_def_id;
      devicesForm.plan_name = planItem.plan_name;
      devicesForm.device_ids = [...(planItem.device_ids || [])];
      devicesDialogVisible.value = true;
    }

    async function submitCreatePlan() {
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
          device_ids: createForm.device_ids
        });
        createDialogVisible.value = false;
        ElMessage.success("计划任务已创建");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "计划任务创建失败");
      }
    }

    async function submitStartPlan() {
      try {
        await plansStore.triggerStartPlan(startForm.plan_def_id, startForm.device_ids);
        startDialogVisible.value = false;
        ElMessage.success("计划任务已启动");
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "计划任务启动失败");
      }
    }

    async function submitDefinitionDevices() {
      try {
        await plansStore.updateDefinitionDevices(devicesForm.plan_def_id, devicesForm.device_ids);
        devicesDialogVisible.value = false;
        ElMessage.success("计划任务默认设备已更新");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "计划任务默认设备更新失败");
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
        ElMessage.success("计划任务已删除");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "计划任务删除失败");
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
                              prop: "device_ids",
                              label: "默认设备数",
                              width: 120,
                              formatter: (row: PlanDefinitionRecord) => String((row.device_ids || []).length)
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
                                minWidth: 320,
                                fixed: "right"
                              },
                              {
                                default: (scope: { row: PlanDefinitionRecord }) =>
                                  h("div", { class: "table-actions" }, [
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
                                      ElButton,
                                      {
                                        link: true,
                                        type: "success",
                                        onClick: () => openDetailDialog(scope.row)
                                      },
                                      () => "查看"
                                    ),
                                    h(
                                      ElButton,
                                      {
                                        link: true,
                                        type: "warning",
                                        loading: mutatingDevices.value && devicesForm.plan_def_id === scope.row.plan_def_id,
                                        onClick: () => openDevicesDialog(scope.row)
                                      },
                                      () => "修改设备"
                                    ),
                                    h(
                                      ElButton,
                                      {
                                        link: true,
                                        type: "danger",
                                        loading: deletingPlanID.value === scope.row.plan_def_id,
                                        onClick: () => void handleDeletePlan(scope.row)
                                      },
                                      () => "删除"
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
                h(ElFormItem, { label: "默认设备" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: createForm.device_ids,
                      "onUpdate:modelValue": (value: string[]) => (createForm.device_ids = value),
                      multiple: true,
                      collapseTags: true
                    },
                    () =>
                      onlineDevices.value.map((item) =>
                        h(ElOption, {
                          key: item.device_id,
                          label: `${item.device_name || item.device_id} (${item.device_id})`,
                          value: item.device_id
                        })
                      )
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
              h(ElAlert, {
                type: "info",
                showIcon: true,
                closable: false,
                class: "dialog-alert",
                title: "启动后会创建新的计划任务实例，并立即对选中设备触发脚本或工作流执行。"
              }),
              h(ElAlert, {
                type: "warning",
                showIcon: true,
                closable: false,
                class: "dialog-alert",
                title:
                  startForm.schedule_type === "daily"
                    ? "当前计划任务定义为按天循环。只有在当天开始时间之前允许手工立即启动；一旦到了开始时间，当天启动将由自动调度接管。"
                    : "当前计划任务定义为一次性（手工启动）。点击启动后会立刻创建并执行一个实例。"
              }),
              h(
                ElSelect,
                {
                  modelValue: startForm.device_ids,
                  "onUpdate:modelValue": (value: string[]) => (startForm.device_ids = value),
                  multiple: true,
                  collapseTags: true,
                  style: "width: 100%; margin-top: 16px;"
                },
                () =>
                  onlineDevices.value.map((item) =>
                    h(ElOption, {
                      key: item.device_id,
                      label: `${item.device_name || item.device_id} (${item.device_id})`,
                      value: item.device_id
                    })
                  )
              )
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
                    h(ElDescriptionsItem, { label: "默认设备", span: 2 }, () =>
                      (selectedPlan.value?.device_ids || []).length > 0
                        ? h(
                            "div",
                            { class: "stack-tags" },
                            (selectedPlan.value?.device_ids || []).map((deviceID) => h(ElTag, { key: deviceID }, () => getDeviceLabel(deviceID)))
                          )
                        : "暂无"
                    )
                  ])
                : null,
            footer: () => h(ElButton, { onClick: () => (detailDialogVisible.value = false) }, () => "关闭")
          }
        ),
        h(
          ElDialog,
          {
            modelValue: devicesDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (devicesDialogVisible.value = value),
            title: devicesForm.plan_name ? `修改默认设备：${devicesForm.plan_name}` : "修改默认设备",
            width: "720px",
            closeOnClickModal: false
          },
          {
            default: () => [
              h(ElAlert, {
                type: "info",
                showIcon: true,
                closable: false,
                class: "dialog-alert",
                title: "保存后会同步更新该计划任务的默认设备。如果当天实例已经启动且还未到截止时间，新追加设备会立即执行；否则在最近一次启动时生效。"
              }),
              h(ElForm, { labelPosition: "top", style: "margin-top: 16px;" }, () => [
                h(ElFormItem, { label: "默认设备" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: devicesForm.device_ids,
                      "onUpdate:modelValue": (value: string[]) => (devicesForm.device_ids = value),
                      multiple: true,
                      collapseTags: true,
                      style: "width: 100%;"
                    },
                    () =>
                      onlineDevices.value.map((item) =>
                        h(ElOption, {
                          key: item.device_id,
                          label: `${item.device_name || item.device_id} (${item.device_id})`,
                          value: item.device_id
                        })
                      )
                  )
                )
              ])
            ],
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (devicesDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: mutatingDevices.value, onClick: () => void submitDefinitionDevices() }, () => "保存")
              ])
          }
        )
      ]);
  }
});


// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDialog,
  ElEmpty,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, ref } from "vue";

import { useWorkflowsStore } from "../../stores/workflows";
import type { WorkflowEventRecord, WorkflowInstanceRecord, WorkflowRunRecord } from "../../types/workflow";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

function renderStatus(status: string) {
  let type: "success" | "danger" | "warning" | "info" = "info";
  if (status === "success") {
    type = "success";
  } else if (status === "failed" || status === "stopped") {
    type = "danger";
  } else if (status === "pending" || status === "running") {
    type = "warning";
  }
  return h(ElTag, { type, effect: "light" }, () => status || "unknown");
}

function getRunSortTimestamp(run: WorkflowRunRecord) {
  return new Date(run.updated_at || run.finished_at || run.started_at || run.created_at || 0).getTime();
}

function getInstanceSortTimestamp(instance: WorkflowInstanceRecord) {
  return new Date(instance.updated_at || instance.finished_at || instance.started_at || instance.created_at || 0).getTime();
}

function buildRunEventSummary(event: WorkflowEventRecord) {
  const extra = event.extra || {};
  const parts: string[] = [];
  if (typeof extra.step_name === "string" && extra.step_name.trim() !== "") {
    parts.push(`步骤：${String(extra.step_name)}`);
  }
  if (typeof extra.status === "string" && extra.status.trim() !== "") {
    parts.push(`状态：${String(extra.status)}`);
  }
  if (typeof extra.task_id === "string" && extra.task_id.trim() !== "") {
    parts.push(`任务：${String(extra.task_id)}`);
  }
  if (typeof extra.result_message === "string" && extra.result_message.trim() !== "") {
    parts.push(`结果：${String(extra.result_message)}`);
  }
  return parts.join(" / ");
}

function buildDeleteWorkflowInstanceErrorMessage(error: unknown) {
  if (error instanceof Error) {
    if (error.message.includes("工作流实例暂不允许删除")) {
      const details = (error as Error & { details?: Record<string, unknown> | null }).details;
      const reason = details && typeof details.reason === "string" ? details.reason.trim() : "";
      return reason || "当前工作流实例暂不允许删除。";
    }
    if (error.message.includes("workflow_instance_delete_not_allowed")) {
      return "只有执行成功或执行失败的工作流实例允许删除，运行中、待执行或已停止实例请勿删除。";
    }
    return error.message;
  }
  return "删除工作流实例失败";
}

export const WorkflowInstancesPage = defineComponent({
  name: "WorkflowInstancesPage",
  setup() {
    const workflowsStore = useWorkflowsStore();
    const { workflowInstances, selectedWorkflowEvents, loading, loadingRuns, loadingEvents, deletingWorkflowInstanceID, errorMessage } =
      storeToRefs(workflowsStore);

    const devicesDialogVisible = ref(false);
    const eventsDialogVisible = ref(false);
    const instancePage = ref(1);
    const instancePageSize = ref(10);
    const eventsFilterType = ref("");
    const selectedInstance = ref<WorkflowInstanceRecord | null>(null);
    const selectedDeviceRun = ref<WorkflowRunRecord | null>(null);

    const sortedInstances = computed(() =>
      [...workflowInstances.value].sort((left, right) => {
        const timeDiff = getInstanceSortTimestamp(right) - getInstanceSortTimestamp(left);
        if (timeDiff !== 0) {
          return timeDiff;
        }
        return right.workflow_instance_id.localeCompare(left.workflow_instance_id);
      })
    );

    const pagedInstances = computed(() => {
      const start = (instancePage.value - 1) * instancePageSize.value;
      return sortedInstances.value.slice(start, start + instancePageSize.value);
    });

    const instanceDeviceRuns = computed(() => {
      if (!selectedInstance.value) {
        return [];
      }
      return [...(selectedInstance.value.device_runs || [])].sort((left, right) => {
        const timeDiff = getRunSortTimestamp(right) - getRunSortTimestamp(left);
        if (timeDiff !== 0) {
          return timeDiff;
        }
        return right.workflow_run_id.localeCompare(left.workflow_run_id);
      });
    });

    const filteredWorkflowEvents = computed(() => {
      if (eventsFilterType.value.trim() === "") {
        return selectedWorkflowEvents.value;
      }
      return selectedWorkflowEvents.value.filter((item) => item.event_type === eventsFilterType.value.trim());
    });

    async function loadPage() {
      await workflowsStore.loadAllWorkflowInstances();
      if (selectedInstance.value) {
        selectedInstance.value =
          workflowInstances.value.find((item) => item.workflow_instance_id === selectedInstance.value?.workflow_instance_id) || null;
      }
    }

    onMounted(() => {
      void loadPage();
    });

    function getRunSummary(instance: WorkflowInstanceRecord) {
      return workflowsStore.summarizeRuns(instance.device_runs || []);
    }

    function openDevicesDialog(instance: WorkflowInstanceRecord) {
      selectedInstance.value = instance;
      devicesDialogVisible.value = true;
    }

    async function openEventsDialog(instance: WorkflowInstanceRecord, run: WorkflowRunRecord) {
      selectedInstance.value = instance;
      selectedDeviceRun.value = run;
      eventsFilterType.value = "";
      try {
        await workflowsStore.loadWorkflowEvents(instance.workflow_def_id, run.workflow_run_id);
        eventsDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("加载工作流设备事件失败，请稍后重试");
      }
    }

    async function refreshSelectedRunEvents() {
      if (!selectedInstance.value || !selectedDeviceRun.value) {
        return;
      }
      try {
        await workflowsStore.loadWorkflowEvents(selectedInstance.value.workflow_def_id, selectedDeviceRun.value.workflow_run_id);
        ElMessage.success("工作流设备事件已刷新");
      } catch (_error) {
        ElMessage.error("刷新工作流设备事件失败，请稍后重试");
      }
    }

    async function handleDeleteInstance(instance: WorkflowInstanceRecord) {
      try {
        await ElMessageBox.confirm(`确认删除工作流实例 ${instance.workflow_instance_id} 吗？删除后实例记录、设备运行记录和事件都会被移除。`, "删除工作流实例确认", {
          confirmButtonText: "确认删除",
          cancelButtonText: "取消",
          type: "warning"
        });
        await workflowsStore.removeWorkflowInstance(instance.workflow_def_id, instance.workflow_instance_id);
        if (selectedInstance.value?.workflow_instance_id === instance.workflow_instance_id) {
          devicesDialogVisible.value = false;
          selectedInstance.value = null;
        }
        ElMessage.success("工作流实例已删除");
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        ElMessage.error(buildDeleteWorkflowInstanceErrorMessage(error));
      }
    }

    return () =>
      h("section", { class: "workflows-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(
            ElButton,
            {
              loading: loading.value || loadingRuns.value,
              onClick: () => {
                void loadPage();
              }
            },
            () => "刷新"
          )
        ]),
        errorMessage.value ? h(ElAlert, { class: "page-alert", type: "error", title: `工作流实例操作失败：${errorMessage.value}`, showIcon: true, closable: false }) : null,
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            header: () =>
              h("div", { class: "card-header" }, [
                h("div", null, [
                  h("div", { class: "card-header__title" }, "实例列表"),
                  h("div", { class: "card-header__subtitle" }, "按实例维度查看当前设备运行情况。")
                ])
              ]),
            default: () =>
              sortedInstances.value.length === 0
                ? h(ElEmpty, { description: "当前还没有工作流实例。" })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: pagedInstances.value, stripe: true, class: "tasks-table", tableLayout: "fixed", height: "100%" },
                        {
                          default: () => [
                            h(ElTableColumn, { prop: "workflow_instance_id", label: "实例 ID", minWidth: 190 }),
                            h(ElTableColumn, { label: "工作流", minWidth: 220 }, { default: ({ row }) => `${row.workflow_name} / ${row.workflow_def_id}` }),
                            h(ElTableColumn, { label: "状态", width: 120 }, { default: ({ row }) => renderStatus(row.status) }),
                            h(ElTableColumn, {
                              label: "设备概览",
                              minWidth: 240,
                              formatter: (row) => {
                                const summary = getRunSummary(row);
                                return `总数 ${summary.total} / 运行中 ${summary.running} / 成功 ${summary.success} / 停止 ${summary.stopped} / 失败 ${summary.failed}`;
                              }
                            }),
                            h(ElTableColumn, { label: "开始时间", minWidth: 180, formatter: (row) => formatDateTime(row.started_at) }),
                            h(ElTableColumn, { label: "结束时间", minWidth: 180, formatter: (row) => formatDateTime(row.finished_at) }),
                            h(
                              ElTableColumn,
                              { label: "操作", minWidth: 220, fixed: "right" },
                              {
                                default: ({ row }) =>
                                  h("div", { class: "table-actions" }, [
                                    h(ElButton, { link: true, type: "primary", onClick: () => openDevicesDialog(row) }, () => "查看设备"),
                                    h(
                                      ElButton,
                                      {
                                        link: true,
                                        type: "danger",
                                        disabled: row.status !== "success" && row.status !== "failed",
                                        loading: deletingWorkflowInstanceID.value === row.workflow_instance_id,
                                        onClick: () => {
                                          void handleDeleteInstance(row);
                                        }
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
                        currentPage: instancePage.value,
                        pageSize: instancePageSize.value,
                        pageSizes: PAGE_SIZES,
                        total: sortedInstances.value.length,
                        layout: "total, sizes, prev, pager, next, jumper",
                        "onUpdate:currentPage": (value: number) => {
                          instancePage.value = value;
                        },
                        "onUpdate:pageSize": (value: number) => {
                          instancePageSize.value = value;
                          instancePage.value = 1;
                        }
                      })
                    )
                  ])
          }
        ),
        h(
          ElDialog,
          { modelValue: devicesDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (devicesDialogVisible.value = value), title: selectedInstance.value ? `实例设备列表：${selectedInstance.value.workflow_instance_id}` : "实例设备列表", width: "1100px", closeOnClickModal: true },
          {
            default: () =>
              h(
                "div",
                { class: "task-events-dialog-scroll" },
                instanceDeviceRuns.value.length === 0
                  ? h(ElEmpty, { description: "当前实例下还没有设备执行记录。" })
                  : h(
                      ElTable,
                      { data: instanceDeviceRuns.value, border: true, stripe: true, height: 520 },
                      {
                        default: () => [
                          h(ElTableColumn, { prop: "device_id", label: "设备 ID", minWidth: 160 }),
                          h(ElTableColumn, { prop: "workflow_run_id", label: "运行记录 ID", minWidth: 180 }),
                          h(ElTableColumn, { label: "状态", width: 120 }, { default: ({ row }) => renderStatus(row.status) }),
                          h(ElTableColumn, { label: "当前节点", minWidth: 140, formatter: (row) => row.current_node_id || "暂无" }),
                          h(ElTableColumn, { label: "最近更新时间", minWidth: 180, formatter: (row) => formatDateTime(row.updated_at) }),
                          h(
                            ElTableColumn,
                            { label: "操作", fixed: "right", width: 120 },
                            {
                              default: ({ row }) =>
                                selectedInstance.value
                                  ? h(ElButton, { link: true, type: "primary", onClick: () => void openEventsDialog(selectedInstance.value as WorkflowInstanceRecord, row) }, () => "查看事件")
                                  : null
                            }
                          )
                        ]
                      }
                    )
              )
          }
        ),
        h(
          ElDialog,
          { modelValue: eventsDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (eventsDialogVisible.value = value), title: selectedDeviceRun.value ? `设备事件：${selectedDeviceRun.value.device_id} / ${selectedDeviceRun.value.workflow_run_id}` : "设备事件", width: "880px", closeOnClickModal: true },
          {
            default: () =>
              loadingEvents.value
                ? h("div", { class: "workflow-runs-loading" }, "正在加载工作流设备事件...")
                : h("div", null, [
                    h("div", { class: "card-header", style: "margin-bottom: 12px;" }, [
                      h("div", null, [
                        h("div", { class: "card-header__title" }, "设备事件"),
                        h("div", { class: "card-header__subtitle" }, "支持按事件类型筛选，并可手动刷新当前设备运行记录。")
                      ]),
                      h("div", { class: "table-actions" }, [
                        h(
                          ElSelect,
                          {
                            modelValue: eventsFilterType.value,
                            placeholder: "筛选事件类型",
                            clearable: true,
                            style: "width: 220px",
                            "onUpdate:modelValue": (value: string) => {
                              eventsFilterType.value = value || "";
                            }
                          },
                          () =>
                            [...new Set(selectedWorkflowEvents.value.map((item) => item.event_type))]
                              .filter((item) => item.trim() !== "")
                              .map((item) => h(ElOption, { key: item, label: item, value: item }))
                        ),
                        h(ElButton, { onClick: () => void refreshSelectedRunEvents() }, () => "刷新")
                      ])
                    ]),
                    h(
                      "div",
                      { class: "task-events-dialog-scroll" },
                      filteredWorkflowEvents.value.length === 0
                        ? h(ElEmpty, { description: "当前设备执行还没有事件记录。" })
                        : h(
                            "div",
                            { class: "task-events-dialog" },
                            filteredWorkflowEvents.value.map((event: WorkflowEventRecord) =>
                              h(
                                ElCard,
                                { key: `${event.id}-${event.created_at}`, class: "task-events-dialog__card", shadow: "never" },
                                {
                                  default: () => [
                                    h("div", { class: "task-events-dialog__header" }, [
                                      h("div", { class: "task-events-dialog__title" }, `${event.event_type} / ${event.node_id || "无节点"}`),
                                      h("div", { class: "task-events-dialog__time" }, formatDateTime(event.created_at))
                                    ]),
                                    h("div", { class: "task-events-dialog__message" }, event.message || "暂无消息"),
                                    buildRunEventSummary(event) ? h("div", { class: "workflow-event-card__summary" }, buildRunEventSummary(event)) : null
                                  ]
                                }
                              )
                            )
                          )
                    )
                  ]),
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (eventsDialogVisible.value = false) }, () => "关闭")])
          }
        )
      ]);
  }
});

// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
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
import { defineComponent, h, onMounted, reactive, ref } from "vue";

import { useDevicesStore } from "../../stores/devices";
import { useScriptsStore } from "../../stores/scripts";
import { useTasksStore } from "../../stores/tasks";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

function parseParamsJSON(input: string): Record<string, unknown> {
  const trimmed = input.trim();
  if (trimmed === "") {
    return {};
  }
  return JSON.parse(trimmed) as Record<string, unknown>;
}

function summarizeBusyDevices(error: unknown): string {
  const details = typeof error === "object" && error !== null ? (error as { details?: Record<string, unknown> }).details : undefined;
  const busyDevices = Array.isArray(details?.busy_devices) ? (details.busy_devices as Array<Record<string, unknown>>) : [];
  if (busyDevices.length === 0) {
    return "";
  }

  return busyDevices
    .map((item) => {
      const deviceID = String(item.device_id || "");
      const occupancyType = String(item.occupancy_type || "");
      if (occupancyType === "manual_task") {
        return `${deviceID} 被手工任务 ${String(item.task_id || "")} 占用`;
      }
      if (occupancyType === "workflow") {
        return `${deviceID} 被工作流实例 ${String(item.workflow_instance_id || "")} 占用`;
      }
      if (occupancyType === "workflow_instance_history") {
        return `${deviceID} 已执行过当前工作流实例`;
      }
      return `${deviceID} 已被占用`;
    })
    .join("；");
}

function renderTaskStatusTag(status: string) {
  let type: "success" | "danger" | "warning" | "info" = "info";
  if (status === "success") {
    type = "success";
  } else if (status === "failed") {
    type = "danger";
  } else if (status === "assigned" || status === "running") {
    type = "warning";
  }

  return h(ElTag, { type, effect: "light" }, () => status || "unknown");
}

export const TasksPage = defineComponent({
  name: "TasksPage",
  setup() {
    const tasksStore = useTasksStore();
    const devicesStore = useDevicesStore();
    const scriptsStore = useScriptsStore();
    const { tasks, total, page, pageSize, selectedTaskID, selectedTaskEvents, loading, assigningTaskID, deletingTaskID, creating, errorMessage } =
      storeToRefs(tasksStore);
    const { devices } = storeToRefs(devicesStore);
    const { scripts } = storeToRefs(scriptsStore);

    const createDialogVisible = ref(false);
    const eventsDialogVisible = ref(false);

    const form = reactive({
      device_id: "",
      script_name: "shoppe_sync",
      script_version: "v0.1.1",
      priority: 1,
      params_json: '{\n  "shop_id": "shop-demo-001"\n}'
    });

    function resetCreateForm() {
      form.device_id = "";
      form.script_name = scripts.value[0]?.script_name || "shoppe_sync";
      form.script_version = scripts.value[0]?.versions[0]?.script_version || "v0.1.1";
      form.priority = 1;
      form.params_json = '{\n  "shop_id": "shop-demo-001"\n}';
    }

    async function loadPageData() {
      await Promise.all([tasksStore.loadTasks(), devicesStore.loadDevices(), scriptsStore.loadScripts()]);
      if (form.script_name === "" && scripts.value.length > 0) {
        resetCreateForm();
      }
    }

    onMounted(() => {
      void loadPageData();
    });

    function openCreateDialog() {
      resetCreateForm();
      createDialogVisible.value = true;
    }

    async function handleCreateTask() {
      try {
        await tasksStore.submitTask({
          device_id: form.device_id.trim(),
          script_name: form.script_name.trim(),
          script_version: form.script_version.trim(),
          priority: Number(form.priority || 0),
          params: parseParamsJSON(form.params_json)
        });
        createDialogVisible.value = false;
        ElMessage.success("任务创建成功");
      } catch (error) {
        const busySummary = summarizeBusyDevices(error);
        if (busySummary !== "") {
          ElMessage.error(`创建任务失败：${busySummary}`);
          return;
        }
        ElMessage.error("创建任务失败，请检查设备、脚本版本和参数 JSON");
      }
    }

    async function handleAssignTask(taskID: string) {
      try {
        await ElMessageBox.confirm(`确认要下发任务 ${taskID} 吗？`, "下发任务确认", {
          confirmButtonText: "确认下发",
          cancelButtonText: "取消",
          type: "warning"
        });
        await tasksStore.dispatchTask(taskID);
        ElMessage.success("任务下发成功");
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        const busySummary = summarizeBusyDevices(error);
        if (busySummary !== "") {
          ElMessage.error(`下发任务失败：${busySummary}`);
          return;
        }
        ElMessage.error("下发任务失败，请检查设备是否在线以及 Agent 是否已经连接");
      }
    }

    async function handleViewEvents(taskID: string) {
      try {
        await tasksStore.loadTaskEvents(taskID);
        eventsDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("加载任务事件失败，请稍后重试");
      }
    }

    async function handleDeleteTask(taskID: string) {
      try {
        await ElMessageBox.confirm(`确认删除任务 ${taskID} 吗？仅允许删除已结束的任务记录。`, "删除任务确认", {
          confirmButtonText: "确认删除",
          cancelButtonText: "取消",
          type: "warning"
        });
        await tasksStore.removeTask(taskID);
        ElMessage.success("任务已删除");
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        ElMessage.error(error instanceof Error ? error.message : "删除任务失败");
      }
    }

    return () =>
      h("section", { class: "tasks-page-shell" }, [
        h("div", { class: "page-toolbar" }, [
          h(ElButton, { type: "primary", onClick: openCreateDialog }, () => "创建任务"),
          h(
            ElButton,
            {
              loading: loading.value,
              onClick: () => {
                void tasksStore.loadTasks();
              }
            },
            () => "刷新"
          )
        ]),
        h("div", { class: "page-scroll-body" }, [
          errorMessage.value
            ? h(ElAlert, { class: "page-alert", type: "error", title: `任务操作失败：${errorMessage.value}`, showIcon: true, closable: false })
            : null,
          h(
            ElCard,
            { class: "page-card page-fill-card", shadow: "never" },
            {
              default: () =>
                tasks.value.length === 0
                  ? h(ElEmpty, { description: "当前还没有任务，请先通过“创建任务”新增任务。" })
                  : h("div", { class: "page-scroll-body" }, [
                      h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                        h(
                          ElTable,
                          { data: tasks.value, stripe: true, class: "tasks-table", tableLayout: "fixed", height: "100%" },
                          {
                            default: () => [
                              h(ElTableColumn, { prop: "task_id", label: "任务 ID", minWidth: 170 }),
                              h(ElTableColumn, { prop: "device_id", label: "设备", minWidth: 130 }),
                              h(ElTableColumn, { label: "脚本", minWidth: 210, formatter: (row) => `${row.script_name}@${row.script_version || "latest"}` }),
                              h(ElTableColumn, { label: "状态", width: 120 }, { default: ({ row }) => renderTaskStatusTag(row.status) }),
                              h(ElTableColumn, { label: "结果", minWidth: 240, formatter: (row) => row.result_message || row.result_code || "暂无" }),
                              h(ElTableColumn, { label: "更新时间", minWidth: 180, formatter: (row) => formatDateTime(row.updated_at) }),
                              h(
                                ElTableColumn,
                                { label: "操作", minWidth: 240, fixed: "right" },
                                {
                                  default: ({ row }) =>
                                    h("div", { class: "table-actions" }, [
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          type: "primary",
                                          disabled: row.status !== "pending" || assigningTaskID.value === row.task_id,
                                          onClick: () => {
                                            void handleAssignTask(row.task_id);
                                          }
                                        },
                                        () => (assigningTaskID.value === row.task_id ? "下发中..." : "下发任务")
                                      ),
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          type: "success",
                                          onClick: () => {
                                            void handleViewEvents(row.task_id);
                                          }
                                        },
                                        () => "查看事件"
                                      ),
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          type: "danger",
                                          loading: deletingTaskID.value === row.task_id,
                                          disabled: row.status === "pending" || row.status === "assigned" || row.status === "running",
                                          onClick: () => {
                                            void handleDeleteTask(row.task_id);
                                          }
                                        },
                                        () => (deletingTaskID.value === row.task_id ? "删除中..." : "删除")
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
                            void tasksStore.changePage(value);
                          },
                          "onUpdate:pageSize": (value: number) => {
                            void tasksStore.changePageSize(value);
                          }
                        })
                      )
                    ])
            }
          ),
          h(
            ElDialog,
            { modelValue: createDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (createDialogVisible.value = value), title: "创建任务", width: "640px", closeOnClickModal: false },
            {
              default: () =>
                h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                  h(
                    ElFormItem,
                    { label: "目标设备" },
                    () =>
                      h(
                        ElSelect,
                        {
                          modelValue: form.device_id,
                          "onUpdate:modelValue": (value: string) => {
                            form.device_id = value;
                          },
                          placeholder: "请选择设备"
                        },
                        () =>
                          devices.value.map((item) =>
                            h(ElOption, {
                              key: item.device_id,
                              label: `${item.device_id} / ${item.device_name || item.model || "未知设备"}`,
                              value: item.device_id
                            })
                          )
                      )
                  ),
                  h(
                    ElFormItem,
                    { label: "脚本名称" },
                    () =>
                      h(
                        ElSelect,
                        {
                          modelValue: form.script_name,
                          "onUpdate:modelValue": (value: string) => {
                            form.script_name = value;
                            const selectedScript = scripts.value.find((item) => item.script_name === value);
                            if (selectedScript && selectedScript.versions.length > 0) {
                              form.script_version = selectedScript.versions[0].script_version;
                            }
                          },
                          placeholder: "请选择脚本名称"
                        },
                        () => scripts.value.map((item) => h(ElOption, { key: item.script_name, label: item.script_name, value: item.script_name }))
                      )
                  ),
                  h(
                    ElFormItem,
                    { label: "脚本版本" },
                    () =>
                      h(
                        ElSelect,
                        {
                          modelValue: form.script_version,
                          "onUpdate:modelValue": (value: string) => {
                            form.script_version = value;
                          },
                          placeholder: "请选择脚本版本"
                        },
                        () =>
                          (scripts.value.find((item) => item.script_name === form.script_name)?.versions || []).map((item) =>
                            h(ElOption, {
                              key: `${item.script_name}-${item.script_version}`,
                              label: `${item.script_version} / ${item.source_type || "未知"} / ${item.storage_type}`,
                              value: item.script_version
                            })
                          )
                      )
                  ),
                  h(
                    ElFormItem,
                    { label: "优先级" },
                    () =>
                      h(ElInputNumber, {
                        modelValue: form.priority,
                        "onUpdate:modelValue": (value: number) => {
                          form.priority = value || 0;
                        },
                        min: 0,
                        max: 100,
                        controlsPosition: "right"
                      })
                  ),
                  h(
                    ElFormItem,
                    { label: "任务参数 JSON" },
                    () =>
                      h(ElInput, {
                        modelValue: form.params_json,
                        "onUpdate:modelValue": (value: string) => {
                          form.params_json = value;
                        },
                        type: "textarea",
                        rows: 8,
                        placeholder: "请输入任务参数 JSON"
                      })
                  )
                ]),
              footer: () =>
                h("div", { class: "dialog-footer" }, [
                  h(ElButton, { onClick: () => (createDialogVisible.value = false) }, () => "取消"),
                  h(
                    ElButton,
                    {
                      type: "primary",
                      loading: creating.value,
                      onClick: () => {
                        void handleCreateTask();
                      }
                    },
                    () => "确认创建"
                  )
                ])
            }
          ),
          h(
            ElDialog,
            { modelValue: eventsDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (eventsDialogVisible.value = value), title: selectedTaskID.value ? `任务事件：${selectedTaskID.value}` : "任务事件", width: "820px", closeOnClickModal: true },
            {
              default: () =>
                selectedTaskID.value
                  ? selectedTaskEvents.value.length === 0
                    ? h(ElEmpty, { description: "当前任务还没有事件记录。" })
                    : h("div", { class: "task-events-dialog-scroll" }, [
                        h(
                          "div",
                          { class: "task-events-dialog" },
                          selectedTaskEvents.value.map((event) =>
                            h(
                              ElCard,
                              { key: `${event.id}-${event.created_at}`, class: "task-events-dialog__card", shadow: "never" },
                              {
                                default: () => [
                                  h("div", { class: "task-events-dialog__header" }, [
                                    h("div", { class: "task-events-dialog__title" }, `${event.event_type} / ${event.task_status}`),
                                    h("div", { class: "task-events-dialog__time" }, formatDateTime(event.created_at))
                                  ]),
                                  h("div", { class: "task-events-dialog__message" }, event.message || "暂无消息"),
                                  h(
                                    ElDescriptions,
                                    { column: 2, size: "small", class: "task-events-dialog__descriptions" },
                                    () => [
                                      h(ElDescriptionsItem, { label: "步骤" }, () => event.step_name || "暂无"),
                                      h(ElDescriptionsItem, { label: "主题" }, () => event.topic || "暂无")
                                    ]
                                  )
                                ]
                              }
                            )
                          )
                        )
                      ])
                  : h(ElEmpty, { description: "暂无可展示的任务事件" }),
              footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (eventsDialogVisible.value = false) }, () => "关闭")])
            }
          )
        ])
      ]);
  }
});

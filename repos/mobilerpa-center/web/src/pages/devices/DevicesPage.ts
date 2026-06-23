// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElEmpty,
  ElMessage,
  ElMessageBox,
  ElPagination,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { defineComponent, h, onMounted, ref } from "vue";

import { fetchDeviceOccupancy, terminateManualTaskOnDevice } from "../../api/devices";
import { useDevicesStore } from "../../stores/devices";
import type { DeviceOccupancyDetail } from "../../types/device";
import { formatDateTime, getDeviceDisplayName, normalizeBindStatus, normalizeDeviceStatus } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

function renderStatusTag(status: string) {
  const normalized = normalizeDeviceStatus(status);
  if (normalized === "online") {
    return h("span", { class: "status-tag status-tag--online" }, "在线");
  }
  if (normalized === "offline") {
    return h("span", { class: "status-tag status-tag--offline" }, "离线");
  }
  return h("span", { class: "status-tag status-tag--unknown" }, "未知");
}

function renderBindTag(bindStatus: string) {
  const normalized = normalizeBindStatus(bindStatus);
  if (normalized === "bound") {
    return h("span", { class: "status-tag status-tag--bound" }, "已绑定");
  }
  if (normalized === "pending") {
    return h("span", { class: "status-tag status-tag--pending" }, "待绑定");
  }
  return h("span", { class: "status-tag status-tag--unknown" }, "未知");
}

function renderOccupancyTag(occupancy: DeviceOccupancyDetail["occupancy"]) {
  if (!occupancy) {
    return h("span", { class: "status-tag status-tag--bound" }, "空闲");
  }
  if (occupancy.occupancy_type === "plan") {
    return h("span", { class: "status-tag status-tag--pending" }, "计划任务占用");
  }
  if (occupancy.occupancy_type === "manual_task") {
    return h("span", { class: "status-tag status-tag--offline" }, "手工任务占用");
  }
  return h("span", { class: "status-tag status-tag--online" }, "工作流占用");
}

export const DevicesPage = defineComponent({
  name: "DevicesPage",
  setup() {
    const devicesStore = useDevicesStore();
    const { devices, total, page, pageSize, loading, deletingDeviceID, errorMessage } = storeToRefs(devicesStore);
    const occupancyDialogVisible = ref(false);
    const loadingOccupancy = ref(false);
    const terminatingTaskID = ref("");
    const selectedOccupancy = ref<DeviceOccupancyDetail | null>(null);

    onMounted(() => {
      void devicesStore.loadDevices();
    });

    async function handleDelete(deviceID: string) {
      try {
        await ElMessageBox.confirm(`确认删除设备 ${deviceID} 吗？仅允许删除离线设备。`, "删除设备确认", {
          confirmButtonText: "确认删除",
          cancelButtonText: "取消",
          type: "warning"
        });
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
      }

      try {
        await devicesStore.removeDevice(deviceID);
        ElMessage.success(`设备 ${deviceID} 已删除`);
      } catch (_error) {
        ElMessage.error("删除设备失败，请先确认该设备已经离线");
      }
    }

    async function handleViewTask(deviceID: string) {
      loadingOccupancy.value = true;
      try {
        selectedOccupancy.value = await fetchDeviceOccupancy(deviceID);
        occupancyDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("加载设备占用详情失败，请稍后重试");
      } finally {
        loadingOccupancy.value = false;
      }
    }

    async function handleTerminateManualTask() {
      const taskID = selectedOccupancy.value?.occupancy?.task_id || "";
      if (taskID === "") {
        return;
      }

      try {
        await ElMessageBox.confirm(`确认人工结束手工任务 ${taskID} 吗？`, "人工结束任务确认", {
          confirmButtonText: "确认结束",
          cancelButtonText: "取消",
          type: "warning"
        });
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
      }

      try {
        terminatingTaskID.value = taskID;
        await terminateManualTaskOnDevice(selectedOccupancy.value.device_id, taskID);
        if (selectedOccupancy.value) {
          selectedOccupancy.value = await fetchDeviceOccupancy(selectedOccupancy.value.device_id);
        }
        await devicesStore.loadDevices();
        ElMessage.success(`手工任务 ${taskID} 已结束`);
      } catch (_error) {
        ElMessage.error("人工结束手工任务失败，请检查中心服务日志");
      } finally {
        terminatingTaskID.value = "";
      }
    }

    return () =>
      h("section", { class: "devices-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(
            ElButton,
            {
              loading: loading.value,
              onClick: () => {
                void devicesStore.loadDevices();
              }
            },
            () => "刷新"
          )
        ]),
        errorMessage.value
          ? h(ElAlert, {
              class: "page-alert",
              type: "error",
              title: `设备列表加载失败：${errorMessage.value}`,
              showIcon: true,
              closable: false
            })
          : null,
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            header: () =>
              h("div", { class: "card-header" }, [
                h("div", null, [
                  h("div", { class: "card-header__title" }, "设备列表"),
                  h("div", { class: "card-header__subtitle" }, "集中查看设备 ID、在线状态、绑定状态、心跳、当前任务、物理位置和占用情况，并支持清理离线设备。")
                ])
              ]),
            default: () =>
              devices.value.length === 0 && !loading.value
                ? h(ElEmpty, { description: "当前还没有设备数据。" })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        {
                          data: devices.value,
                          stripe: true,
                          class: "tasks-table",
                          tableLayout: "auto",
                          height: "100%"
                        },
                          {
                            default: () => [
                              h(
                                ElTableColumn,
                                {
                                  label: "设备名称",
                                  minWidth: 180
                                },
                                {
                                  default: ({ row }) => h("div", { class: "devices-table__name" }, getDeviceDisplayName(row))
                                }
                              ),
                              h(ElTableColumn, {
                                prop: "device_id",
                                label: "设备 ID",
                                minWidth: 160
                              }),
                              h(
                                ElTableColumn,
                                {
                                  label: "在线状态",
                                  width: 110
                                },
                                {
                                  default: ({ row }) => renderStatusTag(row.status)
                                }
                              ),
                              h(
                                ElTableColumn,
                                {
                                  label: "绑定状态",
                                  width: 110
                                },
                                {
                                  default: ({ row }) => renderBindTag(row.bind_status)
                                }
                              ),
                              h(ElTableColumn, {
                                label: "Agent UUID",
                                minWidth: 180,
                                formatter: (row) => (row.agent_uuid?.trim() ? row.agent_uuid : "暂无")
                              }),
                              h(ElTableColumn, {
                                label: "最近心跳",
                                minWidth: 160,
                                formatter: (row) => formatDateTime(row.last_heartbeat_at)
                              }),
                              h(ElTableColumn, {
                                label: "当前任务",
                                minWidth: 140,
                                formatter: (row) => (row.current_task_id?.trim() ? row.current_task_id : "暂无")
                              }),
                              h(
                                ElTableColumn,
                                {
                                  label: "占用状态",
                                  minWidth: 140
                                },
                                {
                                  default: ({ row }) => renderOccupancyTag(row.occupancy)
                                }
                              ),
                              h(ElTableColumn, {
                                label: "物理位置",
                                minWidth: 120,
                                formatter: (row) => (row.physical_slot?.trim() ? row.physical_slot : "未录入")
                              }),
                              h(
                                ElTableColumn,
                                {
                                  label: "操作",
                                  minWidth: 220
                                },
                                {
                                  default: ({ row }) =>
                                    h("div", { class: "table-actions" }, [
                                      h(
                                        ElButton,
                                        {
                                          size: "small",
                                          type: "primary",
                                          plain: true,
                                          loading: loadingOccupancy.value && selectedOccupancy.value?.device_id === row.device_id,
                                          onClick: () => {
                                            void handleViewTask(row.device_id);
                                          }
                                        },
                                        () => "查看任务占用"
                                      ),
                                      h(
                                        ElButton,
                                        {
                                          size: "small",
                                          type: "danger",
                                          plain: true,
                                          disabled: row.status === "online" || deletingDeviceID.value === row.device_id,
                                          onClick: () => {
                                            void handleDelete(row.device_id);
                                          }
                                        },
                                        () => (deletingDeviceID.value === row.device_id ? "删除中..." : "删除")
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
                          void devicesStore.changePage(value);
                        },
                        "onUpdate:pageSize": (value: number) => {
                          void devicesStore.changePageSize(value);
                        }
                      })
                    )
                  ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: occupancyDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              occupancyDialogVisible.value = value;
            },
            title: selectedOccupancy.value ? `设备占用详情：${selectedOccupancy.value.device_id}` : "设备占用详情",
            width: "820px"
          },
          {
            default: () => {
              if (!selectedOccupancy.value) {
                return h("div", "暂无数据");
              }
              const occupancy = selectedOccupancy.value.occupancy;
              return h("div", { class: "device-occupancy-panel" }, [
                h("div", { class: "task-events-dialog__message" }, occupancy ? occupancy.message || "当前存在占用" : "当前没有任务或工作流占用该设备"),
                h(
                  ElDescriptions,
                  {
                    border: true,
                    column: 2,
                    class: "task-events-dialog__descriptions"
                  },
                  () => [
                    h(ElDescriptionsItem, { label: "设备 ID" }, () => selectedOccupancy.value?.device_id || "暂无"),
                    h(ElDescriptionsItem, { label: "设备状态" }, () => selectedOccupancy.value?.device_status || "暂无"),
                    h(ElDescriptionsItem, { label: "占用类型" }, () => occupancy?.occupancy_type || "暂无"),
                    h(ElDescriptionsItem, { label: "任务状态" }, () => occupancy?.task_status || "暂无"),
                    h(ElDescriptionsItem, { label: "任务 ID" }, () => occupancy?.task_id || "暂无"),
                    h(ElDescriptionsItem, { label: "工作流实例" }, () => occupancy?.workflow_instance_id || "暂无"),
                    h(ElDescriptionsItem, { label: "运行记录" }, () => occupancy?.workflow_run_id || "暂无"),
                    h(ElDescriptionsItem, { label: "current_task_id" }, () => selectedOccupancy.value?.current_task_id || "暂无"),
                    h(ElDescriptionsItem, { label: "current_step" }, () => selectedOccupancy.value?.current_step || "暂无"),
                    h(ElDescriptionsItem, { label: "last_error" }, () => selectedOccupancy.value?.last_error || "暂无")
                  ]
                )
              ]);
            },
            footer: () => {
              const occupancy = selectedOccupancy.value?.occupancy;
              const canTerminate = occupancy?.occupancy_type === "manual_task" && (occupancy.task_status === "assigned" || occupancy.task_status === "running");
              return h("div", { class: "dialog-footer" }, [
                canTerminate
                  ? h(
                      ElButton,
                      {
                        type: "danger",
                        loading: terminatingTaskID.value === occupancy?.task_id,
                        onClick: () => {
                          void handleTerminateManualTask();
                        }
                      },
                      () => "人工结束手工任务"
                    )
                  : h(ElTag, { type: "info" }, () => "当前无可人工结束的手工任务"),
                h(ElButton, { onClick: () => (occupancyDialogVisible.value = false) }, () => "关闭")
              ]);
            }
          }
        )
      ]);
  }
});

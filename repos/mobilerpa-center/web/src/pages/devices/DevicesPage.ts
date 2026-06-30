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
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElMessageBox
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";

import { bindLocationNode, fetchDeviceOccupancy, fetchLocationNodes } from "../../api/devices";
import { useDevicesStore } from "../../stores/devices";
import type { DeviceOccupancyDetail, DeviceRecord, LocationNodeRecord } from "../../types/device";
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
  return h("span", { class: "status-tag status-tag--online" }, "运行中占用");
}

function buildSlotLabel(slot?: LocationNodeRecord | null) {
  if (!slot) {
    return "";
  }
  return slot.path_text || [slot.zone_name, slot.row_name, slot.slot_name].map((item) => (item || "").trim()).filter((item) => item !== "").join("-");
}

export const DevicesPage = defineComponent({
  name: "DevicesPage",
  setup() {
    const devicesStore = useDevicesStore();
    const { devices, total, page, pageSize, loading, deletingDeviceID, errorMessage } = storeToRefs(devicesStore);
    const occupancyDialogVisible = ref(false);
    const loadingOccupancy = ref(false);
    const selectedOccupancy = ref<DeviceOccupancyDetail | null>(null);
    const bindDialogVisible = ref(false);
    const bindingDevice = ref<DeviceRecord | null>(null);
    const bindingNodes = ref<LocationNodeRecord[]>([]);
    const loadingSlots = ref(false);
    const binding = ref(false);
    const bindForm = reactive({
      slot_zone: "",
      slot_row: "",
      slot_id: ""
    });

    const availableZones = computed(() =>
      bindingNodes.value.filter((item) => item.node_type === "zone")
    );
    const availableRows = computed(() =>
      bindingNodes.value.filter((item) => item.node_type === "row" && item.parent_id === bindForm.slot_zone.trim())
    );
    const availableSlots = computed(() =>
      bindingNodes.value.filter((item) => item.node_type === "slot" && item.parent_id === bindForm.slot_row.trim())
    );

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

    async function handleOpenBindDialog(device: DeviceRecord) {
      bindingDevice.value = device;
      bindForm.slot_zone = device.slot_zone || "";
      bindForm.slot_row = device.slot_row || "";
      bindForm.slot_id = "";
      loadingSlots.value = true;
      try {
        bindingNodes.value = await fetchLocationNodes();
        const currentSlot = bindingNodes.value.find((item) => item.node_type === "slot" && item.device_id === device.device_id);
        if (currentSlot) {
          bindForm.slot_zone = currentSlot.parent_id ? bindingNodes.value.find((item) => item.node_id === bindingNodes.value.find((row) => row.node_id === currentSlot.parent_id)?.parent_id)?.node_id || "" : "";
          bindForm.slot_row = currentSlot.parent_id || "";
          bindForm.slot_id = currentSlot.node_id;
        }
        bindDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("加载槽位列表失败，请先在设备绑定页创建分区、排号和槽位");
      } finally {
        loadingSlots.value = false;
      }
    }

    async function handleBindDevice() {
      if (!bindingDevice.value) {
        return;
      }
      if (bindForm.slot_id.trim() === "") {
        ElMessage.warning("请先选择槽位");
        return;
      }
      binding.value = true;
      try {
        await bindLocationNode(bindForm.slot_id.trim(), {
          device_id: bindingDevice.value.device_id
        });
        bindDialogVisible.value = false;
        ElMessage.success("设备绑定成功");
        await devicesStore.loadDevices();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "设备绑定失败");
      } finally {
        binding.value = false;
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
                                          type: "success",
                                          plain: true,
                                          onClick: () => {
                                            void handleOpenBindDialog(row);
                                          }
                                        },
                                        () => "绑定"
                                      ),
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
                    h(ElDescriptionsItem, { label: "current_task_id" }, () => selectedOccupancy.value?.current_task_id || "暂无"),
                    h(ElDescriptionsItem, { label: "current_step" }, () => selectedOccupancy.value?.current_step || "暂无"),
                    h(ElDescriptionsItem, { label: "last_error" }, () => selectedOccupancy.value?.last_error || "暂无")
                  ]
                )
              ]);
            },
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (occupancyDialogVisible.value = false) }, () => "关闭")])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: bindDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              bindDialogVisible.value = value;
            },
            title: bindingDevice.value ? `绑定设备：${getDeviceDisplayName(bindingDevice.value)}` : "绑定设备",
            width: "560px"
          },
          {
            default: () =>
              h("div", { class: "device-occupancy-panel" }, [
                h(ElDescriptions, { border: true, column: 1 }, () => [
                  h(ElDescriptionsItem, { label: "设备 ID" }, () => bindingDevice.value?.device_id || "暂无"),
                  h(ElDescriptionsItem, { label: "当前展示位置" }, () => bindingDevice.value?.physical_slot || "未录入")
                ]),
                h(
                  ElSelect,
                  {
                    modelValue: bindForm.slot_zone,
                    filterable: true,
                    placeholder: "请选择分区",
                    loading: loadingSlots.value,
                    "onUpdate:modelValue": (value: string) => {
                      bindForm.slot_zone = value;
                      bindForm.slot_row = "";
                      bindForm.slot_id = "";
                    }
                  },
                    () => availableZones.value.map((item) => h(ElOption, { key: item.node_id, label: item.node_name, value: item.node_id }))
                ),
                h(
                  ElSelect,
                  {
                    modelValue: bindForm.slot_row,
                    filterable: true,
                    placeholder: "请选择排号",
                    disabled: bindForm.slot_zone.trim() === "",
                    "onUpdate:modelValue": (value: string) => {
                      bindForm.slot_row = value;
                      bindForm.slot_id = "";
                    }
                  },
                  () => availableRows.value.map((item) => h(ElOption, { key: item.node_id, label: item.node_name, value: item.node_id }))
                ),
                h(
                  ElSelect,
                  {
                    modelValue: bindForm.slot_id,
                    filterable: true,
                    placeholder: "请选择槽位",
                    disabled: bindForm.slot_row.trim() === "",
                    "onUpdate:modelValue": (value: string) => {
                      bindForm.slot_id = value;
                    }
                  },
                  () =>
                    availableSlots.value.map((item) =>
                      h(ElOption, {
                        key: item.node_id,
                        label: item.device_id && item.device_id !== bindingDevice.value?.device_id ? `${buildSlotLabel(item)}（已绑定 ${item.device_id}）` : buildSlotLabel(item),
                        value: item.node_id,
                        disabled: !!item.device_id && item.device_id !== bindingDevice.value?.device_id
                      })
                    )
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (bindDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: binding.value, onClick: () => void handleBindDevice() }, () => "确认绑定")
              ])
          }
        )
      ]);
  }
});

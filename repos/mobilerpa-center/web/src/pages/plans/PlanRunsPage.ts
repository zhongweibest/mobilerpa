// @ts-nocheck
import {
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
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { fetchDevices } from "../../api/devices";
import { useNoticesStore } from "../../stores/notices";
import { usePlansStore } from "../../stores/plans";
import type { DeviceRecord } from "../../types/device";
import type { PlanDeviceRunRecord, PlanRunRecord } from "../../types/plan";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];
const DEVICE_SELECTOR_PAGE_SIZE = 100;

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

function getTargetTypeLabel(value: string) {
  return value === "workflow" ? "工作流" : "脚本";
}

export const PlanRunsPage = defineComponent({
  name: "PlanRunsPage",
  setup() {
    const plansStore = usePlansStore();
    const noticesStore = useNoticesStore();
    const { runs, runsTotal, runsPage, runsPageSize, selectedEvents, loadingRuns, loadingEvents, stoppingRunID, mutatingDevices, errorMessage } =
      storeToRefs(plansStore);

    const eventsDialogVisible = ref(false);
    const devicesDialogVisible = ref(false);
    const appendDialogVisible = ref(false);
    const selectedRun = ref<PlanRunRecord | null>(null);
    const selectableDevices = ref<DeviceRecord[]>([]);
    const supportingDataWarning = ref("");
    const appendForm = reactive({
      device_ids: [] as string[]
    });
    const deviceEventFilter = ref("");
    const displayedEvents = computed(() => {
      if (!deviceEventFilter.value) {
        return selectedEvents.value;
      }
      return selectedEvents.value.filter((item) => item.device_id === deviceEventFilter.value);
    });

    const appendableDevices = computed(() => {
      if (!selectedRun.value) {
        return [];
      }
      const existing = new Set((selectedRun.value.device_runs || []).map((item) => item.device_id));
      return selectableDevices.value.filter((item) => item.status === "online" && !existing.has(item.device_id));
    });

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
    }

    async function loadPageData() {
      await plansStore.loadRuns();
      try {
        await loadSelectableDevices();
        supportingDataWarning.value = "";
      } catch (_error) {
        supportingDataWarning.value = "设备列表加载失败";
      }
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(
      supportingDataWarning,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.warning(`计划任务实例辅助数据加载不完整：${value}`, 5000);
        }
      }
    );

    watch(
      errorMessage,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.error(`计划任务实例加载失败：${value}`, 5000);
        }
      }
    );

    async function openDeviceEventsDialog(deviceRun: PlanDeviceRunRecord) {
      if (!selectedRun.value) {
        return;
      }
      try {
        deviceEventFilter.value = deviceRun.device_id;
        await plansStore.loadPlanEvents(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id);
        eventsDialogVisible.value = true;
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "设备事件加载失败");
      }
    }

    function openDevicesDialog(item: PlanRunRecord) {
      selectedRun.value = item;
      devicesDialogVisible.value = true;
    }

    function openAppendDialog(item: PlanRunRecord) {
      selectedRun.value = item;
      appendForm.device_ids = [];
      appendDialogVisible.value = true;
    }

    async function stopCurrentRun(item: PlanRunRecord) {
      try {
        await plansStore.triggerStopPlanRun(item.plan_def_id, item.plan_run_id);
        ElMessage.success("计划任务实例已停止");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "计划任务实例停止失败");
      }
    }

    async function appendDevices() {
      if (!selectedRun.value) {
        return;
      }
      try {
        await plansStore.appendPlanDevices(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, appendForm.device_ids);
        appendDialogVisible.value = false;
        ElMessage.success("设备已追加到计划任务实例");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "追加设备失败");
      }
    }

    async function removeDevice(item: PlanDeviceRunRecord) {
      if (!selectedRun.value) {
        return;
      }
      try {
        await plansStore.removeDeviceFromPlan(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, item.device_id);
        ElMessage.success("设备已从计划任务实例移除");
        await loadPageData();
      } catch (error) {
        ElMessage.error(error instanceof Error ? error.message : "移除设备失败");
      }
    }

    async function stopOrRemoveDevice(item: PlanDeviceRunRecord) {
      const isActive = item.status === "pending" || item.status === "running";
      if (isActive) {
        try {
          await ElMessageBox.confirm(`确认停止设备 ${item.device_id} 当前的计划任务执行吗？`, "停止设备确认", {
            confirmButtonText: "确认停止",
            cancelButtonText: "取消",
            type: "warning"
          });
        } catch (error) {
          if (error === "cancel" || error === "close") {
            return;
          }
        }
      }
      await removeDevice(item);
    }

    return () =>
      h("section", { class: "app-page" }, [
        h("div", { class: "page-toolbar" }, [h(ElButton, { loading: loadingRuns.value, onClick: () => void loadPageData() }, () => "刷新")]),
        h("section", { class: "app-page__panel" }, [
          h(ElCard, { class: "page-card", shadow: "never" }, {
            default: () => [
              runs.value.length === 0 && !loadingRuns.value
                ? h(ElEmpty, { description: "暂无计划任务实例" })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: runs.value, stripe: true, border: false, class: "app-table", height: "100%" },
                        {
                          default: () => [
                            h(ElTableColumn, { prop: "plan_run_id", label: "实例ID", minWidth: 140 }),
                            h(ElTableColumn, { prop: "plan_name", label: "计划任务名称", minWidth: 180 }),
                            h(ElTableColumn, {
                              prop: "target_type",
                              label: "目标类型",
                              width: 120,
                              formatter: (_row: unknown, _column: unknown, value: string) => getTargetTypeLabel(value)
                            }),
                            h(ElTableColumn, {
                              label: "设备数",
                              width: 120,
                              formatter: (row: PlanRunRecord) => String((row.device_runs || []).length)
                            }),
                            h(
                              ElTableColumn,
                              { label: "状态", width: 120 },
                              {
                                default: (scope: { row: PlanRunRecord }) => renderStatus(scope.row.status)
                              }
                            ),
                            h(ElTableColumn, {
                              prop: "started_at",
                              label: "开始时间",
                              minWidth: 180,
                              formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                            }),
                            h(ElTableColumn, {
                              prop: "finished_at",
                              label: "结束时间",
                              minWidth: 180,
                              formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                            }),
                            h(
                              ElTableColumn,
                              { label: "操作", width: 260, fixed: "right" },
                              {
                                default: (scope: { row: PlanRunRecord }) =>
                                  h("div", { class: "table-actions" }, [
                                    h(ElButton, { type: "primary", link: true, onClick: () => openDevicesDialog(scope.row) }, () => "查看执行设备"),
                                    h(ElButton, { type: "primary", link: true, onClick: () => openAppendDialog(scope.row) }, () => "追加设备"),
                                    h(
                                      ElButton,
                                      {
                                        type: "danger",
                                        link: true,
                                        loading: stoppingRunID.value === scope.row.plan_run_id,
                                        onClick: () => void stopCurrentRun(scope.row)
                                      },
                                      () => "停止"
                                    )
                                  ])
                              }
                            )
                          ]
                        }
                      )
                    ]),
                    h("div", { class: "page-pagination" }, [
                      h(ElPagination, {
                        currentPage: runsPage.value,
                        pageSize: runsPageSize.value,
                        pageSizes: PAGE_SIZES,
                        total: runsTotal.value,
                        layout: "total, sizes, prev, pager, next, jumper",
                        onSizeChange: (value: number) => void plansStore.changeRunsPageSize(value),
                        onCurrentChange: (value: number) => void plansStore.changeRunsPage(value)
                      })
                    ])
                  ])
            ]
          })
        ]),
        h(
          ElDialog,
          {
            modelValue: devicesDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (devicesDialogVisible.value = value),
            title: selectedRun.value ? `实例设备列表：${selectedRun.value.plan_run_id}` : "实例设备列表",
            width: "980px"
          },
          {
            default: () =>
              h(
                ElTable,
                { data: selectedRun.value?.device_runs || [], stripe: true, border: false, class: "app-table", height: "520px" },
                {
                  default: () => [
                    h(ElTableColumn, { prop: "device_id", label: "设备ID", minWidth: 140 }),
                    h(ElTableColumn, {
                      prop: "target_type",
                      label: "目标类型",
                      width: 120,
                      formatter: (_row: unknown, _column: unknown, value: string) => getTargetTypeLabel(value)
                    }),
                    h(
                      ElTableColumn,
                      { label: "状态", width: 120 },
                      {
                        default: (scope: { row: PlanDeviceRunRecord }) => renderStatus(scope.row.status)
                      }
                    ),
                    h(ElTableColumn, {
                      prop: "started_at",
                      label: "开始时间",
                      minWidth: 180,
                      formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                    }),
                    h(ElTableColumn, {
                      prop: "finished_at",
                      label: "结束时间",
                      minWidth: 180,
                      formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                    }),
                    h(ElTableColumn, { prop: "last_error", label: "最后错误", minWidth: 220 }),
                    h(
                      ElTableColumn,
                      { label: "操作", width: 200, fixed: "right" },
                      {
                        default: (scope: { row: PlanDeviceRunRecord }) =>
                          h("div", { class: "table-actions" }, [
                            h(ElButton, { type: "primary", link: true, onClick: () => void openDeviceEventsDialog(scope.row) }, () => "查看事件"),
                            h(
                              ElButton,
                              {
                                type: "danger",
                                link: true,
                                loading: mutatingDevices.value,
                                onClick: () => void stopOrRemoveDevice(scope.row)
                              },
                              () => (scope.row.status === "pending" || scope.row.status === "running" ? "停止设备" : "移除设备")
                            )
                          ])
                      }
                    )
                  ]
                }
              ),
            footer: () => h(ElButton, { onClick: () => (devicesDialogVisible.value = false) }, () => "关闭")
          }
        ),
        h(
          ElDialog,
          {
            modelValue: eventsDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (eventsDialogVisible.value = value),
            title: selectedRun.value
              ? deviceEventFilter.value
                ? `设备 ${deviceEventFilter.value} 事件：${selectedRun.value.plan_run_id}`
                : `计划任务事件：${selectedRun.value.plan_run_id}`
              : "计划任务事件",
            width: "980px"
          },
          {
            default: () =>
              loadingEvents.value
                ? h("div", { class: "dialog-loading" }, "加载中...")
                : h(
                    ElTable,
                    { data: displayedEvents.value, stripe: true, border: false, class: "app-table", height: "520px" },
                    {
                      default: () => [
                        h(ElTableColumn, { prop: "id", label: "ID", width: 90 }),
                        h(ElTableColumn, { prop: "device_id", label: "设备ID", minWidth: 140 }),
                        h(ElTableColumn, { prop: "event_type", label: "事件类型", minWidth: 180 }),
                        h(ElTableColumn, { prop: "message", label: "事件内容", minWidth: 260 }),
                        h(ElTableColumn, {
                          prop: "created_at",
                          label: "时间",
                          minWidth: 180,
                          formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                        })
                      ]
                    }
                  ),
            footer: () => h(ElButton, { onClick: () => (eventsDialogVisible.value = false) }, () => "关闭")
          }
        ),
        h(
          ElDialog,
          {
            modelValue: appendDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (appendDialogVisible.value = value),
            title: selectedRun.value ? `追加设备：${selectedRun.value.plan_run_id}` : "追加设备",
            width: "720px"
          },
          {
            default: () =>
              h(
                ElSelect,
                {
                  modelValue: appendForm.device_ids,
                  "onUpdate:modelValue": (value: string[]) => (appendForm.device_ids = value),
                  multiple: true,
                  collapseTags: true,
                  style: "width: 100%;"
                },
                () =>
                  appendableDevices.value.map((item) =>
                    h(ElOption, {
                      key: item.device_id,
                      label: `${item.device_name || item.device_id} (${item.device_id})`,
                      value: item.device_id
                    })
                  )
              ),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (appendDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: mutatingDevices.value, onClick: () => void appendDevices() }, () => "确认追加")
              ])
          }
        )
      ]);
  }
});

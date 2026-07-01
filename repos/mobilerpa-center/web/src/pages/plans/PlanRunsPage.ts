// @ts-nocheck
import {
  ElButton,
  ElCard,
  ElDialog,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElEmpty,
  ElMessageBox,
  ElPagination,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, nextTick, onMounted, reactive, ref, watch } from "vue";

import { fetchLocationNodes } from "../../api/devices";
import { useNoticesStore } from "../../stores/notices";
import { usePlansStore } from "../../stores/plans";
import type { LocationNodeRecord } from "../../types/device";
import type { PlanDeviceRunRecord, PlanRunRecord, PlanRowBinding } from "../../types/plan";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

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

type RunDeviceTreeNode = {
  node_key: string;
  node_type: "zone" | "row" | "slot" | "device";
  zone_id: string;
  zone_name: string;
  row_id: string;
  row_name: string;
  slot_id: string;
  slot_name: string;
  device_id: string;
  device_name: string;
  status: string;
  children?: RunDeviceTreeNode[];
};

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

function buildLocationRowTree(nodes: LocationNodeRecord[]): LocationRowTreeNode[] {
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

  const tree = new Map<string, LocationRowTreeNode>();
  for (const row of rowMap.values()) {
    const zoneID = row.parent_id;
    const zoneName = zoneMap.get(zoneID)?.node_name || zoneID;
    const slots = slotMap.get(row.node_id) || [];
    const zoneNode =
      tree.get(zoneID) ||
      {
        node_key: `zone:${zoneID}`,
        node_type: "zone" as const,
        zone_id: zoneID,
        zone_name: zoneName,
        row_id: "",
        row_name: "",
        slot_count: 0,
        device_count: 0,
        children: []
      };

    const rowNode = {
      node_key: `row:${zoneID}:${row.node_id}`,
      node_type: "row" as const,
      zone_id: zoneID,
      zone_name: zoneName,
      row_id: row.node_id,
      row_name: row.node_name || row.node_id,
      slot_count: slots.length,
      device_count: slots.filter((slot) => (slot.device_id || "").trim() !== "").length
    };

    zoneNode.children?.push(rowNode);
    zoneNode.slot_count += rowNode.slot_count;
    zoneNode.device_count += rowNode.device_count;
    tree.set(zoneID, zoneNode);
  }

  return Array.from(tree.values());
}

function buildRunRowBindings(run: PlanRunRecord | null): PlanRowBinding[] {
  if (!run) {
    return [];
  }
  const uniqueRows = new Map<string, PlanRowBinding>();
  for (const item of run.device_runs || []) {
    const key = `${item.zone_id}:${item.row_id}`;
    if (!uniqueRows.has(key)) {
      uniqueRows.set(key, {
        zone_id: item.zone_id,
        row_id: item.row_id
      });
    }
  }
  return Array.from(uniqueRows.values());
}

function buildRunDeviceTree(run: PlanRunRecord | null, nodes: LocationNodeRecord[]): RunDeviceTreeNode[] {
  if (!run) {
    return [];
  }

  const zoneMap = new Map<string, LocationNodeRecord>();
  const rowMap = new Map<string, LocationNodeRecord>();
  const slotMap = new Map<string, LocationNodeRecord>();

  for (const node of nodes) {
    if (node.node_type === "zone") {
      zoneMap.set(node.node_id, node);
    } else if (node.node_type === "row") {
      rowMap.set(node.node_id, node);
    } else if (node.node_type === "slot") {
      slotMap.set(node.node_id, node);
    }
  }

  const zoneTree = new Map<string, RunDeviceTreeNode>();
  for (const item of run.device_runs || []) {
    const zoneNode =
      zoneTree.get(item.zone_id) ||
      {
        node_key: `zone:${item.zone_id}`,
        node_type: "zone" as const,
        zone_id: item.zone_id,
        zone_name: zoneMap.get(item.zone_id)?.node_name || item.zone_id,
        row_id: "",
        row_name: "",
        slot_id: "",
        slot_name: "",
        device_id: "",
        device_name: "",
        status: "",
        children: []
      };

    let rowNode = (zoneNode.children || []).find((child) => child.node_key === `row:${item.zone_id}:${item.row_id}`);
    if (!rowNode) {
      rowNode = {
        node_key: `row:${item.zone_id}:${item.row_id}`,
        node_type: "row" as const,
        zone_id: item.zone_id,
        zone_name: zoneNode.zone_name,
        row_id: item.row_id,
        row_name: rowMap.get(item.row_id)?.node_name || item.row_id,
        slot_id: "",
        slot_name: "",
        device_id: "",
        device_name: "",
        status: "",
        children: []
      };
      zoneNode.children?.push(rowNode);
    }

    let slotNode = (rowNode.children || []).find((child) => child.node_key === `slot:${item.slot_id}`);
    if (!slotNode) {
      slotNode = {
        node_key: `slot:${item.slot_id}`,
        node_type: "slot" as const,
        zone_id: item.zone_id,
        zone_name: zoneNode.zone_name,
        row_id: item.row_id,
        row_name: rowNode.row_name,
        slot_id: item.slot_id,
        slot_name: slotMap.get(item.slot_id)?.node_name || item.slot_id,
        device_id: "",
        device_name: "",
        status: "",
        children: []
      };
      rowNode.children?.push(slotNode);
    }

    slotNode.children?.push({
      node_key: `device:${item.plan_device_run_id}`,
      node_type: "device",
      zone_id: item.zone_id,
      zone_name: zoneNode.zone_name,
      row_id: item.row_id,
      row_name: rowNode.row_name,
      slot_id: item.slot_id,
      slot_name: slotNode.slot_name,
      device_id: item.device_id,
      device_name: `设备 ${item.device_id}`,
      status: item.status
    });

    zoneTree.set(item.zone_id, zoneNode);
  }

  return Array.from(zoneTree.values());
}

export const PlanRunsPage = defineComponent({
  name: "PlanRunsPage",
  setup() {
    const plansStore = usePlansStore();
    const noticesStore = useNoticesStore();
    const { runs, runsTotal, runsPage, runsPageSize, selectedEvents, loadingRuns, loadingEvents, stoppingRunID, mutatingDevices, errorMessage } =
      storeToRefs(plansStore);

    const eventsDialogVisible = ref(false);
    const appendDialogVisible = ref(false);
    const devicesDialogVisible = ref(false);
    const selectedRun = ref<PlanRunRecord | null>(null);
    const locationNodes = ref<LocationNodeRecord[]>([]);
    const supportingDataWarning = ref("");
    const appendTableRef = ref();
    const appendForm = reactive({
      rows: [] as PlanRowBinding[]
    });
    const deviceEventFilter = ref("");

    const displayedEvents = computed(() => {
      if (!deviceEventFilter.value) {
        return selectedEvents.value.filter((item) => !item.device_id || item.device_id === "0");
      }
      return selectedEvents.value.filter((item) => item.device_id === deviceEventFilter.value);
    });

    const availableRowTree = computed(() => buildLocationRowTree(locationNodes.value));
    const runDeviceTree = computed(() => buildRunDeviceTree(selectedRun.value, locationNodes.value));

    async function loadPageData() {
      await plansStore.loadRuns();
      try {
        locationNodes.value = await fetchLocationNodes();
        supportingDataWarning.value = "";
      } catch (_error) {
        supportingDataWarning.value = "位置树加载失败";
      }
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(supportingDataWarning, (value, previousValue) => {
      if (value && value !== previousValue) {
        noticesStore.warning(`计划任务实例辅助数据加载不完整：${value}`, 5000);
      }
    });

    watch(errorMessage, (value, previousValue) => {
      if (value && value !== previousValue) {
        if (loadingRuns.value) {
          noticesStore.error(`计划任务实例加载失败：${value}`, 5000);
        }
      }
    });

    function openEventsDialog(run: PlanRunRecord, deviceRun?: PlanDeviceRunRecord) {
      selectedRun.value = run;
      deviceEventFilter.value = deviceRun?.device_id || "";
      void plansStore.loadPlanEvents(run.plan_def_id, run.plan_run_id).then(() => {
        eventsDialogVisible.value = true;
      });
    }

    function openDevicesDialog(run: PlanRunRecord) {
      selectedRun.value = run;
      devicesDialogVisible.value = true;
    }

    async function stopDeviceRun(deviceRun: PlanDeviceRunRecord) {
      if (!selectedRun.value) {
        return;
      }
      try {
        await plansStore.triggerStopPlanDeviceRun(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, deviceRun.plan_device_run_id);
        noticesStore.success("设备已停止", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "设备停止失败", 5000);
      }
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

    function openAppendDialog(item: PlanRunRecord) {
      selectedRun.value = item;
      appendForm.rows = buildRunRowBindings(item);
      appendDialogVisible.value = true;
      void nextTick(() => {
        syncSelectedRows(appendTableRef.value, appendForm.rows);
      });
    }

    function handleAppendSelectionChange(items: LocationRowTreeNode[]) {
      appendForm.rows = items
        .filter((item) => item.node_type === "row")
        .map((item) => ({
          zone_id: item.zone_id,
          row_id: item.row_id,
          zone_name: item.zone_name,
          row_name: item.row_name
        }));
    }

    async function stopCurrentRun(item: PlanRunRecord) {
      try {
        await plansStore.triggerStopPlanRun(item.plan_def_id, item.plan_run_id);
        noticesStore.success("计划任务实例已停止", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务实例停止失败", 5000);
      }
    }

    async function deleteCurrentRun(item: PlanRunRecord) {
      try {
        await ElMessageBox.confirm(`确认删除计划任务实例 ${item.plan_run_id} 吗？`, "删除计划任务实例", {
          confirmButtonText: "确认删除",
          cancelButtonText: "取消",
          type: "warning"
        });
      } catch (_error) {
        return;
      }

      try {
        await plansStore.removePlanRun(item.plan_def_id, item.plan_run_id);
        noticesStore.success("计划任务实例已删除", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "计划任务实例删除失败", 5000);
      }
    }

    async function submitAppendRows() {
      if (!selectedRun.value) {
        return;
      }
      const currentRows = buildRunRowBindings(selectedRun.value);
      const currentKeys = new Set(currentRows.map((item) => `${item.zone_id}:${item.row_id}`));
      const nextKeys = new Set(appendForm.rows.map((item) => `${item.zone_id}:${item.row_id}`));

      const rowsToAdd = appendForm.rows.filter((item) => !currentKeys.has(`${item.zone_id}:${item.row_id}`));
      const rowsToRemove = currentRows.filter((item) => !nextKeys.has(`${item.zone_id}:${item.row_id}`));

      try {
        for (const item of rowsToAdd) {
          await plansStore.appendPlanRows(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, [item]);
        }
        for (const item of rowsToRemove) {
          await plansStore.removeRowFromPlan(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, item.zone_id, item.row_id);
        }
        appendDialogVisible.value = false;
        noticesStore.success("计划任务实例排选择已更新", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "更新实例排选择失败", 5000);
      }
    }

    async function removeRow(item: PlanDeviceRunRecord) {
      if (!selectedRun.value) {
        return;
      }
      try {
        await plansStore.removeRowFromPlan(selectedRun.value.plan_def_id, selectedRun.value.plan_run_id, item.zone_id, item.row_id);
        noticesStore.success("整排已从计划任务实例移除", 3000);
        await loadPageData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "移除排失败", 5000);
      }
    }

    async function removeOrStopRow(item: PlanDeviceRunRecord) {
      const isActive = item.status === "pending" || item.status === "running";
      if (isActive) {
        try {
          await ElMessageBox.confirm(`确认停止排 ${item.zone_id}-${item.row_id} 当前的计划任务执行吗？`, "停止排确认", {
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
      await removeRow(item);
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
                            h(ElTableColumn, { label: "状态", width: 120 }, { default: (scope: { row: PlanRunRecord }) => renderStatus(scope.row.status) }),
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
                              { label: "操作", width: 160, fixed: "right" },
                              {
                                default: (scope: { row: PlanRunRecord }) =>
                                  h("div", { class: "table-actions table-actions--nowrap" }, [
                                    h(
                                      ElButton,
                                      {
                                        type: "primary",
                                        link: true,
                                        onClick: () => openDevicesDialog(scope.row)
                                      },
                                      () => "查看设备"
                                    ),
                                    h(
                                      ElDropdown,
                                      {
                                        trigger: "click"
                                      },
                                      {
                                        default: () => h(ElButton, { type: "primary", link: true }, () => "更多"),
                                        dropdown: () =>
                                          h(
                                            ElDropdownMenu,
                                            null,
                                            {
                                              default: () => [
                                    h(
                                      ElDropdownItem,
                                      {
                                        key: "view_events",
                                        onClick: () => openEventsDialog(scope.row)
                                      },
                                      () => "查看事件"
                                    ),
                                    h(
                                      ElDropdownItem,
                                      {
                                        key: "append_rows",
                                        onClick: () => openAppendDialog(scope.row)
                                                  },
                                                  () => "追加排"
                                                ),
                                                h(
                                                  ElDropdownItem,
                                                  {
                                                    key: "stop_run",
                                                    disabled: stoppingRunID.value === scope.row.plan_run_id,
                                                    onClick: () => {
                                                      void stopCurrentRun(scope.row);
                                                    }
                                                  },
                                                  () => (stoppingRunID.value === scope.row.plan_run_id ? "停止中..." : "停止实例")
                                                ),
                                                h(
                                                  ElDropdownItem,
                                                  {
                                                    key: "delete_run",
                                                    onClick: () => {
                                                      void deleteCurrentRun(scope.row);
                                                    }
                                                  },
                                                  () => "删除"
                                                )
                                              ]
                                            }
                                          )
                                      },
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
            modelValue: eventsDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (eventsDialogVisible.value = value),
            title: selectedRun.value
              ? deviceEventFilter.value
                ? `设备 ${deviceEventFilter.value} 事件：${selectedRun.value.plan_run_id}`
                : `计划任务实例事件：${selectedRun.value.plan_run_id}`
              : "计划任务实例事件",
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
            modelValue: devicesDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (devicesDialogVisible.value = value),
            title: selectedRun.value ? `运行设备：${selectedRun.value.plan_run_id}` : "运行设备",
            width: "980px"
          },
          {
            default: () =>
              runDeviceTree.value.length === 0
                ? h(ElEmpty, { description: "当前实例还没有设备记录" })
                : h(
                    ElTable,
                    {
                      data: runDeviceTree.value,
                      stripe: true,
                      border: false,
                      class: "app-table",
                      height: "520px",
                      rowKey: "node_key",
                      defaultExpandAll: true,
                      treeProps: {
                        children: "children"
                      }
                    },
                    {
                      default: () => [
                        h(ElTableColumn, {
                          label: "分区 / 排 / 槽位 / 设备",
                          minWidth: 300,
                          formatter: (row: RunDeviceTreeNode) => {
                            if (row.node_type === "zone") {
                              return row.zone_name;
                            }
                            if (row.node_type === "row") {
                              return row.row_name;
                            }
                            if (row.node_type === "slot") {
                              return row.slot_name;
                            }
                            return row.device_name;
                          }
                        }),
                        h(ElTableColumn, {
                          label: "设备ID",
                          minWidth: 120,
                          formatter: (row: RunDeviceTreeNode) => (row.node_type === "device" ? row.device_id : "")
                        }),
                        h(ElTableColumn, {
                          label: "状态",
                          width: 120
                        }, {
                          default: (scope: { row: RunDeviceTreeNode }) =>
                            scope.row.node_type === "device" ? renderStatus(scope.row.status) : h("span", "")
                        }),
                        h(
                          ElTableColumn,
                          {
                            label: "操作",
                            width: 180,
                            fixed: "right"
                          },
                          {
                            default: (scope: { row: RunDeviceTreeNode }) =>
                              scope.row.node_type === "device" && selectedRun.value
                                ? h("div", { class: "table-actions table-actions--nowrap" }, [
                                    h(
                                      ElButton,
                                      {
                                        type: "primary",
                                        link: true,
                                        onClick: () =>
                                          openEventsDialog(selectedRun.value as PlanRunRecord, {
                                            device_id: scope.row.device_id
                                          } as PlanDeviceRunRecord)
                                      },
                                      () => "查看事件"
                                    ),
                                    h(
                                      ElButton,
                                      {
                                        type: "danger",
                                        link: true,
                                        disabled: scope.row.status !== "pending" && scope.row.status !== "running",
                                        onClick: () =>
                                          void stopDeviceRun({
                                            plan_device_run_id: scope.row.node_key.replace("device:", ""),
                                            plan_run_id: selectedRun.value!.plan_run_id,
                                            plan_def_id: selectedRun.value!.plan_def_id,
                                            zone_id: scope.row.zone_id,
                                            row_id: scope.row.row_id,
                                            slot_id: scope.row.slot_id,
                                            device_id: scope.row.device_id,
                                            target_type: "",
                                            target_ref_id: "",
                                            status: scope.row.status,
                                            started_at: "",
                                            finished_at: "",
                                            last_error: "",
                                            created_at: "",
                                            updated_at: ""
                                          } as PlanDeviceRunRecord)
                                      },
                                      () => "停止"
                                    )
                                  ])
                                : h("span", "")
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
            modelValue: appendDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (appendDialogVisible.value = value),
            title: selectedRun.value ? `追加排：${selectedRun.value.plan_run_id}` : "追加排",
            width: "720px"
          },
          {
            default: () =>
              h(
                ElTable,
                {
                  ref: appendTableRef,
                  data: availableRowTree.value,
                  stripe: true,
                  border: true,
                  height: "360px",
                  rowKey: "node_key",
                  defaultExpandAll: true,
                  treeProps: {
                    children: "children",
                    checkStrictly: false
                  },
                  onSelectionChange: handleAppendSelectionChange
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
              ),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (appendDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: mutatingDevices.value, onClick: () => void submitAppendRows() }, () => "保存")
              ])
          }
        )
      ]);
  }
});

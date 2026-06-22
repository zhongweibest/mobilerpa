import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElPagination,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";

import { useDiscoveryStore } from "../../stores/discovery";
import type { DiscoveredDevice } from "../../types/discovery";

const PAGE_SIZES = [10, 20, 30, 50, 100];

function renderBooleanTag(active: boolean, yesLabel: string, noLabel: string) {
  return h("span", { class: ["status-tag", active ? "status-tag--online" : "status-tag--offline"] }, active ? yesLabel : noLabel);
}

function renderSourceLabel(device: DiscoveredDevice) {
  const sourceLabelMap: Record<string, string> = {
    mdns: "来自 mDNS",
    adb_devices: "来自 adb devices",
    merged: "来自 mDNS + adb devices"
  };
  return sourceLabelMap[device.source] || device.source || "未知来源";
}

function renderConnectionKind(device: DiscoveredDevice) {
  const kindLabelMap: Record<string, string> = {
    connected_device: "已连接设备",
    connect_service: "可连接服务",
    pairing_service: "配对服务",
    unknown_service: "未知服务"
  };
  return kindLabelMap[device.connection_kind] || device.connection_kind || "未知";
}

export const DiscoveryPage = defineComponent({
  name: "DiscoveryPage",
  setup() {
    const router = useRouter();
    const discoveryStore = useDiscoveryStore();
    const {
      devices,
      total,
      page,
      pageSize,
      selectedEndpoints,
      deploymentResults,
      loading,
      deploying,
      actingEndpoint,
      errorMessage,
      centerBaseURL,
      resetConfig,
      runAgent,
      latestActionResult,
      selectableDevices,
      pairing
    } = storeToRefs(discoveryStore);

    const deploymentResultsDialogVisible = ref(false);
    const actionResultsDialogVisible = ref(false);
    const pairDialogVisible = ref(false);
    const deployDialogVisible = ref(false);
    const pairForm = reactive({ host: "", port: "", pair_code: "" });

    onMounted(() => {
      void discoveryStore.loadDevices();
      void discoveryStore.loadDiscoverySettings();
    });

    const allSelected = computed(() => selectableDevices.value.length > 0 && selectedEndpoints.value.length === selectableDevices.value.length);

    async function handlePairDevice() {
      if (pairForm.host.trim() === "" || pairForm.port.trim() === "" || pairForm.pair_code.trim() === "") {
        ElMessage.warning("请完整填写手机 IP、端口和配对码");
        return;
      }

      try {
        await discoveryStore.submitPairDevice(pairForm.host.trim(), pairForm.port.trim(), pairForm.pair_code.trim());
        pairDialogVisible.value = false;
        pairForm.host = "";
        pairForm.port = "";
        pairForm.pair_code = "";
        ElMessage.success("配对命令已执行，设备发现结果已刷新");
      } catch (_error) {
        ElMessage.error("连接设备失败，请检查无线调试 IP、端口和配对码");
      }
    }

    function openDeployDialog() {
      if (selectedEndpoints.value.length === 0) {
        ElMessage.warning("请先选择至少一台设备");
        return;
      }
      if (centerBaseURL.value.trim() === "") {
        ElMessage.warning("请先在系统配置中维护中心地址");
        return;
      }
      deployDialogVisible.value = true;
    }

    async function handleDeployConfirm() {
      try {
        await discoveryStore.submitDeployment();
        deployDialogVisible.value = false;
        deploymentResultsDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("批量下发失败，请查看页面错误提示和结果弹窗");
      }
    }

    async function handleSingleDeploy(adbEndpoint: string) {
      if (centerBaseURL.value.trim() === "") {
        ElMessage.warning("请先在系统配置中维护中心地址");
        return;
      }

      try {
        await discoveryStore.submitSingleDeployment(adbEndpoint);
        deploymentResultsDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("当前设备下发 Agent 失败，请查看页面错误提示和结果弹窗");
      }
    }

    async function handleAgentAction(adbEndpoint: string, action: "start" | "stop" | "disconnect") {
      const actionLabel = action === "start" ? "启动" : action === "stop" ? "停止" : "断开连接";

      try {
        await ElMessageBox.confirm(
          action === "disconnect" ? `确认要断开设备 ${adbEndpoint} 的 ADB 连接吗？` : `确认要${actionLabel}设备 ${adbEndpoint} 的 Agent 吗？`,
          action === "disconnect" ? "断开设备连接" : `${actionLabel} Agent`,
          { confirmButtonText: "确认", cancelButtonText: "取消", type: "warning" }
        );
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
      }

      try {
        await discoveryStore.submitAgentAction(adbEndpoint, action);
        actionResultsDialogVisible.value = true;
        if (action === "disconnect") {
          await discoveryStore.loadDevices();
        }
      } catch (_error) {
        ElMessage.error(action === "disconnect" ? "断开设备连接失败，请查看页面错误提示" : `${actionLabel} Agent 失败，请查看页面错误提示`);
      }
    }

    return () =>
      h("section", { class: "discovery-page" }, [
        h("div", { class: "page-toolbar discovery-page__toolbar" }, [
          h("div", { class: "discovery-page__toolbar-left" }, [
            h("span", { class: "discovery-page__toolbar-label" }, "当前中心地址："),
            h("span", { class: "discovery-page__toolbar-value" }, centerBaseURL.value.trim() === "" ? "未配置，请先前往系统配置维护" : centerBaseURL.value),
            h(
              ElButton,
              { link: true, type: "primary", onClick: () => void router.push("/settings") },
              () => "前往系统配置"
            )
          ]),
          h("div", { class: "discovery-page__toolbar-actions" }, [
            h(
              ElButton,
              {
                type: "primary",
                disabled: deploying.value || selectedEndpoints.value.length === 0 || actingEndpoint.value !== "",
                loading: deploying.value,
                onClick: openDeployDialog
              },
              () => "下发 Agent"
            ),
            h(
              ElButton,
              {
                type: "primary",
                disabled: loading.value || deploying.value || actingEndpoint.value !== "",
                onClick: () => {
                  pairDialogVisible.value = true;
                }
              },
              () => "连接设备"
            ),
            h(
              ElButton,
              {
                disabled: loading.value || deploying.value || actingEndpoint.value !== "",
                loading: loading.value,
                onClick: () => {
                  void discoveryStore.loadDevices();
                }
              },
              () => "刷新"
            )
          ])
        ]),
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            default: () =>
              h("div", { class: "page-scroll-body" }, [
                errorMessage.value
                  ? h(ElAlert, {
                      class: "page-alert",
                      type: "error",
                      title: `设备发现或操作失败：${errorMessage.value}`,
                      showIcon: true,
                      closable: false
                    })
                  : null,
                latestActionResult.value
                  ? h(ElAlert, {
                      class: "page-alert",
                      type: latestActionResult.value.status === "ok" ? "success" : "error",
                      title: `设备 ${latestActionResult.value.adb_endpoint} ${latestActionResult.value.action === "start" ? "启动" : latestActionResult.value.action === "stop" ? "停止" : "断开连接"}${latestActionResult.value.action === "disconnect" ? "" : " Agent"}：${latestActionResult.value.message}`,
                      showIcon: true,
                      closable: false
                    })
                  : null,
                devices.value.length === 0 && !loading.value
                  ? h(ElEmpty, { description: "当前没有 adb devices 可识别的设备，请先通过“连接设备”完成配对。" })
                  : h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: devices.value, stripe: true, class: "tasks-table", tableLayout: "fixed", height: "100%" },
                        {
                          default: () => [
                            h(
                              ElTableColumn,
                              { width: 64, align: "center" },
                              {
                                header: () =>
                                  h("input", {
                                    type: "checkbox",
                                    checked: allSelected.value,
                                    disabled: selectableDevices.value.length === 0,
                                    onChange: (event: Event) => {
                                      discoveryStore.toggleSelectAll((event.target as HTMLInputElement).checked);
                                    }
                                  }),
                                default: ({ row }: { row: DiscoveredDevice }) =>
                                  h("input", {
                                    type: "checkbox",
                                    checked: selectedEndpoints.value.includes(row.adb_endpoint),
                                    disabled: !row.connectable,
                                    onChange: (event: Event) => {
                                      discoveryStore.toggleSelection(row.adb_endpoint, (event.target as HTMLInputElement).checked);
                                    }
                                  })
                              }
                            ),
                            h(
                              ElTableColumn,
                              { label: "设备", minWidth: 220 },
                              {
                                default: ({ row }: { row: DiscoveredDevice }) =>
                                  h("div", null, [
                                    h("div", { class: "devices-table__name" }, row.device_name.trim() === "" ? row.service_name : row.device_name),
                                    h("div", { class: "devices-table__meta" }, row.service_name)
                                  ])
                              }
                            ),
                            h(ElTableColumn, { prop: "device_id", label: "中心设备 ID", minWidth: 150, formatter: (row: { device_id?: string }) => row.device_id || "未匹配" }),
                            h(ElTableColumn, { prop: "adb_endpoint", label: "ADB 地址", minWidth: 220 }),
                            h(ElTableColumn, { label: "来源", minWidth: 150 }, { default: ({ row }: { row: DiscoveredDevice }) => renderSourceLabel(row) }),
                            h(ElTableColumn, { label: "类型", minWidth: 120 }, { default: ({ row }: { row: DiscoveredDevice }) => renderConnectionKind(row) }),
                            h(ElTableColumn, { label: "可连接", width: 110 }, { default: ({ row }: { row: DiscoveredDevice }) => renderBooleanTag(row.connectable, "可连接", "不可连接") }),
                            h(ElTableColumn, { label: "已连接", width: 110 }, { default: ({ row }: { row: DiscoveredDevice }) => renderBooleanTag(row.connected, "已连接", "未连接") }),
                            h(
                              ElTableColumn,
                              { label: "操作", minWidth: 320, fixed: "right" },
                              {
                                default: ({ row }: { row: DiscoveredDevice }) =>
                                  h("div", { class: "table-actions" }, [
                                    h(ElButton, { size: "small", onClick: () => void handleSingleDeploy(row.adb_endpoint) }, () => "下发 Agent"),
                                    h(ElButton, { size: "small", onClick: () => void handleAgentAction(row.adb_endpoint, "start") }, () => "启动 Agent"),
                                    h(ElButton, { size: "small", onClick: () => void handleAgentAction(row.adb_endpoint, "stop") }, () => "停止 Agent"),
                                    h(ElButton, { size: "small", type: "danger", plain: true, onClick: () => void handleAgentAction(row.adb_endpoint, "disconnect") }, () => "断开连接")
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
                      void discoveryStore.changePage(value);
                    },
                    "onUpdate:pageSize": (value: number) => {
                      void discoveryStore.changePageSize(value);
                    }
                  })
                )
              ])
          }
        ),
        h(
          ElDialog,
          { modelValue: deployDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (deployDialogVisible.value = value), title: "下发 Agent", width: "560px", closeOnClickModal: false },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElAlert, {
                  class: "dialog-alert",
                  type: "info",
                  title: `本次将下发到 ${selectedEndpoints.value.length} 台已选设备，中心地址将使用当前系统配置中的地址。`,
                  showIcon: true,
                  closable: false
                }),
                h(ElFormItem, { label: "当前中心地址" }, () => h("div", { class: "discovery-page__dialog-value" }, centerBaseURL.value)),
                h(ElFormItem, { label: "下发选项" }, () =>
                  h("div", { class: "discovery-page__deploy-options" }, [
                    h(
                      ElCheckbox,
                      {
                        modelValue: resetConfig.value,
                        "onUpdate:modelValue": (value: boolean | string | number) => {
                          resetConfig.value = Boolean(value);
                        }
                      },
                      () => "重置设备现有配置"
                    ),
                    h(
                      ElCheckbox,
                      {
                        modelValue: runAgent.value,
                        "onUpdate:modelValue": (value: boolean | string | number) => {
                          runAgent.value = Boolean(value);
                        }
                      },
                      () => "下发后自动运行 Agent"
                    )
                  ])
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (deployDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: deploying.value, onClick: () => void handleDeployConfirm() }, () => "确认下发")
              ])
          }
        ),
        h(
          ElDialog,
          { modelValue: pairDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (pairDialogVisible.value = value), title: "连接无线调试设备", width: "560px", closeOnClickModal: false },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "手机 IP" }, () =>
                  h(ElInput, {
                    modelValue: pairForm.host,
                    "onUpdate:modelValue": (value: string) => {
                      pairForm.host = value;
                    },
                    placeholder: "例如 192.168.0.120"
                  })
                ),
                h(ElFormItem, { label: "端口" }, () =>
                  h(ElInput, {
                    modelValue: pairForm.port,
                    "onUpdate:modelValue": (value: string) => {
                      pairForm.port = value;
                    },
                    placeholder: "例如 37123"
                  })
                ),
                h(ElFormItem, { label: "配对码" }, () =>
                  h(ElInput, {
                    modelValue: pairForm.pair_code,
                    "onUpdate:modelValue": (value: string) => {
                      pairForm.pair_code = value;
                    },
                    placeholder: "输入手机无线调试页面显示的配对码"
                  })
                ),
                h(ElAlert, { class: "dialog-alert", type: "info", title: "提交后会由中心服务执行 adb pair IP:端口 配对码，并在成功后自动刷新设备发现结果。", showIcon: true, closable: false })
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (pairDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: pairing.value, onClick: () => void handlePairDevice() }, () => "执行连接")
              ])
          }
        ),
        h(
          ElDialog,
          { modelValue: deploymentResultsDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (deploymentResultsDialogVisible.value = value), title: "Agent 下发结果", width: "880px", closeOnClickModal: true },
          {
            default: () =>
              deploymentResults.value.length === 0
                ? h(ElEmpty, { description: "当前没有可展示的下发结果" })
                : h(
                    ElTable,
                    { data: deploymentResults.value, stripe: true, class: "tasks-table", tableLayout: "fixed" },
                    {
                      default: () => [
                        h(ElTableColumn, { prop: "adb_endpoint", label: "ADB 地址", minWidth: 220 }),
                        h(ElTableColumn, { label: "连接", width: 100 }, { default: ({ row }: { row: { connected: boolean } }) => h(ElTag, { type: row.connected ? "success" : "danger", effect: "light" }, () => (row.connected ? "成功" : "失败")) }),
                        h(ElTableColumn, { label: "推送", width: 100 }, { default: ({ row }: { row: { pushed: boolean } }) => h(ElTag, { type: row.pushed ? "success" : "danger", effect: "light" }, () => (row.pushed ? "成功" : "失败")) }),
                        h(ElTableColumn, { label: "启动", width: 100 }, { default: ({ row }: { row: { started: boolean } }) => h(ElTag, { type: row.started ? "success" : "info", effect: "light" }, () => (row.started ? "成功" : "未启动")) }),
                        h(ElTableColumn, { prop: "status", label: "状态", width: 120 }),
                        h(ElTableColumn, { prop: "message", label: "说明", minWidth: 240 })
                      ]
                    }
                  ),
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (deploymentResultsDialogVisible.value = false) }, () => "关闭")])
          }
        ),
        h(
          ElDialog,
          { modelValue: actionResultsDialogVisible.value, "onUpdate:modelValue": (value: boolean) => (actionResultsDialogVisible.value = value), title: "Agent 操作结果", width: "680px", closeOnClickModal: true },
          {
            default: () =>
              latestActionResult.value
                ? h(
                    ElTable,
                    { data: [latestActionResult.value], stripe: true, class: "tasks-table", tableLayout: "fixed" },
                    {
                      default: () => [
                        h(ElTableColumn, { prop: "adb_endpoint", label: "ADB 地址", minWidth: 220 }),
                        h(ElTableColumn, { prop: "action", label: "动作", width: 120 }),
                        h(ElTableColumn, { prop: "status", label: "状态", width: 120 }),
                        h(ElTableColumn, { prop: "message", label: "说明", minWidth: 220 })
                      ]
                    }
                  )
                : h(ElEmpty, { description: "当前没有可展示的 Agent 操作结果" }),
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (actionResultsDialogVisible.value = false) }, () => "关闭")])
          }
        )
      ]);
  }
});

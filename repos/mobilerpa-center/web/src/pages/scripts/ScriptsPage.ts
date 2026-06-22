// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElMessageBox,
  ElPagination,
  ElScrollbar,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";

import { useDevicesStore } from "../../stores/devices";
import { useScriptsStore } from "../../stores/scripts";
import type { DeviceRecord } from "../../types/device";
import type { ScriptRecord, ScriptVersionRecord, WorkflowReferenceRecord } from "../../types/script";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

type DeployLogItem = {
  device_id: string;
  success: boolean;
  message: string;
};

function formatWorkflowReferences(references: WorkflowReferenceRecord[]): string {
  if (!references || references.length === 0) {
    return "";
  }
  return references.map((item) => `${item.workflow_name || item.workflow_def_id} / ${item.node_name || item.node_id}`).join("；");
}

function getDeviceLabel(device: DeviceRecord): string {
  return `${device.device_id} / ${device.device_name || device.model || "未知设备"}`;
}

export const ScriptsPage = defineComponent({
  name: "ScriptsPage",
  setup() {
    const scriptsStore = useScriptsStore();
    const devicesStore = useDevicesStore();
    const { scripts, total, page, pageSize, selectedManifest, loading, uploading, deploying, errorMessage } = storeToRefs(scriptsStore);
    const { devices } = storeToRefs(devicesStore);

    const uploadDialogVisible = ref(false);
    const uploadFile = ref<File | null>(null);
    const uploadInputKey = ref(0);
    const uploadInputRef = ref<HTMLInputElement | null>(null);
    const detailDialogVisible = ref(false);
    const deployDialogVisible = ref(false);
    const referencesDialogVisible = ref(false);
    const selectedReferenceVersion = ref<{ scriptName: string; scriptVersion: string; references: WorkflowReferenceRecord[] } | null>(null);

    const uploadForm = reactive({
      script_name: "shoppe_sync",
      script_version: "v0.1.1",
      force: false
    });

    const deployForm = reactive({
      script_name: "",
      script_version: "",
      force: false
    });

    const deployDeviceIDs = ref<string[]>([]);
    const deployLogs = ref<DeployLogItem[]>([]);

    const onlineDevices = computed(() => devices.value.filter((item) => item.status === "online"));
    const scriptCount = computed(() => scripts.value.length);
    const versionCount = computed(() => scripts.value.reduce((count, item) => count + item.versions.length, 0));

    async function loadPageData() {
      await Promise.all([scriptsStore.loadScripts(), devicesStore.loadDevices()]);
    }

    onMounted(() => {
      void loadPageData();
    });

    function resetUploadDialog() {
      uploadForm.script_name = "shoppe_sync";
      uploadForm.script_version = "v0.1.1";
      uploadForm.force = false;
      uploadFile.value = null;
      uploadInputKey.value += 1;
    }

    function openUploadDialog() {
      resetUploadDialog();
      uploadDialogVisible.value = true;
    }

    function openUploadFilePicker() {
      uploadInputRef.value?.click();
    }

    async function handleUploadConfirm() {
      if (uploadForm.script_name.trim() === "" || uploadForm.script_version.trim() === "") {
        ElMessage.warning("请先填写脚本名称和脚本版本");
        return;
      }

      if (!uploadFile.value) {
        ElMessage.warning("请先选择 zip 脚本包");
        return;
      }

      try {
        await scriptsStore.submitScriptUpload({
          script_name: uploadForm.script_name.trim(),
          script_version: uploadForm.script_version.trim(),
          source_type: "zip",
          force: uploadForm.force,
          file: uploadFile.value
        });
        uploadDialogVisible.value = false;
        resetUploadDialog();
        ElMessage.success("脚本上传成功");
      } catch (_error) {
        ElMessage.error("上传脚本失败，请检查 zip 解压后的根目录下是否存在 index.js");
      }
    }

    async function handleView(scriptName: string, scriptVersion: string) {
      try {
        await scriptsStore.loadScriptVersion(scriptName, scriptVersion);
        detailDialogVisible.value = true;
      } catch (_error) {
        ElMessage.error("加载脚本详情失败，请稍后重试");
      }
    }

    async function handleDeleteVersion(scriptName: string, scriptVersion: string) {
      try {
        await ElMessageBox.confirm(`确认删除脚本版本 ${scriptName}@${scriptVersion} 吗？删除后该版本的中心库存与文件目录都会一起清理。`, "删除脚本版本", {
          type: "warning",
          confirmButtonText: "确认删除",
          cancelButtonText: "取消"
        });
        await scriptsStore.removeScriptVersion(scriptName, scriptVersion);
        ElMessage.success(`已删除脚本版本 ${scriptName}@${scriptVersion}`);
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        const err = error as Error & { details?: Record<string, unknown> | null };
        if (err?.message === "script_version_referenced_by_workflows") {
          const references = Array.isArray(err.details?.workflow_references) ? err.details.workflow_references : [];
          ElMessage.error(`删除失败，该版本已被工作流引用：${formatWorkflowReferences(references as WorkflowReferenceRecord[])}`);
          return;
        }
        ElMessage.error("删除脚本版本失败，请稍后重试");
      }
    }

    async function handleDeleteScript(scriptName: string) {
      try {
        await ElMessageBox.confirm(`确认删除脚本 ${scriptName} 吗？这会删除该脚本名下的全部版本与中心库存目录。`, "删除脚本", {
          type: "warning",
          confirmButtonText: "确认删除",
          cancelButtonText: "取消"
        });
        await scriptsStore.removeScript(scriptName);
        ElMessage.success(`已删除脚本 ${scriptName}`);
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        const err = error as Error & { details?: Record<string, unknown> | null };
        if (err?.message === "script_referenced_by_workflows") {
          const references = Array.isArray(err.details?.workflow_references) ? err.details.workflow_references : [];
          ElMessage.error(`删除失败，该脚本仍被工作流引用：${formatWorkflowReferences(references as WorkflowReferenceRecord[])}`);
          return;
        }
        ElMessage.error("删除脚本失败，请稍后重试");
      }
    }

    function openReferencesDialog(scriptName: string, scriptVersion: string, references: WorkflowReferenceRecord[]) {
      selectedReferenceVersion.value = {
        scriptName,
        scriptVersion,
        references
      };
      referencesDialogVisible.value = true;
    }

    function openDeployDialog(scriptName: string, scriptVersion: string) {
      deployForm.script_name = scriptName;
      deployForm.script_version = scriptVersion;
      deployForm.force = false;
      deployDeviceIDs.value = [];
      deployLogs.value = [];
      deployDialogVisible.value = true;
    }

    function toggleDeployDevice(deviceID: string) {
      if (deployDeviceIDs.value.includes(deviceID)) {
        deployDeviceIDs.value = deployDeviceIDs.value.filter((item) => item !== deviceID);
        return;
      }
      deployDeviceIDs.value = [...deployDeviceIDs.value, deviceID];
    }

    function selectAllDeployDevices() {
      deployDeviceIDs.value = onlineDevices.value.map((item) => item.device_id);
    }

    function clearDeployDevices() {
      deployDeviceIDs.value = [];
    }

    async function executeDeployForDevices(deviceIDs: string[]) {
      if (deviceIDs.length === 0) {
        ElMessage.warning("当前没有可下发的在线设备");
        return;
      }

      const results: DeployLogItem[] = [];
      for (const deviceID of deviceIDs) {
        try {
          await scriptsStore.triggerScriptDeploy(deviceID, deployForm.script_name, deployForm.script_version, deployForm.force);
          results.push({
            device_id: deviceID,
            success: true,
            message: "已成功发送脚本同步指令"
          });
        } catch (error) {
          results.push({
            device_id: deviceID,
            success: false,
            message: error instanceof Error ? error.message : "deploy_script_failed"
          });
        }
      }

      deployLogs.value = results;
      const successCount = results.filter((item) => item.success).length;
      const failedCount = results.length - successCount;
      if (failedCount === 0) {
        ElMessage.success(`批量下发完成，共 ${successCount} 台设备已发送同步指令`);
      } else {
        ElMessage.warning(`批量下发已结束，成功 ${successCount} 台，失败 ${failedCount} 台`);
      }
    }

    async function handleDeployConfirm() {
      if (deployDeviceIDs.value.length === 0) {
        ElMessage.warning("请至少选择一台设备");
        return;
      }
      await executeDeployForDevices(deployDeviceIDs.value);
    }

    async function handleDeployToAllOnlineDevices() {
      try {
        await scriptsStore.triggerScriptDeployToAll(deployForm.script_name, deployForm.script_version, deployForm.force);
        deployLogs.value = onlineDevices.value.map((item) => ({
          device_id: item.device_id,
          success: true,
          message: "已通过后端批量下发接口发送同步指令"
        }));
        ElMessage.success(`已触发全部在线设备下发，共 ${onlineDevices.value.length} 台`);
      } catch (_error) {
        ElMessage.error("全部下发失败，请检查中心服务批量接口状态");
      }
    }

    return () =>
      h("section", { class: "scripts-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(
            ElButton,
            {
              type: "primary",
              onClick: openUploadDialog
            },
            () => "上传脚本"
          ),
          h(
            ElButton,
            {
              onClick: () => {
                void loadPageData();
              },
              loading: loading.value
            },
            () => "刷新"
          )
        ]),
        errorMessage.value
          ? h(ElAlert, {
              class: "page-alert",
              type: "error",
              title: `脚本操作失败：${errorMessage.value}`,
              showIcon: true,
              closable: false
            })
          : null,
        h(
          ElCard,
          {
            class: "page-card page-fill-card",
            shadow: "never"
          },
          {
            header: () =>
              h("div", { class: "card-header" }, [
                h("div", null, [
                  h("div", { class: "card-header__title" }, "脚本版本总览"),
                  h("div", { class: "card-header__subtitle" }, "相同脚本名称合并为一行，展开后直接查看该脚本的全部版本明细与操作。")
                ])
              ]),
            default: () =>
              scripts.value.length === 0
                ? h(ElEmpty, {
                    description: "当前还没有已上传的脚本版本，请先上传 zip 脚本包。"
                  })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        {
                          data: scripts.value,
                          stripe: true,
                          class: "tasks-table",
                          tableLayout: "auto",
                          height: "100%",
                          rowKey: "script_name"
                        },
                        {
                          default: () => [
                            h(ElTableColumn, { type: "expand", width: 56 }, {
                              default: ({ row }: { row: ScriptRecord }) =>
                                h("div", { class: "script-versions-panel" }, [
                                  h(
                                    ElTable,
                                    {
                                      data: row.versions,
                                      class: "script-version-subtable",
                                      stripe: true,
                                      tableLayout: "auto"
                                    },
                                    {
                                      default: () => [
                                        h(ElTableColumn, {
                                          label: "版本号",
                                          minWidth: 120,
                                          formatter: (version: ScriptVersionRecord) => version.script_version || "暂无"
                                        }),
                                        h(ElTableColumn, {
                                          label: "创建时间",
                                          minWidth: 180,
                                          formatter: (version: ScriptVersionRecord) => formatDateTime(version.created_at)
                                        }),
                                        h(
                                          ElTableColumn,
                                          {
                                            label: "操作",
                                            minWidth: 320
                                          },
                                          {
                                            default: ({ row: version }: { row: ScriptVersionRecord }) =>
                                              h("div", { class: "table-actions" }, [
                                                h(
                                                  ElButton,
                                                  {
                                                    link: true,
                                                    type: "primary",
                                                    onClick: () => {
                                                      void handleView(version.script_name, version.script_version);
                                                    }
                                                  },
                                                  () => "查看详情"
                                                ),
                                                h(
                                                  ElButton,
                                                  {
                                                    link: true,
                                                    type: "warning",
                                                    disabled: !version.workflow_references || version.workflow_references.length === 0,
                                                    onClick: () => {
                                                      openReferencesDialog(version.script_name, version.script_version, version.workflow_references || []);
                                                    }
                                                  },
                                                  () => "查看引用"
                                                ),
                                                h(
                                                  ElButton,
                                                  {
                                                    link: true,
                                                    type: "success",
                                                    onClick: () => {
                                                      openDeployDialog(version.script_name, version.script_version);
                                                    }
                                                  },
                                                  () => "下发设备"
                                                ),
                                                h(
                                                  ElButton,
                                                  {
                                                    link: true,
                                                    type: "danger",
                                                    onClick: () => {
                                                      void handleDeleteVersion(version.script_name, version.script_version);
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
                                ])
                            }),
                            h(
                              ElTableColumn,
                              {
                                label: "脚本名称",
                                minWidth: 180
                              },
                              {
                                default: ({ row }: { row: ScriptRecord }) => h("div", { class: "devices-table__name" }, row.script_name)
                              }
                            ),
                            h(ElTableColumn, {
                              label: "版本数量",
                              width: 110,
                              formatter: (row: ScriptRecord) => String(row.versions.length)
                            }),
                            h(ElTableColumn, {
                              label: "最新版本",
                              minWidth: 120,
                              formatter: (row: ScriptRecord) => row.versions[0]?.script_version || "暂无"
                            }),
                            h(ElTableColumn, {
                              label: "入口文件",
                              minWidth: 140,
                              formatter: (row: ScriptRecord) => row.versions[0]?.entry_file || "暂无"
                            }),
                            h(
                              ElTableColumn,
                              {
                                label: "来源 / 存储",
                                minWidth: 180
                              },
                              {
                                default: ({ row }: { row: ScriptRecord }) =>
                                  row.versions[0]
                                    ? h("div", { class: "stack-tags" }, [
                                        h(
                                          ElTag,
                                          {
                                            type: "warning",
                                            effect: "plain"
                                          },
                                          () => row.versions[0].source_type || "未知来源"
                                        ),
                                        h(
                                          ElTag,
                                          {
                                            type: "info",
                                            effect: "plain"
                                          },
                                          () => row.versions[0].storage_type
                                        )
                                      ])
                                    : "暂无"
                              }
                            ),
                            h(
                              ElTableColumn,
                              {
                                label: "状态",
                                width: 100
                              },
                              {
                                default: ({ row }: { row: ScriptRecord }) =>
                                  row.versions[0]
                                    ? h(
                                        ElTag,
                                        {
                                          type: row.versions[0].status === "ready" ? "success" : "info",
                                          effect: "light"
                                        },
                                        () => row.versions[0].status || "未知"
                                      )
                                    : "暂无"
                              }
                            ),
                            h(ElTableColumn, {
                              label: "创建时间",
                              minWidth: 160,
                              formatter: (row: ScriptRecord) => (row.versions[0] ? formatDateTime(row.versions[0].created_at) : "暂无")
                            }),
                            h(
                              ElTableColumn,
                              {
                                label: "操作",
                                width: 100,
                                fixed: "right"
                              },
                              {
                                default: ({ row }: { row: ScriptRecord }) =>
                                  h("div", { class: "table-actions" }, [
                                    h(
                                      ElButton,
                                      {
                                        size: "small",
                                        type: "danger",
                                        plain: true,
                                        onClick: () => {
                                          void handleDeleteScript(row.script_name);
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
                        currentPage: page.value,
                        pageSize: pageSize.value,
                        pageSizes: PAGE_SIZES,
                        total: total.value,
                        layout: "total, sizes, prev, pager, next, jumper",
                        "onUpdate:currentPage": (value: number) => {
                          void scriptsStore.changePage(value);
                        },
                        "onUpdate:pageSize": (value: number) => {
                          void scriptsStore.changePageSize(value);
                        }
                      })
                    )
                  ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: uploadDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              uploadDialogVisible.value = value;
            },
            title: "上传脚本版本",
            width: "560px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(
                ElForm,
                {
                  labelPosition: "top",
                  class: "dialog-form"
                },
                () => [
                  h(ElFormItem, { label: "脚本名称" }, () =>
                    h(ElInput, {
                      modelValue: uploadForm.script_name,
                      "onUpdate:modelValue": (value: string) => {
                        uploadForm.script_name = value;
                      },
                      placeholder: "例如 shoppe_sync"
                    })
                  ),
                  h(ElFormItem, { label: "脚本版本" }, () =>
                    h(ElInput, {
                      modelValue: uploadForm.script_version,
                      "onUpdate:modelValue": (value: string) => {
                        uploadForm.script_version = value;
                      },
                      placeholder: "例如 v0.1.1"
                    })
                  ),
                  h(ElFormItem, { label: "zip 文件" }, () =>
                    h("div", { class: "upload-field" }, [
                      h("input", {
                        key: `upload-${uploadInputKey.value}`,
                        ref: uploadInputRef,
                        class: "upload-field__input",
                        type: "file",
                        accept: ".zip",
                        onChange: (event: Event) => {
                          const target = event.target as HTMLInputElement;
                          uploadFile.value = target.files?.[0] || null;
                        }
                      }),
                      h(
                        ElButton,
                        {
                          onClick: openUploadFilePicker
                        },
                        () => "选择 zip 文件"
                      ),
                      h("div", { class: "upload-field__filename" }, uploadFile.value ? uploadFile.value.name : "尚未选择文件"),
                      h(
                        ElCheckbox,
                        {
                          modelValue: uploadForm.force,
                          "onUpdate:modelValue": (value: boolean) => {
                            uploadForm.force = value;
                          }
                        },
                        () => "强制覆盖同名同版本脚本"
                      ),
                      h("div", { class: "upload-field__tip" }, "约束：zip 解压后的脚本版本根目录必须存在 index.js，否则上传失败。")
                    ])
                  )
                ]
              ),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(
                  ElButton,
                  {
                    onClick: () => {
                      uploadDialogVisible.value = false;
                    }
                  },
                  () => "取消"
                ),
                h(
                  ElButton,
                  {
                    type: "primary",
                    loading: uploading.value,
                    onClick: () => {
                      void handleUploadConfirm();
                    }
                  },
                  () => "确认上传"
                )
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: detailDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              detailDialogVisible.value = value;
            },
            title: selectedManifest.value ? `脚本详情：${selectedManifest.value.script_name}@${selectedManifest.value.script_version}` : "脚本详情",
            width: "820px",
            closeOnClickModal: true
          },
          {
            default: () =>
              selectedManifest.value
                ? [
                    h(
                      ElDescriptions,
                      {
                        class: "script-descriptions",
                        border: true,
                        column: 2
                      },
                      () => [
                        h(ElDescriptionsItem, { label: "脚本名称" }, () => selectedManifest.value?.script_name || "暂无"),
                        h(ElDescriptionsItem, { label: "脚本版本" }, () => selectedManifest.value?.script_version || "暂无"),
                        h(ElDescriptionsItem, { label: "入口文件" }, () => selectedManifest.value?.entry_file || "暂无"),
                        h(ElDescriptionsItem, { label: "下载地址" }, () => selectedManifest.value?.download_url || "暂无"),
                        h(ElDescriptionsItem, { label: "来源类型" }, () => selectedManifest.value?.source_type || "未知"),
                        h(ElDescriptionsItem, { label: "存储类型" }, () => selectedManifest.value?.storage_type || "未知"),
                        h(ElDescriptionsItem, { label: "校验值", span: 2 }, () => selectedManifest.value?.checksum_sha256 || "暂无")
                      ]
                    ),
                    h(ElAlert, {
                      class: "detail-card__notice",
                      type: "info",
                      title: "真机直接调试建议使用 index_debug.js；任务下发时 Agent 会按 manifest 和 download 接口自行拉取脚本。",
                      showIcon: true,
                      closable: false
                    }),
                    h("div", { class: "file-list" }, [
                      h("div", { class: "file-list__title" }, "文件清单"),
                      h(
                        ElScrollbar,
                        {
                          maxHeight: 320
                        },
                        () =>
                          h(
                            "div",
                            { class: "file-list__items" },
                            (selectedManifest.value?.files || []).map((item) =>
                              h("div", { key: item.relative_path, class: "file-list__item" }, [
                                h("div", { class: "file-list__path" }, item.relative_path),
                                h("div", { class: "file-list__hash" }, item.checksum_sha256)
                              ])
                            )
                          )
                      )
                    ])
                  ]
                : h(ElEmpty, {
                    description: "暂无可展示的脚本详情"
                  }),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(
                  ElButton,
                  {
                    onClick: () => {
                      detailDialogVisible.value = false;
                    }
                  },
                  () => "关闭"
                )
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: referencesDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              referencesDialogVisible.value = value;
            },
            title: selectedReferenceVersion.value
              ? `工作流引用：${selectedReferenceVersion.value.scriptName}@${selectedReferenceVersion.value.scriptVersion}`
              : "工作流引用",
            width: "760px",
            closeOnClickModal: true
          },
          {
            default: () =>
              selectedReferenceVersion.value && selectedReferenceVersion.value.references.length > 0
                ? h(
                    ElScrollbar,
                    {
                      maxHeight: 420
                    },
                    () =>
                      h(
                        "div",
                        { class: "script-references-dialog" },
                        selectedReferenceVersion.value.references.map((item) =>
                          h("div", { key: `${item.workflow_def_id}-${item.node_id}`, class: "script-references-dialog__item" }, [
                            h("div", { class: "script-references-dialog__title" }, item.workflow_name || item.workflow_def_id),
                            h("div", { class: "script-references-dialog__meta" }, `工作流ID：${item.workflow_def_id}`),
                            h("div", { class: "script-references-dialog__meta" }, `节点：${item.node_name || item.node_id}`),
                            h("div", { class: "script-references-dialog__meta" }, `节点ID：${item.node_id}`)
                          ])
                        )
                      )
                  )
                : h(ElEmpty, {
                    description: "当前没有工作流引用该脚本版本"
                  }),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(
                  ElButton,
                  {
                    onClick: () => {
                      referencesDialogVisible.value = false;
                    }
                  },
                  () => "关闭"
                )
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: deployDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              deployDialogVisible.value = value;
            },
            title: `批量下发脚本：${deployForm.script_name}@${deployForm.script_version}`,
            width: "760px",
            closeOnClickModal: false
          },
          {
            default: () => [
              h(ElAlert, {
                class: "dialog-alert",
                type: "info",
                title: "脚本下发会向所选在线设备逐台发送 sync_script 指令，Agent 再通过中心服务的 HTTP 接口拉取脚本文件。",
                showIcon: true,
                closable: false
              }),
              h(
                ElForm,
                {
                  labelPosition: "top",
                  class: "dialog-form"
                },
                () => [
                  h(ElFormItem, { label: "下发选项" }, () =>
                    h(
                      ElCheckbox,
                      {
                        modelValue: deployForm.force,
                        "onUpdate:modelValue": (value: boolean) => {
                          deployForm.force = value;
                        }
                      },
                      () => "强制同步已存在版本"
                    )
                  ),
                  h(ElFormItem, { label: `选择设备（当前在线 ${onlineDevices.value.length} 台）` }, () =>
                    onlineDevices.value.length === 0
                      ? h(ElEmpty, {
                          description: "当前没有在线设备，暂时无法批量下发脚本。"
                        })
                      : h("div", { class: "device-selector" }, [
                          h("div", { class: "device-selector__actions" }, [
                            h(
                              ElButton,
                              {
                                size: "small",
                                type: "primary",
                                plain: true,
                                onClick: selectAllDeployDevices
                              },
                              () => "全部选择"
                            ),
                            h(
                              ElButton,
                              {
                                size: "small",
                                onClick: clearDeployDevices
                              },
                              () => "清空选择"
                            )
                          ]),
                          h(
                            "div",
                            { class: "device-checkbox-group" },
                            onlineDevices.value.map((item) => {
                              const checked = deployDeviceIDs.value.includes(item.device_id);
                              return h(
                                "button",
                                {
                                  key: item.device_id,
                                  type: "button",
                                  class: ["device-checkbox-card", checked ? "device-checkbox-card--checked" : ""],
                                  onClick: () => {
                                    toggleDeployDevice(item.device_id);
                                  }
                                },
                                [
                                  h("div", { class: "device-checkbox-card__indicator" }, [
                                    checked
                                      ? h(
                                          ElTag,
                                          {
                                            size: "small",
                                            type: "success",
                                            effect: "dark",
                                            round: true
                                          },
                                          () => "已选"
                                        )
                                      : h("span", { class: "device-checkbox-card__dot" })
                                  ]),
                                  h("div", { class: "device-checkbox-card__content" }, [
                                    h("div", { class: "device-checkbox-card__title" }, getDeviceLabel(item)),
                                    h("div", { class: "device-checkbox-card__meta" }, `${item.brand || "未知品牌"} / ${item.model || "未知型号"} / ${item.agent_uuid || "无 agent_uuid"}`)
                                  ])
                                ]
                              );
                            })
                          )
                        ])
                  )
                ]
              ),
              deployLogs.value.length > 0
                ? h("div", { class: "deploy-results" }, [
                    h("div", { class: "deploy-results__title" }, "下发结果"),
                    ...deployLogs.value.map((item) =>
                      h(ElAlert, {
                        key: `${item.device_id}-${item.message}`,
                        class: "deploy-results__item",
                        type: item.success ? "success" : "error",
                        title: `${item.device_id}：${item.message}`,
                        showIcon: true,
                        closable: false
                      })
                    )
                  ])
                : null
            ],
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(
                  ElButton,
                  {
                    type: "success",
                    plain: true,
                    disabled: onlineDevices.value.length === 0,
                    loading: deploying.value,
                    onClick: () => {
                      void handleDeployToAllOnlineDevices();
                    }
                  },
                  () => "全部下发"
                ),
                h(
                  ElButton,
                  {
                    onClick: () => {
                      deployDialogVisible.value = false;
                    }
                  },
                  () => "取消"
                ),
                h(
                  ElButton,
                  {
                    type: "primary",
                    loading: deploying.value,
                    disabled: onlineDevices.value.length === 0,
                    onClick: () => {
                      void handleDeployConfirm();
                    }
                  },
                  () => "开始下发"
                )
              ])
          }
        )
      ]);
  }
});

// @ts-nocheck
import {
  ElButton,
  ElCard,
  ElCheckbox,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElScrollbar,
  ElSelect,
  ElTable,
  ElTableColumn,
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { fetchAllDevices, fetchLocationNodes } from "../../api/devices";
import { useNoticesStore } from "../../stores/notices";
import { useScriptsStore } from "../../stores/scripts";
import type { DeviceRecord, LocationNodeRecord } from "../../types/device";
import type { ScriptVersionRecord } from "../../types/script";
import { formatDateTime } from "../../utils/device";

const SCRIPT_NAME_PATTERN = /^[A-Za-z0-9_]+$/;
const PAGE_SIZES = [10, 20, 30, 50, 100];

type DeployDeviceTreeNode = {
  node_key: string;
  node_type: "zone" | "row" | "slot";
  zone_id: string;
  zone_name: string;
  row_id: string;
  row_name: string;
  slot_id: string;
  slot_name: string;
  device_id: string;
  device_name: string;
  device_status: string;
  children?: DeployDeviceTreeNode[];
};

function buildDeployDeviceTree(locationNodes: LocationNodeRecord[], devices: DeviceRecord[]): DeployDeviceTreeNode[] {
  const zoneMap = new Map<string, LocationNodeRecord>();
  const rowMap = new Map<string, LocationNodeRecord>();
  const slotMap = new Map<string, LocationNodeRecord>();
  for (const node of locationNodes) {
    if (node.node_type === "zone") {
      zoneMap.set(node.node_id, node);
    } else if (node.node_type === "row") {
      rowMap.set(node.node_id, node);
    } else if (node.node_type === "slot") {
      slotMap.set(node.node_id, node);
    }
  }

  const sortByOrder = <T extends { sort_order?: number; node_name?: string }>(list: T[]) =>
    [...list].sort((left, right) => {
      const orderDiff = Number(left.sort_order || 0) - Number(right.sort_order || 0);
      if (orderDiff !== 0) {
        return orderDiff;
      }
      return String(left.node_name || "").localeCompare(String(right.node_name || ""), "zh-CN");
    });

  const zoneNodes = sortByOrder(Array.from(zoneMap.values()));
  const tree: DeployDeviceTreeNode[] = [];

  for (const zone of zoneNodes) {
    const zoneTreeNode: DeployDeviceTreeNode = {
      node_key: `zone:${zone.node_id}`,
      node_type: "zone",
      zone_id: zone.node_id,
      zone_name: zone.node_name,
      row_id: "",
      row_name: "",
      slot_id: "",
      slot_name: "",
      device_id: "",
      device_name: "",
      device_status: "",
      children: []
    };

    const rows = sortByOrder(Array.from(rowMap.values()).filter((item) => item.parent_id === zone.node_id));
    for (const row of rows) {
      const rowTreeNode: DeployDeviceTreeNode = {
        node_key: `row:${zone.node_id}:${row.node_id}`,
        node_type: "row",
        zone_id: zone.node_id,
        zone_name: zone.node_name,
        row_id: row.node_id,
        row_name: row.node_name,
        slot_id: "",
        slot_name: "",
        device_id: "",
        device_name: "",
        device_status: "",
        children: []
      };

      const slots = sortByOrder(Array.from(slotMap.values()).filter((item) => item.parent_id === row.node_id));
      for (const slot of slots) {
        const slotTreeNode: DeployDeviceTreeNode = {
          node_key: `slot:${slot.node_id}`,
          node_type: "slot",
          zone_id: zone.node_id,
          zone_name: zone.node_name,
          row_id: row.node_id,
          row_name: row.node_name,
          slot_id: slot.node_id,
          slot_name: slot.node_name,
          device_id: "",
          device_name: "",
          device_status: "",
          children: []
        };

        const boundDevices = devices
          .filter((item) => item.slot_position_id === slot.node_id)
          .sort((left, right) => left.device_id.localeCompare(right.device_id, "zh-CN"));
        const boundDevice = boundDevices[0];
        if (boundDevice) {
          slotTreeNode.device_id = boundDevice.device_id;
          slotTreeNode.device_name = boundDevice.device_name;
          slotTreeNode.device_status = boundDevice.status;
        }

        slotTreeNode.children = undefined;

        rowTreeNode.children?.push(slotTreeNode);
      }

      zoneTreeNode.children?.push(rowTreeNode);
    }

    tree.push(zoneTreeNode);
  }

  return tree;
}

export const ScriptsPage = defineComponent({
  name: "ScriptsPage",
  setup() {
    const scriptsStore = useScriptsStore();
    const noticesStore = useNoticesStore();
    const { scriptNames, loading, uploading, errorMessage, page, pageSize, total } = storeToRefs(scriptsStore);

    const searchKeyword = ref("");
    const selectedScriptName = ref("");
    const createDialogVisible = ref(false);
    const uploadDialogVisible = ref(false);
    const uploadDialogMode = ref<"create" | "update">("create");
    const deployDialogVisible = ref(false);
    const deployingSelectedDevices = ref(false);
    const uploadFile = ref<File | null>(null);
    const uploadInputKey = ref(0);
    const uploadInputRef = ref<HTMLInputElement | null>(null);
    const locationNodes = ref<LocationNodeRecord[]>([]);
    const devices = ref<DeviceRecord[]>([]);
    const deploySelectionKeys = ref<string[]>([]);
    const selectedDeployVersion = ref<ScriptVersionRecord | null>(null);

    const createForm = reactive({ script_name: "" });
    const uploadForm = reactive({
      script_name: "",
      script_version: "",
      force: false
    });
    const deployForm = reactive({
      force: true
    });

    const filteredScriptNames = computed(() => {
      const keyword = searchKeyword.value.trim().toLowerCase();
      if (!keyword) {
        return scriptNames.value;
      }
      return scriptNames.value.filter((item) => item.script_name.toLowerCase().includes(keyword));
    });

    const selectedScript = computed(
      () => scriptNames.value.find((item) => item.script_name === selectedScriptName.value) || null
    );

    const selectedVersions = computed<ScriptVersionRecord[]>(() =>
      scriptsStore.flattenedVersions.filter((item) => item.script_name === selectedScriptName.value)
    );
    const uploadDialogTitle = computed(() => (uploadDialogMode.value === "update" ? "更新脚本版本" : "添加脚本版本"));
    const deployTree = computed(() => buildDeployDeviceTree(locationNodes.value, devices.value));

    async function loadPageData() {
      await Promise.all([scriptsStore.loadScriptNames(), scriptsStore.loadScripts()]);
      if (!selectedScriptName.value && scriptNames.value.length > 0) {
        selectedScriptName.value = scriptNames.value[0].script_name;
      }
      if (selectedScriptName.value && !scriptNames.value.some((item) => item.script_name === selectedScriptName.value)) {
        selectedScriptName.value = scriptNames.value[0]?.script_name || "";
      }
    }

    async function loadDeploySupportData() {
      locationNodes.value = await fetchLocationNodes();
      devices.value = await fetchAllDevices();
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(errorMessage, (value, previousValue) => {
      if (value && value !== previousValue) {
        noticesStore.error(value, 5000);
      }
    });

    function openCreateDialog() {
      createForm.script_name = "";
      createDialogVisible.value = true;
    }

    function openUploadDialog(scriptName = selectedScriptName.value) {
      uploadDialogMode.value = "create";
      uploadForm.script_name = scriptName;
      uploadForm.script_version = "";
      uploadForm.force = false;
      uploadFile.value = null;
      uploadInputKey.value += 1;
      uploadDialogVisible.value = true;
    }

    function openUpdateDialog(scriptName: string, scriptVersion: string) {
      uploadDialogMode.value = "update";
      uploadForm.script_name = scriptName;
      uploadForm.script_version = scriptVersion;
      uploadForm.force = true;
      uploadFile.value = null;
      uploadInputKey.value += 1;
      uploadDialogVisible.value = true;
    }

    async function openDeployDialog(version: ScriptVersionRecord) {
      selectedDeployVersion.value = version;
      deploySelectionKeys.value = [];
      deployForm.force = true;
      try {
        await loadDeploySupportData();
        deployDialogVisible.value = true;
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "加载设备树失败", 5000);
      }
    }

    function openUploadFilePicker() {
      uploadInputRef.value?.click();
    }

    function toggleDeploySelection(node: DeployDeviceTreeNode, checked: boolean) {
      const next = new Set(deploySelectionKeys.value);
      const applyNode = (current: DeployDeviceTreeNode) => {
        if (current.node_type === "slot" && current.device_id.trim()) {
          if (checked) {
            next.add(current.node_key);
          } else {
            next.delete(current.node_key);
          }
        }
        for (const child of current.children || []) {
          applyNode(child);
        }
      };
      applyNode(node);
      deploySelectionKeys.value = Array.from(next);
    }

    function collectSelectedDeviceIDs(): string[] {
      const selectedKeys = new Set(deploySelectionKeys.value);
      const ids = new Set<string>();
      const visit = (list: DeployDeviceTreeNode[]) => {
        for (const item of list) {
          if (item.node_type === "slot" && selectedKeys.has(item.node_key) && item.device_id.trim()) {
            ids.add(item.device_id.trim());
          }
          if (item.children?.length) {
            visit(item.children);
          }
        }
      };
      visit(deployTree.value);
      return Array.from(ids);
    }

    async function handleDeployConfirm() {
      if (!selectedDeployVersion.value) {
        noticesStore.warning("请先选择要下发的脚本版本", 5000);
        return;
      }
      const deviceIDs = collectSelectedDeviceIDs();
      if (deviceIDs.length === 0) {
        noticesStore.warning("请至少选择一台设备", 5000);
        return;
      }

      deployingSelectedDevices.value = true;
      let successCount = 0;
      const failedDevices: string[] = [];

      try {
        for (const deviceID of deviceIDs) {
          try {
            await scriptsStore.triggerScriptDeploy(
              deviceID,
              selectedDeployVersion.value.script_name,
              selectedDeployVersion.value.script_version,
              deployForm.force
            );
            successCount += 1;
          } catch (_error) {
            failedDevices.push(deviceID);
          }
        }
      } finally {
        deployingSelectedDevices.value = false;
      }

      if (failedDevices.length === 0) {
        deployDialogVisible.value = false;
        noticesStore.success(
          `已下发 ${selectedDeployVersion.value.script_name}@${selectedDeployVersion.value.script_version} 到 ${successCount} 台设备`,
          3000
        );
        return;
      }

      noticesStore.warning(
        `成功 ${successCount} 台，失败 ${failedDevices.length} 台：${failedDevices.slice(0, 5).join("、")}${failedDevices.length > 5 ? "..." : ""}`,
        5000
      );
    }

    async function handleCreateScriptName() {
      const scriptName = createForm.script_name.trim();
      if (!SCRIPT_NAME_PATTERN.test(scriptName)) {
        noticesStore.warning("脚本名称只能包含英文、数字和下划线", 5000);
        return;
      }
      if (scriptNames.value.some((item) => item.script_name === scriptName)) {
        noticesStore.warning("脚本名称已存在", 5000);
        return;
      }
      await scriptsStore.submitScriptName(scriptName);
      createDialogVisible.value = false;
      createForm.script_name = "";
      selectedScriptName.value = scriptName;
      await loadPageData();
    }

    async function handleUploadConfirm() {
      const scriptName = uploadForm.script_name.trim();
      const scriptVersion = uploadForm.script_version.trim();
      if (!SCRIPT_NAME_PATTERN.test(scriptName)) {
        noticesStore.warning("脚本名称只能包含英文、数字和下划线", 5000);
        return;
      }
      if (!scriptVersion) {
        noticesStore.warning("请先填写脚本版本", 5000);
        return;
      }
      if (!uploadFile.value) {
        noticesStore.warning("请先选择 zip 脚本包", 5000);
        return;
      }

      await scriptsStore.submitScriptUpload({
        script_name: scriptName,
        script_version: scriptVersion,
        source_type: "zip",
        force: uploadDialogMode.value === "update" ? true : uploadForm.force,
        file: uploadFile.value
      });
      uploadDialogVisible.value = false;
      selectedScriptName.value = scriptName;
      await loadPageData();
      noticesStore.success(
        uploadDialogMode.value === "update"
          ? `脚本版本 ${scriptName}@${scriptVersion} 已更新`
          : `脚本版本 ${scriptName}@${scriptVersion} 已添加`,
        3000
      );
    }

    async function handleDeleteScript(scriptName: string) {
      try {
        await ElMessageBox.confirm(`确认删除脚本 ${scriptName} 吗？`, "删除脚本", {
          type: "warning",
          confirmButtonText: "确认删除",
          cancelButtonText: "取消"
        });
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        throw error;
      }
      await scriptsStore.removeScript(scriptName);
      await loadPageData();
      selectedScriptName.value = scriptNames.value[0]?.script_name || "";
    }

    async function handleDeleteVersion(scriptName: string, scriptVersion: string) {
      try {
        await ElMessageBox.confirm(`确认删除脚本版本 ${scriptName}@${scriptVersion} 吗？`, "删除脚本版本", {
          type: "warning",
          confirmButtonText: "确认删除",
          cancelButtonText: "取消"
        });
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        throw error;
      }
      await scriptsStore.removeScriptVersion(scriptName, scriptVersion);
      await loadPageData();
    }

    return () =>
      h("section", { class: "app-page scripts-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(ElButton, { type: "primary", onClick: openCreateDialog }, () => "添加脚本名称"),
          h(ElButton, { loading: loading.value, onClick: () => void loadPageData() }, () => "刷新")
        ]),
        h("section", { class: "scripts-page__layout" }, [
          h(
            ElCard,
            { class: "page-card scripts-page__sidebar", shadow: "never" },
            {
              default: () => [
                h("div", { class: "scripts-page__search" }, [
                  h(ElInput, {
                    modelValue: searchKeyword.value,
                    "onUpdate:modelValue": (value: string) => (searchKeyword.value = value),
                    placeholder: "搜索脚本名称",
                    clearable: true
                  })
                ]),
                h(
                  ElScrollbar,
                  { class: "scripts-page__sidebar-list" },
                  () =>
                    filteredScriptNames.value.length === 0
                      ? h(ElEmpty, { description: scriptNames.value.length === 0 ? "当前还没有脚本名称" : "没有匹配的脚本名称" })
                      : filteredScriptNames.value.map((item) =>
                          h(
                            "button",
                            {
                              key: item.script_name,
                              type: "button",
                              class: [
                                "scripts-page__name-item",
                                selectedScriptName.value === item.script_name ? "scripts-page__name-item--active" : ""
                              ],
                              onClick: () => (selectedScriptName.value = item.script_name)
                            },
                            [
                              h("span", { class: "scripts-page__name-title" }, item.script_name),
                              h(
                                "a",
                                {
                                  class: "scripts-page__name-delete",
                                  onClick: (event: MouseEvent) => {
                                    event.stopPropagation();
                                    void handleDeleteScript(item.script_name);
                                  }
                                },
                                "删除"
                              )
                            ]
                          )
                        )
                )
              ]
            }
          ),
          h(
            ElCard,
            { class: "page-card scripts-page__content", shadow: "never" },
            {
              default: () =>
                selectedScript.value
                  ? h("div", { class: "scripts-page__content-body" }, [
                      h("div", { class: "scripts-page__content-head" }, [
                        h("div", null, [
                          h("div", { class: "card-header__title" }, "版本管理"),
                          h("div", { class: "card-header__subtitle" }, `${selectedScript.value.script_name} 的全部版本`)
                        ]),
                        h(ElButton, { type: "primary", onClick: () => openUploadDialog(selectedScript.value?.script_name || "") }, () => "添加版本")
                      ]),
                      h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                        h(
                          ElTable,
                          { data: selectedVersions.value, stripe: true, class: "app-table", height: "100%" },
                          {
                            default: () => [
                              h(ElTableColumn, {
                                prop: "script_version",
                                label: "版本号",
                                minWidth: 150
                              }),
                              h(ElTableColumn, {
                                prop: "entry_file",
                                label: "入口文件",
                                minWidth: 180
                              }),
                              h(ElTableColumn, {
                                prop: "status",
                                label: "状态",
                                width: 100
                              }),
                              h(ElTableColumn, {
                                prop: "created_at",
                                label: "创建时间",
                                minWidth: 180,
                                formatter: (_row: unknown, _column: unknown, value: string) => formatDateTime(value)
                              }),
                              h(
                                ElTableColumn,
                                { label: "操作", width: 220, fixed: "right" },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) =>
                                    h("div", { class: "table-actions table-actions--nowrap" }, [
                                      h(ElButton, { link: true, type: "primary", onClick: () => void openDeployDialog(row) }, () => "设备下发"),
                                      h(ElButton, { link: true, type: "primary", onClick: () => openUpdateDialog(row.script_name, row.script_version) }, () => "更新"),
                                      h(ElButton, { link: true, type: "danger", onClick: () => void handleDeleteVersion(row.script_name, row.script_version) }, () => "删除")
                                    ])
                                }
                              )
                            ]
                          }
                        )
                      ])
                    ])
                  : h(ElEmpty, { description: "请先在左侧选择一个脚本名称" })
            }
          )
        ]),
        h(
          ElDialog,
          {
            modelValue: createDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (createDialogVisible.value = value),
            title: "添加脚本名称",
            width: "520px"
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "脚本名称" }, () =>
                  h(ElInput, {
                    modelValue: createForm.script_name,
                    "onUpdate:modelValue": (value: string) => (createForm.script_name = value),
                    placeholder: "只能包含英文、数字和下划线"
                  })
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (createDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", onClick: () => void handleCreateScriptName() }, () => "确认")
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: uploadDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (uploadDialogVisible.value = value),
            title: uploadDialogTitle.value,
            width: "560px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "脚本名称" }, () =>
                  h(ElInput, { modelValue: uploadForm.script_name, readonly: true })
                ),
                h(ElFormItem, { label: "脚本版本" }, () =>
                  h(ElInput, {
                    modelValue: uploadForm.script_version,
                    "onUpdate:modelValue": (value: string) => (uploadForm.script_version = value),
                    placeholder: "例如 v0.1.1",
                    readonly: uploadDialogMode.value === "update"
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
                    h(ElButton, { onClick: openUploadFilePicker }, () => "选择 zip 文件"),
                    h("div", { class: "upload-field__filename" }, uploadFile.value ? uploadFile.value.name : "尚未选择文件"),
                    h(
                      ElSelect,
                      {
                        modelValue: uploadForm.force,
                        "onUpdate:modelValue": (value: boolean) => (uploadForm.force = value),
                        placeholder: "强制覆盖",
                        style: "display:none"
                      },
                      () => []
                    )
                  ])
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (uploadDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: uploading.value, onClick: () => void handleUploadConfirm() }, () =>
                  uploadDialogMode.value === "update" ? "确认更新" : "确认上传"
                )
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: deployDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (deployDialogVisible.value = value),
            title: selectedDeployVersion.value
              ? `设备下发：${selectedDeployVersion.value.script_name}@${selectedDeployVersion.value.script_version}`
              : "设备下发",
            width: "980px",
            closeOnClickModal: false
          },
          {
            default: () =>
                h("div", { class: "scripts-page__deploy-dialog" }, [
                h("div", { class: "scripts-page__deploy-tip" }, "可直接勾选分区、排或槽位。勾选上级会自动选择下级已绑定设备。"),
                h("div", { class: "table-scroll-region table-scroll-region--soft", style: "height: 420px;" }, [
                  deployTree.value.length === 0
                    ? h(ElEmpty, { description: "当前没有可下发的已绑定设备" })
                    : h(
                        ElTable,
                        {
                          data: deployTree.value,
                          rowKey: "node_key",
                          stripe: true,
                          border: false,
                          defaultExpandAll: true,
                          treeProps: { children: "children" },
                          class: "app-table",
                          height: "100%"
                        },
                        {
                          default: () => [
                            h(ElTableColumn, {
                              label: "选择",
                              width: 90
                            }, {
                              default: ({ row }: { row: DeployDeviceTreeNode }) =>
                                h(ElCheckbox, {
                                  modelValue:
                                    row.node_type === "slot"
                                      ? deploySelectionKeys.value.includes(row.node_key)
                                      : (row.children || []).some((child) => {
                                          const visit = (node: DeployDeviceTreeNode): boolean => {
                                            if (node.node_type === "slot" && deploySelectionKeys.value.includes(node.node_key)) {
                                              return true;
                                            }
                                            return (node.children || []).some(visit);
                                          };
                                          return visit(child);
                                        }),
                                  "onUpdate:modelValue": (value: boolean) => toggleDeploySelection(row, value)
                                })
                            }),
                            h(ElTableColumn, {
                              label: "层级",
                              width: 100,
                              formatter: (row: DeployDeviceTreeNode) =>
                                row.node_type === "zone"
                                  ? "分区"
                                  : row.node_type === "row"
                                    ? "排"
                                    : "槽位"
                            }),
                            h(ElTableColumn, {
                              label: "名称",
                              minWidth: 260,
                              formatter: (row: DeployDeviceTreeNode) => {
                                if (row.node_type === "zone") {
                                  return row.zone_name;
                                }
                                if (row.node_type === "row") {
                                  return row.row_name;
                                }
                                if (row.node_type === "slot") {
                                  return row.slot_name;
                                }
                                return "";
                              }
                            }),
                            h(ElTableColumn, {
                              label: "路径",
                              minWidth: 260,
                              formatter: (row: DeployDeviceTreeNode) => {
                                if (row.node_type === "zone") {
                                  return row.zone_name;
                                }
                                if (row.node_type === "row") {
                                  return `${row.zone_name}-${row.row_name}`;
                                }
                                if (row.node_type === "slot") {
                                  return `${row.zone_name}-${row.row_name}-${row.slot_name}`;
                                }
                                return "";
                              }
                            }),
                            h(ElTableColumn, {
                              label: "设备 ID",
                              width: 120,
                              formatter: (row: DeployDeviceTreeNode) => (row.node_type === "slot" ? row.device_id : "")
                            }),
                            h(ElTableColumn, {
                              label: "在线状态",
                              width: 120,
                              formatter: (row: DeployDeviceTreeNode) => (row.node_type === "slot" ? row.device_status || "" : "")
                            })
                          ]
                        }
                      )
                ])
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (deployDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: deployingSelectedDevices.value, onClick: () => void handleDeployConfirm() }, () => "下发")
              ])
          }
        )
      ]);
  }
});

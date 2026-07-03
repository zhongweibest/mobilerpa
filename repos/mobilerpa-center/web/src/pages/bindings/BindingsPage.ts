// @ts-nocheck
import {
  ElButton,
  ElCard,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessageBox,
  ElOption,
  ElSelect,
  ElTag,
  ElTable,
  ElTableColumn,
  ElTree
} from "element-plus";
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";

import { bindLocationNode, createLocationNode, deleteLocationNode, fetchAllDevices, fetchLocationNodes, unbindLocationNode, updateLocationNode } from "../../api/devices";
import { useNoticesStore } from "../../stores/notices";

function normalizeText(value: string) {
  return (value || "").trim().toLowerCase();
}

function resolveNodeTypeLabel(nodeType: string) {
  if (nodeType === "zone") {
    return "分区";
  }
  if (nodeType === "row") {
    return "排号";
  }
  if (nodeType === "slot") {
    return "槽位";
  }
  return nodeType || "-";
}

function resolveNextNodeType(node) {
  if (!node) {
    return "zone";
  }
  if (node.node_type === "zone") {
    return "row";
  }
  if (node.node_type === "row") {
    return "slot";
  }
  return "";
}

function resolveCreatePlaceholder(nodeType: string) {
  if (nodeType === "zone") {
    return "例如：A区";
  }
  if (nodeType === "row") {
    return "例如：第1排";
  }
  if (nodeType === "slot") {
    return "例如：01";
  }
  return "";
}

function buildNodeTitle(node) {
  if (node.node_type === "slot") {
    return node.slot_name || node.node_name || "未命名槽位";
  }
  return node.node_name || "未命名节点";
}

function buildNodePath(node) {
  return node.path_text || [node.zone_name, node.row_name, node.slot_name].map((item) => (item || "").trim()).filter((item) => item !== "").join(" / ");
}

function buildNodeCode(node) {
  if (!node) {
    return "";
  }
  return [node.zone_name, node.row_name, node.slot_name].map((item) => (item || "").trim()).filter((item) => item !== "").join("-");
}

export const BindingsPage = defineComponent({
  name: "BindingsPage",
  setup() {
    const noticesStore = useNoticesStore();
    const loading = ref(false);
    const saving = ref(false);
    const nodes = ref([]);
    const devices = ref([]);
    const keyword = ref("");
    const currentNodeID = ref("");
    const zoneDialogVisible = ref(false);
    const rowDialogVisible = ref(false);
    const slotDialogVisible = ref(false);

    const createForm = reactive({
      node_name: "",
      sort_order: ""
    });
    const editForm = reactive({
      parent_id: "",
      node_name: "",
      sort_order: ""
    });
    const bindForm = reactive({
      device_id: ""
    });

    const currentNode = computed(() => nodes.value.find((item) => item.node_id === currentNodeID.value) || null);
    const currentParentNode = computed(() => {
      if (!currentNode.value || !currentNode.value.parent_id) {
        return null;
      }
      return nodes.value.find((item) => item.node_id === currentNode.value.parent_id) || null;
    });
    const directChildren = computed(() =>
      currentNode.value ? nodes.value.filter((item) => item.parent_id === currentNode.value.node_id) : nodes.value.filter((item) => item.node_type === "zone")
    );
    const nextNodeType = computed(() => resolveNextNodeType(currentNode.value));
    const canCreateChild = computed(() => nextNodeType.value !== "");
    const canBindDevice = computed(() => currentNode.value?.node_type === "slot");
    const usesRowTable = computed(() => currentNode.value?.node_type === "zone");
    const usesSlotTable = computed(() => currentNode.value?.node_type === "row");
    const editableParentOptions = computed(() => {
      if (!currentNode.value) {
        return [];
      }
      if (currentNode.value.node_type === "row") {
        return nodes.value.filter((item) => item.node_type === "zone");
      }
      if (currentNode.value.node_type === "slot") {
        return nodes.value.filter((item) => item.node_type === "row");
      }
      return [];
    });

    function syncFormsFromNode(node) {
      bindForm.device_id = node?.node_type === "slot" ? node.device_id || "" : "";
      createForm.node_name = "";
      createForm.sort_order = "";
      editForm.parent_id = node?.parent_id || "";
      editForm.node_name = node?.node_name || "";
      editForm.sort_order = node ? String(node.sort_order ?? 0) : "";
    }

    const treeData = computed(() => {
      const childrenMap = new Map();
      for (const item of nodes.value) {
        const key = item.parent_id || "__root__";
        const list = childrenMap.get(key) || [];
        list.push(item);
        childrenMap.set(key, list);
      }

      const sortNodes = (list) =>
        [...list].sort((left, right) => {
          const sortDelta = Number(left.sort_order || 0) - Number(right.sort_order || 0);
          if (sortDelta !== 0) {
            return sortDelta;
          }
          return String(left.node_name || "").localeCompare(String(right.node_name || ""), "zh-CN");
        });

      const makeTree = (parentID) =>
        sortNodes(childrenMap.get(parentID || "__root__") || []).map((item) => ({
          ...item,
          label: buildNodeTitle(item),
          children: makeTree(item.node_id)
        }));

      return makeTree("");
    });

    const filteredTreeData = computed(() => {
      const normalizedKeyword = normalizeText(keyword.value);
      if (normalizedKeyword === "") {
        return treeData.value;
      }

      const filterTree = (items) =>
        items
          .map((item) => {
            const children = filterTree(item.children || []);
            const matched =
              normalizeText(item.label).includes(normalizedKeyword) ||
              normalizeText(item.path_text).includes(normalizedKeyword) ||
              normalizeText(item.device_id).includes(normalizedKeyword);
            if (matched || children.length > 0) {
              return {
                ...item,
                children
              };
            }
            return null;
          })
          .filter(Boolean);

      return filterTree(treeData.value);
    });

    async function loadData() {
      loading.value = true;
      try {
        const [locationNodes, devicePage] = await Promise.all([
          fetchLocationNodes(),
          fetchAllDevices()
        ]);
        nodes.value = locationNodes;
        devices.value = devicePage || [];

        if (currentNodeID.value && !locationNodes.find((item) => item.node_id === currentNodeID.value)) {
          currentNodeID.value = "";
        }
        if (currentNodeID.value === "" && locationNodes.length > 0) {
          currentNodeID.value = locationNodes[0].node_id;
        }
        syncFormsFromNode(locationNodes.find((item) => item.node_id === currentNodeID.value) || null);
      } finally {
        loading.value = false;
      }
    }

    onMounted(() => {
      void loadData();
    });

    function handleSelectNode(data) {
      currentNodeID.value = data?.node_id || "";
      syncFormsFromNode(data || null);
    }

    async function handleCreateChild() {
      if (nextNodeType.value === "") {
        noticesStore.warning("槽位节点下不能继续新增下级", 5000);
        return;
      }
      if (createForm.node_name.trim() === "") {
        noticesStore.warning(`请先填写${resolveNodeTypeLabel(nextNodeType.value)}名称`, 5000);
        return;
      }

      saving.value = true;
      try {
        await createLocationNode({
          parent_id: currentNode.value ? currentNode.value.node_id : "",
          node_type: nextNodeType.value,
          node_name: createForm.node_name.trim(),
          sort_order: createForm.sort_order.trim() === "" ? undefined : Number(createForm.sort_order)
        });
        createForm.node_name = "";
        createForm.sort_order = "";
        noticesStore.success(`${resolveNodeTypeLabel(nextNodeType.value)}已创建`, 3000);
        await loadData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "创建节点失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    function handleOpenCreateRowDialog() {
      createForm.node_name = "";
      createForm.sort_order = "";
      rowDialogVisible.value = true;
    }

    function handleOpenCreateSlotDialog() {
      createForm.node_name = "";
      createForm.sort_order = "";
      slotDialogVisible.value = true;
    }

    async function handleCreateRow() {
      if (!currentNode.value || currentNode.value.node_type !== "zone") {
        noticesStore.warning("请先选择分区节点", 5000);
        return;
      }
      await handleCreateChild();
      rowDialogVisible.value = false;
    }

    async function handleCreateSlot() {
      if (!currentNode.value || currentNode.value.node_type !== "row") {
        noticesStore.warning("请先选择排号节点", 5000);
        return;
      }
      await handleCreateChild();
      slotDialogVisible.value = false;
    }

    async function handleCreateZone() {
      if (createForm.node_name.trim() === "") {
        noticesStore.warning("请先填写分区名称", 5000);
        return;
      }

      saving.value = true;
      try {
        const result = await createLocationNode({
          parent_id: "",
          node_type: "zone",
          node_name: createForm.node_name.trim(),
          sort_order: createForm.sort_order.trim() === "" ? undefined : Number(createForm.sort_order)
        });
        createForm.node_name = "";
        createForm.sort_order = "";
        zoneDialogVisible.value = false;
        noticesStore.success("分区已创建", 3000);
        await loadData();
        handleSelectNode(result);
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "创建分区失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    async function handleUpdateNode() {
      if (!currentNode.value) {
        noticesStore.warning("请先选择要编辑的节点", 5000);
        return;
      }
      if (editForm.node_name.trim() === "") {
        noticesStore.warning("请先填写节点名称", 5000);
        return;
      }
      if (currentNode.value.node_type !== "zone" && editForm.parent_id.trim() === "") {
        noticesStore.warning("请先选择父节点", 5000);
        return;
      }

      try {
        await ElMessageBox.confirm(
          `确认保存${resolveNodeTypeLabel(currentNode.value.node_type)}“${editForm.node_name.trim()}”的节点信息吗？`,
          "保存节点确认",
          {
            confirmButtonText: "确认保存",
            cancelButtonText: "取消",
            type: "warning"
          }
        );
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
      }

      saving.value = true;
      try {
        const result = await updateLocationNode(currentNode.value.node_id, {
          parent_id: currentNode.value.node_type === "zone" ? "" : editForm.parent_id.trim(),
          node_name: editForm.node_name.trim(),
          sort_order: editForm.sort_order.trim() === "" ? 0 : Number(editForm.sort_order)
        });
        noticesStore.success("节点已更新", 3000);
        await loadData();
        handleSelectNode(result);
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "更新节点失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    async function handleDeleteNode(targetNode = currentNode.value) {
      if (!targetNode) {
        noticesStore.warning("请先选择要删除的节点", 5000);
        return;
      }

      try {
        await ElMessageBox.confirm(
          `确认删除${resolveNodeTypeLabel(targetNode.node_type)}“${buildNodeTitle(targetNode)}”吗？其下所有子节点会一起删除，槽位上的设备绑定也会被清空。`,
          "删除节点确认",
          {
            confirmButtonText: "确认删除",
            cancelButtonText: "取消",
            type: "warning"
          }
        );
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
      }

      saving.value = true;
      try {
        const deletingNodeID = targetNode.node_id;
        const fallbackNodeID =
          currentNode.value?.node_id === targetNode.node_id ? currentParentNode.value?.node_id || "" : currentNodeID.value;
        await deleteLocationNode(deletingNodeID);
        noticesStore.success("节点已删除", 3000);
        currentNodeID.value = fallbackNodeID;
        bindForm.device_id = "";
        createForm.node_name = "";
        createForm.sort_order = "";
        editForm.parent_id = "";
        editForm.node_name = "";
        editForm.sort_order = "";
        await loadData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "删除节点失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    async function handleBind() {
      if (!currentNode.value || currentNode.value.node_type !== "slot") {
        noticesStore.warning("请先选择槽位节点", 5000);
        return;
      }
      if (bindForm.device_id.trim() === "") {
        noticesStore.warning("请先选择设备", 5000);
        return;
      }

      saving.value = true;
      try {
        await bindLocationNode(currentNode.value.node_id, {
          device_id: bindForm.device_id.trim()
        });
        noticesStore.success("设备绑定成功", 3000);
        await loadData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "绑定设备失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    async function handleUnbind() {
      if (!currentNode.value || currentNode.value.node_type !== "slot" || !currentNode.value.device_id) {
        noticesStore.warning("当前槽位没有已绑定设备", 5000);
        return;
      }

      saving.value = true;
      try {
        await unbindLocationNode(currentNode.value.node_id);
        bindForm.device_id = "";
        noticesStore.success("槽位解绑成功", 3000);
        await loadData();
      } catch (error) {
        noticesStore.error(error instanceof Error ? error.message : "解绑失败", 5000);
      } finally {
        saving.value = false;
      }
    }

    return () =>
      h("section", { class: "devices-page bindings-page" }, [
        h("div", { class: "bindings-page__layout" }, [
          h(
            ElCard,
            { class: "page-card bindings-page__tree-card", shadow: "never" },
            {
              default: () => [
                h("div", { class: "bindings-page__tree-search" }, [
                  h(
                    ElButton,
                    {
                      type: "primary",
                      onClick: () => {
                        createForm.node_name = "";
                        createForm.sort_order = "";
                        zoneDialogVisible.value = true;
                      }
                    },
                    () => "新增分区"
                  ),
                  h(ElInput, {
                    modelValue: keyword.value,
                    clearable: true,
                    placeholder: "输入关键字进行过滤",
                    "onUpdate:modelValue": (value: string) => {
                      keyword.value = value;
                    }
                  })
                ]),
                filteredTreeData.value.length === 0 && !loading.value
                  ? h(ElEmpty, { description: "没有匹配的位置节点" })
                  : h(ElTree, {
                      data: filteredTreeData.value,
                      nodeKey: "node_id",
                      defaultExpandAll: true,
                      expandOnClickNode: false,
                      highlightCurrent: true,
                      currentNodeKey: currentNodeID.value,
                      props: {
                        children: "children",
                        label: "label"
                      },
                      onNodeClick: handleSelectNode
                    }, {
                      default: ({ data }) =>
                        h("div", { class: "bindings-page__tree-node" }, [
                          h("span", { class: "bindings-page__tree-node-text" }, data.label),
                          h("div", { class: "bindings-page__tree-node-actions" }, [
                            data.node_type === "slot"
                              ? h(
                                  ElTag,
                                  {
                                    size: "small",
                                    type: data.device_id ? "success" : "info",
                                    effect: "light"
                                  },
                                  () => (data.device_id ? "已绑定" : "空槽位")
                                )
                              : null,
                            h(
                              "button",
                              {
                                type: "button",
                                class: "bindings-page__tree-delete",
                                onClick: (event: MouseEvent) => {
                                  event.stopPropagation();
                                  void handleDeleteNode(data);
                                }
                              },
                              "删除"
                            )
                          ])
                        ])
                    })
              ]
            }
          ),
          h("div", { class: "bindings-page__main" }, [
            h(
              ElCard,
              { class: "page-card bindings-page__detail-card", shadow: "never" },
              {
                default: () =>
                  !currentNode.value && nodes.value.length === 0 && !loading.value
                    ? h("div", { class: "bindings-page__empty-state" }, [
                        h("div", { class: "card-header__title" }, "还没有位置节点"),
                        h("div", { class: "card-header__subtitle" }, "点击左侧“新增分区”开始创建，后续再逐级新增排号和槽位。")
                      ])
                    : h("div", { class: "bindings-page__detail-scroll" }, [
                        currentNode.value
                          ? h(ElForm, { labelWidth: "110px", class: "bindings-page__detail-form" }, () => [
                              h("div", { class: "bindings-page__section-head" }, [
                                h("div", { class: "bindings-page__section-title" }, "节点信息"),
                                h(
                                  ElButton,
                                  {
                                    type: "warning",
                                    onClick: () => {
                                      void handleUpdateNode();
                                    }
                                  },
                                  () => "保存节点"
                                )
                              ]),
                              h("div", { class: "bindings-page__form-grid" }, [
                                h(
                                  ElFormItem,
                                  { label: "父节点" },
                                  () =>
                                    currentNode.value.node_type === "zone"
                                      ? h(ElInput, { modelValue: "根节点", readonly: true, class: "bindings-page__readonly-input" })
                                      : h(
                                          ElSelect,
                                          {
                                            modelValue: editForm.parent_id,
                                            placeholder: "请选择父节点",
                                            "onUpdate:modelValue": (value: string) => {
                                              editForm.parent_id = value;
                                            }
                                          },
                                          () =>
                                            editableParentOptions.value.map((item) =>
                                              h(ElOption, {
                                                key: item.node_id,
                                                label: item.path_text || item.node_name,
                                                value: item.node_id
                                              })
                                            )
                                          )
                                ),
                                h(ElFormItem, { label: "节点编码" }, () =>
                                  h(ElInput, {
                                    modelValue: buildNodeCode(currentNode.value) || "-",
                                    readonly: true,
                                    class: "bindings-page__readonly-input"
                                  })
                                ),
                                h(ElFormItem, { label: `${resolveNodeTypeLabel(currentNode.value.node_type)}名称` }, () =>
                                  h(ElInput, {
                                    modelValue: editForm.node_name,
                                    "onUpdate:modelValue": (value: string) => {
                                      editForm.node_name = value;
                                    }
                                  })
                                ),
                                h(ElFormItem, { label: "节点层级" }, () =>
                                  h(ElInput, {
                                    modelValue: resolveNodeTypeLabel(currentNode.value.node_type),
                                    readonly: true,
                                    class: "bindings-page__readonly-input"
                                  })
                                ),
                                h(ElFormItem, { label: "排序值" }, () =>
                                  h(ElInput, {
                                    modelValue: editForm.sort_order,
                                    placeholder: "默认 0",
                                    "onUpdate:modelValue": (value: string) => {
                                      editForm.sort_order = value;
                                    }
                                  })
                                )
                              ]),
                              !canBindDevice.value && !usesRowTable.value && !usesSlotTable.value
                                ? h("div", { class: "bindings-page__section-title" }, canCreateChild.value ? `新增${resolveNodeTypeLabel(nextNodeType.value)}` : "下级节点")
                                : null,
                              canCreateChild.value && !canBindDevice.value && !usesRowTable.value && !usesSlotTable.value
                                ? h("div", { class: "bindings-page__form-grid" }, [
                                    h(ElFormItem, { label: `${resolveNodeTypeLabel(nextNodeType.value)}名称` }, () =>
                                      h(ElInput, {
                                        modelValue: createForm.node_name,
                                        placeholder: resolveCreatePlaceholder(nextNodeType.value),
                                        "onUpdate:modelValue": (value: string) => {
                                          createForm.node_name = value;
                                        }
                                      })
                                    ),
                                    h(ElFormItem, { label: "排序值" }, () =>
                                      h(ElInput, {
                                        modelValue: createForm.sort_order,
                                        placeholder: "选填，默认按 0 排序",
                                        "onUpdate:modelValue": (value: string) => {
                                          createForm.sort_order = value;
                                        }
                                      })
                                    )
                                  ])
                                : !canBindDevice.value && !usesRowTable.value && !usesSlotTable.value
                                  ? h("div", { class: "bindings-page__hint" }, "当前已是槽位节点，不能再新增下级。")
                                  : null,
                              canBindDevice.value
                                ? h("div", { class: "bindings-page__section-title" }, "槽位绑定")
                                : null,
                              canBindDevice.value
                                ? h("div", { class: "bindings-page__form-grid" }, [
                                    h(ElFormItem, { label: "选择设备", class: "bindings-page__span-2" }, () =>
                                      h(
                                        ElSelect,
                                        {
                                          modelValue: bindForm.device_id,
                                          filterable: true,
                                          placeholder: "请选择设备",
                                          "onUpdate:modelValue": (value: string) => {
                                            bindForm.device_id = value;
                                          }
                                        },
                                        () =>
                                          devices.value.map((item) =>
                                            h(ElOption, {
                                              key: item.device_id,
                                              label: `${item.device_name || item.device_id} (${item.device_id})`,
                                              value: item.device_id
                                            })
                                          )
                                      )
                                    )
                                  ])
                                : null,
                              canBindDevice.value
                                ? h("div", { class: "bindings-page__actions" }, [
                                    h(
                                      ElButton,
                                      {
                                        type: "success",
                                        onClick: () => {
                                          void handleBind();
                                        }
                                      },
                                      () => "绑定设备"
                                    ),
                                    h(
                                      ElButton,
                                      {
                                        type: "danger",
                                        disabled: !currentNode.value?.device_id,
                                        onClick: () => {
                                          void handleUnbind();
                                        }
                                      },
                                      () => "解绑设备"
                                    )
                                  ])
                                : null,
                              usesRowTable.value
                                ? h("div", { class: "bindings-page__child-table" }, [
                                    h("div", { class: "bindings-page__child-table-header" }, [
                                      h("div", { class: "bindings-page__section-title" }, "排号列表"),
                                      h(
                                        ElButton,
                                        {
                                          type: "primary",
                                          onClick: () => {
                                            handleOpenCreateRowDialog();
                                          }
                                        },
                                        () => "新增排号"
                                      )
                                    ]),
                                    directChildren.value.length > 0
                                      ? h(
                                          ElTable,
                                          {
                                            data: directChildren.value,
                                            border: true,
                                            stripe: true,
                                            class: "bindings-page__child-table-inner"
                                          },
                                          {
                                            default: () => [
                                              h(ElTableColumn, { label: "排号名称", minWidth: 180, formatter: (row) => row.node_name || "-" }),
                                              h(ElTableColumn, { label: "节点编码", minWidth: 180, formatter: (row) => buildNodeCode(row) || "-" }),
                                              h(ElTableColumn, { label: "排序值", width: 120, formatter: (row) => String(row.sort_order ?? 0) }),
                                              h(ElTableColumn, { label: "操作", width: 120 }, {
                                                default: ({ row }) =>
                                                  h(
                                                    ElButton,
                                                    {
                                                      link: true,
                                                      type: "primary",
                                                      onClick: () => handleSelectNode(row)
                                                    },
                                                    () => "查看"
                                                  )
                                              })
                                            ]
                                          }
                                        )
                                      : h("div", { class: "bindings-page__hint" }, "当前分区下还没有排号。")
                                  ])
                                : usesSlotTable.value
                                  ? h("div", { class: "bindings-page__child-table" }, [
                                      h("div", { class: "bindings-page__child-table-header" }, [
                                        h("div", { class: "bindings-page__section-title" }, "槽位列表"),
                                        h(
                                          ElButton,
                                          {
                                            type: "primary",
                                            onClick: () => {
                                              handleOpenCreateSlotDialog();
                                            }
                                          },
                                          () => "新增槽位"
                                        )
                                      ]),
                                      directChildren.value.length > 0
                                        ? h(
                                            ElTable,
                                            {
                                              data: directChildren.value,
                                              border: true,
                                              stripe: true,
                                              class: "bindings-page__child-table-inner"
                                            },
                                            {
                                              default: () => [
                                                h(ElTableColumn, { label: "槽位名称", minWidth: 180, formatter: (row) => row.node_name || "-" }),
                                                h(ElTableColumn, { label: "节点编码", minWidth: 180, formatter: (row) => buildNodeCode(row) || "-" }),
                                                h(ElTableColumn, { label: "绑定设备", minWidth: 180, formatter: (row) => row.device_id || "未绑定" }),
                                                h(ElTableColumn, { label: "排序值", width: 120, formatter: (row) => String(row.sort_order ?? 0) }),
                                                h(ElTableColumn, { label: "操作", width: 120 }, {
                                                  default: ({ row }) =>
                                                    h(
                                                      ElButton,
                                                      {
                                                        link: true,
                                                        type: "primary",
                                                        onClick: () => handleSelectNode(row)
                                                      },
                                                      () => "查看"
                                                    )
                                                })
                                              ]
                                            }
                                          )
                                        : h("div", { class: "bindings-page__hint" }, "当前排号下还没有槽位。")
                                    ])
                                : canBindDevice.value
                                  ? null
                                  : directChildren.value.length > 0
                                ? h(
                                    "div",
                                    { class: "bindings-page__children" },
                                    directChildren.value.map((item) =>
                                      h(
                                        "button",
                                        {
                                          type: "button",
                                          class: ["bindings-page__child-item", item.node_id === currentNodeID.value ? "is-active" : ""],
                                          onClick: () => handleSelectNode(item)
                                        },
                                        [
                                          h("span", { class: "bindings-page__child-main" }, [
                                            h("span", { class: "bindings-page__child-name" }, buildNodeTitle(item)),
                                            h("span", { class: "bindings-page__child-meta" }, resolveNodeTypeLabel(item.node_type))
                                          ]),
                                          item.node_type === "slot"
                                            ? h(
                                                ElTag,
                                                {
                                                  size: "small",
                                                  type: item.device_id ? "success" : "info",
                                                  effect: "light"
                                                },
                                                () => (item.device_id ? item.device_id : "未绑定")
                                              )
                                            : null
                                        ]
                                      )
                                    )
                                  )
                                : null
                            ])
                          : h(ElEmpty, { description: "请选择左侧节点查看详情" })
                      ])
              }
            )
            ,
            h(
              ElDialog,
              {
                modelValue: zoneDialogVisible.value,
                "onUpdate:modelValue": (value: boolean) => {
                  zoneDialogVisible.value = value;
                },
                title: "新增分区",
                width: "520px"
              },
              {
                default: () =>
                  h(ElForm, { labelWidth: "100px", class: "dialog-form" }, () => [
                    h(ElFormItem, { label: "分区名称" }, () =>
                      h(ElInput, {
                        modelValue: createForm.node_name,
                        placeholder: "例如：A区",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.node_name = value;
                        }
                      })
                    ),
                    h(ElFormItem, { label: "排序值" }, () =>
                      h(ElInput, {
                        modelValue: createForm.sort_order,
                        placeholder: "选填，默认按 0 排序",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.sort_order = value;
                        }
                      })
                    )
                  ]),
                footer: () =>
                  h("div", { class: "dialog-footer" }, [
                    h(
                      ElButton,
                      {
                        onClick: () => {
                          zoneDialogVisible.value = false;
                        }
                      },
                      () => "取消"
                    ),
                    h(
                      ElButton,
                      {
                        type: "primary",
                        loading: saving.value,
                        onClick: () => {
                          void handleCreateZone();
                        }
                      },
                      () => "创建分区"
                    )
                  ])
              }
            ),
            h(
              ElDialog,
              {
                modelValue: rowDialogVisible.value,
                "onUpdate:modelValue": (value: boolean) => {
                  rowDialogVisible.value = value;
                },
                title: "新增排号",
                width: "520px"
              },
              {
                default: () =>
                  h(ElForm, { labelWidth: "100px", class: "dialog-form" }, () => [
                    h(ElFormItem, { label: "排号名称" }, () =>
                      h(ElInput, {
                        modelValue: createForm.node_name,
                        placeholder: "例如：第1排",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.node_name = value;
                        }
                      })
                    ),
                    h(ElFormItem, { label: "排序值" }, () =>
                      h(ElInput, {
                        modelValue: createForm.sort_order,
                        placeholder: "选填，默认按 0 排序",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.sort_order = value;
                        }
                      })
                    )
                  ]),
                footer: () =>
                  h("div", { class: "dialog-footer" }, [
                    h(
                      ElButton,
                      {
                        onClick: () => {
                          rowDialogVisible.value = false;
                        }
                      },
                      () => "取消"
                    ),
                    h(
                      ElButton,
                      {
                        type: "primary",
                        loading: saving.value,
                        onClick: () => {
                          void handleCreateRow();
                        }
                      },
                      () => "创建排号"
                    )
                  ])
              }
            ),
            h(
              ElDialog,
              {
                modelValue: slotDialogVisible.value,
                "onUpdate:modelValue": (value: boolean) => {
                  slotDialogVisible.value = value;
                },
                title: "新增槽位",
                width: "520px"
              },
              {
                default: () =>
                  h(ElForm, { labelWidth: "100px", class: "dialog-form" }, () => [
                    h(ElFormItem, { label: "槽位名称" }, () =>
                      h(ElInput, {
                        modelValue: createForm.node_name,
                        placeholder: "例如：001",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.node_name = value;
                        }
                      })
                    ),
                    h(ElFormItem, { label: "排序值" }, () =>
                      h(ElInput, {
                        modelValue: createForm.sort_order,
                        placeholder: "选填，默认按 0 排序",
                        "onUpdate:modelValue": (value: string) => {
                          createForm.sort_order = value;
                        }
                      })
                    )
                  ]),
                footer: () =>
                  h("div", { class: "dialog-footer" }, [
                    h(
                      ElButton,
                      {
                        onClick: () => {
                          slotDialogVisible.value = false;
                        }
                      },
                      () => "取消"
                    ),
                    h(
                      ElButton,
                      {
                        type: "primary",
                        loading: saving.value,
                        onClick: () => {
                          void handleCreateSlot();
                        }
                      },
                      () => "创建槽位"
                    )
                  ])
              }
            )
          ])
        ])
      ]);
  }
});

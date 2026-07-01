import {
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
  ElScrollbar,
  ElTable,
  ElTableColumn
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { useNoticesStore } from "../../stores/notices";
import { useScriptsStore } from "../../stores/scripts";
import type { ScriptVersionRecord, WorkflowReferenceRecord } from "../../types/script";
import { formatDateTime } from "../../utils/device";

const SCRIPT_NAME_PATTERN = /^[A-Za-z0-9_]+$/;

function formatWorkflowReferences(references: WorkflowReferenceRecord[]): string {
  if (!references || references.length === 0) {
    return "";
  }
  return references.map((item) => `${item.workflow_name || item.workflow_def_id} / ${item.node_name || item.node_id}`).join("；");
}

export const ScriptsPage = defineComponent({
  name: "ScriptsPage",
  setup() {
    const scriptsStore = useScriptsStore();
    const noticesStore = useNoticesStore();
    const { scriptNames, selectedManifest, uploading, errorMessage } = storeToRefs(scriptsStore);

    const createDialogVisible = ref(false);
    const uploadDialogVisible = ref(false);
    const detailDialogVisible = ref(false);
    const referencesDialogVisible = ref(false);
    const uploadFile = ref<File | null>(null);
    const uploadInputKey = ref(0);
    const uploadInputRef = ref<HTMLInputElement | null>(null);
    const selectedScriptName = ref("");
    const searchKeyword = ref("");
    const selectedReferenceVersion = ref<{
      scriptName: string;
      scriptVersion: string;
      references: WorkflowReferenceRecord[];
    } | null>(null);

    const createForm = reactive({
      script_name: ""
    });

    const uploadForm = reactive({
      script_name: "",
      script_version: "",
      force: false
    });

    const filteredScriptNames = computed(() => {
      const keyword = searchKeyword.value.trim().toLowerCase();
      if (keyword === "") {
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

    async function loadPageData() {
      await Promise.all([scriptsStore.loadScriptNames(), scriptsStore.loadScripts()]);
      if (!selectedScriptName.value && scriptNames.value.length > 0) {
        selectedScriptName.value = scriptNames.value[0].script_name;
      }
      if (selectedScriptName.value && !scriptNames.value.some((item) => item.script_name === selectedScriptName.value)) {
        selectedScriptName.value = scriptNames.value[0]?.script_name || "";
      }
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(
      errorMessage,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.error(value, 5000);
        }
      }
    );

    async function handleCreateScriptName() {
      const scriptName = createForm.script_name.trim();
      if (scriptName === "") {
        ElMessage.warning("请先填写脚本名称");
        return;
      }
      if (!SCRIPT_NAME_PATTERN.test(scriptName)) {
        ElMessage.warning("脚本名称只能包含英文、数字和下划线");
        return;
      }
      if (scriptNames.value.some((item) => item.script_name === scriptName)) {
        ElMessage.warning("脚本名称已存在");
        return;
      }

      try {
        await scriptsStore.submitScriptName(scriptName);
        createDialogVisible.value = false;
        createForm.script_name = "";
        await loadPageData();
        selectedScriptName.value = scriptName;
        ElMessage.success("脚本名称创建成功");
      } catch {
        ElMessage.error("创建脚本名称失败");
      }
    }

    function openUploadDialog(scriptName = selectedScriptName.value) {
      uploadForm.script_name = scriptName;
      uploadForm.script_version = "";
      uploadForm.force = false;
      uploadFile.value = null;
      uploadInputKey.value += 1;
      uploadDialogVisible.value = true;
    }

    function openUploadFilePicker() {
      uploadInputRef.value?.click();
    }

    async function handleUploadConfirm() {
      const scriptName = uploadForm.script_name.trim();
      const scriptVersion = uploadForm.script_version.trim();

      if (scriptName === "") {
        ElMessage.warning("请先选择脚本名称");
        return;
      }
      if (!SCRIPT_NAME_PATTERN.test(scriptName)) {
        ElMessage.warning("脚本名称只能包含英文、数字和下划线");
        return;
      }
      if (scriptVersion === "") {
        ElMessage.warning("请先填写脚本版本");
        return;
      }
      if (!uploadFile.value) {
        ElMessage.warning("请先选择 zip 脚本包");
        return;
      }

      try {
        await scriptsStore.submitScriptUpload({
          script_name: scriptName,
          script_version: scriptVersion,
          source_type: "zip",
          force: uploadForm.force,
          file: uploadFile.value
        });
        uploadDialogVisible.value = false;
        await loadPageData();
        selectedScriptName.value = scriptName;
        ElMessage.success("脚本版本上传成功");
      } catch {
        ElMessage.error("上传脚本失败，请检查 zip 解压后的根目录下是否存在 index.js");
      }
    }

    async function handleDeleteVersion(scriptName: string, scriptVersion: string) {
      try {
        await ElMessageBox.confirm(`确认删除脚本版本 ${scriptName}@${scriptVersion} 吗？`, "删除脚本版本", {
          type: "warning",
          confirmButtonText: "确认删除",
          cancelButtonText: "取消"
        });
        await scriptsStore.removeScriptVersion(scriptName, scriptVersion);
        await loadPageData();
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
        await loadPageData();
        selectedScriptName.value = scriptNames.value[0]?.script_name || "";
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

    function openCreateDialog() {
      createForm.script_name = "";
      createDialogVisible.value = true;
    }

    return () =>
      h("section", { class: "scripts-page" }, [
        h("div", { class: "scripts-page__layout" }, [
          h(
            ElCard,
            { class: "page-card page-fill-card scripts-page__sidebar", shadow: "never" },
            {
              default: () => [
                h("div", { class: "scripts-page__search" }, [
                  h(
                    ElButton,
                    {
                      type: "primary",
                      onClick: openCreateDialog
                    },
                    () => "添加"
                  ),
                  h(ElInput, {
                    modelValue: searchKeyword.value,
                    "onUpdate:modelValue": (value: string) => {
                      searchKeyword.value = value;
                    },
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
                              class: [
                                "scripts-page__name-item",
                                selectedScriptName.value === item.script_name ? "scripts-page__name-item--active" : ""
                              ],
                              type: "button",
                              onClick: () => {
                                selectedScriptName.value = item.script_name;
                              }
                            },
                            [
                              h("span", { class: "scripts-page__name-title" }, item.script_name),
                              h(
                                ElButton,
                                {
                                  link: true,
                                  type: "danger",
                                  onClick: (event: MouseEvent) => {
                                    event.stopPropagation();
                                    void handleDeleteScript(item.script_name);
                                  }
                                },
                                () => "删除"
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
            { class: "page-card page-fill-card scripts-page__content", shadow: "never" },
            {
              default: () => [
                h("div", { class: "scripts-page__panel-header scripts-page__panel-header--row" }, [
                  h("div", null, [
                    h("div", { class: "card-header__title" }, "版本管理"),
                    h(
                      "div",
                      { class: "card-header__subtitle" },
                      selectedScript.value ? `${selectedScript.value.script_name} 的全部版本` : "请选择左侧脚本名称"
                    )
                  ]),
                  h(
                    ElButton,
                    {
                      type: "primary",
                      disabled: !selectedScript.value,
                      onClick: () => openUploadDialog(selectedScript.value?.script_name || "")
                    },
                    () => "添加版本"
                  )
                ]),
                !selectedScript.value
                  ? h(ElEmpty, { description: "请先在左侧选择一个脚本名称" })
                  : h("div", { class: "scripts-page__content-body" }, [
                      h(ElDescriptions, { border: true, column: 1 }, () => [
                        h(ElDescriptionsItem, { label: "脚本名称" }, () => selectedScript.value?.script_name || "暂无")
                      ]),
                      h("div", { class: "table-scroll-region" }, [
                        h(
                          ElTable,
                          {
                            data: selectedVersions.value,
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
                                  label: "版本号",
                                  minWidth: 140
                                },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) => row.script_version || "暂无"
                                }
                              ),
                              h(
                                ElTableColumn,
                                {
                                  label: "创建时间",
                                  minWidth: 180
                                },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) => formatDateTime(row.created_at)
                                }
                              ),
                              h(
                                ElTableColumn,
                                {
                                  label: "入口文件",
                                  minWidth: 160
                                },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) => row.entry_file || "暂无"
                                }
                              ),
                              h(
                                ElTableColumn,
                                {
                                  label: "状态",
                                  width: 100
                                },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) => row.status || "暂无"
                                }
                              ),
                              h(
                                ElTableColumn,
                                { label: "操作", minWidth: 220, fixed: "right" },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) =>
                                    h("div", { class: "table-actions table-actions--nowrap" }, [
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          type: "primary",
                                          onClick: async () => {
                                            await scriptsStore.loadScriptVersion(row.script_name, row.script_version);
                                            detailDialogVisible.value = true;
                                          }
                                        },
                                        () => "查看"
                                      ),
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          disabled: !row.workflow_references || row.workflow_references.length === 0,
                                          onClick: () =>
                                            openReferencesDialog(row.script_name, row.script_version, row.workflow_references || [])
                                        },
                                        () => "引用"
                                      ),
                                      h(
                                        ElButton,
                                        {
                                          link: true,
                                          type: "danger",
                                          onClick: () => {
                                            void handleDeleteVersion(row.script_name, row.script_version);
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
                    ])
              ]
            }
          )
        ]),
        h(
          ElDialog,
          {
            modelValue: createDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => {
              createDialogVisible.value = value;
            },
            title: "添加脚本名称",
            width: "520px"
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "脚本名称" }, () =>
                  h(ElInput, {
                    modelValue: createForm.script_name,
                    "onUpdate:modelValue": (value: string) => {
                      createForm.script_name = value;
                    },
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
            "onUpdate:modelValue": (value: boolean) => {
              uploadDialogVisible.value = value;
            },
            title: "添加脚本版本",
            width: "560px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "脚本名称" }, () =>
                  h(ElInput, {
                    modelValue: uploadForm.script_name,
                    readonly: true
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
                    h(ElButton, { onClick: openUploadFilePicker }, () => "选择 zip 文件"),
                    h("div", { class: "upload-field__filename" }, uploadFile.value ? uploadFile.value.name : "尚未选择文件"),
                    h(
                      ElCheckbox,
                      {
                        modelValue: uploadForm.force,
                        "onUpdate:modelValue": (value: string | number | boolean) => {
                          uploadForm.force = Boolean(value);
                        }
                      },
                      () => "强制覆盖同名同版本脚本"
                    )
                  ])
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (uploadDialogVisible.value = false) }, () => "取消"),
                h(
                  ElButton,
                  {
                    type: "primary",
                    loading: uploading.value,
                    onClick: () => void handleUploadConfirm()
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
            width: "820px"
          },
          {
            default: () =>
              selectedManifest.value
                ? h(ElDescriptions, { border: true, column: 2 }, () => [
                    h(ElDescriptionsItem, { label: "脚本名称" }, () => selectedManifest.value?.script_name || "暂无"),
                    h(ElDescriptionsItem, { label: "脚本版本" }, () => selectedManifest.value?.script_version || "暂无"),
                    h(ElDescriptionsItem, { label: "入口文件" }, () => selectedManifest.value?.entry_file || "暂无"),
                    h(ElDescriptionsItem, { label: "下载地址" }, () => selectedManifest.value?.download_url || "暂无")
                  ])
                : h(ElEmpty, { description: "暂无可展示的脚本详情" }),
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (detailDialogVisible.value = false) }, () => "关闭")])
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
            width: "760px"
          },
          {
            default: () =>
              selectedReferenceVersion.value && selectedReferenceVersion.value.references.length > 0
                ? h(
                    "div",
                    { class: "script-references-dialog" },
                    selectedReferenceVersion.value.references.map((item) =>
                      h("div", { key: `${item.workflow_def_id}-${item.node_id}`, class: "script-references-dialog__item" }, [
                        h("div", { class: "script-references-dialog__title" }, item.workflow_name || item.workflow_def_id),
                        h("div", { class: "script-references-dialog__meta" }, `节点：${item.node_name || item.node_id}`)
                      ])
                    )
                  )
                : h(ElEmpty, { description: "当前没有工作流引用该脚本版本" }),
            footer: () => h("div", { class: "dialog-footer" }, [h(ElButton, { onClick: () => (referencesDialogVisible.value = false) }, () => "关闭")])
          }
        )
      ]);
  }
});

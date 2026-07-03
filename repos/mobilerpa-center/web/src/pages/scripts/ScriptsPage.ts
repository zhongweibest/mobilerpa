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
  ElPagination,
  ElScrollbar,
  ElSelect,
  ElTable,
  ElTableColumn,
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { useNoticesStore } from "../../stores/notices";
import { useScriptsStore } from "../../stores/scripts";
import type { ScriptVersionRecord } from "../../types/script";
import { formatDateTime } from "../../utils/device";

const SCRIPT_NAME_PATTERN = /^[A-Za-z0-9_]+$/;
const PAGE_SIZES = [10, 20, 30, 50, 100];

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
    const uploadFile = ref<File | null>(null);
    const uploadInputKey = ref(0);
    const uploadInputRef = ref<HTMLInputElement | null>(null);

    const createForm = reactive({ script_name: "" });
    const uploadForm = reactive({
      script_name: "",
      script_version: "",
      force: false
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

    function openUploadFilePicker() {
      uploadInputRef.value?.click();
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
                                { label: "操作", width: 160, fixed: "right" },
                                {
                                  default: ({ row }: { row: ScriptVersionRecord }) =>
                                    h("div", { class: "table-actions table-actions--nowrap" }, [
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
        )
      ]);
  }
});

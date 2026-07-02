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
  ElPagination,
  ElTable,
  ElTableColumn
} from "element-plus";
import { storeToRefs } from "pinia";
import { defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { buildSoftwareDownloadUrl } from "../../api/software";
import { useNoticesStore } from "../../stores/notices";
import { useSoftwareStore } from "../../stores/software";
import type { SoftwarePackageRecord } from "../../types/software";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

export const SoftwarePage = defineComponent({
  name: "SoftwarePage",
  setup() {
    const softwareStore = useSoftwareStore();
    const noticesStore = useNoticesStore();
    const { items, total, page, pageSize, loading, submitting, errorMessage } = storeToRefs(softwareStore);

    const dialogVisible = ref(false);
    const editingSoftwareID = ref("");
    const uploadFile = ref<File | null>(null);
    const uploadInputKey = ref(0);
    const uploadInputRef = ref<HTMLInputElement | null>(null);
    const form = reactive({
      software_name: "",
      description: ""
    });

    async function loadPageData() {
      await softwareStore.loadSoftware();
    }

    onMounted(() => {
      void loadPageData();
    });

    watch(errorMessage, (value, previousValue) => {
      if (value && value !== previousValue) {
        noticesStore.error(value, 5000);
      }
    });

    function resetForm() {
      editingSoftwareID.value = "";
      form.software_name = "";
      form.description = "";
      uploadFile.value = null;
      uploadInputKey.value += 1;
    }

    function openCreateDialog() {
      resetForm();
      dialogVisible.value = true;
    }

    function openEditDialog(item: SoftwarePackageRecord) {
      editingSoftwareID.value = item.software_id;
      form.software_name = item.software_name;
      form.description = item.description || "";
      uploadFile.value = null;
      uploadInputKey.value += 1;
      dialogVisible.value = true;
    }

    function openFilePicker() {
      uploadInputRef.value?.click();
    }

    async function handleSubmit() {
      const softwareName = form.software_name.trim();
      if (!softwareName) {
        noticesStore.warning("请先填写软件名称", 5000);
        return;
      }
      if (!editingSoftwareID.value && !uploadFile.value) {
        noticesStore.warning("请先上传软件包", 5000);
        return;
      }

      if (editingSoftwareID.value) {
        await softwareStore.saveSoftware({
          software_id: editingSoftwareID.value,
          software_name: softwareName,
          description: form.description.trim(),
          file: uploadFile.value
        });
        noticesStore.success("软件信息已更新", 3000);
      } else {
        await softwareStore.submitSoftware({
          software_name: softwareName,
          description: form.description.trim(),
          file: uploadFile.value as File
        });
        noticesStore.success("软件已添加", 3000);
      }
      dialogVisible.value = false;
      resetForm();
    }

    async function handleDelete(item: SoftwarePackageRecord) {
      try {
        await ElMessageBox.confirm(`确认删除软件 ${item.software_name} 吗？`, "删除软件", {
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

      await softwareStore.removeSoftware(item.software_id);
      noticesStore.success("软件已删除", 3000);
    }

    function handleDownload(item: SoftwarePackageRecord) {
      window.open(buildSoftwareDownloadUrl(item.software_id), "_blank");
    }

    return () =>
      h("section", { class: "app-page software-page" }, [
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            default: () =>
              h("div", { class: "page-scroll-body" }, [
                h("div", { class: "page-toolbar page-toolbar--table" }, [
                  h("div", { class: "table-actions" }, [
                    h(ElButton, { loading: loading.value, onClick: () => void loadPageData() }, () => "刷新"),
                    h(ElButton, { type: "primary", onClick: openCreateDialog }, () => "添加软件")
                  ])
                ]),
                items.value.length === 0
                  ? h(ElEmpty, { description: "当前还没有软件包" })
                  : h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: items.value, stripe: true, class: "app-table", height: "100%" },
                        {
                          default: () => [
                            h(ElTableColumn, { prop: "software_name", label: "软件名称", minWidth: 180 }),
                            h(ElTableColumn, { prop: "description", label: "软件描述", minWidth: 260, formatter: (_row, _col, value) => value || "暂无描述" }),
                            h(ElTableColumn, { prop: "package_file_name", label: "软件包", minWidth: 220 }),
                            h(ElTableColumn, {
                              prop: "package_size",
                              label: "大小",
                              width: 120,
                              formatter: (_row, _col, value) => `${Number(value || 0)} B`
                            }),
                            h(ElTableColumn, {
                              prop: "updated_at",
                              label: "更新时间",
                              minWidth: 180,
                              formatter: (_row, _col, value) => formatDateTime(value)
                            }),
                            h(
                              ElTableColumn,
                              { label: "操作", width: 190, fixed: "right" },
                              {
                                default: ({ row }: { row: SoftwarePackageRecord }) =>
                                  h("div", { class: "table-actions table-actions--nowrap" }, [
                                    h(ElButton, { link: true, onClick: () => handleDownload(row) }, () => "下载"),
                                    h(ElButton, { link: true, onClick: () => openEditDialog(row) }, () => "编辑"),
                                    h(ElButton, { link: true, type: "danger", onClick: () => void handleDelete(row) }, () => "删除")
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
                      void softwareStore.changePage(value);
                    },
                    "onUpdate:pageSize": (value: number) => {
                      void softwareStore.changePageSize(value);
                    }
                  })
                )
              ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: dialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (dialogVisible.value = value),
            title: editingSoftwareID.value ? "编辑软件" : "添加软件",
            width: "560px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "软件名称" }, () =>
                  h(ElInput, {
                    modelValue: form.software_name,
                    "onUpdate:modelValue": (value: string) => (form.software_name = value),
                    placeholder: "请输入软件名称"
                  })
                ),
                h(ElFormItem, { label: "软件描述" }, () =>
                  h(ElInput, {
                    modelValue: form.description,
                    "onUpdate:modelValue": (value: string) => (form.description = value),
                    type: "textarea",
                    rows: 4,
                    placeholder: "请输入软件描述"
                  })
                ),
                h(ElFormItem, { label: "软件包上传" }, () =>
                  h("div", { class: "upload-field" }, [
                    h("input", {
                      key: `software-upload-${uploadInputKey.value}`,
                      ref: uploadInputRef,
                      class: "upload-field__input",
                      type: "file",
                      onChange: (event: Event) => {
                        const target = event.target as HTMLInputElement;
                        uploadFile.value = target.files?.[0] || null;
                      }
                    }),
                    h(ElButton, { onClick: openFilePicker }, () => "选择软件包"),
                    h("div", { class: "upload-field__filename" }, uploadFile.value ? uploadFile.value.name : editingSoftwareID.value ? "不更新软件包则保留原文件" : "尚未选择文件")
                  ])
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (dialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: submitting.value, onClick: () => void handleSubmit() }, () => "确认")
              ])
          }
        )
      ]);
  }
});

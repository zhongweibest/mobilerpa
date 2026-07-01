import { ElButton, ElCard, ElForm, ElFormItem, ElInput, ElMessage, ElTag } from "element-plus";
import { defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { fetchDiscoverySettings, saveDiscoverySettings } from "../../api/settings";
import { useNoticesStore } from "../../stores/notices";

export const SettingsPage = defineComponent({
  name: "SettingsPage",
  setup() {
    const noticesStore = useNoticesStore();
    const loading = ref(false);
    const saving = ref(false);
    const errorMessage = ref("");
    const savedMessage = ref("");
    const form = reactive({
      center_base_url: ""
    });

    async function loadSettings() {
      loading.value = true;
      errorMessage.value = "";
      try {
        const settings = await fetchDiscoverySettings();
        form.center_base_url = settings.center_base_url || "";
      } catch (error) {
        errorMessage.value = error instanceof Error ? error.message : "load_settings_failed";
      } finally {
        loading.value = false;
      }
    }

    async function handleSave() {
      saving.value = true;
      errorMessage.value = "";
      savedMessage.value = "";
      try {
        const result = await saveDiscoverySettings({
          center_base_url: form.center_base_url.trim()
        });
        form.center_base_url = result.center_base_url || "";
        savedMessage.value = "系统配置已保存";
        ElMessage.success("系统配置已保存");
      } catch (error) {
        errorMessage.value = error instanceof Error ? error.message : "save_settings_failed";
        ElMessage.error("系统配置保存失败，请稍后重试");
      } finally {
        saving.value = false;
      }
    }

    onMounted(() => {
      void loadSettings();
    });

    watch(
      errorMessage,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.error(`系统配置加载或保存失败：${value}`, 5000);
        }
      }
    );

    watch(
      savedMessage,
      (value, previousValue) => {
        if (value && value !== previousValue) {
          noticesStore.success(value, 3000);
        }
      }
    );

    return () =>
      h("section", { class: "settings-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(
            ElButton,
            {
              loading: loading.value,
              onClick: () => {
                void loadSettings();
              }
            },
            () => "重新加载"
          ),
          h(
            ElButton,
            {
              type: "primary",
              loading: saving.value,
              onClick: () => {
                void handleSave();
              }
            },
            () => "保存配置"
          )
        ]),
        h("div", { class: "settings-grid" }, [
          h(
            ElCard,
            {
              class: "page-card",
              shadow: "never"
            },
            {
              header: () =>
                h("div", { class: "card-header" }, [
                  h("div", null, [
                    h("div", { class: "card-header__title" }, "中心连接配置"),
                    h("div", { class: "card-header__subtitle" }, "设备发现、Agent 下发与后续默认行为统一从这里读取中心服务地址。")
                  ]),
                  h(
                    ElTag,
                    {
                      type: "success",
                      effect: "light"
                    },
                    () => "已接入"
                  )
                ]),
              default: () =>
                h(
                  ElForm,
                  {
                    labelPosition: "top",
                    class: "dialog-form"
                  },
                  () => [
                    h(
                      ElFormItem,
                      {
                        label: "默认中心地址"
                      },
                      () =>
                        h(ElInput, {
                          modelValue: form.center_base_url,
                          "onUpdate:modelValue": (value: string) => {
                            form.center_base_url = value;
                          },
                          placeholder: "例如 http://192.168.0.155:28080"
                        })
                    ),
                    h("div", { class: "settings-help-text" }, "建议填写真机可访问的完整 HTTP 地址。设备发现页和后续 Agent 默认行为都会复用这个配置。")
                  ]
                )
            }
          ),
          h(
            ElCard,
            {
              class: "page-card",
              shadow: "never"
            },
            {
              header: () =>
                h("div", { class: "card-header" }, [
                  h("div", null, [
                    h("div", { class: "card-header__title" }, "默认心跳参数"),
                    h("div", { class: "card-header__subtitle" }, "后续会把心跳间隔、离线超时、扫描周期统一归口到这个页面维护。")
                  ]),
                  h(
                    ElTag,
                    {
                      type: "info",
                      effect: "light"
                    },
                    () => "待接入"
                  )
                ]),
              default: () =>
                h("div", { class: "settings-placeholder" }, [
                  h("div", { class: "settings-placeholder__title" }, "预留配置项"),
                  h("div", { class: "settings-placeholder__text" }, "heartbeat_interval、offline_timeout、scan_interval 会在后端参数化能力完善后统一接入。")
                ])
            }
          ),
          h(
            ElCard,
            {
              class: "page-card",
              shadow: "never"
            },
            {
              header: () =>
                h("div", { class: "card-header" }, [
                  h("div", null, [
                    h("div", { class: "card-header__title" }, "默认 Agent 行为"),
                    h("div", { class: "card-header__subtitle" }, "后续会统一维护启动策略、自动重连、脚本同步和默认调试行为。")
                  ]),
                  h(
                    ElTag,
                    {
                      type: "info",
                      effect: "light"
                    },
                    () => "待接入"
                  )
                ]),
              default: () =>
                h("div", { class: "settings-placeholder" }, [
                  h("div", { class: "settings-placeholder__title" }, "预留配置项"),
                  h("div", { class: "settings-placeholder__text" }, "这里作为系统级配置统一入口，避免后续配置分散到设备、脚本、任务和工作流页面。")
                ])
            }
          )
        ])
      ]);
  }
});

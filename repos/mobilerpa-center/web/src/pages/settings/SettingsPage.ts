import { ElButton, ElCard, ElForm, ElFormItem, ElInput, ElInputNumber, ElSelect, ElOption, ElTag } from "element-plus";
import { defineComponent, h, onMounted, reactive, ref, watch } from "vue";

import { fetchDiscoverySettings, fetchPlanDailyRetrySettings, saveDiscoverySettings, savePlanDailyRetrySettings } from "../../api/settings";
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
      center_base_url: "",
      plan_daily_retry_enabled: true,
      plan_daily_retry_interval_seconds: 60,
      plan_daily_retry_stop_before_deadline_minutes: 30
    });

    async function loadSettings() {
      loading.value = true;
      errorMessage.value = "";
      try {
        const settings = await fetchDiscoverySettings();
        form.center_base_url = settings.center_base_url || "";
        const retrySettings = await fetchPlanDailyRetrySettings();
        form.plan_daily_retry_enabled = retrySettings.plan_daily_retry_enabled;
        form.plan_daily_retry_interval_seconds = retrySettings.plan_daily_retry_interval_seconds || 60;
        form.plan_daily_retry_stop_before_deadline_minutes = retrySettings.plan_daily_retry_stop_before_deadline_minutes || 30;
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
        const retrySettings = await savePlanDailyRetrySettings({
          plan_daily_retry_enabled: form.plan_daily_retry_enabled,
          plan_daily_retry_interval_seconds: Number(form.plan_daily_retry_interval_seconds || 60),
          plan_daily_retry_stop_before_deadline_minutes: Number(form.plan_daily_retry_stop_before_deadline_minutes || 30)
        });
        form.plan_daily_retry_enabled = retrySettings.plan_daily_retry_enabled;
        form.plan_daily_retry_interval_seconds = retrySettings.plan_daily_retry_interval_seconds || 60;
        form.plan_daily_retry_stop_before_deadline_minutes = retrySettings.plan_daily_retry_stop_before_deadline_minutes || 30;
        savedMessage.value = "系统配置已保存";
      } catch (error) {
        errorMessage.value = error instanceof Error ? error.message : "save_settings_failed";
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
                    h("div", { class: "card-header__title" }, "计划任务默认重试"),
                    h("div", { class: "card-header__subtitle" }, "按天循环任务中，离线设备默认多久重试一次，以及截止前多久停止重试。")
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
                    h(ElFormItem, { label: "默认离线重试" }, () =>
                      h(
                        ElSelect,
                        {
                          modelValue: form.plan_daily_retry_enabled ? "enabled" : "disabled",
                          "onUpdate:modelValue": (value: string) => {
                            form.plan_daily_retry_enabled = value === "enabled";
                          }
                        },
                        () => [h(ElOption, { label: "启用", value: "enabled" }), h(ElOption, { label: "停用", value: "disabled" })]
                      )
                    ),
                    h(ElFormItem, { label: "默认重试间隔（秒）" }, () =>
                      h(ElInputNumber, {
                        modelValue: form.plan_daily_retry_interval_seconds,
                        "onUpdate:modelValue": (value?: number) => {
                          form.plan_daily_retry_interval_seconds = Number(value || 60);
                        },
                        min: 60,
                        max: 1800,
                        step: 60
                      })
                    ),
                    h(ElFormItem, { label: "截止前停止重试（分钟）" }, () =>
                      h(ElInputNumber, {
                        modelValue: form.plan_daily_retry_stop_before_deadline_minutes,
                        "onUpdate:modelValue": (value?: number) => {
                          form.plan_daily_retry_stop_before_deadline_minutes = Number(value || 30);
                        },
                        min: 0,
                        max: 180,
                        step: 5
                      })
                    )
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

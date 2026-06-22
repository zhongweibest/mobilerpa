import { defineComponent, h } from "vue";

export const HomePage = defineComponent({
  name: "HomePage",
  setup() {
    return () =>
      h("section", { class: "app-page app-page--dashboard" }, [
        h("section", { class: "app-page__panel" }, [
          h("div", { class: "dashboard-empty" }, [
            h("h2", { class: "dashboard-empty__title" }, "首页看板待补充"),
            h(
              "p",
              { class: "dashboard-empty__summary" },
              "后续会把设备概览、任务趋势、工作流运行态、异常摘要与消息流统一收口到这里，业务页面继续保持列表优先。"
            )
          ])
        ])
      ]);
  }
});

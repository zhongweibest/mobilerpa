import { defineComponent, h } from "vue";
import { useRoute } from "vue-router";

type PlaceholderMeta = {
  title?: string;
  summary?: string;
};

export const PlaceholderPage = defineComponent({
  name: "PlaceholderPage",
  setup() {
    const route = useRoute();

    return () => {
      const meta = (route.meta || {}) as PlaceholderMeta;
      const title = meta.title || "功能页面";
      const summary = meta.summary || "当前页面入口已预留，具体能力会在后续阶段逐步补齐。";

      return h("section", { class: "app-page" }, [
        h("section", { class: "app-page__panel" }, [
          h("div", { class: "placeholder-empty" }, [
            h("div", { class: "placeholder-empty__badge" }, "规划能力"),
            h("h2", { class: "placeholder-empty__title" }, title),
            h("p", { class: "placeholder-empty__summary" }, summary)
          ])
        ])
      ]);
    };
  }
});

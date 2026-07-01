import { ElAlert } from "element-plus";
import { storeToRefs } from "pinia";
import { defineComponent, h } from "vue";

import { useNoticesStore } from "../stores/notices";

export const TopNoticeBar = defineComponent({
  name: "TopNoticeBar",
  setup() {
    const noticesStore = useNoticesStore();
    const { notices } = storeToRefs(noticesStore);

    return () =>
      notices.value.length === 0
        ? null
        : h(
            "div",
            { class: "top-notice-bar" },
            notices.value.map((item) =>
              h(ElAlert, {
                key: item.id,
                class: "top-notice-bar__item",
                type: item.type,
                title: item.message,
                showIcon: true,
                closable: true,
                onClose: () => noticesStore.removeNotice(item.id)
              })
            )
          );
  }
});

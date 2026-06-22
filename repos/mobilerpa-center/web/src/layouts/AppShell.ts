import { ElScrollbar, ElTag } from "element-plus";
import { computed, defineComponent, h } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";

type ShellNavItem = {
  to: string;
  label: string;
  badge?: string;
};

type ShellNavGroup = {
  key: string;
  label: string;
  items: ShellNavItem[];
};

function normalizeBadge(input: unknown): string {
  return typeof input === "string" ? input : "";
}

function normalizeText(input: unknown, fallback = ""): string {
  return typeof input === "string" && input.trim() !== "" ? input : fallback;
}

export const AppShell = defineComponent({
  name: "AppShell",
  setup() {
    const route = useRoute();
    const router = useRouter();

    const navGroups = computed<ShellNavGroup[]>(() => {
      const children = (router.options.routes[0]?.children || []).filter((item) => item.meta?.navVisible !== false);
      const mapped = children.map((item) => {
        const itemMeta = (item.meta || {}) as Record<string, unknown>;
        return {
          to: item.path === "" ? "/" : `/${item.path}`,
          label: normalizeText(itemMeta["title"], String(item.name || item.path || "")),
          badge: normalizeBadge(itemMeta["navBadge"]),
          group: normalizeText(itemMeta["navGroup"], "main"),
          order: Number(itemMeta["navOrder"] || 999)
        };
      });

      const groupDefs = [
        { key: "main", label: "主功能" },
        { key: "workflow", label: "计划与工作流" },
        { key: "ops", label: "运维与配置" }
      ];

      return groupDefs
        .map((group) => ({
          key: group.key,
          label: group.label,
          items: mapped
            .filter((item) => item.group === group.key)
            .sort((left, right) => left.order - right.order)
            .map((item) => ({
              to: item.to,
              label: item.label,
              badge: item.badge
            }))
        }))
        .filter((group) => group.items.length > 0);
    });

    const currentTitle = computed(() => normalizeText(route.meta?.title, "首页"));

    function isActive(target: string) {
      if (target === "/") {
        return route.path === "/";
      }
      return route.path === target || route.path.startsWith(`${target}/`);
    }

    function renderNavGroup(group: ShellNavGroup) {
      return h("section", { class: "shell__nav-group", key: group.key }, [
        h(
          "nav",
          { class: "shell__nav-list" },
          group.items.map((item) =>
            h(
              RouterLink,
              {
                key: item.to,
                to: item.to,
                class: ["shell__nav-item", isActive(item.to) ? "shell__nav-item--active" : ""]
              },
              {
                default: () => [
                  h("div", { class: "shell__nav-item-main" }, [h("div", { class: "shell__nav-label" }, item.label)]),
                  item.badge
                    ? h(
                        ElTag,
                        {
                          size: "small",
                          type: "info",
                          effect: "dark",
                          class: "shell__nav-badge"
                        },
                        () => item.badge
                      )
                    : null
                ]
              }
            )
          )
        )
      ]);
    }

    return () =>
      h("div", { class: "shell" }, [
        h("aside", { class: "shell__sidebar" }, [
          h("div", { class: "shell__brand" }, [
            h("div", { class: "shell__brand-mark" }, "M"),
            h("div", { class: "shell__brand-text" }, [
              h("div", { class: "shell__brand-title" }, "MobileRPA"),
              h("div", { class: "shell__brand-subtitle" }, "中心控制台")
            ])
          ]),
          h(
            ElScrollbar,
            { class: "shell__sidebar-scroll" },
            {
              default: () => navGroups.value.map((group) => renderNavGroup(group))
            }
          )
        ]),
        h("section", { class: "shell__workspace" }, [
          h("header", { class: "shell__header" }, [
            h("div", { class: "shell__header-left" }, [h("div", { class: "shell__breadcrumb" }, [h("span", { class: "shell__breadcrumb-item" }, currentTitle.value)])])
          ]),
          h("main", { class: "shell__content" }, [h(RouterView)])
        ])
      ]);
  }
});

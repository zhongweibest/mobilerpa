// @ts-nocheck
import {
  ElAlert,
  ElButton,
  ElCard,
  ElDialog,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElMessage,
  ElMessageBox,
  ElOption,
  ElPagination,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag
} from "element-plus";
import { storeToRefs } from "pinia";
import { computed, defineComponent, h, onMounted, reactive, ref } from "vue";

import { useScriptsStore } from "../../stores/scripts";
import { useWorkflowsStore } from "../../stores/workflows";
import type { WorkflowDefinitionRecord } from "../../types/workflow";
import { formatDateTime } from "../../utils/device";

const PAGE_SIZES = [10, 20, 30, 50, 100];

type ScriptOption = {
  script_name: string;
  versions: Array<{ script_version: string }>;
};

type SequenceStepForm = {
  id: string;
  node_name: string;
  script_name: string;
  script_version: string;
};

type LoopGroupForm = {
  id: string;
  loop_name: string;
  max_iterations: number;
  steps: SequenceStepForm[];
};

type WorkflowSegmentForm =
  | {
      id: string;
      type: "sequence";
      steps: SequenceStepForm[];
    }
  | {
      id: string;
      type: "loop";
      loop: LoopGroupForm;
    };

function renderWorkflowStatus(status: string) {
  let type: "success" | "danger" | "warning" | "info" = "info";
  if (status === "active") {
    type = "success";
  } else if (status === "draft") {
    type = "warning";
  }
  return h(ElTag, { type, effect: "light" }, () => status || "unknown");
}

function getWorkflowNodes(item?: WorkflowDefinitionRecord | null) {
  return Array.isArray(item?.nodes) ? item.nodes : [];
}

function getWorkflowEdges(item?: WorkflowDefinitionRecord | null) {
  return Array.isArray(item?.edges) ? item.edges : [];
}

function nextID(prefix: string) {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}

function getDefaultScriptSelection(scripts: ScriptOption[]) {
  const firstScript = scripts[0];
  const firstVersion = firstScript?.versions?.[0];
  return {
    script_name: firstScript?.script_name || "",
    script_version: firstVersion?.script_version || ""
  };
}

function createSequenceStep(scripts: ScriptOption[], nodeName = "步骤"): SequenceStepForm {
  const defaults = getDefaultScriptSelection(scripts);
  return {
    id: nextID("step"),
    node_name: nodeName,
    script_name: defaults.script_name,
    script_version: defaults.script_version
  };
}

function createSequenceSegment(scripts: ScriptOption[], stepNames?: string[]): WorkflowSegmentForm {
  const names = Array.isArray(stepNames) && stepNames.length > 0 ? stepNames : ["步骤"];
  return {
    id: nextID("segment_seq"),
    type: "sequence",
    steps: names.map((name) => createSequenceStep(scripts, name))
  };
}

function createLoopSegment(scripts: ScriptOption[], stepNames?: string[], loopName = "循环段", maxIterations = 3): WorkflowSegmentForm {
  const names = Array.isArray(stepNames) && stepNames.length > 0 ? stepNames : ["循环步骤"];
  return {
    id: nextID("segment_loop"),
    type: "loop",
    loop: {
      id: nextID("loop"),
      loop_name: loopName,
      max_iterations: maxIterations,
      steps: names.map((name) => createSequenceStep(scripts, name))
    }
  };
}

function buildWorkflowSummary(item: WorkflowDefinitionRecord) {
  const nodes = getWorkflowNodes(item);
  const loopNodes = nodes.filter((node) => node.node_type === "loop");
  if (loopNodes.length === 0) {
    return "顺序执行";
  }
  return `含 ${loopNodes.length} 个循环段`;
}

function buildWorkflowPathText(item: WorkflowDefinitionRecord) {
  const nodes = getWorkflowNodes(item);
  if (nodes.length === 0) {
    return "暂无节点";
  }

  const labels: string[] = [];
  for (const node of nodes) {
    if (node.node_type === "loop") {
      labels.push(`${node.node_name || "循环"}(${node.max_iterations || 0}次)`);
      continue;
    }
    labels.push(node.node_name || node.node_id);
  }
  return labels.join(" -> ");
}

function ensureStepVersion(step: SequenceStepForm, scripts: ScriptOption[]) {
  const script = scripts.find((item) => item.script_name === step.script_name);
  if (!script) {
    step.script_name = "";
    step.script_version = "";
    return;
  }
  if (!script.versions.some((item) => item.script_version === step.script_version)) {
    step.script_version = script.versions[0]?.script_version || "";
  }
}

export const WorkflowsPage = defineComponent({
  name: "WorkflowsPage",
  setup() {
    const workflowsStore = useWorkflowsStore();
    const scriptsStore = useScriptsStore();

    const { workflows, total, page, pageSize, loading, creating, deletingWorkflowID, errorMessage } = storeToRefs(workflowsStore);
    const { scripts } = storeToRefs(scriptsStore);

    const createDialogVisible = ref(false);
    const createForm = reactive({
      workflow_name: "",
      description: "",
      status: "active"
    });
    const segments = ref<WorkflowSegmentForm[]>([]);

    const workflowDefinitionSummary = computed(() => {
      const totalDefinitions = workflows.value.length;
      const activeDefinitions = workflows.value.filter((item) => item.status === "active").length;
      return `${totalDefinitions} 个定义 / ${activeDefinitions} 个 active`;
    });

    function resetCreateForm() {
      createForm.workflow_name = "";
      createForm.description = "";
      createForm.status = "active";
      segments.value = [
        createSequenceSegment(scripts.value, ["步骤A", "步骤B"]),
        createLoopSegment(scripts.value, ["步骤C", "步骤D", "步骤E"], "循环段1", 3),
        createLoopSegment(scripts.value, ["步骤F", "步骤G"], "循环段2", 5),
        createSequenceSegment(scripts.value, ["步骤H"])
      ];
    }

    async function loadPageData() {
      await Promise.all([workflowsStore.loadWorkflows(), scriptsStore.loadScripts()]);
      if (segments.value.length === 0) {
        resetCreateForm();
      } else {
        for (const segment of segments.value) {
          if (segment.type === "sequence") {
            for (const step of segment.steps) {
              ensureStepVersion(step, scripts.value);
            }
          } else {
            for (const step of segment.loop.steps) {
              ensureStepVersion(step, scripts.value);
            }
          }
        }
      }
    }

    onMounted(() => {
      void loadPageData();
    });

    function openCreateDialog() {
      resetCreateForm();
      createDialogVisible.value = true;
    }

    function addSequenceSegment() {
      segments.value.push(createSequenceSegment(scripts.value, ["新顺序步骤"]));
    }

    function addLoopSegment() {
      segments.value.push(createLoopSegment(scripts.value, ["循环步骤1"], `循环段${segments.value.filter((item) => item.type === "loop").length + 1}`, 3));
    }

    function removeSegment(segmentID: string) {
      if (segments.value.length <= 1) {
        ElMessage.warning("至少需要保留一个编排段");
        return;
      }
      segments.value = segments.value.filter((item) => item.id !== segmentID);
    }

    function moveSegment(segmentID: string, direction: -1 | 1) {
      const index = segments.value.findIndex((item) => item.id === segmentID);
      if (index < 0) {
        return;
      }
      const targetIndex = index + direction;
      if (targetIndex < 0 || targetIndex >= segments.value.length) {
        return;
      }
      const copied = [...segments.value];
      const [current] = copied.splice(index, 1);
      copied.splice(targetIndex, 0, current);
      segments.value = copied;
    }

    function addStepToSequence(segment: WorkflowSegmentForm & { type: "sequence" }) {
      segment.steps.push(createSequenceStep(scripts.value, `步骤${segment.steps.length + 1}`));
    }

    function addStepToLoop(loop: LoopGroupForm) {
      loop.steps.push(createSequenceStep(scripts.value, `循环步骤${loop.steps.length + 1}`));
    }

    function removeStep(stepList: SequenceStepForm[], stepID: string) {
      if (stepList.length <= 1) {
        ElMessage.warning("每个段至少保留一个脚本步骤");
        return;
      }
      const index = stepList.findIndex((item) => item.id === stepID);
      if (index >= 0) {
        stepList.splice(index, 1);
      }
    }

    function updateStepScript(step: SequenceStepForm, scriptName: string) {
      step.script_name = scriptName;
      const selectedScript = scripts.value.find((item) => item.script_name === scriptName);
      step.script_version = selectedScript?.versions?.[0]?.script_version || "";
    }

    function buildWorkflowPayload() {
      const nodes: Array<Record<string, unknown>> = [];
      const edges: Array<Record<string, unknown>> = [];
      let previousTailNodeID = "";
      let nodeSequence = 1;

      for (const segment of segments.value) {
        if (segment.type === "sequence") {
          let firstNodeID = "";
          let previousNodeID = "";

          for (const step of segment.steps) {
            const nodeID = `node_${nodeSequence}`;
            nodeSequence += 1;
            if (firstNodeID === "") {
              firstNodeID = nodeID;
            }
            nodes.push({
              node_id: nodeID,
              node_type: "script",
              node_name: step.node_name.trim(),
              script_name: step.script_name.trim(),
              script_version: step.script_version.trim()
            });
            if (previousNodeID !== "") {
              edges.push({
                from_node_id: previousNodeID,
                to_node_id: nodeID,
                edge_type: "next"
              });
            }
            previousNodeID = nodeID;
          }

          if (previousTailNodeID !== "" && firstNodeID !== "") {
            edges.push({
              from_node_id: previousTailNodeID,
              to_node_id: firstNodeID,
              edge_type: "next"
            });
          }
          previousTailNodeID = previousNodeID;
          continue;
        }

        const loopNodeID = `node_${nodeSequence}`;
        nodeSequence += 1;
        nodes.push({
          node_id: loopNodeID,
          node_type: "loop",
          node_name: segment.loop.loop_name.trim(),
          max_iterations: Number(segment.loop.max_iterations || 0)
        });

        if (previousTailNodeID !== "") {
          edges.push({
            from_node_id: previousTailNodeID,
            to_node_id: loopNodeID,
            edge_type: "next"
          });
        }

        let firstBodyNodeID = "";
        let previousBodyNodeID = "";
        for (const step of segment.loop.steps) {
          const bodyNodeID = `node_${nodeSequence}`;
          nodeSequence += 1;
          if (firstBodyNodeID === "") {
            firstBodyNodeID = bodyNodeID;
          }
          nodes.push({
            node_id: bodyNodeID,
            node_type: "script",
            node_name: step.node_name.trim(),
            script_name: step.script_name.trim(),
            script_version: step.script_version.trim()
          });
          if (previousBodyNodeID !== "") {
            edges.push({
              from_node_id: previousBodyNodeID,
              to_node_id: bodyNodeID,
              edge_type: "next"
            });
          }
          previousBodyNodeID = bodyNodeID;
        }

        if (firstBodyNodeID !== "") {
          edges.push({
            from_node_id: loopNodeID,
            to_node_id: firstBodyNodeID,
            edge_type: "loop_body"
          });
        }
        if (previousBodyNodeID !== "") {
          edges.push({
            from_node_id: previousBodyNodeID,
            to_node_id: loopNodeID,
            edge_type: "next"
          });
        }

        previousTailNodeID = loopNodeID;
      }

      const stopNodeID = `node_${nodeSequence}`;
      nodes.push({
        node_id: stopNodeID,
        node_type: "stop",
        node_name: "结束"
      });

      if (previousTailNodeID !== "") {
        const tailSegment = segments.value[segments.value.length - 1];
        const edgeType = tailSegment?.type === "loop" ? "loop_exit" : "next";
        edges.push({
          from_node_id: previousTailNodeID,
          to_node_id: stopNodeID,
          edge_type: edgeType
        });
      }

      return {
        nodes,
        edges
      };
    }

    function validateCreateForm() {
      if (createForm.workflow_name.trim() === "") {
        return "请先填写工作流名称";
      }
      if (segments.value.length === 0) {
        return "请至少配置一个编排段";
      }

      for (const segment of segments.value) {
        if (segment.type === "sequence") {
          if (segment.steps.length === 0) {
            return "顺序段至少需要一个脚本步骤";
          }
          for (const step of segment.steps) {
            if (step.node_name.trim() === "" || step.script_name.trim() === "" || step.script_version.trim() === "") {
              return "请完整填写顺序段中的步骤名称、脚本名称和版本";
            }
          }
          continue;
        }

        if (segment.loop.loop_name.trim() === "") {
          return "请填写循环段名称";
        }
        if (Number(segment.loop.max_iterations || 0) <= 0) {
          return "循环段最大次数必须大于 0";
        }
        if (segment.loop.steps.length === 0) {
          return "循环段至少需要一个循环体脚本步骤";
        }
        for (const step of segment.loop.steps) {
          if (step.node_name.trim() === "" || step.script_name.trim() === "" || step.script_version.trim() === "") {
            return "请完整填写循环段中的步骤名称、脚本名称和版本";
          }
        }
      }

      return "";
    }

    async function handleCreateWorkflow() {
      const validationMessage = validateCreateForm();
      if (validationMessage !== "") {
        ElMessage.warning(validationMessage);
        return;
      }

      try {
        const payload = buildWorkflowPayload();
        await workflowsStore.submitWorkflow({
          workflow_name: createForm.workflow_name.trim(),
          description: createForm.description.trim(),
          status: createForm.status,
          nodes: payload.nodes,
          edges: payload.edges
        });
        createDialogVisible.value = false;
        ElMessage.success("工作流定义已创建");
        await loadPageData();
      } catch (error) {
        ElMessage.error("创建工作流定义失败，请检查脚本、版本和编排结构");
        throw error;
      }
    }

    async function handleDeleteWorkflow(workflow: WorkflowDefinitionRecord) {
      try {
        await ElMessageBox.confirm(`确认删除工作流 ${workflow.workflow_name} 吗？`, "删除工作流确认", {
          confirmButtonText: "确认删除",
          cancelButtonText: "取消",
          type: "warning"
        });

        await workflowsStore.removeWorkflow(workflow.workflow_def_id);
        ElMessage.success("工作流定义已删除");
      } catch (error) {
        if (error === "cancel" || error === "close") {
          return;
        }
        ElMessage.error("删除工作流定义失败。如果该工作流仍有运行中的实例，请先停止对应计划任务实例。");
      }
    }

    function renderStepEditor(step: SequenceStepForm, stepList: SequenceStepForm[]) {
      const availableVersions = scripts.value.find((item) => item.script_name === step.script_name)?.versions || [];
      return h("div", { class: "workflow-step-editor" }, [
        h("div", { class: "workflow-step-editor__grid" }, [
          h(ElFormItem, { label: "步骤名称" }, () =>
            h(ElInput, {
              modelValue: step.node_name,
              "onUpdate:modelValue": (value: string) => {
                step.node_name = value;
              },
              placeholder: "例如：打开 QQ"
            })
          ),
          h(ElFormItem, { label: "脚本名称" }, () =>
            h(
              ElSelect,
              {
                modelValue: step.script_name,
                "onUpdate:modelValue": (value: string) => {
                  updateStepScript(step, value);
                }
              },
              () => scripts.value.map((item) => h(ElOption, { key: `${step.id}-${item.script_name}`, label: item.script_name, value: item.script_name }))
            )
          ),
          h(ElFormItem, { label: "脚本版本" }, () =>
            h(
              ElSelect,
              {
                modelValue: step.script_version,
                "onUpdate:modelValue": (value: string) => {
                  step.script_version = value;
                }
              },
              () => availableVersions.map((item) => h(ElOption, { key: `${step.id}-${item.script_version}`, label: item.script_version, value: item.script_version }))
            )
          )
        ]),
        h(
          "div",
          { class: "table-actions" },
          h(
            ElButton,
            {
              link: true,
              type: "danger",
              onClick: () => {
                removeStep(stepList, step.id);
              }
            },
            () => "删除步骤"
          )
        )
      ]);
    }

    function renderSequenceSegment(segment: WorkflowSegmentForm & { type: "sequence" }, index: number) {
      return h(
        ElCard,
        { class: "workflow-segment-card", shadow: "never" },
        {
          header: () =>
            h("div", { class: "card-header" }, [
              h("div", null, [
                h("div", { class: "card-header__title" }, `顺序段 ${index + 1}`),
                h("div", { class: "card-header__subtitle" }, "该段中的脚本步骤会按顺序串行执行")
              ]),
              h("div", { class: "table-actions" }, [
                h(
                  ElButton,
                  {
                    link: true,
                    disabled: index === 0,
                    onClick: () => moveSegment(segment.id, -1)
                  },
                  () => "上移"
                ),
                h(
                  ElButton,
                  {
                    link: true,
                    disabled: index === segments.value.length - 1,
                    onClick: () => moveSegment(segment.id, 1)
                  },
                  () => "下移"
                ),
                h(
                  ElButton,
                  {
                    link: true,
                    type: "danger",
                    onClick: () => removeSegment(segment.id)
                  },
                  () => "删除段"
                )
              ])
            ]),
          default: () =>
            h("div", { class: "workflow-segment-body" }, [
              ...segment.steps.map((step) => renderStepEditor(step, segment.steps)),
              h(
                ElButton,
                {
                  plain: true,
                  onClick: () => addStepToSequence(segment)
                },
                () => "添加步骤"
              )
            ])
        }
      );
    }

    function renderLoopSegment(segment: WorkflowSegmentForm & { type: "loop" }, index: number) {
      return h(
        ElCard,
        { class: "workflow-segment-card workflow-segment-card--loop", shadow: "never" },
        {
          header: () =>
            h("div", { class: "card-header" }, [
              h("div", null, [
                h("div", { class: "card-header__title" }, `循环段 ${index + 1}`),
                h("div", { class: "card-header__subtitle" }, "循环节点控制进入循环体的总次数")
              ]),
              h("div", { class: "table-actions" }, [
                h(
                  ElButton,
                  {
                    link: true,
                    disabled: index === 0,
                    onClick: () => moveSegment(segment.id, -1)
                  },
                  () => "上移"
                ),
                h(
                  ElButton,
                  {
                    link: true,
                    disabled: index === segments.value.length - 1,
                    onClick: () => moveSegment(segment.id, 1)
                  },
                  () => "下移"
                ),
                h(
                  ElButton,
                  {
                    link: true,
                    type: "danger",
                    onClick: () => removeSegment(segment.id)
                  },
                  () => "删除段"
                )
              ])
            ]),
          default: () =>
            h("div", { class: "workflow-segment-body" }, [
              h("div", { class: "workflow-segment-grid" }, [
                h(ElFormItem, { label: "循环段名称" }, () =>
                  h(ElInput, {
                    modelValue: segment.loop.loop_name,
                    "onUpdate:modelValue": (value: string) => {
                      segment.loop.loop_name = value;
                    },
                    placeholder: "例如：CDE 循环"
                  })
                ),
                h(ElFormItem, { label: "最大循环次数" }, () =>
                  h(ElInputNumber, {
                    modelValue: segment.loop.max_iterations,
                    "onUpdate:modelValue": (value: number) => {
                      segment.loop.max_iterations = Number(value || 0);
                    },
                    min: 1,
                    step: 1
                  })
                )
              ]),
              ...segment.loop.steps.map((step) => renderStepEditor(step, segment.loop.steps)),
              h(
                ElButton,
                {
                  plain: true,
                  onClick: () => addStepToLoop(segment.loop)
                },
                () => "添加循环体步骤"
              )
            ])
        }
      );
    }

    return () =>
      h("section", { class: "workflows-page" }, [
        h("div", { class: "page-toolbar" }, [
          h(ElButton, { type: "primary", onClick: openCreateDialog }, () => "创建工作流"),
          h(
            ElButton,
            {
              loading: loading.value,
              onClick: () => {
                void loadPageData();
              }
            },
            () => "刷新"
          )
        ]),
        errorMessage.value
          ? h(ElAlert, { class: "page-alert", type: "error", title: `工作流定义加载失败：${errorMessage.value}`, showIcon: true, closable: false })
          : null,
        h(ElAlert, {
          class: "page-alert",
          type: "info",
          showIcon: true,
          closable: false,
          title: "工作流编排页只负责维护工作流定义。正式运行请在“计划任务”页创建 workflow 型计划任务，并在“计划任务实例”页查看运行状态。每日开始、每日截止、次日重启等调度语义归计划任务负责。"
        }),
        h(
          ElCard,
          { class: "page-card page-fill-card", shadow: "never" },
          {
            header: () =>
              h("div", { class: "card-header" }, [
                h("div", null, [
                  h("div", { class: "card-header__title" }, "工作流定义列表"),
                  h("div", { class: "card-header__subtitle" }, `当前仅维护定义，不直接承担运行态调度。${workflowDefinitionSummary.value}`)
                ])
              ]),
            default: () =>
              workflows.value.length === 0
                ? h(ElEmpty, { description: "当前还没有工作流定义，请先点击“创建工作流”。" })
                : h("div", { class: "page-scroll-body" }, [
                    h("div", { class: "table-scroll-region table-scroll-region--soft" }, [
                      h(
                        ElTable,
                        { data: workflows.value, stripe: true, class: "tasks-table", tableLayout: "fixed", height: "100%" },
                        {
                          default: () => [
                            h(
                              ElTableColumn,
                              { label: "工作流", minWidth: 220 },
                              {
                                default: ({ row }) =>
                                  h("div", null, [
                                    h("div", { class: "devices-table__name" }, row.workflow_name),
                                    h("div", { class: "devices-table__meta" }, row.workflow_def_id)
                                  ])
                              }
                            ),
                            h(ElTableColumn, { label: "状态", width: 120 }, { default: ({ row }) => renderWorkflowStatus(row.status) }),
                            h(ElTableColumn, { label: "节点数", minWidth: 120, formatter: (row) => `${getWorkflowNodes(row).length} 个节点` }),
                            h(ElTableColumn, { label: "编排摘要", minWidth: 150, formatter: (row) => buildWorkflowSummary(row) }),
                            h(ElTableColumn, { label: "路径预览", minWidth: 280, formatter: (row) => buildWorkflowPathText(row) }),
                            h(ElTableColumn, { label: "说明", minWidth: 220, formatter: (row) => row.description || "暂无说明" }),
                            h(ElTableColumn, { label: "更新时间", minWidth: 180, formatter: (row) => formatDateTime(row.updated_at) }),
                            h(
                              ElTableColumn,
                              { label: "操作", minWidth: 120, fixed: "right" },
                              {
                                default: ({ row }) =>
                                  h("div", { class: "table-actions" }, [
                                    h(
                                      ElButton,
                                      {
                                        link: true,
                                        type: "danger",
                                        loading: deletingWorkflowID.value === row.workflow_def_id,
                                        onClick: () => {
                                          void handleDeleteWorkflow(row);
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
                          void workflowsStore.changePage(value);
                        },
                        "onUpdate:pageSize": (value: number) => {
                          void workflowsStore.changePageSize(value);
                        }
                      })
                    )
                  ])
          }
        ),
        h(
          ElDialog,
          {
            modelValue: createDialogVisible.value,
            "onUpdate:modelValue": (value: boolean) => (createDialogVisible.value = value),
            title: "创建工作流",
            width: "980px",
            closeOnClickModal: false
          },
          {
            default: () =>
              h(ElForm, { labelPosition: "top", class: "dialog-form" }, () => [
                h(ElFormItem, { label: "工作流名称" }, () =>
                  h(ElInput, {
                    modelValue: createForm.workflow_name,
                    "onUpdate:modelValue": (value: string) => {
                      createForm.workflow_name = value;
                    },
                    placeholder: "例如：A -> B -> CDE循环 -> FG循环 -> H -> stop"
                  })
                ),
                h(ElFormItem, { label: "说明" }, () =>
                  h(ElInput, {
                    modelValue: createForm.description,
                    "onUpdate:modelValue": (value: string) => {
                      createForm.description = value;
                    },
                    type: "textarea",
                    rows: 3,
                    placeholder: "填写当前工作流的业务说明"
                  })
                ),
                h(ElFormItem, { label: "状态" }, () =>
                  h(
                    ElSelect,
                    {
                      modelValue: createForm.status,
                      "onUpdate:modelValue": (value: string) => {
                        createForm.status = value;
                      }
                    },
                    () => [h(ElOption, { label: "active", value: "active" }), h(ElOption, { label: "draft", value: "draft" })]
                  )
                ),
                h(ElAlert, {
                  class: "dialog-alert",
                  type: "info",
                  showIcon: true,
                  closable: false,
                  title: "这里维护的是工作流定义本身。每日开始时间、每日截止时间、次日自动重启等调度语义请放到计划任务中配置。"
                }),
                h("div", { class: "table-actions workflow-builder-actions" }, [
                  h(ElButton, { onClick: addSequenceSegment }, () => "添加顺序段"),
                  h(ElButton, { onClick: addLoopSegment }, () => "添加循环段")
                ]),
                h(
                  "div",
                  { class: "workflow-builder-list" },
                  segments.value.map((segment, index) =>
                    segment.type === "sequence" ? renderSequenceSegment(segment, index) : renderLoopSegment(segment, index)
                  )
                )
              ]),
            footer: () =>
              h("div", { class: "dialog-footer" }, [
                h(ElButton, { onClick: () => (createDialogVisible.value = false) }, () => "取消"),
                h(ElButton, { type: "primary", loading: creating.value, onClick: () => void handleCreateWorkflow() }, () => "确认创建")
              ])
          }
        )
      ]);
  }
});

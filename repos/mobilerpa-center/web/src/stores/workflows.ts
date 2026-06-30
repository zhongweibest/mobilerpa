import { defineStore } from "pinia";
import { ref } from "vue";

import {
  createWorkflow,
  deleteWorkflow,
  fetchWorkflowDetail,
  fetchWorkflows,
  updateWorkflow
} from "../api/workflows";
import type { CreateWorkflowRequest, UpdateWorkflowRequest, WorkflowDefinitionRecord } from "../types/workflow";

export const useWorkflowsStore = defineStore("workflows", () => {
  const workflows = ref<WorkflowDefinitionRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const selectedWorkflow = ref<WorkflowDefinitionRecord | null>(null);
  const loading = ref(false);
  const creating = ref(false);
  const deletingWorkflowID = ref("");
  const errorMessage = ref("");

  async function loadWorkflows() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchWorkflows({
        page: page.value,
        page_size: pageSize.value
      });
      workflows.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_workflows_failed";
      throw error;
    } finally {
      loading.value = false;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadWorkflows();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadWorkflows();
  }

  async function loadWorkflowDetail(workflowDefID: string) {
    errorMessage.value = "";
    try {
      selectedWorkflow.value = await fetchWorkflowDetail(workflowDefID);
      return selectedWorkflow.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_workflow_detail_failed";
      throw error;
    }
  }

  async function submitWorkflow(payload: CreateWorkflowRequest) {
    creating.value = true;
    errorMessage.value = "";
    try {
      const workflow = await createWorkflow(payload);
      return workflow;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "create_workflow_failed";
      throw error;
    } finally {
      creating.value = false;
    }
  }

  async function saveWorkflow(workflowDefID: string, payload: UpdateWorkflowRequest) {
    creating.value = true;
    errorMessage.value = "";
    try {
      const workflow = await updateWorkflow(workflowDefID, payload);
      return workflow;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "update_workflow_failed";
      throw error;
    } finally {
      creating.value = false;
    }
  }

  async function removeWorkflow(workflowDefID: string) {
    deletingWorkflowID.value = workflowDefID;
    errorMessage.value = "";
    try {
      await deleteWorkflow(workflowDefID);
      workflows.value = workflows.value.filter((item) => item.workflow_def_id !== workflowDefID);
      if (total.value > 0) {
        total.value -= 1;
      }
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_workflow_failed";
      throw error;
    } finally {
      deletingWorkflowID.value = "";
    }
  }

  return {
    workflows,
    total,
    page,
    pageSize,
    selectedWorkflow,
    loading,
    creating,
    deletingWorkflowID,
    errorMessage,
    loadWorkflows,
    changePage,
    changePageSize,
    loadWorkflowDetail,
    submitWorkflow,
    saveWorkflow,
    removeWorkflow
  };
});

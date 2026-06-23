import { defineStore } from "pinia";
import { ref } from "vue";

import {
  createWorkflow,
  deleteWorkflow,
  deleteWorkflowInstance,
  fetchAllWorkflowInstances,
  fetchWorkflowDetail,
  fetchWorkflowEvents,
  fetchWorkflowInstances,
  fetchWorkflowRuns,
  fetchWorkflows,
  startWorkflow,
  stopWorkflow
} from "../api/workflows";
import type { CreateWorkflowRequest, WorkflowDefinitionRecord, WorkflowEventRecord, WorkflowInstanceRecord, WorkflowRunRecord, WorkflowRunSummary } from "../types/workflow";

export const useWorkflowsStore = defineStore("workflows", () => {
  const workflows = ref<WorkflowDefinitionRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const selectedWorkflow = ref<WorkflowDefinitionRecord | null>(null);
  const selectedWorkflowInstances = ref<WorkflowInstanceRecord[]>([]);
  const workflowInstances = ref<WorkflowInstanceRecord[]>([]);
  const selectedWorkflowRuns = ref<WorkflowRunRecord[]>([]);
  const selectedWorkflowEvents = ref<WorkflowEventRecord[]>([]);
  const selectedWorkflowRunID = ref("");
  const loading = ref(false);
  const creating = ref(false);
  const deletingWorkflowID = ref("");
  const startingWorkflowID = ref("");
  const loadingRuns = ref(false);
  const loadingEvents = ref(false);
  const stoppingWorkflowID = ref("");
  const deletingWorkflowInstanceID = ref("");
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

  async function loadWorkflowRuns(workflowDefID: string) {
    loadingRuns.value = true;
    errorMessage.value = "";
    try {
      selectedWorkflowRuns.value = await fetchWorkflowRuns(workflowDefID);
      return selectedWorkflowRuns.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_workflow_runs_failed";
      throw error;
    } finally {
      loadingRuns.value = false;
    }
  }

  async function loadWorkflowInstances(workflowDefID: string) {
    loadingRuns.value = true;
    errorMessage.value = "";
    try {
      selectedWorkflowInstances.value = await fetchWorkflowInstances(workflowDefID);
      selectedWorkflowRuns.value = selectedWorkflowInstances.value.flatMap((item) => item.device_runs || []);
      return selectedWorkflowInstances.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_workflow_instances_failed";
      throw error;
    } finally {
      loadingRuns.value = false;
    }
  }

  async function loadAllWorkflowInstances() {
    loadingRuns.value = true;
    errorMessage.value = "";
    try {
      workflowInstances.value = await fetchAllWorkflowInstances();
      return workflowInstances.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_all_workflow_instances_failed";
      throw error;
    } finally {
      loadingRuns.value = false;
    }
  }

  async function loadWorkflowEvents(workflowDefID: string, workflowRunID: string) {
    loadingEvents.value = true;
    errorMessage.value = "";
    try {
      selectedWorkflowRunID.value = workflowRunID;
      selectedWorkflowEvents.value = await fetchWorkflowEvents(workflowDefID, workflowRunID);
      return selectedWorkflowEvents.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_workflow_events_failed";
      throw error;
    } finally {
      loadingEvents.value = false;
    }
  }

  async function triggerStartWorkflow(workflowDefID: string, deviceIDs: string[]) {
    startingWorkflowID.value = workflowDefID;
    errorMessage.value = "";
    try {
      const instance = await startWorkflow(workflowDefID, deviceIDs);
      selectedWorkflowInstances.value = [instance];
      selectedWorkflowRuns.value = instance.device_runs || [];
      await loadWorkflows();
      return instance;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "start_workflow_failed";
      throw error;
    } finally {
      startingWorkflowID.value = "";
    }
  }

  async function terminateWorkflow(workflowDefID: string, workflowInstanceID: string) {
    stoppingWorkflowID.value = workflowInstanceID;
    errorMessage.value = "";
    try {
      const instance = await stopWorkflow(workflowDefID, workflowInstanceID);
      selectedWorkflowInstances.value = [instance];
      selectedWorkflowRuns.value = instance.device_runs || [];
      workflowInstances.value = workflowInstances.value.map((item) =>
        item.workflow_instance_id === instance.workflow_instance_id ? instance : item
      );
      await loadWorkflows();
      return instance;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "stop_workflow_failed";
      throw error;
    } finally {
      stoppingWorkflowID.value = "";
    }
  }

  async function removeWorkflowInstance(workflowDefID: string, workflowInstanceID: string) {
    deletingWorkflowInstanceID.value = workflowInstanceID;
    errorMessage.value = "";
    try {
      await deleteWorkflowInstance(workflowDefID, workflowInstanceID);
      workflowInstances.value = workflowInstances.value.filter((item) => item.workflow_instance_id !== workflowInstanceID);
      selectedWorkflowInstances.value = selectedWorkflowInstances.value.filter((item) => item.workflow_instance_id !== workflowInstanceID);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_workflow_instance_failed";
      throw error;
    } finally {
      deletingWorkflowInstanceID.value = "";
    }
  }

  function summarizeRuns(runs: WorkflowRunRecord[]): WorkflowRunSummary {
    const summary: WorkflowRunSummary = {
      total: runs.length,
      pending: 0,
      running: 0,
      success: 0,
      failed: 0,
      stopped: 0
    };

    for (const item of runs) {
      switch (item.status) {
        case "pending":
          summary.pending += 1;
          break;
        case "running":
          summary.running += 1;
          break;
        case "success":
          summary.success += 1;
          break;
        case "failed":
          summary.failed += 1;
          break;
        case "stopped":
          summary.stopped += 1;
          break;
        default:
          break;
      }
    }
    return summary;
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
    selectedWorkflowInstances,
    workflowInstances,
    selectedWorkflowRuns,
    selectedWorkflowEvents,
    selectedWorkflowRunID,
    loading,
    creating,
    deletingWorkflowID,
    startingWorkflowID,
    loadingRuns,
    loadingEvents,
    stoppingWorkflowID,
    deletingWorkflowInstanceID,
    errorMessage,
    loadWorkflows,
    changePage,
    changePageSize,
    loadWorkflowDetail,
    submitWorkflow,
    loadWorkflowInstances,
    loadAllWorkflowInstances,
    loadWorkflowRuns,
    loadWorkflowEvents,
    triggerStartWorkflow,
    terminateWorkflow,
    removeWorkflowInstance,
    summarizeRuns,
    removeWorkflow
  };
});

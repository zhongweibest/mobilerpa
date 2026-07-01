import { defineStore } from "pinia";
import { ref } from "vue";

import { addPlanRows, createPlan, deletePlan, deletePlanRun, fetchPlanEvents, fetchPlanRuns, fetchPlans, removePlanRow, startPlan, stopPlanDeviceRun, stopPlanRun, updatePlanRows, updatePlanStatus } from "../api/plans";
import type { CreatePlanRequest, PlanDefinitionRecord, PlanEventRecord, PlanRunRecord, PlanRowBinding } from "../types/plan";

export const usePlansStore = defineStore("plans", () => {
  const plans = ref<PlanDefinitionRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const runs = ref<PlanRunRecord[]>([]);
  const runsTotal = ref(0);
  const runsPage = ref(1);
  const runsPageSize = ref(10);
  const selectedEvents = ref<PlanEventRecord[]>([]);
  const loading = ref(false);
  const loadingRuns = ref(false);
  const loadingEvents = ref(false);
  const creating = ref(false);
  const deletingPlanID = ref("");
  const startingPlanID = ref("");
  const stoppingRunID = ref("");
  const deletingRunID = ref("");
  const mutatingDevices = ref(false);
  const mutatingStatusPlanID = ref("");
  const errorMessage = ref("");

  async function loadPlans() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchPlans({
        page: page.value,
        page_size: pageSize.value
      });
      plans.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_plans_failed";
      throw error;
    } finally {
      loading.value = false;
    }
  }

  async function loadRuns() {
    loadingRuns.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchPlanRuns({
        page: runsPage.value,
        page_size: runsPageSize.value
      });
      runs.value = result.items;
      runsTotal.value = result.total;
      runsPage.value = result.page;
      runsPageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_plan_runs_failed";
      throw error;
    } finally {
      loadingRuns.value = false;
    }
  }

  async function loadPlanEvents(planDefID: string, planRunID: string) {
    loadingEvents.value = true;
    errorMessage.value = "";
    try {
      selectedEvents.value = (await fetchPlanEvents(planDefID, planRunID)).sort((left, right) => String(right.created_at || "").localeCompare(String(left.created_at || "")));
      return selectedEvents.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_plan_events_failed";
      throw error;
    } finally {
      loadingEvents.value = false;
    }
  }

  async function submitPlan(payload: CreatePlanRequest) {
    creating.value = true;
    errorMessage.value = "";
    try {
      return await createPlan(payload);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "create_plan_failed";
      throw error;
    } finally {
      creating.value = false;
    }
  }

  async function triggerStartPlan(planDefID: string) {
    startingPlanID.value = planDefID;
    errorMessage.value = "";
    try {
      const run = await startPlan(planDefID);
      await loadRuns();
      return run;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "start_plan_failed";
      throw error;
    } finally {
      startingPlanID.value = "";
    }
  }

  async function removePlan(planDefID: string) {
    deletingPlanID.value = planDefID;
    errorMessage.value = "";
    try {
      await deletePlan(planDefID);
      await loadPlans();
      if (plans.value.length === 0 && page.value > 1) {
        page.value -= 1;
        await loadPlans();
      }
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_plan_failed";
      throw error;
    } finally {
      deletingPlanID.value = "";
    }
  }

  async function updateDefinitionRows(planDefID: string, rows: PlanRowBinding[]) {
    mutatingDevices.value = true;
    errorMessage.value = "";
    try {
      const result = await updatePlanRows(planDefID, { rows });
      await loadPlans();
      await loadRuns();
      return result;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "update_plan_rows_failed";
      throw error;
    } finally {
      mutatingDevices.value = false;
    }
  }

  async function togglePlanStatus(planDefID: string, status: string) {
    mutatingStatusPlanID.value = planDefID;
    errorMessage.value = "";
    try {
      const result = await updatePlanStatus(planDefID, status);
      await loadPlans();
      return result;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "update_plan_status_failed";
      throw error;
    } finally {
      mutatingStatusPlanID.value = "";
    }
  }

  async function triggerStopPlanRun(planDefID: string, planRunID: string) {
    stoppingRunID.value = planRunID;
    errorMessage.value = "";
    try {
      const run = await stopPlanRun(planDefID, planRunID);
      await loadRuns();
      return run;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "stop_plan_run_failed";
      throw error;
    } finally {
      stoppingRunID.value = "";
    }
  }

  async function triggerStopPlanDeviceRun(planDefID: string, planRunID: string, planDeviceRunID: string) {
    mutatingDevices.value = true;
    errorMessage.value = "";
    try {
      const run = await stopPlanDeviceRun(planDefID, planRunID, planDeviceRunID);
      await loadRuns();
      return run;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "stop_plan_device_run_failed";
      throw error;
    } finally {
      mutatingDevices.value = false;
    }
  }

  async function appendPlanRows(planDefID: string, planRunID: string, rows: PlanRowBinding[]) {
    mutatingDevices.value = true;
    errorMessage.value = "";
    try {
      const run = await addPlanRows(planDefID, planRunID, rows);
      await loadRuns();
      return run;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "add_plan_rows_failed";
      throw error;
    } finally {
      mutatingDevices.value = false;
    }
  }

  async function removePlanRun(planDefID: string, planRunID: string) {
    deletingRunID.value = planRunID;
    errorMessage.value = "";
    try {
      await deletePlanRun(planDefID, planRunID);
      await loadRuns();
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_plan_run_failed";
      throw error;
    } finally {
      deletingRunID.value = "";
    }
  }

  async function removeRowFromPlan(planDefID: string, planRunID: string, zoneID: string, rowID: string) {
    mutatingDevices.value = true;
    errorMessage.value = "";
    try {
      const run = await removePlanRow(planDefID, planRunID, zoneID, rowID);
      await loadRuns();
      return run;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "remove_plan_row_failed";
      throw error;
    } finally {
      mutatingDevices.value = false;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadPlans();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadPlans();
  }

  async function changeRunsPage(nextPage: number) {
    runsPage.value = nextPage;
    await loadRuns();
  }

  async function changeRunsPageSize(nextPageSize: number) {
    runsPageSize.value = nextPageSize;
    runsPage.value = 1;
    await loadRuns();
  }

  return {
    plans,
    total,
    page,
    pageSize,
    runs,
    runsTotal,
    runsPage,
    runsPageSize,
    selectedEvents,
    loading,
    loadingRuns,
    loadingEvents,
    creating,
    deletingPlanID,
    startingPlanID,
    stoppingRunID,
    deletingRunID,
    mutatingDevices,
    mutatingStatusPlanID,
    errorMessage,
    loadPlans,
    loadRuns,
    loadPlanEvents,
    submitPlan,
    removePlan,
    updateDefinitionRows,
    togglePlanStatus,
    triggerStartPlan,
    triggerStopPlanRun,
    triggerStopPlanDeviceRun,
    removePlanRun,
    appendPlanRows,
    removeRowFromPlan,
    changePage,
    changePageSize,
    changeRunsPage,
    changeRunsPageSize
  };
});

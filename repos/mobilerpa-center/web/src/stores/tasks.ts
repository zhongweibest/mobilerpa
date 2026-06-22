import { defineStore } from "pinia";
import { ref } from "vue";

import { assignTask, createTask, deleteTask, fetchTaskEvents, fetchTasks } from "../api/tasks";
import type { CreateTaskRequest, TaskEventRecord, TaskRecord } from "../types/task";

export const useTasksStore = defineStore("tasks", () => {
  const tasks = ref<TaskRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const selectedTaskID = ref("");
  const selectedTaskEvents = ref<TaskEventRecord[]>([]);
  const loading = ref(false);
  const assigningTaskID = ref("");
  const deletingTaskID = ref("");
  const creating = ref(false);
  const errorMessage = ref("");

  async function loadTasks() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchTasks({
        page: page.value,
        page_size: pageSize.value
      });
      tasks.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_tasks_failed";
    } finally {
      loading.value = false;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadTasks();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadTasks();
  }

  async function submitTask(payload: CreateTaskRequest) {
    creating.value = true;
    errorMessage.value = "";
    try {
      const task = await createTask(payload);
      total.value += 1;
      page.value = 1;
      await loadTasks();
      return task;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "create_task_failed";
      throw error;
    } finally {
      creating.value = false;
    }
  }

  async function dispatchTask(taskID: string) {
    assigningTaskID.value = taskID;
    errorMessage.value = "";
    try {
      const updated = await assignTask(taskID);
      tasks.value = tasks.value.map((item) => (item.task_id === taskID ? updated : item));
      if (selectedTaskID.value === taskID) {
        await loadTaskEvents(taskID);
      }
      return updated;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "assign_task_failed";
      throw error;
    } finally {
      assigningTaskID.value = "";
    }
  }

  async function loadTaskEvents(taskID: string) {
    selectedTaskID.value = taskID;
    errorMessage.value = "";
    try {
      selectedTaskEvents.value = await fetchTaskEvents(taskID);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_task_events_failed";
      throw error;
    }
  }

  async function removeTask(taskID: string) {
    deletingTaskID.value = taskID;
    errorMessage.value = "";
    try {
      await deleteTask(taskID);
      await loadTasks();
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_task_failed";
      throw error;
    } finally {
      deletingTaskID.value = "";
    }
  }

  return {
    tasks,
    total,
    page,
    pageSize,
    selectedTaskID,
    selectedTaskEvents,
    loading,
    assigningTaskID,
    deletingTaskID,
    creating,
    errorMessage,
    loadTasks,
    changePage,
    changePageSize,
    submitTask,
    dispatchTask,
    loadTaskEvents,
    removeTask
  };
});

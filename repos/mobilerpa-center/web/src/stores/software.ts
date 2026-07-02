import { defineStore } from "pinia";
import { ref } from "vue";

import { createSoftware, deleteSoftware, fetchSoftware, updateSoftware } from "../api/software";
import type { CreateSoftwareRequest, SoftwarePackageRecord, UpdateSoftwareRequest } from "../types/software";

export const useSoftwareStore = defineStore("software", () => {
  const items = ref<SoftwarePackageRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const loading = ref(false);
  const submitting = ref(false);
  const errorMessage = ref("");

  async function loadSoftware() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchSoftware({
        page: page.value,
        page_size: pageSize.value
      });
      items.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_software_failed";
      throw error;
    } finally {
      loading.value = false;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadSoftware();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadSoftware();
  }

  async function submitSoftware(payload: CreateSoftwareRequest) {
    submitting.value = true;
    errorMessage.value = "";
    try {
      const result = await createSoftware(payload);
      await loadSoftware();
      return result;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "create_software_failed";
      throw error;
    } finally {
      submitting.value = false;
    }
  }

  async function saveSoftware(payload: UpdateSoftwareRequest) {
    submitting.value = true;
    errorMessage.value = "";
    try {
      const result = await updateSoftware(payload);
      await loadSoftware();
      return result;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "update_software_failed";
      throw error;
    } finally {
      submitting.value = false;
    }
  }

  async function removeSoftware(softwareID: string) {
    errorMessage.value = "";
    try {
      await deleteSoftware(softwareID);
      await loadSoftware();
      if (items.value.length === 0 && page.value > 1) {
        page.value -= 1;
        await loadSoftware();
      }
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_software_failed";
      throw error;
    }
  }

  return {
    items,
    total,
    page,
    pageSize,
    loading,
    submitting,
    errorMessage,
    loadSoftware,
    changePage,
    changePageSize,
    submitSoftware,
    saveSoftware,
    removeSoftware
  };
});

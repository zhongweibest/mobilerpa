import { defineStore } from "pinia";
import { ref } from "vue";

import { deleteDevice, fetchDevices } from "../api/devices";
import type { DeviceRecord } from "../types/device";

export const useDevicesStore = defineStore("devices", () => {
  const devices = ref<DeviceRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const filters = ref({
    slot_zone: "",
    slot_row: "",
    slot_position: ""
  });
  const loading = ref(false);
  const deletingDeviceID = ref("");
  const errorMessage = ref("");

  async function loadDevices() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchDevices({
        page: page.value,
        page_size: pageSize.value,
        slot_zone: filters.value.slot_zone,
        slot_row: filters.value.slot_row,
        slot_position: filters.value.slot_position
      });
      devices.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_devices_failed";
    } finally {
      loading.value = false;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadDevices();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadDevices();
  }

  async function applyFilters(nextFilters: { slot_zone: string; slot_row: string; slot_position: string }) {
    filters.value = {
      slot_zone: nextFilters.slot_zone || "",
      slot_row: nextFilters.slot_row || "",
      slot_position: nextFilters.slot_position || ""
    };
    page.value = 1;
    await loadDevices();
  }

  async function removeDevice(deviceID: string) {
    deletingDeviceID.value = deviceID;
    errorMessage.value = "";
    try {
      await deleteDevice(deviceID);
      if (total.value > 0) {
        total.value -= 1;
      }
      await loadDevices();
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_device_failed";
      throw error;
    } finally {
      deletingDeviceID.value = "";
    }
  }

  return {
    devices,
    total,
    page,
    pageSize,
    filters,
    loading,
    deletingDeviceID,
    errorMessage,
    loadDevices,
    changePage,
    changePageSize,
    applyFilters,
    removeDevice
  };
});

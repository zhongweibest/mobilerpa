import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { controlAgent, deployAgents, fetchDiscoveredDevices, pairDevice } from "../api/discovery";
import { fetchDiscoverySettings, saveDiscoverySettings } from "../api/settings";
import type { AgentActionResult, AgentDeploymentResult, DiscoveredDevice, PairDeviceResult } from "../types/discovery";

export const useDiscoveryStore = defineStore("discovery", () => {
  const devices = ref<DiscoveredDevice[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const selectedEndpoints = ref<string[]>([]);
  const deploymentResults = ref<AgentDeploymentResult[]>([]);
  const loading = ref(false);
  const deploying = ref(false);
  const actingEndpoint = ref("");
  const errorMessage = ref("");
  const centerBaseURL = ref("");
  const resetConfig = ref(false);
  const runAgent = ref(true);
  const latestActionResult = ref<AgentActionResult | null>(null);
  const latestPairResult = ref<PairDeviceResult | null>(null);
  const savingSettings = ref(false);
  const pairing = ref(false);

  const selectableDevices = computed(() => devices.value.filter((item) => item.connectable));
  const connectedDevices = computed(() => devices.value.filter(isVisibleConnectedDevice));

  async function loadDiscoverySettings() {
    try {
      const settings = await fetchDiscoverySettings();
      centerBaseURL.value = settings.center_base_url || centerBaseURL.value;
    } catch (_error) {
      // 配置加载失败不阻断设备发现页面主体功能。
    }
  }

  async function persistDiscoverySettings() {
    savingSettings.value = true;
    try {
      await saveDiscoverySettings({
        center_base_url: centerBaseURL.value
      });
    } finally {
      savingSettings.value = false;
    }
  }

  async function loadDevices() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchDiscoveredDevices({
        page: page.value,
        page_size: pageSize.value
      });
      const filteredItems = result.items.filter(isVisibleConnectedDevice);
      devices.value = filteredItems;
      total.value = filteredItems.length;
      page.value = result.page;
      pageSize.value = result.page_size;
      const allowed = new Set(devices.value.map((item) => item.adb_endpoint));
      selectedEndpoints.value = selectedEndpoints.value.filter((item) => allowed.has(item));
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_discovery_devices_failed";
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

  function toggleSelection(endpoint: string, checked: boolean) {
    if (checked) {
      if (!selectedEndpoints.value.includes(endpoint)) {
        selectedEndpoints.value = selectedEndpoints.value.concat(endpoint);
      }
      return;
    }

    selectedEndpoints.value = selectedEndpoints.value.filter((item) => item !== endpoint);
  }

  function toggleSelectAll(checked: boolean) {
    if (checked) {
      selectedEndpoints.value = selectableDevices.value.map((item) => item.adb_endpoint);
      return;
    }
    selectedEndpoints.value = [];
  }

  async function submitDeployment() {
    deploying.value = true;
    errorMessage.value = "";
    deploymentResults.value = [];
    try {
      deploymentResults.value = await deployAgents({
        adb_endpoints: selectedEndpoints.value,
        center_base_url: centerBaseURL.value,
        reset_config: resetConfig.value,
        run_agent: runAgent.value
      });
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "deploy_agents_failed";
      throw error;
    } finally {
      deploying.value = false;
    }
  }

  async function submitSingleDeployment(adbEndpoint: string) {
    deploying.value = true;
    errorMessage.value = "";
    deploymentResults.value = [];
    try {
      deploymentResults.value = await deployAgents({
        adb_endpoints: [adbEndpoint],
        center_base_url: centerBaseURL.value,
        reset_config: resetConfig.value,
        run_agent: false
      });
      return deploymentResults.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "deploy_single_agent_failed";
      throw error;
    } finally {
      deploying.value = false;
    }
  }

  async function submitAgentAction(adbEndpoint: string, action: "start" | "stop" | "disconnect") {
    actingEndpoint.value = adbEndpoint;
    errorMessage.value = "";
    latestActionResult.value = null;
    try {
      latestActionResult.value = await controlAgent({
        adb_endpoint: adbEndpoint,
        action
      });
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "control_agent_failed";
      throw error;
    } finally {
      actingEndpoint.value = "";
    }
  }

  async function submitPairDevice(host: string, port: string, pairCode: string) {
    pairing.value = true;
    errorMessage.value = "";
    latestPairResult.value = null;
    try {
      latestPairResult.value = await pairDevice({
        host,
        port,
        pair_code: pairCode
      });
      await loadDevices();
      return latestPairResult.value;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "pair_device_failed";
      throw error;
    } finally {
      pairing.value = false;
    }
  }

  return {
    devices,
    total,
    page,
    pageSize,
    selectedEndpoints,
    deploymentResults,
    loading,
    deploying,
    actingEndpoint,
    errorMessage,
    centerBaseURL,
    resetConfig,
    runAgent,
    latestActionResult,
    latestPairResult,
    savingSettings,
    pairing,
    selectableDevices,
    connectedDevices,
    loadDevices,
    changePage,
    changePageSize,
    loadDiscoverySettings,
    persistDiscoverySettings,
    toggleSelection,
    toggleSelectAll,
    submitDeployment,
    submitSingleDeployment,
    submitAgentAction,
    submitPairDevice
  };
});

function isVisibleConnectedDevice(item: DiscoveredDevice) {
  return item.source === "adb_devices" || item.source === "merged";
}

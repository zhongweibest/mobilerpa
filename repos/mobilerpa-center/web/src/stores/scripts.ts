import { defineStore } from "pinia";
import { computed, ref } from "vue";

import { createScriptName, deleteScript, deleteScriptVersion, deployScript, deployScriptToAll, fetchScriptNames, fetchScripts, fetchScriptVersion, uploadScript } from "../api/scripts";
import type { ScriptManifestRecord, ScriptNameRecord, ScriptRecord, ScriptVersionRecord, UploadScriptRequest } from "../types/script";

export const useScriptsStore = defineStore("scripts", () => {
  const scripts = ref<ScriptRecord[]>([]);
  const scriptNames = ref<ScriptNameRecord[]>([]);
  const total = ref(0);
  const page = ref(1);
  const pageSize = ref(10);
  const selectedScriptName = ref("");
  const selectedScriptVersion = ref("");
  const selectedManifest = ref<ScriptManifestRecord | null>(null);
  const loading = ref(false);
  const uploading = ref(false);
  const deploying = ref(false);
  const errorMessage = ref("");

  const flattenedVersions = computed<ScriptVersionRecord[]>(() =>
    scripts.value.flatMap((scriptItem) => scriptItem.versions)
  );

  async function loadScripts() {
    loading.value = true;
    errorMessage.value = "";
    try {
      const result = await fetchScripts({
        page: page.value,
        page_size: pageSize.value
      });
      scripts.value = result.items;
      total.value = result.total;
      page.value = result.page;
      pageSize.value = result.page_size;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_scripts_failed";
    } finally {
      loading.value = false;
    }
  }

  async function loadScriptNames() {
    errorMessage.value = "";
    try {
      scriptNames.value = await fetchScriptNames();
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_script_names_failed";
      throw error;
    }
  }

  async function submitScriptName(scriptName: string) {
    errorMessage.value = "";
    try {
      const result = await createScriptName({ script_name: scriptName });
      await loadScriptNames();
      return result;
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "create_script_name_failed";
      throw error;
    }
  }

  async function changePage(nextPage: number) {
    page.value = nextPage;
    await loadScripts();
  }

  async function changePageSize(nextPageSize: number) {
    pageSize.value = nextPageSize;
    page.value = 1;
    await loadScripts();
  }

  async function loadScriptVersion(scriptName: string, scriptVersion: string) {
    selectedScriptName.value = scriptName;
    selectedScriptVersion.value = scriptVersion;
    errorMessage.value = "";
    try {
      selectedManifest.value = await fetchScriptVersion(scriptName, scriptVersion);
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "load_script_version_failed";
      throw error;
    }
  }

  async function submitScriptUpload(payload: UploadScriptRequest) {
    uploading.value = true;
    errorMessage.value = "";
    try {
      await uploadScript(payload);
      await loadScripts();
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "upload_script_failed";
      throw error;
    } finally {
      uploading.value = false;
    }
  }

  async function triggerScriptDeploy(deviceID: string, scriptName: string, scriptVersion: string, force: boolean) {
    deploying.value = true;
    errorMessage.value = "";
    try {
      await deployScript({
        device_id: deviceID,
        script_name: scriptName,
        script_version: scriptVersion,
        force
      });
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "deploy_script_failed";
      throw error;
    } finally {
      deploying.value = false;
    }
  }

  async function triggerScriptDeployToAll(scriptName: string, scriptVersion: string, force: boolean) {
    deploying.value = true;
    errorMessage.value = "";
    try {
      await deployScriptToAll({
        script_name: scriptName,
        script_version: scriptVersion,
        force
      });
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "deploy_script_all_failed";
      throw error;
    } finally {
      deploying.value = false;
    }
  }

  async function removeScriptVersion(scriptName: string, scriptVersion: string) {
    errorMessage.value = "";
    try {
      await deleteScriptVersion(scriptName, scriptVersion);
      await loadScripts();
      if (scripts.value.length === 0 && page.value > 1) {
        page.value -= 1;
        await loadScripts();
      }
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_script_version_failed";
      throw error;
    }
  }

  async function removeScript(scriptName: string) {
    errorMessage.value = "";
    try {
      await deleteScript(scriptName);
      await loadScripts();
      if (scripts.value.length === 0 && page.value > 1) {
        page.value -= 1;
        await loadScripts();
      }
    } catch (error) {
      errorMessage.value = error instanceof Error ? error.message : "delete_script_failed";
      throw error;
    }
  }

  return {
    scripts,
    scriptNames,
    total,
    page,
    pageSize,
    flattenedVersions,
    selectedScriptName,
    selectedScriptVersion,
    selectedManifest,
    loading,
    uploading,
    deploying,
    errorMessage,
    loadScripts,
    loadScriptNames,
    submitScriptName,
    changePage,
    changePageSize,
    loadScriptVersion,
    submitScriptUpload,
    triggerScriptDeploy,
    triggerScriptDeployToAll,
    removeScriptVersion,
    removeScript
  };
});

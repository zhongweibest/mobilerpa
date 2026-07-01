export interface ScriptFileRecord {
  relative_path: string;
  checksum_sha256: string;
}

export interface ScriptManifestRecord {
  script_name: string;
  script_version: string;
  entry_file: string;
  checksum_sha256: string;
  download_url: string;
  files: ScriptFileRecord[];
  source_type: string;
  storage_type: string;
}

export interface ScriptVersionRecord {
  script_name: string;
  script_version: string;
  entry_file: string;
  source_type: string;
  storage_type: string;
  status: string;
  created_at: string;
  workflow_references: WorkflowReferenceRecord[];
}

export interface ScriptRecord {
  script_name: string;
  versions: ScriptVersionRecord[];
}

export interface ScriptNameRecord {
  script_name: string;
  created_at: string;
  updated_at: string;
}

export interface WorkflowReferenceRecord {
  workflow_def_id: string;
  workflow_name: string;
  node_id: string;
  node_name: string;
}

export interface DeployScriptRequest {
  device_id: string;
  script_name: string;
  script_version: string;
  force: boolean;
}

export interface DeployAllScriptsRequest {
  script_name: string;
  script_version: string;
  force: boolean;
}

export interface UploadScriptRequest {
  script_name: string;
  script_version: string;
  source_type: "zip";
  force: boolean;
  file: File;
}

export interface CreateScriptNameRequest {
  script_name: string;
}

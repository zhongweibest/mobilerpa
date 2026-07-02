export interface SoftwarePackageRecord {
  software_id: string;
  software_name: string;
  description: string;
  package_file_name: string;
  package_storage_path: string;
  package_size: number;
  created_at: string;
  updated_at: string;
}

export interface CreateSoftwareRequest {
  software_name: string;
  description: string;
  file: File;
}

export interface UpdateSoftwareRequest {
  software_id: string;
  software_name: string;
  description: string;
  file?: File | null;
}

// Types describing the shape of HorneroDB REST API responses.

export type HorneroDbRecord = {
  id: string;
  [field: string]: unknown;
};

export type HorneroDbApiResponse<T> = {
  success: boolean;
  data: T;
  meta?: Record<string, unknown>;
  error?: {
    code: string;
    message: string;
    path?: string;
  };
};

export type HorneroDbListResponse = HorneroDbApiResponse<HorneroDbRecord[]>;
export type HorneroDbRecordResponse = HorneroDbApiResponse<HorneroDbRecord>;
export type HorneroDbMessageResponse = HorneroDbApiResponse<{
  message: string;
}>;

export type HorneroDbWorkspace = {
  id: string;
  name: string;
  slug: string;
  owner_id?: string;
  settings?: Record<string, unknown>;
  created_at?: string;
};

export type HorneroDbTable = {
  id: string;
  workspace_id: string;
  name: string;
  slug: string;
  created_at?: string;
};

export type HorneroDbWorkspacesResponse = HorneroDbApiResponse<
  HorneroDbWorkspace[]
>;
export type HorneroDbTablesResponse = HorneroDbApiResponse<HorneroDbTable[]>;

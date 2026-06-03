import { createFetcherHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { searchRecords } from "../actions/searchRecords";
import { listTablesFetcher, listWorkspacesFetcher } from "../actions/fetchers";
import type {
  HorneroDbTable,
  HorneroDbTablesResponse,
  HorneroDbWorkspace,
  HorneroDbWorkspacesResponse,
} from "../types";

export const listWorkspacesFetcherHandler = createFetcherHandler(
  searchRecords,
  listWorkspacesFetcher.id,
  async ({ credentials: { baseUrl, apiKey } }) => {
    try {
      const data = await ky
        .get(`${baseUrl}/api/v1/workspaces`, {
          headers: { Authorization: `Bearer ${apiKey}` },
        })
        .json<HorneroDbWorkspacesResponse>();
      return {
        data: (data.data ?? []).map((w: HorneroDbWorkspace) => ({
          label: w.name,
          value: w.id,
        })),
      };
    } catch (error) {
      console.error("listWorkspaces fetcher failed", error);
      return { data: [], error: await parseUnknownError({ err: error }) };
    }
  },
);

export const listTablesFetcherHandler = createFetcherHandler(
  searchRecords,
  listTablesFetcher.id,
  async ({ credentials: { baseUrl, apiKey }, options }) => {
    const workspaceId = (options as { workspaceId?: string }).workspaceId;
    if (!workspaceId) return { data: [] };
    try {
      const data = await ky
        .get(
          `${baseUrl}/api/v1/workspaces/${workspaceId}/tables`,
          {
            headers: { Authorization: `Bearer ${apiKey}` },
          },
        )
        .json<HorneroDbTablesResponse>();
      return {
        data: (data.data ?? []).map((t: HorneroDbTable) => ({
          label: `${t.name} (${t.slug})`,
          value: t.slug,
        })),
      };
    } catch (error) {
      console.error("listTables fetcher failed", error);
      return { data: [], error: await parseUnknownError({ err: error }) };
    }
  },
);

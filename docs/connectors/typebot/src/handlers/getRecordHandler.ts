import { createActionHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { getRecord } from "../actions/getRecord";
import { applyResponseMapping } from "../helpers/applyResponseMapping";
import type { HorneroDbRecordResponse } from "../types";

export const getRecordHandler = createActionHandler(getRecord, {
  server: async ({
    credentials: { baseUrl, apiKey },
    options: { workspaceId, tableSlug, recordId, responseMapping },
    variables,
    logs,
  }) => {
    if (!apiKey) return logs.add("API key is required");
    if (!workspaceId) return logs.add("Workspace is required");
    if (!tableSlug) return logs.add("Table is required");
    if (!recordId) return logs.add("Record ID is required");

    try {
      const data = await ky
        .get(
          `${baseUrl}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`,
          {
            headers: {
              Authorization: `Bearer ${apiKey}`,
            },
          },
        )
        .json<HorneroDbRecordResponse>();

      applyResponseMapping({
        records: data.data,
        responseMapping,
        variables,
        logs,
      });
    } catch (error) {
      logs.add(
        await parseUnknownError({
          err: error,
          context: "While fetching HorneroDB record",
        }),
      );
    }
  },
});

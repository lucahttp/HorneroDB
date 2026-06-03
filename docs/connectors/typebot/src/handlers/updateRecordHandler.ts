import { createActionHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { updateRecord } from "../actions/updateRecord";
import { parseRecordUpdateBody } from "../helpers/parseRecordBody";
import type { HorneroDbMessageResponse } from "../types";

export const updateRecordHandler = createActionHandler(updateRecord, {
  server: async ({
    credentials: { baseUrl, apiKey },
    options: { workspaceId, tableSlug, recordId, updates },
    logs,
  }) => {
    if (!apiKey) return logs.add("API key is required");
    if (!workspaceId) return logs.add("Workspace is required");
    if (!tableSlug) return logs.add("Table is required");
    if (!recordId) return logs.add("Record ID is required");
    if (!updates || updates.length === 0)
      return logs.add("At least one update is required");

    try {
      await ky.put(
        `${baseUrl}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`,
        {
          headers: {
            Authorization: `Bearer ${apiKey}`,
          },
          json: parseRecordUpdateBody(updates),
        },
      );

      logs.add("Record updated");
    } catch (error) {
      logs.add(
        await parseUnknownError({
          err: error,
          context: "While updating HorneroDB record",
        }),
      );
    }
  },
});

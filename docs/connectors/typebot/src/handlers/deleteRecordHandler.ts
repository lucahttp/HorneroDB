import { createActionHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { deleteRecord } from "../actions/deleteRecord";

export const deleteRecordHandler = createActionHandler(deleteRecord, {
  server: async ({
    credentials: { baseUrl, apiKey },
    options: { workspaceId, tableSlug, recordId },
    logs,
  }) => {
    if (!apiKey) return logs.add("API key is required");
    if (!workspaceId) return logs.add("Workspace is required");
    if (!tableSlug) return logs.add("Table is required");
    if (!recordId) return logs.add("Record ID is required");

    try {
      await ky.delete(
        `${baseUrl}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`,
        {
          headers: {
            Authorization: `Bearer ${apiKey}`,
          },
        },
      );

      logs.add("Record deleted");
    } catch (error) {
      logs.add(
        await parseUnknownError({
          err: error,
          context: "While deleting HorneroDB record",
        }),
      );
    }
  },
});

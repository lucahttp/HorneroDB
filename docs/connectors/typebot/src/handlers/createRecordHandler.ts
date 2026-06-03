import { createActionHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { createRecord } from "../actions/createRecord";
import { applyResponseMapping } from "../helpers/applyResponseMapping";
import { parseRecordCreateBody } from "../helpers/parseRecordBody";
import type { HorneroDbRecordResponse } from "../types";

export const createRecordHandler = createActionHandler(createRecord, {
  server: async ({
    credentials: { baseUrl, apiKey },
    options: { workspaceId, tableSlug, fields, responseMapping },
    variables,
    logs,
  }) => {
    if (!apiKey) return logs.add("API key is required");
    if (!workspaceId) return logs.add("Workspace is required");
    if (!tableSlug) return logs.add("Table is required");
    if (!fields || fields.length === 0)
      return logs.add("At least one field is required");

    try {
      const data = await ky
        .post(
          `${baseUrl}/api/v1/workspaces/${workspaceId}/data/${tableSlug}`,
          {
            headers: {
              Authorization: `Bearer ${apiKey}`,
            },
            json: parseRecordCreateBody(fields),
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
          context: "While creating HorneroDB record",
        }),
      );
    }
  },
});

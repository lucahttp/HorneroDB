import { createActionHandler } from "@typebot.io/forge";
import { ky } from "@typebot.io/lib/ky";
import { parseUnknownError } from "@typebot.io/lib/parseUnknownError";
import { isDefined } from "@typebot.io/lib/utils";
import { searchRecords } from "../actions/searchRecords";
import { defaultLimit, maxLimit } from "../constants";
import { applyResponseMapping } from "../helpers/applyResponseMapping";
import { parseSearchParams } from "../helpers/parseSearchParams";
import type { HorneroDbListResponse } from "../types";

export const searchRecordsHandler = createActionHandler(searchRecords, {
  server: async ({
    credentials: { baseUrl, apiKey },
    options: {
      workspaceId,
      tableSlug,
      returnType,
      limit,
      offset,
      expand,
      responseMapping,
    },
    variables,
    logs,
  }) => {
    if (!apiKey) return logs.add("API key is required");
    if (!workspaceId) return logs.add("Workspace is required");
    if (!tableSlug) return logs.add("Table is required");

    try {
      const safeLimit = Math.min(
        Math.max(1, Number(limit ?? defaultLimit)),
        maxLimit,
      );

      const data = await ky
        .get(`${baseUrl}/api/v1/workspaces/${workspaceId}/data/${tableSlug}`, {
          headers: {
            Authorization: `Bearer ${apiKey}`,
          },
          searchParams: parseSearchParams({
            limit: safeLimit,
            offset: Math.max(0, Number(offset ?? 0)),
            expand,
          }),
        })
        .json<HorneroDbListResponse>();

      let list = data.data ?? [];

      if (returnType && returnType !== "All" && list.length > 0) {
        let pickIndex: number | undefined;
        if (returnType === "First") {
          pickIndex = 0;
        } else if (returnType === "Last") {
          pickIndex = list.length - 1;
        } else if (returnType === "Random") {
          pickIndex = Math.floor(Math.random() * list.length);
        }
        if (isDefined(pickIndex)) {
          list = [list[pickIndex]];
        }
      }

      if (list.length === 0) {
        logs.add("No records matched the search");
        return;
      }

      applyResponseMapping({
        records: list,
        responseMapping,
        variables,
        logs,
      });
    } catch (error) {
      logs.add(
        await parseUnknownError({
          err: error,
          context: "While searching HorneroDB records",
        }),
      );
    }
  },
});

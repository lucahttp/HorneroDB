import { createAction, option } from "@typebot.io/forge";
import { isDefined } from "@typebot.io/lib/utils";
import { auth } from "../auth";
import { fetchers, listTablesFetcher, listWorkspacesFetcher } from "./fetchers";

export const getRecord = createAction({
  auth,
  name: "Get Record",
  fetchers,
  options: option.object({
    workspaceId: option.string.meta({
      layout: {
        label: "Workspace",
        isRequired: true,
        fetcher: listWorkspacesFetcher.id,
      },
    }),
    tableSlug: option.string.meta({
      layout: {
        label: "Table",
        isRequired: true,
        fetcher: listTablesFetcher.id,
      },
    }),
    recordId: option.string.meta({
      layout: {
        label: "Record ID",
        isRequired: true,
        helperText: "UUID of the record to fetch.",
      },
    }),
    responseMapping: option
      .array(
        option.object({
          fieldName: option.string.meta({
            layout: {
              label: "Field name to extract from the response",
            },
          }),
          variableId: option.string.meta({
            layout: {
              inputType: "variableDropdown",
            },
          }),
        }),
      )
      .meta({
        layout: {
          accordion: "Response Mapping",
        },
      }),
  }),
  getSetVariableIds: ({ responseMapping }) =>
    responseMapping?.map((r) => r.variableId).filter(isDefined) ?? [],
});

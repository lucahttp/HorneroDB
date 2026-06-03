import { createAction, option } from "@typebot.io/forge";
import { isDefined } from "@typebot.io/lib/utils";
import { auth } from "../auth";
import { fetchers, listTablesFetcher, listWorkspacesFetcher } from "./fetchers";
import { defaultLimit, maxLimit } from "../constants";

export const searchRecords = createAction({
  auth,
  name: "Search Records",
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
    returnType: option
      .enum(["All", "First", "Last", "Random"])
      .meta({
        layout: {
          accordion: "Filter",
          defaultValue: "All",
        },
      }),
    limit: option.number.meta({
      layout: {
        accordion: "Filter",
        label: "Limit",
        defaultValue: defaultLimit,
        moreInfoTooltip: `Maximum number of rows to fetch. Capped at ${maxLimit} by the API.`,
      },
    }),
    offset: option.number.meta({
      layout: {
        accordion: "Filter",
        label: "Offset",
        defaultValue: 0,
      },
    }),
    expand: option.string.meta({
      layout: {
        accordion: "Filter",
        label: "Expand relations (optional)",
        placeholder: "relation_a,relation_b",
        helperText:
          "Comma-separated list of relation column names to expand on the response.",
        withVariableButton: true,
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

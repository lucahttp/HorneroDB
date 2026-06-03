import { createAction, option } from "@typebot.io/forge";
import { isDefined } from "@typebot.io/lib/utils";
import { auth } from "../auth";
import { fetchers, listTablesFetcher, listWorkspacesFetcher } from "./fetchers";

export const createRecord = createAction({
  auth,
  name: "Create Record",
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
        helperText:
          "The slug of the target table (lower-case, underscores only).",
      },
    }),
    fields: option
      .array(
        option.object({
          key: option.string.meta({
            layout: {
              label: "Field",
              isRequired: true,
              helperText: "Column name in the target table.",
            },
          }),
          value: option.string.meta({
            layout: {
              label: "Value",
              isRequired: true,
            },
          }),
        }),
      )
      .meta({
        layout: {
          itemLabel: "field",
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

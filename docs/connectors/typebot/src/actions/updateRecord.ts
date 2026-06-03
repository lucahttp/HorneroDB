import { createAction, option } from "@typebot.io/forge";
import { auth } from "../auth";
import { fetchers, listTablesFetcher, listWorkspacesFetcher } from "./fetchers";

export const updateRecord = createAction({
  auth,
  name: "Update Record",
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
        helperText: "UUID of the record to update.",
      },
    }),
    updates: option
      .array(
        option.object({
          key: option.string.meta({
            layout: {
              label: "Field",
              isRequired: true,
              helperText: "Column name to update.",
            },
          }),
          value: option.string.meta({
            layout: {
              label: "Value",
              helperText:
                "Use an empty string to clear the value (the field is sent as null).",
            },
          }),
        }),
      )
      .meta({
        layout: {
          accordion: "Updates",
          itemLabel: "update",
        },
      }),
  }),
});

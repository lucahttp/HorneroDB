import { createAction, option } from "@typebot.io/forge";
import { auth } from "../auth";
import { fetchers, listTablesFetcher, listWorkspacesFetcher } from "./fetchers";

export const deleteRecord = createAction({
  auth,
  name: "Delete Record",
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
        helperText: "UUID of the record to delete.",
      },
    }),
  }),
});

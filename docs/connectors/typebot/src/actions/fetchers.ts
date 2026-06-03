// Shared fetchers used by every action to populate workspace / table dropdowns.
// They are wired to handlers in src/handlers/fetchersHandler.ts.

export const listWorkspacesFetcher = {
  id: "listWorkspaces",
} as const;

export const listTablesFetcher = {
  id: "listTables",
  dependsOn: ["workspaceId"],
} as const;

export const fetchers = [listWorkspacesFetcher, listTablesFetcher] as const;

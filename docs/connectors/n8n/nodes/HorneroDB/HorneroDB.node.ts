import {
  IExecuteFunctions,
  ILoadOptionsFunctions,
  INodeExecutionData,
  INodeListSearchItems,
  INodeListSearchResult,
  INodeType,
  INodeTypeDescription,
  NodeOperationError,
} from "n8n-workflow";

export class HorneroDB implements INodeType {
  description: INodeTypeDescription = {
    displayName: "HorneroDB",
    name: "horneroDb",
    icon: "fa:database",
    group: ["transform"],
    version: 1,
    subtitle: '={{$parameter["operation"] + ": " + $parameter["resource"]}}',
    description: "Interact with HorneroDB API",
    defaults: {
      name: "HorneroDB",
    },
    inputs: ["main"],
    outputs: ["main"],
    credentials: [
      {
        name: "horneroDbApi",
        required: true,
        displayOptions: {
          show: {
            authentication: ["apiKey"],
          },
        },
      },
      {
        name: "horneroDbOAuth2Api",
        required: true,
        displayOptions: {
          show: {
            authentication: ["oAuth2"],
          },
        },
      },
    ],
    properties: [
      {
        displayName: "Authentication",
        name: "authentication",
        type: "options",
        options: [
          { name: "API Key", value: "apiKey" },
          { name: "OAuth2", value: "oAuth2" },
        ],
        default: "apiKey",
      },
      // ─── Resource ────────────────────────────────────────────────────────────
      {
        displayName: "Resource",
        name: "resource",
        type: "options",
        noDataExpression: true,
        options: [
          { name: "Workspace", value: "workspace" },
          { name: "Table", value: "table" },
          { name: "Record", value: "record" },
          { name: "Webhook", value: "webhook" },
        ],
        default: "record",
      },

      // ─── Workspace ───────────────────────────────────────────────────────────
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: { show: { resource: ["workspace"] } },
        options: [
          {
            name: "Get Many",
            value: "getAll",
            description: "List all accessible workspaces",
            action: "List all workspaces",
          },
        ],
        default: "getAll",
      },

      // ─── Table ───────────────────────────────────────────────────────────────
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: { show: { resource: ["table"] } },
        options: [
          {
            name: "Get Many",
            value: "getAll",
            description: "List all tables within a workspace",
            action: "List all tables",
          },
        ],
        default: "getAll",
      },
      {
        displayName: "Workspace",
        name: "workspaceId",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
        displayOptions: { show: { resource: ["table"] } },
        modes: [
          {
            displayName: "From List",
            name: "list",
            type: "list",
            placeholder: "Select workspace…",
            typeOptions: {
              searchListMethod: "listWorkspaces",
              searchable: false,
            },
          },
          {
            displayName: "By ID",
            name: "id",
            type: "string",
            placeholder: "e.g. 550e8400-e29b-41d4-a716-446655440000",
            validation: [
              {
                type: "regex",
                properties: {
                  regex:
                    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
                  errorMessage: "Must be a valid UUID",
                },
              },
            ],
          },
        ],
        description: "The workspace to list tables from",
      },

      // ─── Record ───────────────────────────────────────────────────────────────
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: { show: { resource: ["record"] } },
        options: [
          {
            name: "Create",
            value: "create",
            description: "Create a record",
            action: "Create a record",
          },
          {
            name: "Delete",
            value: "delete",
            description: "Delete a record",
            action: "Delete a record",
          },
          {
            name: "Get",
            value: "get",
            description: "Get a single record by ID",
            action: "Get a record",
          },
          {
            name: "Get Many",
            value: "getAll",
            description: "List records in a table",
            action: "Get many records",
          },
          {
            name: "Update",
            value: "update",
            description: "Update an existing record",
            action: "Update a record",
          },
        ],
        default: "create",
      },
      {
        displayName: "Workspace",
        name: "workspaceId",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
        displayOptions: { show: { resource: ["record"] } },
        modes: [
          {
            displayName: "From List",
            name: "list",
            type: "list",
            placeholder: "Select workspace…",
            typeOptions: {
              searchListMethod: "listWorkspaces",
              searchable: false,
            },
          },
          {
            displayName: "By ID",
            name: "id",
            type: "string",
            placeholder: "e.g. 550e8400-e29b-41d4-a716-446655440000",
            validation: [
              {
                type: "regex",
                properties: {
                  regex:
                    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
                  errorMessage: "Must be a valid UUID",
                },
              },
            ],
          },
        ],
        description: "The workspace containing the table",
      },
      {
        displayName: "Table Slug",
        name: "tableSlug",
        type: "string",
        required: true,
        displayOptions: { show: { resource: ["record"] } },
        default: "",
        description: "The slug of the table (e.g. contacts)",
      },
      {
        displayName: "Record ID",
        name: "recordId",
        type: "string",
        required: true,
        displayOptions: {
          show: { resource: ["record"], operation: ["get", "delete", "update"] },
        },
        default: "",
        description: "UUID of the record",
      },
      {
        displayName: "JSON Parameters",
        name: "jsonParameters",
        type: "boolean",
        default: true,
        displayOptions: {
          show: { resource: ["record"], operation: ["create", "update"] },
        },
      },
      {
        displayName: "Body (JSON)",
        name: "bodyJson",
        type: "json",
        required: true,
        displayOptions: {
          show: {
            resource: ["record"],
            operation: ["create", "update"],
            jsonParameters: [true],
          },
        },
        default: "{}",
        description: "The JSON body to send",
      },
      {
        displayName: "Expand Relations",
        name: "expand",
        type: "string",
        displayOptions: {
          show: { resource: ["record"], operation: ["get", "getAll"] },
        },
        default: "",
        description:
          "Comma-separated list of relation columns to expand into human-readable labels",
      },
      // Pagination for getAll records
      {
        displayName: "Page",
        name: "page",
        type: "number",
        displayOptions: {
          show: { resource: ["record"], operation: ["getAll"] },
        },
        default: 1,
        description: "Page number (1-indexed)",
      },
      {
        displayName: "Per Page",
        name: "perPage",
        type: "number",
        displayOptions: {
          show: { resource: ["record"], operation: ["getAll"] },
        },
        default: 50,
        description: "Number of records per page",
      },

      // ─── Webhook management ───────────────────────────────────────────────────
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: { show: { resource: ["webhook"] } },
        options: [
          {
            name: "Create",
            value: "create",
            description: "Create a webhook subscription",
            action: "Create a webhook",
          },
          {
            name: "Delete",
            value: "delete",
            description: "Delete a webhook subscription",
            action: "Delete a webhook",
          },
          {
            name: "Get All",
            value: "getAll",
            description: "List all webhooks in a workspace",
            action: "Get all webhooks",
          },
        ],
        default: "getAll",
      },
      {
        displayName: "Workspace",
        name: "workspaceId",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
        displayOptions: { show: { resource: ["webhook"] } },
        modes: [
          {
            displayName: "From List",
            name: "list",
            type: "list",
            placeholder: "Select workspace…",
            typeOptions: {
              searchListMethod: "listWorkspaces",
              searchable: false,
            },
          },
          {
            displayName: "By ID",
            name: "id",
            type: "string",
            placeholder: "e.g. 550e8400-e29b-41d4-a716-446655440000",
            validation: [
              {
                type: "regex",
                properties: {
                  regex:
                    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
                  errorMessage: "Must be a valid UUID",
                },
              },
            ],
          },
        ],
        description: "The workspace to manage webhooks in",
      },
      {
        displayName: "Table (Resource UUID)",
        name: "webhookResource",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
        displayOptions: {
          show: { resource: ["webhook"], operation: ["create"] },
        },
        modes: [
          {
            displayName: "From List",
            name: "list",
            type: "list",
            placeholder: "Select table…",
            typeOptions: {
              searchListMethod: "listTables",
              searchable: false,
            },
          },
          {
            displayName: "By ID",
            name: "id",
            type: "string",
            placeholder: "e.g. 550e8400-e29b-41d4-a716-446655440000",
            validation: [
              {
                type: "regex",
                properties: {
                  regex:
                    "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
                  errorMessage: "Must be a valid UUID",
                },
              },
            ],
          },
        ],
        description: "The table to monitor for changes",
      },
      {
        displayName: "Events",
        name: "events",
        type: "multiOptions",
        required: true,
        displayOptions: {
          show: { resource: ["webhook"], operation: ["create"] },
        },
        options: [
          { name: "Record Created", value: "created" },
          { name: "Record Updated", value: "updated" },
          { name: "Record Deleted", value: "deleted" },
        ],
        default: ["created", "updated", "deleted"],
        description: "Which change events to subscribe to",
      },
      {
        displayName: "Notification URL",
        name: "notificationUrl",
        type: "string",
        required: true,
        displayOptions: {
          show: { resource: ["webhook"], operation: ["create"] },
        },
        default: "",
        description: "The URL HorneroDB will POST events to",
      },
      {
        displayName: "Webhook ID",
        name: "webhookId",
        type: "string",
        required: true,
        displayOptions: {
          show: { resource: ["webhook"], operation: ["delete"] },
        },
        default: "",
        description: "UUID of the webhook subscription to delete",
      },
    ],
  };

  // ─── Dynamic option loaders ─────────────────────────────────────────────────

  methods = {
    listSearch: {
      async listWorkspaces(
        this: ILoadOptionsFunctions,
      ): Promise<INodeListSearchResult> {
        const { baseURL, credName } = await getBaseAndCredName(this);
        const response = await this.helpers.requestWithAuthentication.call(
          this,
          credName,
          { method: "GET", url: `${baseURL}/api/v1/workspaces`, json: true },
        );
        const workspaces: Array<{ id: string; name: string }> =
          response.data ?? response;
        const results: INodeListSearchItems[] = workspaces.map((w) => ({
          name: w.name,
          value: w.id,
        }));
        return { results };
      },

      async listTables(
        this: ILoadOptionsFunctions,
      ): Promise<INodeListSearchResult> {
        const { baseURL, credName } = await getBaseAndCredName(this);
        const workspaceId = (
          this.getNodeParameter("workspaceId") as { value: string }
        ).value;
        const response = await this.helpers.requestWithAuthentication.call(
          this,
          credName,
          {
            method: "GET",
            url: `${baseURL}/api/v1/workspaces/${workspaceId}/tables`,
            json: true,
          },
        );
        const tables: Array<{ id: string; name: string }> =
          response.data ?? response;
        const results: INodeListSearchItems[] = tables.map((t) => ({
          name: t.name,
          value: t.id,
        }));
        return { results };
      },
    },
  };

  // ─── Execute ────────────────────────────────────────────────────────────────

  async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
    const items = this.getInputData();
    const returnData: INodeExecutionData[] = [];
    const resource = this.getNodeParameter("resource", 0) as string;
    const operation = this.getNodeParameter("operation", 0) as string;

    const authMethod = this.getNodeParameter("authentication", 0) as string;
    const credName =
      authMethod === "apiKey" ? "horneroDbApi" : "horneroDbOAuth2Api";
    const credentials = await this.getCredentials(credName);
    const baseURL = credentials?.host as string;

    const req = (opts: object) =>
      this.helpers.requestWithAuthentication.call(this, credName, opts);

    for (let i = 0; i < items.length; i++) {
      try {
        // ── Workspace ──────────────────────────────────────────────────────────
        if (resource === "workspace" && operation === "getAll") {
          const res = await req({
            method: "GET",
            url: `${baseURL}/api/v1/workspaces`,
            json: true,
          });
          const data = res.data ?? res;
          returnData.push(
            ...this.helpers.constructExecutionMetaData(
              this.helpers.returnJsonArray(data),
              { itemData: { item: i } },
            ),
          );
        }

        // ── Table ──────────────────────────────────────────────────────────────
        if (resource === "table" && operation === "getAll") {
          const workspaceId = getResourceLocatorValue(
            this.getNodeParameter("workspaceId", i),
          );
          const res = await req({
            method: "GET",
            url: `${baseURL}/api/v1/workspaces/${workspaceId}/tables`,
            json: true,
          });
          const data = res.data ?? res;
          returnData.push(
            ...this.helpers.constructExecutionMetaData(
              this.helpers.returnJsonArray(data),
              { itemData: { item: i } },
            ),
          );
        }

        // ── Record ─────────────────────────────────────────────────────────────
        if (resource === "record") {
          const workspaceId = getResourceLocatorValue(
            this.getNodeParameter("workspaceId", i),
          );
          const tableSlug = this.getNodeParameter("tableSlug", i) as string;
          const base = `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}`;

          if (operation === "get") {
            const recordId = this.getNodeParameter("recordId", i) as string;
            const expand = this.getNodeParameter("expand", i) as string;
            let url = `${base}/${recordId}`;
            if (expand) url += `?expand=${expand}`;
            const res = await req({ method: "GET", url, json: true });
            returnData.push({ json: res.data ?? res });
          }

          if (operation === "getAll") {
            const expand = this.getNodeParameter("expand", i) as string;
            const page = this.getNodeParameter("page", i) as number;
            const perPage = this.getNodeParameter("perPage", i) as number;
            const params = new URLSearchParams();
            if (expand) params.set("expand", expand);
            if (page > 1) params.set("page", String(page));
            if (perPage !== 50) params.set("per_page", String(perPage));
            const qs = params.toString();
            const url = qs ? `${base}?${qs}` : base;
            const res = await req({ method: "GET", url, json: true });
            const data = res.data ?? res;
            returnData.push(
              ...this.helpers.constructExecutionMetaData(
                this.helpers.returnJsonArray(data),
                { itemData: { item: i } },
              ),
            );
          }

          if (operation === "create") {
            const bodyJson = this.getNodeParameter("bodyJson", i) as string;
            const body =
              typeof bodyJson === "string" ? JSON.parse(bodyJson) : bodyJson;
            const res = await req({
              method: "POST",
              url: base,
              body,
              json: true,
            });
            returnData.push({ json: res.data ?? res });
          }

          if (operation === "update") {
            const recordId = this.getNodeParameter("recordId", i) as string;
            const bodyJson = this.getNodeParameter("bodyJson", i) as string;
            const body =
              typeof bodyJson === "string" ? JSON.parse(bodyJson) : bodyJson;
            const res = await req({
              method: "PUT",
              url: `${base}/${recordId}`,
              body,
              json: true,
            });
            returnData.push({ json: res.data ?? res });
          }

          if (operation === "delete") {
            const recordId = this.getNodeParameter("recordId", i) as string;
            const res = await req({
              method: "DELETE",
              url: `${base}/${recordId}`,
              json: true,
            });
            returnData.push({ json: res.data ?? res });
          }
        }

        // ── Webhook management ─────────────────────────────────────────────────
        if (resource === "webhook") {
          const workspaceId = getResourceLocatorValue(
            this.getNodeParameter("workspaceId", i),
          );
          const base = `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks`;

          if (operation === "getAll") {
            const res = await req({ method: "GET", url: base, json: true });
            returnData.push(
              ...this.helpers.constructExecutionMetaData(
                this.helpers.returnJsonArray(res.data ?? res),
                { itemData: { item: i } },
              ),
            );
          }

          if (operation === "create") {
            const webhookResource = getResourceLocatorValue(
              this.getNodeParameter("webhookResource", i),
            );
            const notificationUrl = this.getNodeParameter(
              "notificationUrl",
              i,
            ) as string;
            const events = this.getNodeParameter("events", i) as string[];
            const body = {
              resource: webhookResource,
              notification_url: notificationUrl,
              change_type: events.join(","),
            };
            const res = await req({
              method: "POST",
              url: base,
              body,
              json: true,
            });
            returnData.push({ json: res.data ?? res });
          }

          if (operation === "delete") {
            const webhookId = this.getNodeParameter("webhookId", i) as string;
            const res = await req({
              method: "DELETE",
              url: `${base}/${webhookId}`,
              json: true,
            });
            returnData.push({ json: res.data ?? res });
          }
        }
      } catch (error) {
        if (this.continueOnFail()) {
          returnData.push({
            json: this.getInputData()[i].json,
            error: new NodeOperationError(this.getNode(), error as Error, {
              itemIndex: i,
            }),
          });
          continue;
        }
        throw new NodeOperationError(this.getNode(), error as Error, {
          itemIndex: i,
        });
      }
    }

    return [returnData];
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

/** Extracts the string value from a resourceLocator or plain string param. */
function getResourceLocatorValue(
  param: unknown,
): string {
  if (typeof param === "object" && param !== null && "value" in param) {
    return (param as { value: string }).value;
  }
  return param as string;
}

/** Returns baseURL and credential name for loadOption methods. */
async function getBaseAndCredName(ctx: ILoadOptionsFunctions): Promise<{
  baseURL: string;
  credName: string;
}> {
  const authMethod = ctx.getNodeParameter("authentication") as string;
  const credName =
    authMethod === "apiKey" ? "horneroDbApi" : "horneroDbOAuth2Api";
  const credentials = await ctx.getCredentials(credName);
  return { baseURL: credentials?.host as string, credName };
}

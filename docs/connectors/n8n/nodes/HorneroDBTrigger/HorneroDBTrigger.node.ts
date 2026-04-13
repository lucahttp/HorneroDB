import {
  IHookFunctions,
  ILoadOptionsFunctions,
  INodeListSearchItems,
  INodeListSearchResult,
  INodeType,
  INodeTypeDescription,
  IWebhookFunctions,
  IWebhookResponseData,
} from "n8n-workflow";

export class HorneroDBTrigger implements INodeType {
  description: INodeTypeDescription = {
    displayName: "HorneroDB Trigger",
    name: "horneroDbTrigger",
    icon: "fa:database",
    group: ["trigger"],
    version: 1,
    description:
      'Starts the workflow when HorneroDB data changes. Note: Requires "webhooks: manage" system permission if using API Keys.',
    defaults: { name: "HorneroDB Trigger" },
    inputs: [],
    outputs: ["main"],
    credentials: [
      {
        name: "horneroDbApi",
        required: true,
        displayOptions: { show: { authentication: ["apiKey"] } },
      },
      {
        name: "horneroDbOAuth2Api",
        required: true,
        displayOptions: { show: { authentication: ["oAuth2"] } },
      },
    ],
    webhooks: [
      {
        name: "default",
        httpMethod: "POST",
        responseMode: "onReceived",
        path: "webhook",
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
      // Workspace picker — mirrors Power Automate dynamic values on workspace_id
      {
        displayName: "Workspace",
        name: "workspaceId",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
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
        description: "The workspace containing the table to monitor",
      },
      // Table picker — mirrors Power Automate dynamic values on resource (table ID)
      {
        displayName: "Table",
        name: "tableId",
        type: "resourceLocator",
        default: { mode: "id", value: "" },
        required: true,
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
        description: "The table to listen for changes on",
      },
      // Events — mirrors Power Automate change_type field
      {
        displayName: "Events",
        name: "events",
        type: "multiOptions",
        required: true,
        default: ["created", "updated", "deleted"],
        options: [
          { name: "Record Created", value: "created" },
          { name: "Record Updated", value: "updated" },
          { name: "Record Deleted", value: "deleted" },
        ],
        description: "Which change types to subscribe to",
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
        const workspaceId = getResourceLocatorValue(
          this.getNodeParameter("workspaceId"),
        );
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

  // ─── Webhook lifecycle ───────────────────────────────────────────────────────

  webhookMethods = {
    default: {
      async checkExists(this: IHookFunctions): Promise<boolean> {
        const webhookData = this.getWorkflowStaticData("node");
        if (!webhookData.webhookId) return false;

        try {
          const { baseURL, credName } = await getBaseAndCredName(this);
          const workspaceId = getResourceLocatorValue(
            this.getNodeParameter("workspaceId"),
          );
          const response = await this.helpers.requestWithAuthentication.call(
            this,
            credName,
            {
              method: "GET",
              url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks/${webhookData.webhookId}`,
              json: true,
            },
          );
          return response.id === webhookData.webhookId;
        } catch (error: any) {
          if (error.statusCode === 404) return false;
          throw error;
        }
      },

      async create(this: IHookFunctions): Promise<boolean> {
        const webhookUrl = this.getNodeWebhookUrl("default");
        if (!webhookUrl) return false;

        const webhookData = this.getWorkflowStaticData("node");
        const { baseURL, credName } = await getBaseAndCredName(this);

        const workspaceId = getResourceLocatorValue(
          this.getNodeParameter("workspaceId"),
        );
        const tableId = getResourceLocatorValue(
          this.getNodeParameter("tableId"),
        );
        const events = this.getNodeParameter("events") as string[];

        const body = {
          resource: tableId,
          change_type: events.join(","),
          notification_url: webhookUrl,
          // client_state helps identify this subscription as created by n8n
          client_state: `n8n-${this.getWorkflow().id}`,
        };

        const response = await this.helpers.requestWithAuthentication.call(
          this,
          credName,
          {
            method: "POST",
            url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks`,
            body,
            json: true,
          },
        );

        webhookData.webhookId = response.id;
        return true;
      },

      async delete(this: IHookFunctions): Promise<boolean> {
        const webhookData = this.getWorkflowStaticData("node");
        if (!webhookData.webhookId) return true;

        const { baseURL, credName } = await getBaseAndCredName(this);
        const workspaceId = getResourceLocatorValue(
          this.getNodeParameter("workspaceId"),
        );

        try {
          await this.helpers.requestWithAuthentication.call(this, credName, {
            method: "DELETE",
            url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks/${webhookData.webhookId}`,
            json: true,
          });
        } catch {
          return false;
        }

        delete webhookData.webhookId;
        return true;
      },
    },
  };

  // ─── Incoming webhook handler ─────────────────────────────────────────────

  async webhook(this: IWebhookFunctions): Promise<IWebhookResponseData> {
    const body = this.getRequestObject().body;

    // Normalise both MS Graph-style { value: [...] } wrapper and bare payloads
    const items = Array.isArray(body?.value) ? body.value : [body];

    return {
      workflowData: [this.helpers.returnJsonArray(items)],
    };
  }
}

// ─── Shared helpers ────────────────────────────────────────────────────────────

/** Extracts the string value from a resourceLocator or a plain string param. */
function getResourceLocatorValue(param: unknown): string {
  if (typeof param === "object" && param !== null && "value" in param) {
    return (param as { value: string }).value;
  }
  return param as string;
}

/** Returns baseURL and credential name for both loadOptions and hook contexts. */
async function getBaseAndCredName(
  ctx: ILoadOptionsFunctions | IHookFunctions,
): Promise<{ baseURL: string; credName: string }> {
  const authMethod = ctx.getNodeParameter("authentication") as string;
  const credName =
    authMethod === "apiKey" ? "horneroDbApi" : "horneroDbOAuth2Api";
  const credentials = await ctx.getCredentials(credName);
  return { baseURL: credentials?.host as string, credName };
}

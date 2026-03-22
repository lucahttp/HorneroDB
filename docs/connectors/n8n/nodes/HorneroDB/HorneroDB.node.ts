import {
  IExecuteFunctions,
  INodeExecutionData,
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
          {
            name: "API Key",
            value: "apiKey",
          },
          {
            name: "OAuth2",
            value: "oAuth2",
          },
        ],
        default: "apiKey",
      },
      {
        displayName: "Resource",
        name: "resource",
        type: "options",
        noDataExpression: true,
        options: [
          {
            name: "Workspace",
            value: "workspace",
          },
          {
            name: "Record",
            value: "record",
          },
          {
            name: "Webhook",
            value: "webhook",
          },
        ],
        default: "record",
      },
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: {
          show: {
            resource: ["record"],
          },
        },
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
            description: "Get a record",
            action: "Get a record",
          },
          {
            name: "Get Many",
            value: "getAll",
            description: "Get many records",
            action: "Get many records",
          },
          {
            name: "Update",
            value: "update",
            description: "Update a record",
            action: "Update a record",
          },
        ],
        default: "create",
      },

      /* -------------------------------------------------------------------------- */
      /*                                 record:create                              */
      /* -------------------------------------------------------------------------- */
      {
        displayName: "Workspace ID",
        name: "workspaceId",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["record"],
          },
        },
        default: "",
        description: "The UUID of the workspace",
      },
      {
        displayName: "Table Slug",
        name: "tableSlug",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["record"],
          },
        },
        default: "",
        description: "The slug of the table",
      },
      {
        displayName: "Record ID",
        name: "recordId",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["record"],
            operation: ["get", "delete", "update"],
          },
        },
        default: "",
      },
      {
        displayName: "JSON Parameters",
        name: "jsonParameters",
        type: "boolean",
        default: true,
        displayOptions: {
          show: {
            resource: ["record"],
            operation: ["create", "update"],
          },
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
          show: {
            resource: ["record"],
            operation: ["get", "getAll"],
          },
        },
        default: "",
        description:
          "Comma-separated list of relation columns to expand into human-readable labels",
      },
      /* -------------------------------------------------------------------------- */
      /*                                 webhook                                    */
      /* -------------------------------------------------------------------------- */
      {
        displayName: "Workspace ID",
        name: "workspaceId",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
          },
        },
        default: "",
        description: "The UUID of the workspace",
      },
      {
        displayName: "Operation",
        name: "operation",
        type: "options",
        noDataExpression: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
          },
        },
        options: [
          {
            name: "Create",
            value: "create",
            description: "Create a webhook",
            action: "Create a webhook",
          },
          {
            name: "Delete",
            value: "delete",
            description: "Delete a webhook",
            action: "Delete a webhook",
          },
          {
            name: "Get All",
            value: "getAll",
            description: "Get all webhooks",
            action: "Get all webhooks",
          },
        ],
        default: "getAll",
      },
      {
        displayName: "Resource (Table UUID)",
        name: "webhookResource",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
            operation: ["create"],
          },
        },
        default: "",
        description: "The UUID of the table to monitor",
      },
      {
        displayName: "Notification URL",
        name: "notificationUrl",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
            operation: ["create"],
          },
        },
        default: "",
        description: "The URL to send notifications to",
      },
      {
        displayName: "Events",
        name: "events",
        type: "multiOptions",
        required: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
            operation: ["create"],
          },
        },
        options: [
          {
            name: "Record Created",
            value: "created",
          },
          {
            name: "Record Updated",
            value: "updated",
          },
          {
            name: "Record Deleted",
            value: "deleted",
          },
        ],
        default: ["created", "updated", "deleted"],
      },
      {
        displayName: "Webhook ID",
        name: "webhookId",
        type: "string",
        required: true,
        displayOptions: {
          show: {
            resource: ["webhook"],
            operation: ["delete"],
          },
        },
        default: "",
      },
    ],
  };

  async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
    const items = this.getInputData();
    const returnData: INodeExecutionData[] = [];
    const resource = this.getNodeParameter("resource", 0) as string;
    const operation = this.getNodeParameter("operation", 0) as string;

    let credentials;
    const authenticationMethod = this.getNodeParameter(
      "authentication",
      0,
    ) as string;
    if (authenticationMethod === "apiKey") {
      credentials = await this.getCredentials("horneroDbApi");
    } else {
      credentials = await this.getCredentials("horneroDbOAuth2Api");
    }

    const baseURL = credentials?.host as string;

    for (let i = 0; i < items.length; i++) {
      try {
        if (resource === "record") {
          const workspaceId = this.getNodeParameter("workspaceId", i) as string;
          const tableSlug = this.getNodeParameter("tableSlug", i) as string;

          if (operation === "get") {
            const recordId = this.getNodeParameter("recordId", i) as string;
            let url = `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`;
            const expand = this.getNodeParameter("expand", i) as string;
            if (expand) url += `?expand=${expand}`;

            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "GET",
                  url,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
          }

          if (operation === "getAll") {
            let url = `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}`;
            const expand = this.getNodeParameter("expand", i) as string;
            if (expand) url += `?expand=${expand}`;

            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "GET",
                  url,
                  json: true,
                },
              );

            const executionData = this.helpers.constructExecutionMetaData(
              this.helpers.returnJsonArray(responseData.data || responseData),
              { itemData: { item: i } },
            );
            returnData.push(...executionData);
          }

          if (operation === "create") {
            const bodyJson = this.getNodeParameter("bodyJson", i) as string;
            const body =
              typeof bodyJson === "string" ? JSON.parse(bodyJson) : bodyJson;

            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "POST",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}`,
                  body,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
          }

          if (operation === "update") {
            const bodyJson = this.getNodeParameter("bodyJson", i) as string;
            const recordId = this.getNodeParameter("recordId", i) as string;
            const body =
              typeof bodyJson === "string" ? JSON.parse(bodyJson) : bodyJson;

            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "PUT",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`,
                  body,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
          }

          if (operation === "delete") {
            const recordId = this.getNodeParameter("recordId", i) as string;
            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "DELETE",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/data/${tableSlug}/${recordId}`,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
          }
        }

        if (resource === "webhook") {
          const workspaceId = this.getNodeParameter("workspaceId", i) as string;

          if (operation === "getAll") {
            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "GET",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks`,
                  json: true,
                },
              );
            const executionData = this.helpers.constructExecutionMetaData(
              this.helpers.returnJsonArray(responseData.data || responseData),
              { itemData: { item: i } },
            );
            returnData.push(...executionData);
          }

          if (operation === "create") {
            const webhookResource = this.getNodeParameter(
              "webhookResource",
              i,
            ) as string;
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

            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "POST",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks`,
                  body,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
          }

          if (operation === "delete") {
            const webhookId = this.getNodeParameter("webhookId", i) as string;
            const responseData =
              await this.helpers.requestWithAuthentication.call(
                this,
                authenticationMethod === "apiKey"
                  ? "horneroDbApi"
                  : "horneroDbOAuth2Api",
                {
                  method: "DELETE",
                  url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks/${webhookId}`,
                  json: true,
                },
              );
            returnData.push({ json: responseData.data || responseData });
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

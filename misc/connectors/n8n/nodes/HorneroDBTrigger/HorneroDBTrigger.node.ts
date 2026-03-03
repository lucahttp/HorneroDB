import {
    IHookFunctions,
    IWebhookFunctions,
    INodeType,
    INodeTypeDescription,
    IWebhookResponseData,
} from 'n8n-workflow';

export class HorneroDBTrigger implements INodeType {
    description: INodeTypeDescription = {
        displayName: 'HorneroDB Trigger',
        name: 'horneroDbTrigger',
        icon: 'file:hornerodb.svg',
        group: ['trigger'],
        version: 1,
        description: 'Starts the workflow when HorneroDB data changes',
        defaults: {
            name: 'HorneroDB Trigger',
        },
        inputs: [],
        outputs: ['main'],
        credentials: [
            {
                name: 'horneroDbApi',
                required: true,
                displayOptions: {
                    show: {
                        authentication: ['apiKey'],
                    },
                },
            },
            {
                name: 'horneroDbOAuth2Api',
                required: true,
                displayOptions: {
                    show: {
                        authentication: ['oAuth2'],
                    },
                },
            },
        ],
        webhooks: [
            {
                name: 'default',
                httpMethod: 'POST',
                responseMode: 'onReceived',
                path: 'webhook',
            },
        ],
        properties: [
            {
                displayName: 'Authentication',
                name: 'authentication',
                type: 'options',
                options: [
                    {
                        name: 'API Key',
                        value: 'apiKey',
                    },
                    {
                        name: 'OAuth2',
                        value: 'oAuth2',
                    },
                ],
                default: 'apiKey',
            },
            {
                displayName: 'Workspace ID',
                name: 'workspaceId',
                type: 'string',
                required: true,
                default: '',
                description: 'The UUID of the workspace',
            },
            {
                displayName: 'Table ID',
                name: 'tableId',
                type: 'string',
                required: true,
                default: '',
                description: 'The UUID of the table to listen to',
            },
            {
                displayName: 'Events',
                name: 'events',
                type: 'multiOptions',
                required: true,
                default: ['created', 'updated', 'deleted'],
                options: [
                    {
                        name: 'Record Created',
                        value: 'created',
                    },
                    {
                        name: 'Record Updated',
                        value: 'updated',
                    },
                    {
                        name: 'Record Deleted',
                        value: 'deleted',
                    },
                ],
            },
        ],
    };

    webhookMethods = {
        default: {
            async checkExists(this: IHookFunctions): Promise<boolean> {
                const webhookData = this.getWorkflowStaticData('node');
                if (webhookData.webhookId !== undefined) {
                    try {
                        const authenticationMethod = this.getNodeParameter('authentication') as string;
                        const workspaceId = this.getNodeParameter('workspaceId') as string;

                        let credentials;
                        if (authenticationMethod === 'apiKey') {
                            credentials = await this.getCredentials('horneroDbApi');
                        } else {
                            credentials = await this.getCredentials('horneroDbOAuth2Api');
                        }

                        const baseURL = credentials?.host as string;

                        const response = await this.helpers.requestWithAuthentication.call(
                            this,
                            authenticationMethod === 'apiKey' ? 'horneroDbApi' : 'horneroDbOAuth2Api',
                            {
                                method: 'GET',
                                url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks/${webhookData.webhookId}`,
                                json: true,
                            }
                        );
                        if (response.id === webhookData.webhookId) {
                            return true;
                        }
                    } catch (error) {
                        if (error.statusCode === 404) {
                            return false;
                        }
                        throw error;
                    }
                }
                return false;
            },
            async create(this: IHookFunctions): Promise<boolean> {
                const webhookUrl = this.getNodeWebhookUrl('default');
                if (!webhookUrl) return false;

                const webhookData = this.getWorkflowStaticData('node');
                const authenticationMethod = this.getNodeParameter('authentication') as string;
                const workspaceId = this.getNodeParameter('workspaceId') as string;
                const tableId = this.getNodeParameter('tableId') as string;
                const events = this.getNodeParameter('events') as string[];

                let credentials;
                if (authenticationMethod === 'apiKey') {
                    credentials = await this.getCredentials('horneroDbApi');
                } else {
                    credentials = await this.getCredentials('horneroDbOAuth2Api');
                }

                const baseURL = credentials?.host as string;

                const body = {
                    resource: tableId,
                    change_type: events.join(','),
                    notification_url: webhookUrl,
                    client_state: `n8n-${this.getWorkflow().id}`
                };

                const response = await this.helpers.requestWithAuthentication.call(
                    this,
                    authenticationMethod === 'apiKey' ? 'horneroDbApi' : 'horneroDbOAuth2Api',
                    {
                        method: 'POST',
                        url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks`,
                        body,
                        json: true,
                    }
                );

                webhookData.webhookId = response.id;
                return true;
            },
            async delete(this: IHookFunctions): Promise<boolean> {
                const webhookData = this.getWorkflowStaticData('node');
                if (webhookData.webhookId !== undefined) {
                    const authenticationMethod = this.getNodeParameter('authentication') as string;
                    const workspaceId = this.getNodeParameter('workspaceId') as string;

                    let credentials;
                    if (authenticationMethod === 'apiKey') {
                        credentials = await this.getCredentials('horneroDbApi');
                    } else {
                        credentials = await this.getCredentials('horneroDbOAuth2Api');
                    }

                    const baseURL = credentials?.host as string;

                    try {
                        await this.helpers.requestWithAuthentication.call(
                            this,
                            authenticationMethod === 'apiKey' ? 'horneroDbApi' : 'horneroDbOAuth2Api',
                            {
                                method: 'DELETE',
                                url: `${baseURL}/api/v1/workspaces/${workspaceId}/webhooks/${webhookData.webhookId}`,
                                json: true,
                            }
                        );
                    } catch (error) {
                        return false;
                    }
                    delete webhookData.webhookId;
                }
                return true;
            },
        },
    };

    async webhook(this: IWebhookFunctions): Promise<IWebhookResponseData> {
        const req = this.getRequestObject();
        const body = req.body;

        // MS Graph Style wrapper arrays
        if (body.value && Array.isArray(body.value)) {
            // n8n expects flat objects or arrays of flat objects depending on execution node count
            // For triggers, we typically return the whole raw payload or a mapped version
            return {
                workflowData: [
                    this.helpers.returnJsonArray(body.value)
                ],
            };
        }

        return {
            workflowData: [
                this.helpers.returnJsonArray(body)
            ],
        };
    }
}

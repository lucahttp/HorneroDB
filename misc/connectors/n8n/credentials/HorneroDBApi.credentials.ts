import {
    IAuthenticateGeneric,
    ICredentialTestRequest,
    ICredentialType,
    INodeProperties,
} from 'n8n-workflow';

export class HorneroDBApi implements ICredentialType {
    name = 'horneroDbApi';
    displayName = 'HorneroDB API Key';
    documentationUrl = 'https://github.com/hornerodb/hornerodb';
    Properties: INodeProperties[] = [
        {
            displayName: 'Host URL',
            name: 'host',
            type: 'string',
            default: 'https://api.hornerodb.com',
            required: true,
            description: 'The base URL of your HorneroDB instance',
        },
        {
            displayName: 'API Key',
            name: 'apiKey',
            type: 'string',
            typeOptions: {
                password: true,
            },
            default: '',
            required: true,
        },
    ];

    authenticate: IAuthenticateGeneric = {
        type: 'generic',
        properties: {
            headers: {
                Authorization: '=Bearer {{$credentials.apiKey}}',
            },
        },
    };

    test: ICredentialTestRequest = {
        request: {
            baseURL: '={{$credentials.host}}',
            url: '/api/v1/auth/me',
            method: 'GET',
        },
    };
}

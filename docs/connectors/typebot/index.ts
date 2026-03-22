import { createIntegration } from '@typebot.io/forge'
import { getRecord } from './actions/getRecord'
import { insertRecord } from './actions/insertRecord'
import { searchRecords } from './actions/searchRecords'

export const horneroDbIntegration = createIntegration({
    id: 'horneroDb',
    name: 'HorneroDB',
    logo: 'https://raw.githubusercontent.com/hornerodb/hornerodb/main/logo.svg', // Replace with exact logo path
    description: 'Connect your Typebot chatbot to HorneroDB securely to read, write, and search records.',
    auth: {
        type: 'encrypted',
        name: 'API Key',
        inputs: [
            {
                id: 'host',
                name: 'HorneroDB Host URL',
                type: 'string',
                defaultValue: 'https://api.hornerodb.com',
                required: true,
            },
            {
                id: 'apiKey',
                name: 'API Key',
                type: 'string',
                helperText: 'A Workspace-level API key from your HorneroDB Security panel',
                required: true,
            }
        ]
    },
    actions: [getRecord, insertRecord, searchRecords],
})

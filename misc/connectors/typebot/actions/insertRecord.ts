import { createAction, option } from '@typebot.io/forge'

export const insertRecord = createAction({
    name: 'Insert Record',
    auth: 'horneroDb',
    options: option.object({
        workspaceId: option.string({
            label: 'Workspace ID',
        }),
        tableSlug: option.string({
            label: 'Table Slug',
        }),
        // Typebot doesn't natively support full dynamic un-typed form injection from an external API in the block definition itself 
        // without using a dynamic Array input, so we ask the user to provide a JSON object or map key/values
        recordData: option.string({
            label: 'Record Data (JSON)',
            helperText: 'A JSON object mapping column names to variables (e.g., {"email": "{{Email}}"})'
        })
    }),
    getSetVariableIds: (options) => {
        return options.targetVariableId ? [options.targetVariableId] : []
    },
    run: async ({ options, auth, logs }) => {
        if (!options.workspaceId || !options.tableSlug || !options.recordData) {
            logs.add('Error: Workspace ID, Table Slug, and Record Data are required.')
            return
        }

        try {
            let body;
            try {
                body = JSON.parse(options.recordData);
            } catch (e) {
                logs.add('Error: Record Data is not valid JSON')
                return
            }

            const response = await fetch(`${auth.host}/api/v1/workspaces/${options.workspaceId}/data/${options.tableSlug}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${auth.apiKey}`
                },
                body: JSON.stringify(body)
            })

            if (!response.ok) {
                logs.add(`Error: ${response.status} ${response.statusText}`)
                return
            }

            const result = await response.json()
            logs.add('Successfully inserted record')

        } catch (error) {
            logs.add(`Network Error: ${error.message}`)
            return
        }
    }
})

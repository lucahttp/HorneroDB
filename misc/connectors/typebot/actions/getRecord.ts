import { createAction, option } from '@typebot.io/forge'

export const getRecord = createAction({
    name: 'Get Record',
    auth: 'horneroDb',
    options: option.object({
        workspaceId: option.string({
            label: 'Workspace ID',
        }),
        tableSlug: option.string({
            label: 'Table Slug',
        }),
        recordId: option.string({
            label: 'Record ID',
        }),
        targetVariableId: option.string({
            label: 'Save response to variable',
            helperText: 'Select a variable to store the JSON response'
        })
    }),
    getSetVariableIds: (options) => {
        return options.targetVariableId ? [options.targetVariableId] : []
    },
    run: async ({ options, variables, auth, logs }) => {
        if (!options.workspaceId || !options.tableSlug || !options.recordId) {
            logs.add('Error: Workspace ID, Table Slug, and Record ID are required.')
            return
        }

        try {
            const response = await fetch(`${auth.host}/api/v1/workspaces/${options.workspaceId}/data/${options.tableSlug}/${options.recordId}`, {
                method: 'GET',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${auth.apiKey}`
                }
            })

            if (!response.ok) {
                logs.add(`Error: ${response.status} ${response.statusText}`)
                return
            }

            const result = await response.json()

            if (options.targetVariableId) {
                variables.set(options.targetVariableId, result)
            }

            logs.add('Successfully fetched record')

        } catch (error) {
            logs.add(`Network Error: ${error.message}`)
            return
        }
    }
})

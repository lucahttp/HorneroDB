import { createAction, option } from '@typebot.io/forge'

export const searchRecords = createAction({
    name: 'Search Records',
    auth: 'horneroDb',
    options: option.object({
        workspaceId: option.string({
            label: 'Workspace ID',
        }),
        tableSlug: option.string({
            label: 'Table Slug',
        }),
        queryParams: option.string({
            label: 'Query Parameters (Optional)',
            helperText: 'E.g. ?page=1&per_page=10'
        }),
        targetVariableId: option.string({
            label: 'Save response to variable',
            helperText: 'Select a variable to store the JSON array response'
        })
    }),
    getSetVariableIds: (options) => {
        return options.targetVariableId ? [options.targetVariableId] : []
    },
    run: async ({ options, variables, auth, logs }) => {
        if (!options.workspaceId || !options.tableSlug) {
            logs.add('Error: Workspace ID and Table Slug are required.')
            return
        }

        try {
            const qs = options.queryParams ? options.queryParams : '';
            const response = await fetch(`${auth.host}/api/v1/workspaces/${options.workspaceId}/data/${options.tableSlug}${qs}`, {
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

            logs.add('Successfully fetched records')

        } catch (error) {
            logs.add(`Network Error: ${error.message}`)
            return
        }
    }
})

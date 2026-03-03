# HorneroDB MCP Server

The **Model Context Protocol (MCP)** server allows local AI assistants like **Claude Desktop** and **Cursor** to directly interface with your HorneroDB instance.

By installing this MCP Server, the AI gains "Tools" to:
- List your Workspaces and Tables.
- Intelligently read the **Column Schema** of your tables before taking action.
- Query, Insert, Update, and Delete records in your database conversationally.

## Installation

1. From this directory (`misc/connectors/mcp-server`), run:
   ```bash
   npm install
   npm run build
   ```

2. Note the absolute path to `build/index.js` in this repository.

## Connecting to Claude Desktop

Edit your `claude_desktop_config.json` (usually located at `~/Library/Application Support/Claude/claude_desktop_config.json` on Mac).

Add the following to the `mcpServers` object:

```json
{
  "mcpServers": {
    "horneroDB": {
      "command": "node",
      "args": [
        "/ABSOLUTE/PATH/TO/hornerodb/misc/connectors/mcp-server/build/index.js"
      ],
      "env": {
        "HORNERODB_URL": "http://localhost:8080",
        "HORNERODB_API_KEY": "your_api_key_from_hornerodb"
      }
    }
  }
}
```

Restart Claude Desktop, and you can now ask Claude questions like:
_"Based on my Turnos table in HorneroDB, what is the schema? Can you insert a new appointment for tomorrow?"_

## Connecting to Cursor

1. Open Cursor Settings -> Features -> MCP
2. Click **+ Add new MCP server**
3. Name: `horneroDB`
4. Type: `command`
5. Command: `node /ABSOLUTE/PATH/TO/hornerodb/misc/connectors/mcp-server/build/index.js`

*Note: You must have a `.env` file in the `mcp-server` directory with `HORNERODB_URL` and `HORNERODB_API_KEY` when running via Cursor, as Cursor doesn't natively supply ENV vars in the GUI yet.*

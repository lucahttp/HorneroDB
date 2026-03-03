"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const index_js_1 = require("@modelcontextprotocol/sdk/server/index.js");
const stdio_js_1 = require("@modelcontextprotocol/sdk/server/stdio.js");
const types_js_1 = require("@modelcontextprotocol/sdk/types.js");
const axios_1 = __importDefault(require("axios"));
const dotenv = __importStar(require("dotenv"));
dotenv.config();
const HORNERODB_URL = process.env.HORNERODB_URL || "http://localhost:8080";
const HORNERODB_API_KEY = process.env.HORNERODB_API_KEY;
if (!HORNERODB_API_KEY) {
    console.error("HORNERODB_API_KEY environment variable is required");
    process.exit(1);
}
const api = axios_1.default.create({
    baseURL: `${HORNERODB_URL}/api/v1`,
    headers: {
        Authorization: `Bearer ${HORNERODB_API_KEY}`,
        "Content-Type": "application/json",
    },
});
const server = new index_js_1.Server({
    name: "hornerodb-mcp-server",
    version: "1.0.0",
}, {
    capabilities: {
        tools: {},
    },
});
// Define Tools
server.setRequestHandler(types_js_1.ListToolsRequestSchema, async () => {
    return {
        tools: [
            {
                name: "list_workspaces",
                description: "List all accessible Workspaces in HorneroDB",
                inputSchema: { type: "object", properties: {} },
            },
            {
                name: "list_tables",
                description: "List all tables within a specific workspace",
                inputSchema: {
                    type: "object",
                    properties: {
                        workspace_id: { type: "string", description: "UUID of the workspace" },
                    },
                    required: ["workspace_id"],
                },
            },
            {
                name: "get_table_schema",
                description: "Get the column definitions for a table. REQUIRED before inserting new records to understand data types.",
                inputSchema: {
                    type: "object",
                    properties: {
                        workspace_id: { type: "string" },
                        table_id: { type: "string" },
                    },
                    required: ["workspace_id", "table_id"],
                },
            },
            {
                name: "query_records",
                description: "Fetch records from a table",
                inputSchema: {
                    type: "object",
                    properties: {
                        workspace_id: { type: "string" },
                        table_slug: { type: "string" },
                        page: { type: "number" },
                    },
                    required: ["workspace_id", "table_slug"],
                },
            },
            {
                name: "create_record",
                description: "Insert a new row into a table",
                inputSchema: {
                    type: "object",
                    properties: {
                        workspace_id: { type: "string" },
                        table_slug: { type: "string" },
                        data: {
                            type: "object",
                            description: "JSON Object mapping column names to values"
                        },
                    },
                    required: ["workspace_id", "table_slug", "data"],
                },
            },
        ],
    };
});
// Implement Tool Logic
server.setRequestHandler(types_js_1.CallToolRequestSchema, async (request) => {
    try {
        const { name, arguments: args } = request.params;
        switch (name) {
            case "list_workspaces": {
                const res = await api.get("/workspaces");
                return { content: [{ type: "text", text: JSON.stringify(res.data, null, 2) }] };
            }
            case "list_tables": {
                if (!args || typeof args.workspace_id !== 'string')
                    throw new Error("workspace_id required");
                const res = await api.get(`/workspaces/${args.workspace_id}/tables`);
                return { content: [{ type: "text", text: JSON.stringify(res.data, null, 2) }] };
            }
            case "get_table_schema": {
                if (!args || typeof args.workspace_id !== 'string' || typeof args.table_id !== 'string')
                    throw new Error("workspace_id and table_id required");
                const res = await api.get(`/workspaces/${args.workspace_id}/tables/${args.table_id}/columns`);
                return { content: [{ type: "text", text: JSON.stringify(res.data, null, 2) }] };
            }
            case "query_records": {
                if (!args || typeof args.workspace_id !== 'string' || typeof args.table_slug !== 'string')
                    throw new Error("workspace_id and table_slug required");
                let url = `/workspaces/${args.workspace_id}/data/${args.table_slug}`;
                if (args.page)
                    url += `?page=${args.page}`;
                const res = await api.get(url);
                return { content: [{ type: "text", text: JSON.stringify(res.data, null, 2) }] };
            }
            case "create_record": {
                if (!args || typeof args.workspace_id !== 'string' || typeof args.table_slug !== 'string' || typeof args.data !== 'object')
                    throw new Error("workspace_id, table_slug, and data object required");
                const res = await api.post(`/workspaces/${args.workspace_id}/data/${args.table_slug}`, args.data);
                return { content: [{ type: "text", text: JSON.stringify(res.data, null, 2) }] };
            }
            default:
                throw new Error(`Unknown tool: ${name}`);
        }
    }
    catch (error) {
        const errorMsg = error.response ? JSON.stringify(error.response.data) : error.message;
        return {
            content: [{ type: "text", text: `Error: ${errorMsg}` }],
            isError: true,
        };
    }
});
async function main() {
    const transport = new stdio_js_1.StdioServerTransport();
    await server.connect(transport);
    console.error("HorneroDB MCP Server running on stdio");
}
main().catch((error) => {
    console.error("Server error:", error);
    process.exit(1);
});

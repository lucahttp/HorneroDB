# HorneroDB MCP Server

The **Model Context Protocol (MCP)** server is now built into HorneroDB's Go backend, providing secure, authenticated access for AI assistants like **Claude Desktop**, **Cursor**, and other MCP-compatible clients.

## Overview

The MCP server allows AI assistants to:
- **List Workspaces and Tables** — Discover available data structures
- **Read Column Schema** — Understand table structure before taking action
- **Query Records** — Read data with automatic permission filtering
- **Create, Update, Delete Records** — Modify data with full security enforcement

All operations require authentication and respect HorneroDB's granular permissions (row-level and column-level security).

---

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  AI Assistant   │────→│   MCP Protocol   │────→│  HorneroDB      │
│  (Claude/Cursor)│     │   (SSE/HTTP)     │     │  Go Backend     │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │                           │
                               │ OAuth2/JWT               │ Auth + Permissions
                               │ Authentication           │ Row/Column Level Security
                               └───────────────────────────┘
```

### Security Features

✅ **Authentication Required** — All MCP connections must authenticate via:
- OIDC/SSO (OAuth2 flow issuing JWT session tokens for human users)
- API Keys (Bearer `key_...` tokens for automated services and AI agents)

✅ **Workspace Isolation** — Users can only access workspaces where they have roles

✅ **Permission Enforcement** — Every operation checks:
- Table-level permissions (read/create/update/delete)
- Row-level security (filters by `created_by` for "own" access)
- Column-level security (hides restricted columns)

✅ **Audit Logging** — All MCP operations are logged with user context

---

## Configuration

### 1. Enable MCP Server

The MCP server is enabled by default in HorneroDB. It runs on the same port as the API:

```bash
# Default endpoints (when server runs on port 8080)
SSE Endpoint:    http://localhost:8080/api/v1/mcp/sse
Message Endpoint: http://localhost:8080/api/v1/mcp/message
OAuth Discovery: http://localhost:8080/.well-known/oauth-authorization-server
```

### 2. CORS Configuration

For MCP clients running in browsers (like web-based AI assistants), add their origins to CORS:

```bash
# .env file
HORNERO_ADMIN_URL=http://localhost:5173
CORS_ORIGINS=http://localhost:5173,https://claude.ai,https://cursor.com
```

### 3. Create MCP-Enabled API Key (Recommended)

For service-to-service communication, create an API key with MCP permissions:

```json
// Role permissions for MCP access
{
  "__system__": {
    "mcp": "all",        // Required for MCP access
    "tables": "view",    // Optional: view table metadata
    "api_keys": "none"   // Recommended: don't allow key management
  },
  "customers": {
    "read": "all",
    "create": "own",
    "update": "own",
    "delete": "none"
  }
}
```

Generate the API key in the HorneroDB admin panel and note the key (starts with `key_`).

---

## Connecting Clients

### Claude Desktop

Edit `claude_desktop_config.json`:

**Mac**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "horneroDB": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-sse",
        "http://localhost:8080/api/v1/mcp/sse"
      ],
      "env": {
        "HORNERO_API_KEY": "key_your_api_key_here"
      }
    }
  }
}
```

### Cursor

1. Open Cursor Settings → Features → MCP
2. Click **+ Add new MCP server**
3. Name: `horneroDB`
4. Type: `sse`
5. URL: `http://localhost:8080/api/v1/mcp/sse`
6. Headers (optional, for API Key auth):
   ```
   Authorization: key_your_api_key_here
   ```

### VS Code with Cline/Roo Code

Add to MCP settings:

```json
{
  "mcpServers": {
    "horneroDB": {
      "transport": "sse",
      "url": "http://localhost:8080/api/v1/mcp/sse",
      "headers": {
        "Authorization": "key_your_api_key_here"
      }
    }
  }
}
```

---

## Available Tools

The MCP server exposes these tools to AI assistants:

### `list_workspaces`
List all workspaces the authenticated user has access to.

**Parameters**: None

**Returns**: Array of workspaces with `id`, `name`, `description`

---

### `list_tables`
List all tables in a workspace.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace"
}
```

**Returns**: Array of tables with `id`, `name`, `slug`

---

### `list_columns`
Get the schema (columns) of a table.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers"
}
```

**Returns**: Array of columns with `name`, `slug`, `field_type`, `required`, etc.

**Note**: Requires `read` permission on the table.

---

### `list_records`
Query records from a table with automatic permission filtering.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "limit": 100,      // Optional, max 1000
  "offset": 0        // Optional, for pagination
}
```

**Returns**:
```json
{
  "records": [...],
  "count": 150
}
```

**Security**: 
- If user has `read: "own"`, only returns records where `created_by = user_id`
- Column-level security applied (restricted columns hidden)

---

### `create_record`
Insert a new record.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "data": {
    "name": "John Doe",
    "email": "john@example.com"
  }
}
```

**Returns**: Created record with `id` and all fields

**Security**:
- Requires `create` permission on table
- Column-level security applied (only allowed columns are written)
- `created_by` automatically set to authenticated user

---

### `get_record`
Get a single record by ID.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "record_id": "uuid-of-record"
}
```

**Returns**: Record object or error if not found/no access

**Security**:
- If user has `read: "own"`, only returns if `created_by = user_id`
- Column-level security applied

---

### `update_record`
Update an existing record.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "record_id": "uuid-of-record",
  "data": {
    "status": "completed"
  }
}
```

**Returns**: Updated record

**Security**:
- Requires `update` permission on table
- If user has `update: "own"`, only updates if `created_by = user_id`
- Column-level security applied (only allowed columns updated)

---

### `delete_record`
Delete a record.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "record_id": "uuid-of-record"
}
```

**Returns**: `{ "message": "deleted", "id": "..." }`

**Security**:
- Requires `delete` permission on table
- If user has `delete: "own"`, only deletes if `created_by = user_id`

---

### `create_column`
Add a new column to a table and alter the underlying physical table.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "name": "Status",
  "field_type": "select",
  "meta": { "choices": [{"value": "active", "label": "Active"}] }
}
```

*Supported field types*: `text`, `long_text`, `number`, `float`, `integer`, `boolean`, `date`, `datetime`, `email`, `url`, `attachment`, `select`, `relation`, `json`, `autonumber` (sequential ID e.g. `meta: {"prefix": "PED-", "digits": 3, "current_value": 1}`).

---

### `update_column`
Update an existing column's definition, slug, field type, or metadata, and update the physical database table (`ALTER TABLE ... RENAME / ALTER COLUMN TYPE`).

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "column_slug": "estado",
  "name": "Estado",              // Optional
  "new_slug": "estado_nuevo",    // Optional: renames column physically
  "field_type": "integer",       // Optional: alters column type physically
  "meta": {}                     // Optional: updates field metadata
}
```

---

### `delete_column`
Delete a column and drop its physical data from the database.

**Parameters**:
```json
{
  "workspace_id": "uuid-of-workspace",
  "table_slug": "customers",
  "column_slug": "status"
}
```

---

## Authentication Flow

### Option 1: API Key (Recommended for Services)

```
1. Client sends: Authorization: key_xxxxx header
2. Server validates key hash against database
3. Server extracts role from key
4. All operations use key's role for permission checks
```

### Option 2: OAuth2 (Recommended for User Sessions)

```
1. Client redirects user to /api/v1/mcp/oauth/authorize
2. User authenticates with PocketID (OIDC)
3. Server redirects back with authorization code
4. Client exchanges code for JWT at /api/v1/mcp/oauth/token
5. Client uses JWT for subsequent requests
```

**Refresh tokens:** `/api/v1/mcp/oauth/token` also accepts `grant_type=refresh_token`. Refresh tokens are valid for 30 days, rotated on use, and return a new access token + refresh token pair without re-prompting the user.

**Scopes:** `mcp:read`, `mcp:write`, `mcp:admin` (advertised in `/.well-known/oauth-authorization-server`).

---

## Connecting to Microsoft Copilot Studio

HorneroDB is a first-class MCP server for Copilot Studio. The recommended setup uses the **MCP onboarding wizard** (no Teams Developer Portal registration needed).

### Prerequisites

- A public HTTPS URL for your HorneroDB instance (e.g. `https://api.example.com`)
- The instance must be reachable from Microsoft 365 (no IP allowlists blocking Microsoft's ranges)
- A Power Platform environment with **generative orchestration** enabled
- Admin access in [Copilot Studio](https://web.powerva.microsoft.com/) and the [Power Platform admin center](https://admin.powerplatform.microsoft.com/)

### Transport

HorneroDB exposes **two** MCP transports on the same instance:

| Transport | Endpoint | Spec | Used by |
|---|---|---|---|
| **Streamable HTTP** (preferred) | `POST /api/v1/mcp/stream` | MCP 2025-03-26 | Copilot Studio (after Aug 2025) |
| **SSE** (legacy) | `GET /api/v1/mcp/sse` + `POST /api/v1/mcp/message?sessionId=…` | MCP 2024-11-05 | Claude Desktop, Cursor, VS Code Cline |

When the wizard asks for a **Server URL**, use:
```
https://api.example.com/api/v1/mcp/stream
```

### Option A — Onboarding wizard with Dynamic Discovery (recommended)

1. Open your agent in **Copilot Studio** → **Tools** → **Add a tool** → **New tool** → **Model Context Protocol**.
2. Fill in:
   - **Server name:** `HorneroDB`
   - **Server description:** A clear one-liner about what data the agent can access
   - **Server URL:** `https://api.example.com/api/v1/mcp/stream`
3. For **Authentication type**, choose `OAuth 2.0` and then **Dynamic discovery**.
4. Click **Create**. Copilot Studio will:
   - Fetch `https://api.example.com/.well-known/oauth-authorization-server`
   - Register a client via `POST /api/v1/mcp/oauth/register` (RFC 7591)
   - Open the PocketID login in a popup
   - Exchange the auth code at `POST /api/v1/mcp/oauth/token`
5. After the user signs in, all 15 HorneroDB tools appear as actions in your agent.

### Option B — Custom connector via Power Apps (alternative)

If you need to ship HorneroDB as a managed connector in a solution, use the OpenAPI schema:

1. Go to [Power Apps](https://make.powerapps.com/) → **Custom connectors** → **New custom connector** → **Import OpenAPI file**.
2. Import from `https://api.example.com/api/v1/mcp/schema.yaml` (or download it via `curl`).
3. Set the host to your public URL, choose **OAuth 2.0** as auth, and import.
4. The schema declares `x-ms-agentic-protocol: mcp-streamable-1.0`, which tells Copilot Studio to use the streamable transport.

### DLP (Data Loss Prevention) policies

Copilot Studio applies Power Platform DLP policies to MCP connectors. After you add HorneroDB to an agent, ask a tenant admin to:

1. Go to [Power Platform admin center](https://admin.powerplatform.microsoft.com/) → **Policies** → **Data policies**.
2. Either:
   - Add HorneroDB to the **Business** data group (so the agent can call its tools), or
   - Create a custom connector classification that allows read/write to HorneroDB tables.
3. Re-publish the agent for the DLP change to take effect.

### Tools available to the agent

Every tool returned by `tools/list` becomes an action in Copilot Studio. The agent's generative orchestrator picks tools based on the user's intent. Names and descriptions in HorneroDB are in Spanish today — for an English-first agent, consider updating tool descriptions in `internal/handlers/mcp/server.go`.

### Resources (knowledge sources)

HorneroDB also exposes MCP **resources** that Copilot Studio can ingest as read-only knowledge:

- `table://<workspace_id>/<table_slug>` — schema (columns) of a table
- `table://<workspace_id>/<table_slug>/data` — first 100 rows

Copilot Studio currently consumes resources alongside tools, allowing the agent to ground answers in your database schema and a recent sample of records.

### Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| Wizard says "Unable to reach server" | Public URL not reachable from MS | Check DNS, TLS cert, and that firewall allows MS ranges |
| "Discovery failed" | Missing or broken `/.well-known/oauth-authorization-server` | `curl` that URL; should return JSON with `authorization_endpoint` and `token_endpoint` |
| "redirect_uri mismatch" during login | HorneroDB OAuth rejects unknown redirect URIs | Pre-register `https://*.powerva.microsoft.com/*` or enable `MCP_ALLOW_DYNAMIC_REDIRECT=true` |
| Tools don't appear in the agent | Generative orchestration off | Enable **Generative AI | Generative orchestration** in the agent settings |
| Tools return "access denied" | The authenticated PocketID user has no role in any workspace | Assign a role to the user in HorneroDB admin first |

---

## Security Best Practices

### 1. Principle of Least Privilege

Grant AI assistants only the permissions they need:

```json
// ❌ Too permissive
{
  "*": { "read": "all", "create": "all", "update": "all", "delete": "all" }
}

// ✅ Just enough for a booking assistant
{
  "appointments": {
    "read": "all",
    "create": "all",
    "update": "own",    // Can only update appointments it created
    "delete": "none"
  },
  "customers": {
    "read": ["name", "email", "phone"],  // Only specific columns
    "create": "all",
    "update": "none"
  }
}
```

### 2. Use Dedicated API Keys

Create separate API keys for each AI assistant/integration:
- Easier to revoke if compromised
- Independent rate limiting
- Clear audit trail

### 3. Monitor MCP Activity

Check logs for MCP operations:
```bash
docker logs hornerodb-server | grep "mcp"
```

### 4. CORS Restrictions

Never use `CORS_ORIGINS=*` in production. Always specify exact domains:
```bash
# ❌ Insecure
CORS_ORIGINS=*

# ✅ Secure
CORS_ORIGINS=https://admin.mydomain.com,https://api.mydomain.com
```

---

## Troubleshooting

### "Authentication required" error
- Verify API key is valid and starts with `key_`
- Check that key has `__system__.mcp: "all"` permission
- Ensure key hasn't expired

### "Access denied" error
- Check user's role has permission for the operation
- Verify workspace ID is correct
- For "own" access, verify record's `created_by` matches user

### CORS errors
- Add client's origin to `CORS_ORIGINS` env variable
- Ensure origin includes protocol (http:// or https://)
- Check for trailing slashes (should not have them)

### Connection refused
- Verify HorneroDB server is running
- Check firewall rules for port 8080
- Ensure correct URL in MCP client config

---

## Example: Building an AI Booking Assistant

```javascript
// AI Assistant conversation:
User: "Create a new appointment for John tomorrow at 2pm"

AI: [Uses MCP tools]
1. list_workspaces → Finds "Salon Booking" workspace
2. list_tables → Finds "appointments" table
3. list_columns → Gets schema (customer_name, date, time, service)
4. create_record → Inserts appointment
   {
     workspace_id: "...",
     table_slug: "appointments",
     data: {
       customer_name: "John",
       date: "2024-03-15",
       time: "14:00",
       service: "Haircut"
     }
   }

AI: "✅ Appointment created for John on March 15th at 2:00 PM"
```

All operations automatically respect the AI's permissions (e.g., can only create appointments, not delete them).

---

## Further Reading

- [Model Context Protocol Specification](https://modelcontextprotocol.io/)
- [HorneroDB Security Documentation](../../SECURITY.md)
- [API Key Management](../../API_KEYS.md)

---

**Note**: The MCP server is built into HorneroDB's Go backend (not a separate Node.js server). This provides better performance and security through shared authentication and authorization layers.

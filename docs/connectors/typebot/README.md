# HorneroDB Typebot Forge Block

A [Typebot Forge](https://docs.typebot.com/contribute/the-forge/overview) block that lets any Typebot chat flow read, write, search, update and delete records in a [HorneroDB](https://github.com/hornerodb/hornerodb) workspace. Modelled after the official [NocoDB forge block](https://github.com/baptisteArno/typebot.io/tree/main/packages/forge/blocks/nocodb) so it follows the same conventions the rest of the Typebot codebase uses.

## Folder layout

```
docs/connectors/typebot/
├── package.json
├── tsconfig.json
├── tsconfig.lib.json
├── README.md
└── src/
    ├── index.ts              # createBlock definition
    ├── auth.ts               # createAuth (baseUrl + apiKey)
    ├── constants.ts          # defaultBaseUrl, defaultLimit, maxLimit
    ├── logo.tsx              # HorneroDB SVG logo
    ├── schemas.ts            # parseBlockSchema, parseBlockCredentials
    ├── types.ts              # Response types
    ├── actions/
    │   ├── fetchers.ts       # listWorkspaces / listTables refs
    │   ├── createRecord.ts
    │   ├── getRecord.ts
    │   ├── searchRecords.ts
    │   ├── updateRecord.ts
    │   └── deleteRecord.ts
    ├── helpers/
    │   ├── applyResponseMapping.ts
    │   ├── parseRecordBody.ts
    │   └── parseSearchParams.ts
    └── handlers/
        ├── index.ts
        ├── createRecordHandler.ts
        ├── getRecordHandler.ts
        ├── searchRecordsHandler.ts
        ├── updateRecordHandler.ts
        ├── deleteRecordHandler.ts
        └── fetchersHandler.ts
```

## Block metadata

| Field       | Value                                              |
|-------------|----------------------------------------------------|
| Block id    | `hornerodb`                                        |
| Block name  | `HorneroDB`                                        |
| Auth type   | `encryptedCredentials` (one per workspace)         |
| Actions     | Create, Get, Search, Update, Delete Record         |
| Tags        | `database`                                         |

## Endpoints used

All requests are made with `Authorization: Bearer <apiKey>` against the configured `baseUrl` (`http://localhost:8090` by default):

| Action     | Method | Path                                                |
|------------|--------|-----------------------------------------------------|
| Create     | POST   | `/api/v1/workspaces/:workspaceId/data/:tableSlug`   |
| Get        | GET    | `/api/v1/workspaces/:workspaceId/data/:tableSlug/:id` |
| Search     | GET    | `/api/v1/workspaces/:workspaceId/data/:tableSlug?limit&offset&expand` |
| Update     | PUT    | `/api/v1/workspaces/:workspaceId/data/:tableSlug/:id` |
| Delete     | DELETE | `/api/v1/workspaces/:workspaceId/data/:tableSlug/:id` |
| Fetcher    | GET    | `/api/v1/workspaces`                                |
| Fetcher    | GET    | `/api/v1/workspaces/:workspaceId/tables`            |

## Installing the block in Typebot

This folder is a self-contained drop-in for the Typebot monorepo. To use it:

1. Clone the [typebot.io](https://github.com/baptisteArno/typebot.io) repository (or your fork):

   ```bash
   git clone https://github.com/baptisteArno/typebot.io.git
   cd typebot.io
   cp .env.dev.example .env
   bun install
   bunx nx run-many --target=db:push
   docker compose -f docker-compose.dev.yml up -d
   ```

2. Copy this whole folder into the Forge blocks directory:

   ```bash
   cp -R C:\Users\lucas\HorneroDB\docs\connectors\typebot packages/forge/blocks/hornerodb
   ```

3. The new block uses the same workspace deps (`@typebot.io/forge`, `@typebot.io/lib`) as the other blocks — no new dependencies are needed. Just `pnpm install` again so Turborepo picks the package up.

4. Start the Typebot dev environment (see [Typebot – Local installation](https://docs.typebot.com/contribute/guides/local-installation)) and the **HorneroDB** card will appear in the editor sidebar.

5. Configure the block in your bot: paste the `baseUrl` of your HorneroDB instance and a workspace API key. Then drop a HorneroDB action into the flow.

## Configuring the API key in HorneroDB

1. In HorneroDB, open your workspace → **API Keys** → **Generate Key**.
2. Pick (or create) a role with the permissions the bot needs. A safe default for a bot that only inserts leads is:

   ```json
   {
     "__system__": { "tables": "view" },
     "leads":      { "create": "all", "read": "none" }
   }
   ```

3. Paste the key into the Typebot block. HorneroDB encrypts it at rest; Typebot only sends it over HTTPS to the `baseUrl` you configured.

## Action reference

### Create Record
- `workspaceId` (dropdown of workspaces)
- `tableSlug` (dropdown of tables in the selected workspace)
- `fields[]` — list of `{ key, value }` pairs. Typebot variables can be inlined in `value`.
- `responseMapping[]` *(optional)* — extract fields from the new record into Typebot variables.

### Get Record
- `workspaceId`, `tableSlug`, `recordId` (UUID)
- `responseMapping[]` — extract fields from the record into variables.

### Search Records
- `workspaceId`, `tableSlug`
- `returnType` — `All` / `First` / `Last` / `Random`
- `limit` (1–1000, default 20), `offset` (default 0)
- `expand` *(optional)* — comma-separated relation column names to expand
- `responseMapping[]` — extracts a single column across rows; when `returnType` resolves to a single row, the variable gets a scalar, otherwise an array.

### Update Record
- `workspaceId`, `tableSlug`, `recordId`
- `updates[]` — list of `{ key, value }`. Empty `value` sends `null` so the column is cleared.

### Delete Record
- `workspaceId`, `tableSlug`, `recordId`

## Development tips

- The block uses the standard `ky` HTTP client from `@typebot.io/lib/ky` and `parseUnknownError` for consistent error messages.
- All actions return their work via the `logs` helper, so failures show up in the preview / results tab.
- If you change a schema, regenerate `src/schemas.ts` with the `bun generate-block-schema` CLI command from the Typebot repo.

## Cleaning up

The Typebot reference repo was cloned into `.typebot-ref/` at the repo root to copy the NocoDB block structure. It is listed in `.gitignore` and can be safely deleted once development is done:

```bash
rm -rf /Users/luca/hornerodb/.typebot-ref
```

# Security Fixes — April 2026

Fixing all issues found in the security review, highest-risk/highest-impact first.

## Progress

| ID | Title | Status |
|----|-------|--------|
| S7 | `math/rand` → `crypto/rand` para API key generation | [x] |
| S11 | `mcpDeleteTable` raw DDL concat → `ValidateTableName` | [x] |
| S12 | `mcpDeleteColumn` slug no validado antes del DDL | [x] |
| D5 | `key_hash` expuesto en `ListAPIKeys` response | [x] |
| D2 | Eliminar `fmt.Printf` debug de `ImportUser` + `WorkspaceAuth` | [x] |
| S5/S6 | MCP list tools ignoran workspace owners | [x] |
| S4 | MCP stdio mode requiere credenciales via `MCP_API_KEY` | [x] |
| S2 | `GetWebhook` fuera del sub-grupo `RequireSystemPermission` | [x] — already in webhooksGroup |
| S1 | `RequireSystemPermission` workspace boundary check | [x] — WorkspaceAuth already handles it; doc comment added |
| D3 | `ImportWorkspace` mapa de tipos duplicado → usa `GetColumnSQL` | [x] |

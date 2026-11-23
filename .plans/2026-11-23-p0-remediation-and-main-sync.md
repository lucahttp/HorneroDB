# Plan de Remediación P0 + Sync con `origin/main` — HorneroDB

**Fecha:** 2026-11-23  
**Organizador:** `antigravity/gemini-4-neon`  
**Ejecutor:** `antigravity/gemini-4-hydrogen`  
**Objetivo:** Resolver de raíz el bloqueo de alta de rows/columnas (permisos/sesión), sincronizar el working tree con los 9 commits adelantados de `origin/main` (`e724aa8`) sin perder trabajo de Fase 0/1, cerrar todos los P0 de producción y validar live en Docker/Tailscale.

## Progress

| Wave | Task | Ejecutor (`gemini-4-hydrogen`) | Alcance Principal | Estado |
|---|---|---|---|---|
| 1 | W1-GitSync | `exec-git-sync-origin-main` | Limpiar artefactos basura (`0`, `cmd/tmp-bcryptcheck/`), backup branch + commit local de Fase 0/1 (incluyendo los 3 `.js` untracked), merge limpio de `origin/main` (`e724aa8`: OData, `autonumber`, MCP auth/fixes, `HORNERO_ADMIN_URL` fallback) preservando todo el endurecimiento de Fase 0/1, validar build/lint/tests | 🔄 En curso |
| 2 | W2-AuthPerms | `exec-auth-permissions-fix` | Fix raíz rows/columnas: consumir `GET /api/v1/auth/permissions?workspace_id=...` en `TableView.jsx` y `Settings.jsx`, refrescar/re-resolver rol en `/auth/me` y post-setup/workspace, gating `__system__` en tabs de Settings, tiebreaker `id` en CSV export, `limit: 200` en `/keys` y `/webhooks` de `Settings.jsx` | ⏳ Pendiente |
| 2 | W2-BackendP0 | `exec-backend-p0-infra-sse-errors` | Fix `WriteTimeout` en SSE MCP, normalizar los 8 errores no-envelope a `internal/response`, `meta.total`/`has_more` en list endpoints, pre-flight en migraciones de índices únicos, quitar tailnet personal hardcodeado de `docker-compose.yml`, documentar upgrade de volumen PG18 en `docs/OPERATIONS.md` | ⏳ Pendiente |
| 2 | W2-FrontendLists | `exec-frontend-pagination-ux` | Corregir truncamiento silencioso a 20 items en `Dashboard.jsx`, `Workspace.jsx`, `TableEditModal.jsx`, `SettingsUsers.jsx`, `InstanceAdmin.jsx`; debounce en `RelationPicker.jsx`; rutas `/login` → `/` en `Callback.jsx` y `AxiosInterceptor.jsx` | ⏳ Pendiente |
| 3 | W3-VerifyLive | `exec-rebuild-and-e2e-verify` | Full test suite (`go build/vet/test`, `eslint`, `vite build`), rebuild de `hornerodb-server`, verificación live en `:8080` y `:1411` (permisos dinámicos, alta de columnas/rows, OData, OIDC, MCP SSE) | ⏳ Pendiente |

## Riesgos y Reglas
- **Risk-First:** Wave 1 corre sola antes de Wave 2 para evitar merges dolorosos sobre trabajo nuevo.
- **Disjoint Ownership en Wave 2:** Ningún archivo se comparte entre `W2-AuthPerms`, `W2-BackendP0` y `W2-FrontendLists`.
- **Seguridad Git:** Nunca commitear `.env`, `data/` ni pushear al remoto sin pedido explícito del usuario.
- **No tocar:** `https://desktop-vvfip0e.taila8065d.ts.net` (T3 Code en puerto `3773`).

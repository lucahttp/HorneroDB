# Plan Fase 1 — HorneroDB v1.0 GA Launch

**Fecha:** 2026-11-22  
**Prerrequisito cumplido:** Fase 0 (12 bloqueantes P0 cerrados, `FE_LINT=0`, `FE_BUILD=0`, `GO_VET=0`, `GO_TEST=0`, `GO_BUILD=0`).

## Progress

| Track | Subagente (`MiniMax-M3.1-Flash · thinking`) | Alcance Principal | Estado |
|---|---|---|---|
| GA-1 | `ga1-query-filter-sort-bulk` | `?filter`, `?order`/`?sort`, `?search` (CLS-safe whitelist) + `POST/DELETE /data/:slug/bulk` atómicos + 99 tests | ✅ Completado |
| GA-2 | `ga2-audit-log-webhooks-dlq` | `_hornero_audit_log` + servicio de auditoría (13 tests) + `GET /audit` + instrumentación keys/roles/webhooks/recovery + `GET /webhooks/events` & `POST .../retry` | ✅ Completado |
| GA-3 | `ga3-session-cookie-logout` | Cookie `HttpOnly` `hornero_session` + `POST /auth/logout` + `jti`/`aud` en JWT + fallback cookie en `AuthRequired` | ✅ Completado |
| GA-4 | `ga4-frontend-ga-ux` | Barra Search/Sort en Grid + `thead` sticky, Bulk CSV Import (RFC 4180) & Bulk Delete, pestaña Auditoría ("Actividad"), historial/retry de Webhooks en Settings, `withCredentials` + a11y + 421/421 i18n + lint 0 | ✅ Completado |
| GA-5 | `ga5-openapi-templates-ops` | `docs/openapi.yaml` completo (73 ops / 50 paths), templates v2 con relaciones + CLS + `choices`, scripts backup/restore (Postgres + PocketID), `docs/OPERATIONS.md`, limpieza conectores | ✅ Completado |
| GA-6 | Coordinador (`migrations.go`, `main.go`, `README.md` + Verificación) | Migración `_hornero_audit_log` + reordenamiento `createIndexes()`, rutas `bulk`/`audit`/`webhooks/events`/`retry`/`logout` sin conflictos Gin radix-tree, limpieza final conectores y verificación full-stack | ✅ Completado |

# Plan de Remediación Fase 0 — HorneroDB Production Readiness

**Fecha:** 2026-11-22  
**Alcance aprobado:**
1. Posicionamiento **Security-first self-hosted DB + API/MCP** manteniendo y endureciendo **Webhooks** (recortando vistas tipo Airtable, fórmulas, realtime y storage S3 a v1.1).
2. Modelo de autorización **straightforward en Go** para v1.0 (cerrando todos los SQLi, IDORs, bypasses y cableando validadores + tests; Postgres native RLS pasa a v1.1).
3. Eliminar motor legacy muerto (`_hornero_permissions` / `engine.go`) e implementar niveles reales (`none`/`view`/`manage`/`all`) en `__system__`.

## Progress

| Track | Subagente (`MiniMax-M3.1-Flash · thinking`) | Alcance Principal | Estado |
|---|---|---|---|
| 1 | `fix-security-auth-setup` | Recovery QR takeover, Setup lock/token, API keys/Roles IDORs, OIDC fail-closed/PKCE/`Save`, CSRF/RateLimit/Cookies/ValidateColumn/Slog | ✅ Completado |
| 2 | `fix-permissions-legacy` | Eliminar legacy `_hornero_permissions` & `engine.go`, `__system__` (`view`/`manage`/`all`), precedencia `*` vs tabla + `"inherit"`, tests unitarios | ✅ Completado |
| 3 | `fix-data-engine-ddl` | SQLi en `expand` & DDL, IDORs tablas/columnas, RLS en `DELETE`, CLS simetría read/write, outbox en `tx`, `updated_at`, índices únicos, pool DB, `pagination_test` | ✅ Completado |
| 4 | `fix-webhooks-mcp` | Webhooks (outbox atómico, claim transaccional, sweeper, HMAC, SSRF en Update/redirects/DialContext, backoff exponencial), MCP (`role`, tenant isolation, `resources/list`, OAuth PKCE, SSE races) | ✅ Completado |
| 5 | `fix-frontend-p0-lint` | 54 errores ESLint → 0, manejo global 401, paginación `meta.count`, permisos CLS en grid + UI `__system__` + fix `inherit`, UI de Webhooks en Settings, tipos `select`, cleanup `SetupCheck` | ✅ Completado |
| 6 | `fix-sre-docker-ci-docs` | `config.go` fail-fast/`TRUSTED_PROXIES`/`IsProduction`, `Dockerfile` non-root + `HEALTHCHECK`, `docker-compose.yml` fixes, `.github/workflows/ci.yml`, `README.md` & `docs/SECURITY.md` honestos | ✅ Completado |
| 7 | Coordinador (`main.go`, `auth.go` + Verificación) | Graceful shutdown, `SetTrustedProxies`, CORS allowlist, `/ready`, JSON `slog`, wiring `Validate*Access`, rutas `__system__`, `GetMyPermissions` sobre `_hornero_roles`, 404 JSON `/api/*`, verificación full-stack | ✅ Completado |

## Próximo paso (Fase 1 — v1.0 GA Launch)
- Tabla `_hornero_audit_log` + `GET /audit` + vista "Actividad"
- Filtros/orden/búsqueda server-side (`?filter`, `?order`, `?search` para `RelationPicker` y grid)
- JWT a cookie `HttpOnly` + refresh con rotación
- OpenAPI 3.0 único y completo (~70 rutas) + scripts de backup/restore (`pg_dump` + `pocket-id.db`)
- Virtualización de grid (`@tanstack/react-virtual`) y `@tanstack/react-query`

# HorneroDB — Visión, Arquitectura y Estado Actual

**Última actualización:** 2026-11-23  
**Versión / Baseline:** `v1.0 GA` (rama `main`, listo para producción single-node / self-hosted)

---

## 1. La Idea (Por qué existe HorneroDB)

Hoy, cuando construís una app (a mano o *vibecodeando* con IA) o un CRM interno para una PyME, chocás siempre con el mismo dilema:
- Las plataformas tipo Airtable / NocoDB / Supabase bloquean el **SSO (OIDC)** o los **permisos granulares** detrás de planes *Enterprise*, o te obligan a escribir SQL/políticas a mano para cada endpoint.
- Los backends rápidos (PocketBase) son excelentes en DX, pero no tienen permisos estilo **Dataverse** (tabla + columna por operación + fila) ni PostgreSQL nativo ni protocolo MCP seguro de fábrica.

**HorneroDB** nace para cubrir ese hueco: un **backend PostgreSQL self-hosted y security-first** (Go + React opcional) que combina:

1. **Seguridad sin "SSO Tax":** autenticación humana vía OIDC (BYOIdP, probado end-to-end con **Pocket ID v2** + Passkeys) y **API Keys** con rate-limit y restricciones de dominio/referer para clientes programáticos.
2. **Permisos estilo Microsoft Dataverse sobre PostgreSQL real:**
   - **Nivel tabla / fila (RLS en capa de aplicación):** `all`, `own` (`created_by`) y `none`.
   - **Nivel columna por operación (CLS):** control explícito de qué columnas se pueden leer, crear y actualizar, validado también en filtros, ordenamiento, búsqueda, OData y `expand` (sin oráculos de datos).
   - **Nivel sistema (`__system__`):** delegación granular (`none`, `view`, `manage`, `all`) sobre `tables`, `roles`, `api_keys`, `webhooks`, `settings` y `mcp`.
3. **Superficie moderna para apps e IA:** API REST completa (70+ endpoints en [`docs/openapi.yaml`](openapi.yaml)), consultas **OData** (`$filter`, `$orderby`, `$select`, `$top`, `$skip`, `$expand`), campos `autonumber`, operaciones `bulk` atómicas y un **servidor MCP** autenticado (Streamable HTTP, SSE y stdio con OAuth2 PKCE S256).
4. **Webhooks confiables de primera clase:** outbox transaccional en PostgreSQL, reintentos con backoff exponencial, firma HMAC-SHA256 (`X-Hornero-Signature`), payloads filtrados por permisos del creador y protección SSRF (DNS pinning + re-validación en redirects).
5. **Observabilidad end-to-end abierta (100% opt-in):** trazabilidad distribuida **Web Consumer / UI ↔ HorneroDB ↔ Pocket ID / PostgreSQL / Webhooks** usando **OpenTelemetry** (W3C `traceparent` + OTLP) y **OpenObserve** (binario único en Rust + Parquet + Browser RUM nativo), sin atarse a ningún proveedor.

---

## 2. Arquitectura Actual

```text
[Web Consumer / React UI] ──(Browser RUM + W3C traceparent)──▶ [OpenObserve :5080 (UI/RUM/Parquet)]
         │                                                                   ▲
         │ HTTPS + Cookie HttpOnly / Bearer + traceparent                    │ OTLP gRPC :5081
         ▼                                                                   │ (Basic Auth + org)
[HorneroDB Server (Go / Gin / GORM)] ──(OTLP gRPC :4317)───┐                 │
   ├── Auth OIDC (PKCE S256 + otelhttp) ──▶ [Pocket ID v2] ┼──▶ [otel-collector-contrib]
   ├── Data / DDL / OData / Audit (otelgorm) ▶ [Postgres 18] ◀─┤      ▲ (receiver postgresql)
   └── Webhook Worker (Outbox + Span Links) ──▶ Destinos       │      │
                                                               │  [Host prod Ubuntu (opcional)]
                                                               │  otelcol-contrib systemd
                                                               │  (hostmetrics + docker_stats → 127.0.0.1:4317)
```

---

## 3. Estado Actual por Subsistema

| Área | Estado | Qué está implementado y verificado hoy |
|---|---|---|
| **Autenticación y Sesiones** | ✅ Listo | OIDC PKCE S256 (RFC 7636 sin padding), verificación fail-closed de ID Token/JWKS, cookie de sesión `HttpOnly` (`hornero_session`), `POST /api/v1/auth/logout`, setup inicial con advisory lock + `HORNERO_SETUP_TOKEN`, `upsertUserProfile` que preserva `is_instance_admin`, y resolución viva de rol/admin en `GET /api/v1/auth/me` y `GET /api/v1/auth/permissions?workspace_id=...`. |
| **Permisos (RBAC / CLS / RLS / `__system__`)** | ✅ Listo | Motor unificado sobre `_hornero_roles` (eliminado el motor legacy `_hornero_permissions`/`engine.go`). Validación de acceso por workspace (`ValidateTableAccess`, `ValidateColumnAccess`, `ValidateAPIKeyAccess`), CLS simétrico en lectura/escritura/filtros/sort/OData/`expand`, RLS `own` en `SELECT`/`UPDATE`/`DELETE`, y gating `__system__` (`view` vs `manage`/`all`) en backend y UI (`TableView` + `Settings`). |
| **Motor de Datos, DDL y Queries** | ✅ Listo | Tablas y columnas dinámicas sobre PostgreSQL 18 en transacción con metadata, validación/quoting de identificadores, `autonumber` secuencial atómico, paginación con `meta` real (`count`, `limit`, `offset`, `has_more`), filtros/orden/búsqueda (`?filter`, `?order`, `?search`), soporte OData (`$filter`, `$orderby`, `$select`, `$top`, `$skip`), `POST`/`DELETE /data/:slug/bulk` atómicos y pre-flight de slugs duplicados antes de índices únicos. |
| **Webhooks y Auditoría** | ✅ Listo | Outbox transaccional (`_hornero_webhook_events`) con lease/reaper, backoff exponencial + jitter, firma HMAC-SHA256, protección SSRF (`safeDialContext` + validación de redirects), propagación de `traceparent` + Span Links, historial/reintento en UI (`GET /webhooks/events`, `POST .../retry`), y tabla `_hornero_audit_log` + `GET /audit` + pestaña Actividad. |
| **Seguridad HTTP** | ✅ Listo | CSRF estricto por parseo de esquema+host (`isAllowedOrigin`), CORS allowlist con `traceparent`/`tracestate`/`X-Request-ID`, envelopes canónicos `response.APIResponse` en errores (`401`/`403`/`429`), rate limiter por workspace (default `300 req/min` para sesiones interactivas UI, `60 req/min` para API keys/anónimos, `Retry-After: 60`), headers de seguridad y fail-fast en producción (`JWT_SECRET ≥ 32`, `DB_PASSWORD`, `DB_SSLMODE`). |
| **Servidor MCP (IA)** | ✅ Listo | Transportes Streamable HTTP, SSE (sin corte prematuro por `WriteTimeout`) y stdio; OAuth2 PKCE + discovery; aislamiento por workspace y aplicación de permisos de fila/columna. |
| **Frontend React (UI)** | ✅ Listo | Grilla tipo Excel (edición inline, atajos, relaciones con `RelationPicker` debounced, búsqueda/orden server-side, exportación CSV determinista con tiebreaker `id`, importación CSV RFC 4180 en lote, borrado masivo), fetch separado de metadata vs records en `TableView`, permisos cargados desde `/api/v1/auth/permissions`, 13 idiomas sincronizados (`422` keys exactas), 0 errores de ESLint. |
| **Observabilidad (OTel + OpenObserve)** | ✅ Listo | Instrumentación 100% opt-in (`OTEL_EXPORTER_OTLP_ENDPOINT`): `otelgin`, `otelhttp` hacia Pocket ID, `otelgorm` (`WithoutQueryVariables()` en prod), `WithContext(ctx)` en `data.Service`, correlación `trace_id`/`span_id` en logs JSON (`slog`) + `X-Request-ID`, `@openobserve/browser-rum` + `browser-logs` inicializados por config runtime (`GET /api/v1/telemetry/config`, sin rebuildear imagen) + fallback W3C `traceparent` en Axios, y stack Docker listo en [`deploy/observability/openobserve/`](../deploy/observability/openobserve/). |
| **Infraestructura, CI y Ops** | ✅ Listo | Imagen Docker multi-stage non-root con `HEALTHCHECK` y `/ready`, graceful shutdown (drain HTTP + workers + flush OTel), CI unificado en [`.github/workflows/ci.yml`](../.github/workflows/ci.yml), scripts de backup/restore (`scripts/backup.sh`, `scripts/backup.ps1`, `scripts/restore.sh`) y runbook completo en [`docs/OPERATIONS.md`](OPERATIONS.md). |

---

## 4. Verificación Ejecutada

- **Backend Go:** `go build ./...`, `go vet ./...` y `go test -count=1 ./...` pasando en todos los paquetes (`config`, `handlers/api`, `handlers/mcp`, `middleware`, `query`, `response`, `services/audit`, `services/auth`, `services/data`, `services/permission`, `telemetry`, `workers`).
- **Frontend UI:** `npx eslint .` (0 errores, 0 warnings), `npm run build` exitoso y paridad i18n verificada en 13 locales (`422` keys cada uno).
- **E2E Live (Docker + Tailscale + OpenObserve):**
  - 5 contenedores activos y sanos (`hornerodb-server`, `hornerodb-postgres`, `hornerodb-pocket-id`, `hornerodb-openobserve`, `hornerodb-otel-collector` v`0.165.0`).
  - Flujo OIDC completo con Pocket ID v2 + creación/borrado de columnas y registros + OData + rechazo CSRF con envelope canónico.
  - Traza W3C real (`traceparent`) verificada en OpenObserve enlazando el span HTTP de Gin con sus child spans SQL de GORM/PostgreSQL, junto con trazas de `pocket-id`, 19 streams de métricas de PostgreSQL 18 y 25 streams de métricas de runtime/HTTP de Go.

---

## 5. Limitaciones Conocidas (v1.0) y Roadmap (v1.1+)

Para operar con transparencia total, estos son los ítems que quedan explícitamente fuera de v1.0 y priorizados para **v1.1+**:

1. **Revocación server-side de Bearer JWTs al hacer logout:** `POST /api/v1/auth/logout` limpia las cookies `HttpOnly`, pero tokens Bearer entregados vía `?token=` permanecen válidos hasta expirar (`24h`) aunque ya emiten `jti`. Falta deny-list (DB/Redis).
2. **Rate limiting distribuido:** el rate limiter actual es en memoria por proceso (suficiente para despliegues single-container; multi-réplica requiere Redis).
3. **Migración automática de roles legacy con `*` wildcard:** en v1.0 una regla explícita por tabla tiene precedencia sobre `*` salvo que use `"inherit"`; en despliegues previos a v1.0 conviene revisar roles custom con reglas parciales.
4. **CORS preflight para `allowed_origins` custom por workspace:** el CORS global ahora es fail-closed (`CORS_ORIGINS ∪ AdminURL ∪ PublicURL`); si un cliente browser de API key usa un origen distinto, debe incluirse también en `CORS_ORIGINS` hasta unificar preflight dinámico por workspace.
5. **Retención automática de `_hornero_audit_log`:** hoy crece sin rotación automática; en instancias de alto volumen conviene programar purga/particionado.
6. **Concurrencia alta de sesiones MCP SSE vs pool DB:** el pool está fijado en `25` conexiones máximas; si se esperan >20 sesiones SSE simultáneas conviene dimensionarlo.
7. **Bundle frontend:** el build produce un chunk principal único (~1 MB sin code-splitting); pendiente lazy-loading de rutas (`Settings`, `InstanceAdmin`, `CsvImportWizard`).
8. **Roadmap funcional v1.1:** conectores OIDC de Entra ID y Keycloak, constructor visual de condiciones RLS / PostgreSQL native RLS, storage S3 para adjuntos y Realtime liviano (SSE + `LISTEN/NOTIFY`).

---

## 6. Documentación Relacionada

- [`README.md`](../README.md) — Introducción, quick start y modelo de permisos.
- [`docs/SECURITY.md`](SECURITY.md) — Contrato de seguridad, amenazas cubiertas y webhooks HMAC/SSRF.
- [`docs/OPERATIONS.md`](OPERATIONS.md) — Despliegue en producción, backups/restore, upgrade de Postgres 18, `TRUSTED_PROXIES` y guía paso a paso de OpenTelemetry + OpenObserve (Docker y agente `systemd` en Ubuntu).
- [`docs/openapi.yaml`](openapi.yaml) — Especificación OpenAPI 3.0 completa.

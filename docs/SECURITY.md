# Security Documentation

This document describes the security features and configurations of HorneroDB **as they are implemented today** (v1.0), plus what is explicitly planned for later versions.

## Scope and honesty

The table below is the reference for this document. Anything not listed as implemented is **not** implemented — no section below claims a feature that does not exist in the code.

| Area | Status in v1.0 |
|---|---|
| JWT authentication (HMAC-SHA256) | ✅ Implemented |
| API keys (SHA-256 hashed, `key_` prefix) | ✅ Implemented |
| OIDC SSO with Pocket ID | ✅ Implemented and tested |
| OIDC SSO with Entra ID / Keycloak | ⏳ Planned for v1.1 |
| Passkeys / WebAuthn | ❌ Not implemented (use your IdP's passkey support) |
| Table-level permissions (`all` / `own` / `none`) | ✅ Implemented |
| Column-level permissions per operation | ✅ Implemented |
| Row-level security by owner (`created_by`) | ✅ Implemented (application layer) |
| Row-level custom conditions / visual rule builder | ⏳ Planned for v1.1 |
| PostgreSQL-native RLS | ⏳ Planned for v1.1 |
| `__system__` permissions (`none`/`view`/`manage`/`all`) | ✅ Implemented |
| CSRF protection (Origin validation) | ✅ Implemented |
| CORS allowlist (no `*` by default) | ✅ Implemented |
| Trusted proxies (`TRUSTED_PROXIES`) | ✅ Implemented |
| Initial setup token (`HORNERO_SETUP_TOKEN`) | ✅ Implemented |
| Production fail-fast validation | ✅ Implemented |
| Webhooks: transactional outbox + retries | ✅ Implemented |
| Webhooks: HMAC signature (`X-Hornero-Signature`) | ✅ Implemented |
| Webhooks: SSRF protection | ✅ Implemented |
| Rate limiting (per server / per API key) | ✅ Implemented (in-memory) |
| Structured logging | ✅ Implemented |
| Persistent, queryable audit trail (`audit_log`) | ⏳ Planned for v1.0 GA |

---

## Authentication

HorneroDB has an intentionally minimal and robust security design with **two official authentication methods**:

### Methods Supported

| Method | Use Case | Implementation |
|--------|----------|----------------|
| **OIDC/SSO** | Human Users (Web UI & MCP OAuth) | External Identity Provider (Pocket ID tested; Entra ID and Keycloak planned for v1.1). Issues signed HMAC-SHA256 JWT session cookies/tokens upon login. |
| **API Keys** | Programmatic, Services & AI Agents | SHA-256 hashed Bearer tokens (prefix `key_`) with origin, role, and rate-limit restrictions. |

> **Passkeys**: HorneroDB does not implement WebAuthn/passkeys. If you need them, use the passkey support of your identity provider (Pocket ID and other OIDC providers) — the login happens there, not in HorneroDB.

> [!NOTE]
> JWT is the internal session token format created upon successful OIDC/SSO authentication. It is not a separate authentication option.

### OIDC & JWT Session Configuration

**Development**:
- If `JWT_SECRET` is not set, a random 32-character secret is auto-generated
- Warning displayed in logs
- The generated secret is **stable for the lifetime of the process** (it is cached), so it does not change on every request — only a restart invalidates sessions
- Sessions don't persist across restarts (by design)

**Production**:
```bash
# Required
JWT_SECRET=your-super-secret-minimum-32-characters

# Validation (server refuses to start otherwise):
# - Must be set
# - Must be >= 32 characters
# - Cannot be the default value "change-me-in-production"
```

### API Keys

```json
{
  "name": "Production API",
  "allowed_origins": ["https://myapp.com"],
  "rate_limit_per_minute": 60,
  "role_id": "uuid-of-role"
}
```

Security features:
- Keys stored as SHA-256 hashes (not plaintext)
- Only displayed once at creation
- Can be rotated without downtime
- Origin restrictions per key
- API-key authenticated requests are exempt from CSRF checks (they are not attached automatically by browsers)

---

## Authorization

### Permission Levels

#### 1. Table-Level
```json
{
  "appointments": {
    "create": "all",    // Can create records
    "read": "all",      // Can read all records
    "update": "own",    // Can only update own records
    "delete": "none"    // Cannot delete
  }
}
```

Values: `all`, `own`, `none`

#### 2. Column-Level (per operation)
```json
{
  "appointments": {
    "columns": {
      "read": ["date", "time", "status"],
      "create": ["customer_name", "phone"],
      "update": ["status"]
    }
  }
}
```

Special value: `["*"]` = all columns

#### 3. Row-Level Security (owner based)

Applied automatically when `access` is `"own"`:
```sql
-- Behind the scenes
SELECT * FROM table WHERE created_by = 'current-user-id'
```

This is enforced in the application layer. Custom conditions (e.g. `status = 'open'`) and PostgreSQL-native RLS are planned for v1.1 — they are **not** available in v1.0.

#### 4. System Permissions
```json
{
  "__system__": {
    "mcp": "all",        // MCP server access
    "webhooks": "manage", // Webhook management
    "api_keys": "view",   // API key visibility
    "roles": "manage",    // Role management
    "tables": "all"       // Table schema changes
  }
}
```

Values: `none`, `view`, `manage`, `all`

---

## Data Protection

### SQL Injection Prevention

All dynamic table names are validated:

```go
// Validation regex
^[a-z][a-z0-9_]*$           // Slugs
^data_[0-9a-f-]{36}_[a-z]   // Full table names
```

Example safe table creation:
```go
safeTableName, err := ValidateTableName(workspaceID, slug)
if err != nil {
    return error // Reject invalid names
}
// Now safe to use in SQL
```

### Column Security Filtering

Automatic filtering based on permissions:

```go
// User requests all columns
// System only returns allowed columns
allowedColumns := permService.GetColumnsForOperation(wsID, roleName, tableSlug, "read")

// Filter record
for key := range record {
    if !allowedMap[key] {
        delete(record, key)  // Remove restricted column
    }
}
```

### Input Validation

- Workspace IDs: Must be valid UUID format
- Table slugs: Only lowercase, numbers, underscores
- Record IDs: UUID validation
- String lengths: Maximum limits enforced

---

## Transport Security

### CORS (Cross-Origin Resource Sharing)

**Default Behavior**:
- Only allows requests from `HORNERO_ADMIN_URL`
- Same-origin policy by default (never `*`)

**Configuration**:
```bash
# Single origin (default)
HORNERO_ADMIN_URL=https://admin.mydomain.com

# Multiple origins
CORS_ORIGINS=https://admin.mydomain.com,https://api.mydomain.com
```

> ⚠️ `HORNERO_ADMIN_URL` is a **security boundary**: it is used both as the CORS allowlist and as the allowed origin for the CSRF protection of state-changing requests. In production, always point it to your real admin origin.

**Workspace-Level CORS**:
Each workspace can define additional `allowed_origins` for its API clients.

### CSRF Protection

State-changing requests (`POST`, `PUT`, `PATCH`, `DELETE`) are validated against the allowed origins (the `Origin` header must match `HORNERO_ADMIN_URL`/`CORS_ORIGINS`, or the request must be same-origin). Requests authenticated with an API key (`Authorization: Bearer key_...`) are exempt.

### Security Headers

Automatically added to all responses:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` (in production or when the request is HTTPS)

### HTTPS

**Production Requirement**:
Always use HTTPS in production. The server respects `X-Forwarded-Proto` headers for proper URL generation behind reverse proxies.

### Trusted Proxies

`X-Forwarded-For` / `X-Forwarded-Proto` are only trusted from proxies you declare explicitly:

```bash
# .env — comma-separated IPs or CIDR ranges
TRUSTED_PROXIES=172.16.0.0/12,10.0.0.1
```

If the variable is empty, the list is empty and **nobody** is trusted. The parsed list is exposed as `Config.Server.TrustedProxies` and applied to the HTTP server at startup.

---

## Deployment Security (12-factor)

All configuration comes from the environment. There are no configuration files, no embedded credentials and no baked-in secrets in the image.

### Environment variables

| Variable | Required | Description |
|---|---|---|
| `HORNERO_ENV` | – | `development` / `staging` / `production`. `production` enables the embedded UI and the fail-fast validation below. |
| `ENV` / `NODE_ENV` | – | Alternative environment signals (same effect as `HORNERO_ENV`). |
| `SERVER_PORT` | – | HTTP port (default `8080`). |
| `HORNERO_ADMIN_URL` | ✅ prod | Admin origin; used for CORS **and** CSRF. |
| `CORS_ORIGINS` | – | Comma-separated extra allowed origins. |
| `TRUSTED_PROXIES` | – | Comma-separated proxies allowed to set forwarding headers. |
| `HORNERO_SETUP_TOKEN` | recommended | Shared secret required to complete the initial setup. |
| `JWT_SECRET` | ✅ prod | ≥ 32 characters, not the default value. |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | ✅ | PostgreSQL connection. |
| `DB_SSLMODE` | ✅ prod | `disable` is rejected in production unless explicitly allowed. |
| `DB_ALLOW_INSECURE_SSL` | – | `true` allows `DB_SSLMODE=disable` (internal networks only). |
| `DB_ALLOW_INSECURE_DB` | – | `true` allows the default/empty `DB_PASSWORD` (internal networks only). |
| `POCKETID_*` | – | OIDC client credentials, issuer and redirect URLs. |
| `S3_*` | – | Optional S3-compatible object storage. |

### Production fail-fast

With `HORNERO_ENV=production` the server refuses to start when:

| Check | Requirement | Escape hatch |
|---|---|---|
| `JWT_SECRET` | Set, ≥ 32 chars, not the default value | – |
| `DB_PASSWORD` | Not empty and not `postgres` | `DB_ALLOW_INSECURE_DB=true` |
| `DB_SSLMODE` | Not `disable` | `DB_ALLOW_INSECURE_SSL=true` |

The escape hatches exist for internal/trusted networks — for example the bundled Docker Compose stack, where PostgreSQL is not published outside the Compose network. **Never enable them on an internet-facing deployment.**

### Container hardening

The image built from the repository `Dockerfile`:

- Multi-stage build: the UI is compiled with `npm ci` (lockfile-pinned) and the Go binary is compiled with `CGO_ENABLED=0`
- Final image contains only the static binary, `ca-certificates`, `tzdata` and `wget`
- Runs as a dedicated **non-root** user (`hornero`), never as `root`
- `GIN_MODE=release` (no debug output)
- `HEALTHCHECK` probing `GET /health` every 30s (5s timeout, 10s start period, 3 retries)
- No `apk upgrade` at build time (non-deterministic builds are avoided)

`docker-compose.yml` additionally provides `restart: unless-stopped`, health checks for every service and correct host port mapping for PostgreSQL (`${DB_EXTERNAL_PORT:-5432}:5432`).

### Continuous Integration

`.github/workflows/ci.yml` runs on every push to `main` and every pull request:

1. `npm ci` + `npm run lint` + `npm run build` for the UI (required, because the Go binary embeds `web/ui/dist` via `go:embed`)
2. `go vet ./...`
3. `go test -race ./...`
4. `go build ./cmd/server`
5. A separate job builds the hardened Docker image (no push)

---

## Audit & Monitoring

### Logging

Structured logging with `slog`:

```go
slog.Info("record created",
    "user_id", userID,
    "workspace_id", workspaceID,
    "table", tableSlug,
    "record_id", recordID,
)
```

**Never logged**:
- JWT tokens
- API key secrets (only prefix)
- Passwords
- OIDC tokens

**Always logged**:
- Authentication attempts (success/failure)
- Permission denials
- Database errors (sanitized)
- Webhook deliveries

> **Audit trail status**: today this is *structured logging* to stdout. A persistent, queryable audit trail stored in the database (table `audit_log`, with retention policies and export) is on the roadmap for the v1.0 GA release. Do not assume the logs are a compliance-grade audit trail yet.

### Rate Limiting

**Current Implementation** (per-server):
```go
// In-memory rate limiting
// Note: For multi-server deployments, use Redis (see FUTURE_IMPROVEMENTS.md)
```

**API Key Rate Limits**:
```json
{
  "rate_limit_per_minute": 60,
  "rate_limit_per_hour": 1000
}
```

### Webhook Security

**Delivery guarantees**:
- Outbox pattern: events are queued in `_hornero_webhook_events` inside the same transaction that mutates the record
- At-least-once delivery with retries and exponential backoff (15s × attempt, 5 attempts max)
- The payload is filtered with the **creator's** read permissions (row-level and column-level) before delivery

**Headers sent on every delivery**:

| Header | Value |
|---|---|
| `X-Hornero-Delivery` | Unique delivery id (UUID of the outbox event) — use it to deduplicate. |
| `X-Hornero-Signature` | `t=<unix_timestamp>,v1=<hex digest>` — HMAC-SHA256 signature (only present when the webhook has a `client_state`). |
| `User-Agent` | `HorneroDB-Webhooks/1.0` |

**Signature verification** (receiver side):

```
signed_payload = "<t>." + <raw request body>
expected      = hex(HMAC_SHA256(key = client_state, message = signed_payload))
header        = "t=" + <t> + ",v1=" + expected
```

The signature covers the timestamp, so a captured delivery cannot be replayed indefinitely — compare `t` with the current time and reject stale deliveries.

**SSRF protection**:
- Only `http` and `https` schemes are accepted; `http` is rejected in production
- URLs with embedded credentials are rejected
- Private/loopback/link-local addresses are resolved and rejected
- The URL is validated **again right before the delivery**, because it could have changed since the event was queued
- Redirects are re-validated

---

## MCP (Model Context Protocol) Security

The MCP server has the same security as the REST API:

### Authentication
- API Key: `Authorization: Bearer key_xxxxx`
- OAuth2: JWT tokens via standard flow (PKCE)

### Authorization
- All MCP tools check permissions
- Row-level security enforced
- Column-level security enforced
- Workspace isolation

### Example Secure Configuration
```json
{
  "__system__": {
    "mcp": "all"
  },
  "customers": {
    "read": ["name", "email"],  // AI only sees name and email
    "create": "none",           // AI cannot create customers
    "update": "none"            // AI cannot update customers
  }
}
```

---

## Security Checklist

### Deployment

- [ ] Change default JWT secret (production)
- [ ] Set a strong `DB_PASSWORD` (or explicitly opt out with `DB_ALLOW_INSECURE_DB=true` on internal networks only)
- [ ] Use TLS for the database connection (`DB_SSLMODE=require` or stricter)
- [ ] Configure CORS with specific origins (not `*`) and point `HORNERO_ADMIN_URL` to your real admin origin
- [ ] Enable HTTPS
- [ ] Set `HORNERO_SETUP_TOKEN` before the first setup
- [ ] Declare `TRUSTED_PROXIES` if you run behind a reverse proxy
- [ ] Configure firewall rules
- [ ] Enable structured logging

### Configuration

- [ ] Create dedicated API keys for each service
- [ ] Set appropriate rate limits per key
- [ ] Configure workspace `allowed_origins`
- [ ] Review role permissions (principle of least privilege)
- [ ] Set a `client_state` on webhooks so deliveries are signed

### Monitoring

- [ ] Monitor authentication failure rates
- [ ] Set up alerts for permission denials
- [ ] Review logs for suspicious activity
- [ ] Monitor webhook delivery rates

---

## Vulnerability Disclosure

If you discover a security vulnerability:

1. **DO NOT** open a public issue
2. Report it privately to the maintainers (contact details to be published — placeholder)
3. Include:
   - Description of vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

Please allow up to 48 hours for an initial response.

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [CORS Specification](https://fetch.spec.whatwg.org/#cors-protocol)
- [RFC 2104 — HMAC](https://datatracker.ietf.org/doc/html/rfc2104)

---

**Last Updated**: 2026-11-22
**Version**: 1.0

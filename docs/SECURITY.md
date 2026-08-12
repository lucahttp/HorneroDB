# Security Documentation

This document describes the security features and configurations of HorneroDB.

## Overview

HorneroDB implements defense-in-depth security with multiple layers:

1. **Authentication** - Who are you?
2. **Authorization** - What can you do?
3. **Data Protection** - Row and column-level security
4. **Transport Security** - CORS, HTTPS, secure headers
5. **Audit & Monitoring** - Logging and rate limiting

---

## Authentication

HorneroDB has an intentionally minimal and robust security design with **two official authentication methods**:

### Methods Supported

| Method | Use Case | Implementation |
|--------|----------|----------------|
| **OIDC/SSO** | Human Users (Web UI & MCP OAuth) | External Identity Provider (PocketID, Google, EntraID, Keycloak). Issues signed JWT session cookies/tokens upon login. |
| **API Keys** | Programmatic, Services & AI Agents | SHA-256 hashed Bearer tokens (prefix `key_`) with origin, role, and rate-limit restrictions. |

> [!NOTE]
> JWT is the internal session token format created upon successful OIDC/SSO authentication. It is not a separate authentication option.

### OIDC & JWT Session Configuration

**Development**:
- If `JWT_SECRET` not set, auto-generates random 32-char secret
- Warning displayed in logs
- Sessions don't persist across restarts (by design)

**Production**:
```bash
# Required
JWT_SECRET=your-super-secret-minimum-32-characters

# Validation:
# - Must be set (server won't start without it)
# - Must be >= 32 characters
# - Cannot be default value "change-me-in-production"
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

#### 3. Row-Level Security
Applied automatically when `access` is `"own"`:
```sql
-- Behind the scenes
SELECT * FROM table WHERE created_by = 'current-user-id'
```

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
- Same-origin policy by default

**Configuration**:
```bash
# Single origin (default)
HORNERO_ADMIN_URL=https://admin.mydomain.com

# Multiple origins
CORS_ORIGINS=https://admin.mydomain.com,https://api.mydomain.com
```

**Workspace-Level CORS**:
Each workspace can define additional `allowed_origins` for their API clients.

### Security Headers

Automatically added to all responses:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Referrer-Policy: strict-origin-when-cross-origin`

### HTTPS

**Production Requirement**:
Always use HTTPS in production. The server respects `X-Forwarded-Proto` headers for proper URL generation behind reverse proxies.

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

**Delivery Guarantees**:
- Outbox pattern: Events queued in database
- At-least-once delivery with retries
- Exponential backoff: 15s × attempt_number
- Max 5 attempts before marking failed

**Verification**:
- `ClientState` header for webhook validation
- HTTPS required for webhook URLs in production

---

## MCP (Model Context Protocol) Security

The MCP server has the same security as the REST API:

### Authentication
- API Key: `Authorization: key_xxxxx`
- OAuth2: JWT tokens via standard flow

### Authorization
- All 8 tools check permissions
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
- [ ] Configure CORS with specific origins (not `*`)
- [ ] Enable HTTPS
- [ ] Set up PostgreSQL with SSL
- [ ] Configure firewall rules
- [ ] Enable structured logging

### Configuration

- [ ] Create dedicated API keys for each service
- [ ] Set appropriate rate limits per key
- [ ] Configure workspace `allowed_origins`
- [ ] Review role permissions (principle of least privilege)
- [ ] Set up webhook `clientState` for verification

### Monitoring

- [ ] Monitor authentication failure rates
- [ ] Set up alerts for permission denials
- [ ] Review logs for suspicious activity
- [ ] Monitor webhook delivery rates

---

## Vulnerability Disclosure

If you discover a security vulnerability:

1. **DO NOT** open a public issue
2. Email: [security@hornerodb.com] (placeholder)
3. Include:
   - Description of vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

Response time: Within 48 hours

---

## Security Updates

### Recent Fixes (Week 1)

1. **SQL Injection Prevention** - Added `ValidateTableName()` with regex validation
2. **Token Logging Removal** - Removed all `fmt.Printf` statements logging tokens
3. **CORS Hardening** - Default to same-origin only, configurable via env
4. **Webhook Race Condition** - Implemented `SELECT FOR UPDATE SKIP LOCKED`
5. **JWT Secret Generation** - Auto-generate random secrets in development
6. **Code Deduplication** - Single `ResolveUserRole()` function instead of 4 copies

### Future Improvements

See `docs/FUTURE_IMPROVEMENTS.md` for:
- Redis-based distributed rate limiting
- Enhanced audit logging
- Automated security scanning

---

## Compliance

### Data Protection

- **GDPR**: Right to deletion implemented via API
- **Data Residency**: Self-hosted = you control data location
- **Encryption**: At-rest (PostgreSQL) + in-transit (HTTPS)

### Access Control

- **RBAC**: Role-based access control implemented
- **ABAC**: Attribute-based (row-level) implemented
- **Audit Trail**: All actions logged with user context

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [PostgreSQL Security](https://www.postgresql.org/docs/current/security.html)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [CORS Specification](https://fetch.spec.whatwg.org/#cors-protocol)

---

**Last Updated**: 2024
**Version**: 1.0

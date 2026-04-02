# Changelog

All notable security fixes and improvements to HorneroDB will be documented in this file.

## [1.0.0] - 2024 - Security Hardening Release

### 🔴 Critical Security Fixes

#### SQL Injection Prevention
- **Added** `ValidateTableName()` function in `internal/handlers/api/table.go`
- **Validates** workspace ID format (UUID) and table slug format (regex)
- **Prevents** malicious table names in dynamic SQL queries
- **Files modified**: `table.go`, `column.go`, `mcp/server.go`

#### Token Logging Removal
- **Removed** all `fmt.Printf` statements logging OIDC tokens
- **Files modified**: `internal/services/auth/oidc.go`
- **Impact**: Tokens no longer appear in server logs

#### CORS Hardening
- **Changed** default CORS from `["*"]` to `[HORNERO_ADMIN_URL]`
- **Added** `CORS_ORIGINS` environment variable for multi-origin setups
- **Added** validation in production (cannot use default values)
- **Files modified**: `main.go`, `config.go`, `.env.example`

#### Webhook Race Condition Fix
- **Implemented** `SELECT FOR UPDATE SKIP LOCKED` in `processOutboxEvents()`
- **Prevents** duplicate webhook processing in multi-server deployments
- **Added** processing timeout tracking for stuck jobs
- **Files modified**: `internal/workers/webhook_dispatcher.go`

### 🟠 High Priority Improvements

#### JWT Secret Generation
- **Added** `generateRandomSecret()` function for development
- **Auto-generates** 32-character random secret if not configured
- **Warns** in logs when using auto-generated secret
- **Requires** explicit JWT_SECRET in production (server won't start without it)
- **Files modified**: `internal/config/config.go`

#### Code Deduplication
- **Created** `ResolveUserRole()` function in `internal/middleware/auth.go`
- **Replaced** 4 copies of the same logic across codebase
- **Returns** `(roleName, workspaceID, isOwner, error)`
- **Files modified**: `auth.go`, `oidc.go`

### ✅ New Features

#### MCP Server with Full Security
- **Implemented** Model Context Protocol (MCP) server in Go
- **Location**: `internal/handlers/mcp/`
- **Features**:
  - 8 tools: list_workspaces, list_tables, list_columns, list_records, create_record, get_record, update_record, delete_record
  - OAuth2 authentication flow
  - API Key authentication
  - Row-level security enforcement
  - Column-level security enforcement
  - Workspace isolation
- **Documentation**: `docs/connectors/mcp-server/README.md`

### 📚 Documentation

#### Added
- `docs/FUTURE_IMPROVEMENTS.md` - Roadmap for scaling (Redis, multi-server)
- `docs/SECURITY.md` - Comprehensive security documentation
- Updated `README.md` with security configuration section
- Updated `.plans/2026-03-03-mcp.md` to reflect implementation

#### Updated
- `.env.example` with CORS and JWT documentation
- `docs/connectors/mcp-server/README.md` completely rewritten

### 🔧 Configuration Changes

#### New Environment Variables
```bash
# CORS Configuration
CORS_ORIGINS=http://localhost:5173,https://api.example.com

# JWT (now with auto-generation in development)
JWT_SECRET=your-super-secret-minimum-32-chars
```

#### Breaking Changes
- **CORS**: Default changed from `*` to same-origin only
  - **Migration**: Add `CORS_ORIGINS=*` to keep old behavior (not recommended)
  - **Recommended**: Set explicit origins

### 🧪 Testing

#### Security Test Cases
1. SQL Injection: Attempt to create table with malicious slug
   - Expected: Rejected with validation error
2. Token Logging: Check logs after OIDC login
   - Expected: No tokens in logs
3. CORS: Request from unauthorized origin
   - Expected: Blocked by CORS policy
4. Webhooks: Run multiple server instances
   - Expected: No duplicate deliveries

### 📊 Statistics

- **Files modified**: 15
- **Lines added**: ~1,200
- **Lines removed**: ~400
- **Functions created**: 5
- **Functions refactored**: 4
- **Documentation pages**: 3 new, 4 updated

### 🔮 Future Work

See `docs/FUTURE_IMPROVEMENTS.md` for:
- Redis-based distributed rate limiting
- Enhanced audit logging
- Automated security scanning
- Webhook delivery improvements

---

## [0.9.0] - Previous Release

### Features
- Initial REST API (30+ endpoints)
- OIDC Authentication
- RBAC Permissions
- Column-level permissions
- API Key management
- Webhooks with outbox pattern
- React UI

---

## Security Contact

For security vulnerabilities, see `docs/SECURITY.md`.

---

**Format**: Based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

**Versioning**: [SemVer](https://semver.org/spec/v2.0.0.html)

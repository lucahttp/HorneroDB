# 🐦 HorneroDB

**Open-source, low-code database built in Go + React.**  
Self-hostable alternative to Airtable, NocoDB, PocketBase and Baserow. Secure by default with Passkeys powered by PocketID SSO.

![Preview](web/ui/inline_edit_verify.png)

## ✨ Features

- **Dynamic Tables & Columns** — create schemas on the fly via UI or API
- **14 Field Types** — Text, Number, Bool, Date, Email, URL, JSON, Relation, File, etc.
- **Excel-like Editing** — Inline cell editing, keyboard navigation, copy/paste
- **Role-Based Access** — Granular permissions per table, column, and row (Dataverse-style)
- **OIDC Authentication** — Integrated with PocketID, EntraID, Keycloak
- **MCP Server** — First-class support for AI agents (Claude, OpenCode, Antigravity)
- **Multi-tenant** — Isolated workspaces for different teams or projects
- **Simplified Deployment** — Warm up database + setup the auth service of your preference and you are ready to go

## 🚀 Quick Start

### Docker

```bash
docker-compose up -d
```

### Manual

```bash
# Backend
go build -o bin/hornerodb ./cmd/server
./bin/hornerodb

# Frontend (dev)
cd web/ui && npm run dev
```

Visit `http://localhost:8090` (default port).

## 📚 API Reference

Comprehensive OpenAPI 3.0 documentation available at [`docs/openapi.yaml`](docs/openapi.yaml).

### Key Endpoints

- `GET /api/v1/workspaces`
- `GET /api/v1/workspaces/:ws/tables`
- `GET /api/v1/workspaces/:ws/data/:slug`
- `POST /api/v1/workspaces/:ws/data/:slug`

## 🛠 Stack

- **Backend:** Go, Gin, GORM, PostgreSQL
- **Frontend:** React, Vite, Framer Motion, Iconoir
- **Auth:** OIDC (PocketID)


## 🗺 Roadmap

- [x] Full REST API (30+ endpoints)
- [x] OIDC Auth & RBAC Permissions
- [x] MCP Server
- [x] Visual Table Editor & Inline Editing
- [x] PocketBase-style Field Types
- [ ] Table Relations UI
- [ ] Filters & Search
- [ ] Webhooks & Automation
- [ ] S3 Attachments

---

Made in Argentina with ❤️

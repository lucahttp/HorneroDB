# 🐦 HorneroDB

### The Open-Source, Low-Code Database: Secure by Default.

**Built with Go + React.** A powerful, self-hostable alternative to Airtable, NocoDB, PocketBase, and Baserow. Designed for developers who need granular security and modern authentication (Passkeys/SSO) without the enterprise price tag.

![Preview](web/ui/inline_edit_verify.png)

## ✨ Features

* **Secure Table Creation** — Define schemas on the fly and control visibility (who sees what) via a sleek UI or a robust API.
* **Rich Data Types** — Fully compatible with PostgreSQL types, giving you the flexibility of a professional relational database.
* **Excel-like Experience** — Inline cell editing, full keyboard navigation, and seamless copy/paste functionality.
* **Granular RBAC** — Role-Based Access Control at the table, column, and row levels. Security is a core feature, not a paid add-on.
  * **Column-level security by operation** — Control which columns can be read, created, updated, or deleted separately.
  * **Row-level security** — Filter data by user ownership or custom conditions.
* **Modern Auth (OIDC)** — Native integration with OIDC providers; optimized for **PocketID** with Passkey support.
* **API Security** — Rate limiting per API key, domain/origin restrictions, and automatic column filtering.
* **Seamless Integrations** — Out-of-the-box support for PowerAutomate, n8n, and React applications via the OpenAPI standard.
* **Simplified Deployment** — Spin up your database, connect your auth service, and you're ready for production.

---

## 🆚 Comparison

| Feature | 🐦 HorneroDB | 🟢 Supabase | 🧊 PocketBase | ⚡ NocoDB | 🧩 Airtable | 🛶 Baserow |
| --- | --- | --- | --- | --- | --- | --- |
| **Permissions** | **Granular (Row/Col)** | PG RLS | Collection | Table | Base | Table |
| **Security** | **SSO + Passkeys** | Enterprise | Email/Pass | Enterprise | Enterprise | Enterprise |
| **API** | **Auto-Secure** | REST/GQL | SDK | REST/GQL | REST | REST |
| **Self-Host** | **Docker** | Docker | Binary | Docker | ❌ | Docker |
| **Cost** | **Free / Community** | Free Tier | Free | Free | $$$ | Free /$$$ |

---

## 🛠 Why HorneroDB?

HorneroDB started with a real-world problem. While building an e-commerce management tool for my girlfriend's shop, I hit a wall with existing solutions. Most tools either lacked the ability to restrict API calls by source/IP or locked basic security features—like **SSO and granular column visibility**—behind expensive "Enterprise" subscriptions.

I needed a system where:

1. **Security is accessible:** My girlfriend can log in with a Passkey (biometrics), so she never has to worry about forgotten passwords.
2. **Granular Control:** Staff can manage inventory without seeing sensitive business data.
3. **Developer Friendly:** A single API to rule them all, without the complexity of traditional ERPs.

HorneroDB bridges the gap between the simplicity of a spreadsheet and the security of a modern cloud-native application.

---

## 🚀 Quick Start

### Play with Docker (One-Click Setup)

Test HorneroDB instantly using the Docker Playground:

[!["Run it in Play with Docker"](misc/playwithdocker.png)](https://labs.play-with-docker.com/?stack=https://raw.githubusercontent.com/lucahttp/HorneroDB/refs/heads/main/docker-compose.yml&stack_name=HorneroDB)

### Prerequisites

* Docker & Docker Compose
* A `.env` file (see `.env.example`)
* *For local development:* Node.js and Go runtimes.

### Deployment (Docker)

```bash
# Clone and prepare environment
cp .env.example .env

# Start HorneroDB, PocketID, and PostgreSQL
docker-compose up -d

```

Visit `http://localhost:5173` to access the UI.

### Manual Setup (Development)

```bash
# Build and run the Backend
go build -o bin/hornerodb ./cmd/server
./bin/hornerodb

# Run the Frontend
cd web/ui && npm install && npm run dev

```

---

## 📚 API Reference

HorneroDB is built on the **OpenAPI 3.0** standard. You can find the full documentation in [`docs/openapi.yaml`]().

**Core Endpoints:**

* `GET /api/v1/workspaces`
* `GET /api/v1/workspaces/:ws/tables`
* `GET /api/v1/workspaces/:ws/data/:slug` (Secure Read)
* `POST /api/v1/workspaces/:ws/data/:slug` (Secure Write)

---

## 🔐 Granular Permissions

HorneroDB provides **fine-grained security** at multiple levels:

### Table-Level Permissions

Control access per role: `all`, `own`, or `none`.

```json
{
  "turnos": {
    "create": "all",
    "read": "all",
    "update": "own",
    "delete": "none"
  }
}
```

### Column-Level Permissions by Operation

Define which columns are visible/editable for each operation (read, create, update, delete):

```json
{
  "turnos": {
    "create": "all",
    "read": "all",
    "update": "all",
    "columns": {
      "read": ["from", "to", "fecha"],
      "create": ["cliente", "email", "telefono", "from", "to", "fecha"],
      "update": ["estado"]
    }
  }
}
```

This allows:
- **Public API keys** to only expose specific columns (e.g., appointment times)
- **Limited write access** (e.g., users can only update their appointment status)

### API Key Security

Each API key can have custom restrictions:

```json
{
  "name": "Public Booking API",
  "role_id": "...",
  "rate_limit_per_minute": 10,
  "allowed_origins": ["https://bookings.example.com"],
  "allowed_referers": ["https://bookings.example.com/"]
}
```

### Use Case: Booking System Demo

See [`demo-reservas/`](demo-reservas/) for a complete example:
- **Public site**: View available slots, create reservations
- **Admin site**: Full access to manage appointments and services
- **Bot integration**: Limited update access (only status field)

---

## 🗺 Roadmap

* [x] Full REST API (30+ endpoints)
* [x] OIDC Auth & RBAC Permissions
* [x] Simple UI
* [x] Visual Table Editor
* [x] Column-level permissions by operation
* [x] API key rate limiting & domain restrictions
* [ ] Table Relations UI (In Progress)
* [ ] Advanced Filters & Search
* [ ] Webhooks & Automations
* [ ] S3 Attachment Support

---

## 🐦 Fun Fact

The **Hornero** is Argentina's national bird. They are master builders, creating incredibly strong nests made of mud and twigs that look like small ovens. Like the bird, **HorneroDB** is built to be a sturdy, reliable home for your data.

**Made in Argentina with ❤️**
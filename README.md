# 🐦 HorneroDB

### Wanna vibecode an e-commerce, or build it by hand?
But... What about the website's data? The actual database? Permissions and access control?

Your *vibecoded* app can have its **data protected just like large enterprises (enterprise grade)** using the highest security standards. That's what **HorneroDB** is for.

**Built with Go + React (Optional).** A CRM / database in the style of Airtable, NocoDB, or PocketBase, but with an absolute focus on security: SSO using OIDC, and table, column, and row-level permissions (Dataverse style).

![Preview](docs/screenshot_dashboard.png)

---

## ✨ Why use HorneroDB

* **BYOIdP Security (Bring Your Own Identity Provider)** — Modern authentication (SSO/OIDC) and native support for Passkeys. Your security shouldn't be locked behind an "Enterprise" plan.
* **PostgreSQL at its core** — Rich data types and the flexibility of a professional relational database.
* **Granular Permissions (APIs and Staff)** — Strict control for your APIs (for your public frontend or *customer-facing* AI agents) and for your users (business staff).
  * *Column-level security per operation* (control who reads, creates, updates, or deletes each field).
  * *Row-level security* (filter data by owner or custom conditions).
* **Excel-like Experience** — Inline cell editing, full keyboard navigation, and seamless copy/paste support.

## 🛠️ Developer Tools

We are building an ecosystem designed for the modern developer:

* **(Upcoming) Hornero MCP** — Plug HorneroDB into the AI you are *vibecoding* with. The AI will understand your backend, configure your database, and program the UI automatically.
* **(Upcoming) Data Security Agent** — A System Prompt for a specialized agent that can help you configure and manage your database security.
* **OpenAPI Collection** — Making it extremely simple to integrate with other platforms (Native support for n8n, PowerAutomate, etc.).
* **Implementation Examples** — Ready-to-use templates (e.g., Booking System, E-commerce).

---

## 💼 Services and Business Model

Designed for small businesses that need to solve their management without complications:

* **Turnkey installation and configuration ($$$)**.
* **You choose where it runs:** On-premise or Cloud. It can be hosted on cloud servers, on a Raspberry Pi, on an old computer running in the back of your shop  or even on a Kubernetes cluster. We accompany your scale.
* **Subscription Plans:** There are no subscription plans, you own your data, your server and your business.

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
# Clone and prepare the environment
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

## 🔐 Security in Detail

HorneroDB provides fine-grained security at multiple levels:

### Table-Level Permissions

Access control per role: `all`, `own`, or `none`.

```json
{
  "appointments": {
    "create": "all",
    "read": "all",
    "update": "own",
    "delete": "none"
  }
}
```

### Column-Level Permissions by Operation

Define which columns are visible/editable for each operation (e.g., the public API only reads certain fields, staff sees everything):

```json
{
  "appointments": {
    "columns": {
      "read": ["from", "to", "date"],
      "create": ["customer", "email", "phone", "from", "to", "date"],
      "update": ["status"]
    }
  }
}
```

### Granular System Permissions (Admin & API)

Beyond data, you can control who manages the system itself using the reserved `__system__` namespace. Ideal for limiting "employee" access or creating specialized API Keys for automations (e.g. n8n):

```json
{
  "__system__": {
    "webhooks": "manage",
    "api_keys": "none",
    "roles": "view",
    "mcp": "all"
  }
}
```

Available actions: `webhooks`, `api_keys`, `roles`, `tables`, `settings`, `mcp`.
Levels: `none`, `view`, `manage`, `all`.

### API Key Security

Each API Key can have specific restrictions. Ideal for your public e-commerce:

```json
{
  "name": "Public E-commerce API",
  "rate_limit_per_minute": 60,
  "allowed_origins": ["https://mydomain.com"]
}
```

---

## 🆚 Comparison

| Feature | 🐦 HorneroDB | 🟢 Supabase | 🧊 PocketBase | ⚡ NocoDB | 🧩 Airtable |
| --- | --- | --- | --- | --- | --- |
| **Permissions** | **Granular (Row/Col)** | PG RLS | Collection | Table | Base |
| **Security** | **SSO + Passkeys (BYOIdP)** | Enterprise | Email/Pass | Enterprise | Enterprise |
| **API** | **Auto-Secure** | REST/GQL | SDK | REST/GQL | REST |
| **Self-Host** | **Docker / Local** | Docker | Binary | Docker | ❌ |
| **Cost** | **Free / Open Source** | Free Tier | Free | Free | $$$ |

---

## 🗺 Roadmap

* [x] Full REST API (30+ endpoints)
* [x] OIDC Auth & RBAC Permissions
* [x] Nice style UI
* [x] Granular column-level permissions
* [x] API key rate limiting & domain restrictions
* [ ] (WIP) Table Relations UI
* [ ] (WIP) MCP Server for AI Assistants
* [ ] Advanced Filters and Search
* [ ] Webhooks and Automations

---

## 🐦 Fun Fact

The **Hornero** is Argentina's national bird. They are master builders, crafting incredibly strong nests made of mud and twigs that look like small ovens. Just like the bird, **HorneroDB** is built to be a solid, secure, and reliable home for your data.

**Made in Argentina with ❤️**

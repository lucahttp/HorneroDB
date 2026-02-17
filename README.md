# 🐦 HorneroDB

Low-code database estilo Airtable / NocoDB / PocketBase / Dataverse, hecho en **Go** + **React**.

## Características

- ✅ Tablas dinámicas (creá tablas y columnas desde la UI o la API)
- ✅ 14 tipos de dato (text, number, boolean, date, datetime, email, url, json, relation, etc.)
- ✅ Edición inline estilo Excel — click para editar, Enter para guardar
- ✅ Selección multiple + borrado masivo
- ✅ Iconos de tipo PocketBase-style en headers
- ✅ Permisos Dataverse-style (tabla, columna, row) con roles
- ✅ Auth OIDC (PocketID, EntraID, Keycloak)
- ✅ API REST completa (30+ endpoints)
- ✅ Documentación OpenAPI (`docs/openapi.yaml`)
- ✅ API Keys por workspace
- ✅ Multi-workspace (multi-tenant)
- ✅ Servidor MCP para agentes IA
- ✅ UI embebida en el binario (producción)
- ✅ Inputs por tipo (date picker, checkbox, textarea, validación email/url)
- ✅ Display inteligente (badges ✓/✗, links clickeables, código para JSON)

## Stack

| Capa | Tecnología |
|------|-----------|
| Backend | Go, Gin, GORM, PostgreSQL |
| Frontend | React, Vite, Axios, Framer Motion |
| Auth | OIDC (PocketID/EntraID/Keycloak) |
| Iconos | Iconoir |
| Docs | OpenAPI 3.0 |

## Requisitos

- Go 1.21+
- Node 18+
- PostgreSQL 15+
- Docker (opcional, para PocketID y PostgreSQL en desarrollo)

## Quick Start

### 1. Clonar y setup

```bash
cd hornerodb
cp .env.example .env
```

### 2. Iniciar PostgreSQL y PocketID

```bash
docker-compose up -d
```

### 3. Compilar y ejecutar

```bash
# Backend
go build -o bin/hornerodb ./cmd/server
./bin/hornerodb

# Frontend (dev)
cd web/ui
npm install
npm run dev
```

### 4. Probar la API

```bash
# Health check
curl http://localhost:8090/health

# Crear workspace (requiere auth)
curl -X POST http://localhost:8090/api/v1/workspaces \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Mi Workspace",
    "slug": "mi-workspace",
    "owner_id": "<user-uuid>"
  }'
```

## API Reference

Documentación completa en [`docs/openapi.yaml`](docs/openapi.yaml).

### Resumen de endpoints

```
# Auth
GET  /api/v1/auth/oidc/login          # Iniciar login OIDC
GET  /api/v1/auth/oidc/callback       # Callback OIDC
GET  /api/v1/auth/me                  # Usuario actual
GET  /api/v1/auth/permissions          # Mis permisos

# Workspaces
GET/POST       /api/v1/workspaces
GET/PUT/DELETE /api/v1/workspaces/:id

# Tablas
GET/POST       /api/v1/workspaces/:ws/tables
GET/PUT/DELETE /api/v1/workspaces/:ws/tables/:id

# Columnas
GET/POST       /api/v1/workspaces/:ws/tables/:t/columns
PUT/DELETE     /api/v1/workspaces/:ws/tables/:t/columns/:id

# Datos (Registros)
GET/POST       /api/v1/workspaces/:ws/data/:slug
GET/PUT/DELETE /api/v1/workspaces/:ws/data/:slug/:id

# Roles (Dataverse-style)
GET/POST       /api/v1/workspaces/:ws/roles
GET/PUT/DELETE /api/v1/workspaces/:ws/roles/:id

# Usuarios y Roles
GET            /api/v1/workspaces/:ws/users
POST/DELETE    /api/v1/workspaces/:ws/users/:uid/role

# Permisos (legacy)
GET/POST       /api/v1/workspaces/:ws/permissions
PUT/DELETE     /api/v1/workspaces/:ws/permissions/:id

# API Keys
GET/POST       /api/v1/workspaces/:ws/keys
DELETE         /api/v1/workspaces/:ws/keys/:id
```

### Tipos de dato soportados

| Tipo | SQL | Descripción |
|------|-----|-------------|
| `text` | VARCHAR(255) | Texto corto |
| `long_text` | TEXT | Texto largo |
| `number` | DECIMAL(10,2) | Número decimal |
| `integer` | INTEGER | Entero |
| `float` | DOUBLE PRECISION | Punto flotante |
| `boolean` | BOOLEAN | Verdadero/Falso |
| `date` | DATE | Fecha |
| `datetime` | TIMESTAMPTZ | Fecha y hora |
| `email` | VARCHAR(255) | Email |
| `url` | VARCHAR(500) | URL |
| `select` | VARCHAR(100) | Selección |
| `relation` | UUID | Relación |
| `json` | JSONB | JSON |
| `file` | JSONB | Archivo |

## Configuración

Ver `.env.example` para todas las variables de entorno.

## Roadmap

### ✅ Completados

- [x] API REST completa (30+ endpoints)
- [x] Motor de permisos (Dataverse-style roles)
- [x] Auth PocketID OIDC
- [x] Servidor MCP
- [x] Frontend UI (React)
- [x] Inline table editing (Excel/PocketBase)
- [x] Selección de filas + borrado masivo
- [x] Edición inline de celdas
- [x] Iconos de tipo de dato PocketBase-style
- [x] 14 tipos de dato GORM
- [x] OpenAPI documentation
- [x] API Keys por workspace
- [x] Multi-workspace (multi-tenant)
- [x] Inputs por tipo (date picker, validación, etc.)
- [x] Display inteligente (badges, links, código)

### 🔲 Pendientes

- [ ] Relaciones (picker UI con búsqueda)
- [ ] Filtros y búsqueda en datos
- [ ] Ordenamiento por columna
- [ ] Paginación visible
- [ ] Vistas alternativas (Calendar, Kanban, Gallery)
- [ ] Dashboard con métricas
- [ ] Drag & drop reordenar columnas
- [ ] Exportar/Importar CSV
- [ ] Webhooks
- [ ] Auditoría / log de cambios
- [ ] Row-level security visual
- [ ] Soporte móvil mejorado
- [ ] Dark mode
- [ ] Archivos/uploads con S3

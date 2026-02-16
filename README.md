# 🐦 HorneroDB

Low-code database estilo Airtable/NocoDB/Dataverse, hecho en Go.

## Características

- ✅ Multi-workspace (multi-tenant)
- ✅ Tablas dinámicas (creá tablas desde la API)
- ✅ Permisos a nivel tabla, columna y row
- ✅ API REST auto-generada
- ✅ Auth OIDC (PocketID, EntraID, Keycloak)
- ✅ S3 para archivos
- ✅ Servidor MCP para agentes IA

## Requisitos

- Go 1.21+
- PostgreSQL 14+
- Docker (opcional, para desarrollo)

## Quick Start

### 1. Clonar y setup

```bash
cd hornerodb
cp .env.example .env
```

### 2. Iniciar PostgreSQL

```bash
docker-compose up -d
```

### 3. Compilar y ejecutar

```bash
go build -o bin/hornerodb ./cmd/server
./bin/hornerodb
```

### 4. Probar la API

```bash
# Health check
curl http://localhost:8080/health

# Crear workspace
curl -X POST http://localhost:8080/api/v1/workspaces \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Salón de Uñas",
    "slug": "salon-unas",
    "owner_id": "00000000-0000-0000-0000-000000000001"
  }'

# Listar workspaces
curl http://localhost:8080/api/v1/workspaces
```

## API Reference

### Workspaces

```
GET    /api/v1/workspaces
POST   /api/v1/workspaces
GET    /api/v1/workspaces/:id
PUT    /api/v1/workspaces/:id
DELETE /api/v1/workspaces/:id
```

### Tablas

```
GET    /api/v1/workspaces/:workspace_id/tables
POST   /api/v1/workspaces/:workspace_id/tables
GET    /api/v1/workspaces/:workspace_id/tables/:table_id
PUT    /api/v1/workspaces/:workspace_id/tables/:table_id
DELETE /api/v1/workspaces/:workspace_id/tables/:table_id
```

### Columnas

```
GET    /api/v1/workspaces/:workspace_id/tables/:table_id/columns
POST   /api/v1/workspaces/:workspace_id/tables/:table_id/columns
PUT    /api/v1/workspaces/:workspace_id/tables/:table_id/columns/:column_id
DELETE /api/v1/workspaces/:workspace_id/tables/:table_id/columns/:column_id
```

### Datos (Registros)

```
GET    /api/v1/workspaces/:workspace_id/data/:table_slug
POST   /api/v1/workspaces/:workspace_id/data/:table_slug
GET    /api/v1/workspaces/:workspace_id/data/:table_slug/:id
PUT    /api/v1/workspaces/:workspace_id/data/:table_slug/:id
DELETE /api/v1/workspaces/:workspace_id/data/:table_slug/:id
```

## Configuración

Ver `.env.example` para todas las variables de entorno.

## Roadmap

- [x] API básica
- [ ] Motor de permisos
- [ ] Auth PocketID OIDC
- [ ] Servidor MCP
- [ ] Frontend UI

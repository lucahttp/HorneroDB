# Changas y Deseos (HorneroDB)

Acá tenemos todo lo que falta, lo que se viene y lo que ya cocinamos. La idea es que no se nos escape nada y que cualquiera pueda investigar de dónde vienen estas ideas.

## 🚩 Prioridad 1 (Lo que hay que liquidar ya)

### UI y Datos
- **Filtros y Búsqueda**: Necesitamos una search bar y filtros por columna en la tabla. Falta tanto en el Front como en el Backend.
    - Para el search queremos usar BM25 full-text search — referencia: [pg_textsearch 1.0: How We Built a BM25 Search Engine](https://pganalyze.com/blog/pg-search-bm25-search-engine-postgres)
- **Ordenamiento**: Poder clickear en el header de la columna y que ordene ASC/DESC. Falta en ambos lados.
- **Paginación visible**: El backend ya la tiene, falta que la UI la muestre ("Mostrando 1-20 de 100").

### Bugs y UX
- **Columna tipo Selección [Bug]**: No muestra nada para generar una selección, aparentemente muere ahí y queda como una columna tipo texto. Hay que arreglar el input y el display.
- **Fix Settings [Error]**: Arreglar ese freeze que se manda cuando vas a settings adentro de un workspace. Sospechamos que es un choque de nombres con los iconos.
- **Responsive Mobile**: Los navbars y las tablas se ven re apretados en el celu.
    - Idea: En mobile, mostrar el selector de idioma y el toggle de theme como iconos o adentro de un menú de "hamburguesa" para que no ocupen tanto espacio.
- **Iconos en Mobile**: En las pantallas de Workspace y Tablas, que se vean solo iconos para ahorrar lugar. El checkbox de "seleccionar todo" y el ID de fila ocupan lugar al cuete en pantallas chicas.

### Infraestructura y Deuda Técnica
- **Rate Limiting con Redis [Escalabilidad]**: El rate limiter actual usa un mapa en memoria (`map[string]*visitor` en `internal/middleware/ratelimit.go`), lo que significa que en multi-servidor cada instancia tiene su propio limitador — los usuarios pueden saltárselo pegando a distintos servidores.
    - **Solución**: Reemplazar con Redis usando `INCR` + `EXPIRE` por key.
    - **Librería**: `github.com/go-redis/redis/v8`
    - **Config nueva**: `REDIS_URL=redis://redis:6379` y `RATE_LIMITER=redis` en el `.env`.
    - **Path de migración**: Agregar Redis al `docker-compose.yml`, crear `internal/ratelimit/redis.go`, hacer que por defecto use memoria y Redis solo cuando se configure.
    - **Esfuerzo estimado**: 2-4 horas.
- **Mover `ResolveUserRole()` al servicio de auth**: Actualmente vive en `internal/middleware/auth.go` como helper suelto. Cuando el servicio de auth crezca, hay que moverlo a `internal/services/auth/role.go` para que tenga caché de lookups y sea más testeable.

## ✨ Estaría Bueno (Ideas a futuro)

- **Gestión de Usuarios con PocketID** _(encaminado, le falta pulido)_: Poder buscar usuarios que ya existen en PocketID y agregarlos al workspace sin tener que invitarlos si ya están registrados. También falta la vista de todos los usuarios del workspace con sus permisos.
    - [API de PocketID](https://pocket-id.org/docs/api) — [Gestión de usuarios](https://pocket-id.org/docs/setup/user-management)
- **Webhooks UI**: El backend ya los tiene andando, pero falta la pantallita para configurarlos.
    - Inspiración: [Microsoft Graph Webhooks](https://learn.microsoft.com/en-us/graph/api/resources/subscription?view=graph-rest-1.0)
- **Columnas calculadas**: Fórmulas tipo Excel, PowerFX o Sharepoint que se calculen solas (capaz vía WASM para que vuele).
- **Encriptación**: Poder encriptar columnas sensibles (tarjetas, PII) para que solo el usuario final vea la data.
- **Imágenes y Miniaturas**: Poder guardar imágenes y ver una miniatura en la tabla.
    - Transformaciones: Estaría bueno integrar algo como [imgpush](https://github.com/hauxir/imgpush) para manejar las imágenes con una API por detrás.
- **IA Search**: Embeddings en columnas con algo tipo pgvector pero hay que formular bien el caso de uso.
- **Rotación Automática de API Keys**: Hoy la rotación es manual vía API. Mejorar con expiración con aviso, período de gracia con las dos keys activas y limpieza automática.
- **Checklist de Escalado a Multi-Servidor**:
    - [ ] Redis para Rate Limiting (ver arriba)
    - [ ] Redis para Session Storage (si usamos sesiones server-side)
    - [ ] PgBouncer para connection pooling de Postgres
    - [ ] Health checks en el Load Balancer
    - [ ] Logging centralizado (ELK stack o similar)
    - [ ] Distributed Tracing (Jaeger/Zipkin)
    - [x] Webhook delivery con garantías (ya está con outbox pattern ✅)

## 🕒 Para después (Features secundarias)

- **Drag & Drop**: Poder reordenar las columnas moviéndolas con el mouse.
- **Vistas Alternativas**: Meter un Kanban para estados, Calendario para fechas y una Galería para cuando tengamos imágenes.
- **Dashboard / Resumen**: Una vista con métricas (views, búsquedas, usuarios únicos, API calls) usando algo como [counter.dev](https://github.com/ihucos/counter.dev) más la data del backend.
- **Auditoría**: Un historial para ver quién tocó qué en cada registro (Audit logs).
- **RLS Visual (Seguridad por fila)**: Poder configurar filtros de filas por rol desde la UI.
    - Referencia técnica: [Postgres RLS Tutorial](https://www.enterprisedb.com/postgres-tutorials/how-implement-column-and-row-level-security-postgresql)

## 🚀 Vamos a soñar (Loqueras para el futuro)

- **Automation Flowchart**: Un editor low-code para armar flujos (tipo n8n). Inspiración: [Flowise](https://github.com/flowiseai/flowise) y [xyflow](https://xyflow.com/).
- **Conectores Oficiales** _(PowerAutomate ya funciona, n8n encaminado)_:
    - [PowerAutomate Connector](https://learn.microsoft.com/es-es/connectors/custom-connectors/paconn-cli) (con OAuth de PocketID y API Key) — ✅ funcionando.
    - [n8n Nodes](https://docs.n8n.io/integrations/creating-nodes/build/) para agentes de IA — en progreso.
    - [Typebot Forge](https://docs.typebot.io/contribute/the-forge/overview).
- **Backups Automáticos**: Con políticas de retención.
- **Instalador UI** _(más o menos funciona, revisar UX)_: Estilo WordPress/Drupal para configurar las variables de entorno la primera vez que lo instalás. Hay que revisar si es realmente útil y si el flujo es cómodo para el usuario final.

---

# ✅ Cosas hechas (Worklog)

## 2026-04-12 — Internationalization (i18n) Total
- ¡Liquidamos el 100% de los textos hardcodeados! Ahora todo pasa por `react-i18next`.
- Tenemos 13 idiomas soportados con paridad total (313 keys cada uno).
- Los tipos de datos ahora se traducen dinámicamente usando `fieldTypeConfig.jsx`.

## 2026-04-12 — Templates de Workspace
- Habilitada la selección de templates (CRM, Inventario, Proyectos) al crear un nuevo workspace.
- Los templates se cargan dinámicamente del backend y se importan con todo el esquema.

## 2026-04-12 — Exportación Completa
- Botón "Exportar JSON" para el schema completo del workspace (Tablas, Columnas, Roles).
- Exportación y Mapéo de CSV (Wizard) funcionando joya.

## 2026-03-XX — Seguridad y Auth
- **OIDC Integrado**: Soporte para PocketID, EntraID y Keycloak.
- **Webhooks Asincrónicos**: Implementado pattern de outbox para no frenar la API.
- **API Keys Seguras**: Generación de keys con validación de origen (CORS/Referer).
- **RLS Básico**: El backend ya filtra por `created_by` si el rol tiene acceso "Own".

## 2026-03-XX — UI / UX Core
- **Tablas PocketBase-style**: Edición inline, ghost row para crear registros rápido.
- **Permisos Inline**: Matriz de permisos (Create/Read/Update/Delete) por tabla y columna.
- **Tipos de Datos**: 14 tipos GORM (text, json, relation, file, etc.) con sus inputs específicos.
- **MCP Server**: Servidor para que agentes de IA puedan leer y escribir en HorneroDB.
- **Dark Mode**: Toggle funcional y persistente.
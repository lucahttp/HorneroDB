# Wishes / Feature Requests

Historial de deseos y features que se fueron pidiendo para HorneroDB.

## ✅ Implementados

- **Tablas inline (Excel/PocketBase style)** — crear rows en la ghost row, crear columnas con "+" en el header, dropdown de opciones (editar/borrar) en hover sobre cada columna header.
- **Permisos inline** — la tabla de permisos se edita directamente en la vista Configuración, sin modales. Botón de guardar inline.
- **Manejo de errores (PocketBase style)** — toast notifications con detalle expandible, sistema de error boundary.
- **Iconoir icon pack** — integrados en la UI.
- **Manual de marca** — estilos inspirados en giffgaff, Gumroad, PocketBase, FunctionGemma Playground.
- **Selección de filas** — checkboxes por fila + select all, toolbar de acciones bulk (borrado masivo).
- **Edición inline de celdas** — click en una celda la convierte en input, Enter guarda, Escape cancela.
- **Iconos de tipo de dato PocketBase-style** — badges de color por tipo en los headers de columna (T, #, @, 📅, ☑, ⟲, {}, etc.)
- **14 tipos de dato GORM completos** — text, long_text, number, integer, float, boolean, date, datetime, email, url, select, relation, json, file.
- **Inputs por tipo** — date picker para fechas, checkbox para boolean, textarea para JSON/long_text, input email/url con validación nativa.
- **Display inteligente** — badges ✓/✗ para booleans, links clickeables para email/url, `<code>` para JSON.
- **OpenAPI YAML** — documentación completa de las 30+ rutas en `docs/openapi.yaml`.
- **API REST completa** — 30+ endpoints para workspaces, tablas, columnas, datos, roles, usuarios, permisos, keys.
- **Auth OIDC** — integración con PocketID, EntraID, Keycloak.
- **Multi-workspace** — soporte multi-tenant con workspaces aislados.
- **Servidor MCP** — servidor para agentes IA.
- **API Keys** — generación de keys por workspace.

## 🔲 Pendientes / Ideas futuras

### Datos y Tablas

- **Relaciones entre tablas (picker estilo PocketBase)** — selector visual con búsqueda, mostrar nombres legibles en vez de IDs en celdas de relación
- **Filtros y búsqueda en los datos de tabla** — filter by, search bar
- **Ordenamiento por columna** — click en header para ASC/DESC
- **Paginación visible en la tabla** — "Mostrando 1-20 de 150"
- **Drag & drop para reordenar columnas** — reordenar visualmente las columnas de una tabla

### Vistas Alternativas

- **Vistas personalizadas** — Kanban (para estados), Calendar (para fechas), Gallery (para imágenes)
- **Dashboard / vista de resumen por workspace** — métricas, gráficos de ingresos/gastos, totales por período

### Import/Export

- **Exportar datos** — CSV, JSON
- **Importar datos** — CSV con mapeo de columnas

### Seguridad y Auditoría

- **Webhooks por tabla** — triggers en create/update/delete
- **Auditoría / log de cambios** — historial de modificaciones por registro
- **Row-level security visual** — UI para configurar row_filter por rol

### UX/UI

- **Soporte móvil mejorado** — long press para acciones, swipe para acciones
- **Dark mode toggle** — tema oscuro
- **Archivos/uploads** — preview de imágenes, S3 integration con UI de preview

---

## Inspirado en análisis de sistemas similares

Análisis de PocketBase y otros sistemas low-code reveló features muy demandados en práctica real:

- Dashboard con métricas (gastos vs ingresos, totales por categoría)
- Calendario de turnos/reservas
- Relación visual entre tablas (no solo guardar IDs)
- Normalización de datos (categorías como relaciones, no texto libre)
- Estados visuales para turnos (pendiente, confirmado, completado, cancelado)

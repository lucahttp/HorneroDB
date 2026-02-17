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






### Settings [Error]

cuando estas dentro de un workspace y queres ir a settings se freza la pantalla y no funciona mas, creo que es algo relacionado con los iconos que se llamaban igual que la funcion Settings


### Tables in Workspace [Error]

cuando estas dentro de un workspace y ves las tablas de datos vas a ver arriba de todo en una esquina un icono de editar y otro de borrar. nada que ver deberia de aparecer en la tabla de datos al mantener el mouse encima de la fila de datos o al mantener precionado el touch arriba de la tabla de datos.

### View Table [Error]

cuando estas una tabla de datos y ves la tabla de datos vas a notar de que no es responsive, cuando haces scroll horizontal para ver la tabla completa se vuelve un poco feo. tambien pensandolo para cuando sea mobile que se pueda ver la tabla de datos.


### Login de usuarios [Feature]

quiero poder gestionar nuevos usuarios y que permisos tienen dentro de cada workspace, 


### Languaje dropdown [Feature]
al lado del toggle theme de dark / light quiero que este, y que sea un dropdown que te permita cambiar el idioma de la app.
quiero que recuerde el idioma que el usuario eligio


### Manage Users  [Feature]

quiero poder gestionar los usuarios del workspace, que pueda crear, editar y borrar usuarios.

la idea seria que el admin pueda ver los usuarios de todos los workspaces y que se pueda ver los permisos de cada usuario.

y que el owner pueda ver los usuarios de su workspace y que se pueda ver los permisos de cada usuario

ademas poder agregar roles y permisos a los usuarios

adenas lo mas importante es que el admin o el workspace owner pueda agregar usuarios a su workspace usando pocketid en un principio, que genere el usuario usando la api de pocketid y que lo agregue a el grupo correspondiente.

https://pocket-id.org/docs/setup/user-management
https://pocket-id.org/docs/api


### Manage Users 1.1 [Feature]

en configuracion al lado de "Roles de Seguridad" y "API Keys" quiero que aparezcan los usuarios de cada workspace,
quiero poder controlar que permiso tienen asignado los usuarios como las API Keys.

### Manage Users 1.2 bis [Feature]

quiero poder gestionar desde que web o app se llaman a las API Keys de cada workspace.
agregar tambien para apps programaticas una source IP si no es desde un front end y es por ejemplo desde un agente de IA o un MCP.


### Manage Users 1.3 bis [Research]

quiero saber que opciones de seguridad tienes pocketbase, nocodb (creo que son muy pocas por eso creamos HorneroDB) y ademas quiero agregar un apartado dentro de configuracion que sea un dashboard de seguridad que muestre los logs de seguridad y api calls como en pocketbase.




### agregar usuarios sigue roto [Error]


POST http://localhost:8080/api/v1/workspaces/6d1a2f07-ee97-488d-a9f2-1eec6fd4dd70/users 404 (Not Found)
AxiosInterceptor.jsx:20 Global API Error: AxiosError: Request failed with status code 404
    at settle (axios.js?v=d4df10c7:1281:12)
    at XMLHttpRequest.onloadend (axios.js?v=d4df10c7:1638:7)
SettingsUsers.jsx:61 AxiosError: Request failed with status code 404
    at async handleImport (SettingsUsers.jsx:51:13)


"User not found in PocketID or Local DB"


### no puedo asignar roles a las api keys [Feature]

quiero poder asignar roles a las api keys

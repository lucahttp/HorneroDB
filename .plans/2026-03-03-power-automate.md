# PowerAutomate Custom Connector (paconn)

El objetivo es crear un conector personalizado para **Microsoft Power Automate**, **Power Apps** y **Logic Apps** que permita a los usuarios leer, escribir y reaccionar a cambios en HorneroDB.

Se construirá utilizando la herramienta oficial [`paconn-cli`](https://learn.microsoft.com/es-es/connectors/custom-connectors/paconn-cli).

## Estructura del Conector

Un Custom Connector en PowerAutomate se define tradicionalmente mediante tres piezas (o un OpenAPI definition vitaminado):
1. **`apiProperties.json`**: Define el logo, color, host y el **esquema de Autenticación**.
2. **`apiDefinition.swagger.json`**: El archivo OpenAPI 2.0 (o 3.0 importado) que describe los endpoints.
3. **`icon.png`**: El logo del conector.

Alojaremos estos archivos en un nuevo directorio: `/misc/connectors/power-automate/` a nivel repositorio.

## 1. Autenticación Múltiple (API Key & OIDC)

Power Automate requiere que definamos cómo se autorizarán las llamadas en `apiProperties.json`. El requerimiento exige usar tanto _PocketID OAuth_ (OIDC) como _API Keys_.

Para lograr esto en Custom Connectors, tenemos dos opciones principales de diseño, dado que Power Automate asigna **una sola forma de autenticación por entorno de conexión**:

### Opción Recomendada: Múltiples Definiciones O Conector Oauth2 con Fallback
Power Automate soporta el Auth Type `OAuth 2.0`. Si configuramos el conector como OAuth 2.0 contra PocketID:
- **Authorization URL**: `https://<pocketid-url>/api/v1/auth/oidc/login`
- **Token URL**: `https://<pocketid-url>/api/v1/auth/oidc/token`
- **Scope**: `openid profile email offline_access`

**¿Y las API Keys?**
Si el usuario prefiere usar un API Key de HorneroDB en vez de loguearse manualmente, Power Automate requerirá configurar un **Auth Type: API Key** (Header `Authorization: Bearer <token>`).
Dado que un único conector no puede mezclar la UI interactiva (OAuth) con inputs manuales estáticos simultáneamente para la misma "definición", la mejor práctica es:
1. Exportar en la plantilla base la autenticación orientada a **OAuth2**.
2. Documentar cómo hacer un override local/scripted a `API Key` para servicios Service-to-Service desatendidos usando policy templates.

## 2. Endpoints Clave (Acciones)

Exportaremos un subset optimizado de `docs/openapi.yaml` convertido al formato requerido por Power Automate.
- **Acciones Estándar (Actions)**:
  - `List Records` (GET `/api/v1/workspaces/{id}/data/{table_slug}`)
  - `Create Record` (POST `/api/v1/workspaces/{id}/data/{table_slug}`)
  - `Update Record` (PUT `/api/v1/workspaces/{id}/data/{table_slug}/{id}`)
  - `Delete Record` (DELETE `/api/v1/workspaces/{id}/data/{table_slug}/{id}`)
  - `List Workspaces` (GET `/api/v1/workspaces`)
  - `List Tables` (GET `/api/v1/workspaces/{id}/tables`)

## 3. Gatillos Reactivos (Triggers)

Aquí brilla nuestra nueva arquitectura de **Webhooks**. PowerAutomate usa el modelo de `x-ms-notification-content` en Swagger para definir Triggers.

Configuraremos tres Triggers principales:
1. **When a record is created** (`changeType: "created"`)
2. **When a record is updated** (`changeType: "updated"`)
3. **When a record is deleted** (`changeType: "deleted"`)

**Mecánica del Trigger (Suscripción/Unsubscribe)**:
En el `apiDefinition.swagger.json`, extenderemos el endpoint de POST Webhooks (`/api/v1/workspaces/{id}/webhooks`) con la anotación `x-ms-notification-content`. Power Automate, en el momento de crear el flujo:
1. Mandará un `POST` a HorneroDB para registrarse automáticamente con su Callback URL.
2. Al apagar el flujo, disparará un `DELETE` al webhook.

## Progress
| Tarea | Estado |
|-------|--------|
| Limpiar OpenAPI YAML a formato compatible (Swagger/OpenAPI 2) | [ ] |
| Crear `apiProperties.json` con `OAuth2` & `API Key` | [ ] |
| Decorar `apiDefinition.json` con metadatos `x-ms-*` de Triggers | [ ] |
| Empaquetar script de inicialización con `paconn-cli` | [ ] |

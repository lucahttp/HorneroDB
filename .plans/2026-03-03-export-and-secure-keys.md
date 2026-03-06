# Workspace Export/Import & Secure API Keys

El usuario ha solicitado dos nuevas grandes funcionalidades (features) para HorneroDB:
1. **Exportar/Importar esquemas completos de Workspaces**: Lo cual incluye Tablas, Roles, Permisos y Configuraciones, permitiendo mover arquitecturas entre entornos.
2. **Mejorar la seguridad de las API Keys (Secretos ciegos)**: Las API Keys (tokens) deben mostrarse **solamente una vez** al ser creadas en texto plano. Después, deben almacenarse como un hash para evitar filtraciones, con la capacidad de "rotar" (re-crear) el token.

## Parte 1: Secure API Keys (Hashing)

Actualmente las API Keys se guardan y devuelven en texto plano (`token` column en `_hornero_api_keys`). Esto es un riesgo de seguridad.

### Proposed Changes

#### 1. Modificar Modelo `APIKey` (`internal/models/metadata/role.go`)
- **[MODIFY]** Cambiar cómo opera el `Token`. Añadiremos dos virtudes:
	- Mantendremos `Token` como campo de la base de datos, pero en lugar de guardar el JWT/UUID en crudo, guardaremos un **SHA-256 Hash**.
    - Crearemos un struct de respuesta específico o inyectaremos un campo virtual `PlainToken` que *sólo* se poblará al hacer un `POST` (crear) o en un endpoint de rotación.
- **[MODIFY]** `internal/middleware/auth.go`: Modificar la función que autentica API Keys para **hashear el Bearer token entrante** y buscar en la base de datos el Hash (en lugar de buscar el raw token).

#### 2. Endpoints de Rotación
- **[NEW]** `POST /api/v1/workspaces/{workspace_id}/apikeys/{key_id}/rotate`:
    Generará un nuevo secret aleatorio (`uuid.New().String()`), lo hasheará, actualizará la base de datos, y devolverá el **nuevo token en texto plano** sólo por esa request.

#### 3. Impacto de UI
- Ahora los endpoints `GET /apikeys` no devolverán el token original, sólo algo que actúe de hint, ej: `TokenPrefix` (los primeros 4 o los últimos 4 chars del original para que el user lo reconozca si lo guardó) O simplemente un placeholder como `******************`.
- Añadiremos un campo `token_hint` a la base de datos que guardará los primeros 4 o ultimos 4 caracteres del secret original sin hashear.

## Parte 2: Export/Import de Workspaces

Poder hacer un dump del esquema de un Workspace y reiniciarlo en otro lado. Esto es estrictamente un **Export de Metadata**, NO un backup de la tabla de "datos" (Data rows) en sí. Exportaremos la arquitectura.

### Estructura de Exportación (`WorkspaceSchemaDump`)
Crearemos un modelo en memoria (JSON) que empaquete:
- Detalles del Workspace
- Lista de `Tables`
- Lista de `Columns` por cada tabla
- Lista de `Roles`
- Lista de `TablePermissions` y `ColumnPermissions` combinados en sus Roles.

### Proposed Changes

#### 1. Endpoints de Exportación/Importación
- **[NEW]** `GET /api/v1/workspaces/{workspace_id}/export`:
    Recolectará toda la Metadata de la base de datos (`_hornero_tables`, `_hornero_columns`, `_hornero_roles`, `_hornero_permissions`) filtados por el `workspace_id`. Los empaquetará en un JSON único estructurado asimetricamente para facilitar la lectura.

- **[NEW]** `POST /api/v1/workspaces/import`:
    Recibirá el JSON del dump.
    - Creará un Workspace nuevo con el mismo o distinto nombre.
    - Envolverá la creación en una **Transacción SQL** (`database.DB.Transaction`).
	- Re-creará las Tablas.
	- Re-creará las Columnas físicamente (`ALTER TABLE...`).
	- Insertará los Roles.
	- Insertará los Permisos mapeándolos a los nuevos UUIDs si fue necesario clonarlos.

### User Review Required

> [!CAUTION]
> Modificar las API Keys a Hashes es un un **Breaking Change**.
> Todas las API Keys existentes en la base de datos dejarán de funcionar si actualizamos el middleware para hacer Hash del Token en cada request entrante (porque la base de datos ya las tiene guardadas en texto plano en lugar del hash). 
> **¿Quisieras que implementemos una Migración de Datos (un script al iniciar que lea los viejos text-plain tokens y los superponga con el Hash), o preferís simplemente invalidar todas las API keys previas creadas por seguridad total?**

Por favor revisa ambos planes. Si estás de acuerdo con la estrategia de "Hint" en base de datos para la UI + Hashing, y la lógica de Import/Export, procedemos!

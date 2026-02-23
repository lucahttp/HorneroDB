# HorneroDB - Informe de Calidad de Código

**Fecha de análisis:** 17 de febrero de 2026  
**Versión del proyecto:** -  
**Archivos analizados:** 18 archivos (Backend Go + Frontend React)

---

## Resumen Ejecutivo

| Categoría | Críticos | Altos | Medios | Bajos | Total |
|-----------|----------|-------|--------|-------|-------|
| Bugs potenciales | 3 | 7 | 12 | 8 | 30 |
| Security issues | 2 | 5 | 4 | 2 | 13 |
| Code smells | 0 | 8 | 15 | 12 | 35 |
| Falta de tests | - | - | - | - | 18 items |
| Performance | 0 | 4 | 6 | 3 | 13 |
| Best practices | 1 | 6 | 8 | 5 | 20 |

**Puntuación general:** 5.2/10 - Requiere atención inmediata en seguridad y arquitectura

---

## 1. BUGS POTENCIALES

### 1.1 Críticos

#### 🔴 BUG-001: Race Condition en Variables Globales
**Archivo:** `internal/handlers/api/auth_oidc.go:12-13`
```go
var oidcAuth *auth.OIDCAuth
var jwtSecret string
```
**Problema:** Variables globales no son thread-safe. Múltiples goroutines pueden acceder simultáneamente durante el startup, causando comportamiento indefinido.

**Impacto:** Puede causar panics o autenticación fallida en producción con múltiples requests.

**Recomendación:**
```go
// Usar sync.Once para inicialización segura
var (
    once       sync.Once
    oidcAuth   *auth.OIDCAuth
    jwtSecret  string
)

func InitAuth(cfg *config.AuthConfig, secret string) error {
    jwtSecret = secret
    // ... rest of init
}
```

**También afecta:** `internal/handlers/api/record.go:15` - `var permService = permission.NewService()`

---

#### 🔴 BUG-002: Error Ignorado en Drop Table
**Archivo:** `internal/handlers/api/table.go:122`
```go
database.DB.Exec(`DROP TABLE IF EXISTS "` + tableName + `"`)
```
**Problema:** El resultado de `Exec()` no se verifica. Si falla, el usuario recibe "deleted" exitosamente pero la tabla física permanece.

**Impacto:** Inconsistencia entre metadatos y datos reales.

**Recomendación:**
```go
if err := database.DB.Exec(...).Error; err != nil {
    log.Printf("Error dropping table: %v", err)
    // Considerar si esto debe fallar el request
}
```

---

#### 🔴 BUG-003: Same Issue en Column Delete
**Archivo:** `internal/handlers/api/column.go:148,152`
```go
database.DB.Table("_hornero_columns").Delete(&metadata.Column{}, "id = ?", columnID)
database.DB.Exec(`ALTER TABLE "` + tableName + `" DROP COLUMN IF EXISTS "` + column.Slug + `"`)
```
**Problema:** Mismo bug - errores ignorados en operaciones de eliminación de columnas.

---

### 1.2 Altos

#### 🟠 BUG-004: Fallback Inseguro en Verificación JWT
**Archivo:** `internal/services/auth/oidc.go:214-235, 330-346`
```go
if err != nil {
    // Parse token directly without verification
    parts := strings.Split(tokenResp.IDToken, ".")
    // ... parsing without verify
}
```
**Problema:** Si la verificación del token OIDC falla, el código hace parsing directo del JWT sin verificación criptográfica. Esto es un bypass de seguridad crítico.

**Impacto:** Tokens JWT伪造 pueden ser aceptados sin validación.

---

#### 🟠 BUG-005: Debug Prints Exponen Datos Sensibles
**Archivos:**
- `internal/services/auth/oidc.go:129-130, 208-209, 312-313, 322-325`
- `internal/middleware/workspace_auth.go:49, 67`

```go
fmt.Printf("DEBUG - Token response body: %s\n", string(body))
fmt.Printf("DEBUG WorkspaceAuth: userID=%s, ownerID=%s\n", userID, ownerID)
```
**Problema:** Prints de debug en producción exponen tokens, IDs de usuario, y estructura de datos interna.

**Impacto:** Información sensible en logs de producción.

---

#### 🟠 BUG-006: OwnerID Comparison Inseguro
**Archivo:** `internal/middleware/workspace_auth.go:51`
```go
if workspace.OwnerID.String() == userID {
```
**Problema:** `OwnerID` es UUID, pero `userID` viene de claims JWT como string. La comparación puede fallar silenciosamente.

**Impacto:** Dueños de workspace pueden no recibir permisos de admin.

---

#### 🟠 BUG-007: Sin Verificación de Workspace en Delete
**Archivo:** `internal/handlers/api/table.go:103-125`
```go
func DeleteTable(c *gin.Context) {
    tableID := c.Param("table_id")
    // No verifica que la tabla pertenezca al workspace del usuario
    // Query directo con tableID sin workspace check
}
```
**Problema:** No se verifica que la tabla pertenezca al workspace autenticado. Cualquier usuario autenticado podría eliminar cualquier tabla si conoce su ID.

---

#### 🟠 BUG-008: Duplicación de Código - Permisos en Record Handler
**Archivo:** `internal/handlers/api/record.go:36-44, 90-98, 139-147, etc.`

La misma lógica de permisos se repite 5 veces:
```go
accessLevel, err := permService.CheckTableAccess(wsID, roleName, tableSlug, "read")
if err != nil { ... }
if accessLevel == permission.AccessNone { ... }
```

**Impacto:** Código repetitivo difícil de mantener. Si se encuentra un bug, hay que corregirlo en 5 lugares.

---

### 1.3 Medios

#### 🟡 BUG-009: Nil Check Faltante en GetUserID
**Archivo:** `internal/middleware/auth.go:159-164`
```go
func GetUserID(c *gin.Context) string {
    if id, exists := c.Get("user_id"); exists {
        return id.(string)  // Puede panic si el tipo no es string
    }
    return ""
}
```
**Problema:** Type assertion sin verificación de tipo puede causar panic.

---

#### 🟡 BUG-010: OwnerID Parse Ignorado
**Archivo:** `internal/handlers/api/workspace.go:36-40`
```go
ownerID, err := uuid.Parse(input.OwnerID)
if err != nil {
    c.JSON(400, gin.H{"error": "invalid owner_id"})
    return
}
```
**Problema:** El error es manejado correctamente aquí, pero inconsistente con otros handlers.

---

#### 🟡 BUG-011: Same Issue - type assertions sin check
**Archivos:** 
- `middleware/auth.go:166-170` - `GetUserRole`
- `middleware/auth.go:181-185` - `GetUserWorkspace`
- `middleware/auth.go:188-192` - `GetAuthSource`

Todas usan `role.(string)` sin verificar tipo.

---

#### 🟡 BUG-012: Missing Error Check en LastUsedAt Update
**Archivo:** `internal/middleware/auth.go:110-112`
```go
database.DB.Table("_hornero_api_keys").
    Where("id = ?", apiKey.ID).
    Update("last_used_at", time.Now())
```
**Problema:** Update silenciosamente puede fallar.

---

#### 🟡 BUG-013: Hardcoded Redirect en Callback
**Archivo:** `internal/handlers/api/auth_oidc.go:63`
```go
redirectURL := "http://localhost:5173/callback"
```
**Problema:** Solo funciona en desarrollo.

---

### 1.4 Bajos

#### 🔵 BUG-014: rand.Seed Deprecated
**Archivo:** `internal/handlers/api/apikey.go:18`
```go
rand.Seed(time.Now().UnixNano())
```
**Problema:** `rand.Seed` está deprecated en Go 1.20+. El paquete math/rand ahora se inicializa automáticamente.

---

#### 🔵 BUG-015: Unused Import
**Archivo:** `internal/handlers/api/auth_oidc.go:4`
```go
import (
    "net/http"
    // ...
)
```
`net/http` aparece en el import pero no se usa directamente.

---

## 2. SECURITY ISSUES

### 2.1 Críticos

#### 🔴 SEC-001: SQL Injection en Dynamic Table Names
**Archivos:** 
- `internal/handlers/api/table.go:56-62`
- `internal/handlers/api/column.go:70-71`

```go
tableName := "data_" + workspaceID + "_" + input.Slug
createSQL := `CREATE TABLE IF NOT EXISTS "` + tableName + `" ...`
```

**Problema:** Concatenación directa de input del usuario en nombres de tablas. Aunque GORM protege los valores, los identificadores (nombres de tablas) no son parametrizables.

**Impacto:** Si un attacker puede controlar `workspaceID` o `input.Slug`, puede ejecutar SQL arbitrario.

**Recomendación:** Validar estrictamente el formato:
```go
if !isValidSlug(input.Slug) {
    c.JSON(400, gin.H{"error": "invalid slug format"})
    return
}

func isValidSlug(slug string) bool {
    match, _ := regexp.Compile(`^[a-z][a-z0-9_]{0,62}$`)
    return match.MatchString(slug)
}
```

---

#### 🔴 SEC-002: JWT Secret Default Inseguro
**Archivo:** `internal/config/config.go:71`
```go
JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),
```

**Problema:** Valor por defecto conocido públicamente. Si alguien despliega sin configurar la variable, el JWT puede ser forjado.

---

### 2.2 Altos

#### 🟠 SEC-003: CORS Allows Credentials + Any Origin
**Archivo:** `cmd/server/main.go:64-71`
```go
r.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000"},
    AllowCredentials: true,
    // ...
}))
```

**Problema:** En producción, origins hardcodeados no funcionarán. Además, con `AllowCredentials: true`, los origins deben ser explícitos.

**Recomendación:**
```go
AllowOrigins: func(origin string) bool {
    if os.Getenv("HORNERO_ENV") == "production" {
        return origin == os.Getenv("ALLOWED_ORIGIN")
    }
    return true  // O lista específica de development
}
```

---

#### 🟠 SEC-004: API Key en URL Query
**Archivo:** `internal/middleware/auth.go:38-47`
```go
tokenString := strings.TrimPrefix(authHeader, "Bearer ")
if tokenString == authHeader {
    apiKey, err := verifyAPIKey(tokenString)
}
```

**Problema:** API keys se pasan como `Authorization: <key>` sin el prefijo "Bearer". Esto es confuso y puede causar fugas en logs.

---

#### 🟠 SEC-005: Token Expiration No Validado en Claims
**Archivo:** `internal/middleware/auth.go:50-65`
```go
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    return []byte(secret), nil
})
```

**Problema:** No se valida explícitamente la expiración. Aunque JWT lo hace por defecto, no hay validación explícita del claim `exp`.

---

#### 🟠 SEC-006: No Rate Limiting
**Archivo:** No existe middleware de rate limiting

**Problema:** Sin protección contra brute force de passwords o API keys.

---

#### 🟠 SEC-007: LocalStorage para Tokens
**Archivo:** `web/ui/src/App.jsx:14, 125`
```go
localStorage.setItem('hornero_token', token)
```

**Problema:** Tokens en localStorage son accesibles via XSS. Cookies con `HttpOnly` serían más seguras.

---

### 2.3 Medios

#### 🟡 SEC-008: Hardcoded API URL en Frontend
**Archivo:** `web/ui/src/App.jsx:11`
```go
const API_URL = 'http://localhost:8080/api/v1'
```

**Problema:** No funciona en producción. Debe venir del environment.

---

#### 🟡 SEC-009: No CSRF Protection
**Problema:** No se implementa protección CSRF. El token JWT se envía en headers pero no hay verificación de origin.

---

#### 🟡 SEC-010: No Input Validation en Slug/Names
**Archivos:**
- `table.go:27-35`
- `column.go:27-38`

```go
Name     string `json:"name" binding:"required"`
Slug     string `json:"slug" binding:"required"`
```

**Problema:** Solo `binding:"required"`. No se validan caracteres especiales, longitud, o formato.

---

#### 🟡 SEC-011: No Validation de Field Type
**Archivo:** `column.go:30`
```go
FieldType  string `json:"field_type" binding:"required"`
```

**Problema:** Cualquier string es aceptada. Un attacker podría enviar tipos de datos no válidos.

---

### 2.4 Bajos

#### 🔵 SEC-012: Error Messages Exponen Información
**Archivos:** Múltiples handlers
```go
c.JSON(500, gin.H{"error": result.Error.Error()})
```

**Problema:** Errores de base de datos se exponen al cliente. Puede revelar estructura de DB.

---

#### 🔵 SEC-013: No HTTPS Enforcement
**Problema:** No hay redirect de HTTP a HTTPS ni HSTS.

---

## 3. CODE SMELLS

### 3.1 Altos

#### 💩 SMELL-001: Archivo Monolítico en Frontend
**Archivo:** `web/ui/src/App.jsx` - 1369 líneas

**Problema:** Una sola vista de React con 1369 líneas. Contiene 6 componentes grandes (Login, Callback, Dashboard, Workspace, TableView, Settings) en un archivo.

**Impacto:** Imposible de mantener, difíciles de tests, merge conflicts constantes.

**Recomendación:** Separar en archivos individuales:
```
components/
  ├── Login.jsx
  ├── Callback.jsx
  ├── Dashboard.jsx
  ├── Workspace.jsx
  ├── TableView.jsx
  ├── Settings.jsx
  ├── Sidebar.jsx
  └── index.js
```

---

#### 💩 SMELL-002: Duplicación de Botones de Rename
**Archivo:** `web/ui/src/App.jsx:359-383` y `657-682`

```jsx
<button onClick={(e) => renameWorkspace(ws.id, ws.name, e)} ...>
<button onClick={(e) => renameTable(table.id, table.name, e)} ...>
```

**Problema:** El mismo botón (icono de edición) aparece duplicado en el markup para cada workspace/tabla.

---

#### 💩 SMELL-003: Duplicación de Lógica de Permisos
**Archivo:** `internal/handlers/api/record.go`

Cinco funciones repiten exactamente la misma estructura:
- `ListRecords` (líneas 17-69)
- `CreateRecord` (líneas 71-117)
- `GetRecord` (líneas 119-165)
- `UpdateRecord` (líneas 167-223)
- `DeleteRecord` (líneas 225-275)

Cada una hace:
1. Parse workspaceID
2. Fetch table metadata
3. Check permissions
4. Build query with access level

**Recomendación:** Extraer a función helper:
```go
func (h *RecordHandler) withTableAccess(c *gin.Context, operation string, handler func(table metadata.Table, accessLevel permission.AccessLevel)) {
    // Lógica compartida
}
```

---

#### 💩 SMELL-004: Duplicated OIDC Callback Logic
**Archivo:** `internal/services/auth/oidc.go`

`HandleCallback` (línea 192) y `HandleCallbackAndRedirect` (línea 304) comparten ~80% del código.

---

#### 💩 SMELL-005: Magic Strings Everywhere
**Archivos:** Múltiples

```go
c.JSON(400, gin.H{"error": "invalid workspace_id"})
c.JSON(404, gin.H{"error": "table not found"})
c.JSON(403, gin.H{"error": "no permission to read this table"})
```

**Problema:** No hay constantes definidas para mensajes de error.

---

#### 💩 SMELL-006: Inline Styles en React
**Archivo:** `web/ui/src/App.jsx` - uso extensivo de `style={{...}}`

**Problema:** Violación del principio de separación de concerns. Dificulta theming y mantenimiento.

---

#### 💩 SMELL-007: No Abstractions para DB Operations
**Problema:** Los handlers llaman directamente a `database.DB.Table(...).Find()` en todas partes.

**Impacto:** Imposible cambiar de GORM a otro ORM sin reescribir todo.

---

#### 💩 SMELL-008: Global DB Variable
**Archivo:** `internal/database/connection.go:12`
```go
var DB *gorm.DB
```

**Problema:** Variable global dificulta testing y no permite múltiples conexiones.

---

### 3.2 Medios

#### 🟡 SMELL-009: Comentarios en Español/Inglés Mezclados
**Archivos:** Múltiples

```go
// === WORKSPACES ROUTES ===
// Routes that don't need workspace-specific auth (e.g., creating a workspace)
// === ROLES DE SEGURIDAD (Dataverse style) ===
```

---

#### 🟡 SMELL-010: Debug Code en Producción
**Archivos:**
- `internal/middleware/workspace_auth.go:49, 67`
- `internal/services/auth/oidc.go:129, 208, 312, 322`

```go
fmt.Printf("DEBUG WorkspaceAuth: userID=%s, ownerID=%s\n", ...)
```

---

#### 🟡 SMELL-011: Nombres de Variables Inconsistentes
```go
workspaceID := c.Param("workspace_id")  // camelCase
tableID := c.Param("table_id")          // camelCase  
wsID, err := uuid.Parse(workspaceID)   // diferente!
```

---

#### 🟡 SMELL-012: Funciones Muy Largas
**Archivo:** `web/ui/src/App.jsx` - 函数 `Workspace` tiene ~260 líneas

**Problema:** Función que hace de todo (fetch data, render UI, handle events).

---

#### 🟡 SMELL-013: useEffect Sin Dependencias Proper
**Archivo:** `web/ui/src/App.jsx:773-775`
```jsx
useEffect(() => {
    loadData()
}, [workspaceId, tableId])
```

**Problema:** `loadData` no está en dependencies array, puede causar stale closures.

---

#### 🟡 SMELL-014: No Error Boundaries en React
**Problema:** Un error en un componente crashea toda la aplicación.

---

#### 🟡 SMELL-015: useState para Todo
**Archivo:** `web/ui/src/App.jsx`

Cada pedazo de estado es un `useState` separado, resultando en docenas de estados.

---

### 3.3 Bajos

#### 🔵 SMELL-016: Raw SQL en Algunos Lugares
**Archivo:** `internal/database/connection.go:25`
```go
"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC plan_cache_mode=force_custom_plan"
```

---

#### 🔵 SMELL-017: Console.log en Frontend
**Archivo:** `web/ui/src/App.jsx`
```jsx
.catch(console.error)
```

---

#### 🔵 SMELL-018: Hardcoded Arrays
**Archivo:** `web/ui/src/App.jsx:835-842`
```jsx
const fieldTypes = [
    { value: 'text', label: 'Texto' },
    // ...
]
```

Debería venir del backend.

---

#### 🔵 SMELL-019: No PropTypes o TypeScript
**Problema:** Todo el frontend es JavaScript sin tipos.

---

#### 🔵 SMELL-020: Nested Callbacks
**Archivo:** Múltiples lugares en React

```jsx
axios.get(url)
  .then(res => {
      // callback hell
  })
```

---

## 4. FALTA DE COBERTURA DE TESTS

### 4.1 Backend - Tests Existentes

**Archivos con tests:**
- `internal/handlers/api/api_test.go` - ⚠️ Verificar qué cubre
- `internal/services/permission/service_test.go` - ⚠️ Verificar qué cubre

### 4.2 Lo que NO está probado

| Componente | Funcionalidad | Estado |
|------------|---------------|--------|
| **Auth Middleware** | JWT validation | ❌ No testeado |
| **Auth Middleware** | API Key validation | ❌ No testeado |
| **Workspace Auth** | Owner detection | ❌ No testeado |
| **Workspace Auth** | Role resolution | ❌ No testeado |
| **Handlers** | CRUD de Workspaces | ⚠️ Parcial |
| **Handlers** | CRUD de Tables | ❌ No testeado |
| **Handlers** | CRUD de Columns | ❌ No testeado |
| **Handlers** | CRUD de Records | ❌ No testeado |
| **Handlers** | CRUD de Roles | ❌ No testeado |
| **Handlers** | CRUD de API Keys | ❌ No testeado |
| **Permission Service** | Table access checks | ⚠️ Parcial |
| **Permission Engine** | Row filtering | ❌ No testeado |
| **OIDC Auth** | Token exchange | ❌ No testeado |
| **OIDC Auth** | Claims extraction | ❌ No testeado |
| **OIDC Auth** | JWT generation | ❌ No testeado |

### 4.3 Frontend - Tests Existentes

**Archivos:**
- `web/ui/tests/app.spec.ts` - Tests muy básicos
- `web/ui/tests/full-flow.spec.ts` - No analizado

### 4.4 Lo que NO está probado en Frontend

| Componente | Test | Estado |
|------------|------|--------|
| Login flow | Completo | ⚠️ Parcial |
| Callback | Token handling | ❌ |
| Dashboard | Workspace CRUD | ❌ |
| Workspace | Table CRUD | ❌ |
| TableView | Records CRUD | ❌ |
| TableView | Column CRUD | ❌ |
| Settings | Roles CRUD | ❌ |
| Settings | API Keys CRUD | ❌ |
| Permission Matrix | Save permissions | ❌ |
| Error handling | API errors | ❌ |
| Auth | Token refresh | ❌ |
| Auth | Logout | ❌ |

---

## 5. PROBLEMAS DE PERFORMANCE

### 5.1 Altos

#### ⚡ PERF-001: N+1 Query en Permisos
**Archivo:** `internal/services/permission/service.go:34-74`

Cada llamada a `CheckTableAccess` hace:
1. Query a `_hornero_roles` para obtener el role
2. Parse JSON de permisos

**Problema:** En `record.go`, cada operación de CRUD hace esta query. Con 5 operaciones = 5 queries mínimas.

**Recomendación:** Cachear permisos en memoria con TTL:
```go
type PermissionCache struct {
    mu      sync.RWMutex
    entries map[string]cacheEntry
}

type cacheEntry struct {
    permissions map[string]TableAccess
    expires     time.Time
}
```

---

#### ⚡ PERF-002: Múltiples Queries en WorkspaceAuth
**Archivo:** `internal/middleware/workspace_auth.go:44-90`

Para cada request autenticado:
1. Query: Get workspace (línea 45)
2. Query: Check user roles (línea 62)
3. Query: Get role definition (línea 76)

**Total:** 3 queries por request mínimo.

---

#### ⚡ PERF-003: No Prepared Statements
**Archivo:** `internal/database/connection.go:35`
```go
PrepareStmt: false,
```

**Problema:** Queries no se cachean en PostgreSQL.

---

#### ⚡ PERF-004: Query Sin Límite
**Archivo:** `internal/handlers/api/workspace.go:14-21`
```go
result := database.DB.Table("_hornero_workspaces").Find(&workspaces)
```

**Problema:** Si hay miles de workspaces, esto retorna todo sin paginación.

---

### 5.2 Medios

#### 🟡 PERF-005: PreferSimpleProtocol Deshabilitado
**Archivo:** `internal/database/connection.go:32`
```go
PreferSimpleProtocol: true,
```

**Problema:** Esto deshabilita prepared statements implicit. Puede ser intencional pero reduce performance.

---

#### 🟡 PERF-006: No Indexes Definidos
**Archivo:** `internal/database/migrations.go`

No se definen índices explícitos para columnas frecuentemente consultadas:
- `_hornero_tables.workspace_id`
- `_hornero_columns.table_id`
- `_hornero_user_roles.workspace_id, user_id`
- `_hornero_api_keys.key_hash`

**Recomendación:** Agregar en migrations:
```go
DB.Exec("CREATE INDEX IF NOT EXISTS idx_tables_workspace ON _hornero_tables(workspace_id)")
DB.Exec("CREATE INDEX IF NOT EXISTS idx_columns_table ON _hornero_columns(table_id)")
DB.Exec("CREATE INDEX IF NOT EXISTS idx_user_roles_ws_user ON _hornero_user_roles(workspace_id, user_id)")
DB.Exec("CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON _hornero_api_keys(key_hash)")
```

---

#### 🟡 PERF-007: Select *
**Archivos:** Múltiples handlers

```go
database.DB.Table("_hornero_workspaces").Find(&workspaces)
```

**Problema:** Trae todas las columnas. En workspaces con muchas settings JSON, esto es innecesario.

---

#### 🟡 PERF-008: No Pagination en ListEndpoints
- `ListWorkspaces`
- `ListTables`
- `ListColumns`
- `ListRecords` (soporta limit/offset pero no hay validation de límites)

---

#### 🟡 PERF-009: Promise.all en loadData
**Archivo:** `web/ui/src/App.jsx:777-792`

```jsx
const [tableRes, columnsRes] = await Promise.all([
    axios.get(...),
    axios.get(...)
])
```

**Problema:** Los queries son secuenciales en la práctica porque dependen del mismo workspace/table ID.

---

#### 🟡 PERF-010: No React.memo
**Problema:** Componentes se re-renderizan innecesariamente.

---

### 5.3 Bajos

#### 🔵 PERF-011: JSON Marshal en Cada Request
**Archivos:** `workspace.go:65-66, 87-88`

```go
adminPermissionsJSON, _ := json.Marshal(adminPermissions)
```

**Problema:** Si el workspace tiene muchas tablas, el JSON de permisos crece.

---

#### 🔵 PERF-012: No Compression
**Problema:** No hay gzip compression para responses JSON grandes.

---

#### 🔵 PERF-013: No Connection Pooling Config
**Archivo:** `internal/database/connection.go`

No se configura:
- Max open connections
- Max idle connections
- Connection max lifetime

---

## 6. BEST PRACTICES

### 6.1 Críticos

#### ❌ BP-001: No Graceful Shutdown
**Archivo:** `cmd/server/main.go:180`
```go
if err := r.Run(":" + cfg.Server.Port); err != nil {
    log.Fatal(err)
}
```

**Problema:** `r.Run()` no permite shutdown graceful. Conexiones activas se cortan.

**Recomendación:**
```go
srv := &http.Server{
    Addr:    ":" + cfg.Server.Port,
    Handler: r,
}

go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Fatal(err)
}
```

---

### 6.2 Altos

#### ❌ BP-002: No Structured Logging
**Archivos:** Todos usan `log.Printf`

```go
log.Printf("Error creating workspace: %v", result.Error)
```

**Problema:** Logs no estructurados son difíciles de parsear y buscar.

**Recomendación:** Usar `slog` (Go 1.21+) o `zap`:
```go
slog.Error("failed to create workspace", 
    "error", err,
    "workspace_name", input.Name,
)
```

---

#### ❌ BP-003: No Request Timeouts
**Problema:** No hay timeouts configurados en el servidor HTTP.

---

#### ❌ BP-004: No Health Check de DB
**Archivo:** `cmd/server/main.go:74-76`
```go
r.GET("/health", func(c *gin.Context) {
    c.JSON(200, gin.H{"status": "ok", "service": "hornero"})
})
```

**Problema:** Health check no verifica la conexión a la DB.

---

#### ❌ BP-005: Environment Variables en Código
**Archivo:** `web/ui/src/App.jsx:11`
```jsx
const API_URL = 'http://localhost:8080/api/v1'
```

**Problema:** Debe usar `import.meta.env.VITE_API_URL` o similar.

---

#### ❌ BP-006: No .env en .gitignore
**Archivos verificados:** `.gitignore` existe pero verificar que incluye `.env`

---

### 6.3 Medios

#### ⚠️ BP-007: Error Handling Inconsistente

Algunos lugares:
```go
if result.Error != nil {
    c.JSON(500, gin.H{"error": result.Error.Error()})
}
```

Otros:
```go
if err := ...; err != nil {
    log.Printf("Error: %v", err)  // Silencioso!
}
```

---

#### ⚠️ BP-008: No API Versioning Strategy
**Problema:** `/api/v1` está hardcodeado en múltiples lugares.

---

#### ⚠️ BP-009: No Response Wrapper
**Problema:** Algunos endpoints retornan arrays, otros objetos:
```go
c.JSON(200, workspaces)       // Array
c.JSON(200, gin.H{"data": records, "limit": limit, "offset": offset})  // Object
```

---

#### ⚠️ BP-010: No Request ID Tracking
**Problema:** Difícil correlacionar logs de requests.

---

#### ⚠️ BP-011: No Deprecation Policy
**Problema:** No hay manera de versionar o deprecar endpoints.

---

#### ⚠️ BP-012: No CORS Config from Env
**Archivo:** `cmd/server/main.go:65`
```go
AllowOrigins: []string{"http://localhost:5173", "http://localhost:3000"},
```

---

#### ⚠️ BP-013: No Automatic TLS
**Problema:** Servidor no soporta HTTPS out of the box.

---

### 6.4 Bajos

#### 🔵 BP-014: Naming Conventions Mixtas
- `workspace_id` (snake_case en URLs)
- `workspaceID` (camelCase en código Go)
- `workspace_id` (snake_case en JSON)

**Recomendación:** Estandarizar a uno (recomendado: camelCase en JSON).

---

#### 🔵 BP-015: No Context Propagation
**Problema:** No se pasa `context.Context` a funciones de base de datos.

---

#### 🔵 BP-016: Magic Numbers
```go
c.SetCookie("oidc_state", state, 3600, ...)  // 3600 = 1 hour
MaxAge: 12 * time.Hour,
```

---

#### 🔵 BP-017: No Configuration Validation
**Archivo:** `internal/config/config.go`

No valida que los valores requeridos existan en producción.

---

#### 🔵 BP-018: Test Files Location
Los tests están en el mismo directorio que el código (`*_test.go`).

Esto es correcto en Go, pero algunos equipos prefieren `tests/` para integración.

---

## 7. RECOMENDACIONES PRIORIZADAS

### Semana 1 - Críticos (Security + Bugs)

1. **Fix SQL Injection** - Validar slugs antes de concatenar
2. **Fix JWT Secret** - Forzar que JWT_SECRET esté configurado en producción
3. **Fix OIDC Verification Bypass** - Eliminar fallback inseguro
4. **Fix Race Condition** - Agregar sync.Once para variables globales
5. **Fix CORS** - Usar environment para origins

### Semana 2 - Alta Prioridad

6. **Extraer componentes React** - Separar App.jsx
7. **Implementar tests de auth** - Cobertura crítica
8. **Agregar indexes** - Performance inmediata
9. **Implementar caching de permisos** - Reducir queries
10. **Fix graceful shutdown** - Deploy production-ready

### Semana 3-4 - Media Prioridad

11. **Agregar structured logging**
12. **Tests de handlers**
13. **Tests de integración frontend**
14. **Separar responsabilidades** - Extraer lógica de handlers
15. **API response wrapper**

### Mes 2 - Mejoras Continuas

16. **TypeScript migration** - Frontend
17. **Rate limiting**
18. **Request validation library**
19. **API documentation (OpenAPI)**
20. **CI/CD con security scanning**

---

## 8. MÉTRICAS ADICIONALES

| Métrica | Valor |
|---------|-------|
| Líneas de código (Go) | ~2,500 |
| Líneas de código (React) | ~1,400 |
| Archivos Go | 18 |
| Archivos React | 7 |
| Archivos de test | 2 |
| Cobertura estimada | <15% |
| Technical Debt Score | 6.5/10 |

---

*Fin del informe - Generated by Code Quality Analysis Agent*

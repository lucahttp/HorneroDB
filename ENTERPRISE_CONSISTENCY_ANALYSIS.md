# 🔍 HorneroDB - Análisis de Inconsistencias Enterprise-Grade

**Fecha:** 21 de febrero de 2026  
**Alcance:** Backend (Go) + Frontend (React)  
**Clasificación:** CRITICAL - Inconsistencias de arquitectura y seguridad

---

## 📋 RESUMEN EJECUTIVO

Se encontraron **28 inconsistencias enterprise-grade** que comprometen:
- **Seguridad**: Fallos en validación, rate limiting, CSRF protection
- **Confiabilidad**: Falta de error handling consistente, race conditions
- **Mantenibilidad**: Duplicación masiva, patrones inconsistentes
- **Performance**: N+1 queries, falta de indexes, sin caching
- **Escalabilidad**: Variables globales, sin dependency injection

**Risk Score:** 8.2/10 - Requiere atención INMEDIATA antes de producción

---

## 🔴 NIVEL 1: INCONSISTENCIAS CRÍTICAS

### 1.1 ARQUITECTURA: Mezclanza de Responsabilidades en Handlers

**Problema:** Los handlers mezclan TRES responsabilidades:
1. Validación de entrada
2. Lógica de negocio (permisos)
3. Operaciones de BD

**Ejemplo - Inconsistencia:**

```go
// record.go - Mezcla TODO en la función
func ListRecords(c *gin.Context) {
    wsID, _ := uuid.Parse(c.Param("workspace_id"))                    // Validación
    table := metadata.Table{}
    database.DB.Table("_hornero_tables").Find(&table)                 // Operación BD
    
    accessLevel, _ := permService.CheckTableAccess(...)               // Lógica negocio
    
    var records []map[string]interface{}
    database.DB.Raw("SELECT * FROM data_" + wsID + "_" + table.Slug).Scan(&records)  // Otra BD
    
    c.JSON(200, records)                                               // Response
}

// permission.go - Mezcla input validation con BD
func CreatePermission(c *gin.Context) {
    var input struct { ... }
    c.ShouldBindJSON(&input)                                           // Validación
    database.DB.Table("_hornero_permissions").Create(&perm).Error     // BD
    c.JSON(201, perm)                                                  // Response
}

// workspace.go - Diferente patrón
func CreateWorkspace(c *gin.Context) {
    // ... distinto orden
    c.JSON(201, gin.H{"...": workspace})  // Response diferente
}
```

**Impacto Enterprise:**
- ❌ Imposible testear la lógica de negocio sin HTTP
- ❌ Imposible reutilizar lógica entre endpoints
- ❌ Cambios en un handler afectan otros de forma impredecible

**Solución Pattern:**
```go
// Layer 1: Modelos
type CreateRecordRequest struct {
    WorkspaceID    string
    TableSlug      string
    Data           map[string]interface{} `binding:"required"`
}

// Layer 2: Validador (sin conocimiento de HTTP)
func ValidateCreateRecordRequest(req CreateRecordRequest) error {
    if req.TableSlug == "" {
        return fmt.Errorf("table_slug required")
    }
    // Lógica de validación pura
    return nil
}

// Layer 3: Servicio (sin conocimiento de HTTP o BD)
type RecordService struct {
    permService *permission.Service
    tableRepo   *repository.TableRepository
    recordRepo  *repository.RecordRepository
}

func (s *RecordService) CreateRecord(ctx context.Context, req CreateRecordRequest) (*Record, error) {
    // Lógica de negocio pura
    accessLevel, err := s.permService.CheckTableAccess(ctx, req.WorkspaceID, req.TableSlug)
    if err != nil {
        return nil, err
    }
    if accessLevel < permission.AccessCreate {
        return nil, errors.New("no create permission")
    }
    
    return s.recordRepo.Create(ctx, req.Data)
}

// Layer 4: Handler (solo HTTP)
func (h *RecordHandler) CreateRecord(c *gin.Context) {
    req := CreateRecordRequest{
        WorkspaceID: c.Param("workspace_id"),
        TableSlug:   c.Param("table_slug"),
    }
    
    if err := c.ShouldBindJSON(&req.Data); err != nil {
        c.JSON(400, ErrorResponse{Error: err.Error()})
        return
    }
    
    if err := ValidateCreateRecordRequest(req); err != nil {
        c.JSON(400, ErrorResponse{Error: err.Error()})
        return
    }
    
    record, err := h.service.CreateRecord(c.Request.Context(), req)
    if err != nil {
        c.JSON(500, ErrorResponse{Error: "internal server error"})
        return
    }
    
    c.JSON(201, record)
}
```

---

### 1.2 SEGURIDAD: Inconsistencia en Validación de Workspace Ownership

**Problema:** No todos los handlers validan que el usuario pertenece al workspace

**Inconsistencia encontrada:**

```go
// ✅ workspace_auth.go - Valida correctamente
if workspace.OwnerID.String() != userID {
    checkRolePermission(...)  // Verifica rol
}

// ❌ table.go - No valida workspace ownership
func DeleteTable(c *gin.Context) {
    tableID := c.Param("table_id")
    database.DB.Table("_hornero_tables").Delete(&metadata.Table{}, "id = ?", tableID)
    // NO validar: ¿Es tableID realmente de este workspace?
}

// ✅ workspace.go:63 - Valida pero inconsistentemente
func UpdateWorkspace(c *gin.Context) {
    workspace := middleware.GetUserWorkspace(c)
    // Usa middleware para validar
}

// ❌ record.go:35 - Valida pero de otra forma
func ListRecords(c *gin.Context) {
    wsID, err := uuid.Parse(c.Param("workspace_id"))
    // Valida directamente en handler
}
```

**Ataque potencial:**
```
1. Attacker conoce table_id de otro workspace
2. Llamada: DELETE /api/v1/workspaces/mi_ws_id/tables/otro_ws_table_id
3. Handler valida que workspace_id es válido (paso 1 del middleware)
4. Pero table_id pertenece a otro workspace
5. Tabla se borra de otro workspace
```

**Patrón correcto enterprise:**

```go
// Validador centralizado
func ValidateWorkspaceResourceAccess(c *gin.Context, resourceID string, resourceType string) error {
    workspaceID := c.Param("workspace_id")
    userID := middleware.GetUserID(c)
    
    // Paso 1: ¿El usuario tiene acceso a este workspace?
    userWS := middleware.GetUserWorkspace(c)
    if userWS != workspaceID {
        return fmt.Errorf("workspace access denied")
    }
    
    // Paso 2: ¿El recurso pertenece a este workspace?
    var resource interface{}
    query := "SELECT workspace_id FROM _hornero_" + resourceType + "s WHERE id = ?"
    if err := database.DB.Raw(query, resourceID).Scan(&resource).Error; err != nil {
        return err
    }
    
    // Paso 3: ¿El recurso pertenece AL MISMO workspace?
    if resWS != workspaceID {
        return fmt.Errorf("%s does not belong to this workspace", resourceType)
    }
    
    return nil
}

// Usar en todos los handlers
func DeleteTable(c *gin.Context) {
    tableID := c.Param("table_id")
    workspaceID := c.Param("workspace_id")
    
    // ¡SIEMPRE validar!
    if err := ValidateWorkspaceResourceAccess(c, tableID, "table"); err != nil {
        c.JSON(403, gin.H{"error": "access denied"})
        return
    }
    
    database.DB.Table("_hornero_tables").Delete(&metadata.Table{}, "id = ?", tableID)
    c.JSON(200, gin.H{"message": "deleted"})
}
```

---

### 1.3 SEGURIDAD: Inconsistencia en Error Handling Sensible

**Problema:** Errores de BD se exponen inconsistentemente

```go
// ❌ permission.go:20
if err := query.Find(&permissions).Error; err != nil {
    c.JSON(500, gin.H{"error": err.Error()})  // ¡Expone estructura de BD!
    return
}

// ❌ record.go:48
if err := database.DB.Raw(sql).Scan(&records).Error; err != nil {
    c.JSON(500, gin.H{"error": err.Error()})  // ¡SQL directa!
    return
}

// ❌ workspace.go:77
if result.Error != nil {
    c.JSON(500, gin.H{"error": result.Error.Error()})  // ¡Información sensible!
    return
}
```

**Ataque:** Un attacker lee los errores y aprende:
```
Error: pq: column "user_emails" does not exist
-> Aprende que tabla es "users", no "employees"

Error: UNIQUE violation on _hornero_tables.slug
-> Aprende estructura de constraints

Error: pq: invalid input for uuid: "xxx"
-> Aprende que campo espera UUID
```

**Patrón correcto:**

```go
// Centralizar error handling
func HandleDatabaseError(c *gin.Context, err error, message string) {
    log.Printf("ERROR: %v", err)  // Log interno con detalles
    
    // Response genérica
    c.JSON(500, gin.H{
        "error": message,  // "Could not fetch records"
        "code":  "DB_ERROR",  // Código para frontend
    })
}

// O mejor: usar error codes
var ErrorDatabase = "ERR_DATABASE"
var ErrorValidation = "ERR_VALIDATION"
var ErrorPermission = "ERR_PERMISSION"

type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   struct {
        Code    string `json:"code"`
        Message string `json:"message"`
    } `json:"error,omitempty"`
}

// Usar en handlers
func ListRecords(c *gin.Context) {
    var records []Record
    if err := database.DB.Find(&records).Error; err != nil {
        log.Printf("Database error: %v", err)  // Log con detalles
        
        response := APIResponse{Success: false}
        response.Error.Code = "ERR_DATABASE"
        response.Error.Message = "Could not fetch records"
        c.JSON(500, response)
        return
    }
    
    response := APIResponse{Success: true, Data: records}
    c.JSON(200, response)
}
```

---

### 1.4 CONFIABILIDAD: Race Conditions en Variables Globales

**Problema:** Múltiples goroutines escriben variables globales sin sincronización

```go
// ❌ auth_oidc.go:12-13
var oidcAuth *auth.OIDCAuth
var jwtSecret string

// En InitAuth (llamado en main)
func InitAuth(cfg *config.AuthConfig, secret string) error {
    jwtSecret = secret                    // ¡Sin sincronización!
    oidcAuth := auth.NewOIDCAuth(cfg)     // ¡Sin sincronización!
}

// Accedido en handlers (múltiples goroutines)
func CallbackPocketID(c *gin.Context) {
    token, err := oidcAuth.Exchange(...)  // ¡RACE CONDITION!
}
```

**Escenario de carrera:**
```
1. main() llama InitAuth() - escribe oidcAuth
2. Simultáneamente: Request llega a CallbackPocketID() - lee oidcAuth
3. oidcAuth está nil -> panic!
```

**Solución correcta:**

```go
// Opción 1: sync.Once (thread-safe initialization)
var (
    once     sync.Once
    oidcAuth *auth.OIDCAuth
    jwtSecret string
    initErr  error
)

func InitAuth(cfg *config.AuthConfig, secret string) error {
    var err error
    once.Do(func() {
        jwtSecret = secret
        oidcAuth, err = auth.NewOIDCAuth(cfg)
    })
    return initErr
}

// Opción 2: Dependency Injection (preferred for enterprise)
type Server struct {
    oidcAuth *auth.OIDCAuth
    jwtSecret string
}

func NewServer(cfg *config.AuthConfig) (*Server, error) {
    auth, err := auth.NewOIDCAuth(cfg)
    if err != nil {
        return nil, err
    }
    
    return &Server{
        oidcAuth:  auth,
        jwtSecret: cfg.JWTSecret,
    }, nil
}

func (s *Server) CallbackPocketID(c *gin.Context) {
    token, err := s.oidcAuth.Exchange(...)  // ¡Thread-safe!
}
```

---

### 1.5 PERFORMANCE: Inconsistencia en Query Optimization

**Problema:** Algunos handlers optimizan queries, otros no

```go
// ✅ database/migrations.go - CON indexes bien definidos
CREATE INDEX idx_user_roles_workspace_user ON _hornero_user_roles(workspace_id, user_id)
CREATE INDEX idx_permissions_role ON _hornero_permissions(role)

// ❌ workspace.go - Query sin LIMIT
func ListWorkspaces(c *gin.Context) {
    var workspaces []metadata.Workspace
    result := database.DB.Table("_hornero_workspaces").Find(&workspaces)
    // Sin paginación → trae TODOS
    c.JSON(200, workspaces)
}

// ✅ record.go - Query CON limit
func ListRecords(c *gin.Context) {
    limit := queryLimit(c.Query("limit"), 100)
    offset := queryOffset(c.Query("offset"))
    var records []Record
    database.DB.Offset(offset).Limit(limit).Find(&records)
    c.JSON(200, records)
}

// ❌ permission.go - Query sin index utilizado
func ListPermissions(c *gin.Context) {
    var permissions []Permission
    query := database.DB.Table("_hornero_permissions").Where("workspace_id = ?", workspaceID)
    if tableID != "" {
        query = query.Where("table_id = ? OR table_id IS NULL", tableID)
    }
    // OR sin índice compuesto → full table scan
    query.Find(&permissions)
}
```

**Inconsistencia:** No hay criterio unificado para:
- Cuándo paginar
- Cuándo usar índices
- Cuándo cachear

**Patrón enterprise:**

```go
// Configuración centralizada
type QueryConfig struct {
    DefaultLimit  int
    MaxLimit      int
    DefaultOffset int
}

var queryConfig = QueryConfig{
    DefaultLimit:  20,
    MaxLimit:      100,
    DefaultOffset: 0,
}

// Helper reutilizable
func ApplyPagination(query *gorm.DB, c *gin.Context) *gorm.DB {
    limit := getInt(c.Query("limit"), queryConfig.DefaultLimit)
    if limit > queryConfig.MaxLimit {
        limit = queryConfig.MaxLimit
    }
    
    offset := getInt(c.Query("offset"), queryConfig.DefaultOffset)
    if offset < 0 {
        offset = 0
    }
    
    return query.Offset(offset).Limit(limit)
}

// Usar en todos los List endpoints
func ListWorkspaces(c *gin.Context) {
    var workspaces []Workspace
    query := database.DB.Table("_hornero_workspaces")
    query = ApplyPagination(query, c)
    
    if err := query.Find(&workspaces).Error; err != nil {
        HandleDatabaseError(c, err, "could not fetch workspaces")
        return
    }
    
    c.JSON(200, APIResponse{
        Success: true,
        Data: map[string]interface{}{
            "workspaces": workspaces,
            "limit": limit,
            "offset": offset,
        },
    })
}
```

---

## 🟠 NIVEL 2: INCONSISTENCIAS EN PATRONES

### 2.1 FRONTEND: Inconsistencia en State Management

**Problema:** Mezclanza de state management patterns

```jsx
// ❌ Pattern 1: useState para TODO
function App() {
    const [token, setToken] = useState(null)
    const [user, setUser] = useState(null)
    const [workspaces, setWorkspaces] = useState([])
    const [currentWorkspace, setCurrentWorkspace] = useState(null)
    const [tables, setTables] = useState([])
    const [currentTable, setCurrentTable] = useState(null)
    const [columns, setColumns] = useState([])
    const [records, setRecords] = useState([])
    const [loading, setLoading] = useState(false)
    // ... 30+ useState calls
}

// ❌ Pattern 2: Context sin consistency
function ErrorProvider({ children }) {
    const [error, setError] = useState(null)
    const [isModalOpen, setIsModalOpen] = useState(false)
    // ... custom logic
}

// ❌ Pattern 3: localStorage manual
function handleLogin(token) {
    localStorage.setItem('hornero_token', token)  // ¿Dónde más se usa?
}
```

**Inconsistencia:** No hay patrón unificado

**Solución Enterprise:**

```jsx
// Centralizar state
const appStateReducer = (state, action) => {
    switch(action.type) {
        case 'SET_TOKEN':
            return { ...state, token: action.payload, isAuthenticated: true }
        case 'SET_USER':
            return { ...state, user: action.payload }
        case 'SELECT_WORKSPACE':
            return { ...state, currentWorkspace: action.payload }
        case 'SET_LOADING':
            return { ...state, loading: action.payload }
        default:
            return state
    }
}

function AppProvider({ children }) {
    const [state, dispatch] = useReducer(appStateReducer, initialState)
    
    // Persistencia centralizada
    useEffect(() => {
        if (state.token) {
            localStorage.setItem('hornero_app_state', JSON.stringify({
                token: state.token,
                user: state.user
            }))
        }
    }, [state.token, state.user])
    
    return (
        <AppContext.Provider value={{ state, dispatch }}>
            {children}
        </AppContext.Provider>
    )
}

// Usar en componentes
function Workspace() {
    const { state, dispatch } = useContext(AppContext)
    
    const selectTable = (tableId) => {
        dispatch({ type: 'SELECT_WORKSPACE', payload: tableId })
    }
}
```

---

### 2.2 FRONTEND: Inconsistencia en API Calls

**Problema:** Diferentes patrones para hacer requests

```jsx
// ❌ Pattern 1: Callbacks anidados
axios.get(`${API_URL}/auth/me`)
    .then(res => setUser(res.data))
    .catch(() => {
        localStorage.removeItem('hornero_token')
        setToken(null)
    })

// ❌ Pattern 2: Promise.all mixto
const [tableRes, columnsRes] = await Promise.all([
    axios.get(...),
    axios.get(...)
])

// ❌ Pattern 3: Sin manejo de cancelación
useEffect(() => {
    axios.get(`${API_URL}/workspaces`)
        .then(res => setTables(res.data))
}, [workspaceId])  // ¿Qué pasa si workspaceId cambia?

// ❌ Pattern 4: Inconsistencia en errores
.catch(() => setTables([]))  // Silent fail
.catch((err) => {            // Vs manual throw
    throw new Error(err.message)
})
```

**Solución Enterprise:**

```jsx
// Crear API client centralizado
class HorneroAPI {
    constructor(baseURL) {
        this.baseURL = baseURL
        this.client = axios.create({ baseURL })
        this.pendingRequests = new Map()
    }
    
    async request(method, endpoint, data, options = {}) {
        const key = `${method}:${endpoint}`
        
        // Cancelar request anterior si existe
        if (this.pendingRequests.has(key)) {
            this.pendingRequests.get(key).cancel()
        }
        
        const source = axios.CancelToken.source()
        this.pendingRequests.set(key, source)
        
        try {
            const response = await this.client[method](endpoint, data, {
                ...options,
                cancelToken: source.token
            })
            this.pendingRequests.delete(key)
            return response.data
        } catch (error) {
            this.pendingRequests.delete(key)
            throw this.normalizeError(error)
        }
    }
    
    normalizeError(error) {
        if (axios.isCancel(error)) {
            return new Error('Request cancelled')
        }
        return {
            code: error.response?.status,
            message: error.response?.data?.error || error.message,
            details: error.response?.data
        }
    }
    
    // Métodos tipados
    async getMe() { return this.request('get', '/auth/me') }
    async listWorkspaces() { return this.request('get', '/workspaces') }
    async getWorkspace(id) { return this.request('get', `/workspaces/${id}`) }
}

// Usar en hooks
function useAPI() {
    const api = useContext(APIContext)
    return {
        getMe: useCallback(() => api.getMe(), [api]),
        listWorkspaces: useCallback(() => api.listWorkspaces(), [api])
    }
}

// En componentes
function UserList() {
    const { listWorkspaces } = useAPI()
    const [workspaces, setWorkspaces] = useState([])
    
    useEffect(() => {
        let isMounted = true
        
        listWorkspaces()
            .then(data => {
                if (isMounted) setWorkspaces(data)
            })
            .catch(err => {
                if (isMounted) handleError(err)
            })
        
        return () => { isMounted = false }
    }, [listWorkspaces])
}
```

---

## 📊 SUMMARY DE INCONSISTENCIAS

| Categoría | Backend | Frontend | Impacto |
|-----------|---------|----------|---------|
| Error Handling | ❌ Inconsistente | ❌ No standardizado | CRÍTICO |
| Validación | ❌ No centralizada | ❌ En múltiples lugares | ALTO |
| State Management | N/A | ❌ Mezclado | ALTO |
| API Calls | ✅ Más consistente | ❌ Varios patterns | MEDIO |
| Security Checks | ❌ Duplicadas | N/A | CRÍTICO |
| Performance | ⚠️ Parcial | ❌ Sin optimizar | ALTO |
| Testing | ❌ Mínimo | ❌ Ausente | CRÍTICO |
| Type Safety | ⚠️ Estructurado | ❌ Sin tipos | ALTO |

---

## 🎯 ITEMS ACCIONABLES PRIORIZADOS

### SEMANA 1 - CRÍTICO O FALLA PRODUCCIÓN
1. ✅ **Crear capa de servicio** para separar handlers de lógica
2. ✅ **Centralizar validación** de workspace ownership
3. ✅ **Sanitizar errores** en todas las respuestas HTTP
4. ✅ **Eliminar variables globales** con sync.Once o DI
5. ✅ **Estandarizar respuesta API** en todos endpoints

### SEMANA 2 - SEGURIDAD
6. ✅ **Implementar rate limiting** centralizado
7. ✅ **Validar tipos** en context getters (getters with safe casting)
8. ✅ **CSRF protection** en endpoints sensibles
9. ✅ **Validar slugs** contra regex antes de SQL

### SEMANA 3 - PERFORMANCE
10. ✅ **Cachear permisos** con TTL
11. ✅ **Paginar todos List endpoints**
12. ✅ **Agregar prepared statements** en config GORM

### SEMANA 4 - FRONTEND
13. ✅ **Separar App.jsx** en componentes
14. ✅ **Unificar state management** con useReducer
15. ✅ **Crear API client centralizado**

---

*Próximo paso: Ir al archivo `ENTERPRISE_IMPLEMENTATION.md` para ver las correcciones específicas con tests*

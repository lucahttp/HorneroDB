## Análisis Completo de Cumplimiento Normativo (HIPAA, GDPR, SOC2, ISO27001) en HorneroDB
Análisis completo del estado actual de cumplimiento normativo de HorneroDB y las brechas críticas identificadas para HIPAA, GDPR, SOC2 e ISO27001. Cubre desde la [1a] infraestructura de autenticación existente hasta las [7d] necesidades de auditoría, mostrando la base sólida actual y los puntos específicos que requieren implementación para cumplimiento completo.

### 1. Infraestructura de Autenticación y Control de Acceso
Sistema central de autenticación OIDC y middleware que forma la base para todos los estándares de cumplimiento

### 1a. Middleware de Autenticación Global (`main.go:106`)
Protege todas las rutas API con autenticación JWT o API Key
```text
protected.Use(middleware.AuthRequired(cfg.Auth.JWTSecret))
```

### 1b. Validación de Credenciales (`auth.go:34`)
Extrae y valida el token Bearer de la cabecera
```text
authHeader := c.GetHeader("Authorization")
```

### 1c. Parseo de Claims JWT (`auth.go:55`)
Decodifica el token JWT con claims estructurados
```text
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
```

### 1d. Autorización por Workspace (`main.go:117`)
Verifica acceso específico al workspace solicitado
```text
workspaceGroup.Use(middleware.WorkspaceAuth())
```

### 2. Motor de Permisos Granulares y Row-Level Security
Sistema de permisos a nivel de tabla, columna y fila requerido por HIPAA y GDPR

### 2a. Evaluación de Permisos (`engine.go:42`)
Función central que verifica permisos de acceso
```text
func (e *PermissionEngine) CheckPermission(check PermissionCheck) (bool, string, error) {
```

### 2b. Permisos por Operación (`engine.go:63`)
Valida permisos específicos según operación CRUD
```text
case "read": hasPermission = perm.AllowRead
```

### 2c. Filtros a Nivel de Fila (`engine.go:84`)
Aplica filtros dinámicos para row-level security
```text
if perm.RowFilter != nil && len(perm.RowFilter) > 0 {
```

### 2d. Control de Acceso a Tablas (`service.go:136`)
Verifica acceso a nivel de tabla con niveles none/own/all
```text
func (s *Service) CheckTableAccess(workspaceID uuid.UUID, roleName, tableSlug, operation string) (AccessLevel, error) {
```

### 3. Validación de Recursos y Aislamiento Multi-Tenant
Sistema que asegura aislamiento completo entre workspaces para cumplimiento de seguridad

### 3a. Validador de Recursos (`validate_resource.go:37`)
Verifica que los recursos pertenezcan al workspace
```text
func (v *ResourceValidator) ValidateResourceAccess(c *gin.Context, resourceType string, resourceID string) error {
```

### 3b. Consulta de Propiedad (`validate_resource.go:69`)
Verifica en BD que el recurso pertenece al workspace
```text
result := database.DB.Table(tableName).Where("id = ? AND workspace_id = ?", resID, wsID).Count(&count)
```

### 3c. Middleware de Tablas (`main.go:313`)
Aplica validación a rutas de operaciones de tablas
```text
tableGroup.Use(middleware.ValidateTableAccess())
```

### 3d. Middleware de Columnas (`main.go:322`)
Extiende validación a operaciones de columnas
```text
columnGroup.Use(middleware.ValidateColumnAccess())
```

### 4. Gestión de API Keys y Control de Origen
Sistema de API keys con restricciones y rate limiting para seguridad programática

### 4a. Verificación de API Key (`auth.go:143`)
Valida y procesa API keys para acceso programático
```text
func verifyAPIKey(key string) (*metadata.APIKey, error) {
```

### 4b. Hash Seguro de API Key (`auth.go:153`)
Genera hash SHA-256 para comparación segura
```text
hash := sha256.Sum256([]byte(key))
```

### 4c. Configuración de Rate Limit (`auth.go:183`)
Establece límites de tasa por API key
```text
c.Set("api_key_rate_limit", apiKey.RateLimitPerMin)
```

### 4d. Restricción de Orígenes (`role.go:89`)
Almacena orígenes permitidos para CORS por API key
```text
AllowedOrigins JSON `gorm:"type:jsonb" json:"allowed_origins,omitempty"`
```

### 5. Estructura de Datos para Auditoría y Trazabilidad
Modelos de datos con campos de auditoría requeridos para cumplimiento normativo

### 5a. Registro de Actividad (`user.go:16`)
Campo para auditoría de últimos accesos
```text
LastLoginAt time.Time `json:"last_login_at"`
```

### 5b. Uso de API Keys (`role.go:85`)
Registro de última utilización de API keys
```text
LastUsedAt *time.Time `json:"last_used_at"`
```

### 5c. Caducidad de Credenciales (`role.go:86`)
Control de vida útil de credenciales
```text
ExpiresAt *time.Time `json:"expires_at"`
```

### 5d. Asignación de Roles (`role.go:63`)
Timestamp para auditoría de asignaciones
```text
AssignedAt time.Time `json:"assigned_at"`
```

### 6. Configuración de Seguridad y Validaciones de Producción
Sistema de configuración centralizada con validaciones para entorno de producción

### 6a. Validación de Secretos (`config.go:72`)
Fuerza cambio de secretos en producción
```text
if isProduction && jwtSecret == "change-me-in-production" {
```

### 6b. Longitud Mínima de JWT (`config.go:77`)
Requiere longitud mínima para secretos JWT
```text
if isProduction && len(jwtSecret) < 32 {
```

### 6c. Configuración OIDC (`config.go:40`)
Estructura para proveedores de identidad externos
```text
type OIDCProvider struct {
```

### 6d. Detección de Entorno (`config.go:62`)
Identifica entorno de producción para aplicar políticas estrictas
```text
isProduction := os.Getenv("NODE_ENV") == "production"
```

### 7. Brechas Críticas de Cumplimiento y Puntos de Extensión
Áreas identificadas que requieren implementación para cumplimiento normativo completo

### 7a. Migraciones de Base de Datos (`migrations.go:55`)
Punto para agregar campos de auditoría y cumplimiento
```text
// Add new columns to existing API keys table if they don't exist
```

### 7b. Requisito de Encriptación (`WHISHES.md:199`)
Necesidad identificada para encriptación a nivel de columna
```text
allow column level encryption, only end user can see his data
```

### 7c. Políticas de Backup (`WHISHES.md:223`)
Requisito para backups automatizados con retención
```text
allow automated backups of the database, with retention policies
```

### 7d. Sistema de Auditoría (`WHISHES.md:48`)
Necesidad de logs de cambios para cumplimiento
```text
Auditoría / log de cambios — historial de modificaciones por registro
```

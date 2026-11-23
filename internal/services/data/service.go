package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"hornerodb/internal/database"
	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"
	"hornerodb/internal/services/permission"
	"hornerodb/internal/workers"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// validTableNamePattern valida que el nombre de tabla física sea seguro
// Pattern: data_<uuid>_<slug>
var validTableNamePattern = regexp.MustCompile(`^data_[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}_[a-zA-Z][a-zA-Z0-9_]*$`)

// validColumnNamePattern valida que un identificador de columna sea seguro para
// interpolarse en SQL (usado como segunda barrera además del quoting).
var validColumnNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// systemColumns son columnas de sistema siempre legibles (id + auditoría).
var systemColumns = map[string]bool{
	"id":         true,
	"created_at": true,
	"updated_at": true,
	"created_by": true,
}

// quoteIdentifier escapa un identificador SQL (tabla/columna) para usarlo
// dentro de comillas dobles. Escapa las comillas dobles duplicándolas
// (standard SQL) y neutraliza cualquier otro caracter.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// validateTableName verifica que el nombre de tabla sea seguro para usar en SQL
func validateTableName(tableName string) bool {
	return validTableNamePattern.MatchString(tableName)
}

// MaxBulkRecords es el máximo de registros (o ids) aceptados por una única
// petición bulk. Es un límite duro para evitar transacciones巨大.
const MaxBulkRecords = 500

// ValidationError marca un error causado por una entrada de usuario inválida
// (columna inexistente o no legible, operador desconocido, id mal formado...).
// Los handlers lo traducen a HTTP 400 en lugar de a un error 500.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// IsValidationError indica si err es (o envuelve) un ValidationError.
func IsValidationError(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}

// newValidationError construye un error de validación con formato.
func newValidationError(format string, args ...interface{}) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// RequestContext holds the resolved state needed for data operations
type RequestContext struct {
	WsID        uuid.UUID
	Table       metadata.Table
	TableName   string
	UserID      string
	RoleName    string
	AccessLevel permission.AccessLevel
}

type Service struct {
	permSvc *permission.Service
}

func NewService() *Service {
	return &Service{
		permSvc: permission.NewService(),
	}
}

// ListOptions agrupa los parámetros de listado: paginación, expansión,
// búsqueda de texto libre, ordenamiento, filtros y consulta OData opcional.
type ListOptions struct {
	Limit   int
	Offset  int
	Expand  string
	Search  string
	Sorts   []query.SortClause
	Filters []query.FilterCondition
	OData   *query.ODataQuery
}

// ListRecords devuelve los registros que coinciden con la consulta OData, el total y un error.
//
// Delega en ListRecordsWithQuery para aplicar la misma validación de nombre de
// tabla, CLS sobre columnas reales, RLS y expansión segura de relaciones.
func (s *Service) ListRecords(ctx RequestContext, odata *query.ODataQuery) ([]map[string]interface{}, int, error) {
	opts := ListOptions{
		Limit:  50,
		Offset: 0,
		OData:  odata,
	}
	if odata != nil {
		if odata.Top > 0 {
			opts.Limit = odata.Top
		}
		if odata.Skip >= 0 {
			opts.Offset = odata.Skip
		}
		opts.Expand = odata.Expand
	}
	return s.ListRecordsWithQuery(ctx, opts)
}

// ListRecordsWithQuery devuelve los registros que coinciden con los filtros,
// la búsqueda, el ordenamiento y las opciones OData indicados, el total y un error.
//
// Seguridad:
//
//   - toda columna usada en un filtro, en un ORDER BY, en un $select o en la búsqueda debe
//     existir realmente en la tabla Y ser legible por el rol (CLS). Si no lo
//     es se devuelve un error de validación: nunca se ignora en silencio
//     (eso convertiría un filtro inválido en un oráculo sobre los datos);
//   - los valores viajan siempre como parámetros `?`, nunca concatenados;
//   - los nombres de columna se entrecomillan con quoteIdentifier.
func (s *Service) ListRecordsWithQuery(ctx RequestContext, opts ListOptions) ([]map[string]interface{}, int, error) {
	if !validateTableName(ctx.TableName) {
		return nil, 0, newValidationError("invalid table name format")
	}

	columns, err := s.loadTableColumns(ctx.Table.ID)
	if err != nil {
		return nil, 0, err
	}

	allowedColumns, err := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "read")
	if err != nil {
		return nil, 0, err
	}

	// Conjunto de columnas que el rol puede leer: sistema + reales que pasan
	// la column level security.
	readableCols := buildReadableColumns(columns, allowedColumns)

	// Validar que todas las columnas referenciadas por OData existan en la tabla y sean legibles (CLS).
	if opts.OData != nil {
		for _, col := range opts.OData.ReferencedColumns {
			if !readableCols[col] {
				return nil, 0, newValidationError("column %q does not exist or is not readable", col)
			}
		}
	}

	filterSQL, filterArgs, err := buildFilterClause(opts.Filters, readableCols)
	if err != nil {
		return nil, 0, err
	}

	orderSQL := ""
	if len(opts.Sorts) > 0 || opts.OData == nil || opts.OData.OrderBy == "" {
		orderSQL, err = buildOrderClause(opts.Sorts, readableCols)
		if err != nil {
			return nil, 0, err
		}
	} else {
		orderSQL = opts.OData.OrderBy
	}

	dbQuery := database.DB.Table(ctx.TableName)

	// Row level security: "own" sólo ve sus propios registros.
	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	if filterSQL != "" {
		dbQuery = dbQuery.Where(filterSQL, filterArgs...)
	}

	if opts.OData != nil && opts.OData.FilterSQL != "" {
		dbQuery = dbQuery.Where(opts.OData.FilterSQL, opts.OData.FilterArgs...)
	}

	// Búsqueda de texto libre sobre columnas de texto legibles.
	term := strings.TrimSpace(opts.Search)
	if term != "" {
		searchSQL, searchArgs := buildSearchClause(term, columns, readableCols)
		if searchSQL == "" {
			// No hay ninguna columna de texto legible: el término nunca puede
			// coincidir, así que la respuesta es una lista vacía (sin error).
			return []map[string]interface{}{}, 0, nil
		}
		dbQuery = dbQuery.Where(searchSQL, searchArgs...)
	}

	// Total antes de paginar.
	var totalCount int64
	if err := dbQuery.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	if opts.OData != nil && len(opts.OData.Select) > 0 {
		quotedSelect := make([]string, len(opts.OData.Select))
		for i, col := range opts.OData.Select {
			quotedSelect[i] = quoteIdentifier(col)
		}
		dbQuery = dbQuery.Select(quotedSelect)
	}

	// Paginación (mismo comportamiento histórico: siempre se aplican).
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	dbQuery = dbQuery.Offset(offset).Limit(limit)

	if orderSQL != "" {
		dbQuery = dbQuery.Order(orderSQL)
	}

	var records []map[string]interface{}
	if err := dbQuery.Find(&records).Error; err != nil {
		return nil, 0, err
	}

	// Expand relation columns
	expandParam := opts.Expand
	if expandParam == "" && opts.OData != nil {
		expandParam = opts.OData.Expand
	}
	if expandParam != "" && len(records) > 0 {
		s.expandRecords(ctx, records, expandParam)
	}

	if allowedColumns != nil {
		for _, record := range records {
			s.filterRecordColumns(record, allowedColumns)
		}
	}

	return records, int(totalCount), nil
}

// buildReadableColumns calcula el conjunto de columnas que el rol puede leer.
//
//	allowedColumns == nil              -> sin restricción CLS: todas
//	allowedColumns contiene "*"        -> sin restricción CLS: todas
//	allowedColumns == []string{}       -> ninguna columna real (sólo sistema)
//
// Las columnas de sistema (id, created_at, updated_at, created_by) siempre son
// legibles.
func buildReadableColumns(columns []metadata.Column, allowedColumns []string) map[string]bool {
	readable := make(map[string]bool, len(columns)+len(systemColumns))
	for name := range systemColumns {
		readable[name] = true
	}

	allowedAll := allowedColumns == nil
	allowedSet := make(map[string]bool)
	if !allowedAll {
		for _, col := range allowedColumns {
			if col == "*" {
				allowedAll = true
				break
			}
			allowedSet[col] = true
		}
	}

	for _, col := range columns {
		if !allowedAll && !allowedSet[col.Slug] {
			continue
		}
		readable[col.Slug] = true
	}

	return readable
}

// buildFilterClause construye la cláusula WHERE de los filtros.
//
// Devuelve la cláusula ya parametrizada (valores en `args`) o un error de
// validación si alguna columna no es legible o algún operador no está soportado.
func buildFilterClause(filters []query.FilterCondition, readableCols map[string]bool) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}

	clauses := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))

	for _, filter := range filters {
		column := strings.TrimSpace(filter.Column)
		if column == "" || !validColumnNamePattern.MatchString(column) {
			return "", nil, newValidationError("invalid filter column %q", filter.Column)
		}
		if !readableCols[column] {
			return "", nil, newValidationError("filter column %q is not readable", filter.Column)
		}

		operator := strings.ToLower(strings.TrimSpace(filter.Operator))
		if operator == "" {
			operator = query.DefaultOperator
		}

		quoted := quoteIdentifier(column)

		switch operator {
		case "eq":
			clauses = append(clauses, quoted+" = ?")
			args = append(args, filter.Value)
		case "neq":
			clauses = append(clauses, quoted+" != ?")
			args = append(args, filter.Value)
		case "gt":
			clauses = append(clauses, quoted+" > ?")
			args = append(args, filter.Value)
		case "gte":
			clauses = append(clauses, quoted+" >= ?")
			args = append(args, filter.Value)
		case "lt":
			clauses = append(clauses, quoted+" < ?")
			args = append(args, filter.Value)
		case "lte":
			clauses = append(clauses, quoted+" <= ?")
			args = append(args, filter.Value)
		case "like":
			clauses = append(clauses, quoted+" LIKE ?")
			args = append(args, filter.Value)
		case "ilike":
			clauses = append(clauses, quoted+" ILIKE ?")
			args = append(args, filter.Value)
		case "in":
			values := splitCSV(filter.Value)
			if len(values) == 0 {
				return "", nil, newValidationError("filter %q requires at least one value for the 'in' operator", column)
			}
			clauses = append(clauses, quoted+" IN ("+sqlPlaceholders(len(values))+")")
			for _, value := range values {
				args = append(args, value)
			}
		case "is_null":
			switch strings.ToLower(strings.TrimSpace(filter.Value)) {
			case "true":
				clauses = append(clauses, quoted+" IS NULL")
			case "false":
				clauses = append(clauses, quoted+" IS NOT NULL")
			default:
				return "", nil, newValidationError("filter %q: the 'is_null' operator only accepts 'true' or 'false'", column)
			}
		default:
			return "", nil, newValidationError("unsupported filter operator %q", filter.Operator)
		}
	}

	return strings.Join(clauses, " AND "), args, nil
}

// buildOrderClause construye la cláusula ORDER BY.
//
// Sin sorts explícitos se usa un orden estable por defecto
// (`"created_at" DESC`).
func buildOrderClause(sorts []query.SortClause, readableCols map[string]bool) (string, error) {
	parts := make([]string, 0, len(sorts))

	for _, sort := range sorts {
		column := strings.TrimSpace(sort.Column)
		if column == "" || !validColumnNamePattern.MatchString(column) {
			return "", newValidationError("invalid sort column %q", sort.Column)
		}
		if !readableCols[column] {
			return "", newValidationError("sort column %q is not readable", sort.Column)
		}

		direction := "ASC"
		if sort.Desc {
			direction = "DESC"
		}
		parts = append(parts, quoteIdentifier(column)+" "+direction)
	}

	if len(parts) == 0 {
		return quoteIdentifier("created_at") + " DESC", nil
	}

	return strings.Join(parts, ", "), nil
}

// buildSearchClause construye la cláusula de búsqueda de texto libre.
//
// Sólo se busca sobre columnas legibles de tipo texto (`text`, `long_text`,
// `email`, `url`, `select`). Si no hay ninguna, devuelve una cláusula vacía
// (el llamador responde con una lista vacía).
func buildSearchClause(term string, columns []metadata.Column, readableCols map[string]bool) (string, []interface{}) {
	parts := make([]string, 0, len(columns))

	for _, col := range columns {
		if !readableCols[col.Slug] {
			continue
		}
		if !searchableColumnType(col.FieldType) {
			continue
		}
		parts = append(parts, "CAST("+quoteIdentifier(col.Slug)+" AS TEXT) ILIKE ?")
	}

	if len(parts) == 0 {
		return "", nil
	}

	pattern := "%" + escapeLikePattern(term) + "%"
	args := make([]interface{}, len(parts))
	for i := range args {
		args[i] = pattern
	}

	return "(" + strings.Join(parts, " OR ") + ")", args
}

// searchableColumnType indica si el tipo de campo puede participates en una
// búsqueda de texto libre.
func searchableColumnType(fieldType string) bool {
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "text", "long_text", "email", "url", "select":
		return true
	}
	return false
}

// escapeLikePattern neutraliza los comodines de LIKE/ILIKE para que un
// término de búsqueda sea siempre literal.
func escapeLikePattern(term string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(term)
}

// splitCSV divide una lista separada por comas, ignorando espacios y
// elementos vacíos.
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

// sqlPlaceholders devuelve "?,?,?" para n parámetros.
func sqlPlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// CreateRecord creates a new record and enqueues the webhook event in the SAME
// transaction (transactional outbox): if the outbox insert fails, the record
// creation is rolled back and no "ghost" webhook is delivered.
func (s *Service) CreateRecord(ctx RequestContext, input map[string]interface{}) (map[string]interface{}, error) {
	if input == nil {
		input = map[string]interface{}{}
	}
	input = s.filterDefinedColumns(ctx.Table.ID, input)

	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "create")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
	}

	// Immutable / server-controlled fields: the client can never choose them.
	stripImmutableFields(input)

	if err := s.validateTypedFields(ctx, input); err != nil {
		return nil, newValidationError("%s", err.Error())
	}

	if err := s.generateAutonumberFields(ctx.Table.ID, input); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	input["id"] = uuid.New()
	input["created_by"] = ctx.UserID
	input["created_at"] = now
	input["updated_at"] = now

	recordID := input["id"].(uuid.UUID).String()

	var created map[string]interface{}
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(ctx.TableName).Create(input).Error; err != nil {
			return err
		}

		if err := tx.Table(ctx.TableName).Where("id = ?", recordID).Take(&created).Error; err != nil {
			return err
		}

		return workers.EnqueueWebhookTx(tx, ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "created", created)
	})
	if err != nil {
		return nil, err
	}

	return input, nil
}

// BulkCreateRecords crea varios registros en una única transacción.
//
// Cada fila pasa por las mismas validaciones que un create individual
// (column level security, tipos, campos inmutables) y los eventos de webhook
// "created" se encolan dentro de la MISMA transacción (transactional outbox):
// si algo falla, no queda ningún registro a medias ni ningún webhook
// huérfano.
func (s *Service) BulkCreateRecords(ctx RequestContext, inputs []map[string]interface{}) ([]map[string]interface{}, error) {
	if err := validateBulkSize(len(inputs), "records"); err != nil {
		return nil, err
	}

	if !validateTableName(ctx.TableName) {
		return nil, newValidationError("invalid table name format")
	}

	columns, err := s.loadTableColumns(ctx.Table.ID)
	if err != nil {
		return nil, err
	}

	// Los permisos de escritura se resuelven UNA sola vez para todas las filas.
	writableColumns, err := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "create")
	if err != nil {
		return nil, err
	}

	rows, err := s.prepareBulkRows(ctx, inputs, columns, writableColumns)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(rows))
	rowsByID := make(map[string]map[string]interface{}, len(rows))
	for _, row := range rows {
		id, ok := row["id"].(uuid.UUID)
		if !ok {
			continue
		}
		ids = append(ids, id.String())
		rowsByID[id.String()] = row
	}

	var created []map[string]interface{}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			if err := tx.Table(ctx.TableName).Create(row).Error; err != nil {
				return err
			}
		}

		// Recuperamos los registros ya insertados para que el webhook reciba
		// el payload canónico (valores persistidos por la base).
		var stored []map[string]interface{}
		if err := tx.Table(ctx.TableName).Where("id IN ?", ids).Find(&stored).Error; err != nil {
			return err
		}

		byID := make(map[string]map[string]interface{}, len(stored))
		for _, record := range stored {
			if key, ok := recordIDKey(record["id"]); ok {
				byID[key] = record
			}
		}

		created = make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			record, ok := byID[id]
			if !ok {
				record = rowsByID[id]
			}
			if err := workers.EnqueueWebhookTx(tx, ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "created", record); err != nil {
				return err
			}
			created = append(created, record)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

// prepareBulkRows normaliza, filtra y valida las filas de un bulk create.
//
// Es una función pura (no toca la base) para poder testearla de forma
// aislada: aplica CLS de escritura, descarta los campos inmutables y asigna
// los valores controlados por el servidor.
func (s *Service) prepareBulkRows(ctx RequestContext, inputs []map[string]interface{}, columns []metadata.Column, writableColumns []string) ([]map[string]interface{}, error) {
	now := time.Now().UTC()
	rows := make([]map[string]interface{}, 0, len(inputs))

	for i, input := range inputs {
		// Copia defensiva: nunca mutamos el payload del llamador.
		row := make(map[string]interface{}, len(input)+4)
		for key, value := range input {
			row[key] = value
		}

		// Column level security: se descartan las columnas no escribibles.
		if writableColumns != nil {
			row = s.filterInputColumns(row, writableColumns)
		}

		// Campos inmutables / controlados por el servidor.
		stripImmutableFields(row)

		if err := validateTypedFieldsWithColumns(columns, row); err != nil {
			return nil, newValidationError("record %d: %s", i+1, err.Error())
		}

		row["id"] = uuid.New()
		row["created_by"] = ctx.UserID
		row["created_at"] = now
		row["updated_at"] = now

		rows = append(rows, row)
	}

	return rows, nil
}

// validateBulkSize comprueba el límite de registros de una operación bulk.
func validateBulkSize(count int, what string) error {
	if count == 0 {
		return newValidationError("no %s provided", what)
	}
	if count > MaxBulkRecords {
		return newValidationError("too many %s: the maximum is %d per request", what, MaxBulkRecords)
	}
	return nil
}

// recordIDKey normaliza el valor de la columna "id" a su forma string.
func recordIDKey(value interface{}) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, v != ""
	case uuid.UUID:
		return v.String(), true
	case []byte:
		return string(v), true
	}
	return "", false
}

// GetRecord returns a single record by ID
func (s *Service) GetRecord(ctx RequestContext, recordID string) (map[string]interface{}, error) {
	dbQuery := database.DB.Table(ctx.TableName).Where("id = ?", recordID)

	if ctx.AccessLevel == permission.AccessOwn {
		dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
	}

	var record map[string]interface{}
	if err := dbQuery.Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.New("RECORD_NOT_FOUND")
		}
		return nil, err
	}

	allowedColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "read")
	if allowedColumns != nil {
		s.filterRecordColumns(record, allowedColumns)
	}

	return record, nil
}

// UpdateRecord updates an existing record and enqueues the webhook event in the
// SAME transaction (transactional outbox).
func (s *Service) UpdateRecord(ctx RequestContext, recordID string, input map[string]interface{}) error {
	if input == nil {
		input = map[string]interface{}{}
	}
	input = s.filterDefinedColumns(ctx.Table.ID, input)

	writableColumns, _ := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, ctx.Table.Slug, "update")
	if writableColumns != nil {
		input = s.filterInputColumns(input, writableColumns)
	}

	// Immutable fields: id / created_at / created_by are server controlled.
	// Without this, a client could take ownership of another user's record.
	stripImmutableFields(input)

	if err := s.validateTypedFields(ctx, input); err != nil {
		return newValidationError("%s", err.Error())
	}

	input["updated_at"] = time.Now().UTC()

	return database.DB.Transaction(func(tx *gorm.DB) error {
		dbQuery := tx.Table(ctx.TableName).Where("id = ?", recordID)
		if ctx.AccessLevel == permission.AccessOwn {
			dbQuery = dbQuery.Where("created_by = ?", ctx.UserID)
		}

		result := dbQuery.Updates(input)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return errors.New("RECORD_NOT_FOUND")
		}

		var updated map[string]interface{}
		if err := tx.Table(ctx.TableName).Where("id = ?", recordID).Take(&updated).Error; err != nil {
			return err
		}

		return workers.EnqueueWebhookTx(tx, ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "updated", updated)
	})
}

// DeleteRecord deletes a record.
//
// The whole operation runs inside a transaction and the DELETE itself is
// filtered by `created_by` when the role only has "own" access. Without that
// filter a user with own-level access could delete records belonging to
// other users (RLS bypass).
func (s *Service) DeleteRecord(ctx RequestContext, recordID string) error {
	// Validate table name before using in SQL
	if !validateTableName(ctx.TableName) {
		return errors.New("invalid table name format")
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		readQuery := tx.Table(ctx.TableName).Where("id = ?", recordID)
		if ctx.AccessLevel == permission.AccessOwn {
			readQuery = readQuery.Where("created_by = ?", ctx.UserID)
		}

		var recordToDelete map[string]interface{}
		if err := readQuery.Take(&recordToDelete).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("RECORD_NOT_FOUND")
			}
			return err
		}

		deleteSQL := `DELETE FROM ` + quoteIdentifier(ctx.TableName) + ` WHERE id = ?`
		args := []interface{}{recordID}
		if ctx.AccessLevel == permission.AccessOwn {
			deleteSQL += ` AND created_by = ?`
			args = append(args, ctx.UserID)
		}

		result := tx.Exec(deleteSQL, args...)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return errors.New("RECORD_NOT_FOUND")
		}

		return workers.EnqueueWebhookTx(tx, ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "deleted", recordToDelete)
	})
}

// BulkDeleteRecords borra varios registros en una única transacción.
//
// La lectura previa respeta el RLS ("own") y el DELETE incluye el mismo
// filtro `created_by`, de modo que un rol con acceso "own" nunca puede borrar
// registros de otro usuario. Por cada registro eliminado se encola un
// webhook "deleted" dentro de la misma transacción.
func (s *Service) BulkDeleteRecords(ctx RequestContext, ids []string) (int64, error) {
	if err := validateBulkSize(len(ids), "ids"); err != nil {
		return 0, err
	}

	if !validateTableName(ctx.TableName) {
		return 0, newValidationError("invalid table name format")
	}

	// Todos los ids deben ser UUID válidos antes de tocar la base.
	normalized := make([]string, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(strings.TrimSpace(id))
		if err != nil {
			return 0, newValidationError("invalid record id %q", id)
		}
		normalized = append(normalized, parsed.String())
	}

	var deleted int64

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 1) Leer los registros a borrar respetando el RLS.
		readQuery := tx.Table(ctx.TableName).Where("id IN ?", normalized)
		if ctx.AccessLevel == permission.AccessOwn {
			readQuery = readQuery.Where("created_by = ?", ctx.UserID)
		}

		var records []map[string]interface{}
		if err := readQuery.Find(&records).Error; err != nil {
			return err
		}

		// 2) Borrado en una única sentencia parametrizada.
		deleteSQL := "DELETE FROM " + quoteIdentifier(ctx.TableName) +
			" WHERE id IN (" + sqlPlaceholders(len(normalized)) + ")"
		args := make([]interface{}, 0, len(normalized)+1)
		for _, id := range normalized {
			args = append(args, id)
		}
		if ctx.AccessLevel == permission.AccessOwn {
			deleteSQL += " AND created_by = ?"
			args = append(args, ctx.UserID)
		}

		result := tx.Exec(deleteSQL, args...)
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected

		// 3) Un webhook "deleted" por registro eliminado.
		for _, record := range records {
			if err := workers.EnqueueWebhookTx(tx, ctx.WsID, ctx.Table.ID, ctx.Table.Slug, "deleted", record); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return deleted, nil
}

// stripImmutableFields removes server-controlled fields from a client payload.
func stripImmutableFields(input map[string]interface{}) {
	for _, key := range []string{"id", "created_at", "updated_at", "created_by"} {
		delete(input, key)
	}
}

// Helper methods

// expandRecords resolves `?expand=` relation columns.
//
// Security rules enforced here:
//   - the target table is resolved from `_hornero_tables` (the raw JSON value is
//     NEVER trusted, it could point to any table of any workspace);
//   - the role must have "read" access on the target table;
//   - the display column must be a real column of the target table AND readable
//     by the role (column level security);
//   - every identifier used in SQL is validated and quoted.
func (s *Service) expandRecords(ctx RequestContext, records []map[string]interface{}, expandParam string) {
	if len(records) == 0 || expandParam == "" {
		return
	}

	columns, err := s.loadTableColumns(ctx.Table.ID)
	if err != nil {
		return
	}

	expandFields := make(map[string]bool)
	for _, f := range strings.Split(expandParam, ",") {
		if f = strings.TrimSpace(f); f != "" {
			expandFields[f] = true
		}
	}

	for _, col := range columns {
		if col.FieldType != "relation" || !expandFields[col.Slug] {
			continue
		}

		var meta map[string]interface{}
		if len(col.Meta) > 0 {
			_ = json.Unmarshal(col.Meta, &meta)
		}

		targetTableSlug, _ := meta["target_table"].(string)
		displayCol, _ := meta["display_column"].(string)
		cardinality, _ := meta["cardinality"].(string)
		isMany := cardinality == "many"

		if targetTableSlug == "" || displayCol == "" {
			continue
		}

		// 1) Resolve the target table inside the current workspace.
		var targetTable metadata.Table
		if err := database.DB.Table("_hornero_tables").
			Where("workspace_id = ? AND slug = ?", ctx.WsID, targetTableSlug).
			First(&targetTable).Error; err != nil {
			// Unknown / cross-workspace table: never expand it.
			continue
		}

		// 2) The role must be allowed to READ the target table.
		targetAccess, err := s.permSvc.CheckTableAccess(ctx.WsID, ctx.RoleName, targetTable.Slug, "read")
		if err != nil || targetAccess == permission.AccessNone {
			continue
		}

		// 3) The display column must exist on the target table and be readable.
		if !s.canReadColumn(ctx, targetTable, displayCol) {
			continue
		}

		// 4) The physical table name must be safe to interpolate in SQL.
		targetTableName := "data_" + ctx.WsID.String() + "_" + targetTable.Slug
		if !validateTableName(targetTableName) {
			continue
		}

		ids := make(map[string]bool)
		for _, rec := range records {
			val, ok := rec[col.Slug]
			if !ok || val == nil {
				continue
			}
			if isMany {
				if arr, ok := val.([]interface{}); ok {
					for _, item := range arr {
						if idStr, ok := item.(string); ok && idStr != "" {
							ids[idStr] = true
						}
					}
				}
				continue
			}
			if idStr, ok := val.(string); ok && idStr != "" {
				ids[idStr] = true
			}
		}
		if len(ids) == 0 {
			continue
		}

		idList := make([]string, 0, len(ids))
		for id := range ids {
			idList = append(idList, id)
		}

		// The table name is validated above and GORM quotes it when building
		// the FROM clause; the SELECT column is emitted as raw SQL by GORM,
		// so it must be quoted manually (displayCol is validated too).
		query := database.DB.Table(targetTableName).
			Select("id", quoteIdentifier(displayCol)).
			Where("id IN ?", idList)

		// Row level security on the target table.
		if targetAccess == permission.AccessOwn {
			query = query.Where("created_by = ?", ctx.UserID)
		}

		var targetRecords []map[string]interface{}
		if err := query.Find(&targetRecords).Error; err != nil {
			continue
		}

		lookup := make(map[string]interface{})
		for _, tr := range targetRecords {
			switch id := tr["id"].(type) {
			case string:
				lookup[id] = tr[displayCol]
			case uuid.UUID:
				lookup[id.String()] = tr[displayCol]
			}
		}

		for _, rec := range records {
			if rec["expand"] == nil {
				rec["expand"] = make(map[string]interface{})
			}
			expand, ok := rec["expand"].(map[string]interface{})
			if !ok {
				continue
			}
			if isMany {
				arr, _ := rec[col.Slug].([]interface{})
				resolved := make([]interface{}, 0, len(arr))
				for _, item := range arr {
					if idStr, ok := item.(string); ok {
						resolved = append(resolved, lookup[idStr])
					}
				}
				expand[col.Slug] = resolved
				continue
			}
			if val, ok := rec[col.Slug].(string); ok {
				expand[col.Slug] = lookup[val]
			}
		}
	}
}

// canReadColumn reports whether `columnSlug` may be used as display column of
// a relation expansion: it must be a real column of the target table (or a
// system column) and it must be readable by the role on that table.
func (s *Service) canReadColumn(ctx RequestContext, targetTable metadata.Table, columnSlug string) bool {
	if !validColumnNamePattern.MatchString(columnSlug) {
		return false
	}

	// System columns always exist on every physical table.
	if !systemColumns[columnSlug] {
		var count int64
		if err := database.DB.Table("_hornero_columns").
			Where("table_id = ? AND slug = ?", targetTable.ID, columnSlug).
			Count(&count).Error; err != nil {
			return false
		}
		if count == 0 {
			return false
		}
	}

	// Column level security on the target table.
	allowedColumns, err := s.permSvc.GetColumnsForOperation(ctx.WsID, ctx.RoleName, targetTable.Slug, "read")
	if err != nil {
		return false
	}
	if allowedColumns == nil {
		// No column level restriction configured for this role.
		return true
	}
	for _, col := range allowedColumns {
		if col == "*" {
			return true
		}
	}
	if systemColumns[columnSlug] {
		return true
	}
	for _, col := range allowedColumns {
		if col == columnSlug {
			return true
		}
	}
	return false
}

// validateTypedFields enforces column-level constraints that cannot be expressed
// purely at the SQL layer: select choices and cardinality shape (one vs many).
// Relation UUIDs are NOT validated for existence here — that would couple writes
// to read access on the target table and is left to the frontend/expand step.
func (s *Service) validateTypedFields(ctx RequestContext, input map[string]interface{}) error {
	columns, err := s.loadTableColumns(ctx.Table.ID)
	if err != nil {
		return err
	}

	return validateTypedFieldsWithColumns(columns, input)
}

// validateTypedFieldsWithColumns es la versión pura de validateTypedFields:
// recibe las columnas ya cargadas para poder reutilizarla en el bulk create.
func validateTypedFieldsWithColumns(columns []metadata.Column, input map[string]interface{}) error {
	for _, col := range columns {
		val, present := input[col.Slug]
		if !present || val == nil {
			continue
		}

		switch col.FieldType {
		case "select":
			if err := validateSelectValue(col, val); err != nil {
				return err
			}
		case "relation":
			if err := validateRelationShape(col, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) loadTableColumns(tableID uuid.UUID) ([]metadata.Column, error) {
	var cols []metadata.Column
	if err := database.DB.Table("_hornero_columns").Where("table_id = ?", tableID).Find(&cols).Error; err != nil {
		return nil, err
	}
	return cols, nil
}

func validateSelectValue(col metadata.Column, val interface{}) error {
	// meta may be nil / empty for legacy select columns — accept anything.
	var meta map[string]interface{}
	if len(col.Meta) > 0 {
		_ = json.Unmarshal(col.Meta, &meta)
	}
	if meta == nil {
		return nil
	}

	// "choices" is the canonical key; "options" is the legacy/fallback key
	// used by the existing templates.
	raw, exists := meta["choices"]
	if !exists || raw == nil {
		raw, exists = meta["options"]
		if !exists || raw == nil {
			return nil
		}
	}

	allowed := make(map[string]bool)
	switch choices := raw.(type) {
	case []interface{}:
		if len(choices) == 0 {
			return nil
		}
		for _, item := range choices {
			switch entry := item.(type) {
			case string:
				if entry != "" {
					allowed[entry] = true
				}
			case map[string]interface{}:
				if v, _ := entry["value"].(string); v != "" {
					allowed[v] = true
				}
			}
		}
	case []string:
		for _, entry := range choices {
			if entry != "" {
				allowed[entry] = true
			}
		}
	default:
		// Unknown shape: be permissive (backward compatible).
		return nil
	}

	if len(allowed) == 0 {
		return nil
	}

	cardinality, _ := meta["cardinality"].(string)
	if cardinality != "many" {
		cardinality = "one"
	}

	if cardinality == "many" {
		arr, ok := val.([]interface{})
		if !ok {
			return fmt.Errorf("column %q expects an array of choices", col.Slug)
		}
		for _, item := range arr {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("column %q: every choice must be a string", col.Slug)
			}
			if !allowed[s] {
				return fmt.Errorf("column %q: value %q is not in choices", col.Slug, s)
			}
		}
		return nil
	}

	// cardinality == "one"
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("column %q: expected a string choice", col.Slug)
	}
	if !allowed[s] {
		return fmt.Errorf("column %q: value %q is not in choices", col.Slug, s)
	}
	return nil
}

func validateRelationShape(col metadata.Column, val interface{}) error {
	var meta map[string]interface{}
	if len(col.Meta) > 0 {
		_ = json.Unmarshal(col.Meta, &meta)
	}
	cardinality := "one"
	if meta != nil {
		if c, _ := meta["cardinality"].(string); c == "many" {
			cardinality = "many"
		}
	}
	if cardinality == "many" {
		if _, ok := val.([]interface{}); !ok {
			return fmt.Errorf("column %q expects an array of relation ids", col.Slug)
		}
		return nil
	}
	if _, ok := val.(string); !ok {
		return fmt.Errorf("column %q expects a single relation id (string)", col.Slug)
	}
	return nil
}

// filterRecordColumns removes from the record every column the role is not
// allowed to read. System columns (id/created_at/updated_at/created_by) are
// always kept, and so is the "expand" payload (already permission checked).
func (s *Service) filterRecordColumns(record map[string]interface{}, allowedColumns []string) {
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return
		}
		allowedMap[col] = true
	}
	for _, col := range []string{"id", "created_at", "updated_at", "created_by", "expand"} {
		allowedMap[col] = true
	}
	for key := range record {
		if !allowedMap[key] {
			delete(record, key)
		}
	}
}

// filterInputColumns removes from the payload every column the role is not
// allowed to write.
//
// Symmetry with filterRecordColumns:
//   - allowedColumns == nil            -> no column level restriction, keep all
//   - allowedColumns == []string{}     -> NO writable columns at all
//   - allowedColumns contains "*"      -> keep all
func (s *Service) filterInputColumns(input map[string]interface{}, allowedColumns []string) map[string]interface{} {
	if allowedColumns == nil {
		// No restriction configured.
		return input
	}
	if len(allowedColumns) == 0 {
		// Restriction configured with zero allowed columns: nothing is writable.
		return map[string]interface{}{}
	}
	allowedMap := make(map[string]bool)
	for _, col := range allowedColumns {
		if col == "*" {
			return input
		}
		allowedMap[col] = true
	}
	filtered := make(map[string]interface{})
	for key, value := range input {
		if allowedMap[key] {
			filtered[key] = value
		}
	}
	return filtered
}

func (s *Service) filterDefinedColumns(tableID uuid.UUID, input map[string]interface{}) map[string]interface{} {
	cols, err := s.loadTableColumns(tableID)
	if err != nil || len(cols) == 0 {
		return input
	}

	validSlugs := make(map[string]bool)
	for _, c := range cols {
		validSlugs[c.Slug] = true
	}
	for _, sys := range []string{"id", "created_at", "updated_at", "created_by"} {
		validSlugs[sys] = true
	}

	filtered := make(map[string]interface{})
	for k, v := range input {
		if validSlugs[k] {
			filtered[k] = v
		}
	}
	return filtered
}

func (s *Service) generateAutonumberFields(tableID uuid.UUID, input map[string]interface{}) error {
	cols, err := s.loadTableColumns(tableID)
	if err != nil {
		return err
	}

	for _, col := range cols {
		if col.FieldType != "autonumber" {
			continue
		}
		val, present := input[col.Slug]
		if !present || val == nil || val == "" {
			autoVal, err := s.generateNextAutonumber(col.ID)
			if err != nil {
				return fmt.Errorf("failed to generate autonumber for %s: %v", col.Slug, err)
			}
			input[col.Slug] = autoVal
		}
	}
	return nil
}

func (s *Service) generateNextAutonumber(colID uuid.UUID) (string, error) {
	var formattedVal string
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var col metadata.Column
		if err := tx.Table("_hornero_columns").Set("gorm:query_option", "FOR UPDATE").First(&col, "id = ?", colID).Error; err != nil {
			return err
		}

		meta := make(map[string]interface{})
		if len(col.Meta) > 0 {
			_ = json.Unmarshal(col.Meta, &meta)
		}

		prefix, _ := meta["prefix"].(string)
		suffix, _ := meta["suffix"].(string)
		digits := 3
		if dFloat, ok := meta["digits"].(float64); ok && dFloat >= 0 {
			digits = int(dFloat)
		} else if dInt, ok := meta["digits"].(int); ok && dInt >= 0 {
			digits = dInt
		}

		currentVal := 1
		if cFloat, ok := meta["current_value"].(float64); ok && cFloat > 0 {
			currentVal = int(cFloat)
		} else if cInt, ok := meta["current_value"].(int); ok && cInt > 0 {
			currentVal = cInt
		}

		if digits > 0 {
			formattedVal = fmt.Sprintf("%s%0*d%s", prefix, digits, currentVal, suffix)
		} else {
			formattedVal = fmt.Sprintf("%s%d%s", prefix, currentVal, suffix)
		}

		meta["current_value"] = currentVal + 1
		metaBytes, _ := json.Marshal(meta)
		return tx.Table("_hornero_columns").Where("id = ?", colID).Update("meta", metadata.JSON(metaBytes)).Error
	})

	return formattedVal, err
}

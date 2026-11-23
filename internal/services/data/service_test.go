package data

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"hornerodb/internal/models/metadata"
	"hornerodb/internal/query"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// validateTableName
// ---------------------------------------------------------------------------

func TestValidateTableName(t *testing.T) {
	wsID := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"

	tests := []struct {
		name      string
		tableName string
		want      bool
	}{
		{"valid table name", "data_" + wsID + "_orders", true},
		{"valid table name with underscore suffix", "data_" + wsID + "_my_table_2", true},
		{"missing workspace prefix", wsID + "_orders", false},
		{"missing slug", "data_" + wsID, false},
		{"sql injection attempt", "data_" + wsID + "_orders; DROP TABLE users--", false},
		{"quoted identifier injection", `data_" + wsID + "_orders"`, false},
		{"uppercase uuid is accepted", "data_3F2504E0-4F89-41D3-9A0C-0305E82C3301_orders", true},
		{"empty", "", false},
		{"just the prefix", "data_", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validateTableName(tt.tableName))
		})
	}
}

// ---------------------------------------------------------------------------
// filterRecordColumns (read side)
// ---------------------------------------------------------------------------

func TestFilterRecordColumns_RestrictedColumns(t *testing.T) {
	s := NewService()
	record := map[string]interface{}{
		"id":         "abc",
		"name":       "secret",
		"email":      "a@b.c",
		"created_by": "user-1",
		"created_at": "2024-01-01",
		"updated_at": "2024-01-02",
	}

	s.filterRecordColumns(record, []string{"name"})

	// Only the allowed column plus the system columns survive.
	assert.Equal(t, "secret", record["name"])
	assert.Contains(t, record, "id")
	assert.Contains(t, record, "created_by")
	assert.Contains(t, record, "created_at")
	assert.Contains(t, record, "updated_at")
	assert.NotContains(t, record, "email")
}

func TestFilterRecordColumns_EmptyListKeepsOnlySystemColumns(t *testing.T) {
	s := NewService()
	record := map[string]interface{}{
		"id":    "abc",
		"name":  "secret",
		"email": "a@b.c",
	}

	s.filterRecordColumns(record, []string{})

	assert.Contains(t, record, "id")
	assert.NotContains(t, record, "name")
	assert.NotContains(t, record, "email")
}

func TestFilterRecordColumns_WildcardKeepsEverything(t *testing.T) {
	s := NewService()
	record := map[string]interface{}{
		"id":    "abc",
		"name":  "visible",
		"email": "a@b.c",
	}

	s.filterRecordColumns(record, []string{"*"})

	assert.Contains(t, record, "name")
	assert.Contains(t, record, "email")
}

// ---------------------------------------------------------------------------
// filterInputColumns (write side) — symmetry with filterRecordColumns
// ---------------------------------------------------------------------------

func TestFilterInputColumns_NilMeansNoRestriction(t *testing.T) {
	s := NewService()
	input := map[string]interface{}{"name": "value", "email": "a@b.c"}

	result := s.filterInputColumns(input, nil)

	// nil == no column level restriction: everything is kept (same map).
	assert.Equal(t, input, result)
	assert.Len(t, result, 2)
}

func TestFilterInputColumns_EmptyListMeansNoWritableColumns(t *testing.T) {
	s := NewService()
	input := map[string]interface{}{"name": "value", "email": "a@b.c"}

	result := s.filterInputColumns(input, []string{})

	// A non-nil empty list is an explicit "nothing is writable".
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

func TestFilterInputColumns_RestrictedColumns(t *testing.T) {
	s := NewService()
	input := map[string]interface{}{"name": "value", "email": "a@b.c", "note": "x"}

	result := s.filterInputColumns(input, []string{"name", "note"})

	assert.Equal(t, map[string]interface{}{"name": "value", "note": "x"}, result)
}

func TestFilterInputColumns_WildcardKeepsEverything(t *testing.T) {
	s := NewService()
	input := map[string]interface{}{"name": "value", "email": "a@b.c"}

	result := s.filterInputColumns(input, []string{"*"})

	assert.Equal(t, input, result)
	assert.Len(t, result, 2)
}

// ---------------------------------------------------------------------------
// validateSelectValue
// ---------------------------------------------------------------------------

func selectColumn(t *testing.T, meta map[string]interface{}) metadata.Column {
	t.Helper()
	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	return metadata.Column{Slug: "status", FieldType: "select", Meta: metadata.JSON(raw)}
}

func TestValidateSelectValue_Choices(t *testing.T) {
	col := selectColumn(t, map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{"value": "todo", "label": "To Do"},
			map[string]interface{}{"value": "done", "label": "Done"},
		},
	})

	assert.NoError(t, validateSelectValue(col, "todo"))
	assert.NoError(t, validateSelectValue(col, "done"))
	assert.Error(t, validateSelectValue(col, "archived"))
	assert.Error(t, validateSelectValue(col, 42))
}

func TestValidateSelectValue_OptionsFallback(t *testing.T) {
	col := selectColumn(t, map[string]interface{}{
		"options": []interface{}{"todo", "done"},
	})

	assert.NoError(t, validateSelectValue(col, "todo"))
	assert.Error(t, validateSelectValue(col, "archived"))
}

func TestValidateSelectValue_OptionsAsObjects(t *testing.T) {
	col := selectColumn(t, map[string]interface{}{
		"options": []interface{}{
			map[string]interface{}{"value": "todo"},
		},
	})

	assert.NoError(t, validateSelectValue(col, "todo"))
	assert.Error(t, validateSelectValue(col, "done"))
}

func TestValidateSelectValue_ManyCardinality(t *testing.T) {
	col := selectColumn(t, map[string]interface{}{
		"cardinality": "many",
		"choices":     []interface{}{map[string]interface{}{"value": "todo"}},
	})

	assert.NoError(t, validateSelectValue(col, []interface{}{"todo"}))
	assert.Error(t, validateSelectValue(col, []interface{}{"other"}))
	assert.Error(t, validateSelectValue(col, "todo"))
}

func TestValidateSelectValue_NoMetaIsPermissive(t *testing.T) {
	col := metadata.Column{Slug: "status", FieldType: "select"}
	assert.NoError(t, validateSelectValue(col, "whatever"))

	empty := metadata.Column{Slug: "status", FieldType: "select", Meta: metadata.JSON(`{}`)}
	assert.NoError(t, validateSelectValue(empty, "whatever"))
}

// ---------------------------------------------------------------------------
// validateRelationShape
// ---------------------------------------------------------------------------

func relationColumn(t *testing.T, cardinality string) metadata.Column {
	t.Helper()
	meta := map[string]interface{}{"cardinality": cardinality}
	raw, err := json.Marshal(meta)
	require.NoError(t, err)
	return metadata.Column{Slug: "customer", FieldType: "relation", Meta: metadata.JSON(raw)}
}

func TestValidateRelationShape_One(t *testing.T) {
	col := relationColumn(t, "one")

	assert.NoError(t, validateRelationShape(col, "6f1b8b6e-2a5c-4a5f-9f0a-1b2c3d4e5f60"))
	assert.Error(t, validateRelationShape(col, []interface{}{"a"}))
	assert.Error(t, validateRelationShape(col, 42))
}

func TestValidateRelationShape_Many(t *testing.T) {
	col := relationColumn(t, "many")

	assert.NoError(t, validateRelationShape(col, []interface{}{"a", "b"}))
	assert.Error(t, validateRelationShape(col, "a"))
}

func TestValidateRelationShape_DefaultCardinalityIsOne(t *testing.T) {
	col := metadata.Column{Slug: "customer", FieldType: "relation"}

	assert.NoError(t, validateRelationShape(col, "an-id"))
	assert.Error(t, validateRelationShape(col, []interface{}{"an-id"}))
}

// ---------------------------------------------------------------------------
// stripImmutableFields
// ---------------------------------------------------------------------------

func TestStripImmutableFields(t *testing.T) {
	input := map[string]interface{}{
		"id":         "client-supplied",
		"created_at": "2000-01-01",
		"updated_at": "2000-01-01",
		"created_by": "another-user",
		"name":       "keep me",
	}

	stripImmutableFields(input)

	assert.NotContains(t, input, "id")
	assert.NotContains(t, input, "created_at")
	assert.NotContains(t, input, "updated_at")
	assert.NotContains(t, input, "created_by")
	assert.Equal(t, "keep me", input["name"])
}

// ---------------------------------------------------------------------------
// quoteIdentifier
// ---------------------------------------------------------------------------

func TestQuoteIdentifier(t *testing.T) {
	assert.Equal(t, `"name"`, quoteIdentifier("name"))
	assert.Equal(t, `"a""b"`, quoteIdentifier(`a"b`))
	assert.Equal(t, `""`, quoteIdentifier(""))
}

// ---------------------------------------------------------------------------
// buildReadableColumns (CLS read side)
// ---------------------------------------------------------------------------

func textColumn(slug, fieldType string) metadata.Column {
	return metadata.Column{Slug: slug, Name: slug, FieldType: fieldType}
}

func TestBuildReadableColumns_NoRestriction(t *testing.T) {
	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("saldo", "number"),
	}

	readable := buildReadableColumns(columns, nil)

	assert.True(t, readable["nombre"])
	assert.True(t, readable["saldo"])
	// Columnas de sistema siempre legibles.
	assert.True(t, readable["id"])
	assert.True(t, readable["created_at"])
	assert.True(t, readable["updated_at"])
	assert.True(t, readable["created_by"])
}

func TestBuildReadableColumns_Wildcard(t *testing.T) {
	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("saldo", "number"),
	}

	readable := buildReadableColumns(columns, []string{"*"})

	assert.True(t, readable["nombre"])
	assert.True(t, readable["saldo"])
}

func TestBuildReadableColumns_Restricted(t *testing.T) {
	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("saldo", "number"),
		textColumn("secreto", "text"),
	}

	readable := buildReadableColumns(columns, []string{"nombre"})

	assert.True(t, readable["nombre"])
	assert.False(t, readable["saldo"])
	assert.False(t, readable["secreto"])
	// Las columnas de sistema siguen siendo legibles.
	assert.True(t, readable["id"])
}

func TestBuildReadableColumns_EmptyRestrictionKeepsOnlySystem(t *testing.T) {
	columns := []metadata.Column{textColumn("nombre", "text")}

	readable := buildReadableColumns(columns, []string{})

	assert.False(t, readable["nombre"])
	assert.True(t, readable["id"])
	assert.True(t, readable["created_at"])
}

// ---------------------------------------------------------------------------
// buildFilterClause
// ---------------------------------------------------------------------------

func readableSet(cols ...string) map[string]bool {
	set := map[string]bool{"id": true, "created_at": true, "updated_at": true, "created_by": true}
	for _, col := range cols {
		set[col] = true
	}
	return set
}

func TestBuildFilterClause_Empty(t *testing.T) {
	sql, args, err := buildFilterClause(nil, readableSet("nombre"))

	assert.NoError(t, err)
	assert.Equal(t, "", sql)
	assert.Empty(t, args)
}

func TestBuildFilterClause_Operators(t *testing.T) {
	tests := []struct {
		operator string
		value    string
		wantSQL  string
		wantArgs []interface{}
	}{
		{"eq", "juan", `"nombre" = ?`, []interface{}{"juan"}},
		{"neq", "juan", `"nombre" != ?`, []interface{}{"juan"}},
		{"gt", "10", `"nombre" > ?`, []interface{}{"10"}},
		{"gte", "10", `"nombre" >= ?`, []interface{}{"10"}},
		{"lt", "10", `"nombre" < ?`, []interface{}{"10"}},
		{"lte", "10", `"nombre" <= ?`, []interface{}{"10"}},
		{"like", "%j%", `"nombre" LIKE ?`, []interface{}{"%j%"}},
		{"ilike", "%j%", `"nombre" ILIKE ?`, []interface{}{"%j%"}},
	}

	for _, tt := range tests {
		t.Run(tt.operator, func(t *testing.T) {
			sql, args, err := buildFilterClause(
				[]query.FilterCondition{{Column: "nombre", Operator: tt.operator, Value: tt.value}},
				readableSet("nombre"),
			)

			require.NoError(t, err)
			assert.Equal(t, tt.wantSQL, sql)
			assert.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestBuildFilterClause_EmptyOperatorDefaultsToEq(t *testing.T) {
	sql, args, err := buildFilterClause(
		[]query.FilterCondition{{Column: "nombre", Value: "juan"}},
		readableSet("nombre"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"nombre" = ?`, sql)
	assert.Equal(t, []interface{}{"juan"}, args)
}

func TestBuildFilterClause_In(t *testing.T) {
	sql, args, err := buildFilterClause(
		[]query.FilterCondition{{Column: "ciudad", Operator: "in", Value: "CABA,Rosario , "}},
		readableSet("ciudad"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"ciudad" IN (?,?)`, sql)
	assert.Equal(t, []interface{}{"CABA", "Rosario"}, args)
}

func TestBuildFilterClause_InWithoutValuesIsRejected(t *testing.T) {
	_, _, err := buildFilterClause(
		[]query.FilterCondition{{Column: "ciudad", Operator: "in", Value: " , "}},
		readableSet("ciudad"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildFilterClause_IsNull(t *testing.T) {
	sql, args, err := buildFilterClause(
		[]query.FilterCondition{{Column: "email", Operator: "is_null", Value: "true"}},
		readableSet("email"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"email" IS NULL`, sql)
	assert.Empty(t, args)

	sql, args, err = buildFilterClause(
		[]query.FilterCondition{{Column: "email", Operator: "is_null", Value: "false"}},
		readableSet("email"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"email" IS NOT NULL`, sql)
	assert.Empty(t, args)
}

func TestBuildFilterClause_IsNullWithInvalidValueIsRejected(t *testing.T) {
	_, _, err := buildFilterClause(
		[]query.FilterCondition{{Column: "email", Operator: "is_null", Value: "maybe"}},
		readableSet("email"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildFilterClause_UnknownOperatorIsRejected(t *testing.T) {
	_, _, err := buildFilterClause(
		[]query.FilterCondition{{Column: "nombre", Operator: "drop", Value: "x"}},
		readableSet("nombre"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildFilterClause_NotReadableColumnIsRejected(t *testing.T) {
	// "saldo" existe en la tabla pero el rol no lo puede leer.
	_, _, err := buildFilterClause(
		[]query.FilterCondition{{Column: "saldo", Operator: "eq", Value: "1"}},
		readableSet("nombre"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildFilterClause_UnknownColumnIsRejected(t *testing.T) {
	_, _, err := buildFilterClause(
		[]query.FilterCondition{{Column: "no_existe", Operator: "eq", Value: "1"}},
		readableSet("nombre"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildFilterClause_SQLInjectionColumnIsRejected(t *testing.T) {
	attempts := []string{
		`nombre"; DROP TABLE users; --`,
		"nombre OR 1=1",
		"nombre, (SELECT password FROM users)",
		"nombre--",
	}

	for _, column := range attempts {
		t.Run(column, func(t *testing.T) {
			_, _, err := buildFilterClause(
				[]query.FilterCondition{{Column: column, Operator: "eq", Value: "x"}},
				readableSet(column),
			)

			require.Error(t, err)
			assert.True(t, IsValidationError(err))
		})
	}
}

func TestBuildFilterClause_ValuesAreNeverConcatenated(t *testing.T) {
	evil := `'; DROP TABLE users; --`

	sql, args, err := buildFilterClause(
		[]query.FilterCondition{{Column: "nombre", Operator: "eq", Value: evil}},
		readableSet("nombre"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"nombre" = ?`, sql)
	assert.NotContains(t, sql, "DROP")
	assert.Equal(t, []interface{}{evil}, args)
}

func TestBuildFilterClause_MultipleFiltersAreANDed(t *testing.T) {
	sql, args, err := buildFilterClause(
		[]query.FilterCondition{
			{Column: "ciudad", Operator: "eq", Value: "CABA"},
			{Column: "edad", Operator: "gte", Value: "18"},
		},
		readableSet("ciudad", "edad"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"ciudad" = ? AND "edad" >= ?`, sql)
	assert.Equal(t, []interface{}{"CABA", "18"}, args)
}

// ---------------------------------------------------------------------------
// buildOrderClause
// ---------------------------------------------------------------------------

func TestBuildOrderClause_DefaultIsStable(t *testing.T) {
	sql, err := buildOrderClause(nil, readableSet())

	require.NoError(t, err)
	assert.Equal(t, `"created_at" DESC`, sql)
}

func TestBuildOrderClause_ExplicitSorts(t *testing.T) {
	sql, err := buildOrderClause(
		[]query.SortClause{{Column: "nombre"}, {Column: "created_at", Desc: true}},
		readableSet("nombre", "created_at"),
	)

	require.NoError(t, err)
	assert.Equal(t, `"nombre" ASC, "created_at" DESC`, sql)
}

func TestBuildOrderClause_NotReadableColumnIsRejected(t *testing.T) {
	_, err := buildOrderClause([]query.SortClause{{Column: "saldo"}}, readableSet("nombre"))

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestBuildOrderClause_SQLInjectionColumnIsRejected(t *testing.T) {
	_, err := buildOrderClause(
		[]query.SortClause{{Column: "nombre; DROP TABLE users"}},
		readableSet("nombre; DROP TABLE users"),
	)

	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

// ---------------------------------------------------------------------------
// buildSearchClause
// ---------------------------------------------------------------------------

func TestBuildSearchClause_OnlyTextColumns(t *testing.T) {
	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("bio", "long_text"),
		textColumn("email", "email"),
		textColumn("web", "url"),
		textColumn("estado", "select"),
		textColumn("saldo", "number"),
		textColumn("fecha", "date"),
		textColumn("cliente", "relation"),
	}

	sql, args := buildSearchClause("juan", columns, readableSet("nombre", "bio", "email", "web", "estado", "saldo", "fecha", "cliente"))

	require.NotEmpty(t, sql)
	assert.Equal(t, `(CAST("nombre" AS TEXT) ILIKE ? OR CAST("bio" AS TEXT) ILIKE ? OR CAST("email" AS TEXT) ILIKE ? OR CAST("web" AS TEXT) ILIKE ? OR CAST("estado" AS TEXT) ILIKE ?)`, sql)
	require.Len(t, args, 5)
	for _, arg := range args {
		assert.Equal(t, "%juan%", arg)
	}
}

func TestBuildSearchClause_OnlyReadableColumns(t *testing.T) {
	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("secreto", "text"),
	}

	sql, args := buildSearchClause("juan", columns, readableSet("nombre"))

	require.NotEmpty(t, sql)
	assert.Equal(t, `(CAST("nombre" AS TEXT) ILIKE ?)`, sql)
	assert.Equal(t, []interface{}{"%juan%"}, args)
}

func TestBuildSearchClause_NoSearchableColumns(t *testing.T) {
	columns := []metadata.Column{
		textColumn("saldo", "number"),
		textColumn("nombre", "text"),
	}

	// "nombre" existe pero no es legible: no hay nada que buscar.
	sql, args := buildSearchClause("juan", columns, readableSet())

	assert.Equal(t, "", sql)
	assert.Empty(t, args)
}

func TestBuildSearchClause_EscapesWildcards(t *testing.T) {
	columns := []metadata.Column{textColumn("nombre", "text")}

	sql, args := buildSearchClause("100%", columns, readableSet("nombre"))

	require.NotEmpty(t, sql)
	assert.Equal(t, []interface{}{"%100\\%%"}, args)
}

// ---------------------------------------------------------------------------
// Bulk limits
// ---------------------------------------------------------------------------

func TestValidateBulkSize(t *testing.T) {
	assert.NoError(t, validateBulkSize(1, "records"))
	assert.NoError(t, validateBulkSize(MaxBulkRecords, "records"))

	err := validateBulkSize(0, "records")
	require.Error(t, err)
	assert.True(t, IsValidationError(err))

	err = validateBulkSize(MaxBulkRecords+1, "records")
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

// ---------------------------------------------------------------------------
// prepareBulkRows
// ---------------------------------------------------------------------------

func TestPrepareBulkRows_StripsImmutableFieldsAndAssignsServerValues(t *testing.T) {
	s := NewService()
	ctx := RequestContext{UserID: "user-1"}

	columns := []metadata.Column{textColumn("nombre", "text")}
	inputs := []map[string]interface{}{
		{
			"id":         "client-id",
			"created_at": "2000-01-01",
			"updated_at": "2000-01-01",
			"created_by": "another-user",
			"nombre":     "juan",
		},
	}

	rows, err := s.prepareBulkRows(ctx, inputs, columns, nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	row := rows[0]
	// Los valores del cliente se descartan y los reasigna el servidor.
	assert.Equal(t, "user-1", row["created_by"])
	assert.Equal(t, "juan", row["nombre"])

	serverID, ok := row["id"].(uuid.UUID)
	require.True(t, ok)
	assert.NotEqual(t, "client-id", serverID.String())

	serverCreatedAt, ok := row["created_at"].(time.Time)
	require.True(t, ok)
	assert.NotEqual(t, "2000-01-01", serverCreatedAt.String())

	serverUpdatedAt, ok := row["updated_at"].(time.Time)
	require.True(t, ok)
	assert.NotEqual(t, "2000-01-01", serverUpdatedAt.String())
}

func TestPrepareBulkRows_AppliesWritableColumnRestrictions(t *testing.T) {
	s := NewService()
	ctx := RequestContext{UserID: "user-1"}

	columns := []metadata.Column{
		textColumn("nombre", "text"),
		textColumn("secreto", "text"),
	}
	inputs := []map[string]interface{}{
		{"nombre": "juan", "secreto": "hidden"},
	}

	rows, err := s.prepareBulkRows(ctx, inputs, columns, []string{"nombre"})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	assert.Equal(t, "juan", rows[0]["nombre"])
	assert.NotContains(t, rows[0], "secreto")
}

func TestPrepareBulkRows_DoesNotMutateTheCallerPayload(t *testing.T) {
	s := NewService()
	ctx := RequestContext{UserID: "user-1"}

	columns := []metadata.Column{textColumn("nombre", "text")}
	inputs := []map[string]interface{}{{"nombre": "juan"}}

	_, err := s.prepareBulkRows(ctx, inputs, columns, nil)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{"nombre": "juan"}, inputs[0])
}

func TestPrepareBulkRows_ValidatesTypedFields(t *testing.T) {
	s := NewService()
	ctx := RequestContext{UserID: "user-1"}

	columns := []metadata.Column{selectColumn(t, map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{"value": "todo"},
		},
	})}
	inputs := []map[string]interface{}{{"status": "invalid-choice"}}

	_, err := s.prepareBulkRows(ctx, inputs, columns, nil)
	require.Error(t, err)
	assert.True(t, IsValidationError(err))
}

func TestPrepareBulkRows_GeneratesDistinctIDs(t *testing.T) {
	s := NewService()
	ctx := RequestContext{UserID: "user-1"}

	columns := []metadata.Column{textColumn("nombre", "text")}
	inputs := []map[string]interface{}{{"nombre": "a"}, {"nombre": "b"}}

	rows, err := s.prepareBulkRows(ctx, inputs, columns, nil)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	first, ok := rows[0]["id"].(uuid.UUID)
	require.True(t, ok)
	second, ok := rows[1]["id"].(uuid.UUID)
	require.True(t, ok)
	assert.NotEqual(t, first, second)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestSplitCSV(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, splitCSV("a,b"))
	assert.Equal(t, []string{"a", "b"}, splitCSV(" a , b "))
	assert.Empty(t, splitCSV(""))
	assert.Empty(t, splitCSV(" , , "))
}

func TestSQLPlaceholders(t *testing.T) {
	assert.Equal(t, "", sqlPlaceholders(0))
	assert.Equal(t, "?", sqlPlaceholders(1))
	assert.Equal(t, "?,?,?", sqlPlaceholders(3))
}

func TestRecordIDKey(t *testing.T) {
	key, ok := recordIDKey("abc")
	assert.True(t, ok)
	assert.Equal(t, "abc", key)

	key, ok = recordIDKey(uuid.New())
	assert.True(t, ok)
	assert.Len(t, key, 36)

	_, ok = recordIDKey(nil)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// WithContext
// ---------------------------------------------------------------------------

func TestWithContext_ReturnsShallowCopyWithoutMutatingOriginal(t *testing.T) {
	svc := NewService()
	type ctxKey struct{}
	reqCtx := context.WithValue(context.Background(), ctxKey{}, "trace-123")

	bound := svc.WithContext(reqCtx)

	require.NotNil(t, bound)
	assert.NotSame(t, svc, bound)
	assert.Nil(t, svc.ctx)
	assert.Equal(t, reqCtx, bound.ctx)
	assert.Same(t, svc.permSvc, bound.permSvc)

	var nilSvc *Service
	nilBound := nilSvc.WithContext(reqCtx)
	require.NotNil(t, nilBound)
	assert.Equal(t, reqCtx, nilBound.ctx)
	assert.NotNil(t, nilBound.permSvc)
}

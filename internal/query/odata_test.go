package query

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseODataQuery_BasicFilters(t *testing.T) {
	allowed := []string{"comanda_id", "status", "precio", "cantidad", "nombre"}

	tests := []struct {
		name        string
		filter      string
		expectedSQL string
		expectedArgs []interface{}
	}{
		{
			name:        "string eq",
			filter:      "comanda_id eq 'c-123'",
			expectedSQL: "comanda_id = ?",
			expectedArgs: []interface{}{"c-123"},
		},
		{
			name:        "number comparison and logical and",
			filter:      "precio gt 100 and cantidad le 5",
			expectedSQL: "(precio > ? AND cantidad <= ?)",
			expectedArgs: []interface{}{int64(100), int64(5)},
		},
		{
			name:        "logical or with string ne",
			filter:      "status eq 'ready' or status ne 'cancelled'",
			expectedSQL: "(status = ? OR status != ?)",
			expectedArgs: []interface{}{"ready", "cancelled"},
		},
		{
			name:        "null check",
			filter:      "comanda_id eq null",
			expectedSQL: "comanda_id IS NULL",
			expectedArgs: nil,
		},
		{
			name:        "not null check",
			filter:      "comanda_id ne null",
			expectedSQL: "comanda_id IS NOT NULL",
			expectedArgs: nil,
		},
		{
			name:        "in operator",
			filter:      "status in ('pending', 'cooking', 'ready')",
			expectedSQL: "status IN (?,?,?)",
			expectedArgs: []interface{}{"pending", "cooking", "ready"},
		},
		{
			name:        "contains function",
			filter:      "contains(nombre, 'pizza')",
			expectedSQL: "nombre ILIKE ?",
			expectedArgs: []interface{}{"%pizza%"},
		},
		{
			name:        "startswith function",
			filter:      "startswith(nombre, 'Emp')",
			expectedSQL: "nombre ILIKE ?",
			expectedArgs: []interface{}{"Emp%"},
		},
		{
			name:        "parentheses grouping",
			filter:      "(comanda_id eq '1' or comanda_id eq '2') and precio gt 50",
			expectedSQL: "((comanda_id = ? OR comanda_id = ?) AND precio > ?)",
			expectedArgs: []interface{}{"1", "2", int64(50)},
		},
		{
			name:        "not unary",
			filter:      "not (status eq 'deleted')",
			expectedSQL: "NOT (status = ?)",
			expectedArgs: []interface{}{"deleted"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vals := url.Values{}
			vals.Set("$filter", tc.filter)

			q, err := ParseODataQuery(vals, allowed)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedSQL, q.FilterSQL)
			assert.Equal(t, tc.expectedArgs, q.FilterArgs)
		})
	}
}

func TestParseODataQuery_ColumnSecurity(t *testing.T) {
	allowed := []string{"nombre", "public_info"}

	t.Run("forbidden column in filter is blocked", func(t *testing.T) {
		vals := url.Values{}
		vals.Set("$filter", "secret_salary gt 50000")

		_, err := ParseODataQuery(vals, allowed)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrColumnAccessDenied))
	})

	t.Run("forbidden column in orderby is blocked", func(t *testing.T) {
		vals := url.Values{}
		vals.Set("$orderby", "secret_salary desc")

		_, err := ParseODataQuery(vals, allowed)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrColumnAccessDenied))
	})

	t.Run("forbidden column in select is blocked", func(t *testing.T) {
		vals := url.Values{}
		vals.Set("$select", "nombre,secret_salary")

		_, err := ParseODataQuery(vals, allowed)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrColumnAccessDenied))
	})

	t.Run("system columns are always permitted", func(t *testing.T) {
		vals := url.Values{}
		vals.Set("$filter", "created_by eq 'usr-1' and id ne null")
		vals.Set("$orderby", "created_at desc")
		vals.Set("$select", "id,created_at,nombre")

		q, err := ParseODataQuery(vals, allowed)
		require.NoError(t, err)
		assert.Equal(t, "(created_by = ? AND id IS NOT NULL)", q.FilterSQL)
		assert.Equal(t, "created_at DESC", q.OrderBy)
		assert.Equal(t, []string{"id", "created_at", "nombre"}, q.Select)
	})

	t.Run("empty non-nil allowedColumns denies all non-system columns", func(t *testing.T) {
		emptyAllowed := []string{}
		for _, param := range []struct {
			k string
			v string
		}{
			{"$filter", "nombre eq 'juan'"},
			{"$orderby", "nombre desc"},
			{"$select", "nombre"},
		} {
			vals := url.Values{}
			vals.Set(param.k, param.v)
			_, err := ParseODataQuery(vals, emptyAllowed)
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrColumnAccessDenied))
		}

		// System columns still work with empty non-nil allowedColumns
		vals := url.Values{}
		vals.Set("$filter", "created_by eq 'u1'")
		_, err := ParseODataQuery(vals, emptyAllowed)
		require.NoError(t, err)
	})

	t.Run("nil allowedColumns means unrestricted", func(t *testing.T) {
		vals := url.Values{}
		vals.Set("$filter", "any_col eq 'val'")
		q, err := ParseODataQuery(vals, nil)
		require.NoError(t, err)
		assert.Equal(t, "any_col = ?", q.FilterSQL)
	})
}

func TestParseODataQuery_OrderByAndPagination(t *testing.T) {
	allowed := []string{"created_at", "nombre", "precio"}

	vals := url.Values{}
	vals.Set("$orderby", "precio desc, nombre asc")
	vals.Set("$top", "25")
	vals.Set("$skip", "50")
	vals.Set("$expand", "categoria_id")

	q, err := ParseODataQuery(vals, allowed)
	require.NoError(t, err)
	assert.Equal(t, "precio DESC, nombre ASC", q.OrderBy)
	assert.Equal(t, 25, q.Top)
	assert.Equal(t, 50, q.Skip)
	assert.Equal(t, "categoria_id", q.Expand)
}

func TestParseODataFromArgs_MCP(t *testing.T) {
	allowed := []string{"comanda_id", "status"}

	args := map[string]interface{}{
		"filter":  "comanda_id eq 'c-999' and status eq 'open'",
		"orderby": "created_at desc",
		"limit":   float64(10),
		"offset":  float64(20),
	}

	q, err := ParseODataFromArgs(args, allowed)
	require.NoError(t, err)
	assert.Equal(t, "(comanda_id = ? AND status = ?)", q.FilterSQL)
	assert.Equal(t, []interface{}{"c-999", "open"}, q.FilterArgs)
	assert.Equal(t, "created_at DESC", q.OrderBy)
	assert.Equal(t, 10, q.Top)
	assert.Equal(t, 20, q.Skip)
}

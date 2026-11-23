package query

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseSort
// ---------------------------------------------------------------------------

func TestParseSort_Empty(t *testing.T) {
	assert.Nil(t, ParseSort(""))
	assert.Nil(t, ParseSort("   "))
}

func TestParseSort_ExplicitDirection(t *testing.T) {
	sorts := ParseSort("nombre:asc,created_at:desc")

	require.Len(t, sorts, 2)
	assert.Equal(t, SortClause{Column: "nombre", Desc: false}, sorts[0])
	assert.Equal(t, SortClause{Column: "created_at", Desc: true}, sorts[1])
}

func TestParseSort_DirectionIsCaseInsensitive(t *testing.T) {
	sorts := ParseSort("nombre:ASC,created_at:DESC")

	require.Len(t, sorts, 2)
	assert.False(t, sorts[0].Desc)
	assert.True(t, sorts[1].Desc)
}

func TestParseSort_LeadingMinusMeansDesc(t *testing.T) {
	sorts := ParseSort("-created_at,nombre")

	require.Len(t, sorts, 2)
	assert.Equal(t, SortClause{Column: "created_at", Desc: true}, sorts[0])
	assert.Equal(t, SortClause{Column: "nombre", Desc: false}, sorts[1])
}

func TestParseSort_SingleColumnWithoutDirection(t *testing.T) {
	sorts := ParseSort("nombre")

	require.Len(t, sorts, 1)
	assert.Equal(t, SortClause{Column: "nombre", Desc: false}, sorts[0])
}

func TestParseSort_EmptySegmentsAreIgnored(t *testing.T) {
	sorts := ParseSort("nombre,,  ,created_at")

	require.Len(t, sorts, 2)
	assert.Equal(t, "nombre", sorts[0].Column)
	assert.Equal(t, "created_at", sorts[1].Column)
}

func TestParseSort_InvalidDirectionIsIgnored(t *testing.T) {
	sorts := ParseSort("nombre:sideways,created_at:desc")

	require.Len(t, sorts, 1)
	assert.Equal(t, "created_at", sorts[0].Column)
}

func TestParseSort_InvalidColumnIsIgnored(t *testing.T) {
	tests := []string{
		"nombre;DROP TABLE users",
		"nombre OR 1=1",
		"nombre`",
		"1nombre",
		"_nombre",
		"nombre;--",
		"\"nombre\"",
		"nombre asc",
		"na me",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			assert.Empty(t, ParseSort(raw))
		})
	}
}

func TestParseSort_SQLInjectionColumnKeepsValidNeighbours(t *testing.T) {
	sorts := ParseSort("nombre:asc,evil;DROP TABLE users--,created_at:desc")

	require.Len(t, sorts, 2)
	assert.Equal(t, "nombre", sorts[0].Column)
	assert.Equal(t, "created_at", sorts[1].Column)
}

func TestParseSort_MaxSortsIsCapped(t *testing.T) {
	raw := ""
	for i := 0; i < MaxSorts+5; i++ {
		if i > 0 {
			raw += ","
		}
		raw += "col"
	}

	sorts := ParseSort(raw)
	assert.Len(t, sorts, MaxSorts)
}

// ---------------------------------------------------------------------------
// ParseFilters
// ---------------------------------------------------------------------------

func TestParseFilters_Empty(t *testing.T) {
	assert.Nil(t, ParseFilters(url.Values{}))
}

func TestParseFilters_DefaultOperatorIsEq(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[nombre]": {"juan"},
	})

	require.Len(t, filters, 1)
	assert.Equal(t, FilterCondition{Column: "nombre", Operator: "eq", Value: "juan"}, filters[0])
}

func TestParseFilters_ExplicitOperator(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[edad][gte]":  {"18"},
		"filter[ciudad][eq]": {"CABA"},
	})

	require.Len(t, filters, 2)
	// Deterministic order: column then operator.
	assert.Equal(t, FilterCondition{Column: "ciudad", Operator: "eq", Value: "CABA"}, filters[0])
	assert.Equal(t, FilterCondition{Column: "edad", Operator: "gte", Value: "18"}, filters[1])
}

func TestParseFilters_AllSupportedOperators(t *testing.T) {
	for _, op := range []string{"eq", "neq", "gt", "gte", "lt", "lte", "like", "ilike", "in", "is_null"} {
		t.Run(op, func(t *testing.T) {
			filters := ParseFilters(url.Values{
				"filter[col][" + op + "]": {"value"},
			})

			require.Len(t, filters, 1)
			assert.Equal(t, op, filters[0].Operator)
			assert.Equal(t, "value", filters[0].Value)
		})
	}
}

func TestParseFilters_OperatorIsCaseInsensitive(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[edad][GTE]": {"18"},
	})

	require.Len(t, filters, 1)
	assert.Equal(t, "gte", filters[0].Operator)
}

func TestParseFilters_InvalidOperatorIsIgnored(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[nombre][drop]":  {"juan"},
		"filter[nombre][union]": {"x"},
		"filter[nombre]":        {"ok"},
	})

	require.Len(t, filters, 1)
	assert.Equal(t, "nombre", filters[0].Column)
	assert.Equal(t, "eq", filters[0].Operator)
}

func TestParseFilters_InvalidColumnIsIgnored(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[nombre;DROP TABLE users]": {"x"},
		"filter[or 1=1]":                  {"x"},
		"filter[1abc]":                    {"x"},
		"filter[]":                        {"x"},
	})

	assert.Empty(t, filters)
}

func TestParseFilters_OnlyFilterKeysAreConsidered(t *testing.T) {
	filters := ParseFilters(url.Values{
		"search":   {"juan"},
		"order":    {"nombre"},
		"limit":    {"10"},
		"filters":  {"nombre"},
		"filter_a": {"nombre"},
	})

	assert.Empty(t, filters)
}

func TestParseFilters_FirstValueWins(t *testing.T) {
	filters := ParseFilters(url.Values{
		"filter[nombre]": {"first", "second"},
	})

	require.Len(t, filters, 1)
	assert.Equal(t, "first", filters[0].Value)
}

func TestParseFilters_MaxFiltersIsCapped(t *testing.T) {
	values := url.Values{}
	for i := 0; i < MaxFilters+10; i++ {
		values.Set("filter[col"+string(rune('a'+i%26))+string(rune('a'+i/26))+"]", "v")
	}

	filters := ParseFilters(values)
	assert.Len(t, filters, MaxFilters)
}

// ---------------------------------------------------------------------------
// ExtractListQueryParams
// ---------------------------------------------------------------------------

func newListContext(t *testing.T, target string) *gin.Context {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", target, nil)
	return c
}

func TestExtractListQueryParams_Empty(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t, "/api/data"))

	assert.Equal(t, "", params.Search)
	assert.Empty(t, params.Sorts)
	assert.Empty(t, params.Filters)
}

func TestExtractListQueryParams_SearchIsTrimmed(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t, "/api/data?search=%20%20juan%20%20"))

	assert.Equal(t, "juan", params.Search)
}

func TestExtractListQueryParams_OrderParameter(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t, "/api/data?order=nombre:asc,created_at:desc"))

	require.Len(t, params.Sorts, 2)
	assert.Equal(t, "nombre", params.Sorts[0].Column)
	assert.False(t, params.Sorts[0].Desc)
	assert.Equal(t, "created_at", params.Sorts[1].Column)
	assert.True(t, params.Sorts[1].Desc)
}

func TestExtractListQueryParams_SortParameterIsFallback(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t, "/api/data?sort=-created_at,nombre"))

	require.Len(t, params.Sorts, 2)
	assert.Equal(t, "created_at", params.Sorts[0].Column)
	assert.True(t, params.Sorts[0].Desc)
	assert.Equal(t, "nombre", params.Sorts[1].Column)
}

func TestExtractListQueryParams_OrderWinsOverSort(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t, "/api/data?order=nombre:asc&sort=created_at:desc"))

	require.Len(t, params.Sorts, 1)
	assert.Equal(t, "nombre", params.Sorts[0].Column)
}

func TestExtractListQueryParams_Filters(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t,
		"/api/data?filter[nombre]=juan&filter[edad][gte]=18"))

	require.Len(t, params.Filters, 2)
	assert.Equal(t, FilterCondition{Column: "edad", Operator: "gte", Value: "18"}, params.Filters[0])
	assert.Equal(t, FilterCondition{Column: "nombre", Operator: "eq", Value: "juan"}, params.Filters[1])
}

func TestExtractListQueryParams_Combined(t *testing.T) {
	params := ExtractListQueryParams(newListContext(t,
		"/api/data?search=juan&order=nombre:asc&filter[edad][gte]=18&filter[ciudad]=CABA"))

	assert.Equal(t, "juan", params.Search)
	require.Len(t, params.Sorts, 1)
	require.Len(t, params.Filters, 2)
	assert.Equal(t, "ciudad", params.Filters[0].Column)
	assert.Equal(t, "edad", params.Filters[1].Column)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestIsValidFilterOperator(t *testing.T) {
	assert.True(t, IsValidFilterOperator("eq"))
	assert.True(t, IsValidFilterOperator("IS_NULL"))
	assert.False(t, IsValidFilterOperator(""))
	assert.False(t, IsValidFilterOperator("drop"))
	assert.False(t, IsValidFilterOperator("; DROP TABLE users"))
}

func TestIsValidColumnName(t *testing.T) {
	assert.True(t, IsValidColumnName("nombre"))
	assert.True(t, IsValidColumnName("created_at"))
	assert.True(t, IsValidColumnName("col2"))
	assert.False(t, IsValidColumnName(""))
	assert.False(t, IsValidColumnName("2col"))
	assert.False(t, IsValidColumnName("nombre;"))
	assert.False(t, IsValidColumnName(`nombre"`))
	assert.False(t, IsValidColumnName("nombre OR 1=1"))
}

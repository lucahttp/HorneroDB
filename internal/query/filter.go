package query

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// FilterCondition represents a single server-side filter clause.
//
// It is built from the query string using the following shapes:
//
//	filter[column]=value              -> operator "eq"
//	filter[column][operator]=value    -> explicit operator
//
// The column is NEVER interpolated into SQL by this package: the data layer
// validates it against the real table columns and against the column level
// security rules before quoting it.
type FilterCondition struct {
	Column   string
	Operator string // eq, neq, gt, gte, lt, lte, like, ilike, in, is_null
	Value    string
}

// SortClause represents one ORDER BY term.
type SortClause struct {
	Column string
	Desc   bool
}

// ListQueryParams holds the server-side query primitives extracted from the
// request: free text search, ordering and filters.
type ListQueryParams struct {
	Search  string
	Sorts   []SortClause
	Filters []FilterCondition
}

const (
	// MaxSorts caps the number of ORDER BY terms accepted from a client.
	MaxSorts = 10
	// MaxFilters caps the number of filter clauses accepted from a client.
	MaxFilters = 25
)

// DefaultOperator is used when a filter is declared without an operator.
const DefaultOperator = "eq"

// validColumnNamePattern restricts sort/filter column names to plain
// identifiers. This is the first barrier against SQL injection through a
// column name; the data layer performs a second barrier by checking the
// column against the real table metadata.
var validColumnNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// filterKeyPattern matches `filter[col]` and `filter[col][op]`.
var filterKeyPattern = regexp.MustCompile(`^filter\[([a-zA-Z][a-zA-Z0-9_]*)\](?:\[([a-zA-Z_][a-zA-Z0-9_]*)\])?$`)

// validFilterOperators is the allow-list of supported filter operators.
// Anything else is rejected (never interpolated into SQL).
var validFilterOperators = map[string]bool{
	"eq":      true,
	"neq":     true,
	"gt":      true,
	"gte":     true,
	"lt":      true,
	"lte":     true,
	"like":    true,
	"ilike":   true,
	"in":      true,
	"is_null": true,
}

// IsValidFilterOperator reports whether op is a supported filter operator.
func IsValidFilterOperator(op string) bool {
	return validFilterOperators[strings.ToLower(strings.TrimSpace(op))]
}

// IsValidColumnName reports whether name is a syntactically valid identifier
// that can be safely quoted and interpolated into SQL.
func IsValidColumnName(name string) bool {
	return validColumnNamePattern.MatchString(name)
}

// ExtractListQueryParams extracts search, ordering and filters from the
// request query string.
//
//	?search=juan
//	?order=nombre:asc,created_at:desc      (or ?sort=-created_at,nombre)
//	?filter[edad][gte]=18&filter[ciudad]=CABA
func ExtractListQueryParams(c *gin.Context) ListQueryParams {
	params := ListQueryParams{}

	if c == nil || c.Request == nil {
		return params
	}

	params.Search = strings.TrimSpace(c.Query("search"))

	rawOrder := strings.TrimSpace(c.Query("order"))
	if rawOrder == "" {
		rawOrder = strings.TrimSpace(c.Query("sort"))
	}
	params.Sorts = ParseSort(rawOrder)

	params.Filters = ParseFilters(c.Request.URL.Query())

	return params
}

// ParseSort parses an ordering expression into a list of sort clauses.
//
// Supported formats:
//
//	"nombre:asc,created_at:desc"  explicit direction
//	"-created_at,nombre"          leading "-" means DESC
//
// Clauses with an unknown direction or with a column name that is not a plain
// identifier are ignored: they can never reach the SQL builder.
func ParseSort(raw string) []SortClause {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	sorts := make([]SortClause, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		desc := false
		column := part

		if strings.HasPrefix(column, "-") {
			desc = true
			column = strings.TrimSpace(strings.TrimPrefix(column, "-"))
		}

		if idx := strings.Index(column, ":"); idx >= 0 {
			direction := strings.ToLower(strings.TrimSpace(column[idx+1:]))
			column = strings.TrimSpace(column[:idx])
			switch direction {
			case "asc":
				desc = false
			case "desc":
				desc = true
			default:
				// Unknown direction: ignore the whole clause.
				continue
			}
		}

		if !validColumnNamePattern.MatchString(column) {
			// SQL injection attempt or malformed name: drop it.
			continue
		}

		sorts = append(sorts, SortClause{Column: column, Desc: desc})
		if len(sorts) >= MaxSorts {
			break
		}
	}

	if len(sorts) == 0 {
		return nil
	}
	return sorts
}

// ParseFilters extracts the filter clauses from the raw query values.
//
// Supported keys:
//
//	filter[column]=value              -> operator "eq"
//	filter[column][operator]=value    -> explicit operator
//
// Clauses with an unknown operator or with a malformed column name are
// ignored. The remaining clauses are sorted by column and operator so the
// generated SQL is deterministic.
func ParseFilters(values url.Values) []FilterCondition {
	if len(values) == 0 {
		return nil
	}

	filters := make([]FilterCondition, 0, len(values))

	for key, vals := range values {
		if !strings.HasPrefix(key, "filter[") {
			continue
		}

		matches := filterKeyPattern.FindStringSubmatch(key)
		if matches == nil {
			continue
		}

		column := matches[1]
		operator := matches[2]
		if operator == "" {
			operator = DefaultOperator
		}
		operator = strings.ToLower(strings.TrimSpace(operator))

		if !validFilterOperators[operator] {
			// Unsupported operator: never forwarded to the SQL builder.
			continue
		}

		if len(vals) == 0 {
			continue
		}

		filters = append(filters, FilterCondition{
			Column:   column,
			Operator: operator,
			Value:    vals[0],
		})
	}

	if len(filters) == 0 {
		return nil
	}

	sort.Slice(filters, func(i, j int) bool {
		if filters[i].Column != filters[j].Column {
			return filters[i].Column < filters[j].Column
		}
		return filters[i].Operator < filters[j].Operator
	})

	if len(filters) > MaxFilters {
		filters = filters[:MaxFilters]
	}

	return filters
}

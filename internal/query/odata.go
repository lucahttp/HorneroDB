package query

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrInvalidFilter      = errors.New("invalid filter expression")
	ErrColumnAccessDenied = errors.New("column access denied")
	ErrInvalidOrderBy     = errors.New("invalid orderby expression")
	identPattern          = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

type ODataQuery struct {
	FilterSQL  string
	FilterArgs []interface{}
	OrderBy    string
	Select     []string
	Top        int
	Skip       int
	Expand     string
}

// ParseODataQuery parses OData query parameters from url.Values and validates column permissions
func ParseODataQuery(values url.Values, allowedColumns []string) (*ODataQuery, error) {
	q := &ODataQuery{
		Top:  ExtractLimit(values),
		Skip: ExtractOffset(values),
	}

	// 1. Expand
	q.Expand = getFirstValue(values, "$expand", "expand")

	// 2. Select
	selectRaw := getFirstValue(values, "$select", "select")
	if selectRaw != "" && selectRaw != "*" {
		parts := strings.Split(selectRaw, ",")
		for _, p := range parts {
			col := strings.TrimSpace(p)
			if col == "" {
				continue
			}
			if !isValidIdentifier(col) {
				return nil, fmt.Errorf("%w: invalid select column %q", ErrInvalidFilter, col)
			}
			if !isColumnAllowed(col, allowedColumns) {
				return nil, fmt.Errorf("%w: column %q is not accessible", ErrColumnAccessDenied, col)
			}
			q.Select = append(q.Select, col)
		}
	}

	// 3. OrderBy
	orderByRaw := getFirstValue(values, "$orderby", "orderby", "sort")
	if orderByRaw != "" {
		sqlOrder, err := parseOrderBy(orderByRaw, allowedColumns)
		if err != nil {
			return nil, err
		}
		q.OrderBy = sqlOrder
	}

	// 4. Filter
	filterRaw := getFirstValue(values, "$filter", "filter")
	if filterRaw != "" {
		sqlFilter, args, err := parseFilter(filterRaw, allowedColumns)
		if err != nil {
			return nil, err
		}
		q.FilterSQL = sqlFilter
		q.FilterArgs = args
	}

	return q, nil
}

// ParseODataFromArgs parses OData options from MCP map[string]interface{} arguments
func ParseODataFromArgs(args map[string]interface{}, allowedColumns []string) (*ODataQuery, error) {
	values := url.Values{}

	if f, ok := args["filter"].(string); ok && f != "" {
		values.Set("$filter", f)
	}
	if ob, ok := args["orderby"].(string); ok && ob != "" {
		values.Set("$orderby", ob)
	}
	if s, ok := args["select"].(string); ok && s != "" {
		values.Set("$select", s)
	}
	if exp, ok := args["expand"].(string); ok && exp != "" {
		values.Set("$expand", exp)
	}
	if l, ok := args["limit"].(float64); ok {
		values.Set("limit", strconv.Itoa(int(l)))
	} else if l, ok := args["top"].(float64); ok {
		values.Set("$top", strconv.Itoa(int(l)))
	}
	if o, ok := args["offset"].(float64); ok {
		values.Set("offset", strconv.Itoa(int(o)))
	} else if o, ok := args["skip"].(float64); ok {
		values.Set("$skip", strconv.Itoa(int(o)))
	}

	return ParseODataQuery(values, allowedColumns)
}

func ExtractLimit(values url.Values) int {
	raw := getFirstValue(values, "$top", "top", "$limit", "limit")
	if raw == "" {
		return 50 // Default
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		return 50
	}
	if val > 1000 {
		return 1000 // Max
	}
	return val
}

func ExtractOffset(values url.Values) int {
	raw := getFirstValue(values, "$skip", "skip", "$offset", "offset")
	if raw == "" {
		return 0
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val < 0 {
		return 0
	}
	return val
}

func getFirstValue(values url.Values, keys ...string) string {
	for _, k := range keys {
		if v := values.Get(k); v != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isValidIdentifier(name string) bool {
	return identPattern.MatchString(name)
}

func isColumnAllowed(col string, allowedColumns []string) bool {
	if len(allowedColumns) == 0 {
		return true
	}
	for _, a := range allowedColumns {
		if a == "*" || a == col {
			return true
		}
	}
	for _, sys := range []string{"id", "created_at", "updated_at", "created_by"} {
		if col == sys {
			return true
		}
	}
	return false
}

func parseOrderBy(raw string, allowedColumns []string) (string, error) {
	clauses := strings.Split(raw, ",")
	var result []string

	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}
		parts := strings.Fields(clause)
		if len(parts) == 0 || len(parts) > 2 {
			return "", fmt.Errorf("%w: invalid orderby clause %q", ErrInvalidOrderBy, clause)
		}

		col := parts[0]
		if !isValidIdentifier(col) {
			return "", fmt.Errorf("%w: invalid orderby column %q", ErrInvalidOrderBy, col)
		}
		if !isColumnAllowed(col, allowedColumns) {
			return "", fmt.Errorf("%w: column %q is not accessible", ErrColumnAccessDenied, col)
		}

		dir := "ASC"
		if len(parts) == 2 {
			d := strings.ToUpper(parts[1])
			if d != "ASC" && d != "DESC" {
				return "", fmt.Errorf("%w: invalid direction %q (use asc or desc)", ErrInvalidOrderBy, parts[1])
			}
			dir = d
		}
		result = append(result, fmt.Sprintf("%s %s", col, dir))
	}

	if len(result) == 0 {
		return "", nil
	}
	return strings.Join(result, ", "), nil
}

// Filter Tokenizer & Parser
type tokenType int

const (
	tokIdent tokenType = iota
	tokString
	tokNumber
	tokBool
	tokNull
	tokOp
	tokLParen
	tokRParen
	tokComma
	tokEOF
)

type token struct {
	typ tokenType
	val string
}

func parseFilter(raw string, allowedColumns []string) (string, []interface{}, error) {
	tokens, err := tokenize(raw)
	if err != nil {
		return "", nil, err
	}
	if len(tokens) == 0 || (len(tokens) == 1 && tokens[0].typ == tokEOF) {
		return "", nil, nil
	}

	p := &filterParser{
		tokens:         tokens,
		pos:            0,
		allowedColumns: allowedColumns,
	}

	sql, args, err := p.parseExpression()
	if err != nil {
		return "", nil, err
	}
	if p.pos < len(p.tokens) && p.tokens[p.pos].typ != tokEOF {
		return "", nil, fmt.Errorf("%w: unexpected token %q at position %d", ErrInvalidFilter, p.tokens[p.pos].val, p.pos)
	}

	return sql, args, nil
}

type filterParser struct {
	tokens         []token
	pos            int
	allowedColumns []string
}

func (p *filterParser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *filterParser) next() token {
	t := p.peek()
	p.pos++
	return t
}

func (p *filterParser) parseExpression() (string, []interface{}, error) {
	return p.parseOr()
}

func (p *filterParser) parseOr() (string, []interface{}, error) {
	leftSQL, leftArgs, err := p.parseAnd()
	if err != nil {
		return "", nil, err
	}

	for p.peek().typ == tokOp && strings.ToLower(p.peek().val) == "or" {
		p.next() // consume 'or'
		rightSQL, rightArgs, err := p.parseAnd()
		if err != nil {
			return "", nil, err
		}
		leftSQL = fmt.Sprintf("(%s OR %s)", leftSQL, rightSQL)
		leftArgs = append(leftArgs, rightArgs...)
	}

	return leftSQL, leftArgs, nil
}

func (p *filterParser) parseAnd() (string, []interface{}, error) {
	leftSQL, leftArgs, err := p.parseUnary()
	if err != nil {
		return "", nil, err
	}

	for p.peek().typ == tokOp && strings.ToLower(p.peek().val) == "and" {
		p.next() // consume 'and'
		rightSQL, rightArgs, err := p.parseUnary()
		if err != nil {
			return "", nil, err
		}
		leftSQL = fmt.Sprintf("(%s AND %s)", leftSQL, rightSQL)
		leftArgs = append(leftArgs, rightArgs...)
	}

	return leftSQL, leftArgs, nil
}

func (p *filterParser) parseUnary() (string, []interface{}, error) {
	if p.peek().typ == tokOp && strings.ToLower(p.peek().val) == "not" {
		p.next()
		sql, args, err := p.parseUnary()
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("NOT (%s)", sql), args, nil
	}
	return p.parsePrimary()
}

func (p *filterParser) parsePrimary() (string, []interface{}, error) {
	tok := p.peek()

	// Parenthesized subexpression
	if tok.typ == tokLParen {
		p.next()
		sql, args, err := p.parseExpression()
		if err != nil {
			return "", nil, err
		}
		if p.peek().typ != tokRParen {
			return "", nil, fmt.Errorf("%w: missing closing parenthesis", ErrInvalidFilter)
		}
		p.next()
		return sql, args, nil
	}

	// Function call: contains(col, 'str'), startswith(col, 'str'), endswith(col, 'str')
	if tok.typ == tokIdent {
		fnName := strings.ToLower(tok.val)
		if (fnName == "contains" || fnName == "startswith" || fnName == "endswith" || fnName == "substringof") && p.peekAhead(1).typ == tokLParen {
			return p.parseFunctionCall(fnName)
		}
	}

	// Binary comparison: Column op Value
	if tok.typ != tokIdent {
		return "", nil, fmt.Errorf("%w: expected column name, got %q", ErrInvalidFilter, tok.val)
	}

	colTok := p.next()
	col := colTok.val
	if !isValidIdentifier(col) {
		return "", nil, fmt.Errorf("%w: invalid column identifier %q", ErrInvalidFilter, col)
	}
	if !isColumnAllowed(col, p.allowedColumns) {
		return "", nil, fmt.Errorf("%w: column %q is not accessible", ErrColumnAccessDenied, col)
	}

	opTok := p.next()
	if opTok.typ != tokOp {
		return "", nil, fmt.Errorf("%w: expected operator after column %q, got %q", ErrInvalidFilter, col, opTok.val)
	}

	op := strings.ToLower(opTok.val)

	// IN operator: col in ('a', 'b')
	if op == "in" {
		return p.parseInClause(col)
	}

	valTok := p.next()
	val, err := parseTokenValue(valTok)
	if err != nil {
		return "", nil, err
	}

	switch op {
	case "eq", "=":
		if val == nil {
			return fmt.Sprintf("%s IS NULL", col), nil, nil
		}
		return fmt.Sprintf("%s = ?", col), []interface{}{val}, nil
	case "ne", "!=", "<>":
		if val == nil {
			return fmt.Sprintf("%s IS NOT NULL", col), nil, nil
		}
		return fmt.Sprintf("%s != ?", col), []interface{}{val}, nil
	case "gt", ">":
		return fmt.Sprintf("%s > ?", col), []interface{}{val}, nil
	case "ge", ">=":
		return fmt.Sprintf("%s >= ?", col), []interface{}{val}, nil
	case "lt", "<":
		return fmt.Sprintf("%s < ?", col), []interface{}{val}, nil
	case "le", "<=":
		return fmt.Sprintf("%s <= ?", col), []interface{}{val}, nil
	default:
		return "", nil, fmt.Errorf("%w: unsupported operator %q", ErrInvalidFilter, op)
	}
}

func (p *filterParser) parseFunctionCall(fnName string) (string, []interface{}, error) {
	p.next() // function name
	p.next() // '('

	firstTok := p.next()
	if p.next().typ != tokComma {
		return "", nil, fmt.Errorf("%w: function %s requires 2 arguments", ErrInvalidFilter, fnName)
	}
	secondTok := p.next()
	if p.next().typ != tokRParen {
		return "", nil, fmt.Errorf("%w: missing closing parenthesis for function %s", ErrInvalidFilter, fnName)
	}

	var col string
	var val interface{}

	if fnName == "substringof" {
		// OData v2: substringof('needle', col)
		v, err := parseTokenValue(firstTok)
		if err != nil {
			return "", nil, err
		}
		val = v
		col = secondTok.val
	} else {
		// OData v4: contains(col, 'needle'), startswith(col, 'needle'), endswith(col, 'needle')
		col = firstTok.val
		v, err := parseTokenValue(secondTok)
		if err != nil {
			return "", nil, err
		}
		val = v
	}

	if !isValidIdentifier(col) {
		return "", nil, fmt.Errorf("%w: invalid column identifier %q", ErrInvalidFilter, col)
	}
	if !isColumnAllowed(col, p.allowedColumns) {
		return "", nil, fmt.Errorf("%w: column %q is not accessible", ErrColumnAccessDenied, col)
	}

	strVal := fmt.Sprint(val)
	switch fnName {
	case "contains", "substringof":
		return fmt.Sprintf("%s ILIKE ?", col), []interface{}{"%" + strVal + "%"}, nil
	case "startswith":
		return fmt.Sprintf("%s ILIKE ?", col), []interface{}{strVal + "%"}, nil
	case "endswith":
		return fmt.Sprintf("%s ILIKE ?", col), []interface{}{"%" + strVal}, nil
	}
	return "", nil, fmt.Errorf("%w: unknown function %s", ErrInvalidFilter, fnName)
}

func (p *filterParser) parseInClause(col string) (string, []interface{}, error) {
	if p.next().typ != tokLParen {
		return "", nil, fmt.Errorf("%w: expected '(' after IN", ErrInvalidFilter)
	}

	var args []interface{}
	var placeholders []string

	for {
		tok := p.next()
		if tok.typ == tokRParen {
			break
		}
		val, err := parseTokenValue(tok)
		if err != nil {
			return "", nil, err
		}
		args = append(args, val)
		placeholders = append(placeholders, "?")

		nextTok := p.peek()
		if nextTok.typ == tokComma {
			p.next()
		} else if nextTok.typ == tokRParen {
			p.next()
			break
		} else {
			return "", nil, fmt.Errorf("%w: expected comma or ')' in IN list", ErrInvalidFilter)
		}
	}

	if len(args) == 0 {
		return "1=0", nil, nil // Empty IN is always false
	}

	return fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ",")), args, nil
}

func (p *filterParser) peekAhead(n int) token {
	idx := p.pos + n
	if idx >= len(p.tokens) {
		return token{typ: tokEOF}
	}
	return p.tokens[idx]
}

func parseTokenValue(tok token) (interface{}, error) {
	switch tok.typ {
	case tokString:
		return tok.val, nil
	case tokNumber:
		if strings.Contains(tok.val, ".") {
			return strconv.ParseFloat(tok.val, 64)
		}
		return strconv.ParseInt(tok.val, 10, 64)
	case tokBool:
		return strings.ToLower(tok.val) == "true", nil
	case tokNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: expected literal value, got %q", ErrInvalidFilter, tok.val)
	}
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	runes := []rune(input)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		if unicode.IsSpace(r) {
			i++
			continue
		}

		if r == '(' {
			tokens = append(tokens, token{typ: tokLParen, val: "("})
			i++
			continue
		}
		if r == ')' {
			tokens = append(tokens, token{typ: tokRParen, val: ")"})
			i++
			continue
		}
		if r == ',' {
			tokens = append(tokens, token{typ: tokComma, val: ","})
			i++
			continue
		}

		// String literals: 'string' or "string"
		if r == '\'' || r == '"' {
			quote := r
			i++
			var sb strings.Builder
			for i < n {
				if runes[i] == quote {
					if i+1 < n && runes[i+1] == quote { // escaped quote ''
						sb.WriteRune(quote)
						i += 2
						continue
					}
					i++
					break
				}
				sb.WriteRune(runes[i])
				i++
			}
			tokens = append(tokens, token{typ: tokString, val: sb.String()})
			continue
		}

		// Numbers (integer or decimal)
		if unicode.IsDigit(r) || (r == '-' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			start := i
			i++
			hasDot := false
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				if runes[i] == '.' {
					if hasDot {
						break
					}
					hasDot = true
				}
				i++
			}
			tokens = append(tokens, token{typ: tokNumber, val: string(runes[start:i])})
			continue
		}

		// Symbols / operators: =, !=, <>, >, >=, <, <=
		if r == '=' {
			tokens = append(tokens, token{typ: tokOp, val: "="})
			i++
			continue
		}
		if r == '!' && i+1 < n && runes[i+1] == '=' {
			tokens = append(tokens, token{typ: tokOp, val: "!="})
			i += 2
			continue
		}
		if r == '<' {
			if i+1 < n && runes[i+1] == '=' {
				tokens = append(tokens, token{typ: tokOp, val: "<="})
				i += 2
			} else if i+1 < n && runes[i+1] == '>' {
				tokens = append(tokens, token{typ: tokOp, val: "<>"})
				i += 2
			} else {
				tokens = append(tokens, token{typ: tokOp, val: "<"})
				i++
			}
			continue
		}
		if r == '>' {
			if i+1 < n && runes[i+1] == '=' {
				tokens = append(tokens, token{typ: tokOp, val: ">="})
				i += 2
			} else {
				tokens = append(tokens, token{typ: tokOp, val: ">"})
				i++
			}
			continue
		}

		// Identifiers, keywords, operators (eq, ne, and, or, not, true, false, null, in)
		if unicode.IsLetter(r) || r == '_' {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			lower := strings.ToLower(word)

			switch lower {
			case "eq", "ne", "gt", "ge", "lt", "le", "and", "or", "not", "in":
				tokens = append(tokens, token{typ: tokOp, val: lower})
			case "true", "false":
				tokens = append(tokens, token{typ: tokBool, val: lower})
			case "null":
				tokens = append(tokens, token{typ: tokNull, val: lower})
			default:
				tokens = append(tokens, token{typ: tokIdent, val: word})
			}
			continue
		}

		return nil, fmt.Errorf("%w: unexpected character %q at position %d", ErrInvalidFilter, string(r), i)
	}

	tokens = append(tokens, token{typ: tokEOF})
	return tokens, nil
}

package query

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokInt
	tokFloat
	tokString
	tokPunct // single character punctuation
	tokArrow // -> or <-
	tokDash  // -
	tokDoubleDash // --
	tokLParen
	tokRParen
	tokLBracket
	tokRBracket
	tokDot
	tokComma
	tokColon
	tokPipe
	tokStar
	tokPlus
	tokMinus
	tokEq
	tokNEq
	tokLt
	tokGt
	tokLtEq
	tokGtEq
	tokDollar
	tokDotDot
)

type token struct {
	kind  tokenKind
	value string
	line  int
	col   int
}

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

type lexer struct {
	input  []rune
	pos    int
	line   int
	col    int
	tokens []token
}

func lex(input string) ([]token, error) {
	l := &lexer{
		input: []rune(input),
		line:  1,
		col:   1,
	}
	if err := l.tokenise(); err != nil {
		return nil, err
	}
	return l.tokens, nil
}

func (l *lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *lexer) peek2() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	return l.input[l.pos+1]
}

func (l *lexer) advance() rune {
	ch := l.input[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *lexer) emit(kind tokenKind, value string, line, col int) {
	l.tokens = append(l.tokens, token{kind: kind, value: value, line: line, col: col})
}

func (l *lexer) tokenise() error {
	for l.pos < len(l.input) {
		ch := l.peek()

		// Skip whitespace.
		if unicode.IsSpace(ch) {
			l.advance()
			continue
		}

		// Skip line comments (// …).
		if ch == '/' && l.peek2() == '/' {
			for l.pos < len(l.input) && l.peek() != '\n' {
				l.advance()
			}
			continue
		}

		line, col := l.line, l.col

		switch {
		case ch == '(':
			l.advance()
			l.emit(tokLParen, "(", line, col)

		case ch == ')':
			l.advance()
			l.emit(tokRParen, ")", line, col)

		case ch == '[':
			l.advance()
			l.emit(tokLBracket, "[", line, col)

		case ch == ']':
			l.advance()
			l.emit(tokRBracket, "]", line, col)

		case ch == ',':
			l.advance()
			l.emit(tokComma, ",", line, col)

		case ch == '.':
			if l.peek2() == '.' {
				l.advance()
				l.advance()
				l.emit(tokDotDot, "..", line, col)
			} else {
				l.advance()
				l.emit(tokDot, ".", line, col)
			}

		case ch == ':':
			l.advance()
			l.emit(tokColon, ":", line, col)

		case ch == '|':
			l.advance()
			l.emit(tokPipe, "|", line, col)

		case ch == '*':
			l.advance()
			l.emit(tokStar, "*", line, col)

		case ch == '+':
			l.advance()
			l.emit(tokPlus, "+", line, col)

		case ch == '$':
			l.advance()
			l.emit(tokDollar, "$", line, col)

		case ch == '=':
			l.advance()
			l.emit(tokEq, "=", line, col)

		case ch == '!':
			l.advance()
			if l.peek() == '=' {
				l.advance()
				l.emit(tokNEq, "!=", line, col)
			} else {
				return &QueryError{Line: line, Column: col, Message: fmt.Sprintf("unexpected character '!' at line %d column %d", line, col)}
			}

		case ch == '<':
			l.advance()
			if l.peek() == '>' {
				l.advance()
				l.emit(tokNEq, "<>", line, col)
			} else if l.peek() == '=' {
				l.advance()
				l.emit(tokLtEq, "<=", line, col)
			} else if l.peek() == '-' {
				l.advance()
				l.emit(tokArrow, "<-", line, col)
			} else {
				l.emit(tokLt, "<", line, col)
			}

		case ch == '>':
			l.advance()
			if l.peek() == '=' {
				l.advance()
				l.emit(tokGtEq, ">=", line, col)
			} else {
				l.emit(tokGt, ">", line, col)
			}

		case ch == '-':
			l.advance()
			if l.peek() == '>' {
				l.advance()
				l.emit(tokArrow, "->", line, col)
			} else if l.peek() == '-' {
				l.advance()
				l.emit(tokDoubleDash, "--", line, col)
			} else {
				l.emit(tokMinus, "-", line, col)
			}

		case ch == '"' || ch == '\'':
			s, err := l.readString(ch)
			if err != nil {
				return err
			}
			l.emit(tokString, s, line, col)

		case unicode.IsLetter(ch) || ch == '_':
			word := l.readIdent()
			l.emit(tokIdent, word, line, col)

		case unicode.IsDigit(ch):
			num, kind, err := l.readNumber()
			if err != nil {
				return err
			}
			l.emit(kind, num, line, col)

		default:
			return &QueryError{
				Line:    line,
				Column:  col,
				Message: fmt.Sprintf("unexpected character %q at line %d column %d", string(ch), line, col),
			}
		}
	}
	l.emit(tokEOF, "", l.line, l.col)
	return nil
}

func (l *lexer) readIdent() string {
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	return sb.String()
}

func (l *lexer) readNumber() (string, tokenKind, error) {
	var sb strings.Builder
	isFloat := false
	for l.pos < len(l.input) {
		ch := l.peek()
		if unicode.IsDigit(ch) {
			sb.WriteRune(l.advance())
		} else if ch == '.' && l.peek2() != '.' {
			isFloat = true
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	if isFloat {
		return sb.String(), tokFloat, nil
	}
	return sb.String(), tokInt, nil
}

func (l *lexer) readString(quote rune) (string, error) {
	l.advance() // consume opening quote
	var sb strings.Builder
	for l.pos < len(l.input) {
		ch := l.advance()
		if rune(ch) == quote {
			return sb.String(), nil
		}
		if ch == '\\' && l.pos < len(l.input) {
			next := l.advance()
			switch next {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case '\\':
				sb.WriteRune('\\')
			default:
				sb.WriteRune(rune(next))
			}
			continue
		}
		sb.WriteRune(rune(ch))
	}
	return "", &QueryError{Message: "unterminated string literal"}
}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

// parser is a recursive-descent parser for the Cypher subset.
type parser struct {
	tokens []token
	pos    int
	query  string
}

func newParser(query string) (*parser, error) {
	tokens, err := lex(query)
	if err != nil {
		return nil, err
	}
	return &parser{tokens: tokens, query: query}, nil
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) peekN(n int) token {
	idx := p.pos + n
	if idx >= len(p.tokens) {
		return token{kind: tokEOF}
	}
	return p.tokens[idx]
}

func (p *parser) consume() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(kind tokenKind, literal string) (token, error) {
	t := p.peek()
	if t.kind != kind {
		return token{}, p.errorf(t, "expected %q but found %q", literal, t.value)
	}
	if literal != "" && !strings.EqualFold(t.value, literal) {
		return token{}, p.errorf(t, "expected %q but found %q", literal, t.value)
	}
	return p.consume(), nil
}

func (p *parser) expectIdent(keyword string) error {
	t := p.peek()
	if t.kind != tokIdent || !strings.EqualFold(t.value, keyword) {
		return p.errorf(t, "expected %q but found %q at line %d column %d", keyword, t.value, t.line, t.col)
	}
	p.consume()
	return nil
}

func (p *parser) errorf(t token, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return &QueryError{
		Query:   p.query,
		Message: msg,
		Line:    t.line,
		Column:  t.col,
	}
}

// keywordIs returns true when the current token is the given keyword (case-insensitive).
func (p *parser) keywordIs(kw string) bool {
	t := p.peek()
	return t.kind == tokIdent && strings.EqualFold(t.value, kw)
}

// Parse is the entry point. It rejects mutation and unsupported keywords before
// building the AST.
func Parse(query string) (*Statement, error) {
	if err := rejectForbiddenKeywords(query); err != nil {
		return nil, err
	}

	p, err := newParser(query)
	if err != nil {
		return nil, err
	}
	return p.parseStatement()
}

// rejectForbiddenKeywords does a quick keyword scan before full parsing.
func rejectForbiddenKeywords(query string) error {
	upper := strings.ToUpper(query)
	// Mutation keywords — must appear as whole words.
	for _, kw := range []string{"CREATE", "SET", "DELETE", "MERGE", "REMOVE"} {
		if containsWholeWord(upper, kw) {
			return &MutationError{Keyword: kw}
		}
	}
	// Unsupported but legal Cypher clauses.
	for _, kw := range []string{"WITH", "UNWIND", "CASE"} {
		if containsWholeWord(upper, kw) {
			return &UnsupportedClauseError{Keyword: kw}
		}
	}
	// OPTIONAL MATCH — check the pair.
	if strings.Contains(upper, "OPTIONAL") {
		return &UnsupportedClauseError{Keyword: "OPTIONAL MATCH"}
	}
	return nil
}

// containsWholeWord reports whether s contains kw as a standalone word.
func containsWholeWord(s, kw string) bool {
	idx := 0
	for {
		i := strings.Index(s[idx:], kw)
		if i < 0 {
			return false
		}
		abs := idx + i
		before := abs == 0 || !isIdentRune(rune(s[abs-1]))
		after := abs+len(kw) >= len(s) || !isIdentRune(rune(s[abs+len(kw)]))
		if before && after {
			return true
		}
		idx = abs + 1
	}
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// ---------------------------------------------------------------------------
// Grammar rules
// ---------------------------------------------------------------------------

func (p *parser) parseStatement() (*Statement, error) {
	stmt := &Statement{}

	match, err := p.parseMatchClause()
	if err != nil {
		return nil, err
	}
	stmt.Match = match

	// Optional WHERE
	if p.keywordIs("WHERE") {
		where, err := p.parseWhereClause()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// Required RETURN
	if !p.keywordIs("RETURN") {
		t := p.peek()
		return nil, p.errorf(t, "expected RETURN but found %q at line %d column %d", t.value, t.line, t.col)
	}
	ret, err := p.parseReturnClause()
	if err != nil {
		return nil, err
	}
	stmt.Return = ret

	// Optional ORDER BY
	if p.keywordIs("ORDER") {
		orderBy, err := p.parseOrderByClause()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// Optional LIMIT
	if p.keywordIs("LIMIT") {
		limit, err := p.parseLimitClause()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	// Ensure nothing remains.
	if p.peek().kind != tokEOF {
		t := p.peek()
		return nil, p.errorf(t, "unexpected token %q at line %d column %d", t.value, t.line, t.col)
	}

	return stmt, nil
}

func (p *parser) parseMatchClause() (*MatchClause, error) {
	if err := p.expectIdent("MATCH"); err != nil {
		return nil, err
	}

	mc := &MatchClause{}
	pattern, err := p.parsePathPattern()
	if err != nil {
		return nil, err
	}
	mc.Patterns = append(mc.Patterns, pattern)

	for p.peek().kind == tokComma {
		p.consume()
		pattern, err = p.parsePathPattern()
		if err != nil {
			return nil, err
		}
		mc.Patterns = append(mc.Patterns, pattern)
	}

	return mc, nil
}

func (p *parser) parsePathPattern() (*PathPattern, error) {
	pp := &PathPattern{}

	start, err := p.parseNodePattern()
	if err != nil {
		return nil, err
	}
	pp.Start = start

	// Parse chained steps: (-[edge]->(node))*
	for {
		if p.peek().kind == tokMinus || p.peek().kind == tokArrow || p.peek().kind == tokDoubleDash {
			step, err := p.parsePathStep()
			if err != nil {
				return nil, err
			}
			pp.Steps = append(pp.Steps, step)
		} else {
			break
		}
	}

	return pp, nil
}

func (p *parser) parseNodePattern() (*NodePattern, error) {
	t := p.peek()
	if t.kind != tokLParen {
		return nil, p.errorf(t, "expected '(' to begin node pattern but found %q at line %d column %d", t.value, t.line, t.col)
	}
	p.consume()

	np := &NodePattern{}

	// Optional variable name.
	if p.peek().kind == tokIdent && !isReservedKeyword(p.peek().value) {
		np.Variable = p.consume().value
	}

	// Optional label(s): :TypeA|TypeB
	for p.peek().kind == tokColon {
		p.consume()
		lt := p.peek()
		if lt.kind != tokIdent {
			return nil, p.errorf(lt, "expected edge type after ':' but found %q at line %d column %d", lt.value, lt.line, lt.col)
		}
		np.Labels = append(np.Labels, p.consume().value)
		for p.peek().kind == tokPipe {
			p.consume()
			lt = p.peek()
			if lt.kind != tokIdent {
				return nil, p.errorf(lt, "expected label after '|' but found %q at line %d column %d", lt.value, lt.line, lt.col)
			}
			np.Labels = append(np.Labels, p.consume().value)
		}
	}

	// Consume closing ')'.
	if p.peek().kind != tokRParen {
		t = p.peek()
		return nil, p.errorf(t, "expected ')' to close node pattern but found %q at line %d column %d", t.value, t.line, t.col)
	}
	p.consume()

	return np, nil
}

// parsePathStep parses a single edge+node step in a path chain.
func (p *parser) parsePathStep() (*PathStep, error) {
	edge, err := p.parseEdgePattern()
	if err != nil {
		return nil, err
	}
	node, err := p.parseNodePattern()
	if err != nil {
		return nil, err
	}
	return &PathStep{Edge: edge, Node: node}, nil
}

// parseEdgePattern parses the edge portion of a step.
// Supported forms:
//
//	-->          (any edge, outgoing)
//	<--          (any edge, incoming)
//	--           (any edge, undirected)
//	-[:type]->   (typed, outgoing)
//	<-[:type]-   (typed, incoming)
//	-[:type]-    (typed, undirected)
//	-[*1..3]->   (variable-length, outgoing)
func (p *parser) parseEdgePattern() (*EdgePattern, error) {
	ep := &EdgePattern{Direction: "none"}

	t := p.peek()

	// Undirected --
	if t.kind == tokDoubleDash {
		p.consume()
		ep.Direction = "none"
		return ep, nil
	}

	// Incoming: starts with <-
	if t.kind == tokArrow && t.value == "<-" {
		p.consume()
		ep.Direction = "in"

		// Optional bracket section.
		if p.peek().kind == tokLBracket {
			types, varLen, err := p.parseBracketSection()
			if err != nil {
				return nil, err
			}
			ep.Types = types
			ep.VarLength = varLen
		}

		// Consume trailing -
		if p.peek().kind == tokMinus {
			p.consume()
		} else {
			// No trailing dash means just <- with no bracket = still valid.
		}

		return ep, nil
	}

	// Outgoing or undirected: starts with -
	if t.kind == tokMinus {
		p.consume()

		// Optional bracket section.
		if p.peek().kind == tokLBracket {
			types, varLen, err := p.parseBracketSection()
			if err != nil {
				return nil, err
			}
			ep.Types = types
			ep.VarLength = varLen
		}

		// What follows the bracket (or bare dash)?
		next := p.peek()
		if next.kind == tokArrow && next.value == "->" {
			p.consume()
			ep.Direction = "out"
		} else if next.kind == tokMinus {
			p.consume()
			ep.Direction = "none"
		}
		// Else direction remains "none" (undirected without trailing dash).

		return ep, nil
	}

	return nil, p.errorf(t, "expected edge pattern (-, --, <-, or ->) but found %q at line %d column %d", t.value, t.line, t.col)
}

// parseBracketSection parses the [...] portion of an edge pattern.
// Returns (types, varLength, error).
func (p *parser) parseBracketSection() ([]string, *VarLengthSpec, error) {
	p.consume() // consume [

	var types []string
	var varLen *VarLengthSpec

	t := p.peek()

	// Variable-length: [*min..max] or [*] or [*min] or [*..max]
	if t.kind == tokStar {
		p.consume()
		spec, err := p.parseVarLengthSpec()
		if err != nil {
			return nil, nil, err
		}
		varLen = spec
	} else if t.kind == tokColon {
		// Type label(s): [:type1|type2]
		for p.peek().kind == tokColon {
			p.consume()
			lt := p.peek()
			if lt.kind != tokIdent {
				return nil, nil, p.errorf(lt, "expected edge type after ':' but found %q at line %d column %d", lt.value, lt.line, lt.col)
			}
			types = append(types, p.consume().value)
			for p.peek().kind == tokPipe {
				p.consume()
				lt = p.peek()
				if lt.kind != tokIdent {
					return nil, nil, p.errorf(lt, "expected edge type after '|' but found %q at line %d column %d", lt.value, lt.line, lt.col)
				}
				types = append(types, p.consume().value)
			}
		}
	}
	// else empty brackets [] — any type

	if p.peek().kind != tokRBracket {
		t = p.peek()
		return nil, nil, p.errorf(t, "expected ']' to close edge pattern but found %q at line %d column %d", t.value, t.line, t.col)
	}
	p.consume()

	return types, varLen, nil
}

// parseVarLengthSpec parses the numeric bounds after * in [*…].
// Supported: [*], [*n], [*n..m], [*..m]
func (p *parser) parseVarLengthSpec() (*VarLengthSpec, error) {
	spec := &VarLengthSpec{Min: 1, Max: defaultMaxPathDepth}

	t := p.peek()

	if t.kind == tokInt {
		n, err := strconv.Atoi(t.value)
		if err != nil {
			return nil, p.errorf(t, "invalid path length %q", t.value)
		}
		p.consume()
		spec.Min = n
		spec.Max = n

		// Optional ..max
		if p.peek().kind == tokDotDot {
			p.consume()
			if p.peek().kind == tokInt {
				m, err := strconv.Atoi(p.peek().value)
				if err != nil {
					return nil, p.errorf(p.peek(), "invalid path length %q", p.peek().value)
				}
				p.consume()
				spec.Max = m
			}
		}
	} else if t.kind == tokDotDot {
		p.consume()
		if p.peek().kind == tokInt {
			m, err := strconv.Atoi(p.peek().value)
			if err != nil {
				return nil, p.errorf(p.peek(), "invalid path length %q", p.peek().value)
			}
			p.consume()
			spec.Max = m
		}
	}
	// else [*] — use defaults

	return spec, nil
}

// ---------------------------------------------------------------------------
// WHERE clause
// ---------------------------------------------------------------------------

func (p *parser) parseWhereClause() (*WhereClause, error) {
	p.consume() // consume WHERE

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &WhereClause{Expr: expr}, nil
}

// ---------------------------------------------------------------------------
// Expression parsing (recursive descent, operator precedence)
// ---------------------------------------------------------------------------

// parseExpression is the top-level expression entry point (lowest precedence: OR).
func (p *parser) parseExpression() (Expression, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.keywordIs("OR") {
		p.consume()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Operator: "OR", Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expression, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.keywordIs("AND") {
		p.consume()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Operator: "AND", Right: right}
	}
	return left, nil
}

func (p *parser) parseNot() (Expression, error) {
	if p.keywordIs("NOT") {
		p.consume()
		operand, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Operator: "NOT", Operand: operand}, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (Expression, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}

	// IS NULL / IS NOT NULL
	if p.keywordIs("IS") {
		p.consume()
		negated := false
		if p.keywordIs("NOT") {
			p.consume()
			negated = true
		}
		if err := p.expectIdent("NULL"); err != nil {
			return nil, err
		}
		return &IsNullExpr{Operand: left, Negated: negated}, nil
	}

	ops := map[tokenKind]string{
		tokEq:   "=",
		tokNEq:  "<>",
		tokLt:   "<",
		tokGt:   ">",
		tokLtEq: "<=",
		tokGtEq: ">=",
	}
	op, ok := ops[p.peek().kind]
	if !ok {
		return left, nil
	}
	p.consume()

	right, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	return &BinaryExpr{Left: left, Operator: op, Right: right}, nil
}

func (p *parser) parseAdditive() (Expression, error) {
	// Used for date arithmetic on builtins; no general arithmetic.
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expression, error) {
	t := p.peek()

	// Parenthesised expression.
	if t.kind == tokLParen {
		p.consume()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			nt := p.peek()
			return nil, p.errorf(nt, "expected ')' but found %q at line %d column %d", nt.value, nt.line, nt.col)
		}
		p.consume()
		return expr, nil
	}

	// Built-in variable: $today, $now, $week_start, $month_start
	if t.kind == tokDollar {
		return p.parseBuiltinVariable()
	}

	// String literal.
	if t.kind == tokString {
		p.consume()
		return &StringLiteral{Value: t.value}, nil
	}

	// Integer or float literal.
	if t.kind == tokInt {
		p.consume()
		n, _ := strconv.ParseInt(t.value, 10, 64)
		return &IntLiteral{Value: n}, nil
	}
	if t.kind == tokFloat {
		p.consume()
		f, _ := strconv.ParseFloat(t.value, 64)
		return &FloatLiteral{Value: f}, nil
	}

	// Negated number: -5
	if t.kind == tokMinus {
		p.consume()
		nt := p.peek()
		if nt.kind == tokInt {
			p.consume()
			n, _ := strconv.ParseInt(nt.value, 10, 64)
			return &IntLiteral{Value: -n}, nil
		}
		if nt.kind == tokFloat {
			p.consume()
			f, _ := strconv.ParseFloat(nt.value, 64)
			return &FloatLiteral{Value: -f}, nil
		}
		return nil, p.errorf(nt, "expected number after '-' but found %q", nt.value)
	}

	// Identifier: could be keyword (true/false/null), function call, property, or variable.
	if t.kind == tokIdent {
		upper := strings.ToUpper(t.value)
		switch upper {
		case "TRUE":
			p.consume()
			return &BoolLiteral{Value: true}, nil
		case "FALSE":
			p.consume()
			return &BoolLiteral{Value: false}, nil
		case "NULL":
			p.consume()
			return &NullLiteral{}, nil
		}

		// Function call: name(args…)
		if p.peekN(1).kind == tokLParen {
			return p.parseFunctionCall()
		}

		// Property access: variable.property (or chained: variable.a.b)
		if p.peekN(1).kind == tokDot {
			varName := p.consume().value
			var props []string
			for p.peek().kind == tokDot {
				p.consume() // consume dot
				propTok := p.peek()
				if propTok.kind != tokIdent {
					return nil, p.errorf(propTok, "expected property name after '.' but found %q at line %d column %d", propTok.value, propTok.line, propTok.col)
				}
				props = append(props, p.consume().value)
			}
			return &PropertyExpr{Variable: varName, Properties: props}, nil
		}

		// Plain variable reference.
		p.consume()
		return &VariableExpr{Name: t.value}, nil
	}

	return nil, p.errorf(t, "unexpected token %q at line %d column %d", t.value, t.line, t.col)
}

func (p *parser) parseBuiltinVariable() (*BuiltinVariable, error) {
	p.consume() // consume $
	t := p.peek()
	if t.kind != tokIdent {
		return nil, p.errorf(t, "expected built-in variable name after '$' but found %q at line %d column %d", t.value, t.line, t.col)
	}
	name := p.consume().value

	// Validate known built-in names.
	switch strings.ToLower(name) {
	case "today", "now", "week_start", "month_start":
	default:
		return nil, p.errorf(t, "unknown built-in variable $%s; expected today, now, week_start, or month_start", name)
	}

	bv := &BuiltinVariable{Name: strings.ToLower(name)}

	// Optional arithmetic offset: + 7d or - 30d
	if p.peek().kind == tokPlus || p.peek().kind == tokMinus {
		sign := p.consume().value
		nt := p.peek()
		if nt.kind != tokInt {
			return nil, p.errorf(nt, "expected integer after '%s' in date offset but found %q", sign, nt.value)
		}
		amountStr := p.consume().value
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			return nil, p.errorf(nt, "invalid offset amount %q", amountStr)
		}

		// Unit token directly attached to the number (no space) or separate ident.
		unitTok := p.peek()
		unit := ""
		if unitTok.kind == tokIdent {
			switch strings.ToLower(unitTok.value) {
			case "d", "w", "m", "y":
				unit = strings.ToLower(unitTok.value)
				p.consume()
			default:
				return nil, p.errorf(unitTok, "expected date offset unit (d, w, m, y) but found %q at line %d column %d", unitTok.value, unitTok.line, unitTok.col)
			}
		} else {
			return nil, p.errorf(unitTok, "expected date offset unit (d, w, m, y) after amount but found %q at line %d column %d", unitTok.value, unitTok.line, unitTok.col)
		}

		bv.Offset = &DateOffset{Sign: sign, Amount: amount, Unit: unit}
	}

	return bv, nil
}

func (p *parser) parseFunctionCall() (*FunctionCall, error) {
	name := strings.ToLower(p.consume().value)
	p.consume() // consume (

	var args []Expression
	if p.peek().kind != tokRParen {
		// Handle count(*) specially.
		if p.peek().kind == tokStar {
			p.consume()
			args = append(args, &VariableExpr{Name: "*"})
		} else {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			for p.peek().kind == tokComma {
				p.consume()
				arg, err = p.parseExpression()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
			}
		}
	}

	if p.peek().kind != tokRParen {
		t := p.peek()
		return nil, p.errorf(t, "expected ')' to close function call but found %q at line %d column %d", t.value, t.line, t.col)
	}
	p.consume()

	return &FunctionCall{Name: name, Args: args}, nil
}

// ---------------------------------------------------------------------------
// RETURN clause
// ---------------------------------------------------------------------------

func (p *parser) parseReturnClause() (*ReturnClause, error) {
	p.consume() // consume RETURN

	rc := &ReturnClause{}

	item, err := p.parseReturnItem()
	if err != nil {
		return nil, err
	}
	rc.Items = append(rc.Items, item)

	for p.peek().kind == tokComma {
		p.consume()
		item, err = p.parseReturnItem()
		if err != nil {
			return nil, err
		}
		rc.Items = append(rc.Items, item)
	}

	return rc, nil
}

func (p *parser) parseReturnItem() (*ReturnItem, error) {
	ri := &ReturnItem{}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	ri.Expr = expr

	// Optional AS alias.
	if p.keywordIs("AS") {
		p.consume()
		t := p.peek()
		if t.kind != tokIdent {
			return nil, p.errorf(t, "expected alias name after AS but found %q at line %d column %d", t.value, t.line, t.col)
		}
		ri.Alias = p.consume().value
	}

	return ri, nil
}

// ---------------------------------------------------------------------------
// ORDER BY clause
// ---------------------------------------------------------------------------

func (p *parser) parseOrderByClause() (*OrderByClause, error) {
	p.consume() // consume ORDER
	if err := p.expectIdent("BY"); err != nil {
		return nil, err
	}

	ob := &OrderByClause{}

	item, err := p.parseOrderByItem()
	if err != nil {
		return nil, err
	}
	ob.Items = append(ob.Items, item)

	for p.peek().kind == tokComma {
		p.consume()
		item, err = p.parseOrderByItem()
		if err != nil {
			return nil, err
		}
		ob.Items = append(ob.Items, item)
	}

	return ob, nil
}

func (p *parser) parseOrderByItem() (*OrderByItem, error) {
	oi := &OrderByItem{}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	oi.Expr = expr

	if p.keywordIs("DESC") {
		p.consume()
		oi.Descending = true
	} else if p.keywordIs("ASC") {
		p.consume()
	}

	return oi, nil
}

// ---------------------------------------------------------------------------
// LIMIT clause
// ---------------------------------------------------------------------------

func (p *parser) parseLimitClause() (*LimitClause, error) {
	p.consume() // consume LIMIT
	t := p.peek()
	if t.kind != tokInt {
		return nil, p.errorf(t, "expected integer after LIMIT but found %q at line %d column %d", t.value, t.line, t.col)
	}
	n, err := strconv.Atoi(p.consume().value)
	if err != nil {
		return nil, p.errorf(t, "invalid LIMIT value %q", t.value)
	}
	return &LimitClause{Count: n}, nil
}

// ---------------------------------------------------------------------------
// Reserved keyword detection
// ---------------------------------------------------------------------------

var reservedKeywords = map[string]bool{
	"MATCH": true, "WHERE": true, "RETURN": true, "ORDER": true, "BY": true,
	"LIMIT": true, "AND": true, "OR": true, "NOT": true, "AS": true,
	"TRUE": true, "FALSE": true, "NULL": true, "IS": true, "ASC": true, "DESC": true,
	"WITH": true, "UNWIND": true, "CASE": true, "OPTIONAL": true,
	"CREATE": true, "SET": true, "DELETE": true, "MERGE": true, "REMOVE": true,
}

func isReservedKeyword(s string) bool {
	return reservedKeywords[strings.ToUpper(s)]
}

// defaultMaxPathDepth is the ceiling for variable-length path traversal.
const defaultMaxPathDepth = 5

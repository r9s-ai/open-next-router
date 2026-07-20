package dslconfig

import (
	"math"
	"strconv"
	"strings"
)

// stmtWord is one whitespace-separated argument word of a statement.
// quoted marks words that came from a single string token; text holds the
// unquoted value for quoted words and the raw joined text otherwise.
type stmtWord struct {
	text   string
	quoted bool
	pos    int
}

// collectStmtWords reads tokens until the terminating ';' and joins adjacent
// non-trivia tokens that have no whitespace between them into words, so kv
// arguments like min=0.5 or -1 arrive as a single word even though the scanner
// splits them into several tokens. A quoted word must stand alone; mixing a
// string token with adjacent tokens in one word is a syntax error.
func collectStmtWords(s *scanner, directive string) ([]stmtWord, error) {
	var words []stmtWord
	var cur *stmtWord
	curQuoted := false
	for {
		tok := s.next()
		switch tok.kind {
		case tokEOF:
			return nil, s.errAt(tok, "unexpected EOF in "+directive)
		case tokSemicolon:
			return words, nil
		case tokWhitespace, tokComment:
			cur = nil
			continue
		case tokLBrace, tokRBrace:
			return nil, s.errAt(tok, directive+" does not use braces")
		}
		if cur != nil && (curQuoted || tok.kind == tokString) {
			return nil, s.errAt(tok, directive+" arguments must be separated by whitespace")
		}
		if cur == nil {
			words = append(words, stmtWord{pos: tok.pos})
			cur = &words[len(words)-1]
			curQuoted = false
		}
		if tok.kind == tokString {
			cur.text = unquoteString(tok.text)
			cur.quoted = true
			curQuoted = true
			continue
		}
		cur.text += tok.text
	}
}

// splitStmtKV splits a bare word of the form key=value. It returns ok=false
// for quoted words and words without '='.
func splitStmtKV(w stmtWord) (key, value string, ok bool) {
	if w.quoted {
		return "", "", false
	}
	key, value, ok = strings.Cut(w.text, "=")
	return key, value, ok
}

func parseReqValidationSourceAndTarget(s *scanner, directive string, words []stmtWord) (source, pathOrName string, rest []stmtWord, err error) {
	if len(words) < 2 {
		return "", "", nil, s.errAt(token{pos: s.lastPos}, directive+" expects: "+directive+" <body|header|query> <path-or-name> ...;")
	}
	source = strings.TrimSpace(words[0].text)
	pathOrName = strings.TrimSpace(words[1].text)
	if pathOrName == "" {
		return "", "", nil, s.errAt(token{pos: words[1].pos}, directive+" requires a non-empty path or name")
	}
	return source, pathOrName, words[2:], nil
}

func newReqValidationRule(op, source, pathOrName string) RequestValidationRule {
	rule := RequestValidationRule{Op: op, Source: source}
	if source == ReqValidationSourceBody {
		rule.Path = pathOrName
	} else {
		rule.Name = pathOrName
	}
	return rule
}

func parseReqRequiredStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_required"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	rule := newReqValidationRule(ReqRuleRequired, source, pathOrName)
	for _, w := range rest {
		key, value, ok := splitStmtKV(w)
		if !ok || key != "allow_null" {
			return s.errAt(token{pos: w.pos}, directive+" only accepts allow_null=true|false")
		}
		switch value {
		case "true":
			rule.AllowNull = true
		case "false":
			rule.AllowNull = false
		default:
			return s.errAt(token{pos: w.pos}, directive+" allow_null expects true or false")
		}
	}
	t.ValidationRules = append(t.ValidationRules, rule)
	return nil
}

func parseReqForbidStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_forbid"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return s.errAt(token{pos: rest[0].pos}, directive+" expects: "+directive+" <source> <path-or-name>;")
	}
	t.ValidationRules = append(t.ValidationRules, newReqValidationRule(ReqRuleForbid, source, pathOrName))
	return nil
}

func parseReqTypeStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_type"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return s.errAt(token{pos: s.lastPos}, directive+" expects: "+directive+" <source> <path-or-name> <type>;")
	}
	rule := newReqValidationRule(ReqRuleType, source, pathOrName)
	rule.Type = strings.TrimSpace(rest[0].text)
	t.ValidationRules = append(t.ValidationRules, rule)
	return nil
}

func parseReqRangeStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_range"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	rule := newReqValidationRule(ReqRuleRange, source, pathOrName)
	for _, w := range rest {
		key, value, ok := splitStmtKV(w)
		if !ok || (key != "min" && key != "max") {
			return s.errAt(token{pos: w.pos}, directive+" only accepts min=<number> and max=<number>")
		}
		f, err := parseReqRangeNumber(value)
		if err != nil {
			return s.errAt(token{pos: w.pos}, directive+" "+key+" expects a decimal number literal")
		}
		if key == "min" {
			rule.Min = &f
		} else {
			rule.Max = &f
		}
	}
	t.ValidationRules = append(t.ValidationRules, rule)
	return nil
}

func parseReqRangeNumber(value string) (float64, error) {
	if strings.ContainsAny(value, "eE") {
		return 0, strconv.ErrSyntax
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, strconv.ErrSyntax
	}
	return f, nil
}

func parseReqLenStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_len"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	rule := newReqValidationRule(ReqRuleLen, source, pathOrName)
	for _, w := range rest {
		key, value, ok := splitStmtKV(w)
		if !ok || (key != "min" && key != "max") {
			return s.errAt(token{pos: w.pos}, directive+" only accepts min=<int> and max=<int>")
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return s.errAt(token{pos: w.pos}, directive+" "+key+" expects an integer literal")
		}
		if key == "min" {
			rule.MinLen = &n
		} else {
			rule.MaxLen = &n
		}
	}
	t.ValidationRules = append(t.ValidationRules, rule)
	return nil
}

func parseReqEnumStmt(s *scanner, t *RequestTransform) error {
	const directive = "req_enum"
	words, err := collectStmtWords(s, directive)
	if err != nil {
		return err
	}
	source, pathOrName, rest, err := parseReqValidationSourceAndTarget(s, directive, words)
	if err != nil {
		return err
	}
	rule := newReqValidationRule(ReqRuleEnum, source, pathOrName)
	for _, w := range rest {
		// Quoted candidates keep canonical double quotes so config validation can
		// tell string literals from bare number/bool/null literals.
		if w.quoted {
			rule.Values = append(rule.Values, strconv.Quote(w.text))
			continue
		}
		rule.Values = append(rule.Values, w.text)
	}
	t.ValidationRules = append(t.ValidationRules, rule)
	return nil
}

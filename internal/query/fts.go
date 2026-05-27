package query

import (
	"fmt"
	"strings"
	"unicode"
)

// buildFTSQuery turns user input into an FTS5 query with prefix matching.
// It supports AND, OR, and parenthesized grouping.
func buildFTSQuery(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	ast, err := parseQuery(input)
	if err != nil {
		return "", fmt.Errorf("parse query: %w", err)
	}

	fts := emitFTS(ast)
	if fts == "" {
		return "", fmt.Errorf("query string has no searchable terms")
	}
	return fts, nil
}

func emitFTS(node queryNode) string {
	switch n := node.(type) {
	case *termNode:
		return ftsTerm(n.raw)
	case *andNode:
		return emitBool(" AND ", n.children)
	case *orNode:
		return emitBool(" OR ", n.children)
	default:
		return ""
	}
}

func emitBool(op string, children []queryNode) string {
	parts := make([]string, 0, len(children))
	for _, child := range children {
		if emitted := emitChild(child); emitted != "" {
			parts = append(parts, emitted)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, op)
}

func emitChild(node queryNode) string {
	switch n := node.(type) {
	case *termNode:
		return ftsTerm(n.raw)
	case *andNode:
		emitted := emitBool(" AND ", n.children)
		if emitted == "" {
			return ""
		}
		return "(" + emitted + ")"
	case *orNode:
		emitted := emitBool(" OR ", n.children)
		if emitted == "" {
			return ""
		}
		return "(" + emitted + ")"
	default:
		return ""
	}
}

func ftsTerm(term string) string {
	tokens := ftsTokens(term)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token+"*")
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

func ftsTokens(term string) []string {
	var tokens []string
	var b strings.Builder
	for _, r := range term {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

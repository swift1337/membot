package query

import (
	"strings"
	"unicode"
)

// buildFTSQuery turns user input into an FTS5 query with prefix matching.
func buildFTSQuery(input string) string {
	terms := strings.Fields(strings.TrimSpace(input))
	if len(terms) == 0 {
		return ""
	}

	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		if cleaned := ftsTerm(term); cleaned != "" {
			parts = append(parts, cleaned)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
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

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
			parts = append(parts, cleaned+"*")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}

func ftsTerm(term string) string {
	var b strings.Builder
	for _, r := range term {
		switch r {
		case '"':
			b.WriteString(`""`)
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

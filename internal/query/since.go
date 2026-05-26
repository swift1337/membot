package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/ijt/go-anytime"
)

// parseSince parses a human time expression into an RFC3339 cutoff string.
// An empty input disables the filter.
func parseSince(input string, now time.Time) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	if duration, ok := parseCompactDuration(input); ok {
		return now.Add(-duration).UTC().Format(time.RFC3339), nil
	}

	parsed, err := anytime.Parse(input, now)
	if err != nil {
		return "", fmt.Errorf("parse --since %q: %w", input, err)
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func parseCompactDuration(input string) (time.Duration, bool) {
	s := strings.TrimSpace(strings.ToLower(input))
	s = strings.TrimSuffix(s, " ago")

	replacements := []struct {
		from string
		to   string
	}{
		{"weeks", "w"},
		{"week", "w"},
		{"days", "d"},
		{"day", "d"},
		{"hours", "h"},
		{"hour", "h"},
		{"minutes", "m"},
		{"minute", "m"},
		{"seconds", "s"},
		{"second", "s"},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.from, r.to)
	}

	if d, err := time.ParseDuration(s); err == nil {
		return d, true
	}

	// Support forms like "1w" that Go's ParseDuration does not accept.
	if len(s) < 2 {
		return 0, false
	}
	unit := s[len(s)-1]
	value := s[:len(s)-1]
	multiplier := time.Duration(0)
	switch unit {
	case 'w':
		multiplier = 7 * 24 * time.Hour
	case 'd':
		multiplier = 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	case 's':
		multiplier = time.Second
	default:
		return 0, false
	}

	var amount int64
	if _, err := fmt.Sscanf(value, "%d", &amount); err != nil {
		return 0, false
	}
	return time.Duration(amount) * multiplier, true
}

package query

import (
	"fmt"
	"strings"
)

func normalizeAgent(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "cursor", "claude":
		return value, nil
	case "claude-code", "claudecode":
		return "claude", nil
	default:
		return "", fmt.Errorf("agent must be cursor or claude")
	}
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

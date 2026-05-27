package query

import (
	"fmt"
	"sort"
	"strings"
)

func parseOrderBy(input string) (OrderBy, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return OrderByScore, nil
	}

	switch OrderBy(input) {
	case OrderByScore, OrderByDateDesc, OrderByDateAsc:
		return OrderBy(input), nil
	default:
		return "", fmt.Errorf("invalid --order-by %q: want score, date-desc, or date-asc", input)
	}
}

func sortHits(hits []hit, orderBy OrderBy) {
	switch orderBy {
	case OrderByDateDesc:
		sortHitsByDate(hits, desc)
	case OrderByDateAsc:
		sortHitsByDate(hits, asc)
	default:
		sortHitsByScore(hits)
	}
}

type sortDirection int

const (
	desc sortDirection = iota
	asc
)

func sortHitsByScore(hits []hit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return compareCreatedAt(hits[i].createdAt, hits[j].createdAt, desc)
	})
}

func sortHitsByDate(hits []hit, direction sortDirection) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].createdAt != hits[j].createdAt {
			return compareCreatedAt(hits[i].createdAt, hits[j].createdAt, direction)
		}
		return hits[i].score > hits[j].score
	})
}

func compareCreatedAt(a, b string, direction sortDirection) bool {
	if a == b {
		return false
	}
	if a == "" {
		return false
	}
	if b == "" {
		return true
	}
	if direction == asc {
		return a < b
	}
	return a > b
}

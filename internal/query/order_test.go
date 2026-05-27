package query

import "testing"

func TestParseOrderBy(t *testing.T) {
	tests := []struct {
		input   string
		want    OrderBy
		wantErr bool
	}{
		{input: "", want: OrderByScore},
		{input: "score", want: OrderByScore},
		{input: "date-desc", want: OrderByDateDesc},
		{input: "date-asc", want: OrderByDateAsc},
		{input: " relevance ", wantErr: true},
	}

	for _, tt := range tests {
		got, err := parseOrderBy(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("parseOrderBy(%q) error = nil, want error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseOrderBy(%q) error = %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("parseOrderBy(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSortHitsByScore(t *testing.T) {
	hits := []hit{
		{score: 0.5, createdAt: "2026-01-01T00:00:00Z"},
		{score: 0.5, createdAt: "2026-02-01T00:00:00Z"},
		{score: 0.9, createdAt: "2026-01-01T00:00:00Z"},
	}

	sortHits(hits, OrderByScore)

	if hits[0].score != 0.9 {
		t.Fatalf("first hit score = %v, want 0.9", hits[0].score)
	}
	if hits[1].createdAt != "2026-02-01T00:00:00Z" {
		t.Fatalf("second hit date = %q, want newer tied score first", hits[1].createdAt)
	}
}

func TestSortHitsByDateDesc(t *testing.T) {
	hits := []hit{
		{score: 0.2, createdAt: "2026-01-01T00:00:00Z"},
		{score: 0.9, createdAt: "2026-02-01T00:00:00Z"},
		{score: 0.8, createdAt: "2026-02-01T00:00:00Z"},
	}

	sortHits(hits, OrderByDateDesc)

	if hits[0].createdAt != "2026-02-01T00:00:00Z" || hits[0].score != 0.9 {
		t.Fatalf("first hit = %+v, want newest date with highest score", hits[0])
	}
	if hits[1].createdAt != "2026-02-01T00:00:00Z" || hits[1].score != 0.8 {
		t.Fatalf("second hit = %+v, want same date tie broken by score", hits[1])
	}
}

func TestSortHitsByDateAsc(t *testing.T) {
	hits := []hit{
		{score: 0.2, createdAt: "2026-02-01T00:00:00Z"},
		{score: 0.9, createdAt: "2026-01-01T00:00:00Z"},
	}

	sortHits(hits, OrderByDateAsc)

	if hits[0].createdAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("first hit date = %q, want oldest first", hits[0].createdAt)
	}
}

func TestSortHitsEmptyCreatedAtLast(t *testing.T) {
	hits := []hit{
		{score: 0.9, createdAt: ""},
		{score: 0.1, createdAt: "2026-01-01T00:00:00Z"},
	}

	sortHits(hits, OrderByDateDesc)

	if hits[0].createdAt == "" {
		t.Fatal("empty created_at should sort after dated entries")
	}
}

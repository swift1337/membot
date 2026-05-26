package query

import (
	"testing"
	"time"
)

func TestParseSinceCompactDuration(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	got, err := parseSince("3h", now)
	if err != nil {
		t.Fatalf("parseSince() error = %v", err)
	}
	want := now.Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	if got != want {
		t.Fatalf("parseSince() = %q, want %q", got, want)
	}
}

func TestParseSinceYesterday(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	got, err := parseSince("yesterday", now)
	if err != nil {
		t.Fatalf("parseSince() error = %v", err)
	}
	if got == "" {
		t.Fatal("parseSince() returned empty cutoff")
	}
}

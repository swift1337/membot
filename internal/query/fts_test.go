package query

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	got := buildFTSQuery("query capabilities")
	if got != "query* OR capabilities*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryEmpty(t *testing.T) {
	if got := buildFTSQuery("   "); got != "" {
		t.Fatalf("buildFTSQuery() = %q, want empty", got)
	}
}

package query

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	got := buildFTSQuery("query capabilities")
	if got != "query* OR capabilities*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQuerySplitsHyphenatedIdentifiers(t *testing.T) {
	got := buildFTSQuery("attestor-sandbox")
	if got != "(attestor* AND sandbox*)" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQuerySplitsPaths(t *testing.T) {
	got := buildFTSQuery("ibc/localnet/docker-compose.yml")
	if got != "(ibc* AND localnet* AND docker* AND compose* AND yml*)" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryEmpty(t *testing.T) {
	if got := buildFTSQuery("   "); got != "" {
		t.Fatalf("buildFTSQuery() = %q, want empty", got)
	}
}

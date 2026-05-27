package query

import "testing"

func TestBuildFTSQuery(t *testing.T) {
	got, err := buildFTSQuery("query capabilities")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "query* AND capabilities*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQuerySplitsHyphenatedIdentifiers(t *testing.T) {
	got, err := buildFTSQuery("attestor-sandbox")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "(attestor* AND sandbox*)" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQuerySplitsPaths(t *testing.T) {
	got, err := buildFTSQuery("ibc/localnet/docker-compose.yml")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "(ibc* AND localnet* AND docker* AND compose* AND yml*)" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryEmpty(t *testing.T) {
	got, err := buildFTSQuery("   ")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "" {
		t.Fatalf("buildFTSQuery() = %q, want empty", got)
	}
}

func TestBuildFTSQueryOR(t *testing.T) {
	got, err := buildFTSQuery("foo OR bar")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "foo* OR bar*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryGroupedAND(t *testing.T) {
	got, err := buildFTSQuery("(foo OR bar) AND fizz")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "(foo* OR bar*) AND fizz*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryPrecedence(t *testing.T) {
	got, err := buildFTSQuery("foo OR bar AND baz")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "foo* OR (bar* AND baz*)" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryHyphenInGroup(t *testing.T) {
	got, err := buildFTSQuery("attestor-sandbox OR docker")
	if err != nil {
		t.Fatalf("buildFTSQuery() error = %v", err)
	}
	if got != "(attestor* AND sandbox*) OR docker*" {
		t.Fatalf("buildFTSQuery() = %q", got)
	}
}

func TestBuildFTSQueryParseError(t *testing.T) {
	if _, err := buildFTSQuery("(foo"); err == nil {
		t.Fatal("buildFTSQuery() = nil, want error")
	}
}

package query

import "testing"

func TestParseQueryImplicitAND(t *testing.T) {
	node, err := parseQuery("foo bar")
	if err != nil {
		t.Fatalf("parseQuery() error = %v", err)
	}
	and, ok := node.(*andNode)
	if !ok || len(and.children) != 2 {
		t.Fatalf("parseQuery() = %#v, want andNode with 2 children", node)
	}
}

func TestParseQueryExplicitOR(t *testing.T) {
	node, err := parseQuery("foo OR bar")
	if err != nil {
		t.Fatalf("parseQuery() error = %v", err)
	}
	or, ok := node.(*orNode)
	if !ok || len(or.children) != 2 {
		t.Fatalf("parseQuery() = %#v, want orNode with 2 children", node)
	}
}

func TestParseQueryGrouping(t *testing.T) {
	node, err := parseQuery("(foo OR bar) AND fizz")
	if err != nil {
		t.Fatalf("parseQuery() error = %v", err)
	}
	and, ok := node.(*andNode)
	if !ok || len(and.children) != 2 {
		t.Fatalf("parseQuery() = %#v, want andNode with 2 children", node)
	}
	or, ok := and.children[0].(*orNode)
	if !ok || len(or.children) != 2 {
		t.Fatalf("first child = %#v, want orNode", and.children[0])
	}
}

func TestParseQueryPrecedence(t *testing.T) {
	node, err := parseQuery("foo OR bar AND baz")
	if err != nil {
		t.Fatalf("parseQuery() error = %v", err)
	}
	or, ok := node.(*orNode)
	if !ok || len(or.children) != 2 {
		t.Fatalf("parseQuery() = %#v, want orNode", node)
	}
	if _, ok := or.children[1].(*andNode); !ok {
		t.Fatalf("second child = %#v, want andNode", or.children[1])
	}
}

func TestParseQueryLowercaseOperatorsAreTerms(t *testing.T) {
	node, err := parseQuery("and or")
	if err != nil {
		t.Fatalf("parseQuery() error = %v", err)
	}
	and, ok := node.(*andNode)
	if !ok || len(and.children) != 2 {
		t.Fatalf("parseQuery() = %#v, want andNode with term children", node)
	}
}

func TestParseQueryErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{input: "(foo"},
		{input: "foo)"},
		{input: "()"},
		{input: "AND foo"},
		{input: "foo OR"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if _, err := parseQuery(tc.input); err == nil {
				t.Fatalf("parseQuery(%q) = nil, want error", tc.input)
			}
		})
	}
}

package query

import "testing"

func TestNormalizeAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "cursor", input: "cursor", want: "cursor"},
		{name: "claude", input: "Claude", want: "claude"},
		{name: "claude code alias", input: "claude-code", want: "claude"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeAgent(test.input)
			if err != nil {
				t.Fatalf("normalizeAgent() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("normalizeAgent() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeAgentRejectsUnknown(t *testing.T) {
	t.Parallel()

	if _, err := normalizeAgent("codex"); err == nil {
		t.Fatal("normalizeAgent() error = nil, want error")
	}
}

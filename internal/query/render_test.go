package query

import (
	"encoding/json"
	"strings"
	"testing"

	generateddb "github.com/swift1337/membot/internal/db"
)

func TestRenderProjectsText(t *testing.T) {
	t.Parallel()

	got := RenderProjectsText([]generateddb.ListProjectSummariesRow{
		{
			ProjectSlug: "Users-me-membot",
			ProjectName: "membot",
			ProjectDir:  "/Users/example/membot",
			Chats:       2,
		},
	})

	for _, want := range []string{
		"1 project",
		"- membot (Users-me-membot)",
		"2 chats",
		"/Users/example/membot",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderProjectsText() = %q, want to contain %q", got, want)
		}
	}
}

func TestRenderProjectsTextEmpty(t *testing.T) {
	t.Parallel()

	if got := RenderProjectsText(nil); !strings.Contains(got, "No projects indexed.") {
		t.Fatalf("RenderProjectsText(nil) = %q", got)
	}
}

func TestToolCallArgumentsMarshalAsJSON(t *testing.T) {
	t.Parallel()

	resp := Response{
		ToolCalls: []ToolCall{
			{
				ToolName:  "ReadFile",
				Arguments: decodeToolArguments(`{"path":"/tmp/file.go","offset":2,"limit":5}`),
				MessageID: 1,
			},
		},
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal(Response) error = %v", err)
	}
	got := string(encoded)
	if strings.Contains(got, `"{\"path\"`) {
		t.Fatalf("encoded arguments are still double-encoded: %s", got)
	}
	for _, want := range []string{
		`"arguments":{"path":"/tmp/file.go","offset":2,"limit":5}`,
		`"tool_name":"ReadFile"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Marshal(Response) = %s, want to contain %s", got, want)
		}
	}
}

func TestToolCallArgumentsFallbackString(t *testing.T) {
	t.Parallel()

	if got := decodeToolArguments(`not-json`); got != `not-json` {
		t.Fatalf("decodeToolArguments() = %#v, want fallback string", got)
	}
}

package query

import (
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
			ProjectDir:  "/Users/me/membot",
			Chats:       2,
		},
	})

	for _, want := range []string{
		"1 project",
		"- membot (Users-me-membot)",
		"2 chats",
		"/Users/me/membot",
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

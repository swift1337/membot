package cursor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCursorTimestampFromInjectedTag(t *testing.T) {
	t.Parallel()

	got, ok := parseCursorTimestamp("Wednesday, Apr 29, 2026, 5:43 PM (UTC+2)")
	if !ok {
		t.Fatal("expected Cursor timestamp to parse")
	}
	const want = "2026-04-29T15:43:00Z"
	if got != want {
		t.Fatalf("parseCursorTimestamp() = %q, want %q", got, want)
	}
}

func TestParseLineExtractsTimestampFromTextBlock(t *testing.T) {
	t.Parallel()

	line, err := parseLine([]byte(`{"role":"user","message":{"content":[{"type":"text","text":"<timestamp>Tuesday, May 26, 2026, 10:57 PM (UTC+2)</timestamp>\n<user_query>hello</user_query>"}]}}`), 1)
	if err != nil {
		t.Fatalf("parseLine() error = %v", err)
	}
	const want = "2026-05-26T20:57:00Z"
	if line.CreatedAt != want {
		t.Fatalf("line.CreatedAt = %q, want %q", line.CreatedAt, want)
	}
}

func TestParseLineCleansCursorWrapperTags(t *testing.T) {
	t.Parallel()

	line, err := parseLine([]byte(`{"role":"user","message":{"content":[{"type":"text","text":"<timestamp>Tuesday, May 26, 2026, 10:57 PM (UTC+2)</timestamp>\n<user_query>\n@example-app/service/src/server.sh:47-61 Please use the public service endpoint here.\n</user_query>"}]}}`), 1)
	if err != nil {
		t.Fatalf("parseLine() error = %v", err)
	}
	const want = "@example-app/service/src/server.sh:47-61 Please use the public service endpoint here."
	if got := line.Text(); got != want {
		t.Fatalf("line.Text() = %q, want %q", got, want)
	}
	if got := line.Blocks[0].Text; got != want {
		t.Fatalf("block text = %q, want %q", got, want)
	}
}

func TestTitleFromLinesUsesCleanedUserQuery(t *testing.T) {
	t.Parallel()

	line, err := parseLine([]byte(`{"role":"user","message":{"content":[{"type":"text","text":"<timestamp>Thursday, May 21, 2026, 3:34 PM (UTC+2)</timestamp> <user_query>hello from cursor</user_query>"}]}}`), 1)
	if err != nil {
		t.Fatalf("parseLine() error = %v", err)
	}
	title := titleFromLines([]parsedLine{line})
	if !title.Valid || title.String != "hello from cursor" {
		t.Fatalf("titleFromLines() = %#v, want hello from cursor", title)
	}
}

func TestIndexCursorMissingRootIsOptional(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st, err := openTestStore(ctx, filepath.Join(t.TempDir(), "membot.db"))
	if err != nil {
		t.Fatalf("openTestStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	result, err := Index(ctx, st, Options{Root: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if result != (Result{}) {
		t.Fatalf("Index() result = %#v, want empty result", result)
	}
}

func TestConversationTimesFallsBackToSourceMtime(t *testing.T) {
	t.Parallel()

	startedAt, endedAt := conversationTimes([]parsedLine{{Role: "assistant"}}, "2026-04-29T15:43:00Z")
	const want = "2026-04-29T15:43:00Z"
	if startedAt != want {
		t.Fatalf("startedAt = %q, want %q", startedAt, want)
	}
	if endedAt != want {
		t.Fatalf("endedAt = %q, want %q", endedAt, want)
	}
}

func TestExtractApplyPatchFiles(t *testing.T) {
	t.Parallel()

	patch := `*** Begin Patch
*** Update File: /Users/example/project/foo.go
@@
+line
*** Add File: /Users/example/project/bar.go
+new
*** Delete File: /Users/example/project/old.go
*** End Patch`

	got := extractApplyPatchFiles(patch)
	want := []string{
		"/Users/example/project/foo.go",
		"/Users/example/project/bar.go",
		"/Users/example/project/old.go",
	}
	if len(got) != len(want) {
		t.Fatalf("extractApplyPatchFiles() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("file %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCompactToolArgumentsApplyPatch(t *testing.T) {
	t.Parallel()

	input := []byte(`"*** Begin Patch\n*** Update File: /tmp/a.go\n*** End Patch"`)
	got := compactToolArguments("ApplyPatch", input)
	const want = `{"files":["/tmp/a.go"]}`
	if got != want {
		t.Fatalf("compactToolArguments() = %q, want %q", got, want)
	}
}

func TestProjectsFromWorkspaceFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspaceDir := filepath.Join(root, "workspaces", "sample-workspace")
	appRepo := filepath.Join(root, "sample-app")
	libRepo := filepath.Join(root, "sample-lib")
	missingRepo := filepath.Join(root, "missing")
	for _, dir := range []string{workspaceDir, appRepo, libRepo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	workspacePath := filepath.Join(workspaceDir, "sample.code-workspace")
	workspaceJSON := `{
		"folders": [
			{"path": "../../sample-app"},
			{"path": "../../sample-lib", "name": "library"},
			{"path": "../../sample-lib"},
			{"path": "../../missing"},
			// VS Code accepts JSONC in workspace files.
		],
		"settings": {
			"example": "https://example.com/not-a-comment",
		}
	}`
	if err := os.WriteFile(workspacePath, []byte(workspaceJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", workspacePath, err)
	}

	got, err := projectsFromWorkspaceFile(workspacePath)
	if err != nil {
		t.Fatalf("projectsFromWorkspaceFile() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("projectsFromWorkspaceFile() len = %d, want 2: %#v", len(got), got)
	}

	want := []struct {
		path string
		name string
	}{
		{path: appRepo, name: "sample-app"},
		{path: libRepo, name: "library"},
	}
	for i := range want {
		if !got[i].CanonicalPath.Valid || got[i].CanonicalPath.String != want[i].path {
			t.Fatalf("project %d path = %#v, want %q", i, got[i].CanonicalPath, want[i].path)
		}
		if !got[i].Name.Valid || got[i].Name.String != want[i].name {
			t.Fatalf("project %d name = %#v, want %q", i, got[i].Name, want[i].name)
		}
		if got[i].Slug != pathSlug(want[i].path) {
			t.Fatalf("project %d slug = %q, want %q", i, got[i].Slug, pathSlug(want[i].path))
		}
	}
	if _, err := os.Stat(missingRepo); !os.IsNotExist(err) {
		t.Fatalf("missing fixture unexpectedly exists: %v", err)
	}
}

func TestExtractToolFileMentionsShell(t *testing.T) {
	t.Parallel()

	input := []byte(`{"command":"curl http://localhost:8080/health","working_directory":"/Users/example/project"}`)
	if got := extractToolFileMentions("Shell", input); len(got) != 0 {
		t.Fatalf("Shell should not produce file mentions, got %#v", got)
	}
}

func TestExtractToolFileMentionsReadFile(t *testing.T) {
	t.Parallel()

	input := []byte(`{"path":"/Users/example/project/main.go","offset":1,"limit":50}`)
	got := extractToolFileMentions("ReadFile", input)
	if len(got) != 1 || got[0].Path != "/Users/example/project/main.go" || got[0].Kind != "tool_read" {
		t.Fatalf("extractToolFileMentions() = %#v", got)
	}
}

func TestExtractFileMentions(t *testing.T) {
	t.Parallel()

	text := "Updated `service/config/app.yml` and @example-app/.cursor/refactor-plan.md.\n\n```247:265:/Users/example/Code/sample-app/service/src/server.sh\nstart_service\n```"
	got := extractFileMentions(text)

	want := []fileMention{
		{Path: "/Users/example/Code/sample-app/service/src/server.sh", Kind: "code_ref", LineStart: 247, LineEnd: 265},
		{Path: "service/config/app.yml", Kind: "inline_code"},
		{Path: "example-app/.cursor/refactor-plan.md", Kind: "at_ref"},
	}
	if len(got) != len(want) {
		t.Fatalf("extractFileMentions() len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Kind != want[i].Kind ||
			got[i].LineStart != want[i].LineStart || got[i].LineEnd != want[i].LineEnd {
			t.Fatalf("mention %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

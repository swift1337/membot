package cursor

import "testing"

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
*** Update File: /Users/me/proj/foo.go
@@
+line
*** Add File: /Users/me/proj/bar.go
+new
*** Delete File: /Users/me/proj/old.go
*** End Patch`

	got := extractApplyPatchFiles(patch)
	want := []string{
		"/Users/me/proj/foo.go",
		"/Users/me/proj/bar.go",
		"/Users/me/proj/old.go",
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

func TestExtractToolFileMentionsShell(t *testing.T) {
	t.Parallel()

	input := []byte(`{"command":"cast send 0xabc --rpc-url http://localhost:8545","working_directory":"/Users/me/evm"}`)
	if got := extractToolFileMentions("Shell", input); len(got) != 0 {
		t.Fatalf("Shell should not produce file mentions, got %#v", got)
	}
}

func TestExtractToolFileMentionsReadFile(t *testing.T) {
	t.Parallel()

	input := []byte(`{"path":"/Users/me/proj/main.go","offset":1,"limit":50}`)
	got := extractToolFileMentions("ReadFile", input)
	if len(got) != 1 || got[0].Path != "/Users/me/proj/main.go" || got[0].Kind != "tool_read" {
		t.Fatalf("extractToolFileMentions() = %#v", got)
	}
}

func TestExtractFileMentions(t *testing.T) {
	t.Parallel()

	text := "Updated `ibc/localnet/docker-compose.yml` and @sandbox/.cursor/evm-evm-localnet-refactor-plan.md.\n\n```247:265:/Users/dmitry/Code/sandbox/ibc/src/cmd_localnet.sh\nensure_attestor_keystore\n```"
	got := extractFileMentions(text)

	want := []fileMention{
		{Path: "/Users/dmitry/Code/sandbox/ibc/src/cmd_localnet.sh", Kind: "code_ref", LineStart: 247, LineEnd: 265},
		{Path: "ibc/localnet/docker-compose.yml", Kind: "inline_code"},
		{Path: "sandbox/.cursor/evm-evm-localnet-refactor-plan.md", Kind: "at_ref"},
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

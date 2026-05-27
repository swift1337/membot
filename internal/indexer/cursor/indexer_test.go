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

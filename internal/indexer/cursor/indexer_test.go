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

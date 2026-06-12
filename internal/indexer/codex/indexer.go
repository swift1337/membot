package codex

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

const (
	sourceKind    = "codex"
	parserVersion = "codex-jsonl-v1"
)

type Options struct {
	Root string
}

type Result struct {
	ProjectsDiscovered int `json:"projects_discovered"`
	ProjectsIndexed    int `json:"projects_indexed"`
	FilesDiscovered    int `json:"files_discovered"`
	FilesIndexed       int `json:"files_indexed"`
	FilesSkipped       int `json:"files_skipped"`
	MessagesIndexed    int `json:"messages_indexed"`
	ParseErrors        int `json:"parse_errors"`
}

type transcriptFile struct {
	Path string
}

type projectInfo struct {
	Slug          string
	CanonicalPath sql.NullString
	Name          sql.NullString
}

type conversationMeta struct {
	ExternalID string
	Title      sql.NullString
	Cwd        string
}

type transcriptLine struct {
	LineNumber int64
	Role       string
	CreatedAt  string
	Raw        []byte
	Blocks     []contentBlock
	ToolCall   *toolCall
}

func (l transcriptLine) Text() string {
	var parts []string
	for _, block := range l.Blocks {
		if block.Text != "" && block.Type != "function_call" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

type contentBlock struct {
	Type string
	Text string
	Raw  json.RawMessage
}

type toolCall struct {
	Name       string
	Args       json.RawMessage
	WorkingDir string
}

type parseError struct {
	Line  int64
	Raw   []byte
	Error string
}

type eventEnvelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	TS        string          `json:"ts"`
	Time      string          `json:"time"`
	Payload   json.RawMessage `json:"payload"`
	Item      json.RawMessage `json:"item"`
	Msg       json.RawMessage `json:"msg"`
	Raw       json.RawMessage `json:"-"`
}

// DefaultRoot returns the durable Codex transcript store. Desktop profile data
// under Application Support is runtime/cache state and is intentionally ignored.
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".codex")
	}
	return filepath.Join(home, ".codex")
}

func Index(ctx context.Context, st *store.Store, opts Options) (Result, error) {
	root := opts.Root
	if root == "" {
		root = DefaultRoot()
	}
	root, err := expandPath(root)
	if err != nil {
		return Result{}, err
	}

	source, err := st.Queries().UpsertSource(ctx, db.UpsertSourceParams{
		Kind:        sourceKind,
		RootPath:    root,
		DisplayName: sql.NullString{String: "Codex", Valid: true},
	})
	if err != nil {
		return Result{}, fmt.Errorf("upsert Codex source: %w", err)
	}

	transcripts, result, err := discover(root)
	if err != nil {
		return Result{}, err
	}
	stateMeta := readStateMetadata(root)
	indexMeta := readSessionIndexMetadata(root)

	now := time.Now().UTC().Format(time.RFC3339)
	indexedProjects := make(map[string]struct{})
	for _, transcript := range transcripts {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		indexed, messages, parseErrors, projectSlug, err := indexTranscript(
			ctx,
			st,
			source.ID,
			transcript,
			now,
			stateMeta,
			indexMeta,
		)
		if err != nil {
			return result, err
		}
		if projectSlug != "" {
			indexedProjects[projectSlug] = struct{}{}
		}
		if indexed {
			result.FilesIndexed++
			result.MessagesIndexed += messages
			result.ParseErrors += parseErrors
			continue
		}
		result.FilesSkipped++
	}
	result.ProjectsDiscovered = len(indexedProjects)
	result.ProjectsIndexed = len(indexedProjects)

	return result, nil
}

func discover(root string) ([]transcriptFile, Result, error) {
	sessionsRoot := filepath.Join(root, "sessions")
	var transcripts []transcriptFile
	err := filepath.WalkDir(sessionsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(entry.Name(), "rollout-") && strings.HasSuffix(entry.Name(), ".jsonl") {
			transcripts = append(transcripts, transcriptFile{Path: path})
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Result{}, nil
		}
		return nil, Result{}, fmt.Errorf("discover Codex sessions in %s: %w", sessionsRoot, err)
	}
	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].Path < transcripts[j].Path
	})
	return transcripts, Result{FilesDiscovered: len(transcripts)}, nil
}

func indexTranscript(
	ctx context.Context,
	st *store.Store,
	sourceID int64,
	transcript transcriptFile,
	indexedAt string,
	stateMeta map[string]conversationMeta,
	indexMeta map[string]conversationMeta,
) (bool, int, int, string, error) {
	info, err := os.Stat(transcript.Path)
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("stat transcript %s: %w", transcript.Path, err)
	}

	existing, err := st.Queries().GetSourceFileByPath(ctx, db.GetSourceFileByPathParams{
		SourceID: sourceID,
		Path:     transcript.Path,
	})
	if err == nil && sourceFileMetadataUnchanged(existing, info) {
		projectSlug := ""
		if existing.ProjectID.Valid {
			project, projectErr := st.Queries().GetProject(ctx, existing.ProjectID.Int64)
			if projectErr == nil {
				projectSlug = project.Slug
			}
		}
		return false, 0, 0, projectSlug, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, "", fmt.Errorf("read indexed source file %s: %w", transcript.Path, err)
	}

	hash, err := fileSHA256(transcript.Path)
	if err != nil {
		return false, 0, 0, "", err
	}
	if sourceFileUnchanged(existing, info, hash) {
		return false, 0, 0, "", nil
	}

	lines, parsedMeta, parseErrors, err := parseTranscriptFile(transcript.Path)
	if err != nil {
		return false, 0, 0, "", err
	}
	meta := mergeMetadata(transcript.Path, parsedMeta, stateMeta, indexMeta)
	project := projectFromCwd(meta.Cwd, transcript.Path)

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("begin Codex transcript transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	q := db.New(tx)

	dbProject, err := q.UpsertProject(ctx, db.UpsertProjectParams{
		CanonicalPath: project.CanonicalPath,
		Slug:          project.Slug,
		Name:          project.Name,
		FirstSeenAt:   sql.NullString{String: indexedAt, Valid: true},
		LastSeenAt:    sql.NullString{String: indexedAt, Valid: true},
	})
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("upsert project %s: %w", project.Slug, err)
	}

	sourceFile, err := q.UpsertSourceFile(ctx, db.UpsertSourceFileParams{
		SourceID:      sourceID,
		ProjectID:     sql.NullInt64{Int64: dbProject.ID, Valid: true},
		Path:          transcript.Path,
		Kind:          "transcript",
		SizeBytes:     sql.NullInt64{Int64: info.Size(), Valid: true},
		MtimeUnix:     sql.NullInt64{Int64: info.ModTime().Unix(), Valid: true},
		Sha256:        sql.NullString{String: hash, Valid: true},
		IndexedAt:     sql.NullString{String: indexedAt, Valid: true},
		ParserVersion: parserVersion,
	})
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("upsert source file %s: %w", transcript.Path, err)
	}

	fallbackCreatedAt := info.ModTime().UTC().Format(time.RFC3339)
	startedAt, endedAt := conversationTimes(lines, fallbackCreatedAt)
	if !meta.Title.Valid {
		meta.Title = titleFromLines(lines)
	}

	conversation, err := q.UpsertConversation(ctx, db.UpsertConversationParams{
		SourceID:     sourceID,
		ProjectID:    sql.NullInt64{Int64: dbProject.ID, Valid: true},
		SourceFileID: sql.NullInt64{Int64: sourceFile.ID, Valid: true},
		ExternalID:   meta.ExternalID,
		Title:        meta.Title,
		StartedAt:    sql.NullString{String: startedAt, Valid: startedAt != ""},
		EndedAt:      sql.NullString{String: endedAt, Valid: endedAt != ""},
		MessageCount: int64(len(lines)),
		RawPath:      transcript.Path,
	})
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("upsert conversation %s: %w", meta.ExternalID, err)
	}

	for _, parseErr := range parseErrors {
		if _, err := q.CreateParseError(ctx, db.CreateParseErrorParams{
			SourceFileID:  sql.NullInt64{Int64: sourceFile.ID, Valid: true},
			RawLine:       sql.NullInt64{Int64: parseErr.Line, Valid: true},
			ParserVersion: parserVersion,
			Error:         parseErr.Error,
			RawHash:       sql.NullString{String: hashString(parseErr.Raw), Valid: true},
		}); err != nil {
			return false, 0, 0, "", fmt.Errorf("record parse error in %s:%d: %w", transcript.Path, parseErr.Line, err)
		}
	}

	for seq, line := range lines {
		messageCreatedAt := lineCreatedAt(line, fallbackCreatedAt)
		message, err := q.UpsertMessage(ctx, db.UpsertMessageParams{
			ConversationID: conversation.ID,
			SourceFileID:   sql.NullInt64{Int64: sourceFile.ID, Valid: true},
			Role:           line.Role,
			Seq:            int64(seq + 1),
			CreatedAt:      sql.NullString{String: messageCreatedAt, Valid: true},
			Text:           sql.NullString{String: line.Text(), Valid: line.Text() != ""},
			RawJson:        string(line.Raw),
			RawLine:        sql.NullInt64{Int64: line.LineNumber, Valid: true},
			ContentHash:    hashString(line.Raw),
		})
		if err != nil {
			return false, 0, 0, "", fmt.Errorf("upsert message %s:%d: %w", transcript.Path, line.LineNumber, err)
		}
		if err := q.DeleteToolCallFileMentionsForMessage(ctx, message.ID); err != nil {
			return false, 0, 0, "", fmt.Errorf("delete tool call file mentions %s:%d: %w", transcript.Path, line.LineNumber, err)
		}
		if err := q.DeleteToolCallsForMessage(ctx, message.ID); err != nil {
			return false, 0, 0, "", fmt.Errorf("delete tool calls %s:%d: %w", transcript.Path, line.LineNumber, err)
		}

		for blockSeq, block := range line.Blocks {
			messageBlock, err := q.UpsertMessageBlock(ctx, db.UpsertMessageBlockParams{
				MessageID: message.ID,
				Seq:       int64(blockSeq + 1),
				Type:      blockType(block.Type),
				Text:      sql.NullString{String: block.Text, Valid: block.Text != ""},
				RawJson:   sql.NullString{String: string(block.Raw), Valid: len(block.Raw) > 0},
			})
			if err != nil {
				return false, 0, 0, "", fmt.Errorf("upsert message block %s:%d: %w", transcript.Path, line.LineNumber, err)
			}
			if line.ToolCall != nil && block.Type == "function_call" {
				if err := createToolCall(ctx, q, message.ID, messageBlock.ID, line.ToolCall, messageCreatedAt, meta.Cwd); err != nil {
					return false, 0, 0, "", fmt.Errorf("index tool call %s:%d: %w", transcript.Path, line.LineNumber, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return false, 0, 0, "", fmt.Errorf("commit Codex transcript %s: %w", transcript.Path, err)
	}

	return true, len(lines), len(parseErrors), project.Slug, nil
}

func parseTranscriptFile(path string) ([]transcriptLine, conversationMeta, []parseError, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, conversationMeta{}, nil, fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var lines []transcriptLine
	var parseErrors []parseError
	meta := conversationMeta{ExternalID: conversationIDFromPath(path)}
	var lineNumber int64
	for scanner.Scan() {
		lineNumber++
		raw := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}

		line, lineMeta, ok, err := parseLine(raw, lineNumber)
		if err != nil {
			parseErrors = append(parseErrors, parseError{Line: lineNumber, Raw: raw, Error: err.Error()})
			continue
		}
		meta = mergeSingleMeta(meta, lineMeta)
		if ok {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, conversationMeta{}, nil, fmt.Errorf("scan transcript %s: %w", path, err)
	}
	return lines, meta, parseErrors, nil
}

func parseLine(raw []byte, lineNumber int64) (transcriptLine, conversationMeta, bool, error) {
	var env eventEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return transcriptLine{}, conversationMeta{}, false, err
	}
	env.Raw = append([]byte(nil), raw...)

	eventType := strings.TrimSpace(env.Type)
	createdAt := parseTimestamp(firstNonEmpty(env.Timestamp, env.TS, env.Time))
	meta := metaFromPayload(env.Payload)
	if eventType == "session_meta" {
		return transcriptLine{}, meta, false, nil
	}
	if eventType == "turn_context" {
		return transcriptLine{}, meta, false, nil
	}
	if isRuntimeEvent(eventType) {
		return transcriptLine{}, meta, false, nil
	}

	if eventType == "event_msg" {
		payload := firstRaw(env.Msg, env.Payload)
		line, ok, err := parseEventMessage(payload, raw, lineNumber, createdAt)
		return line, meta, ok, err
	}

	if eventType == "response_item" {
		payload := firstRaw(env.Item, env.Payload)
		line, ok, err := parseResponseItem(payload, raw, lineNumber, createdAt)
		return line, meta, ok, err
	}

	return transcriptLine{}, meta, false, nil
}

func parseEventMessage(rawPayload json.RawMessage, rawLine []byte, lineNumber int64, createdAt string) (transcriptLine, bool, error) {
	if len(rawPayload) == 0 {
		return transcriptLine{}, false, nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return transcriptLine{}, false, err
	}
	msgType := jsonString(payload["type"])
	switch msgType {
	case "user_message":
		text := firstNonEmpty(jsonString(payload["message"]), jsonString(payload["text"]), jsonString(payload["content"]))
		if strings.TrimSpace(text) == "" {
			return transcriptLine{}, false, nil
		}
		return transcriptLine{
			LineNumber: lineNumber,
			Role:       "user",
			CreatedAt:  createdAt,
			Raw:        rawLine,
			Blocks:     []contentBlock{{Type: "text", Text: text, Raw: rawPayload}},
		}, true, nil
	case "agent_message":
		return transcriptLine{}, false, nil
	default:
		return transcriptLine{}, false, nil
	}
}

func parseResponseItem(rawItem json.RawMessage, rawLine []byte, lineNumber int64, createdAt string) (transcriptLine, bool, error) {
	if len(rawItem) == 0 {
		return transcriptLine{}, false, nil
	}
	var item map[string]json.RawMessage
	if err := json.Unmarshal(rawItem, &item); err != nil {
		return transcriptLine{}, false, err
	}

	itemType := jsonString(item["type"])
	if itemType == "message" {
		role := strings.ToLower(jsonString(item["role"]))
		if role == "developer" || role == "system" || role == "" {
			return transcriptLine{}, false, nil
		}
		blocks, err := parseMessageContent(item["content"])
		if err != nil {
			return transcriptLine{}, false, err
		}
		if len(blocks) == 0 {
			return transcriptLine{}, false, nil
		}
		return transcriptLine{LineNumber: lineNumber, Role: role, CreatedAt: createdAt, Raw: rawLine, Blocks: blocks}, true, nil
	}

	if itemType == "function_call" || itemType == "tool_call" {
		name := firstNonEmpty(jsonString(item["name"]), jsonString(item["tool_name"]))
		if name == "" {
			return transcriptLine{}, false, nil
		}
		args := firstRaw(item["arguments"], item["args"], item["input"])
		workingDir := firstNonEmpty(jsonString(item["working_directory"]), jsonString(item["cwd"]))
		text := "Tool call: " + name
		return transcriptLine{
			LineNumber: lineNumber,
			Role:       "assistant",
			CreatedAt:  createdAt,
			Raw:        rawLine,
			Blocks:     []contentBlock{{Type: "function_call", Text: text, Raw: rawItem}},
			ToolCall:   &toolCall{Name: name, Args: args, WorkingDir: workingDir},
		}, true, nil
	}

	return transcriptLine{}, false, nil
}

func parseMessageContent(raw json.RawMessage) ([]contentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []contentBlock{{Type: "text", Text: text, Raw: raw}}, nil
	}
	var rawBlocks []json.RawMessage
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return nil, err
	}
	blocks := make([]contentBlock, 0, len(rawBlocks))
	for _, rawBlock := range rawBlocks {
		var block map[string]json.RawMessage
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			return nil, err
		}
		blockType := firstNonEmpty(jsonString(block["type"]), "text")
		text := firstNonEmpty(jsonString(block["text"]), jsonString(block["content"]))
		if strings.TrimSpace(text) == "" {
			continue
		}
		blocks = append(blocks, contentBlock{Type: blockType, Text: text, Raw: rawBlock})
	}
	return blocks, nil
}

func metaFromPayload(raw json.RawMessage) conversationMeta {
	var meta conversationMeta
	if len(raw) == 0 {
		return meta
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return meta
	}
	meta.Cwd = firstNonEmpty(jsonString(payload["cwd"]), jsonString(payload["working_directory"]))
	meta.ExternalID = firstNonEmpty(jsonString(payload["thread_id"]), jsonString(payload["session_id"]), jsonString(payload["id"]))
	title := firstNonEmpty(jsonString(payload["title"]), jsonString(payload["thread_name"]), jsonString(payload["name"]))
	if title != "" {
		meta.Title = sql.NullString{String: title, Valid: true}
	}
	return meta
}

func mergeMetadata(
	path string,
	parsed conversationMeta,
	stateMeta map[string]conversationMeta,
	indexMeta map[string]conversationMeta,
) conversationMeta {
	id := firstNonEmpty(parsed.ExternalID, conversationIDFromPath(path))
	meta := conversationMeta{ExternalID: id}
	if state, ok := stateMeta[id]; ok {
		meta = mergeSingleMeta(meta, state)
	}
	if indexed, ok := indexMeta[id]; ok {
		meta = mergeSingleMeta(meta, indexed)
	}
	meta = mergeSingleMeta(meta, parsed)
	if meta.ExternalID == "" {
		meta.ExternalID = conversationIDFromPath(path)
	}
	return meta
}

func mergeSingleMeta(dst, src conversationMeta) conversationMeta {
	if dst.ExternalID == "" && src.ExternalID != "" {
		dst.ExternalID = src.ExternalID
	}
	if !dst.Title.Valid && src.Title.Valid {
		dst.Title = src.Title
	}
	if dst.Cwd == "" && src.Cwd != "" {
		dst.Cwd = src.Cwd
	}
	return dst
}

func readStateMetadata(root string) map[string]conversationMeta {
	path := filepath.Join(root, "state_5.sqlite")
	conn, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil
	}
	defer func() {
		_ = conn.Close()
	}()
	rows, err := conn.Query(`SELECT id, title, cwd FROM threads`)
	if err != nil {
		return nil
	}
	defer func() {
		_ = rows.Close()
	}()
	metadata := make(map[string]conversationMeta)
	for rows.Next() {
		var id string
		var title, cwd sql.NullString
		if err := rows.Scan(&id, &title, &cwd); err != nil {
			continue
		}
		metadata[id] = conversationMeta{ExternalID: id, Title: title, Cwd: cwd.String}
	}
	return metadata
}

func readSessionIndexMetadata(root string) map[string]conversationMeta {
	path := filepath.Join(root, "session_index.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() {
		_ = file.Close()
	}()

	metadata := make(map[string]conversationMeta)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var row map[string]json.RawMessage
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		id := firstNonEmpty(jsonString(row["thread_id"]), jsonString(row["session_id"]), jsonString(row["id"]))
		if id == "" {
			continue
		}
		title := firstNonEmpty(jsonString(row["thread_name"]), jsonString(row["title"]), jsonString(row["name"]))
		meta := conversationMeta{
			ExternalID: id,
			Cwd:        firstNonEmpty(jsonString(row["cwd"]), jsonString(row["working_directory"])),
		}
		if title != "" {
			meta.Title = sql.NullString{String: title, Valid: true}
		}
		metadata[id] = meta
	}
	return metadata
}

func projectFromCwd(cwd string, fallbackPath string) projectInfo {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		cwd = filepath.Dir(fallbackPath)
	}
	slug := pathSlug(cwd)
	name := filepath.Base(filepath.Clean(cwd))
	if name == "." || name == string(filepath.Separator) {
		name = slug
	}
	return projectInfo{
		Slug:          slug,
		CanonicalPath: sql.NullString{String: cwd, Valid: cwd != ""},
		Name:          sql.NullString{String: name, Valid: name != ""},
	}
}

func createToolCall(
	ctx context.Context,
	q *db.Queries,
	messageID int64,
	blockID int64,
	call *toolCall,
	createdAt string,
	fallbackCwd string,
) error {
	if call == nil || call.Name == "" {
		return nil
	}
	args := ""
	if len(call.Args) > 0 {
		args = string(call.Args)
	}
	workingDir := call.WorkingDir
	if workingDir == "" {
		workingDir = fallbackCwd
	}
	_, err := q.CreateToolCall(ctx, db.CreateToolCallParams{
		MessageID:        messageID,
		BlockID:          sql.NullInt64{Int64: blockID, Valid: true},
		ToolName:         call.Name,
		ArgumentsJson:    sql.NullString{String: args, Valid: args != ""},
		WorkingDirectory: sql.NullString{String: workingDir, Valid: workingDir != ""},
		Status:           sql.NullString{String: "requested", Valid: true},
		CreatedAt:        sql.NullString{String: createdAt, Valid: createdAt != ""},
	})
	if err != nil {
		return fmt.Errorf("create tool call %s: %w", call.Name, err)
	}
	return nil
}

func conversationIDFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !strings.HasPrefix(base, "rollout-") {
		return base
	}
	rest := strings.TrimPrefix(base, "rollout-")
	if idx := strings.LastIndex(rest, "Z-"); idx >= 0 && idx+2 < len(rest) {
		return rest[idx+2:]
	}
	return rest
}

func isRuntimeEvent(eventType string) bool {
	switch eventType {
	case "", "token_count", "turn_context", "session_meta", "reasoning", "encrypted_reasoning", "app_event", "telemetry", "sentry":
		return true
	default:
		return false
	}
}

func conversationTimes(lines []transcriptLine, fallback string) (string, string) {
	startedAt := ""
	endedAt := ""
	for _, line := range lines {
		createdAt := lineCreatedAt(line, fallback)
		if createdAt == "" {
			continue
		}
		if startedAt == "" {
			startedAt = createdAt
		}
		endedAt = createdAt
	}
	if startedAt == "" {
		startedAt = fallback
	}
	if endedAt == "" {
		endedAt = startedAt
	}
	return startedAt, endedAt
}

func lineCreatedAt(line transcriptLine, fallback string) string {
	if line.CreatedAt != "" {
		return line.CreatedAt
	}
	return fallback
}

func titleFromLines(lines []transcriptLine) sql.NullString {
	for _, line := range lines {
		if line.Role != "user" {
			continue
		}
		text := strings.TrimSpace(line.Text())
		if text == "" {
			continue
		}
		text = strings.ReplaceAll(text, "\n", " ")
		if len(text) > 120 {
			text = text[:120]
		}
		return sql.NullString{String: text, Valid: true}
	}
	return sql.NullString{}
}

func parseTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

func firstRaw(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) > 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return value
	}
	return ""
}

func pathSlug(path string) string {
	trimmed := strings.TrimPrefix(filepath.Clean(path), string(filepath.Separator))
	slug := strings.ReplaceAll(trimmed, string(filepath.Separator), "-")
	if slug == "" || slug == "." {
		return "unknown"
	}
	return slug
}

func blockType(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func sourceFileMetadataUnchanged(file db.SourceFile, info os.FileInfo) bool {
	return file.SizeBytes.Valid &&
		file.SizeBytes.Int64 == info.Size() &&
		file.MtimeUnix.Valid &&
		file.MtimeUnix.Int64 == info.ModTime().Unix() &&
		file.ParserVersion == parserVersion
}

func sourceFileUnchanged(file db.SourceFile, info os.FileInfo, hash string) bool {
	return file.SizeBytes.Valid &&
		file.SizeBytes.Int64 == info.Size() &&
		file.MtimeUnix.Valid &&
		file.MtimeUnix.Int64 == info.ModTime().Unix() &&
		file.Sha256.Valid &&
		file.Sha256.String == hash &&
		file.ParserVersion == parserVersion
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s for hashing: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashString(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}

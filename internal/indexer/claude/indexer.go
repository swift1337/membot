package claude

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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

const (
	sourceKind    = "claude"
	parserVersion = "claude-jsonl-v1"
)

var (
	applyPatchFilePattern = regexp.MustCompile(`(?m)^\*\*\* (?:Update|Add|Delete) File: (.+)$`)
	codeRefPattern        = regexp.MustCompile("(?m)^```(\\d+):(\\d+):([^\\n`]+)")
	inlineCodePattern     = regexp.MustCompile("`([^`\\n]+)`")
	atPathPattern         = regexp.MustCompile(`@([A-Za-z0-9_./~:-]+)`)
	lineRangePattern      = regexp.MustCompile(`^\d+:\d+:`)
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

type projectDir struct {
	Path          string
	Slug          string
	CanonicalPath sql.NullString
	Name          sql.NullString
}

type transcriptFile struct {
	Project projectDir
	Path    string
}

type transcriptLine struct {
	Type      string            `json:"type"`
	Parent    *string           `json:"parentUuid"`
	Sidechain bool              `json:"isSidechain"`
	AgentID   string            `json:"agentId"`
	Message   transcriptMessage `json:"message"`
	UUID      string            `json:"uuid"`
	Timestamp string            `json:"timestamp"`
	Cwd       string            `json:"cwd"`
	SessionID string            `json:"sessionId"`
	Raw       json.RawMessage   `json:"-"`
}

type transcriptMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Raw     json.RawMessage `json:"-"`
}

type parsedLine struct {
	LineNumber int64
	Role       string
	CreatedAt  string
	Raw        []byte
	Cwd        string
	Blocks     []contentBlock
}

func (l parsedLine) Text() string {
	var parts []string
	for _, block := range l.Blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

type parseError struct {
	Line  int64
	Raw   []byte
	Error string
}

type fileMention struct {
	Path      string
	Kind      string
	LineStart int64
	LineEnd   int64
	Snippet   string
}

// DefaultRoot returns the Claude Code data directory, which contains
// project-scoped JSONL transcripts under projects/.
func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".claude")
	}
	return filepath.Join(home, ".claude")
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
		DisplayName: sql.NullString{String: "Claude Code", Valid: true},
	})
	if err != nil {
		return Result{}, fmt.Errorf("upsert Claude source: %w", err)
	}

	projects, transcripts, result, err := discover(root)
	if err != nil {
		return Result{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	indexedProjects := make(map[string]struct{})
	for _, project := range projects {
		if err := upsertDiscoveredProject(ctx, st, project, now); err != nil {
			return result, err
		}
		indexedProjects[project.Slug] = struct{}{}
	}

	for _, transcript := range transcripts {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		indexed, messages, parseErrors, err := indexTranscript(ctx, st, source.ID, transcript, now)
		if err != nil {
			return result, err
		}
		if indexed {
			indexedProjects[transcript.Project.Slug] = struct{}{}
			result.FilesIndexed++
			result.MessagesIndexed += messages
			result.ParseErrors += parseErrors
			continue
		}
		result.FilesSkipped++
	}
	result.ProjectsIndexed = len(indexedProjects)

	return result, nil
}

func discover(root string) ([]projectDir, []transcriptFile, Result, error) {
	projectsRoot := filepath.Join(root, "projects")
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, Result{}, nil
		}
		return nil, nil, Result{}, fmt.Errorf("read Claude projects root %s: %w", projectsRoot, err)
	}

	projects := make([]projectDir, 0, len(entries))
	var transcripts []transcriptFile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project := newProjectDir(filepath.Join(projectsRoot, entry.Name()), entry.Name())
		projects = append(projects, project)
		err := filepath.WalkDir(project.Path, func(path string, dirEntry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if dirEntry.IsDir() {
				return nil
			}
			if filepath.Ext(dirEntry.Name()) != ".jsonl" {
				return nil
			}
			transcripts = append(transcripts, transcriptFile{Project: project, Path: path})
			return nil
		})
		if err != nil {
			return nil, nil, Result{}, fmt.Errorf("discover transcripts in %s: %w", project.Path, err)
		}
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Slug < projects[j].Slug
	})
	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].Path < transcripts[j].Path
	})

	return projects, transcripts, Result{
		ProjectsDiscovered: len(projects),
		FilesDiscovered:    len(transcripts),
	}, nil
}

func newProjectDir(path string, encodedName string) projectDir {
	canonical := inferCanonicalPath(encodedName)
	slug := strings.TrimPrefix(encodedName, "-")
	if slug == "" {
		slug = encodedName
	}
	name := slug
	if canonical.Valid {
		name = filepath.Base(canonical.String)
	}
	return projectDir{
		Path:          path,
		Slug:          slug,
		CanonicalPath: canonical,
		Name:          sql.NullString{String: name, Valid: name != ""},
	}
}

func inferCanonicalPath(encodedName string) sql.NullString {
	trimmed := strings.TrimPrefix(encodedName, "-")
	if strings.HasPrefix(trimmed, "Users-") || strings.HasPrefix(trimmed, "Volumes-") {
		return sql.NullString{
			String: string(filepath.Separator) + strings.ReplaceAll(trimmed, "-", string(filepath.Separator)),
			Valid:  true,
		}
	}
	return sql.NullString{}
}

func upsertDiscoveredProject(ctx context.Context, st *store.Store, project projectDir, indexedAt string) error {
	_, err := st.Queries().UpsertProject(ctx, db.UpsertProjectParams{
		CanonicalPath: project.CanonicalPath,
		Slug:          project.Slug,
		Name:          project.Name,
		FirstSeenAt:   sql.NullString{String: indexedAt, Valid: true},
		LastSeenAt:    sql.NullString{String: indexedAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("upsert discovered project %s: %w", project.Slug, err)
	}
	return nil
}

func indexTranscript(
	ctx context.Context,
	st *store.Store,
	sourceID int64,
	transcript transcriptFile,
	indexedAt string,
) (bool, int, int, error) {
	info, err := os.Stat(transcript.Path)
	if err != nil {
		return false, 0, 0, fmt.Errorf("stat transcript %s: %w", transcript.Path, err)
	}

	existing, err := st.Queries().GetSourceFileByPath(ctx, db.GetSourceFileByPathParams{
		SourceID: sourceID,
		Path:     transcript.Path,
	})
	if err == nil && sourceFileMetadataUnchanged(existing, info) {
		return false, 0, 0, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, fmt.Errorf("read indexed source file %s: %w", transcript.Path, err)
	}

	hash, err := fileSHA256(transcript.Path)
	if err != nil {
		return false, 0, 0, err
	}
	if sourceFileUnchanged(existing, info, hash) {
		return false, 0, 0, nil
	}

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, fmt.Errorf("begin Claude transcript transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	q := db.New(tx)

	project, err := q.UpsertProject(ctx, db.UpsertProjectParams{
		CanonicalPath: transcript.Project.CanonicalPath,
		Slug:          transcript.Project.Slug,
		Name:          transcript.Project.Name,
		FirstSeenAt:   sql.NullString{String: indexedAt, Valid: true},
		LastSeenAt:    sql.NullString{String: indexedAt, Valid: true},
	})
	if err != nil {
		return false, 0, 0, fmt.Errorf("upsert project %s: %w", transcript.Project.Slug, err)
	}

	sourceFile, err := q.UpsertSourceFile(ctx, db.UpsertSourceFileParams{
		SourceID:      sourceID,
		ProjectID:     sql.NullInt64{Int64: project.ID, Valid: true},
		Path:          transcript.Path,
		Kind:          "transcript",
		SizeBytes:     sql.NullInt64{Int64: info.Size(), Valid: true},
		MtimeUnix:     sql.NullInt64{Int64: info.ModTime().Unix(), Valid: true},
		Sha256:        sql.NullString{String: hash, Valid: true},
		IndexedAt:     sql.NullString{String: indexedAt, Valid: true},
		ParserVersion: parserVersion,
	})
	if err != nil {
		return false, 0, 0, fmt.Errorf("upsert source file %s: %w", transcript.Path, err)
	}

	lines, parseErrors, err := parseTranscriptFile(transcript.Path)
	if err != nil {
		return false, 0, 0, err
	}
	fallbackCreatedAt := info.ModTime().UTC().Format(time.RFC3339)
	startedAt, endedAt := conversationTimes(lines, fallbackCreatedAt)
	conversationID := externalID(transcript.Path)

	conversation, err := q.UpsertConversation(ctx, db.UpsertConversationParams{
		SourceID:         sourceID,
		ProjectID:        sql.NullInt64{Int64: project.ID, Valid: true},
		SourceFileID:     sql.NullInt64{Int64: sourceFile.ID, Valid: true},
		ExternalID:       conversationID,
		ParentExternalID: parentExternalID(transcript.Path),
		Title:            titleFromLines(lines),
		StartedAt:        sql.NullString{String: startedAt, Valid: startedAt != ""},
		EndedAt:          sql.NullString{String: endedAt, Valid: endedAt != ""},
		MessageCount:     int64(len(lines)),
		IsSubagent:       boolInt64(isSubagentPath(transcript.Path)),
		RawPath:          transcript.Path,
	})
	if err != nil {
		return false, 0, 0, fmt.Errorf("upsert conversation %s: %w", conversationID, err)
	}

	for _, parseErr := range parseErrors {
		if _, err := q.CreateParseError(ctx, db.CreateParseErrorParams{
			SourceFileID:  sql.NullInt64{Int64: sourceFile.ID, Valid: true},
			RawLine:       sql.NullInt64{Int64: parseErr.Line, Valid: true},
			ParserVersion: parserVersion,
			Error:         parseErr.Error,
			RawHash:       sql.NullString{String: hashString(parseErr.Raw), Valid: true},
		}); err != nil {
			return false, 0, 0, fmt.Errorf("record parse error in %s:%d: %w", transcript.Path, parseErr.Line, err)
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
			return false, 0, 0, fmt.Errorf("upsert message %s:%d: %w", transcript.Path, line.LineNumber, err)
		}

		if err := q.DeleteToolCallFileMentionsForMessage(ctx, message.ID); err != nil {
			return false, 0, 0, fmt.Errorf("delete tool call file mentions %s:%d: %w", transcript.Path, line.LineNumber, err)
		}
		if err := q.DeleteToolCallsForMessage(ctx, message.ID); err != nil {
			return false, 0, 0, fmt.Errorf("delete tool calls %s:%d: %w", transcript.Path, line.LineNumber, err)
		}
		if err := indexFileMentions(ctx, q, project.ID, message.ID, line.Text()); err != nil {
			return false, 0, 0, fmt.Errorf("index file mentions %s:%d: %w", transcript.Path, line.LineNumber, err)
		}

		for blockSeq, block := range line.Blocks {
			rawBlock := string(block.Raw)
			messageBlock, err := q.UpsertMessageBlock(ctx, db.UpsertMessageBlockParams{
				MessageID: message.ID,
				Seq:       int64(blockSeq + 1),
				Type:      blockType(block.Type),
				Text:      sql.NullString{String: block.Text, Valid: block.Text != ""},
				RawJson:   sql.NullString{String: rawBlock, Valid: rawBlock != ""},
			})
			if err != nil {
				return false, 0, 0, fmt.Errorf("upsert message block %s:%d: %w", transcript.Path, line.LineNumber, err)
			}

			if block.Type == "tool_use" && len(block.Raw) > 0 {
				if err := indexToolCall(ctx, q, project.ID, message.ID, messageBlock.ID, block.Raw, messageCreatedAt, line.Cwd); err != nil {
					return false, 0, 0, fmt.Errorf("index tool call %s:%d: %w", transcript.Path, line.LineNumber, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return false, 0, 0, fmt.Errorf("commit Claude transcript %s: %w", transcript.Path, err)
	}

	return true, len(lines), len(parseErrors), nil
}

func parseTranscriptFile(path string) ([]parsedLine, []parseError, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open transcript %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	var lines []parsedLine
	var parseErrors []parseError
	var lineNumber int64
	for scanner.Scan() {
		lineNumber++
		raw := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(raw))) == 0 {
			continue
		}

		parsed, ok, err := parseLine(raw, lineNumber)
		if err != nil {
			parseErrors = append(parseErrors, parseError{
				Line:  lineNumber,
				Raw:   raw,
				Error: err.Error(),
			})
			continue
		}
		if ok {
			lines = append(lines, parsed)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan transcript %s: %w", path, err)
	}

	return lines, parseErrors, nil
}

func parseLine(raw []byte, lineNumber int64) (parsedLine, bool, error) {
	var line transcriptLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return parsedLine{}, false, err
	}
	line.Raw = append([]byte(nil), raw...)

	if line.Message.Role == "" && line.Type != "user" && line.Type != "assistant" {
		return parsedLine{}, false, nil
	}
	blocks, err := parseContent(line.Message.Content)
	if err != nil {
		return parsedLine{}, false, err
	}

	role := line.Message.Role
	if role == "" {
		role = line.Type
	}
	if role == "" {
		role = "unknown"
	}
	return parsedLine{
		LineNumber: lineNumber,
		Role:       role,
		CreatedAt:  parseClaudeTimestamp(line.Timestamp),
		Raw:        line.Raw,
		Cwd:        line.Cwd,
		Blocks:     blocks,
	}, true, nil
}

func parseContent(raw json.RawMessage) ([]contentBlock, error) {
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
		var block contentBlock
		if err := json.Unmarshal(rawBlock, &block); err != nil {
			return nil, err
		}
		block.Raw = append([]byte(nil), rawBlock...)
		if block.Text == "" && len(block.Content) > 0 {
			block.Text = contentText(block.Content)
		}
		if block.Text == "" && block.Type == "tool_use" && block.Name != "" {
			block.Text = block.Name
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func contentText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var blocks []contentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n\n")
}

func parseClaudeTimestamp(value string) string {
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

func conversationTimes(lines []parsedLine, fallback string) (string, string) {
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

func lineCreatedAt(line parsedLine, fallback string) string {
	if line.CreatedAt != "" {
		return line.CreatedAt
	}
	return fallback
}

func indexFileMentions(
	ctx context.Context,
	q *db.Queries,
	projectID int64,
	messageID int64,
	text string,
) error {
	if err := q.DeleteFileMentionsForMessage(ctx, sql.NullInt64{Int64: messageID, Valid: true}); err != nil {
		return err
	}

	mentions := extractFileMentions(text)
	for _, mention := range mentions {
		file, err := q.UpsertFile(ctx, db.UpsertFileParams{
			ProjectID:      sql.NullInt64{Int64: projectID, Valid: true},
			Path:           mention.Path,
			NormalizedPath: sql.NullString{String: normalizedFilePath(mention.Path), Valid: true},
			Kind:           sql.NullString{String: "mentioned", Valid: true},
			Basename:       fileBasename(mention.Path),
		})
		if err != nil {
			return err
		}

		if _, err := q.CreateFileMention(ctx, db.CreateFileMentionParams{
			FileID:      file.ID,
			MessageID:   sql.NullInt64{Int64: messageID, Valid: true},
			MentionKind: mention.Kind,
			LineStart:   nullInt64(mention.LineStart),
			LineEnd:     nullInt64(mention.LineEnd),
			Snippet:     sql.NullString{String: mention.Snippet, Valid: mention.Snippet != ""},
		}); err != nil {
			return err
		}
	}

	return nil
}

func extractFileMentions(text string) []fileMention {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	seen := make(map[string]struct{})
	mentions := make([]fileMention, 0)
	add := func(candidate, kind string, lineStart, lineEnd int64) {
		path := cleanMentionPath(candidate)
		if path == "" {
			return
		}
		key := normalizedFilePath(path)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		mentions = append(mentions, fileMention{
			Path:      path,
			Kind:      kind,
			LineStart: lineStart,
			LineEnd:   lineEnd,
			Snippet:   truncateMentionSnippet(text),
		})
	}

	for _, match := range codeRefPattern.FindAllStringSubmatch(text, -1) {
		add(match[3], "code_ref", parseInt64(match[1]), parseInt64(match[2]))
	}
	for _, match := range inlineCodePattern.FindAllStringSubmatch(text, -1) {
		add(match[1], "inline_code", 0, 0)
	}
	for _, match := range atPathPattern.FindAllStringSubmatch(text, -1) {
		add(match[1], "at_ref", 0, 0)
	}

	return mentions
}

func indexToolCall(
	ctx context.Context,
	q *db.Queries,
	projectID int64,
	messageID int64,
	blockID int64,
	rawBlock []byte,
	createdAt string,
	cwd string,
) error {
	var payload contentBlock
	if err := json.Unmarshal(rawBlock, &payload); err != nil {
		return fmt.Errorf("parse tool_use block: %w", err)
	}
	if payload.Name == "" {
		return nil
	}

	argsJSON := compactToolArguments(payload.Name, payload.Input)
	toolCall, err := q.CreateToolCall(ctx, db.CreateToolCallParams{
		MessageID:        messageID,
		BlockID:          sql.NullInt64{Int64: blockID, Valid: true},
		ToolName:         payload.Name,
		ArgumentsJson:    sql.NullString{String: argsJSON, Valid: argsJSON != ""},
		WorkingDirectory: toolWorkingDirectory(payload.Name, payload.Input, cwd),
		Status:           sql.NullString{String: "requested", Valid: true},
		CreatedAt:        sql.NullString{String: createdAt, Valid: createdAt != ""},
	})
	if err != nil {
		return fmt.Errorf("create tool call %s: %w", payload.Name, err)
	}

	for _, mention := range extractToolFileMentions(payload.Name, payload.Input) {
		file, err := q.UpsertFile(ctx, db.UpsertFileParams{
			ProjectID:      sql.NullInt64{Int64: projectID, Valid: true},
			Path:           mention.Path,
			NormalizedPath: sql.NullString{String: normalizedFilePath(mention.Path), Valid: true},
			Kind:           sql.NullString{String: "tool", Valid: true},
			Basename:       fileBasename(mention.Path),
		})
		if err != nil {
			return err
		}

		if _, err := q.CreateFileMention(ctx, db.CreateFileMentionParams{
			FileID:      file.ID,
			MessageID:   sql.NullInt64{Int64: messageID, Valid: true},
			ToolCallID:  sql.NullInt64{Int64: toolCall.ID, Valid: true},
			MentionKind: mention.Kind,
			LineStart:   nullInt64(mention.LineStart),
			LineEnd:     nullInt64(mention.LineEnd),
			Snippet:     sql.NullString{String: mention.Snippet, Valid: mention.Snippet != ""},
		}); err != nil {
			return err
		}
	}

	return nil
}

type pathInput struct {
	Path            string `json:"path"`
	FilePath        string `json:"file_path"`
	GlobPattern     string `json:"glob_pattern"`
	TargetDirectory string `json:"target_directory"`
}

type bashInput struct {
	Command          string `json:"command"`
	WorkingDirectory string `json:"working_directory"`
}

func compactToolArguments(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	if toolName == "ApplyPatch" {
		var patch string
		if err := json.Unmarshal(input, &patch); err == nil {
			encoded, err := json.Marshal(map[string][]string{"files": extractApplyPatchFiles(patch)})
			if err == nil {
				return string(encoded)
			}
		}
	}
	return string(input)
}

func toolWorkingDirectory(toolName string, input json.RawMessage, fallback string) sql.NullString {
	if toolName != "Bash" {
		return sql.NullString{}
	}
	var in bashInput
	if err := json.Unmarshal(input, &in); err == nil && in.WorkingDirectory != "" {
		return sql.NullString{String: in.WorkingDirectory, Valid: true}
	}
	if fallback != "" {
		return sql.NullString{String: fallback, Valid: true}
	}
	return sql.NullString{}
}

func extractToolFileMentions(toolName string, input json.RawMessage) []fileMention {
	switch toolName {
	case "Read", "Write", "Edit", "MultiEdit", "NotebookEdit":
		return pathMentions(input, toolFileKind(toolName))
	case "Glob", "Grep":
		return searchPathMentions(input)
	case "ApplyPatch":
		var patch string
		if err := json.Unmarshal(input, &patch); err != nil {
			return nil
		}
		return applyPatchMentions(patch)
	default:
		return nil
	}
}

func toolFileKind(toolName string) string {
	switch toolName {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return "tool_write"
	default:
		return "tool_read"
	}
}

func pathMentions(input json.RawMessage, kind string) []fileMention {
	var in pathInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil
	}
	path := in.FilePath
	if path == "" {
		path = in.Path
	}
	path = cleanMentionPath(path)
	if path == "" {
		return nil
	}
	return []fileMention{{Path: path, Kind: kind}}
}

func searchPathMentions(input json.RawMessage) []fileMention {
	var in pathInput
	if err := json.Unmarshal(input, &in); err != nil {
		return nil
	}
	if path := cleanMentionPath(in.Path); path != "" {
		return []fileMention{{Path: path, Kind: "tool_read"}}
	}
	if path := cleanMentionPath(in.TargetDirectory); path != "" {
		return []fileMention{{Path: path, Kind: "tool_read"}}
	}
	return nil
}

func applyPatchMentions(patch string) []fileMention {
	paths := extractApplyPatchFiles(patch)
	mentions := make([]fileMention, 0, len(paths))
	for _, path := range paths {
		mentions = append(mentions, fileMention{Path: path, Kind: "tool_patch"})
	}
	return mentions
}

func extractApplyPatchFiles(patch string) []string {
	seen := make(map[string]struct{})
	var paths []string
	for _, match := range applyPatchFilePattern.FindAllStringSubmatch(patch, -1) {
		path := cleanMentionPath(strings.TrimSpace(match[1]))
		if path == "" {
			continue
		}
		key := normalizedFilePath(path)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func cleanMentionPath(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	candidate = strings.Trim(candidate, "`'\".,;:)]}>")
	candidate = strings.TrimPrefix(candidate, "@")
	candidate = lineRangePattern.ReplaceAllString(candidate, "")
	if candidate == "" ||
		strings.Contains(candidate, "://") ||
		strings.ContainsAny(candidate, " \t\n\r") ||
		!strings.Contains(candidate, "/") {
		return ""
	}
	cleaned := normalizedFilePath(candidate)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return ""
	}
	return cleaned
}

func normalizedFilePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func fileBasename(path string) sql.NullString {
	base := filepath.Base(normalizedFilePath(path))
	if base == "" || base == "." || base == "/" {
		return sql.NullString{}
	}
	return sql.NullString{String: base, Valid: true}
}

func truncateMentionSnippet(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	const maxSnippetLength = 240
	if len(text) <= maxSnippetLength {
		return text
	}
	return text[:maxSnippetLength-3] + "..."
}

func parseInt64(value string) int64 {
	var parsed int64
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		parsed = parsed*10 + int64(r-'0')
	}
	return parsed
}

func nullInt64(value int64) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
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

func externalID(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if base != "" {
		return base
	}
	return path
}

func parentExternalID(path string) sql.NullString {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i, part := range parts {
		if part == "subagents" && i > 0 {
			return sql.NullString{String: parts[i-1], Valid: true}
		}
	}
	return sql.NullString{}
}

func isSubagentPath(path string) bool {
	return strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator))
}

func titleFromLines(lines []parsedLine) sql.NullString {
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

func blockType(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
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

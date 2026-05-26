package cursor

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
	sourceKind    = "cursor"
	parserVersion = "cursor-jsonl-v1"
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
	Role    string          `json:"role"`
	Message transcriptMsg   `json:"message"`
	Raw     json.RawMessage `json:"-"`
}

type transcriptMsg struct {
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type string          `json:"type"`
	Text string          `json:"text"`
	Raw  json.RawMessage `json:"-"`
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".cursor", "projects")
	}
	return filepath.Join(home, ".cursor", "projects")
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
		DisplayName: sql.NullString{String: "Cursor projects", Valid: true},
	})
	if err != nil {
		return Result{}, fmt.Errorf("upsert Cursor source: %w", err)
	}

	transcripts, result, err := discover(root)
	if err != nil {
		return Result{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	indexedProjects := make(map[string]struct{})
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

func discover(root string) ([]transcriptFile, Result, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, Result{}, fmt.Errorf("read Cursor projects root %s: %w", root, err)
	}

	var projects []projectDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectPath := filepath.Join(root, entry.Name())
		if !looksLikeProjectDir(projectPath, entry.Name()) {
			continue
		}
		projects = append(projects, newProjectDir(projectPath, entry.Name()))
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Slug < projects[j].Slug
	})

	var transcripts []transcriptFile
	for _, project := range projects {
		err := filepath.WalkDir(filepath.Join(project.Path, "agent-transcripts"), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(entry.Name()) != ".jsonl" {
				return nil
			}
			transcripts = append(transcripts, transcriptFile{Project: project, Path: path})
			return nil
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, Result{}, fmt.Errorf("discover transcripts in %s: %w", project.Path, err)
		}
	}
	sort.Slice(transcripts, func(i, j int) bool {
		return transcripts[i].Path < transcripts[j].Path
	})

	return transcripts, Result{
		ProjectsDiscovered: len(projects),
		FilesDiscovered:    len(transcripts),
	}, nil
}

func looksLikeProjectDir(path, name string) bool {
	if _, err := os.Stat(filepath.Join(path, "agent-transcripts")); err == nil {
		return true
	}
	return strings.HasPrefix(name, "Users-") || strings.HasPrefix(name, "Volumes-")
}

func newProjectDir(path, slug string) projectDir {
	canonical := inferCanonicalPath(slug)
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

func inferCanonicalPath(slug string) sql.NullString {
	prefixes := []string{"Users-", "Volumes-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(slug, prefix) {
			return sql.NullString{
				String: string(filepath.Separator) + strings.ReplaceAll(slug, "-", string(filepath.Separator)),
				Valid:  true,
			}
		}
	}
	return sql.NullString{}
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
	hash, err := fileSHA256(transcript.Path)
	if err != nil {
		return false, 0, 0, err
	}

	existing, err := st.Queries().GetSourceFileByPath(ctx, db.GetSourceFileByPathParams{
		SourceID: sourceID,
		Path:     transcript.Path,
	})
	if err == nil && sourceFileUnchanged(existing, info, hash) {
		return false, 0, 0, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, 0, 0, fmt.Errorf("read indexed source file %s: %w", transcript.Path, err)
	}

	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return false, 0, 0, fmt.Errorf("begin Cursor transcript transaction: %w", err)
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

	conversationID := externalID(transcript.Path)
	lines, parseErrors, err := parseTranscriptFile(transcript.Path)
	if err != nil {
		return false, 0, 0, err
	}

	conversation, err := q.UpsertConversation(ctx, db.UpsertConversationParams{
		SourceID:         sourceID,
		ProjectID:        sql.NullInt64{Int64: project.ID, Valid: true},
		SourceFileID:     sql.NullInt64{Int64: sourceFile.ID, Valid: true},
		ExternalID:       conversationID,
		ParentExternalID: parentExternalID(transcript.Path),
		Title:            titleFromLines(lines),
		MessageCount:     int64(len(lines)),
		IsSubagent:       boolInt64(strings.Contains(transcript.Path, string(filepath.Separator)+"subagents"+string(filepath.Separator))),
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
		message, err := q.UpsertMessage(ctx, db.UpsertMessageParams{
			ConversationID: conversation.ID,
			SourceFileID:   sql.NullInt64{Int64: sourceFile.ID, Valid: true},
			Role:           line.Role,
			Seq:            int64(seq + 1),
			Text:           sql.NullString{String: line.Text(), Valid: line.Text() != ""},
			RawJson:        string(line.Raw),
			RawLine:        sql.NullInt64{Int64: line.LineNumber, Valid: true},
			ContentHash:    hashString(line.Raw),
		})
		if err != nil {
			return false, 0, 0, fmt.Errorf("upsert message %s:%d: %w", transcript.Path, line.LineNumber, err)
		}

		for blockSeq, block := range line.Blocks {
			rawBlock := string(block.Raw)
			if _, err := q.UpsertMessageBlock(ctx, db.UpsertMessageBlockParams{
				MessageID: message.ID,
				Seq:       int64(blockSeq + 1),
				Type:      blockType(block.Type),
				Text:      sql.NullString{String: block.Text, Valid: block.Text != ""},
				RawJson:   sql.NullString{String: rawBlock, Valid: rawBlock != ""},
			}); err != nil {
				return false, 0, 0, fmt.Errorf("upsert message block %s:%d: %w", transcript.Path, line.LineNumber, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return false, 0, 0, fmt.Errorf("commit Cursor transcript %s: %w", transcript.Path, err)
	}

	return true, len(lines), len(parseErrors), nil
}

type parsedLine struct {
	LineNumber int64
	Role       string
	Raw        []byte
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

		parsed, err := parseLine(raw, lineNumber)
		if err != nil {
			parseErrors = append(parseErrors, parseError{
				Line:  lineNumber,
				Raw:   raw,
				Error: err.Error(),
			})
			continue
		}
		lines = append(lines, parsed)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan transcript %s: %w", path, err)
	}

	return lines, parseErrors, nil
}

func parseLine(raw []byte, lineNumber int64) (parsedLine, error) {
	var line transcriptLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return parsedLine{}, err
	}
	line.Raw = append([]byte(nil), raw...)

	blocks, err := parseContent(line.Message.Content)
	if err != nil {
		return parsedLine{}, err
	}

	role := line.Role
	if role == "" {
		role = "unknown"
	}
	return parsedLine{
		LineNumber: lineNumber,
		Role:       role,
		Raw:        line.Raw,
		Blocks:     blocks,
	}, nil
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
		blocks = append(blocks, block)
	}
	return blocks, nil
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
		if part == "agent-transcripts" && i+1 < len(parts) {
			parent := parts[i+1]
			if parent != externalID(path) {
				return sql.NullString{String: parent, Valid: true}
			}
		}
	}
	return sql.NullString{}
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

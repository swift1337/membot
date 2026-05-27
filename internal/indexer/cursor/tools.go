package cursor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/swift1337/membot/internal/db"
)

var applyPatchFilePattern = regexp.MustCompile(`(?m)^\*\*\* (?:Update|Add|Delete) File: (.+)$`)

type toolUsePayload struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type pathInput struct {
	Path            string `json:"path"`
	GlobPattern     string `json:"glob_pattern"`
	TargetDirectory string `json:"target_directory"`
}

type shellInput struct {
	Command          string `json:"command"`
	WorkingDirectory string `json:"working_directory"`
	Description      string `json:"description"`
}

func indexToolCall(
	ctx context.Context,
	q *db.Queries,
	projectID int64,
	messageID int64,
	blockID int64,
	rawBlock []byte,
	createdAt string,
) error {
	var payload toolUsePayload
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
		WorkingDirectory: toolWorkingDirectory(payload.Name, payload.Input),
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

		lineStart := sql.NullInt64{}
		if mention.LineStart > 0 {
			lineStart = sql.NullInt64{Int64: mention.LineStart, Valid: true}
		}
		lineEnd := sql.NullInt64{}
		if mention.LineEnd > 0 {
			lineEnd = sql.NullInt64{Int64: mention.LineEnd, Valid: true}
		}
		if _, err := q.CreateFileMention(ctx, db.CreateFileMentionParams{
			FileID:      file.ID,
			MessageID:   sql.NullInt64{Int64: messageID, Valid: true},
			ToolCallID:  sql.NullInt64{Int64: toolCall.ID, Valid: true},
			MentionKind: mention.Kind,
			LineStart:   lineStart,
			LineEnd:     lineEnd,
			Snippet:     sql.NullString{String: mention.Snippet, Valid: mention.Snippet != ""},
		}); err != nil {
			return err
		}
	}

	return nil
}

func compactToolArguments(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	if toolName == "ApplyPatch" {
		var patch string
		if err := json.Unmarshal(input, &patch); err == nil {
			files := extractApplyPatchFiles(patch)
			encoded, err := json.Marshal(map[string][]string{"files": files})
			if err == nil {
				return string(encoded)
			}
		}
	}
	return string(input)
}

func toolWorkingDirectory(toolName string, input json.RawMessage) sql.NullString {
	if toolName != "Shell" || len(input) == 0 {
		return sql.NullString{}
	}
	var in shellInput
	if err := json.Unmarshal(input, &in); err != nil || in.WorkingDirectory == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: in.WorkingDirectory, Valid: true}
}

func extractToolFileMentions(toolName string, input json.RawMessage) []fileMention {
	switch toolName {
	case "Read", "ReadFile", "Write", "Delete", "StrReplace", "ReadLints":
		return pathMentions(input, toolFileKind(toolName))
	case "Glob", "Grep", "rg":
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
	case "Write", "Delete", "StrReplace":
		return "tool_write"
	default:
		return "tool_read"
	}
}

func pathMentions(input json.RawMessage, kind string) []fileMention {
	var in pathInput
	if err := json.Unmarshal(input, &in); err != nil || in.Path == "" {
		return nil
	}
	path := cleanMentionPath(in.Path)
	if path == "" {
		return nil
	}
	return []fileMention{{
		Path: path,
		Kind: kind,
	}}
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
		mentions = append(mentions, fileMention{
			Path: path,
			Kind: "tool_patch",
		})
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

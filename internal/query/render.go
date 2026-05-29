package query

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	generateddb "github.com/swift1337/membot/internal/db"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// RenderFileContextText formats a file context search response for terminal output.
func RenderFileContextText(resp FileContextResponse) string {
	if len(resp.Result) == 0 {
		label := resp.Query
		if resp.Project != "" {
			label = fmt.Sprintf("%s in %s", resp.Query, resp.Project)
		}
		return mutedStyle.Render(fmt.Sprintf("No conversations found for %s.", label))
	}

	var b strings.Builder
	heading := fmt.Sprintf("%d conversations for %s", len(resp.Result), resp.Query)
	if resp.Project != "" {
		heading = fmt.Sprintf("%s in %s", heading, resp.Project)
	}
	b.WriteString(headerStyle.Render(heading))
	b.WriteString("\n\n")

	var lastPath string
	for i, item := range resp.Result {
		if item.Path != lastPath {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(headerStyle.Render(item.Path))
			b.WriteString("\n")
			lastPath = item.Path
		}

		kinds := ""
		if len(item.MentionKinds) > 0 {
			kinds = fmt.Sprintf(" (%d mentions: %s)", item.Mentions, strings.Join(item.MentionKinds, ", "))
		} else if item.Mentions > 0 {
			kinds = fmt.Sprintf(" (%d mentions)", item.Mentions)
		}

		line := fmt.Sprintf("- %s", item.ProjectName)
		if item.LastMentionedAt != "" {
			line = fmt.Sprintf("%s %s", line, mutedStyle.Render("["+relativeTime(item.LastMentionedAt)+"]"))
		}
		title := item.ConversationTitle
		if title == "" {
			title = fmt.Sprintf("conv:%d", item.ConversationID)
		} else {
			title = truncate(title, 48)
		}
		fmt.Fprintf(&b, "%s conv:%d %s%s\n", line, item.ConversationID, title, kinds)
		if item.Snippet != "" {
			fmt.Fprintf(&b, "  %s\n", formatSnippet(item.Snippet))
		}
	}

	return b.String()
}

// RenderProjectsText formats project summaries for terminal output.
func RenderProjectsText(projects []generateddb.ListProjectSummariesRow) string {
	if len(projects) == 0 {
		return mutedStyle.Render("No projects indexed.")
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(pluralize(len(projects), "project")))
	b.WriteString("\n\n")

	for _, project := range projects {
		label := project.ProjectName
		if label == "" {
			label = project.ProjectSlug
		}
		if project.ProjectSlug != "" && project.ProjectSlug != label {
			label = fmt.Sprintf("%s (%s)", label, project.ProjectSlug)
		}

		fmt.Fprintf(&b, "- %s %s", label, mutedStyle.Render("["+pluralize(int(project.Chats), "chat")+"]"))
		if project.ProjectDir != "" {
			fmt.Fprintf(&b, "\n  %s", project.ProjectDir)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderText formats a search response for terminal output.
func RenderText(resp Response) string {
	if len(resp.Result) == 0 {
		return mutedStyle.Render("No results.")
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%d results", len(resp.Result))))
	b.WriteString("\n\n")

	for i, item := range resp.Result {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(resultHeading(item))
		b.WriteString("\n")
		b.WriteString(formatSnippet(item.Snippet))
	}

	if len(resp.RelatedFiles) > 0 {
		b.WriteString("\n\n")
		b.WriteString(headerStyle.Render("Related files"))
		b.WriteString("\n\n")
		for _, file := range resp.RelatedFiles {
			fmt.Fprintf(&b, "- %s (%s, %d mentions)\n", file.Path, file.ProjectName, file.Mentions)
		}
	}

	if len(resp.ToolCalls) > 0 {
		b.WriteString("\n\n")
		b.WriteString(headerStyle.Render("Tool calls"))
		b.WriteString("\n\n")
		for _, call := range resp.ToolCalls {
			status := call.Status
			if status == "" {
				status = "unknown"
			}
			fmt.Fprintf(&b, "- %s [%s] message:%d", call.ToolName, status, call.MessageID)
			if call.WorkingDirectory != "" {
				fmt.Fprintf(&b, " wd:%s", call.WorkingDirectory)
			}
			if call.CreatedAt != "" {
				fmt.Fprintf(&b, " %s", mutedStyle.Render("["+relativeTime(call.CreatedAt)+"]"))
			}
			b.WriteString("\n")
			if args := formatToolArguments(call); args != "" {
				fmt.Fprintf(&b, "  %s\n", mutedStyle.Render(args))
			}
		}
	}

	return b.String()
}

func resultHeading(item ResultItem) string {
	project := item.ProjectName
	if project == "" {
		project = "unknown"
	}

	heading := project
	if item.ProjectDir != "" {
		heading = fmt.Sprintf("%s(%s)", project, item.ProjectDir)
	}
	if item.CreatedAt != "" {
		heading = fmt.Sprintf("%s %s", heading, mutedStyle.Render("["+relativeTime(item.CreatedAt)+"]"))
	}

	ref := resultRef(item)
	if ref != "" {
		heading = fmt.Sprintf("%s %s", heading, mutedStyle.Render(ref))
	}
	return headerStyle.Render(heading)
}

func resultRef(item ResultItem) string {
	switch item.Type {
	case "message":
		if item.ConversationTitle != "" {
			return fmt.Sprintf("msg:%d %s", item.MessageID, truncate(item.ConversationTitle, 24))
		}
		return fmt.Sprintf("msg:%d", item.MessageID)
	default:
		return item.Type
	}
}

func truncate(value string, max int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func formatSnippet(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return mutedStyle.Render("(empty snippet)")
	}
	return value
}

func relativeTime(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}

	duration := time.Since(parsed)
	if duration < 0 {
		duration = -duration
	}

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration / time.Minute)
		return pluralize(minutes, "minute") + " ago"
	case duration < 24*time.Hour:
		hours := int(duration / time.Hour)
		return pluralize(hours, "hour") + " ago"
	case duration < 7*24*time.Hour:
		days := int(duration / (24 * time.Hour))
		return pluralize(days, "day") + " ago"
	case duration < 30*24*time.Hour:
		weeks := int(duration / (7 * 24 * time.Hour))
		return pluralize(weeks, "week") + " ago"
	case duration < 365*24*time.Hour:
		months := int(duration / (30 * 24 * time.Hour))
		return pluralize(months, "month") + " ago"
	default:
		years := int(duration / (365 * 24 * time.Hour))
		return pluralize(years, "year") + " ago"
	}
}

func pluralize(value int, unit string) string {
	if value == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}

func formatToolArguments(call ToolCall) string {
	args := strings.TrimSpace(toolArgumentsString(call.Arguments))
	if args == "" {
		return ""
	}
	const maxLen = 240
	if len(args) <= maxLen {
		return args
	}
	return args[:maxLen-3] + "..."
}

func toolArgumentsString(arguments any) string {
	switch typed := arguments.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

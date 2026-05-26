package query

import (
	"context"
	"fmt"
	"strings"

	generateddb "github.com/swift1337/membot/internal/db"
)

// resolveProject fuzzy-matches a project filter against slug, name, and path.
func resolveProject(ctx context.Context, q *generateddb.Queries, name string) (generateddb.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return generateddb.Project{}, fmt.Errorf("project name is empty")
	}

	projects, err := q.ListProjects(ctx)
	if err != nil {
		return generateddb.Project{}, err
	}
	if len(projects) == 0 {
		return generateddb.Project{}, fmt.Errorf("project not found: %q", name)
	}

	needle := strings.ToLower(name)
	type candidate struct {
		project generateddb.Project
		score   int
	}
	var matches []candidate

	for _, project := range projects {
		score := scoreProject(project, needle)
		if score > 0 {
			matches = append(matches, candidate{project: project, score: score})
		}
	}
	if len(matches) == 0 {
		return generateddb.Project{}, fmt.Errorf("project not found: %q", name)
	}

	best := matches[0]
	for _, match := range matches[1:] {
		if match.score > best.score {
			best = match
		}
	}

	var closeSeconds []string
	for _, match := range matches {
		if match.score >= best.score-50 {
			label := projectLabel(match.project)
			closeSeconds = append(closeSeconds, label)
		}
	}
	if len(closeSeconds) > 1 {
		return generateddb.Project{}, fmt.Errorf("ambiguous project %q: matches %s", name, strings.Join(closeSeconds, ", "))
	}

	return best.project, nil
}

func scoreProject(project generateddb.Project, needle string) int {
	score := 0
	slug := strings.ToLower(project.Slug)
	if slug == needle {
		score += 1000
	} else if strings.Contains(slug, needle) {
		score += 500
	}

	if project.Name.Valid {
		name := strings.ToLower(project.Name.String)
		if name == needle {
			score += 900
		} else if strings.Contains(name, needle) {
			score += 400
		}
	}

	if project.CanonicalPath.Valid {
		path := strings.ToLower(project.CanonicalPath.String)
		if strings.Contains(path, needle) {
			score += 300
		}
	}
	if project.GitRoot.Valid {
		path := strings.ToLower(project.GitRoot.String)
		if strings.Contains(path, needle) {
			score += 250
		}
	}

	return score
}

func projectLabel(project generateddb.Project) string {
	if project.Name.Valid && project.Name.String != "" {
		return project.Name.String
	}
	return project.Slug
}

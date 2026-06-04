package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/query"
)

var cmdQueryProjects = &cobra.Command{
	Use:   "projects",
	Short: "List indexed projects",
	RunE:  runQueryProjects,
}

var cmdQuery = &cobra.Command{
	Use:     "query [query string]",
	Aliases: []string{"q"},
	Short:   "Query indexed memory as JSON",
	Long: `Search indexed assistant conversation messages.

Query syntax supports AND, OR, and parentheses. Adjacent terms are AND'd.
Use single quotes in the shell when the query contains parentheses.

Examples:
  membot query 'migration sqlc'
  membot query '(foo OR bar) AND fizz' --project membot --text`,
	RunE: runQuery,
}

var cmdQueryFiles = &cobra.Command{
	Use:     "files <filename-or-path>",
	Aliases: []string{"f"},
	Short:   "Find conversations that referenced a file by name or path",
	RunE:    runQueryFiles,
}


func runQuery(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	db, err := openStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	response, err := query.Search(cmd.Context(), db, query.Options{
		Query:   strings.Join(args, " "),
		Project: runtimeConfig.Query.Project,
		Agent:   runtimeConfig.Query.Agent,
		Since:   runtimeConfig.Query.Since,
		Limit:   runtimeConfig.Query.Limit,
		OrderBy: query.OrderBy(runtimeConfig.Query.OrderBy),
	})
	if err != nil {
		return err
	}

	if runtimeConfig.Query.Text {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderText(response))
		return err
	}
	return writeJSON(cmd, response)
}

func runQueryFiles(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	db, err := openStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	response, err := query.SearchFileContext(cmd.Context(), db, query.FileContextOptions{
		FilenameOrPath: strings.Join(args, " "),
		Project:        runtimeConfig.Query.Project,
		Agent:          runtimeConfig.Query.Agent,
		Limit:          runtimeConfig.Query.Limit,
	})
	if err != nil {
		return err
	}

	if runtimeConfig.Query.Text {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderFileContextText(response))
		return err
	}
	return writeJSON(cmd, response)
}

func runQueryProjects(cmd *cobra.Command, args []string) error {
	db, err := openStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	projects, err := db.Queries().ListProjectSummaries(cmd.Context())
	if err != nil {
		return err
	}

	if runtimeConfig.Query.Text {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderProjectsText(projects))
		return err
	}
	return writeJSON(cmd, projects)
}

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/haleyrc/github"
)

func main() {
	ctx := context.Background()

	var org, repo string
	var debug bool

	flag.StringVar(&org, "org", "", "The Github organization")
	flag.StringVar(&repo, "repo", "", "The Github repository")
	flag.BoolVar(&debug, "debug", false, "Enable debug mode")
	flag.Parse()

	client := github.New(os.Getenv("GITHUB_TOKEN"))
	client.Debug = debug

	issuesService := github.NewIssuesService(client)
	issues, err := issuesService.GetIssues(ctx, github.GetIssuesParams{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		panic(err)
	}
	mustSave("issues.json", issues)

	commentsService := github.NewCommentsService(client)
	comments, err := commentsService.GetComments(ctx, github.GetCommentsParams{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		panic(err)
	}
	mustSave("comments.json", comments)

	labelsService := github.NewLabelsService(client)
	labels, err := labelsService.GetLabels(ctx, github.GetLabelsParams{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		panic(err)
	}
	mustSave("labels.json", labels)

	usersService := github.NewUsersService(client)
	assignees, err := usersService.GetAssignees(ctx, github.GetAssigneesParams{
		Org:  org,
		Repo: repo,
	})
	if err != nil {
		panic(err)
	}
	mustSave("assignees.json", assignees)
}

func mustSave(fn string, data interface{}) {
	if err := save(fn, data); err != nil {
		panic(err)
	}
}

func save(fn string, data interface{}) error {
	fn = filepath.Join(".", "dump", fn)

	f, err := os.Create(fn)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	if err := enc.Encode(data); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	return nil
}

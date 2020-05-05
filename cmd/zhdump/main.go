package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/haleyrc/github"
	"github.com/haleyrc/zenhub"
)

func main() {
	ctx := context.Background()

	githubIssues := mustLoad("./dump/issues.json")

	client := zenhub.New(os.Getenv("ZENHUB_TOKEN"))
	issuesService := zenhub.NewIssuesService(client)

	issues := []*zenhub.Issue{}
	for i, issue := range githubIssues {
		fmt.Fprintf(os.Stderr, "Processing issue %d of %d...\n", i+1, len(githubIssues))
		issue, err := issuesService.GetIssue(ctx, repoIDFromEnv(), issue.Number)
		if err != nil {
			panic(err)
		}
		issues = append(issues, issue)
	}
	mustSave("zhissues.json", issues)
}

func save(fn string, data interface{}) error {
	f, err := os.Create(filepath.Join(".", "dump", fn))
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func mustSave(fn string, data interface{}) {
	if err := save(fn, data); err != nil {
		panic(err)
	}
}

func repoIDFromEnv() int64 {
	sid := os.Getenv("ZENHUB_REPO_ID")
	id, _ := strconv.ParseInt(sid, 10, 64)
	return id
}

func mustLoad(fn string) []*github.Issue {
	issues, err := load(fn)
	if err != nil {
		panic(err)
	}
	return issues
}

func load(fn string) ([]*github.Issue, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var issues []*github.Issue
	if err := json.NewDecoder(f).Decode(&issues); err != nil {
		return nil, err
	}

	return issues, nil
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/haleyrc/clubhouse"
	"github.com/haleyrc/github"
	"github.com/haleyrc/gztc"
	"github.com/haleyrc/zenhub"
)

var loginsToIDs = map[string]string{
	"haleyrc":                     "5eaad9b2-d01b-4614-93f4-8f265a8ed5f8",
	"frazerbot":                   "5eaae91d-64c0-46aa-a2c7-4441babe7a31",
	"crm-issues-integration[bot]": "5eaae91d-64c0-46aa-a2c7-4441babe7a31",
}

var missingLogins = map[string]struct{}{}

func githubLoginToClubhouseID(login string) (string, bool) {
	id, ok := loginsToIDs[login]
	if !ok {
		missingLogins[login] = struct{}{}
		return loginsToIDs["frazerbot"], true
	}
	return id, true
}

func main() {
	ctx := context.Background()

	repoID, err := strconv.ParseInt(os.Getenv("ZENHUB_REPO_ID"), 10, 64)
	if err != nil {
		panic(err)
	}

	var debug bool
	params := gztc.MigrateParams{
		MappingFunc: githubLoginToClubhouseID,
		RepoID:      repoID,
	}

	flag.StringVar(&params.Org, "org", "", "The Github organization")
	flag.StringVar(&params.Repo, "repo", "", "The Github repository")
	flag.StringVar(&params.Project, "project", "", "The Clubhouse project")
	flag.BoolVar(&params.DryRun, "dryrun", false, "Don't actually create Clubhouse entities")
	flag.BoolVar(&debug, "debug", false, "Debug")
	flag.Parse()

	githubClient := github.New(os.Getenv("GITHUB_TOKEN"))
	githubClient.Debug = debug

	clubhouseClient := clubhouse.New(os.Getenv("CLUBHOUSE_TOKEN"))
	clubhouseClient.Debug = debug

	zenhubClient := zenhub.New(os.Getenv("ZENHUB_TOKEN"))
	zenhubClient.Debug = debug

	converter := gztc.Converter{
		Clubhouse: clubhouseClient,
		Github:    githubClient,
		Zenhub:    zenhubClient,
		Debug:     debug,
	}

	if err := converter.Migrate(ctx, params); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("Missing logins:")
	for k := range missingLogins {
		fmt.Printf("\t%s\n", k)
	}
}

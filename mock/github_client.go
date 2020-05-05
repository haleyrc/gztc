package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/haleyrc/github"
)

type GithubClient struct{}

func (gc GithubClient) GetIssues(ctx context.Context, org, repo string) ([]*github.Issue, error) {
	f, err := os.Open("./mock/issues.json")
	if err != nil {
		return nil, fmt.Errorf("GithubClient.GetIssues: %w", err)
	}
	defer f.Close()

	var issues []*github.Issue
	if err := json.NewDecoder(f).Decode(&issues); err != nil {
		return nil, fmt.Errorf("GithubClient.GetIssues: %w", err)
	}

	return issues, nil
}

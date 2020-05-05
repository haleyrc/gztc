package mock

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/haleyrc/clubhouse"
)

type ClubhouseClient struct{}

func (cc ClubhouseClient) ListProjects(ctx context.Context) ([]*clubhouse.Project, error) {
	b, err := ioutil.ReadFile("./mock/projects.json")
	if err != nil {
		return nil, err
	}
	var projects []*clubhouse.Project
	if err := json.Unmarshal(b, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

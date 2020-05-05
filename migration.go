package gztc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/haleyrc/clubhouse"
	"github.com/haleyrc/github"
)

type Entities struct {
	CreateEpicParams  []clubhouse.CreateEpicParams
	CreateStoryParams []clubhouse.CreateStoryParams
	StoryLabels       map[string]*StoryLabel
}

type StoryLabel struct {
	CreateLabelParams clubhouse.CreateLabelParams
	IssueIDs          []int64
}

type Migration struct {
	Org         string
	Repo        string
	RepoID      int64
	ProjectID   int64
	MappingFunc MappingFunc `json:"-"`
	Entities    *Entities
	Debug       bool `json:"-"`
}

func (m *Migration) AddLabelsFromIssue(ctx context.Context, issue *github.Issue) {
	for _, label := range issue.Labels {
		if existing, ok := m.Entities.StoryLabels[label.Name]; ok {
			existing.IssueIDs = append(existing.IssueIDs, issue.ID)
			continue
		}
		m.Entities.StoryLabels[label.Name] = &StoryLabel{
			CreateLabelParams: clubhouse.CreateLabelParams{
				Color:       String("#" + label.Color),
				Description: String(label.Description),
				ExternalID:  String(fmt.Sprint(label.ID)),
				Name:        label.Name,
			},
			IssueIDs: []int64{issue.ID},
		}
	}
}

func (m *Migration) AddLabelToIssue(ctx context.Context, name string, issueNumber int64) error {
	id := strconv.FormatInt(issueNumber, 10)

	for _, story := range m.Entities.CreateStoryParams {
		if *story.ExternalID == id {
			if existing, ok := m.Entities.StoryLabels[name]; ok {
				existing.IssueIDs = append(existing.IssueIDs, issueNumber)
				return nil
			}

			m.Entities.StoryLabels[name] = &StoryLabel{
				CreateLabelParams: clubhouse.CreateLabelParams{
					Name: name,
				},
			}

			return nil
		}
	}

	return fmt.Errorf("Migration.addLabelToIssue: no story with id %s", id)
}

func (m *Migration) AddEpicFromIssue(ctx context.Context, issue *github.Issue) error {
	params := clubhouse.CreateEpicParams{
		CreatedAt:   issue.CreatedAt,
		Description: issue.Body,
		ExternalID:  fmt.Sprint(issue.Number),
		Name:        issue.Title,
		UpdatedAt:   &issue.UpdatedAt,
	}

	if issue.User != nil {
		id, found := m.MappingFunc(issue.User.Login)
		if !found {
			m.logf("Failed to set owner %s on epic %q\n", issue.User.Login, params.Name)
		} else {
			params.RequestedByID = id
		}
	}

	if issue.ClosedAt != nil {
		closed, err := time.Parse(time.RFC3339, *issue.ClosedAt)
		if err != nil {
			m.logf("Failed to set closed at on epic %q\n", params.Name)
		} else {
			params.CompletedAtOverride = &closed
		}
	}

	for _, assignee := range issue.Assignees {
		id, found := m.MappingFunc(assignee.Login)
		if !found {
			m.logf("Failed to add assignee %s to epic %q\n", assignee.Login, params.Name)
			continue
		}
		params.OwnerIDs = append(params.OwnerIDs, id)
	}

	for _, label := range issue.Labels {
		params.Labels = append(params.Labels, clubhouse.CreateLabelParams{
			Color:       String("#" + label.Color),
			Description: String(label.Description),
			ExternalID:  String(fmt.Sprint(label.ID)),
			Name:        label.Name,
		})
	}

	m.Entities.CreateEpicParams = append(m.Entities.CreateEpicParams, params)

	return nil
}

func (m *Migration) StoryFromIssue(ctx context.Context, issue *github.Issue) (clubhouse.CreateStoryParams, error) {
	storyType := "feature"
	if hasLabel(issue, "Bug") {
		storyType = "bug"
	}

	params := clubhouse.CreateStoryParams{
		CreatedAt:   Time(issue.CreatedAt),
		Description: strings.TrimSpace(issue.Body),
		ExternalID:  String(fmt.Sprint(issue.Number)),
		Name:        issue.Title,
		ProjectID:   m.ProjectID,
		StoryType:   storyType,
		UpdatedAt:   Time(issue.UpdatedAt),
	}

	if id, _ := m.MappingFunc(issue.User.Login); id != "" {
		params.RequestedByID = String(id)
	}

	for _, assignee := range issue.Assignees {
		if id, exact := m.MappingFunc(assignee.Login); id != "" && exact {
			params.OwnerIDs = append(params.OwnerIDs, id)
		}
	}

	return params, nil
}

func (m *Migration) log(args ...interface{}) {
	fmt.Fprint(os.Stderr, "migration: ")
	fmt.Fprintln(os.Stderr, args...)
}

func (m *Migration) logf(format string, args ...interface{}) {
	fmt.Fprint(os.Stderr, "migration: ")
	fmt.Fprintf(os.Stderr, format, args...)
}

func (m *Migration) debugln(args ...interface{}) {
	if m.Debug {
		fmt.Fprint(os.Stderr, "migration: ")
		fmt.Fprintln(os.Stderr, args...)
	}
}

func (m *Migration) debugf(format string, args ...interface{}) {
	if m.Debug {
		fmt.Fprint(os.Stderr, "migration: ")
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

func hasLabel(issue *github.Issue, label string) bool {
	for _, l := range issue.Labels {
		if l.Name == label {
			return true
		}
	}
	return false
}

package gztc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/haleyrc/clubhouse"
	"github.com/haleyrc/github"
	"github.com/haleyrc/zenhub"
)

type MappingFunc func(githubLogin string) (clubhouseID string, exact bool)

type Converter struct {
	Clubhouse *clubhouse.Client
	Github    *github.Client
	Zenhub    *zenhub.Client
	Debug     bool
}

func (c *Converter) AddLabelsForPipeline(ctx context.Context, migration *Migration) error {
	epicsService := zenhub.NewEpicsService(c.Zenhub)

	for _, param := range migration.Entities.CreateEpicParams {
		id, err := strconv.ParseInt(param.ExternalID, 10, 64)
		if err != nil {
			return fmt.Errorf("Convert.AddLabelsForPipeline: %w", err)
		}

		epic, err := epicsService.GetEpic(ctx, migration.RepoID, id)
		if err != nil {
			return fmt.Errorf("Convert.AddLabelsForPipeline: %w", err)
		}

		for _, issue := range epic.Issues {
			if issue.Pipeline.Name == "" {
				continue
			}
			if err := migration.AddLabelToIssue(ctx, issue.Pipeline.Name, issue.IssueNumber); err != nil {
				c.logf("failed to add label %s to issue #%d\n", issue.Pipeline.Name, issue.IssueNumber)
			}
		}
	}

	return nil
}

func (c *Converter) convert(ctx context.Context, migration *Migration) error {
	migration.Entities = &Entities{
		StoryLabels: make(map[string]*StoryLabel),
	}

	c.logf("Fetching issues for %s/%s...", migration.Org, migration.Repo)
	issues, err := c.Github.GetIssues(ctx, migration.Org, migration.Repo)
	if err != nil {
		return fmt.Errorf("Converter.convert: %w", err)
	}

	numIssues := len(issues)
	c.log()
	for i, issue := range issues {
		c.logf("\r")
		c.logf("converter: Processing issue %d of %d...                         ", i+1, numIssues)

		// TODO (RCH): Should we do anything with pull requests? Maybe just
		// automatically add a comment with the story id!
		if issue.PullRequest != nil {
			continue
		}

		// TODO (RCH): Coalesce the epic so we can create one for realsies
		if isEpic(issue) {
			if err := migration.AddEpicFromIssue(ctx, issue); err != nil {
				return fmt.Errorf("Converter.convert: %w", err)
			}
			continue
		}

		csp, err := migration.StoryFromIssue(ctx, issue)
		if err != nil {
			return fmt.Errorf("Converter.convert: %w", err)
		}

		comments, err := c.Github.GetIssueComments(ctx, github.GetIssueCommentsParams{
			Org:         migration.Org,
			Repo:        migration.Repo,
			IssueNumber: issue.Number,
		})
		if err != nil {
			return fmt.Errorf("Converter.convert: %w", err)
		}

		for _, comment := range comments {
			cscp := clubhouse.CreateStoryCommentParams{
				CreatedAt:  Time(comment.CreatedAt),
				ExternalID: String(fmt.Sprint(comment.ID)),
				Text:       strings.TrimSpace(comment.Body),
				UpdatedAt:  Time(comment.UpdatedAt),
			}
			id, found := migration.MappingFunc(comment.User.Login)
			if !found {
				return fmt.Errorf(
					"Converter.convert: failed to find Clubhouse user for Github login %q",
					comment.User.Login,
				)
			}
			cscp.AuthorID = String(id)
			csp.Comments = append(csp.Comments, cscp)
		}

		migration.Entities.CreateStoryParams = append(migration.Entities.CreateStoryParams, csp)

		migration.AddLabelsFromIssue(ctx, issue)
	}
	c.logf("\r")
	c.logf("converter: Processed %d issues.                                        \n", numIssues)

	if err := c.AddLabelsForPipeline(ctx, migration); err != nil {
		return fmt.Errorf("Converter.convert: %w", err)
	}

	return nil
}

type MigrateParams struct {
	Org         string
	Repo        string
	RepoID      int64
	Project     string
	MappingFunc MappingFunc
	DryRun      bool
}

func (c *Converter) Migrate(ctx context.Context, params MigrateParams) error {
	c.logf("Getting project %q...\n", params.Project)
	project, err := c.getProjectWithName(ctx, params.Project)
	if err != nil {
		return err
	}

	migration := Migration{
		Org:         params.Org,
		Repo:        params.Repo,
		RepoID:      params.RepoID,
		ProjectID:   project.ID,
		MappingFunc: params.MappingFunc,
		Debug:       c.Debug,
	}
	if err := c.convert(ctx, &migration); err != nil {
		return err
	}

	if params.DryRun {
		dump(migration)
		return nil
	}

	if err := c.persist(ctx, &migration); err != nil {
		return err
	}

	return nil
}

func (c *Converter) persist(ctx context.Context, migration *Migration) error {
	zhEpicsService := zenhub.NewEpicsService(c.Zenhub)
	chEpicsService := clubhouse.NewEpicsService(c.Clubhouse)

	numEpics := 0
	for _, params := range migration.Entities.CreateEpicParams {
		epic, err := chEpicsService.CreateEpic(ctx, params)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create epic %q: %v\n", params.Name, err)
			continue
		}

		// Get the Zenhub epic associated with the original issue so we can
		// grab the related issue IDs.
		issueID, err := strconv.ParseInt(params.ExternalID, 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get epic %s %q: %v", params.ExternalID, params.Name, err)
			continue
		}
		zhEpic, err := zhEpicsService.GetEpic(ctx, migration.RepoID, issueID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to get epic %s %q: %v", params.ExternalID, params.Name, err)
			continue
		}

		// Set the epic ID for any stories in the current ZH epic by comparing
		// the story's external ID with the issue ID.
		for _, issue := range zhEpic.Issues {
			if issue.IsEpic {
				continue
			}
			for _, story := range migration.Entities.CreateStoryParams {
				if *story.ExternalID != fmt.Sprint(issue.IssueNumber) {
					continue
				}

				story.EpicID = Int64(epic.ID)
			}
		}

		numEpics++
	}

	c.logf("Processed %d epics...\n", numEpics)

	stories, err := c.Clubhouse.StoriesCreateMultiple(ctx, migration.Entities.CreateStoryParams)
	if err != nil {
		return err
	}

	c.logf("Processed %d stories...\n", len(stories))

	for _, storyLabel := range migration.Entities.StoryLabels {
		ids := getStoryIDsFromExternalIDs(stories, storyLabel.IssueIDs)
		if err := c.Clubhouse.AddLabelToMultipleStories(ctx, ids, storyLabel.CreateLabelParams); err != nil {
			c.logf("failed to add label %s to %v: %v\n", storyLabel.CreateLabelParams.Name, ids, err)
		}
	}

	c.logf("Processed %d labels...\n", len(migration.Entities.StoryLabels))

	return nil
}

func (c *Converter) getProjectWithName(ctx context.Context, name string) (*clubhouse.Project, error) {
	projects, err := c.Clubhouse.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	for i := range projects {
		if projects[i].Name == name {
			return projects[i], nil
		}
	}

	return nil, fmt.Errorf("no project with name %s", name)
}

func (c *Converter) log(args ...interface{}) {
	fmt.Fprint(os.Stderr, "converter: ")
	fmt.Fprintln(os.Stderr, args...)
}

func (c *Converter) logf(format string, args ...interface{}) {
	fmt.Fprint(os.Stderr, "converter: ")
	fmt.Fprintf(os.Stderr, format, args...)
}

func (c *Converter) debugln(args ...interface{}) {
	if c.Debug {
		fmt.Fprint(os.Stderr, "converter: ")
		fmt.Fprintln(os.Stderr, args...)
	}
}

func (c *Converter) debugf(format string, args ...interface{}) {
	if c.Debug {
		fmt.Fprint(os.Stderr, "converter: ")
		fmt.Fprintf(os.Stderr, format, args...)
	}
}

package gztc

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/haleyrc/clubhouse"
	"github.com/haleyrc/github"
)

func getStoryIDsFromExternalIDs(stories []*clubhouse.Story, issueNumbers []int64) []int64 {
	sids := []int64{}
	for _, num := range issueNumbers {
		for _, story := range stories {
			if story.ExternalID == fmt.Sprint(num) {
				sids = append(sids, story.ID)
			}
		}
	}
	return sids
}

func isEpic(issue *github.Issue) bool {
	for _, label := range issue.Labels {
		if label.Name == "Epic" {
			return true
		}
	}
	return false
}

func dump(i interface{}) {
	b, _ := json.MarshalIndent(i, "", "    ")
	fmt.Fprintln(os.Stdout, string(b))
}

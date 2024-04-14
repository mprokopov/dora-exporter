//go:build integration
// +build integration

package github_test

import (
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/config"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/github"
)

const timeFormat = "2006-01-02 15:04:05"

// GITHUB_TOKEN is needed during test time
func TestFindFirstCommitDate(t *testing.T) {

	examples := []struct {
		CommitId string
		Date     string
		Repo     string
	}{
		{ // has related PR
			CommitId: "4678358ceb36e4db10dc7c8ca2ad38aab33941ba",
			Date:     "2022-09-15 05:57:02",
			Repo:     "payment",
		},
		{ // has related PR
			CommitId: "817471c704d6b5a4f777789fc158efcbea3c6502",
			Date:     "2022-09-21 09:51:41",
			Repo:     "core-api",
		},
		{ // no related PR
			CommitId: "af45d3155340352e9823ec6766e2656e264fb9b4",
			Repo:     "core-api",
			Date:     "2022-09-21 13:28:28",
		},
		{ // has related PR
			CommitId: "719a106a6dc175785a63f16db7d9a08ab4afc039",
			Repo:     "platform-terraform",
			Date:     "2022-09-01 13:46:12",
		},
	}

	logger := log.NewLogfmtLogger(os.Stderr)
	github.SetLogger(logger)
	github.SetGitHubApi(config.Github{Token: os.Getenv("GITHUB_TOKEN"), Owner: "mprokopov"})
	t.Logf("%+v", github.GetGitHubApi())
	api := github.GetGitHubApi()

	for _, example := range examples {
		want, _ := time.Parse(timeFormat, example.Date)

		get := api.FindFirstCommitDate(example.Repo, example.CommitId)

		if want != get {
			t.Errorf("Commit date %s doesn't match %s", get, want)
		}
	}
}

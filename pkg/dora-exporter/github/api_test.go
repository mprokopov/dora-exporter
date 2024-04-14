package github_test

import (
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/github"
)

func TestPullRequestId(t *testing.T) {

	logger := log.NewLogfmtLogger(os.Stderr)
	github.SetLogger(logger)

	examples := []struct {
		Pr    string
		Want  string
		Error bool
	}{
		{
			Pr:    "PAY-2669 Introduce multi language for Debtor Reimbursement (#1786)",
			Want:  "1786",
			Error: false,
		},
		{
			Pr:    "Merge pull request #1 from mprokopov/restructure",
			Want:  "1",
			Error: false,
		},
		{
			Pr:    "Merge pull request from mprokopov/restructure",
			Want:  "",
			Error: true,
		},
	}

	for _, want := range examples {
		commit := github.Commit{Message: want.Pr}
		got, err := commit.PullRequestId()
		if got != want.Want || (err == nil) == want.Error {
			t.Errorf("Wants %v got %s", want, got)
		}
	}
}

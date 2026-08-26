package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/config"
)

var logger log.Logger

func SetLogger(log log.Logger) {
	logger = log
}

type GithubApi struct {
	Owner, Token string

	BaseUrl url.URL
}

var githubApi GithubApi

func SetGitHubApi(conf config.Github) {

	// Default GitHub url
	u := url.URL{
		Scheme: "https",
		Host:   "api.github.com",
	}
	githubApi = GithubApi{BaseUrl: u,
		Owner: conf.Owner,
		Token: conf.Token}
}

func GetGitHubApi() GithubApi {
	return githubApi
}

func (api GithubApi) Fetch(path string) ([]byte, error) {
	url := url.URL{Scheme: api.BaseUrl.Scheme,
		Host: api.BaseUrl.Host,
		Path: path,
	}

	req, err := http.NewRequest(http.MethodGet, url.String(), http.NoBody)
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return nil, err
	}
	req.Header.Add("Authorization", "token "+api.Token)

	client := http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)

	level.Debug(logger).Log("component", "github_api", "call", url.String())

	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
		level.Error(logger).Log(err)
		return nil, err
	}

	return io.ReadAll(resp.Body)
}

type Author struct {
	Name string
	Date time.Time
}

type Commit struct {
	Author  Author
	Message string
}

type PullRequest struct {
	Sha    string
	NodeId string `json:"node_id"`
	Commit Commit
}

// https://api.github.com/repos/{{owner}}/{{repo}}/pulls/{{pull_number}}/commits
func (api GithubApi) PullRequestInfo(repo string, pullRequestNumber string) ([]PullRequest, error) {
	var pullRequests []PullRequest
	resBody, err := api.Fetch(fmt.Sprintf("/repos/%s/%s/pulls/%s/commits", api.Owner, repo, pullRequestNumber))
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return nil, err
	}

	err = json.Unmarshal(resBody, &pullRequests)
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return nil, err
	}

	level.Debug(logger).Log("component", "github_api", "repo", repo, "pull_request", pullRequestNumber)

	return pullRequests, nil
}

// https://api.github.com/repos/{{owner}}/{{repo}}/git/commits/{{commit_sha}}
func (api GithubApi) CommitInfo(repo string, sha string) (Commit, error) {
	var commit Commit
	resBody, err := api.Fetch(fmt.Sprintf("/repos/%s/%s/git/commits/%s", api.Owner, repo, sha))
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return Commit{}, err
	}

	err = json.Unmarshal(resBody, &commit)
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return Commit{}, err
	}

	level.Debug(logger).Log("component", "github_api", "repo", repo, "commit_info", sha)

	return commit, nil
}

// BETA-136: ticket notification log no exception (#12)
func (commit Commit) PullRequestId() (string, error) {
	r, _ := regexp.Compile(`#(\d+)`)

	if r.MatchString(commit.Message) {
		level.Debug(logger).Log("commit_message", commit.Message, "pull_request_reference", "found")
		return r.FindStringSubmatch(commit.Message)[1], nil
	}

	level.Debug(logger).Log("commit_message", commit.Message, "pull_request_reference", "not_found")
	return "", errors.New("commit: no PR")
}

// ErrNoCommitDate reports that GitHub answered without an authoring date. The
// zero time.Time it leaves behind turns into a ~292 year lead time downstream,
// so it is rejected here rather than measured.
var ErrNoCommitDate = errors.New("commit: no author date")

// FindFirstCommitDate returns the authoring date the lead time is measured
// from. Every failure to establish that date is returned as an error: callers
// must not fall back to the zero time.
func (api GithubApi) FindFirstCommitDate(repo, sha string) (time.Time, error) {
	commit, err := api.CommitInfo(repo, sha)
	if err != nil {
		return time.Time{}, err
	}

	prId, prErr := commit.PullRequestId()
	if prErr != nil {
		level.Debug(logger).Log("repo", repo, "sha", sha, "date", "current_commit")
		// no pull request associated
		return validCommitDate(commit.Author.Date)
	}

	prInfo, err := api.PullRequestInfo(repo, prId)
	if err != nil {
		return time.Time{}, err
	}
	if len(prInfo) == 0 {
		level.Debug(logger).Log("repo", repo, "sha", sha, "PR", prId, "date", "current_commit", "reason", "no_pr_commits")
		return validCommitDate(commit.Author.Date)
	}

	level.Debug(logger).Log("repo", repo, "sha", sha, "PR", prId, "date", "pr_first_commit")
	return validCommitDate(prInfo[0].Commit.Author.Date)
}

// validCommitDate rejects the zero time a well-formed response with a missing
// or empty author date unmarshals into.
func validCommitDate(date time.Time) (time.Time, error) {
	if date.IsZero() {
		return time.Time{}, ErrNoCommitDate
	}
	return date, nil
}

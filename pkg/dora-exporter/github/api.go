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
	return
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
	req.Header.Add("Authorization", "token "+api.Token)

	if err != nil {
		level.Error(logger).Log(err)
	}

	client := http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)

	level.Debug(logger).Log("component", "github_api", "call", url.String())

	if err != nil {
		level.Error(logger).Log(err)
	}

	defer resp.Body.Close()

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
func (api GithubApi) PullRequestInfo(repo string, pullRequestNumber string) []PullRequest {
	var pullRequests []PullRequest
	resBody, _ := api.Fetch(fmt.Sprintf("/repos/%s/%s/pulls/%s/commits", api.Owner, repo, pullRequestNumber))

	err := json.Unmarshal([]byte(resBody), &pullRequests)
	if err != nil {
		level.Error(logger).Log(err)
	}

	level.Debug(logger).Log("component", "github_api", "repo", repo, "pull_request", pullRequestNumber)

	return pullRequests
}

// https://api.github.com/repos/{{owner}}/{{repo}}/git/commits/{{commit_sha}}
func (api GithubApi) CommitInfo(repo string, sha string) Commit {
	var commit Commit
	resBody, _ := api.Fetch(fmt.Sprintf("/repos/%s/%s/git/commits/%s", api.Owner, repo, sha))

	err := json.Unmarshal([]byte(resBody), &commit)
	if err != nil {
		level.Error(logger).Log(err)
	}

	level.Debug(logger).Log("component", "github_api", "repo", repo, "commit_info", sha)

	return commit
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

func (api GithubApi) FindFirstCommitDate(repo, sha string) time.Time {
	commit := api.CommitInfo(repo, sha)
	prId, err := commit.PullRequestId()

	if err != nil {

		level.Debug(logger).Log("repo", repo, "sha", sha, "date", "current_commit")
		// no pull request associated
		return commit.Author.Date
	}

	prInfo := api.PullRequestInfo(repo, prId)

	level.Debug(logger).Log("repo", repo, "sha", sha, "PR", prId, "date", "pr_first_commit")
	return prInfo[0].Commit.Author.Date
}

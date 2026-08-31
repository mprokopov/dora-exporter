package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
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
func (api GithubApi) PullRequestInfo(repo string, pullRequestNumber string) []PullRequest {
	var pullRequests []PullRequest
	fullRepo := repo
	if !strings.Contains(repo, "/") {
		fullRepo = fmt.Sprintf("%s/%s", api.Owner, repo)
	}
	resBody, err := api.Fetch(fmt.Sprintf("/repos/%s/pulls/%s/commits", fullRepo, pullRequestNumber))
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return pullRequests
	}

	err = json.Unmarshal(resBody, &pullRequests)
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return nil
	}

	level.Debug(logger).Log("component", "github_api", "repo", repo, "pull_request", pullRequestNumber)

	return pullRequests
}

// https://api.github.com/repos/{{owner}}/{{repo}}/git/commits/{{commit_sha}}
func (api GithubApi) CommitInfo(repo string, sha string) Commit {
	var commit Commit
	fullRepo := repo
	if !strings.Contains(repo, "/") {
		fullRepo = fmt.Sprintf("%s/%s", api.Owner, repo)
	}
	resBody, err := api.Fetch(fmt.Sprintf("/repos/%s/git/commits/%s", fullRepo, sha))
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
		return commit
	}

	err = json.Unmarshal(resBody, &commit)
	if err != nil {
		level.Error(logger).Log("component", "github_api", "error", err)
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
	if len(prInfo) == 0 {
		level.Debug(logger).Log("repo", repo, "sha", sha, "PR", prId, "date", "current_commit", "reason", "no_pr_commits")
		return commit.Author.Date
	}

	level.Debug(logger).Log("repo", repo, "sha", sha, "PR", prId, "date", "pr_first_commit")
	return prInfo[0].Commit.Author.Date
}

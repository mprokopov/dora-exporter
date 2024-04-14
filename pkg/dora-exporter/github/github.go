package github

import (
	"encoding/json"
	"github.com/go-kit/log/level"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
	prom "github.com/mprokopov/dora-exporter/pkg/dora-exporter/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"time"
)

type Deployment_Status struct {
	State string
	Url   string
	Id    int
}

type Deployment struct {
	Url         string
	Id          int
	Ref         string
	Sha         string
	Environment string
}

type Repository struct {
	Id        int
	Name      string
	Full_Name string
}

type Sender struct {
	Login string
	Id    int
	Type  string
}

type GitHubWebhookPayload struct {
	Action            string
	Deployment_Status Deployment_Status
	Deployment        Deployment
	Repository        Repository
	Sender            Sender
}

var teams catalog.Teams

func SetTeams(t catalog.Teams) {
	teams = t
	return
}

var cat catalog.TeamsCatalog

func SetCatalog(catalog catalog.TeamsCatalog) {
	cat = catalog
	level.Info(logger).Log("github", "catalog service set")
}

// GetPullRequestDuration returns duration between current time
// and first commit found either from commit itself or from associated PR
func (payload GitHubWebhookPayload) GetCommitDuration() float64 {
	firstCommitDate := githubApi.FindFirstCommitDate(payload.Repository.Name, payload.Deployment.Sha)

	level.Debug(logger).Log("commit_duration", time.Since(firstCommitDate))

	return time.Since(firstCommitDate).Seconds()
}

func GithubAPIHandler(w http.ResponseWriter, r *http.Request) {
	var payload GitHubWebhookPayload
	var duration float64

	if r.Header.Get("X-GitHub-Event") != "deployment_status" {
		w.WriteHeader(202)
		return
	}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		level.Error(logger).Log("endpoint", "github", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	labels := prometheus.Labels{
		"repo":        payload.Repository.Name,
		"environment": payload.Deployment.Environment,
		"team":        cat.GetTeamNameByRepository(payload.Repository.Full_Name),
		"status":      payload.Deployment_Status.State,
	}

	duration = payload.GetCommitDuration()

	prom.IncDeploymentsCount(labels)
	prom.AddDeploymentsDuration(labels, duration)

	level.Info(logger).Log(
		"endpoint", "github",
		"environment", labels["environment"],
		"repository", labels["repo"],
		"status", labels["status"],
		"team", labels["team"],
		"sha", payload.Deployment.Sha)
}

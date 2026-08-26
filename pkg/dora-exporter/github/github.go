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
	Payload     struct {
		Redeployment string
	}
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

var cat catalog.TeamsCatalog

func SetCatalog(catalog catalog.TeamsCatalog) {
	cat = catalog
	level.Info(logger).Log("github", "catalog service set")
}

// GetCommitDuration returns duration between current time
// and first commit found either from commit itself or from associated PR.
// It returns an error when that first commit date could not be established,
// so callers can skip the event instead of recording a bogus lead time.
func (payload GitHubWebhookPayload) GetCommitDuration() (float64, error) {
	firstCommitDate, err := githubApi.FindFirstCommitDate(payload.Repository.Name, payload.Deployment.Sha)
	if err != nil {
		return 0, err
	}

	level.Debug(logger).Log("commit_duration", time.Since(firstCommitDate))

	return time.Since(firstCommitDate).Seconds(), nil
}

func GithubAPIHandler(w http.ResponseWriter, r *http.Request) {
	var payload GitHubWebhookPayload

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
		"repo":         payload.Repository.Name,
		"environment":  payload.Deployment.Environment,
		"team":         cat.GetTeamNameByRepository(payload.Repository.Full_Name),
		"status":       payload.Deployment_Status.State,
		"redeployment": payload.Deployment.Payload.Redeployment,
	}

	// A GitHub API failure leaves the lead time unknown. Record nothing and
	// fail the delivery so it can be redelivered, rather than poisoning the
	// duration gauge and its running sum with a value we cannot compute.
	duration, err := payload.GetCommitDuration()
	if err != nil {
		level.Error(logger).Log(
			"endpoint", "github",
			"repository", labels["repo"],
			"sha", payload.Deployment.Sha,
			"metrics", "skipped",
			"error", err)
		http.Error(w, "github api lookup failed", http.StatusBadGateway)
		return
	}

	prom.IncDeploymentsCount(labels)
	prom.AddDeploymentsDuration(labels, duration)

	level.Info(logger).Log(
		"endpoint", "github",
		"environment", labels["environment"],
		"repository", labels["repo"],
		"status", labels["status"],
		"team", labels["team"],
		"redeployment", labels["redeployment"],
		"sha", payload.Deployment.Sha)
}

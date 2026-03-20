package jira

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-kit/log/level"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
	prom "github.com/mprokopov/dora-exporter/pkg/dora-exporter/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

var logger log.Logger

func SetLogger(log log.Logger) {
	logger = log
	return
}

var cat catalog.TeamsCatalog

func SetCatalog(catalog catalog.TeamsCatalog) {
	cat = catalog
	level.Info(logger).Log("jira", "catalog service set")
}

// GetDuration returns time difference since time.now and issue.Fields.Created in seconds
func (issue Issue) GetDuration() float64 {
	level.Debug(logger).Log("incident_duration", time.Since(issue.Fields.Created.Time))
	return time.Since(issue.Fields.Created.Time).Seconds()
}

type JiraTime struct {
	time.Time
}

type Issue struct {
	Key    string // INF-XXX
	Fields struct {
		Status struct {
			Name           string
			StatusCategory struct {
				Name string
				Key  string
			}
		}
		Teams []struct {
			Name string `json:"value"`
		} `json:"customfield_10792"`
		Created JiraTime
		Project struct {
			Key string
		}
		IssueType struct {
			Name string
		} `json:"issuetype"`
	}
}

func (jtime *JiraTime) UnmarshalJSON(b []byte) error {
	var timestamp int64

	err := json.Unmarshal(b, &timestamp)
	if err != nil {
		return err
	}

	jtime.Time = time.UnixMilli(timestamp)

	return nil
}

type JiraPayload struct {
	Event string
	Issue Issue
}

func ExtractIssue(body io.ReadCloser) (error, Issue) {
	var payload JiraPayload

	decoder := json.NewDecoder(body)
	err := decoder.Decode(&payload)

	if err != nil {
		return err, Issue{}
	}

	level.Info(logger).Log(
		"event", payload.Event,
	)

	return nil, payload.Issue
}

func JiraNewTicketHandler(w http.ResponseWriter, r *http.Request) {
	var labels prometheus.Labels
	err, issue := ExtractIssue(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	labels = prometheus.Labels{
		"type":    issue.Fields.IssueType.Name,
		"project": issue.Fields.Project.Key,
	}

	level.Info(logger).Log(
		"endpoint", "jira_new_ticket",
		"issue_status", issue.Fields.Status.Name,
		"key", issue.Key,
		"created", issue.Fields.Created,
		"type", issue.Fields.IssueType.Name,
		"project", issue.Fields.Project.Key,
	)

	prom.IncNewTicketsCount(labels)
}

func JiraClosedTicketHandler(w http.ResponseWriter, r *http.Request) {
	var labels prometheus.Labels
	err, issue := ExtractIssue(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	labels = prometheus.Labels{
		"type":    issue.Fields.IssueType.Name,
		"project": issue.Fields.Project.Key,
	}
	prom.IncClosedTicketsCount(labels)

	level.Info(logger).Log(
		"endpoint", "jira_close_ticket",
		"issue_status", issue.Fields.Status.Name,
		"key", issue.Key,
		"created", issue.Fields.Created,
		"type", issue.Fields.IssueType.Name,
		"project", issue.Fields.Project.Key,
	)
}

func JiraIncidentHandler(w http.ResponseWriter, r *http.Request) {
	var issue Issue
	var labels prometheus.Labels

	err, issue := ExtractIssue(r.Body)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, team := range issue.Fields.Teams {
		labels = prometheus.Labels{
			"team": team.Name,
		}

		prom.IncIncidentsCount(labels)
		prom.AddIncidentsDuration(labels, issue.GetDuration())

		level.Info(logger).Log(
			"endpoint", "jira",
			"issue_status", issue.Fields.Status.Name,
			"key", issue.Key,
			"team", team.Name,
			"created", issue.Fields.Created,
			"duration", issue.GetDuration(),
			"type", issue.Fields.IssueType.Name,
			"project", issue.Fields.Project.Key,
		)
	}
}

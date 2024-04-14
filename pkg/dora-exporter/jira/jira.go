package jira

import (
	"encoding/json"
	"net/http"
	"strings"
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

const jiraTime = "2006-01-02T15:04:05.000-0700"

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
		// Jira uses non-standard time for created_at
		Created JiraTime
		Project struct {
			Key string
		}
		IssueType struct {
			Name string
		} `json:"issuetype"`
	}
}

func (jtime *JiraTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		jtime.Time = time.Time{}
		return
	}
	jtime.Time, err = time.Parse(jiraTime, s)
	return nil
}

type JiraPayload struct {
	Event string
	Issue Issue
}

func JiraHandler(w http.ResponseWriter, r *http.Request) {
	var payload JiraPayload
	var issue Issue
	var team string
	var labels prometheus.Labels

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	issue = payload.Issue
	team = cat.GetTeamNameByProject(issue.Fields.Project.Key)

	labels = prometheus.Labels{
		"team":    team,
		"project": issue.Fields.Project.Key,
	}

	prom.IncIncidentsCount(labels)
	prom.AddIncidentsDuration(labels, issue.GetDuration())

	level.Info(logger).Log(
		"endpoint", "jira",
		"issue_status", issue.Fields.Status.Name,
		"event", payload.Event,
		"key", issue.Key,
		"team", team,
		"created", issue.Fields.Created,
		"duration", issue.GetDuration(),
		"type", issue.Fields.IssueType.Name,
		"project", issue.Fields.Project.Key,
	)
}

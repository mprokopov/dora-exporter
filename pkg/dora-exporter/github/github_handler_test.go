package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
	prom "github.com/mprokopov/dora-exporter/pkg/dora-exporter/prometheus"
	"github.com/prometheus/client_golang/prometheus"
)

const commitPath = "/repos/mprokopov/dora-exporter/git/commits/abc123"
const pullRequestPath = "/repos/mprokopov/dora-exporter/pulls/42/commits"

const deploymentStatusPayload = `{
  "action": "created",
  "deployment_status": {"state": "success"},
  "deployment": {"sha": "abc123", "environment": "production"},
  "repository": {"name": "dora-exporter", "full_name": "mprokopov/dora-exporter"}
}`

// newHandlerFixture points the package-level GitHub API at a stub server and
// installs a fresh exporter collected by the returned registry, so a test can
// assert exactly which metrics GithubAPIHandler wrote.
func newHandlerFixture(t *testing.T, stub http.HandlerFunc) *prometheus.Registry {
	t.Helper()
	SetLogger(log.NewNopLogger())
	prom.SetLogger(log.NewNopLogger())
	SetCatalog(catalog.Teams{})

	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	githubApi = GithubApi{BaseUrl: *baseURL}

	exporter := prom.NewExporter()
	prom.SetExporter(exporter)
	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter)

	return registry
}

// collect flattens the registry into family name -> value. The handler writes
// at most one label set per test, so a single value per family is enough.
func collect(t *testing.T, registry *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	values := make(map[string]float64)
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			switch {
			case metric.Gauge != nil:
				values[family.GetName()] = metric.GetGauge().GetValue()
			case metric.Counter != nil:
				values[family.GetName()] = metric.GetCounter().GetValue()
			}
		}
	}
	return values
}

// postDeploymentStatus drives one deployment_status webhook through the
// handler and returns the response plus whatever it wrote to the registry.
func postDeploymentStatus(t *testing.T, registry *prometheus.Registry) (*httptest.ResponseRecorder, map[string]float64) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/github", strings.NewReader(deploymentStatusPayload))
	request.Header.Set("X-GitHub-Event", "deployment_status")
	recorder := httptest.NewRecorder()

	GithubAPIHandler(recorder, request)

	return recorder, collect(t, registry)
}

// assertNoDeploymentMetrics is the shared, default-deny assertion every failure
// case runs: a GitHub API failure must leave the deployment metrics untouched.
func assertNoDeploymentMetrics(t *testing.T, values map[string]float64) {
	t.Helper()
	for _, name := range []string{
		"github_deployments_duration",
		"github_deployments_duration_sum",
		"github_deployments_total",
	} {
		if value, ok := values[name]; ok {
			t.Errorf("%s was recorded (%v); expected no metric after a GitHub API failure", name, value)
		}
	}
}

// Positive control: without it, every "no metrics were written" case below
// would pass just as happily against a handler that writes nothing at all.
func TestGithubAPIHandlerUsesPayloadRepositoryOwnerAndRecordsDuration(t *testing.T) {
	commitDate := time.Now().Add(-2 * time.Hour).UTC()
	var requestedPath string

	registry := newHandlerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		fmt.Fprintf(w, `{"Author":{"Date":%q},"Message":"fix bug"}`, commitDate.Format(time.RFC3339))
	})

	recorder, values := postDeploymentStatus(t, registry)
	if requestedPath != commitPath {
		t.Fatalf("GitHub API path = %q, want %q", requestedPath, commitPath)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	duration, ok := values["github_deployments_duration"]
	if !ok {
		t.Fatal("github_deployments_duration was not recorded")
	}
	if duration < 7000 || duration > 7400 {
		t.Errorf("github_deployments_duration = %v, want roughly 7200 (2h)", duration)
	}
	if count := values["github_deployments_total"]; count != 1 {
		t.Errorf("github_deployments_total = %v, want 1", count)
	}
}

func TestGithubAPIHandlerSkipsMetricsWhenCommitLookupFails(t *testing.T) {
	registry := newHandlerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	})

	recorder, values := postDeploymentStatus(t, registry)
	if recorder.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	assertNoDeploymentMetrics(t, values)
}

func TestGithubAPIHandlerSkipsMetricsWhenPullRequestLookupFails(t *testing.T) {
	commitDate := time.Now().Add(-2 * time.Hour).UTC()

	registry := newHandlerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case commitPath:
			fmt.Fprintf(w, `{"Author":{"Date":%q},"Message":"fix bug (#42)"}`, commitDate.Format(time.RFC3339))
		case pullRequestPath:
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	})

	recorder, values := postDeploymentStatus(t, registry)
	if recorder.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	assertNoDeploymentMetrics(t, values)
}

func TestGithubAPIHandlerSkipsMetricsWhenCommitResponseIsMalformed(t *testing.T) {
	registry := newHandlerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != commitPath {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"Author": `)
	})

	recorder, values := postDeploymentStatus(t, registry)
	if recorder.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	assertNoDeploymentMetrics(t, values)
}

// A 200 that simply omits the author date unmarshals cleanly, so error
// propagation alone cannot catch it: only the zero-date guard can.
func TestGithubAPIHandlerSkipsMetricsWhenCommitHasNoDate(t *testing.T) {
	registry := newHandlerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != commitPath {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"Message":"fix bug"}`)
	})

	recorder, values := postDeploymentStatus(t, registry)
	if recorder.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	assertNoDeploymentMetrics(t, values)
}

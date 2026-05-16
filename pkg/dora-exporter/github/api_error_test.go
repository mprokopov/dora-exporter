package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
)

func TestFindFirstCommitDateFallsBackWhenPullRequestHasNoCommits(t *testing.T) {
	SetLogger(log.NewNopLogger())
	commitDate := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/mprokopov/dora-exporter/git/commits/abc123":
			fmt.Fprintf(w, `{"Author":{"Date":%q},"Message":"fix bug (#42)"}`, commitDate.Format(time.RFC3339))
		case "/repos/mprokopov/dora-exporter/pulls/42/commits":
			fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	api := GithubApi{Owner: "mprokopov", BaseUrl: *baseURL}
	if got := api.FindFirstCommitDate("dora-exporter", "abc123"); !got.Equal(commitDate) {
		t.Fatalf("date = %v, want %v", got, commitDate)
	}
}

func TestFetchReturnsTransportError(t *testing.T) {
	SetLogger(log.NewNopLogger())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	server.Close()

	api := GithubApi{BaseUrl: *baseURL}
	if _, err := api.Fetch("/repos/mprokopov/dora-exporter"); err == nil {
		t.Fatal("expected transport error")
	}
}

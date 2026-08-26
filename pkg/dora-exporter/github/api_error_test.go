package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	got, err := api.FindFirstCommitDate("dora-exporter", "abc123")
	if err != nil {
		t.Fatalf("FindFirstCommitDate: %v", err)
	}
	if !got.Equal(commitDate) {
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

// newCommitApiStub points a GithubApi at a stub server for commit lookups.
func newCommitApiStub(t *testing.T, stub http.HandlerFunc) GithubApi {
	t.Helper()
	SetLogger(log.NewNopLogger())

	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	return GithubApi{Owner: "mprokopov", BaseUrl: *baseURL}
}

// The handler-level tests cannot tell an unpropagated API error from the
// zero-date guard: both refuse identically. These assert on the error identity,
// which only correct propagation can produce.
func TestFindFirstCommitDateReturnsCommitApiError(t *testing.T) {
	api := newCommitApiStub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
	})

	_, err := api.FindFirstCommitDate("dora-exporter", "abc123")
	if err == nil {
		t.Fatal("expected an error from the failed commit lookup")
	}
	if errors.Is(err, ErrNoCommitDate) {
		t.Fatalf("error = %v, want the API error, not the zero-date guard", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to carry the 401 status", err)
	}
}

func TestFindFirstCommitDateReturnsCommitDecodeError(t *testing.T) {
	api := newCommitApiStub(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Author": `)
	})

	_, err := api.FindFirstCommitDate("dora-exporter", "abc123")
	if err == nil {
		t.Fatal("expected an error from the malformed commit response")
	}
	if errors.Is(err, ErrNoCommitDate) {
		t.Fatalf("error = %v, want the decode error, not the zero-date guard", err)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v (%T), want a *json.SyntaxError", err, err)
	}
}

func TestFindFirstCommitDateReturnsPullRequestApiError(t *testing.T) {
	commitDate := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)

	api := newCommitApiStub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/mprokopov/dora-exporter/git/commits/abc123":
			fmt.Fprintf(w, `{"Author":{"Date":%q},"Message":"fix bug (#42)"}`, commitDate.Format(time.RFC3339))
		case "/repos/mprokopov/dora-exporter/pulls/42/commits":
			http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	})

	_, err := api.FindFirstCommitDate("dora-exporter", "abc123")
	if err == nil {
		t.Fatal("expected an error from the failed pull request lookup")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %v, want it to carry the 403 status", err)
	}
}

// The commit resolves cleanly but carries no author date: only the zero-date
// guard can reject this one.
func TestFindFirstCommitDateReturnsErrNoCommitDate(t *testing.T) {
	api := newCommitApiStub(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Message":"fix bug"}`)
	})

	_, err := api.FindFirstCommitDate("dora-exporter", "abc123")
	if !errors.Is(err, ErrNoCommitDate) {
		t.Fatalf("error = %v, want ErrNoCommitDate", err)
	}
}

package catalog_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
)

func TestBackstageCatalogPreservesBasePath(t *testing.T) {
	catalog.SetLogger(log.NewLogfmtLogger(os.Stdout))

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath != "/backstage/api/catalog/entities" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("filter"); got != "metadata.annotations.github.com/project-slug=mprokopov/dora-exporter" {
			t.Fatalf("filter = %q", got)
		}
		fmt.Fprint(w, `[{"Spec":{"Owner":"Platform"}}]`)
	}))
	defer server.Close()

	service, err := catalog.NewCatalogFromBackstage(server.URL+"/backstage", "")
	if err != nil {
		t.Fatalf("new backstage catalog: %v", err)
	}
	if got := service.GetTeamNameByRepository("mprokopov/dora-exporter"); got != "Platform" {
		t.Fatalf("team = %q, want Platform", got)
	}
	if gotPath != "/backstage/api/catalog/entities" {
		t.Fatalf("path = %q, want /backstage/api/catalog/entities", gotPath)
	}
}

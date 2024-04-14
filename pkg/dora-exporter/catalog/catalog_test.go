package catalog_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
)

var example = `
- name: Payments
  github_repositories:
    - mprokopov/borscht
    - mprokopov/paella-core
  jira_projects:
    - PAY
- name: Risk
  github_repositories:
    - mprokopov/alfred
    - mprokopov/ds-transaction-identification
  jira_projects:
    - RISK
- name: Infra
  github_repositories:
    - mprokopov/provisioner
    - mprokopov/provisioner2
    - mprokopov/infrastructure
    - mprokopov/sdda
  jira_projects:
    - INF
`

var logger = log.NewLogfmtLogger(os.Stdout)

func SetupCatalog() catalog.TeamsCatalog {
	catalog.SetLogger(logger)
	return catalog.NewCatalogFromYaml(example)
}

func TestGetCatalogByRepository(t *testing.T) {
	service := SetupCatalog()

	examples := map[string]string{
		"mprokopov/provisioner": "Infra",
		"mprokopov/alfred":      "Risk",
		"not_found":           "Unknown"}

	for repo, want := range examples {
		got := service.GetTeamNameByRepository(repo)
		if got != want {
			t.Errorf("Wanted %s got %s", want, got)
		}
	}

}

func TestGetCatalogByProject(t *testing.T) {
	service := SetupCatalog()
	catalog.SetLogger(logger)
	examples := map[string]string{"INF": "Infra",
		"PAY":       "Payments",
		"not_found": "Unknown"}

	for project, want := range examples {

		got := service.GetTeamNameByProject(project)
		if got != want {
			t.Errorf("Wanted %s got %s", want, got)
		}
	}

}
func Example_basic() {
	service := SetupCatalog()

	fmt.Println(service.GetTeamNameByProject("INF"))
	fmt.Println(service.GetTeamNameByProject("PAY"))
	// Output:
	// Infra
	// Payments
}

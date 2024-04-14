package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"gopkg.in/yaml.v3"
)

var logger log.Logger

func SetLogger(log log.Logger) {
	logger = log
	return
}

type Team struct {
	Name         string
	Repositories []string `yaml:"github_repositories"`
	Projects     []string `yaml:"jira_projects"`
}

type Teams []Team

type TeamsCatalog interface {
	// Fetch team name by repository name
	GetTeamNameByRepository(repo string) string
	// Fetch team name by repository name
	GetTeamNameByProject(project string) string
}

type BackstageCatalog struct {
	Endpoint url.URL
}

type BackstageResponse struct {
	Spec struct {
		Owner string
	}
}

func (teams Teams) GetTeamNameByRepository(repository string) string {
	for _, team := range teams {
		for _, repo := range team.Repositories {
			if repo == repository {
				return team.Name
			}
		}
	}
	return "Unknown"
}

// GetTeamNameByProject returns team name from catalog query
func (teams Teams) GetTeamNameByProject(project string) string {
	for _, team := range teams {
		for _, teamProject := range team.Projects {
			if project == teamProject {
				return team.Name
			}
		}
	}
	return "Unknown"
}

func NewCatalogFromYaml(yamlString string) TeamsCatalog {
	var teams Teams
	err := yaml.Unmarshal([]byte(yamlString), &teams)
	if err != nil {
		level.Error(logger).Log(err)
		panic(1)
	}
	level.Info(logger).Log("catalog", "static", "teams", len(teams))
	return teams
}

func NewCatalogFromBacktage(backstageUrl string) TeamsCatalog {
	url, err := url.Parse(backstageUrl)
	if err != nil {
		level.Error(logger).Log(err)
		panic(1)
	}

	var backstage = BackstageCatalog{Endpoint: *url}
	level.Info(logger).Log("catalog", "backstage", "endpoint", url.String())
	return backstage
}

// GET :base-url/:base-path/entities?filter=metadata.annotations.github.com/project-slug=mprokopov/dora-exporter

func (backstage BackstageCatalog) GetTeamNameByRepository(repository string) string {
	var filter string
	var backstageResults []BackstageResponse
	filter = fmt.Sprintf("metadata.annotations.github.com/project-slug=%s", repository)

	jsonResp, _ := backstage.Fetch(filter)

	err := json.Unmarshal(jsonResp, &backstageResults)

	if err != nil {
		level.Error(logger).Log("catalog", err)
	}

	if len(backstageResults) == 0 || backstageResults[0].Spec.Owner == "" {
		level.Info(logger).Log("catalog", "owner could not be determined from backstage", "repository", repository)
		return "Unknown"
	}

	level.Info(logger).Log("catalog", "owner successfully determined from backstage", "owner", backstageResults[0].Spec.Owner, "repository", repository)

	return backstageResults[0].Spec.Owner
}

func (backstage BackstageCatalog) GetTeamNameByProject(project string) string {
	level.Info(logger).Log("catalog", "query backstage team name by project", "repository", project)
	return "Unknown"
}

func (backstage BackstageCatalog) Fetch(filter string) ([]byte, error) {
	var uri url.URL
	uri = backstage.Endpoint
	uri.Path = "/api/catalog/entities"
	q := uri.Query()

	q.Add("filter", filter)
	uri.RawQuery = q.Encode()
	level.Info(logger).Log("catalog", "query", "filter", filter, "uri", uri.String())

	req, err := http.NewRequest(http.MethodGet, uri.String(), http.NoBody)
	if err != nil {
		level.Error(logger).Log("catalog", err)
	}

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		level.Error(logger).Log("catalog", err)
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

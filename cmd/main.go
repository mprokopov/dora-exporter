package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/config"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/github"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/jira"
	prom "github.com/mprokopov/dora-exporter/pkg/dora-exporter/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var fileName string

var exp *prom.Exporter

var logger log.Logger = log.NewLogfmtLogger(os.Stderr)

var conf config.Config

var cat catalog.TeamsCatalog

var configFile string

func init() {
	lvl := flag.String("log", "info", "debug, info, warn, error")
	flag.StringVar(&configFile, "config.file", "config.yml", "Configuration file path")
	flag.Parse()
	logger = level.NewFilter(logger, level.Allow(level.ParseDefault(*lvl, level.InfoValue())))
	github.SetLogger(logger)
	config.SetLogger(logger)
	prom.SetLogger(logger)
	jira.SetLogger(logger)
	catalog.SetLogger(logger)
}

func HandlerWithSave(file string, handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r)

		prom.SaveMetricsToFile(file)
	}
}

func main() {
	conf.Load(configFile)
	fileName = conf.Storage.File.Path

	github.SetGitHubApi(conf.Github)

	if conf.Catalog.Mode == "backstage" {
		cat = catalog.NewCatalogFromBacktage(conf.Catalog.Endpoint)
	} else {
		cat = catalog.NewCatalogFromYaml(conf.GetTeamsString())
	}

	github.SetCatalog(cat)
	jira.SetCatalog(cat)

	exp = prom.NewExporter()
	prom.SetExporter(exp)
	prometheus.MustRegister(exp)
	err := prom.LoadMetricsFromFile(fileName)

	if err != nil {
		_ = level.Info(logger).Log("metrics", "loader", "file", fileName, "status", "creating new file")
		prom.SaveMetricsToFile(fileName)
	}

	http.HandleFunc("/api/github", HandlerWithSave(fileName, github.GithubAPIHandler))
	http.HandleFunc("/api/jira", HandlerWithSave(fileName, jira.JiraHandler))
	http.Handle("/metrics", promhttp.Handler())

	_ = level.Info(logger).Log("server", "started", "port", conf.Server.Port)
	err = http.ListenAndServe(":"+conf.Server.Port, nil)
	if err != nil {
		level.Error(logger).Log(err)
	}
}

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

		if err := prom.SaveMetricsToFile(file); err != nil {
			level.Error(logger).Log("metrics", "save", "file", file, "error", err)
		}
	}
}

func main() {
	if err := conf.Load(configFile); err != nil {
		level.Error(logger).Log("config", "load", "file", configFile, "error", err)
		os.Exit(1)
	}
	fileName = conf.Storage.File.Path

	github.SetGitHubApi(conf.Github)

	var catErr error
	if conf.Catalog.Mode == "backstage" {
		cat, catErr = catalog.NewCatalogFromBackstage(conf.Catalog.Endpoint, conf.Catalog.Token)
	} else {
		cat, catErr = catalog.NewCatalogFromYaml(conf.GetTeamsString())
	}
	if catErr != nil {
		level.Error(logger).Log("catalog", "init", "mode", conf.Catalog.Mode, "error", catErr)
		os.Exit(1)
	}

	github.SetCatalog(cat)

	exp = prom.NewExporter()
	prom.SetExporter(exp)
	prometheus.MustRegister(exp)
	err := prom.LoadMetricsFromFile(fileName)

	if err != nil {
		_ = level.Info(logger).Log("metrics", "loader", "file", fileName, "status", "creating new file")
		if err := prom.SaveMetricsToFile(fileName); err != nil {
			level.Error(logger).Log("metrics", "save", "file", fileName, "error", err)
		}
	}

	http.HandleFunc("/api/github", HandlerWithSave(fileName, github.GithubAPIHandler))
	http.HandleFunc("/api/jira/incidents/rca", HandlerWithSave(fileName, jira.JiraIncidentHandler))
	http.HandleFunc("/api/jira/tickets/new", HandlerWithSave(fileName, jira.JiraNewTicketHandler))
	http.HandleFunc("/api/jira/tickets/close", HandlerWithSave(fileName, jira.JiraClosedTicketHandler))
	http.Handle("/metrics", promhttp.Handler())

	_ = level.Info(logger).Log("server", "started", "port", conf.Server.Port)
	err = http.ListenAndServe(":"+conf.Server.Port, nil)
	if err != nil {
		level.Error(logger).Log("server", "listen", "error", err)
		os.Exit(1)
	}
}

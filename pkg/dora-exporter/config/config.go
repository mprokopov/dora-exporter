package config

import (
	"io/ioutil"
	"os"

	"errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mprokopov/dora-exporter/pkg/dora-exporter/catalog"
	"gopkg.in/yaml.v3"
)

const defaultExporterFile = "dora-exporter.prom"

type Github struct {
	Owner string
	Token string
	//BaseUrl url.URL
}

type Config struct {
	Github  Github
	Catalog struct {
		Mode     string
		Endpoint string
	}
	Teams  catalog.Teams
	Server struct {
		Port string
	}
	// Prometheus metrics snapshot storage
	Storage struct {
		File struct {
			Path string
		}
	}
}

var logger log.Logger

func SetLogger(log log.Logger) {
	logger = log
	return
}

func NewConfigFromFile(file string) *Config {
	var conf *Config
	conf.Load(file)
	return conf
}

func (c *Config) Load(file string) *Config {
	yamlFile, err := ioutil.ReadFile(file)

	if err != nil {
		level.Error(logger).Log("config", file, "error", err)
		os.Exit(1)
	}

	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		level.Error(logger).Log("config", file, "error", err)
	}
	if c.Github.Token == "" {
		if os.Getenv("GITHUB_TOKEN") == "" {
			err := errors.New("No GitHub token found")
			level.Error(logger).Log("config", file, "github_token", err)
			os.Exit(1)
		}
		c.Github.Token = os.Getenv("GITHUB_TOKEN")
	}

	if c.Storage.File.Path == "" {
		if os.Getenv("STORAGE_FILE_PATH") != "" {
			c.Storage.File.Path = os.Getenv("STORAGE_FILE_PATH")
		} else {
			c.Storage.File.Path = defaultExporterFile
		}
	}
	level.Info(logger).Log("config", "storage", "file", c.Storage.File.Path)

	for _, team := range c.Teams {
		level.Info(logger).Log("config", "load", "team", team.Name, "repos", len(team.Repositories), "projects", len(team.Projects))
	}

	if c.Catalog.Mode == "" {
		// default is static when not specified
		c.Catalog.Mode = "static"
	}

	level.Info(logger).Log("config", "finished", "file", file)
	return c
}

func (c *Config) GetTeams() catalog.Teams {
	return append(c.Teams, catalog.Team{Name: "Unknown"})
}

func (c *Config) GetTeamsString() string {
	data, err := yaml.Marshal(c.GetTeams())
	if err != nil {
		level.Error(logger).Log(err)
	}
	return string(data)
}

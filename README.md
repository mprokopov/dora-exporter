# DORA exporter

DORA metrics prometheus compatible exporter.

![Grafana Dashboard Screenshot](/images/screenshot.png "Grafana Dashboard")


## How it works

dora-exporter listens to deployment events from GitHub and ticket events from Jira.

### GitHub Integration
Every successfull deployment increases counter `github_deployments_count`. Such counter sets labels: `team`, `status`, `environment`, `repo`.

The `team` label can be set using manual mapping with configuration file, or by querying `Backstage` backend.

Whenever GitHub deployment event received, dora-exporter does query to GitHub API to calculate a time between the first commit in the related Pull Request and deployment event.

This time is added to counter `github_deployments_duration`. Such counter contains labels: 
`team`, `status`, `environment`, `repo`.

### Jira Integration

## Grafana Dashboard

https://grafana.com/grafana/dashboards/20889-dora-v2/

## Installation

DORA exporter is a single binary that doesn't require any dependencies. Though you might want to run it using docker.
The default port is 8090, but this can be changed in the configuration.

### Run in docker

```shell
docker run --rm -e GITHUB_TOKEN=gh_xxxxx -p 8090:8090 dora-exporter
```

## Configuration

Configuration file location can be specified using command line flag -config.file.

```shell
dora-exporter -config.file=config.yml
```

GitHub token is required to query information about the deployment and commit, so we expect the GITHUB_TOKEN environment variable to contain valid token. See [Generate GitHub token](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) for details.

## Snapshot path

DORA-exporter saves state in the prometheus compatible file format. This allows to preserve the statistics state between reboots.

Location for the storage can be set in the following config.yml section.

```yaml
storage:
  file:
    path: /data/prometheus.prom
```

It is advised to map it to the external volume to preserve state between restarts.

## Backstage backend support

DORA exporter has support for catalog either from enterprise Backstage installation or from static source using Yaml configuration.

### Backstage

Backstage component in order to be available for dora-exporter query should contain [github.com/project-slug annotation](https://backstage.io/docs/features/software-catalog/well-known-annotations#githubcomproject-slug).
In case an owner can't be determined, default `Unknown` team will be used.

Example

```yaml
---
apiVersion: backstage.io/v1alpha1
kind: Component
metadata:
  name: dora-exporter
  title: DORA-Metrics
  description: DORA Metrics exporter
  annotations:
    github.com/project-slug: mprokopov/dora-exporter
    github.com/team-slug: mprokopov/Infrastructure
```

Put to the config file the following settings

```yaml
catalog:
  mode: backstage
  endpoint: http://backstage.com
```

### Static

Statis is the default mode and will use the information about the teams from the yaml dictionary as per example below.

```yaml
catalog:
  mode: static

teams:
  - name: team1
    github_repositories:
      - owner/repo1
      - owner/repo2
    jira_projects:
      - PROJECT1
      - PROJECT2
  - name: team2
    github_repositories:
      - owner/repo3
      - owner/repo4
```

## Debugging

Error level can be selected from command line using flag `-log`.
Possible error log values are: info,debug,warning or error

### Example

```shell
dora-exporter -log debug
```

## GitHub Integration setup
Dora Exporter requires information about deployments of your applications. The easiest way is to setup a organization-wide webhook. Make sure your GitHub repository shows deployments information.

```
https://<dora-exporter-url>/api/github
```
Ensure GitHub repository has information about deployments 
![Deployments](/images/deployments.png "Deployments for application")

Ensure setting up organization-wide webhook as per screenshots

![Deployment Webhook Setup](/images/deployment-webhook.png "Deployment Webhook Setup")

![Deployment Webhook Setup 2](/images/deployment-webhook2.png "Deployment Webhook Setup Step 2")

If the integration successfull, and GitHub sends deployment signals to dora-exporter, its `/metrics` endpoint should contain `github_deployments_duration` metric.

```prometheus
# HELP github_deployments_duration The last deployments duration
# TYPE github_deployments_duration gauge
github_deployments_duration{environment="staging",repo="adminka-core",status="success",team="Platform"} 9.223372036854776e+09
# HELP github_deployments_duration_sum The last deployments duration sum
# TYPE github_deployments_duration_sum gauge
github_deployments_duration_sum{environment="staging",repo="adminka-core",status="success",team="Platform"} 9.223372036854776e+09
# HELP github_deployments_total The amount of successful deployments.
# TYPE github_deployments_total counter
github_deployments_total{environment="staging",repo="adminka-core",status="success",team="Platform"} 1
```

## Jira integration setup

Setup webhook for Jira issues to point to:

```
https://<dora-exporter-url>/api/jira
```


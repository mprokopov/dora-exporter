package prometheus

import (
	"os"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type Exporter struct {
	mu                       sync.Mutex
	deployments_count        *prometheus.CounterVec
	deployments_duration     *prometheus.GaugeVec
	deployments_duration_sum *prometheus.GaugeVec
	incidents_count          *prometheus.CounterVec
	incidents_duration_sum   *prometheus.GaugeVec
}

var logger log.Logger

func SetLogger(log log.Logger) {
	logger = log
	return
}

var exp *Exporter

func SetExporter(e *Exporter) {
	exp = e
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.deployments_count.Collect(ch)
	e.deployments_duration.Collect(ch)
	e.deployments_duration_sum.Collect(ch)
	e.incidents_count.Collect(ch)
	e.incidents_duration_sum.Collect(ch)
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	e.deployments_count.Describe(ch)
	e.deployments_duration.Describe(ch)
	e.deployments_duration_sum.Describe(ch)
	e.incidents_count.Describe(ch)
	e.incidents_duration_sum.Describe(ch)
}

var JiraLabels = []string{"project", "team"}
var GithubLabels = []string{"repo", "environment", "team", "status"}

func NewExporter() *Exporter {
	return &Exporter{
		deployments_count: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "github",
			Name:      "deployments_total",
			Help:      "The amount of successful deployments.",
		}, GithubLabels),
		deployments_duration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "github",
			Name:      "deployments_duration",
			Help:      "The last deployments duration",
		}, GithubLabels),
		deployments_duration_sum: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "github",
			Name:      "deployments_duration_sum",
			Help:      "The last deployments duration sum",
		}, GithubLabels),
		incidents_duration_sum: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "jira",
			Name:      "incidents_duration_sum",
			Help:      "The amount of incidents."},
			JiraLabels,
		),
		incidents_count: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "jira",
			Name:      "incidents",
			Help:      "The amount of incidents.",
		}, JiraLabels),
	}
}

// UpdateCounter resets counter and increments using value from metric
func UpdateCounter(counter *prometheus.CounterVec, metric *io_prometheus_client.Metric) {
	var labels prometheus.Labels = make(map[string]string)
	var value float64
	for _, labelPair := range metric.GetLabel() {
		labels[*labelPair.Name] = *labelPair.Value
		_ = level.Debug(logger).Log("import", "counter", "label", *labelPair.Name, "value", *labelPair.Value)
	}
	value = metric.GetCounter().GetValue()

	_ = level.Debug(logger).Log("import", "counter", "counter_value", value)
	counter.Delete(labels) // Reset the counter
	counter.With(labels).Add(value)
}

// UpdateGauge sets gauge to value from metric
func UpdateGauge(gauge *prometheus.GaugeVec, metric *io_prometheus_client.Metric) {
	var labels prometheus.Labels = make(map[string]string)
	for _, labelPair := range metric.GetLabel() {
		labels[labelPair.GetName()] = labelPair.GetValue()
		_ = level.Debug(logger).Log("import", "gauge", "label", *labelPair.Name, "value", *labelPair.Value)
	}

	_ = level.Debug(logger).Log("import", "gauge", "gauge_value", metric.GetGauge().GetValue())
	gauge.With(labels).Set(metric.GetGauge().GetValue())
}

func (e *Exporter) Update(family string, metric *io_prometheus_client.Metric) {
	switch family {
	case "jira_incidents_duration_sum":
		UpdateGauge(exp.incidents_duration_sum, metric)

	case "github_deployments_duration":
		UpdateGauge(exp.deployments_duration, metric)

	case "github_deployments_duration_sum":
		UpdateGauge(exp.deployments_duration_sum, metric)

	case "jira_incidents":
		UpdateCounter(exp.incidents_count, metric)

	case "github_deployments_total":
		UpdateCounter(exp.deployments_count, metric)
	}
}

func SaveMetricsToFile(file string) {
	_ = prometheus.WriteToTextfile(file, prometheus.DefaultGatherer)
	_ = level.Info(logger).Log("metrics", "exported", "file", file)
}

func LoadMetricsFromFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		level.Info(logger).Log("metrics", "loader", "file", file, "status", "not found")
		return err
	}
	defer f.Close()

	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(f)
	if err != nil {
		level.Info(logger).Log("metrics", "loader", "file", file, "status", "couldn't parse metrics")
		return err
	}

	for i, metricFamily := range families {
		for _, metric := range metricFamily.GetMetric() {
			_ = level.Debug(logger).Log("family", i, "metric", metric.String())
			exp.Update(i, metric)
		}
	}

	_ = level.Info(logger).Log("metrics", "imported", "file", file)
	return nil
}

func IncIncidentsCount(labels prometheus.Labels) {
	exp.incidents_count.With(labels).Inc()
	_ = level.Debug(logger).Log("counter", "incidents_count", "action", "inc")
}

func AddIncidentsDuration(labels prometheus.Labels, duration float64) {
	exp.incidents_duration_sum.With(labels).Add(duration)
	_ = level.Debug(logger).Log("gauge", "incidents_duration_sum", "action", "set", "value", duration)
}

func IncDeploymentsCount(labels prometheus.Labels) {
	exp.deployments_count.With(labels).Inc()

	_ = level.Debug(logger).Log("counter", "deployments_count", "action", "inc")
}

func AddDeploymentsDuration(labels prometheus.Labels, duration float64) {

	exp.deployments_duration.With(labels).Set(duration)

	_ = level.Debug(logger).Log("counter", "deployments_duration", "action", "set", "value", duration)

	exp.deployments_duration_sum.With(labels).Add(duration)
}

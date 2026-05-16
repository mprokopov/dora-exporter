package prometheus

import (
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	clientmodel "github.com/prometheus/client_model/go"
)

func counterMetric(value float64, labels map[string]string) *clientmodel.Metric {
	metric := &clientmodel.Metric{
		Counter: &clientmodel.Counter{Value: &value},
	}
	for name, value := range labels {
		name, value := name, value
		metric.Label = append(metric.Label, &clientmodel.LabelPair{Name: &name, Value: &value})
	}
	return metric
}

func TestUpdateUsesReceiverWithoutGlobalExporter(t *testing.T) {
	SetLogger(log.NewNopLogger())
	exp = nil
	exporter := NewExporter()
	labels := prometheus.Labels{"team": "Platform"}

	exporter.Update("jira_incidents", counterMetric(3, labels))

	metric, err := exporter.incidents_count.GetMetricWith(labels)
	if err != nil {
		t.Fatalf("get metric: %v", err)
	}

	var got clientmodel.Metric
	if err := metric.Write(&got); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if got.GetCounter().GetValue() != 3 {
		t.Fatalf("counter = %v, want 3", got.GetCounter().GetValue())
	}
}

func TestSaveMetricsToFileReturnsWriteError(t *testing.T) {
	SetLogger(log.NewNopLogger())
	missingParent := filepath.Join(t.TempDir(), "missing", "metrics.prom")
	if err := SaveMetricsToFile(missingParent); err == nil {
		t.Fatal("expected write error for missing parent directory")
	}
}

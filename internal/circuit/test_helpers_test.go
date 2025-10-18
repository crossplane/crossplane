package circuit

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

func metricValue(t *testing.T, reg prometheus.Gatherer, name string, labels map[string]string) float64 {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	for _, mf := range families {
		if mf.GetName() != name {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if matchesLabels(metric.GetLabel(), labels) {
				if c := metric.GetCounter(); c != nil {
					return c.GetValue()
				}
			}
		}
	}

	return 0
}

func matchesLabels(pairs []*io_prometheus_client.LabelPair, expected map[string]string) bool {
	if len(pairs) != len(expected) {
		return false
	}

	for _, lp := range pairs {
		if expected[lp.GetName()] != lp.GetValue() {
			return false
		}
	}

	return true
}

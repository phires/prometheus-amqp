package filter

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promlog"
)

func TestInit(t *testing.T) {
	Init("../conf/empty.conf")
	t.Logf("Found %d elements in empty config\n", len(filters))
	Init("../conf/filter.test.conf")
	t.Logf("Found %d elements in config\n", len(filters))
	for _, f := range filters {
		t.Logf("%s %s %s\n", f.Label, f.Filter, f.Value)
	}

}

func TestMachtesFilter(t *testing.T) {
	// Build a single metric
	m := buildSingleMetric()
	logLevel := promlog.AllowedLevel{}
	logLevel.Set("debug")
	logger := promlog.New(logLevel)
	match, _ := MatchesFilter(m, logger)

	if match {
		t.Log("Matches")
	} else {
		t.Error("No match")
	}
}

func BenchmarkMachtesFilter(b *testing.B) {
	// Build a single metric
	m := buildMultipleMetrics(b.N)
	logLevel := promlog.AllowedLevel{}
	logLevel.Set("debug")
	logger := promlog.New(logLevel)

	for i := 0; i < b.N; i++ {
		MatchesFilter(m[i], logger)
	}
}

func buildSingleMetric() model.Metric {
	var m model.Metric
	m = make(map[model.LabelName]model.LabelValue)
	m["__name__"] = "nginx_http_requests_total"
	m["app"] = "testApp"
	m["instance"] = "testInstance"

	return m
}

func buildMultipleMetrics(count int) []model.Metric {

	var r []model.Metric
	for i := 0; i < count; i++ {
		var m model.Metric
		m = make(map[model.LabelName]model.LabelValue)
		m["__name__"] = "nginx_http_requests_total"
		m["app"] = "testApp"
		m["instance"] = "testInstance"
		r = append(r, m)
	}

	return r
}

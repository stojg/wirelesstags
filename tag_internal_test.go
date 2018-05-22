package wirelesstags

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGet(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "GetMultiTagStatsRaw") {
			http.ServeFile(w, r, "testdata/GetMultiTagStatsRaw_temperature.json")
		}
	}))
	defer ts.Close()

	wt, _ := NewHTTPClient(HTTPConfig{
		Addr:     ts.URL,
		Location: time.Local,
	})

	since := time.Date(2017, time.October, 15, 15, 0, 0, 0, time.Local)
	metrics := make(map[uint8]MetricsCollection)
	err := wt.getMetrics([]uint8{3, 1}, typeTemperature, metrics, since)
	if err != nil {
		t.Error(err)
		return
	}

	if len(metrics) != 2 {
		t.Errorf("Expected %d tag metrics, got %d", 10, len(metrics))
		return
	}

	if len(metrics[1]) != 42 {
		t.Errorf("Expected %d metrics for tag 1, got %d", 42, len(metrics[1]))
		return
	}

	if len(metrics[3]) != 38 {
		t.Errorf("Expected %d metrics for tag 3, got %d", 38, len(metrics[3]))
		return
	}
}

func TestWindowFileTime(t *testing.T) {
	actual := windowsFileTime(131557748239379584)
	expected := time.Unix(1511301223, 937000000)

	if actual != expected {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

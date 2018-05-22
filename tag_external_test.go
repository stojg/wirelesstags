package wirelesstags_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"strings"

	"github.com/stojg/grabber/lib/wirelesstags"
)

func TestGet(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "GetTagList2") {
			http.ServeFile(w, r, "testdata/GetTagList2.json")
		}
		if strings.Contains(r.URL.String(), "LoadTempSensorConfig") {
			http.ServeFile(w, r, "testdata/LoadTempSensorConfig.json")
		}
		if strings.Contains(r.URL.String(), "GetMultiTagStatsRaw") {
			http.ServeFile(w, r, "testdata/GetMultiTagStatsRaw_temperature.json")
		}
	}))
	defer ts.Close()

	wt, _ := wirelesstags.NewHTTPClient(wirelesstags.HTTPConfig{
		Addr:     ts.URL,
		Location: time.Local,
	})
	sensors, err := wt.Get(time.Now())
	if err != nil {
		t.Error(err)
		return
	}
	expected := 10
	actual := len(sensors)
	if actual != expected {
		t.Errorf("Expected %d sensors, got %d sensors", actual, expected)
		return
	}

	sensor := sensors[5]
	expectedComment := "location=bath,level=5,num=3"
	if sensor.Comment != expectedComment {
		t.Errorf("Expected tag comment to be '%s', got '%s'", expectedComment, sensor.Comment)
	}
}

func TestSensorTags(t *testing.T) {
	s := &wirelesstags.Sensor{
		Name:    "My Room",
		SlaveID: 3,
		Comment: "location=bath ,level =5, color=yellow ",
	}

	labels := s.Labels()
	if len(labels) != 5 {
		t.Errorf("Expected sensor to have 5 labels, got %d", len(labels))
	}

	expected := make(map[string]string)
	expected["name"] = s.Name
	expected["id"] = "3"
	expected["location"] = "bath"
	expected["level"] = "5"
	expected["color"] = "yellow"

	for label, value := range expected {
		if labels[label] != value {
			t.Errorf("Expected label '%s' to be '%s', got '%s'", label, value, labels[label])
		}
	}
}

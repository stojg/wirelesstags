package wirelesstags

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"crypto/tls"

	"errors"
	"net/url"

	"github.com/mitchellh/mapstructure"
)

type metricType byte

func (m metricType) toAPI() string {
	switch m {
	case typeTemperature:
		return "temperature"
	case typeLux:
		return "light"
	case typeHumidity:
		return "cap"
	case typeMotion:
		return "motion"
	case typeBattery:
		return "batteryVolt"
	case typeSignal:
		return "signal"
	}
	return "unknown"
}

func (m metricType) toName() string {
	switch m {
	case typeTemperature:
		return "temperature"
	case typeLux:
		return "lux"
	case typeHumidity:
		return "humidity"
	case typeMotion:
		return "motion"
	case typeBattery:
		return "battery"
	case typeSignal:
		return "signal"
	}
	return "unknown"
}

const (
	_ metricType = iota
	typeTemperature
	typeLux
	typeHumidity
	typeMotion
	typeBattery
	typeSignal
)

// Metric has a name and value
type Metric struct {
	value float32
	name  metricType
}

// Name returns the name representation of the metric
func (m *Metric) Name() string {
	return m.name.toName()
}

// Value returns the value of the metric
func (m *Metric) Value() float32 {
	return m.value
}

// MetricsCollection buckets metrics in a timestamp for a more efficient updating of the metrics in the backend
type MetricsCollection map[int64][]*Metric

// HTTPConfig is the config data needed to create an HTTP Client.
type HTTPConfig struct {
	// Addr should be of the form "http://host:port"
	// or "http://[ipv6-host%zone]:port".
	Addr string

	// Token is the API token for the wirelesstag API
	Token string

	// Location is what timezone that the tags has been set to
	Location *time.Location

	// UserAgent is the http User Agent, defaults to "WirelessTagClient".
	UserAgent string

	// Timeout for gets writes, defaults to no timeout.
	Timeout time.Duration

	// InsecureSkipVerify gets passed to the http client, if true, it will
	// skip https certificate verification. Defaults to false.
	InsecureSkipVerify bool

	// TLSConfig allows the user to set their own TLS config for the HTTP
	// Client. If set, this option overrides InsecureSkipVerify.
	TLSConfig *tls.Config
}

// NewHTTPClient creates a new httpClient that is used for fetching tag sensor information from http://www.wirelesstag.net/
func NewHTTPClient(conf HTTPConfig) (*Client, error) {

	if conf.UserAgent == "" {
		conf.UserAgent = "WirelessTagClient"
	}

	u, err := url.Parse(conf.Addr)
	if err != nil {
		return nil, err
	} else if u.Scheme != "http" && u.Scheme != "https" {
		m := fmt.Sprintf("Unsupported protocol scheme: %s, your address must start with http:// or https://", u.Scheme)
		return nil, errors.New(m)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: conf.InsecureSkipVerify,
		},
	}
	if conf.TLSConfig != nil {
		tr.TLSClientConfig = conf.TLSConfig
	}
	return &Client{
		url:       *u,
		token:     conf.Token,
		useragent: conf.UserAgent,
		location:  conf.Location,
		httpClient: &http.Client{
			Timeout:   conf.Timeout,
			Transport: tr,
		},
	}, nil

}

// Client is a holder for information used for getting and parsing sensor tag data
type Client struct {
	url        url.URL
	token      string
	location   *time.Location
	useragent  string
	httpClient *http.Client
}

// Get all sensor data and return a list of Sensor
func (c *Client) Get(since time.Time) ([]*Sensor, error) {

	u := c.url
	u.Path = "ethClient.asmx/GetTagList2"

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer([]byte(`{}`)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	var result map[string]interface{}

	var resp *http.Response
	if resp, err = c.httpClient.Do(req); err != nil {
		return nil, fmt.Errorf("error during tag GetTagList2: %v", err)
	}
	defer closer(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status code %d", resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	if err = dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing JSON response body: %v", err)
	}

	var tags []*Sensor
	if err = mapstructure.Decode(result["d"], &tags); err != nil {
		return nil, fmt.Errorf("error while decoding sensor tag data: %v", err)
	}

	var temperatureTags []uint8
	var lightTags []uint8
	var humidityTags []uint8

	for _, t := range tags {

		lastConn := windowsFileTime(t.LastComm)

		// we want to ensure that tags that reported since last time will get their new metrics in
		if lastConn.Before(since) && !t.OutOfRange {
			since = lastConn
		}

		if t.hasTempSensor() {
			temperatureTags = append(temperatureTags, t.SlaveID)
		}
		if t.hasLightSensor() {
			lightTags = append(lightTags, t.SlaveID)
		}
		if t.hasHumiditySensor() {
			humidityTags = append(humidityTags, t.SlaveID)
		}
	}

	// the metrics are keyed by the sensors slaveID
	metrics := make(map[uint8]MetricsCollection)
	if err = c.getMetrics(temperatureTags, typeTemperature, metrics, since); err != nil {
		return nil, err
	}

	if err = c.getMetrics(humidityTags, typeHumidity, metrics, since); err != nil {
		return nil, err
	}

	if err = c.getMetrics(lightTags, typeLux, metrics, since); err != nil {
		return nil, err
	}

	for _, tag := range tags {
		if m, ok := metrics[tag.SlaveID]; ok {
			tag.Metrics = m
		}
	}

	return tags, err
}

func (c *Client) getMetrics(ids []uint8, mType metricType, metrics map[uint8]MetricsCollection, since time.Time) error {

	if len(ids) == 0 {
		return nil
	}

	var resp *http.Response
	var err error
	if resp, err = c.requestMetrics(ids, mType.toAPI(), since); err != nil {
		return err
	}

	defer closer(resp.Body)

	var body []byte
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var message struct {
			Message       string
			ExceptionType string
			StackTrace    string
		}
		if err = json.Unmarshal(body, &message); err != nil {
			return err
		}
		return fmt.Errorf("%s", message.Message)
	}

	var result map[string]struct {
		Stats []struct {
			Date      string      `json:"date"`
			IDs       []uint8     `json:"ids"`
			Values    [][]float32 `json:"values"`
			TimeOfDay [][]uint32  `json:"tods"`
		} `json:"stats"`
		TempUnit int      `json:"temp_unit"`
		Ids      []int    `json:"ids"`
		Names    []string `json:"names"`
	}

	if err = json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("error decoding JSON response %s: %v", resp.Request.URL, err)
	}

	var auckland *time.Location
	if auckland, err = time.LoadLocation("Pacific/Auckland"); err != nil {
		return err
	}

	for _, stat := range result["d"].Stats {
		startDate, err := time.ParseInLocation("1/2/2006", stat.Date, auckland)
		if err != nil {
			return fmt.Errorf("can't parse start date %s", stat.Date)
		}
		for i, slaveID := range stat.IDs {
			for j := range stat.TimeOfDay[i] {
				timestamp := startDate.Add(time.Second * time.Duration(stat.TimeOfDay[i][j]))
				if timestamp.Before(since) {
					continue
				}

				if _, ok := metrics[slaveID]; !ok {
					metrics[slaveID] = make(MetricsCollection)
				}

				metrics[slaveID][timestamp.Unix()] = append(metrics[slaveID][timestamp.Unix()], &Metric{
					name:  mType,
					value: stat.Values[i][j],
				})
			}
		}
	}

	return nil
}

func (c *Client) requestMetrics(ids []uint8, metricType string, since time.Time) (*http.Response, error) {
	input := &getMultiTagStatsRawInput{
		IDs:      ids,
		Type:     metricType,
		FromDate: since.Format("1/2/2006"),
		ToDate:   time.Now().In(c.location).Format("1/2/2006"),
	}

	requestBody, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	u := c.url
	u.Path = "ethLogs.asmx/GetMultiTagStatsRaw"

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

type getMultiTagStatsRawInput struct {
	IDs      []uint8 `json:"ids"`
	Type     string  `json:"type"`
	FromDate string  `json:"fromDate"`
	ToDate   string  `json:"toDate"`
}

// IDs []uint8 encodes as a base64-encoded string, so we need our own marshalling
func (t *getMultiTagStatsRawInput) MarshalJSON() ([]byte, error) {
	var ids string
	if t.IDs == nil {
		ids = "null"
	} else {
		ids = strings.Join(strings.Fields(fmt.Sprintf("%d", t.IDs)), ",")
	}
	jsonResult := fmt.Sprintf(`{"ids": %s, "type": "%s","fromDate": "%s", "toDate": "%s"}`, ids, t.Type, t.FromDate, t.ToDate)
	return []byte(jsonResult), nil
}

func closer(c io.Closer) {
	err := c.Close()
	if err != nil {
		fmt.Printf("Error during Close: err")
	}
}

// windowsFileTime returns the windows FILETIME value in Unix time.
//  - Windows FILETIME is 100 nanosecond intervals since January 1, 1601 (UTC)
//  - Unix Date time is seconds since January 1, 1970 (UTC)
//  - Offset between the two epochs in milliseconds is 116444736e+5
// Note that the smallest return resolution is milliseconds
func windowsFileTime(intervals int64) time.Time {
	// we need to convert 100ns intervals to ms so we don't overflow on int64
	var ms = intervals / 10 / 1000

	// offset between windows epoch and unix epoch in milliseconds
	var epochOffset int64 = 116444736e+5

	// millisecond since unix epoch start
	var unix = time.Millisecond * time.Duration(ms-epochOffset)

	sec := unix / time.Second
	nsec := unix % time.Second

	return time.Unix(int64(sec), int64(nsec))
}

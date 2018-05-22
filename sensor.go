package wirelesstags

import (
	"fmt"
	"strings"
)

// Sensor represents a single sensor tag with the information provided from the API. The metrics can be found at Sensor.Metrics
type Sensor struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	//TempEventState int     `json:"tempEventState"` // Disarmed, TooLow, TooHigh, Normal
	OutOfRange bool `json:"outOfRange"`
	//Temperature    float64 `json:"temperature"`
	SlaveID uint8 `json:"slaveId"`
	//BatteryVolt    float64 `json:"batteryVolt"`
	//Lux            float64 `json:"lux"`
	//Humidity       float64 `json:"cap"` // humidity
	//Type           string  `json:"__type"`
	//ManagerName    string  `json:"managerName"`
	TagType  int   `json:"tagType"`
	LastComm int64 `json:"lastComm"`
	//Alive          bool    `json:"alive"`
	//SignaldBm      int     `json:"signaldBm"`
	//Beeping        bool    `json:"beeping"`
	//Lit            bool    `json:"lit"`
	//MigrationPending    bool    `json:"migrationPending"`
	//BeepDurationDefault int     `json:"beepDurationDefault"`
	//EventState          int     `json:"eventState"`
	//TempCalOffset       float64 `json:"tempCalOffset"`
	//CapCalOffset        int     `json:"capCalOffset"`
	//CapRaw              int     `json:"capRaw"`
	//Az2                 int     `json:"az2"`
	//CapEventState       int     `json:"capEventState"`
	//LightEventState     int     `json:"lightEventState"`
	//Shorted             bool    `json:"shorted"`
	//PostBackInterval    int     `json:"postBackInterval"`
	//Rev                 int     `json:"rev"`
	//Version1            int     `json:"version1"`
	//FreqOffset          int     `json:"freqOffset"`
	//FreqCalApplied      int     `json:"freqCalApplied"`
	//ReviveEvery         int     `json:"reviveEvery"`
	//OorGrace            int     `json:"oorGrace"`
	//LBTh                float64 `json:"LBTh"`
	//EnLBN               bool    `json:"enLBN"`
	//Txpwr               int     `json:"txpwr"`
	//RssiMode            bool    `json:"rssiMode"`
	//Ds18                bool    `json:"ds18"`
	//BatteryRemaining    float64 `json:"batteryRemaining"`

	Metrics MetricsCollection
}

// Labels returns a map of key / value. 'name' and 'id' is always returned. The extra labels are added to the comment
// field in the UI and follows the name1=value1,name2=value2 format. It should be relatively resilient to whitespaces.
func (s *Sensor) Labels() map[string]string {
	labels := make(map[string]string)
	labels["name"] = s.Name
	labels["id"] = fmt.Sprintf("%d", s.SlaveID)
	extraLabels := strings.Split(s.Comment, ",")
	for _, extra := range extraLabels {
		keyValues := strings.Split(extra, "=")
		if len(keyValues) != 2 {
			continue
		}
		key := strings.Trim(keyValues[0], " ")
		labels[key] = strings.Trim(keyValues[1], " ")
	}
	return labels
}

func (s *Sensor) hasMotionSensor() bool {
	return inArray(s.TagType, []int{12, 13, 21})
}

func (s *Sensor) hasLightSensor() bool {
	return inArray(s.TagType, []int{26})
}

func (s *Sensor) hasMoistureSensor() bool {
	return inArray(s.TagType, []int{32, 33})
}

func (s *Sensor) hasWaterSensor() bool {
	return inArray(s.TagType, []int{32, 33})
}

func (s *Sensor) hasReedSensor() bool {
	return inArray(s.TagType, []int{52, 53})
}

func (s *Sensor) hasPIRSensor() bool {
	return inArray(s.TagType, []int{72})
}

func (s *Sensor) hasEventSensor() bool {
	return s.hasMotionSensor() || s.hasLightSensor() || s.hasReedSensor() || s.hasPIRSensor()
}

func (s *Sensor) hasHumiditySensor() bool {
	return s.hasHTU()
}

func (s *Sensor) hasTempSensor() bool {
	return !inArray(s.TagType, []int{82, 92})
}

func (s *Sensor) hasCurrentSensor() bool {
	return s.TagType == 42
}

/** Whether the tag's temperature sensor is high-precision (> 8-bit). */
func (s *Sensor) hasHTU() bool {
	return inArray(s.TagType, []int{13, 21, 52, 26, 72})
}

// can playback data that was recorded while being offline
func (s *Sensor) canPlayback() bool {
	return s.TagType == 21
}

func inArray(needle int, haystack []int) bool {
	for _, v := range haystack {
		if needle == v {
			return true
		}
	}
	return false
}

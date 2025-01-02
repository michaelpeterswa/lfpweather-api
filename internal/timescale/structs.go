package timescale

import "time"

type WeatherRow struct {
	Time time.Time `json:"time"`
	Min  float64   `json:"min"`
	Max  float64   `json:"max"`
	Avg  float64   `json:"avg"`
}

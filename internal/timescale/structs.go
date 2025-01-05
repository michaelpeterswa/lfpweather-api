package timescale

import "time"

type GetColumnResponse struct {
	Time time.Time `json:"time"`
	Min  float64   `json:"min"`
	Max  float64   `json:"max"`
	Avg  float64   `json:"avg"`
}

type GetColumnLastResponse struct {
	Time time.Time `json:"time"`
	Last float64   `json:"last"`
}

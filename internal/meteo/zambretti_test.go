package meteo

import (
	"testing"
	"time"
)

// Vectors computed by hand from the pywws ZambrettiCore algorithm (wind sector,
// seasonal adjustment, F formula, round-clip, table lookup). They pin the port.
func TestZambretti(t *testing.T) {
	cases := []struct {
		name     string
		pressure float64 // hPa
		windDir  float64 // degrees
		month    time.Month
		trend    float64 // hPa/hour
		wantCode string
		wantText string
	}{
		{
			name:     "high steady pressure, north wind, winter -> settled fine",
			pressure: 1030, windDir: 0, month: time.January, trend: 0,
			wantCode: "A", wantText: "Settled fine",
		},
		{
			name:     "low falling pressure, south wind, summer -> rain very unsettled",
			pressure: 990, windDir: 180, month: time.July, trend: -0.5,
			wantCode: "X", wantText: "Rain, very unsettled",
		},
		{
			name:     "mid rising pressure, west wind, spring -> fairly fine early showers",
			pressure: 1000, windDir: 270, month: time.April, trend: 0.3,
			wantCode: "G", wantText: "Fairly fine, possible showers early",
		},
		{
			name:     "low steady pressure, east wind, winter -> frequent rain",
			pressure: 985, windDir: 90, month: time.December, trend: 0,
			wantCode: "W", wantText: "Rain at frequent intervals",
		},
		{
			name:     "very high rising pressure, north wind, winter -> settled fine",
			pressure: 1035, windDir: 0, month: time.January, trend: 0.5,
			wantCode: "A", wantText: "Settled fine",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, text := Zambretti(c.pressure, c.windDir, c.month, c.trend)
			if code != c.wantCode || text != c.wantText {
				t.Errorf("Zambretti(%.0f, %.0f, %s, %.1f) = (%q, %q), want (%q, %q)",
					c.pressure, c.windDir, c.month, c.trend, code, text, c.wantCode, c.wantText)
			}
		})
	}
}

func TestInHgToHPa(t *testing.T) {
	// 30.00 inHg is about 1015.9 hPa.
	got := 30.0 * InHgToHPa
	if got < 1015 || got > 1017 {
		t.Errorf("30 inHg = %.1f hPa, want ~1015.9", got)
	}
}

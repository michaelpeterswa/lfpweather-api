package meteo

import (
	"math"
	"testing"
	"time"
)

// Golden vectors produced by compiling and running the reference Argonne C
// implementation (mdljts/wbgt) at 47.68 N, 122.28 W. A faithful port must
// reproduce them to within floating-point noise.
func TestWBGTAgainstReference(t *testing.T) {
	cases := []struct {
		name                       string
		utc                        time.Time
		tair, rh, pres, solar, spd float64
		wantTg, wantTnwb, wantWBGT float64
	}{
		{
			name: "midday sun 25C 50% 800",
			utc:  time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC),
			tair: 25, rh: 50, pres: 1013, solar: 800, spd: 1.0,
			wantTg: 42.8694, wantTnwb: 21.3993, wantWBGT: 26.0534,
		},
		{
			name: "pre-dawn 15C 90% 0",
			utc:  time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC),
			tair: 15, rh: 90, pres: 1013, solar: 0, spd: 0.5,
			wantTg: 13.3310, wantTnwb: 13.6540, wantWBGT: 13.7240,
		},
		{
			name: "hot afternoon 30C 40% 500",
			utc:  time.Date(2026, 8, 1, 21, 0, 0, 0, time.UTC),
			tair: 30, rh: 40, pres: 1015, solar: 500, spd: 2.0,
			wantTg: 39.9625, wantTnwb: 21.9905, wantWBGT: 26.3859,
		},
		{
			name: "winter noon 5C 85% 200",
			utc:  time.Date(2026, 1, 15, 20, 0, 0, 0, time.UTC),
			tair: 5, rh: 85, pres: 1020, solar: 200, spd: 3.0,
			wantTg: 8.2865, wantTnwb: 5.0141, wantWBGT: 5.6672,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := WBGT(c.utc, 47.68, -122.28, c.tair, c.rh, c.pres, c.solar, c.spd)
			if !got.OK {
				t.Fatalf("solver did not converge")
			}
			if math.Abs(got.Globe-c.wantTg) > 0.01 {
				t.Errorf("Tg = %.4f, want %.4f", got.Globe, c.wantTg)
			}
			if math.Abs(got.NaturalWetBulb-c.wantTnwb) > 0.01 {
				t.Errorf("Tnwb = %.4f, want %.4f", got.NaturalWetBulb, c.wantTnwb)
			}
			if math.Abs(got.WBGT-c.wantWBGT) > 0.01 {
				t.Errorf("WBGT = %.4f, want %.4f", got.WBGT, c.wantWBGT)
			}
		})
	}
}

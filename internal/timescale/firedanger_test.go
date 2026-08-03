package timescale

import "testing"

// season percentiles used across the classification cases, taken from the
// real station distribution (Tolt-comparable, fuel model Y, May-Oct):
// p50=9, p80=14.7, p90=17, p97=19.9.
func testERC(value float64) ERCContext {
	return ERCContext{Value: value, P50: 9, P80: 14.7, P90: 17, P97: 19.9, Max: 25.3}
}

func TestClassifyFireDanger(t *testing.T) {
	cases := []struct {
		name      string
		erc       float64
		kbdi      float64
		dead10hr  float64
		wantClass string
		wantLevel int
		wantIn    string // substring expected in Detail
	}{
		{
			name: "below median is low", erc: 6, kbdi: 200, dead10hr: 22,
			wantClass: "Low", wantLevel: 0, wantIn: "keeping the fine fuels damp",
		},
		{
			name: "median to p80 is moderate", erc: 10.4, kbdi: 405, dead10hr: 20,
			wantClass: "Moderate", wantLevel: 1, wantIn: "Drought is elevated",
		},
		{
			name: "p80 to p90 is high", erc: 15, kbdi: 300, dead10hr: 12,
			wantClass: "High", wantLevel: 2, wantIn: "high for the season",
		},
		{
			name: "p90 to p97 is very high", erc: 18, kbdi: 300, dead10hr: 12,
			wantClass: "Very High", wantLevel: 3, wantIn: "top tenth",
		},
		{
			name: "at or above p97 is extreme", erc: 20, kbdi: 300, dead10hr: 12,
			wantClass: "Extreme", wantLevel: 4, wantIn: "top few percent",
		},
		{
			name: "critically dry fine fuels drive the detail", erc: 15, kbdi: 300, dead10hr: 6,
			wantClass: "High", wantLevel: 2, wantIn: "critically dry",
		},
		{
			name: "severe drought outranks elevated", erc: 15, kbdi: 650, dead10hr: 12,
			wantClass: "High", wantLevel: 2, wantIn: "Soil drought is severe",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := FireDangerSummary{
				EnergyRelease: testERC(c.erc),
				Drought:       DroughtContext{KBDI: c.kbdi},
				FuelMoisture:  FuelMoistureContext{Dead10hr: c.dead10hr},
			}
			got := classifyFireDanger(s)

			if got.Class != c.wantClass {
				t.Errorf("class = %q, want %q", got.Class, c.wantClass)
			}
			if got.Level != c.wantLevel {
				t.Errorf("level = %d, want %d", got.Level, c.wantLevel)
			}
			if got.Headline != c.wantClass+" fire danger" {
				t.Errorf("headline = %q, want %q", got.Headline, c.wantClass+" fire danger")
			}
			if !contains(got.Detail, c.wantIn) {
				t.Errorf("detail = %q, want to contain %q", got.Detail, c.wantIn)
			}
		})
	}
}

// boundary check: a value exactly on a breakpoint takes the higher class.
func TestClassifyFireDangerBoundaries(t *testing.T) {
	cases := []struct {
		erc       float64
		wantClass string
	}{
		{9, "Moderate"},   // == p50
		{14.7, "High"},    // == p80
		{17, "Very High"}, // == p90
		{19.9, "Extreme"}, // == p97
	}
	for _, c := range cases {
		s := FireDangerSummary{EnergyRelease: testERC(c.erc)}
		if got := classifyFireDanger(s); got.Class != c.wantClass {
			t.Errorf("erc %.1f: class = %q, want %q", c.erc, got.Class, c.wantClass)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

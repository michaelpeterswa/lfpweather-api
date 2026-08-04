package meteo

import "testing"

func TestGrowingDegreeDaysF(t *testing.T) {
	cases := []struct {
		name             string
		tMax, tMin, base float64
		want             float64
	}{
		{"warm day base 50", 80, 60, 50, 20},       // mean 70, -50 = 20
		{"mean equals base", 55, 45, 50, 0},        // mean 50, -50 = 0
		{"cold day floors at zero", 45, 35, 50, 0}, // mean 40, would be -10
		{"exactly at base is zero", 50, 50, 50, 0},
		{"grape winkler style base 50", 90, 50, 50, 20}, // mean 70
		{"cool-season base 41", 60, 40, 41, 9},          // mean 50, -41 = 9
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GrowingDegreeDaysF(c.tMax, c.tMin, c.base)
			if got != c.want {
				t.Errorf("GrowingDegreeDaysF(%.0f, %.0f, %.0f) = %.2f, want %.2f",
					c.tMax, c.tMin, c.base, got, c.want)
			}
		})
	}
}

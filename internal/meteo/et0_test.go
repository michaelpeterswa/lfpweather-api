package meteo

import (
	"math"
	"testing"
)

// TestFAO56EToExample18 reproduces FAO-56 Example 18 (Uccle, Brussels; 6 July;
// 50.80 N, 100 m): Tmax 21.5, Tmin 12.3, RHmax 84, RHmin 63, u2 2.078 m/s,
// Rs 22.07 MJ/m2/day. The published result is ETo = 3.9 mm/day.
func TestFAO56EToExample18(t *testing.T) {
	got := FAO56ETo(21.5, 12.3, 84, 63, 2.078, 22.07, 100, 50.80, 187)
	if math.Abs(got-3.9) > 0.05 {
		t.Errorf("FAO56ETo Example 18 = %.3f mm/day, want 3.9 (+/-0.05)", got)
	}
}

func TestFAO56EToNonNegative(t *testing.T) {
	// A cold, humid, still, dark winter day should yield a small non-negative ETo.
	got := FAO56ETo(38, 30, 95, 80, 0.3, 1.5, 34, 47.68, 15)
	if got < 0 {
		t.Errorf("ETo = %.3f, want >= 0", got)
	}
}

func TestUnitConversions(t *testing.T) {
	if c := FAHtoC(50); math.Abs(c-10) > 1e-9 {
		t.Errorf("FAHtoC(50) = %.3f, want 10", c)
	}
	if ms := MPHtoMS(10); math.Abs(ms-4.4704) > 1e-6 {
		t.Errorf("MPHtoMS(10) = %.4f, want 4.4704", ms)
	}
	// A 3 m anemometer reads slightly high vs the 2 m reference (factor ~0.92).
	if u2 := WindTo2m(1.0, 3.0); math.Abs(u2-0.921) > 0.005 {
		t.Errorf("WindTo2m(1, 3) = %.3f, want ~0.921", u2)
	}
	if mj := Wm2ToMJPerDay(100); math.Abs(mj-8.64) > 1e-6 {
		t.Errorf("Wm2ToMJPerDay(100) = %.3f, want 8.64", mj)
	}
}

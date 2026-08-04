package meteo

import (
	"math"
	"time"
)

// InHgToHPa converts a pressure in inches of mercury to hectopascals (hPa/mb).
// The Davis station reports barometer_sea_level in inHg; Zambretti wants hPa.
const InHgToHPa = 33.8639

// zambrettiForecasts maps the letter code A..Z (index 0..25) to its forecast
// text, from the Zambretti Forecaster.
var zambrettiForecasts = [26]string{
	"Settled fine",                        // A
	"Fine weather",                        // B
	"Becoming fine",                       // C
	"Fine, becoming less settled",         // D
	"Fine, possible showers",              // E
	"Fairly fine, improving",              // F
	"Fairly fine, possible showers early", // G
	"Fairly fine, showery later",          // H
	"Showery early, improving",            // I
	"Changeable, mending",                 // J
	"Fairly fine, showers likely",         // K
	"Rather unsettled clearing later",     // L
	"Unsettled, probably improving",       // M
	"Showery, bright intervals",           // N
	"Showery, becoming less settled",      // O
	"Changeable, some rain",               // P
	"Unsettled, short fine intervals",     // Q
	"Unsettled, rain later",               // R
	"Unsettled, some rain",                // S
	"Mostly very unsettled",               // T
	"Occasional rain, worsening",          // U
	"Rain at times, very unsettled",       // V
	"Rain at frequent intervals",          // W
	"Rain, very unsettled",                // X
	"Stormy, may improve",                 // Y
	"Stormy, much rain",                   // Z
}

// The three trend lookup tables, as forecast indices (A=0..Z=25). Rounded,
// clipped Z indexes these to pick the forecast letter.
var (
	zRising  = []int{0, 1, 1, 2, 5, 6, 8, 9, 11, 12, 12, 16, 19, 24}                // A B B C F G I J L M M Q T Y
	zFalling = []int{1, 3, 7, 14, 17, 20, 21, 23, 23, 25}                           // B D H O R U V X X Z
	zSteady  = []int{0, 1, 1, 1, 4, 10, 13, 13, 15, 15, 18, 22, 22, 23, 23, 23, 25} // A B B B E K N N P P S W W X X X Z
)

// zWindAdj adjusts pressure by wind direction, indexed by the 16-point compass
// sector (0 = north, clockwise). A northerly adds pressure (fairer), a
// southerly subtracts it.
var zWindAdj = [16]float64{5.2, 4.2, 3.2, 1.05, -1.1, -3.15, -5.2, -8.35, -11.5, -9.4, -7.3, -5.25, -3.2, -1.15, 0.9, 3.05}

// Zambretti returns the forecast letter code (A..Z) and text for the northern
// hemisphere from sea-level pressure (hPa), wind direction (degrees from north),
// the calendar month, and the 3-hour pressure trend in hPa per hour (positive
// rising). The trend deadband is +/-0.1 hPa/hour.
//
// Reference: the Zambretti Forecaster (Negretti and Zambra, ~1915), as
// reconstructed in the pywws ZambrettiCore module (Jim Easterbrook), which is
// the de facto reference implementation. pywws normalizes pressure to a
// configurable range; with its default 950-1050 hPa range that step is the
// identity, so it is omitted here.
func Zambretti(pressureHPa, windDirDeg float64, month time.Month, trendHPaPerHour float64) (code, text string) {
	pressure := pressureHPa

	// Wind direction adjustment (16-point compass, north = sector 0).
	sector := int(math.Round(windDirDeg/22.5)) % 16
	if sector < 0 {
		sector += 16
	}
	pressure += zWindAdj[sector]

	summer := month >= time.April && month <= time.September

	var f float64
	var lut []int
	switch {
	case trendHPaPerHour >= 0.1:
		if summer {
			pressure += 3.2
		}
		f = 0.1740 * (1031.40 - pressure)
		lut = zRising
	case trendHPaPerHour <= -0.1:
		if summer {
			pressure -= 3.2
		}
		f = 0.1553 * (1029.95 - pressure)
		lut = zFalling
	default:
		f = 0.2314 * (1030.81 - pressure)
		lut = zSteady
	}

	idx := int(math.Floor(f + 0.5))
	if idx < 0 {
		idx = 0
	}
	if idx > len(lut)-1 {
		idx = len(lut) - 1
	}

	forecast := lut[idx]
	return string(rune('A' + forecast)), zambrettiForecasts[forecast]
}

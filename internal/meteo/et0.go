package meteo

import "math"

// FAHtoC converts degrees Fahrenheit to degrees Celsius.
func FAHtoC(f float64) float64 { return (f - 32) * 5.0 / 9.0 }

// MPHtoMS converts miles per hour to metres per second.
func MPHtoMS(mph float64) float64 { return mph * 0.44704 }

// WindTo2m adjusts a wind speed measured at height z (metres) to the 2 m
// reference height using the FAO-56 logarithmic profile (eq. 47).
func WindTo2m(uz, zMeters float64) float64 {
	return uz * 4.87 / math.Log(67.8*zMeters-5.42)
}

// Wm2ToMJPerDay converts a mean solar irradiance in W/m^2 over a day to the
// day's total radiation in MJ per m^2 (1 W/m^2 sustained for a day is 0.0864
// MJ/m^2).
func Wm2ToMJPerDay(meanWm2 float64) float64 { return meanWm2 * 0.0864 }

// saturationVapourPressure returns the saturation vapour pressure (kPa) at
// temperature t (deg C), FAO-56 eq. 11.
func saturationVapourPressure(t float64) float64 {
	return 0.6108 * math.Exp(17.27*t/(t+237.3))
}

// FAO56ETo returns the daily reference evapotranspiration (mm/day) by the FAO-56
// Penman-Monteith method for a grass reference surface.
//
// Inputs are already in SI and at the reference height: daily max/min air
// temperature (deg C), max/min relative humidity (%), wind speed at 2 m (m/s),
// the day's total solar radiation (MJ/m^2/day), site elevation (m), latitude
// (degrees), and the day of the year (1-366). Soil heat flux G is taken as zero
// for a daily step.
//
// Reference: Allen, R.G., Pereira, L.S., Raes, D., Smith, M. (1998), "Crop
// evapotranspiration - Guidelines for computing crop water requirements", FAO
// Irrigation and Drainage Paper 56, equations 6, 8, 11-14, 21-25, 37-40, 47.
func FAO56ETo(tMaxC, tMinC, rhMax, rhMin, u2, rsMJ, elevM, latDeg float64, dayOfYear int) float64 {
	tMean := (tMaxC + tMinC) / 2

	// Atmospheric pressure (eq. 7) and psychrometric constant (eq. 8).
	p := 101.3 * math.Pow((293-0.0065*elevM)/293, 5.26)
	gamma := 0.000665 * p

	// Slope of the saturation vapour pressure curve at Tmean (eq. 13).
	delta := 4098 * saturationVapourPressure(tMean) / ((tMean + 237.3) * (tMean + 237.3))

	// Saturation and actual vapour pressure (eq. 12, 17).
	esMax := saturationVapourPressure(tMaxC)
	esMin := saturationVapourPressure(tMinC)
	es := (esMax + esMin) / 2
	ea := (esMin*rhMax/100 + esMax*rhMin/100) / 2

	// Extraterrestrial radiation (eq. 21-25).
	j := float64(dayOfYear)
	phi := latDeg * math.Pi / 180
	dr := 1 + 0.033*math.Cos(2*math.Pi*j/365)
	decl := 0.409 * math.Sin(2*math.Pi*j/365-1.39)
	ws := math.Acos(-math.Tan(phi) * math.Tan(decl))
	ra := (24 * 60 / math.Pi) * 0.0820 * dr *
		(ws*math.Sin(phi)*math.Sin(decl) + math.Cos(phi)*math.Cos(decl)*math.Sin(ws))

	// Clear-sky radiation (eq. 37), net shortwave (eq. 38), net longwave
	// (eq. 39), net radiation (eq. 40).
	rso := (0.75 + 2e-5*elevM) * ra
	rns := (1 - 0.23) * rsMJ
	tMaxK := tMaxC + 273.16
	tMinK := tMinC + 273.16
	rnl := 4.903e-9 * ((math.Pow(tMaxK, 4) + math.Pow(tMinK, 4)) / 2) *
		(0.34 - 0.14*math.Sqrt(ea)) * (1.35*rsMJ/rso - 0.35)
	rn := rns - rnl

	// Penman-Monteith reference ET (eq. 6), G = 0 for a daily step.
	num := 0.408*delta*rn + gamma*(900/(tMean+273))*u2*(es-ea)
	den := delta + gamma*(1+0.34*u2)
	et0 := num / den
	if et0 < 0 {
		return 0
	}
	return et0
}

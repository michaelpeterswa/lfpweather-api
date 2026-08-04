package meteo

import (
	"math"
	"time"
)

// Wet Bulb Globe Temperature by the Liljegren model: a faithful port of the
// Argonne National Laboratory reference C implementation (WBGT v1.1, James C.
// Liljegren), via the MIT-licensed mdljts/wbgt packaging. The model predicts
// the black-globe and natural-wet-bulb temperatures by energy balance from
// standard weather, then combines them as the outdoor WBGT.
//
// Reference: Liljegren, J.C., Carhart, R.A., Lawday, P., Tschopp, S., Sharp, R.
// (2008), "Modeling the Wet Bulb Globe Temperature Using Standard Meteorological
// Measurements", Journal of Occupational and Environmental Hygiene 5(10):645-655.
// Solar position after Larson (Pacific Northwest National Laboratory).

// Physical constants and instrument parameters, from the reference wbgt.h.
const (
	wbgtSolarConst  = 1367.0
	wbgtStefanB     = 5.6696e-8
	wbgtCp          = 1003.5
	wbgtMAir        = 28.97
	wbgtMH2O        = 18.015
	wbgtRGas        = 8314.34
	wbgtRAir        = wbgtRGas / wbgtMAir
	wbgtRatio       = wbgtCp * wbgtMAir / wbgtMH2O
	wbgtPr          = wbgtCp / (wbgtCp + 1.25*wbgtRAir)
	wbgtEmisWick    = 0.95
	wbgtAlbWick     = 0.4
	wbgtDWick       = 0.007
	wbgtLWick       = 0.0254
	wbgtEmisGlobe   = 0.95
	wbgtAlbGlobe    = 0.05
	wbgtDGlobe      = 0.0508
	wbgtEmisSfc     = 0.999
	wbgtAlbSfc      = 0.45
	wbgtCzaMin      = 0.00873
	wbgtNormSolar   = 0.85
	wbgtMinSpeed    = 0.13
	wbgtConvergence = 0.02
	wbgtMaxIter     = 500
	wbgtDegRad      = math.Pi / 180
	wbgtRadDeg      = 180 / math.Pi
	wbgtTwoPi       = 2 * math.Pi
)

// WBGTResult holds the outdoor WBGT and its component temperatures (deg C).
type WBGTResult struct {
	WBGT                 float64
	Globe                float64
	NaturalWetBulb       float64
	PsychrometricWetBulb float64
	OK                   bool // false if a solver failed to converge
}

// WBGT computes the outdoor wet bulb globe temperature for the given UTC time
// and location from air temperature (deg C), relative humidity (%), sea-level
// pressure (hPa/mb), solar irradiance (W/m^2), and the wind speed already
// adjusted to the 2 m reference height (m/s).
func WBGT(utc time.Time, latDeg, lonDeg, tairC, relhumPct, presMb, solarWm2, speed2m float64) WBGTResult {
	// gmt = 0 and avg = 0 because the time is already UTC.
	hourGmt := float64(utc.Hour()) + float64(utc.Minute())/60.0
	dday := float64(utc.Day()) + hourGmt/24.0

	solar, cza, fdir := calcSolarParameters(utc.Year(), int(utc.Month()), dday, latDeg, lonDeg, solarWm2)

	tk := tairC + 273.15
	rh := 0.01 * relhumPct

	tg := tGlobe(tk, rh, presMb, speed2m, solar, fdir, cza)
	tnwb := twb(tk, rh, presMb, speed2m, solar, fdir, cza, 1)
	tpsy := twb(tk, rh, presMb, speed2m, solar, fdir, cza, 0)

	if tg == -9999 || tnwb == -9999 {
		return WBGTResult{OK: false}
	}
	return WBGTResult{
		WBGT:                 0.1*tairC + 0.2*tg + 0.7*tnwb,
		Globe:                tg,
		NaturalWetBulb:       tnwb,
		PsychrometricWetBulb: tpsy,
		OK:                   true,
	}
}

// esat is the saturation vapour pressure (mb) over liquid water (phase 0) or ice
// (phase 1), Buck (1981).
func esat(tk float64, phase int) float64 {
	var es, y float64
	if phase == 0 {
		y = (tk - 273.15) / (tk - 32.18)
		es = 6.1121 * math.Exp(17.502*y)
	} else {
		y = (tk - 273.15) / (tk - 0.6)
		es = 6.1115 * math.Exp(22.452*y)
	}
	return 1.004 * es
}

// dewPoint is the dew point (phase 0) or frost point (phase 1) temperature (K).
func dewPoint(e float64, phase int) float64 {
	if phase == 0 {
		z := math.Log(e / (6.1121 * 1.004))
		return 273.15 + 240.97*z/(17.502-z)
	}
	z := math.Log(e / (6.1115 * 1.004))
	return 273.15 + 272.55*z/(22.452-z)
}

// viscosity of air, kg/(m s), BSL page 23.
func viscosity(tair float64) float64 {
	const sigma = 3.617
	const epsKappa = 97.0
	tr := tair / epsKappa
	omega := (tr-2.9)/0.4*(-0.034) + 1.048
	return 2.6693e-6 * math.Sqrt(wbgtMAir*tair) / (sigma * sigma * omega)
}

// thermalCond of air, W/(m K), BSL page 257.
func thermalCond(tair float64) float64 {
	return (wbgtCp + 1.25*wbgtRAir) * viscosity(tair)
}

// diffusivity of water vapour in air, m^2/s, BSL page 505.
func diffusivity(tair, pair float64) float64 {
	const pcritAir = 36.4
	const pcritH2O = 218.0
	const tcritAir = 132.0
	const tcritH2O = 647.3
	const a = 3.640e-4
	const b = 2.334
	pcrit13 := math.Pow(pcritAir*pcritH2O, 1.0/3.0)
	tcrit512 := math.Pow(tcritAir*tcritH2O, 5.0/12.0)
	tcrit12 := math.Sqrt(tcritAir * tcritH2O)
	mmix := math.Sqrt(1.0/wbgtMAir + 1.0/wbgtMH2O)
	patm := pair / 1013.25
	return a * math.Pow(tair/tcrit12, b) * pcrit13 * tcrit512 * mmix / patm * 1e-4
}

// evap is the heat of evaporation, J/(kg K), for 283-313 K.
func evap(tair float64) float64 {
	return (313.15-tair)/30.0*(-71100.0) + 2.4073e6
}

// emisAtm is the atmospheric emissivity, Oke (2nd ed.) page 373.
func emisAtm(tair, rh float64) float64 {
	e := rh * esat(tair, 0)
	return 0.575 * math.Pow(e, 0.143)
}

// hCylinderInAir is the convective heat transfer coefficient, W/(m2 K), for a
// long cylinder in cross flow (Bedingfield and Drew, eqn 32). length is
// unused, kept to match the reference signature.
func hCylinderInAir(diameter, length, tair, pair, speed float64) float64 {
	_ = length
	const a = 0.56
	const b = 0.281
	const c = 0.4
	density := pair * 100.0 / (wbgtRAir * tair)
	re := math.Max(speed, wbgtMinSpeed) * density * diameter / viscosity(tair)
	nu := b * math.Pow(re, 1.0-c) * math.Pow(wbgtPr, 1.0-a)
	return nu * thermalCond(tair) / diameter
}

// hSphereInAir is the convective heat transfer coefficient, W/(m2 K), for flow
// around a sphere (Bird, Stewart, Lightfoot, page 409).
func hSphereInAir(diameter, tair, pair, speed float64) float64 {
	density := pair * 100.0 / (wbgtRAir * tair)
	re := math.Max(speed, wbgtMinSpeed) * density * diameter / viscosity(tair)
	nu := 2.0 + 0.6*math.Sqrt(re)*math.Pow(wbgtPr, 0.3333)
	return nu * thermalCond(tair) / diameter
}

// tGlobe iteratively solves the black-globe energy balance and returns the globe
// temperature (deg C), or -9999 on non-convergence.
func tGlobe(tair, rh, pair, speed, solar, fdir, cza float64) float64 {
	tsfc := tair
	tglobePrev := tair
	for iter := 0; iter < wbgtMaxIter; iter++ {
		tref := 0.5 * (tglobePrev + tair)
		h := hSphereInAir(wbgtDGlobe, tref, pair, speed)
		tglobeNew := math.Pow(
			0.5*(emisAtm(tair, rh)*math.Pow(tair, 4)+wbgtEmisSfc*math.Pow(tsfc, 4))-
				h/(wbgtStefanB*wbgtEmisGlobe)*(tglobePrev-tair)+
				solar/(2.0*wbgtStefanB*wbgtEmisGlobe)*(1.0-wbgtAlbGlobe)*(fdir*(1.0/(2.0*cza)-1.0)+1.0+wbgtAlbSfc),
			0.25)
		if math.Abs(tglobeNew-tglobePrev) < wbgtConvergence {
			return tglobeNew - 273.15
		}
		tglobePrev = 0.9*tglobePrev + 0.1*tglobeNew
	}
	return -9999
}

// twb iteratively solves the wetted-wick energy balance and returns the natural
// wet bulb temperature (rad = 1) or the psychrometric wet bulb (rad = 0), in
// deg C, or -9999 on non-convergence.
func twb(tair, rh, pair, speed, solar, fdir, cza float64, rad int) float64 {
	const a = 0.56 // Bedingfield and Drew
	tsfc := tair
	sza := math.Acos(cza)
	eair := rh * esat(tair, 0)
	twbPrev := dewPoint(eair, 0)
	for iter := 0; iter < wbgtMaxIter; iter++ {
		tref := 0.5 * (twbPrev + tair)
		h := hCylinderInAir(wbgtDWick, wbgtLWick, tref, pair, speed)
		fatm := wbgtStefanB*wbgtEmisWick*
			(0.5*(emisAtm(tair, rh)*math.Pow(tair, 4)+wbgtEmisSfc*math.Pow(tsfc, 4))-math.Pow(twbPrev, 4)) +
			(1.0-wbgtAlbWick)*solar*
				((1.0-fdir)*(1.0+0.25*wbgtDWick/wbgtLWick)+fdir*((math.Tan(sza)/math.Pi)+0.25*wbgtDWick/wbgtLWick)+wbgtAlbSfc)
		ewick := esat(twbPrev, 0)
		density := pair * 100.0 / (wbgtRAir * tref)
		sc := viscosity(tref) / (density * diffusivity(tref, pair))
		twbNew := tair - evap(tref)/wbgtRatio*(ewick-eair)/(pair-ewick)*math.Pow(wbgtPr/sc, a) + (fatm/h)*float64(rad)
		if math.Abs(twbNew-twbPrev) < wbgtConvergence {
			return twbNew - 273.15
		}
		twbPrev = 0.9*twbPrev + 0.1*twbNew
	}
	return -9999
}

// calcSolarParameters returns the (possibly bounded) solar irradiance, the
// cosine of the solar zenith angle, and the direct-beam fraction.
func calcSolarParameters(year, month int, day, lat, lon, solar float64) (adjSolar, cza, fdir float64) {
	elev, soldist := solarPosition(year, month, day, lat, lon)
	cza = math.Cos((90.0 - elev) * wbgtDegRad)
	toasolar := wbgtSolarConst * math.Max(0.0, cza) / (soldist * soldist)
	if cza < wbgtCzaMin {
		toasolar = 0.0
	}
	adjSolar = solar
	if toasolar > 0.0 {
		normsolar := math.Min(solar/toasolar, wbgtNormSolar)
		adjSolar = normsolar * toasolar
		if normsolar > 0.0 {
			fdir = math.Exp(3.0 - 1.34*normsolar - 1.65/normsolar)
			fdir = math.Max(math.Min(fdir, 0.9), 0.0)
		}
	}
	return adjSolar, cza, fdir
}

// daynum is the sequential day number within a Gregorian year.
func daynum(year, month, day int) int {
	begmonth := [13]int{0, 0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	if year < 1 {
		return -1
	}
	leap := 0
	if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
		leap = 1
	}
	d := begmonth[month] + day
	if leap == 1 && month > 2 {
		d++
	}
	return d
}

// solarPosition returns the Sun's altitude above the horizon (degrees, including
// refraction) and its distance (AU), using the low-precision formulae of the
// 1990 Astronomical Almanac. Date is year, month, and fractional day (UT).
func solarPosition(year, month int, day, latitude, longitude float64) (altitude, distance float64) {
	// math.Modf returns (integer, fractional); C's modf returns the fractional
	// part, which is what this needs.
	frac := func(x float64) float64 { _, f := math.Modf(x); return f }

	daynumber := daynum(year, month, int(day))
	deltaYears := year - 2000
	deltaDays := deltaYears*365 + deltaYears/4 + daynumber
	if year > 2000 {
		deltaDays++
	}
	daysJ2000 := float64(deltaDays) - 1.5
	centJ2000 := daysJ2000 / 36525.0

	utFrac := frac(day)
	daysJ2000 += utFrac
	ut := utFrac * 24.0

	meanAnomaly := frac((357.528+0.9856003*daysJ2000)/360.0) * wbgtTwoPi
	meanLongitude := frac((280.460+0.9856474*daysJ2000)/360.0) * wbgtTwoPi
	meanObliquity := (23.439 - 4.0e-7*daysJ2000) * wbgtDegRad
	eclipticLong := (1.915*math.Sin(meanAnomaly)+0.020*math.Sin(2.0*meanAnomaly))*wbgtDegRad + meanLongitude

	distance = 1.00014 - 0.01671*math.Cos(meanAnomaly) - 0.00014*math.Cos(2.0*meanAnomaly)

	apRa := math.Atan2(math.Cos(meanObliquity)*math.Sin(eclipticLong), math.Cos(eclipticLong))
	if apRa < 0 {
		apRa += wbgtTwoPi
	}
	apRa = frac(apRa/wbgtTwoPi) * 24.0
	apDec := math.Asin(math.Sin(meanObliquity) * math.Sin(eclipticLong))

	gmst0h := 24110.54841 + centJ2000*(8640184.812866+centJ2000*(0.093104-centJ2000*6.2e-6))
	gmst0h = frac(gmst0h/3600.0/24.0) * 24.0
	if gmst0h < 0 {
		gmst0h += 24.0
	}
	lmst := gmst0h + ut*1.00273790934 + longitude/15.0
	lmst = frac(lmst/24.0) * 24.0
	if lmst < 0 {
		lmst += 24.0
	}
	localHa := lmst - apRa
	if localHa < -12.0 {
		localHa += 24.0
	} else if localHa > 12.0 {
		localHa -= 24.0
	}

	latRad := latitude * wbgtDegRad
	localHaRad := localHa / 24.0 * wbgtTwoPi
	cosApdec := math.Cos(apDec)
	sinApdec := math.Sin(apDec)
	alt := math.Asin(sinApdec*math.Sin(latRad) + cosApdec*math.Cos(localHaRad)*math.Cos(latRad))

	var tanAlt float64
	if math.Abs(alt) < 1.57079615 {
		tanAlt = math.Tan(alt)
	} else {
		tanAlt = 6.0e6
	}
	altDeg := alt * wbgtRadDeg

	const pressure = 1013.25
	const temp = 15.0
	var refraction float64
	switch {
	case altDeg < -1.0 || tanAlt == 6.0e6:
		refraction = 0.0
	case altDeg < 19.225:
		refraction = (0.1594 + altDeg*(0.0196+0.00002*altDeg)) * pressure
		refraction /= (1.0 + altDeg*(0.505+0.0845*altDeg)) * (273.0 + temp)
	default:
		refraction = 0.00452 * (pressure / (273.0 + temp)) / tanAlt
	}
	return altDeg + refraction, distance
}

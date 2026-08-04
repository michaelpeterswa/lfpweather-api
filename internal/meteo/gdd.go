// Package meteo computes derived weather metrics from standard observations.
//
// Every function is pure and takes explicit units in its signature, following
// the discipline of the firewx library: no hidden state, the unit named at the
// boundary, and a cited reference per algorithm. The lfpweather-api handlers
// read raw rows from sensors.vantagepro2plus and pass them here; nothing in this
// package touches a database.
package meteo

// GrowingDegreeDaysF returns the growing degree days accumulated on one day from
// that day's maximum and minimum air temperature and a base temperature, all in
// degrees Fahrenheit. It uses the simple-average method:
//
//	GDD = max(0, (Tmax+Tmin)/2 - Tbase)
//
// The base is the temperature below which development effectively stops; 50 F is
// the common agronomic and pest-phenology default. This is the unmodified form:
// it does not clamp Tmin up to the base or Tmax down to an upper threshold, so
// it is the right choice for a temperate site that rarely reaches a cap.
//
// Reference: McMaster, G.S. and Wilhelm, W.W. (1997), "Growing degree-days: one
// equation, two interpretations", Agricultural and Forest Meteorology
// 87(4):291-300.
func GrowingDegreeDaysF(tMaxF, tMinF, baseF float64) float64 {
	gdd := (tMaxF+tMinF)/2 - baseF
	if gdd < 0 {
		return 0
	}
	return gdd
}

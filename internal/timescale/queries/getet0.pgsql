-- Daily aggregates needed for FAO-56 reference evapotranspiration, per local day
-- since the start of the current calendar year, in the site timezone.
-- Temperature is degrees F, humidity is percent, wind is mph, and solar is the
-- 24-hour mean W/m^2 (the handler multiplies it out to the day's total MJ/m^2).
-- The handler converts units and computes ETo per day.
SELECT
    date_trunc('day', "time" AT TIME ZONE 'America/Los_Angeles') AS day,
    MAX(temperature) AS tmax_f,
    MIN(temperature) AS tmin_f,
    MAX(humidity) AS rh_max,
    MIN(humidity) AS rh_min,
    AVG(wind_speed_last) AS wind_mph,
    AVG(solar_radiation) AS solar_wm2
FROM sensors.vantagepro2plus
WHERE "time" >= date_trunc('year', now() AT TIME ZONE 'America/Los_Angeles')
                AT TIME ZONE 'America/Los_Angeles'
GROUP BY 1
ORDER BY 1;

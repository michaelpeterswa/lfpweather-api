-- Daily maximum and minimum air temperature (degrees F) for each local day
-- since the start of the current calendar year, in the site timezone. The
-- handler turns each day's max/min into growing degree days and accumulates
-- them. The day boundary and the year start both use America/Los_Angeles so a
-- "day" is a civil day at the station, not a UTC day.
SELECT
    date_trunc('day', "time" AT TIME ZONE 'America/Los_Angeles') AS day,
    MIN(temperature) AS tmin,
    MAX(temperature) AS tmax
FROM sensors.vantagepro2plus
WHERE "time" >= date_trunc('year', now() AT TIME ZONE 'America/Los_Angeles')
                AT TIME ZONE 'America/Los_Angeles'
GROUP BY 1
ORDER BY 1;

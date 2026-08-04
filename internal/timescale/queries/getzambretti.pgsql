-- Current sea-level pressure and wind direction, plus the sea-level pressure
-- about three hours earlier, for the Zambretti forecaster. Pressure is in inHg
-- (Davis native); the handler converts to hPa and derives the trend. p_3h is
-- NULL when less than three hours of history exists, which the handler treats
-- as a steady trend.
WITH latest AS (
    SELECT "time", barometer_sea_level AS p, wind_dir_avg_last_10_min AS wind
    FROM sensors.vantagepro2plus
    ORDER BY "time" DESC
    LIMIT 1
),
past AS (
    SELECT v.barometer_sea_level AS p
    FROM sensors.vantagepro2plus v, latest l
    WHERE v."time" <= l."time" - INTERVAL '3 hours'
    ORDER BY v."time" DESC
    LIMIT 1
)
SELECT l."time", l.p, l.wind, (SELECT p FROM past) AS p_3h
FROM latest l;

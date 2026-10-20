-- All rows of the newest run (max issued_at) per source, limited to runs issued
-- within the last $1 (seconds). Column order matches the
-- forecastRow scan target.
SELECT f.issued_at, f.source, f.valid_at, f.lead_hours, f.temperature, f.quantiles, f.model
FROM sensors.temperature_forecast f
JOIN (
    SELECT source, max(issued_at) AS issued_at
    FROM sensors.temperature_forecast
    WHERE issued_at >= now() - make_interval(secs => $1::float8)
    GROUP BY source
) latest ON latest.source = f.source AND latest.issued_at = f.issued_at
ORDER BY f.source, f.valid_at;

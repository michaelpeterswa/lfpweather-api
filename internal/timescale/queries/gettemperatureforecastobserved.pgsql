-- Hourly observed temperature between $1 and $2 (the 12 hours up to the
-- forecast's issued_at).
SELECT valid_at, temperature
FROM sensors.temperature_hourly($1::timestamptz, $2::timestamptz)
ORDER BY valid_at;

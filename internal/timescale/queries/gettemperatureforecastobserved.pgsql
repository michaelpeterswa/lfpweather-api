-- Hourly observed temperature for the 12 hours up to $1.
SELECT valid_at, temperature
FROM sensors.temperature_hourly($1::timestamptz - interval '12 hours', $1::timestamptz)
ORDER BY valid_at;

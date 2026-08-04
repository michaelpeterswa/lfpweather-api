-- The latest observation for the wet bulb globe temperature: air temperature
-- (deg F), humidity (%), sea-level pressure (inHg), solar irradiance (W/m^2),
-- and wind speed (mph). The handler converts units, adjusts wind to 2 m, and
-- runs the Liljegren model with the observation time and site location.
SELECT "time", temperature, humidity, barometer_sea_level, solar_radiation, wind_speed_last
FROM sensors.vantagepro2plus
ORDER BY "time" DESC
LIMIT 1;

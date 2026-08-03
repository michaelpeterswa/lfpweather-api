-- Fire danger summary: the latest non-spin-up reading for the station that
-- reported most recently, plus that station's fire-season ERC/BI percentile
-- context. The percentiles are recomputed on every call, so the danger
-- classification self-calibrates as the record grows.
--
-- The season window is months 5-10 (May-October). Fire-danger thresholds are
-- set on the fire-season subset, not the whole year, so the wet winter does not
-- drag the percentiles down.
WITH latest AS (
    SELECT *
    FROM sensors.fire_danger
    WHERE NOT spinup
    ORDER BY "time" DESC
    LIMIT 1
),
season AS (
    SELECT
        percentile_cont(0.50) WITHIN GROUP (ORDER BY f.energy_release_component) AS erc_p50,
        percentile_cont(0.80) WITHIN GROUP (ORDER BY f.energy_release_component) AS erc_p80,
        percentile_cont(0.90) WITHIN GROUP (ORDER BY f.energy_release_component) AS erc_p90,
        percentile_cont(0.97) WITHIN GROUP (ORDER BY f.energy_release_component) AS erc_p97,
        MAX(f.energy_release_component) AS erc_max,
        percentile_cont(0.90) WITHIN GROUP (ORDER BY f.burning_index) AS bi_p90,
        percentile_cont(0.97) WITHIN GROUP (ORDER BY f.burning_index) AS bi_p97,
        AVG(CASE WHEN f.energy_release_component <= (SELECT energy_release_component FROM latest)
                 THEN 1.0 ELSE 0.0 END)::float8 AS erc_pct
    FROM sensors.fire_danger f, latest l
    WHERE NOT f.spinup
        AND f.device_id = l.device_id
        AND f.fuel_model = l.fuel_model
        AND EXTRACT(MONTH FROM f."time") BETWEEN 5 AND 10
)
SELECT
    l."time", l.device_id, l.fuel_model,
    l.energy_release_component, l.burning_index, l.spread_component, l.ignition_component,
    l.kbdi, l.gsi,
    l.dead_fuel_moisture_1hr, l.dead_fuel_moisture_10hr,
    l.dead_fuel_moisture_100hr, l.dead_fuel_moisture_1000hr,
    l.live_fuel_moisture_herbaceous, l.live_fuel_moisture_woody,
    s.erc_p50, s.erc_p80, s.erc_p90, s.erc_p97, s.erc_max, s.erc_pct,
    s.bi_p90, s.bi_p97
FROM latest l, season s;

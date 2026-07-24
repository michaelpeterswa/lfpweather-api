package timescale

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// airgradientSerial is the single AirGradient unit whose readings are exposed.
const airgradientSerial = "84fce6070dd4"

// metricDef is a resolved, ready-to-query metric: a concrete table + column
// (column empty for count metrics) plus the flags needed to build the SQL.
type metricDef struct {
	Table          string
	Column         string
	Type           MetricType
	SerialFiltered bool
}

// tableDef is the static, per-table policy. The set of tables is a fixed
// allowlist — callers can never reach a table outside this map — while the
// columns within each table are discovered at runtime (see Catalog).
type tableDef struct {
	Type           MetricType
	SerialFiltered bool
}

// tableRegistry is the table allowlist. A query may only ever touch a table
// named here, and only its introspected columns.
var tableRegistry = map[string]tableDef{
	"vantagepro2plus":        {Type: MetricTypeGauge},
	"airgradient":            {Type: MetricTypeGauge, SerialFiltered: true},
	"airgradient_aqi":        {Type: MetricTypeGauge, SerialFiltered: true},
	"litime":                 {Type: MetricTypeGauge},
	"renogychargecontroller": {Type: MetricTypeGauge},
	"birdnet":                {Type: MetricTypeCount},
}

// aliasRegistry gives friendly names to the most commonly queried columns so
// callers can say "pressure" instead of "vantagepro2plus.barometer_sea_level".
// Any other column is reachable via a "table.column" reference.
var aliasRegistry = map[string]struct{ Table, Column string }{
	"temperature":     {"vantagepro2plus", "temperature"},
	"humidity":        {"vantagepro2plus", "humidity"},
	"pressure":        {"vantagepro2plus", "barometer_sea_level"},
	"solar_radiation": {"vantagepro2plus", "solar_radiation"},
	"wind_speed":      {"vantagepro2plus", "wind_speed_last"},
	"wind_gust":       {"vantagepro2plus", "wind_speed_high_last_10_min"},
	"rain_rate":       {"vantagepro2plus", "rain_rate_last"},
	"rain_24h":        {"vantagepro2plus", "rain_last_24_hour"},
	"uv_index":        {"vantagepro2plus", "uv_index"},
	"aqi":             {"airgradient_aqi", "aqi"},
	"co2":             {"airgradient", "rco2"},
	"nox_index":       {"airgradient", "nox_index"},
	"tvoc_index":      {"airgradient", "tvoc_index"},
}

// numericTypes are the information_schema data types a gauge aggregation can be
// applied to. Text/boolean/jsonb/array columns are excluded from gauge stats.
var numericTypes = map[string]bool{
	"smallint":         true,
	"integer":          true,
	"bigint":           true,
	"numeric":          true,
	"real":             true,
	"double precision": true,
}

// ColumnInfo is one introspected column.
type ColumnInfo struct {
	DataType string
	Numeric  bool
}

// Catalog is the live column set of the allowlisted tables, discovered from
// information_schema at startup. It is what makes "any column" safe: a
// requested column must match an introspected column before it is ever
// interpolated into SQL.
type Catalog struct {
	Tables map[string]map[string]ColumnInfo
}

func (c Catalog) column(table, col string) (ColumnInfo, bool) {
	cols, ok := c.Tables[table]
	if !ok {
		return ColumnInfo{}, false
	}
	ci, ok := cols[col]
	return ci, ok
}

func tableNamesSlice() []string {
	names := make([]string, 0, len(tableRegistry))
	for n := range tableRegistry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func tableNames() string {
	return strings.Join(tableNamesSlice(), ", ")
}

// resolveMetric turns a caller-supplied metric string into a concrete metricDef.
// It accepts a count-table name (e.g. "birdnet"), a friendly gauge alias, or a
// "table.column" reference, validating every part against the allowlist and the
// live catalog.
func resolveMetric(metric string, catalog Catalog) (metricDef, error) {
	if metric == "" {
		return metricDef{}, verr("metric is required")
	}

	// Count-table name (e.g. birdnet).
	if td, ok := tableRegistry[metric]; ok && td.Type == MetricTypeCount {
		return metricDef{Table: metric, Type: MetricTypeCount}, nil
	}

	// Friendly gauge alias.
	if a, ok := aliasRegistry[metric]; ok {
		return newGaugeDef(a.Table, a.Column, catalog)
	}

	// "table.column" reference.
	if table, col, ok := strings.Cut(metric, "."); ok {
		td, ok := tableRegistry[table]
		if !ok {
			return metricDef{}, verr("unknown table %q (allowed: %s)", table, tableNames())
		}
		if td.Type != MetricTypeGauge {
			return metricDef{}, verr("table %q is not a gauge table; query it by name for counts", table)
		}
		return newGaugeDef(table, col, catalog)
	}

	return metricDef{}, verr("unknown metric %q; use a known alias, a count table (%s), or a table.column reference", metric, tableNames())
}

func newGaugeDef(table, col string, catalog Catalog) (metricDef, error) {
	ci, ok := catalog.column(table, col)
	if !ok {
		return metricDef{}, verr("unknown column %q in table %q", col, table)
	}
	if !ci.Numeric {
		return metricDef{}, verr("column %q is %s, not numeric; only numeric columns can be aggregated as a gauge", col, ci.DataType)
	}
	return metricDef{
		Table:          table,
		Column:         col,
		Type:           MetricTypeGauge,
		SerialFiltered: tableRegistry[table].SerialFiltered,
	}, nil
}

// IntrospectCatalog reads the columns of the allowlisted tables from
// information_schema. Call once at startup; the result is passed to PlanQuery.
func (c *TimescaleClient) IntrospectCatalog(ctx context.Context) (Catalog, error) {
	rows, err := c.Pool.Query(ctx,
		`SELECT table_name, column_name, data_type
		 FROM information_schema.columns
		 WHERE table_schema = 'sensors' AND table_name = ANY($1)`,
		tableNamesSlice(),
	)
	if err != nil {
		return Catalog{}, fmt.Errorf("introspect catalog: %w", err)
	}
	defer rows.Close()

	cat := Catalog{Tables: map[string]map[string]ColumnInfo{}}
	for rows.Next() {
		var table, column, dataType string
		if err := rows.Scan(&table, &column, &dataType); err != nil {
			return Catalog{}, fmt.Errorf("scan catalog row: %w", err)
		}
		if cat.Tables[table] == nil {
			cat.Tables[table] = map[string]ColumnInfo{}
		}
		cat.Tables[table][column] = ColumnInfo{DataType: dataType, Numeric: numericTypes[dataType]}
	}
	if err := rows.Err(); err != nil {
		return Catalog{}, fmt.Errorf("iterate catalog rows: %w", err)
	}

	return cat, nil
}

// FieldInfo describes one queryable column in the discovery response.
type FieldInfo struct {
	Name     string `json:"name"`
	DataType string `json:"data_type"`
	Numeric  bool   `json:"numeric"`
}

// TableFields is one table's entry in the discovery response.
type TableFields struct {
	Type    string      `json:"type"`
	Columns []FieldInfo `json:"columns"`
}

// FieldsResponse is the payload of GET /api/v1/query/fields: every queryable
// table, its columns and types, and the friendly aliases.
type FieldsResponse struct {
	Tables  map[string]TableFields `json:"tables"`
	Aliases map[string]string      `json:"aliases"`
}

// Describe renders the catalog + registries as the discovery response.
func (c Catalog) Describe() FieldsResponse {
	tables := make(map[string]TableFields, len(tableRegistry))
	for name, td := range tableRegistry {
		var fields []FieldInfo
		for col, ci := range c.Tables[name] {
			fields = append(fields, FieldInfo{Name: col, DataType: ci.DataType, Numeric: ci.Numeric})
		}
		sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
		tables[name] = TableFields{Type: string(td.Type), Columns: fields}
	}

	aliases := make(map[string]string, len(aliasRegistry))
	for a, tc := range aliasRegistry {
		aliases[a] = tc.Table + "." + tc.Column
	}

	return FieldsResponse{Tables: tables, Aliases: aliases}
}

package timescale

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/cespare/xxhash/v2"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/michaelpeterswa/lfpweather-api/internal/dragonfly"
	"github.com/redis/go-redis/v9"
)

type TimescaleClient struct {
	Pool              *pgxpool.Pool
	Dfly              *dragonfly.DragonflyClient
	getColumnTemplate *template.Template
}

//go:embed queries/getcolumn.pgsql.gotmpl
var getColumnTemplate string

type TemplateParameters struct {
	ColumnName       string
	TimeBucket       string
	LookbackInterval string
}

func (t *TemplateParameters) String() string {
	return fmt.Sprintf("%s-%s-%s",
		strings.ReplaceAll(t.ColumnName, " ", ""),
		strings.ReplaceAll(t.TimeBucket, " ", ""),
		strings.ReplaceAll(t.LookbackInterval, " ", ""))
}

func (t *TemplateParameters) Hash() string {
	return strconv.FormatUint(xxhash.Sum64String(t.String()), 16)
}

type TimescaleClientOption func(*TimescaleClient)

func WithDragonflyClient(dfly *dragonfly.DragonflyClient) TimescaleClientOption {
	return func(c *TimescaleClient) {
		c.Dfly = dfly
	}
}

func NewTimescaleClient(ctx context.Context, connString string, opts ...TimescaleClientOption) (*TimescaleClient, error) {
	timescaleClient := &TimescaleClient{}

	tmpl, err := template.New("getColumn").Parse(getColumnTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse query template: %w", err)
	}

	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	cfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(pingCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	cancel()

	for _, opt := range opts {
		opt(timescaleClient)
	}

	timescaleClient.Pool = pool
	timescaleClient.getColumnTemplate = tmpl

	return timescaleClient, nil
}

func (c *TimescaleClient) Close() {
	c.Pool.Close()
}

func (c *TimescaleClient) GetColumn(ctx context.Context, tp TemplateParameters) ([]WeatherRow, error) {
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash())).Result()
		if err == nil {
			var weatherRows []WeatherRow
			err := json.Unmarshal([]byte(res), &weatherRows)
			if err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			}
			return weatherRows, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	query := bytes.NewBuffer(nil)
	err := c.getColumnTemplate.Execute(query, tp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query template: %w", err)
	}

	rows, err := c.Pool.Query(ctx, query.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s for the last %s: %w", tp.ColumnName, tp.LookbackInterval, err)
	}
	defer rows.Close()

	var weatherRows []WeatherRow

	for rows.Next() {
		var row WeatherRow
		err := rows.Scan(&row.Time, &row.Avg, &row.Min, &row.Max)
		if err != nil {
			slog.Error("failed to scan row", slog.String("error", err.Error()))
			continue
		}

		weatherRows = append(weatherRows, row)
	}

	if c.Dfly != nil {
		weatherRowsJSON, err := json.Marshal(weatherRows)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			err := c.Dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash()), weatherRowsJSON, c.Dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return weatherRows, nil
}

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
	Pool                  *pgxpool.Pool
	Dfly                  *dragonfly.DragonflyClient
	getColumnTemplate     *template.Template
	getColumnLastTemplate *template.Template
}

//go:embed queries/getcolumn.pgsql.gotmpl
var getColumnTemplate string

//go:embed queries/getcolumnlast.pgsql.gotmpl
var getColumnLastTemplate string

type GetColumnTemplateParameters struct {
	ColumnName       string
	TimeBucket       string
	LookbackInterval string
	TableName        string
}

func (t *GetColumnTemplateParameters) String() string {
	return fmt.Sprintf("%s-%s-%s-%s",
		strings.ReplaceAll(t.ColumnName, " ", ""),
		strings.ReplaceAll(t.TimeBucket, " ", ""),
		strings.ReplaceAll(t.LookbackInterval, " ", ""),
		strings.ReplaceAll(t.TableName, " ", ""))
}

type GetColumnLastTemplateParameters struct {
	ColumnName string
	TableName  string
}

func (t *GetColumnLastTemplateParameters) String() string {
	return fmt.Sprintf("%s-%s",
		strings.ReplaceAll(t.ColumnName, " ", ""),
		strings.ReplaceAll(t.TableName, " ", ""))
}

func (t *GetColumnTemplateParameters) Hash() string {
	return strconv.FormatUint(xxhash.Sum64String(t.String()), 16)
}

func (t *GetColumnLastTemplateParameters) Hash() string {
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

	getColumnTmpl, err := template.New("getColumn").Parse(getColumnTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse query template: %w", err)
	}

	getColumnLastTmpl, err := template.New("getColumnLast").Parse(getColumnLastTemplate)
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
	timescaleClient.getColumnTemplate = getColumnTmpl
	timescaleClient.getColumnLastTemplate = getColumnLastTmpl

	return timescaleClient, nil
}

func (c *TimescaleClient) Close() {
	c.Pool.Close()
}

func (c *TimescaleClient) GetColumn(ctx context.Context, tp GetColumnTemplateParameters) ([]GetColumnResponse, error) {
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash())).Result()
		if err == nil {
			var getColumnResponses []GetColumnResponse
			err := json.Unmarshal([]byte(res), &getColumnResponses)
			if err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			}
			return getColumnResponses, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	query := bytes.NewBuffer(nil)
	err := c.getColumnTemplate.Execute(query, tp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query template: %w", err)
	}

	slog.Debug("query", slog.String("query", query.String()))

	rows, err := c.Pool.Query(ctx, query.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s for the last %s: %w", tp.ColumnName, tp.LookbackInterval, err)
	}
	defer rows.Close()

	var getColumnResponses []GetColumnResponse

	for rows.Next() {
		var row GetColumnResponse
		err := rows.Scan(&row.Time, &row.Avg, &row.Min, &row.Max)
		if err != nil {
			slog.Error("failed to scan row", slog.String("error", err.Error()))
			continue
		}

		getColumnResponses = append(getColumnResponses, row)
	}

	if c.Dfly != nil {
		getColumnResponsesJSON, err := json.Marshal(getColumnResponses)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			err := c.Dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash()), getColumnResponsesJSON, c.Dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return getColumnResponses, nil
}

func (c *TimescaleClient) GetColumnLast(ctx context.Context, tp GetColumnLastTemplateParameters) (*GetColumnLastResponse, error) {
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash())).Result()
		if err == nil {
			var getColumnLastResponse GetColumnLastResponse
			err := json.Unmarshal([]byte(res), &getColumnLastResponse)
			if err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			}
			return &getColumnLastResponse, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	query := bytes.NewBuffer(nil)
	err := c.getColumnLastTemplate.Execute(query, tp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query template: %w", err)
	}

	slog.Debug("query", slog.String("query", query.String()))

	row := c.Pool.QueryRow(ctx, query.String())

	var getColumnLastResponse GetColumnLastResponse

	err = row.Scan(&getColumnLastResponse.Time, &getColumnLastResponse.Last)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %w", tp.ColumnName, err)
	}

	if c.Dfly != nil {
		getColumnLastResponseJSON, err := json.Marshal(getColumnLastResponse)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			err := c.Dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash()), getColumnLastResponseJSON, c.Dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return &getColumnLastResponse, nil
}

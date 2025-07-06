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

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"

	_ "embed"
)

type GetBirdnetTemplateParameters struct {
	LookbackInterval string
}

type GetBirdnetResponse struct {
	CommonName string `json:"common_name"`
	Count      int    `json:"count"`
}

func (t *GetBirdnetTemplateParameters) String() string {
	return fmt.Sprintf("birdnet-%s", strings.ReplaceAll(t.LookbackInterval, " ", ""))
}

func (t *GetBirdnetTemplateParameters) Hash() string {
	return strconv.FormatUint(xxhash.Sum64String(t.String()), 16)
}

//go:embed queries/getbirdnet.pgsql.gotmpl
var getBirdnetTemplate string

func (c *TimescaleClient) GetBirdnet(ctx context.Context, tp GetBirdnetTemplateParameters) ([]GetBirdnetResponse, error) {
	if c.Dfly != nil {
		res, err := c.Dfly.GetClient().Get(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash())).Result()
		if err == nil {
			var getBirdnetResponses []GetBirdnetResponse
			err := json.Unmarshal([]byte(res), &getBirdnetResponses)
			if err != nil {
				slog.Error("failed to unmarshal from dragonfly", slog.String("error", err.Error()))
			}
			return getBirdnetResponses, nil
		} else if !errors.Is(err, redis.Nil) {
			slog.Error("failed to get from dragonfly", slog.String("error", err.Error()))
		}
	}

	query := bytes.NewBuffer(nil)
	err := c.getBirdnetTemplate.Execute(query, tp)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query template: %w", err)
	}

	slog.Debug("query", slog.String("query", query.String()))

	rows, err := c.Pool.Query(ctx, query.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get birds for the last %s: %w", tp.LookbackInterval, err)
	}
	defer rows.Close()

	var getBirdnetResponses []GetBirdnetResponse

	for rows.Next() {
		var row GetBirdnetResponse
		err := rows.Scan(&row.CommonName, &row.Count)
		if err != nil {
			slog.Error("failed to scan row", slog.String("error", err.Error()))
			continue
		}

		getBirdnetResponses = append(getBirdnetResponses, row)
	}

	if c.Dfly != nil {
		getBirdnetResponsesJSON, err := json.Marshal(getBirdnetResponses)
		if err != nil {
			slog.Error("failed to marshal to dragonfly", slog.String("error", err.Error()))
		} else {
			err := c.Dfly.GetClient().Set(ctx, fmt.Sprintf("%s-%s", c.Dfly.KeyPrefix, tp.Hash()), getBirdnetResponsesJSON, c.Dfly.CacheResultsDuration).Err()
			if err != nil {
				slog.Error("failed to set to dragonfly", slog.String("error", err.Error()))
			}
		}
	}

	return getBirdnetResponses, nil
}

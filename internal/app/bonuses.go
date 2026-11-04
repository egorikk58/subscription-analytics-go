package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"subscription-analytics/internal/core"
)

var telegramDecimal = regexp.MustCompile(`^[0-9]{1,19}$`)

func parseTelegram(value string) (int64, error) {
	if !telegramDecimal.MatchString(value) {
		return 0, fmt.Errorf("invalid telegram_id")
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("telegram_id must be a positive int64")
	}
	return id, nil
}

func utmParameter(r *http.Request) (*string, error) {
	values, present := r.URL.Query()["utm_link"]
	if !present {
		return nil, nil
	}
	if len(values) != 1 || len(values[0]) > 2048 {
		return nil, fmt.Errorf("invalid utm_link")
	}
	value := core.NormalizeUTM(values[0])
	return &value, nil
}

// Reusable persistent response cache for the indexed lookup. The body also
// represents a negative lookup, so both 200 and 404 survive Redis failure.
func (s *service) cachedBonus(ctx context.Context, id int64, key string, build func() ([]byte, error)) ([]byte, error) {
	redisKey := fmt.Sprintf("snapshot:%d:%s", id, key)
	value, err, _ := s.flight.Do(redisKey, func() (any, error) {
		rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		body, cacheErr := s.redis.Get(rctx, redisKey).Bytes()
		cancel()
		s.obs.cacheResult("redis", cacheErr)
		if cacheErr == nil {
			return body, nil
		}
		cacheErr = s.db.QueryRow(ctx, "SELECT body FROM analytics.responses WHERE snapshot_id=$1 AND key=$2", id, key).Scan(&body)
		s.obs.cacheResult("postgres", cacheErr)
		if errors.Is(cacheErr, pgx.ErrNoRows) {
			defer s.obs.buildStarted()()
			body, cacheErr = build()
			if cacheErr == nil {
				_, cacheErr = s.db.Exec(ctx, "INSERT INTO analytics.responses VALUES($1,$2,$3) ON CONFLICT DO NOTHING", id, key, body)
			}
		}
		if cacheErr != nil {
			return nil, cacheErr
		}
		rctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
		_ = s.redis.Set(rctx, redisKey, body, 0).Err()
		cancel()
		return body, nil
	})
	if err != nil {
		return nil, err
	}
	return value.([]byte), nil
}

func (s *service) telegramLTV(w http.ResponseWriter, r *http.Request) {
	id, err := parseTelegram(r.PathValue("telegram_id"))
	if err != nil {
		write(w, 400, map[string]any{"error": err.Error(), "data_as_of": nil})
		return
	}
	var version *int64
	var asof *time.Time
	var format *string
	err = s.db.QueryRow(r.Context(), "SELECT current_id,(SELECT data_as_of FROM analytics.snapshots WHERE id=current_id),(SELECT data->>'format_version' FROM analytics.snapshots WHERE id=current_id) FROM analytics.state WHERE id=1").Scan(&version, &asof, &format)
	if err != nil || version == nil || format == nil || *format != "2" {
		w.Header().Set("Retry-After", "5")
		write(w, 503, map[string]any{"error": "snapshot unavailable", "data_as_of": nil})
		return
	}
	body, err := s.cachedBonus(r.Context(), *version, cacheKey("telegram:"+strconv.FormatInt(id, 10), core.Filter{}), func() ([]byte, error) {
		var body []byte
		err := s.db.QueryRow(r.Context(), "SELECT body FROM analytics.user_ltv WHERE snapshot_id=$1 AND telegram_id=$2", *version, id).Scan(&body)
		if errors.Is(err, pgx.ErrNoRows) {
			return json.Marshal(map[string]any{"error": "user not found", "data_as_of": asof})
		}
		return body, err
	})
	if err != nil {
		write(w, 503, map[string]any{"error": "cannot load user LTV", "data_as_of": asof})
		return
	}
	var result struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &result)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", fmt.Sprintf("\"%d\"", *version))
	w.Header().Set("Last-Modified", asof.UTC().Format(http.TimeFormat))
	if result.Error != "" {
		w.WriteHeader(404)
	} else {
		for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
			if strings.TrimPrefix(strings.TrimSpace(tag), "W/") == fmt.Sprintf("\"%d\"", *version) || strings.TrimSpace(tag) == "*" {
				w.WriteHeader(304)
				return
			}
		}
	}
	_, _ = w.Write(body)
}

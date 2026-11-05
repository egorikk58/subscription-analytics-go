package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"subscription-analytics/internal/core"
)

const responseWorkTimeout = 15 * time.Second
const redisCacheTimeout = 200 * time.Millisecond

func (s *service) coalesceResponse(waitCtx context.Context, key string, work func(context.Context) ([]byte, error)) ([]byte, error) {
	if err := waitCtx.Err(); err != nil {
		return nil, err
	}
	result := s.flight.DoChan(key, func() (any, error) {
		baseCtx := s.ctx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		// The result is shared. Its lifetime belongs to the service, while
		// each request independently decides how long to wait for it.
		ctx, cancel := context.WithTimeout(baseCtx, responseWorkTimeout)
		defer cancel()
		return work(ctx)
	})
	select {
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	case completed := <-result:
		if completed.Err != nil {
			return nil, completed.Err
		}
		return completed.Val.([]byte), nil
	}
}

func (s *service) cachedResponse(ctx context.Context, id int64, key string, build func(context.Context) ([]byte, error)) ([]byte, error) {
	redisKey := fmt.Sprintf("snapshot:%d:%s", id, key)
	return s.coalesceResponse(ctx, redisKey, func(ctx context.Context) ([]byte, error) {
		redisCtx, cancel := context.WithTimeout(ctx, redisCacheTimeout)
		body, err := s.redis.Get(redisCtx, redisKey).Bytes()
		cancel()
		s.obs.cacheResult("redis", err)
		if err == nil {
			return body, nil
		}
		err = s.db.QueryRow(ctx, "SELECT body FROM analytics.responses WHERE snapshot_id=$1 AND key=$2", id, key).Scan(&body)
		s.obs.cacheResult("postgres", err)
		if errors.Is(err, pgx.ErrNoRows) {
			defer s.obs.buildStarted()()
			body, err = build(ctx)
			if err == nil {
				_, err = s.db.Exec(ctx, "INSERT INTO analytics.responses VALUES($1,$2,$3) ON CONFLICT DO NOTHING", id, key, body)
			}
		}
		if err != nil {
			return nil, err
		}
		redisCtx, cancel = context.WithTimeout(ctx, redisCacheTimeout)
		_ = s.redis.Set(redisCtx, redisKey, body, 0).Err()
		cancel()
		return body, nil
	})
}

func (s *service) loadSnapshot(ctx context.Context, id int64) (core.Snapshot, error) {
	var snap core.Snapshot
	var raw []byte
	if err := s.db.QueryRow(ctx, "SELECT data FROM analytics.snapshots WHERE id=$1", id).Scan(&raw); err != nil {
		return snap, err
	}
	err := json.Unmarshal(raw, &snap)
	return snap, err
}

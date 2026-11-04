package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"subscription-analytics/internal/core"
)

func dashboardQuery(r *http.Request) (core.DashboardQuery, error) {
	values := r.URL.Query()
	q := core.DashboardQuery{Plan: values.Get("plan_id"), Selection: values.Get("selection"), Search: strings.TrimSpace(values.Get("search")), Sort: values.Get("sort"), Page: 1}
	var paramErr error
	q.UTMLink, paramErr = utmParameter(r)
	if paramErr != nil {
		return q, paramErr
	}
	if q.Selection == "" {
		q.Selection = "first_plan"
	}
	if q.Selection != "first_plan" && q.Selection != "chain" {
		return q, fmt.Errorf("invalid selection")
	}
	if q.Plan == "" {
		q.Plan = "all"
	}
	if q.Sort == "" {
		q.Sort = "ltv_desc"
	}
	if q.Sort != "ltv_desc" && q.Sort != "ltv_asc" && q.Sort != "joined_desc" {
		return q, fmt.Errorf("invalid sort")
	}
	if len(q.Search) > 100 {
		return q, fmt.Errorf("search too long")
	}
	if p := values.Get("page"); p != "" {
		var e error
		q.Page, e = strconv.Atoi(p)
		if e != nil || q.Page < 1 || q.Page > 1000000 {
			return q, fmt.Errorf("invalid page")
		}
	}
	for name, target := range map[string]**time.Time{"from": &q.From, "to": &q.To} {
		if raw := values.Get(name); raw != "" {
			t, e := time.Parse(time.RFC3339Nano, raw)
			if e != nil {
				return q, fmt.Errorf("invalid %s", name)
			}
			t = t.UTC()
			*target = &t
		}
	}
	if q.From != nil && q.To != nil && !q.To.After(*q.From) {
		return q, fmt.Errorf("invalid date range")
	}
	return q, nil
}

func (s *service) dashboard(w http.ResponseWriter, r *http.Request) {
	q, err := dashboardQuery(r)
	if err != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "data_as_of": nil})
		return
	}
	var id *int64
	var asof, next *time.Time
	if err = s.db.QueryRow(r.Context(), "SELECT current_id,(SELECT data_as_of FROM analytics.snapshots WHERE id=current_id),next_sync FROM analytics.state WHERE id=1").Scan(&id, &asof, &next); err != nil || id == nil {
		w.Header().Set("Retry-After", "5")
		write(w, http.StatusServiceUnavailable, map[string]any{"error": "snapshot unavailable", "data_as_of": nil})
		return
	}
	encoded, _ := json.Marshal(q)
	hash := sha256.Sum256(append([]byte("dashboard-v3:"), encoded...))
	key := fmt.Sprintf("%x", hash)
	redisKey := fmt.Sprintf("snapshot:%d:%s", *id, key)
	value, err, _ := s.flight.Do(redisKey, func() (any, error) {
		redisCtx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
		body, cacheErr := s.redis.Get(redisCtx, redisKey).Bytes()
		s.obs.cacheResult("redis", cacheErr)
		cancel()
		if cacheErr == nil {
			return body, nil
		}
		cacheErr = s.db.QueryRow(r.Context(), "SELECT body FROM analytics.responses WHERE snapshot_id=$1 AND key=$2", *id, key).Scan(&body)
		s.obs.cacheResult("postgres", cacheErr)
		if errors.Is(cacheErr, pgx.ErrNoRows) {
			defer s.obs.buildStarted()()
			var raw []byte
			if cacheErr = s.db.QueryRow(r.Context(), "SELECT data FROM analytics.snapshots WHERE id=$1", *id).Scan(&raw); cacheErr != nil {
				return nil, cacheErr
			}
			var snap core.Snapshot
			if cacheErr = json.Unmarshal(raw, &snap); cacheErr != nil {
				return nil, cacheErr
			}
			report, e := snap.Dashboard(q)
			if e != nil {
				return nil, e
			}
			report.SnapshotID = id
			body, cacheErr = json.Marshal(report)
			if cacheErr == nil {
				_, cacheErr = s.db.Exec(r.Context(), "INSERT INTO analytics.responses VALUES($1,$2,$3) ON CONFLICT DO NOTHING", *id, key, body)
			}
		}
		if cacheErr != nil {
			return nil, cacheErr
		}
		redisCtx, cancel = context.WithTimeout(r.Context(), 200*time.Millisecond)
		_ = s.redis.Set(redisCtx, redisKey, body, 0).Err()
		cancel()
		return body, nil
	})
	if err != nil {
		status := http.StatusServiceUnavailable
		if strings.Contains(err.Error(), "unknown plan") {
			status = http.StatusBadRequest
		}
		write(w, status, map[string]any{"error": "cannot load dashboard", "data_as_of": asof})
		return
	}
	tag := fmt.Sprintf("\"%d\"", *id)
	w.Header().Set("ETag", tag)
	w.Header().Set("Last-Modified", asof.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Data-As-Of", asof.UTC().Format(time.RFC3339Nano))
	w.Header().Set("X-Snapshot-Version", tag)
	age := 0
	if next != nil && !s.running.Load() {
		age = max(0, min(600, int(time.Until(*next).Seconds())))
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", age))
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == tag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(value.([]byte))
}

type previewRequest struct {
	Seed          int64  `json:"seed"`
	UsersCount    int    `json:"users_count"`
	HistoryMonths int    `json:"history_months"`
	DataAsOf      string `json:"data_as_of"`
}

var previewGate = make(chan struct{}, 1)

func validatePreview(p previewRequest, now time.Time) (time.Time, error) {
	asof, e := time.Parse(time.RFC3339, p.DataAsOf)
	if e != nil || asof.After(now) || asof.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return time.Time{}, fmt.Errorf("data_as_of must be a past RFC3339 timestamp after 2000")
	}
	if p.UsersCount < 10000 || p.UsersCount > 100000 {
		return time.Time{}, fmt.Errorf("users_count must be between 10000 and 100000")
	}
	if p.HistoryMonths < 18 || p.HistoryMonths > 24 {
		return time.Time{}, fmt.Errorf("history_months must be between 18 and 24")
	}
	return asof.UTC(), nil
}

func (s *service) preview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q, e := dashboardQuery(r)
	if e != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": e.Error(), "data_as_of": nil})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	p, asof, e := previewConfig(r)
	if e != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": e.Error(), "data_as_of": nil})
		return
	}
	select {
	case previewGate <- struct{}{}:
		defer func() { <-previewGate }()
	default:
		w.Header().Set("Retry-After", "2")
		write(w, http.StatusTooManyRequests, map[string]any{"error": "generation already running", "data_as_of": nil})
		return
	}
	if r.Context().Err() != nil {
		return
	}
	dataset := core.Generate(p.Seed, p.UsersCount, p.HistoryMonths, asof)
	snapshot, e := core.Prepare(dataset, asof)
	if e != nil {
		write(w, http.StatusInternalServerError, map[string]any{"error": "cannot generate dataset", "data_as_of": nil})
		return
	}
	_ = snapshot.Dataset.SetConfig(fmt.Sprintf("%s/%d/%d/%d/%s", core.GeneratorVersion, p.Seed, p.UsersCount, p.HistoryMonths, asof.Format(time.RFC3339)))
	report, e := snapshot.Dashboard(q)
	if e != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": e.Error(), "data_as_of": asof})
		return
	}
	if r.Context().Err() != nil {
		return
	}
	report.Mode = "preview"
	write(w, http.StatusOK, report)
}

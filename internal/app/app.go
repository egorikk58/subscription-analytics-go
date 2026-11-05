package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
	"subscription-analytics/internal/core"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func positiveDuration(k, d string) (time.Duration, error) {
	v, e := time.ParseDuration(env(k, d))
	if e != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", k)
	}
	return v, nil
}
func boundedInt(k, d string, minValue, maxValue int) (int, error) {
	v, e := strconv.Atoi(env(k, d))
	if e != nil || v < minValue || v > maxValue {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", k, minValue, maxValue)
	}
	return v, nil
}
func Main(kind string) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", kind))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if e := run(ctx, kind); e != nil {
		slog.Error("service stopped", "error", e)
		os.Exit(1)
	}
}
func run(ctx context.Context, kind string) error {
	config, e := loadConfig(kind)
	if e != nil {
		return e
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pool, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return e
	}
	defer pool.Close()
	if e = pool.Ping(ctx); e != nil {
		return e
	}
	if kind == "bootstrap" {
		return bootstrap(ctx, pool)
	}
	obs := newObservability(kind, pool)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if e := pool.Ping(r.Context()); e != nil {
			write(w, http.StatusServiceUnavailable, map[string]any{"error": "database unavailable", "data_as_of": nil})
			return
		}
		var asof *time.Time
		if kind == "analytics-service" {
			if err := pool.QueryRow(r.Context(), "SELECT (SELECT data_as_of FROM analytics.snapshots WHERE id=current_id) FROM analytics.state WHERE id=1").Scan(&asof); err != nil {
				write(w, http.StatusServiceUnavailable, map[string]any{"error": "snapshot metadata unavailable", "data_as_of": nil})
				return
			}
		}
		write(w, 200, map[string]any{"status": "ok", "data_as_of": asof})
	})
	var worker *service
	if kind == "analytics-service" {
		worker = &service{ctx: ctx, obs: obs, db: pool, interval: config.interval, redis: redis.NewClient(&redis.Options{Addr: env("REDIS_ADDR", "redis:6379"), MaxRetries: -1, ReadTimeout: 200 * time.Millisecond, WriteTimeout: 200 * time.Millisecond, ContextTimeoutEnabled: true})}
		defer func() { _ = worker.redis.Close() }()
		mux.HandleFunc("GET /status", worker.status)
		mux.HandleFunc("GET /api/dashboard", worker.dashboard)
		mux.HandleFunc("POST /api/demo/preview", worker.preview)
		mux.HandleFunc("GET /api/reports/export.xlsx", worker.exportWorkbook)
		mux.HandleFunc("POST /api/reports/export.xlsx", worker.exportWorkbook)
		mux.HandleFunc("GET /users/by-telegram/{telegram_id}/ltv", worker.telegramLTV)
		mux.HandleFunc("GET /api/users/by-telegram/{telegram_id}/history", worker.userHistory)
		for _, route := range []string{"summary", "by-plan", "by-utm", "retention", "plans"} {
			name := route
			path := "/metrics/ltv/" + route
			if route == "retention" {
				path = "/metrics/retention"
			}
			if route == "plans" {
				path = "/api/plans"
			}
			mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) { worker.metrics(w, r, name) })
		}
		if e = registerTools(mux, obs); e != nil {
			return e
		}
		mux.Handle("/", frontend())
	} else {
		schema := strings.TrimSuffix(kind, "-service")
		routes := []string{schema}
		if schema == "subscriptions" {
			routes = []string{"plans", "subscriptions"}
		}
		for _, table := range routes {
			t := table
			mux.HandleFunc("GET /"+t, func(w http.ResponseWriter, r *http.Request) { config.mock.serve(w, r, pool, schema, t) })
		}
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", obs.handler())
	metricsMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	baseContext := func(net.Listener) context.Context { return ctx }
	metricsServer := &http.Server{Addr: ":" + strconv.Itoa(config.metricsPort), Handler: metricsMux, BaseContext: baseContext, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	server := &http.Server{Addr: ":" + strconv.Itoa(config.port), Handler: obs.middleware(requestTimeouts(mux)), BaseContext: baseContext, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 125 * time.Second}
	var background sync.WaitGroup
	defer func() {
		cancel()
		background.Wait()
	}()
	if worker != nil {
		worker.running.Store(true)
		background.Add(1)
		go func() {
			defer background.Done()
			worker.loop(ctx, config.syncTimeout)
		}()
	}
	background.Add(1)
	go func() {
		defer background.Done()
		obs.poll(ctx, pool, worker)
	}()
	errs := make(chan error, 2)
	go func() { errs <- metricsServer.ListenAndServe() }()
	go func() { errs <- server.ListenAndServe() }()
	var listenErr error
	select {
	case <-ctx.Done():
	case listenErr = <-errs:
		if errors.Is(listenErr, http.ErrServerClosed) {
			listenErr = nil
		}
	}
	cancel()
	stopCtx, stop := context.WithTimeout(context.Background(), 20*time.Second)
	defer stop()
	return errors.Join(listenErr, shutdownHTTP(stopCtx, metricsServer), shutdownHTTP(stopCtx, server))
}

func shutdownHTTP(ctx context.Context, server *http.Server) error {
	if err := server.Shutdown(ctx); err != nil {
		return errors.Join(err, server.Close())
	}
	return nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func bootstrap(ctx context.Context, db *pgxpool.Pool) error {
	tx, e := db.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(794123); CREATE SCHEMA IF NOT EXISTS users; CREATE SCHEMA IF NOT EXISTS subscriptions; CREATE SCHEMA IF NOT EXISTS payments; CREATE SCHEMA IF NOT EXISTS analytics;
 CREATE TABLE IF NOT EXISTS analytics.migrations(version int PRIMARY KEY); INSERT INTO analytics.migrations VALUES(1) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS analytics.seed(id int PRIMARY KEY, config text NOT NULL);
 CREATE TABLE IF NOT EXISTS users.users(id uuid PRIMARY KEY, telegram_id bigint UNIQUE NOT NULL, updated_at timestamptz NOT NULL, data jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS subscriptions.plans(id text PRIMARY KEY, updated_at timestamptz NOT NULL, data jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS subscriptions.subscriptions(id uuid PRIMARY KEY,user_id uuid REFERENCES users.users(id),plan_id text REFERENCES subscriptions.plans(id),updated_at timestamptz NOT NULL,data jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS payments.payments(id uuid PRIMARY KEY,user_id uuid REFERENCES users.users(id),subscription_id uuid REFERENCES subscriptions.subscriptions(id),refund_of uuid UNIQUE REFERENCES payments.payments(id),updated_at timestamptz NOT NULL,data jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS analytics.raw(kind text,id text,data jsonb NOT NULL,PRIMARY KEY(kind,id));
 CREATE TABLE IF NOT EXISTS analytics.snapshots(id bigserial PRIMARY KEY,data_as_of timestamptz NOT NULL,retired_at timestamptz,data jsonb NOT NULL);
 CREATE TABLE IF NOT EXISTS analytics.responses(snapshot_id bigint REFERENCES analytics.snapshots(id) ON DELETE CASCADE,key text,body bytea NOT NULL,PRIMARY KEY(snapshot_id,key));
 CREATE TABLE IF NOT EXISTS analytics.user_ltv(snapshot_id bigint REFERENCES analytics.snapshots(id) ON DELETE CASCADE,telegram_id bigint,body bytea NOT NULL,PRIMARY KEY(snapshot_id,telegram_id));
 INSERT INTO analytics.migrations VALUES(2) ON CONFLICT DO NOTHING;
 ALTER TABLE analytics.snapshots ADD COLUMN IF NOT EXISTS history_ready boolean NOT NULL DEFAULT false;
 ALTER TABLE analytics.user_ltv ADD COLUMN IF NOT EXISTS history bytea;
 INSERT INTO analytics.migrations VALUES(3) ON CONFLICT DO NOTHING;
 CREATE TABLE IF NOT EXISTS analytics.state(id int PRIMARY KEY CHECK(id=1),current_id bigint REFERENCES analytics.snapshots(id),cursor_at timestamptz,last_success timestamptz,next_sync timestamptz,last_error text,running boolean NOT NULL DEFAULT false); INSERT INTO analytics.state(id) VALUES(1) ON CONFLICT DO NOTHING;`)
	if e != nil {
		return e
	}
	for _, schema := range []string{"users", "subscriptions", "payments", "analytics"} {
		role := schema + "_reader"
		if schema == "analytics" {
			role = "analytics_app"
		}
		password := os.Getenv(strings.ToUpper(schema) + "_PASSWORD")
		if password == "" {
			return fmt.Errorf("%s_PASSWORD is required", strings.ToUpper(schema))
		}
		var exists bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1)", role).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			_, e = tx.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD '"+strings.ReplaceAll(password, "'", "''")+"'")
			if e != nil {
				return e
			}
		}
		priv := "SELECT"
		if schema == "analytics" {
			priv = "ALL"
		}
		_, e = tx.Exec(ctx, fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s; GRANT %s ON ALL TABLES IN SCHEMA %s TO %s; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA %s TO %s", schema, role, priv, schema, role, schema, role))
		if e != nil {
			return e
		}
	}
	if e = grantDatabaseViewer(ctx, tx); e != nil {
		return e
	}
	seed, e := strconv.ParseInt(env("SEED", "42"), 10, 64)
	if e != nil {
		return fmt.Errorf("SEED must be an int64")
	}
	months, e := boundedInt("HISTORY_MONTHS", "24", 18, 24)
	if e != nil {
		return e
	}
	asof, e := time.Parse(time.RFC3339, env("DATASET_AS_OF", "2026-11-01T00:00:00Z"))
	if e != nil || asof.After(time.Now()) {
		return fmt.Errorf("DATASET_AS_OF must be an RFC3339 timestamp at or before now")
	}
	var old string
	e = tx.QueryRow(ctx, "SELECT config FROM analytics.seed WHERE id=1").Scan(&old)
	exists := e == nil
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	count, err := seedUsersCount(env("USERS_COUNT", "auto"), seed, old)
	if err != nil {
		return err
	}
	config := fmt.Sprintf("%s/%d/%d/%d/%s", core.GeneratorVersion, seed, count, months, asof.Format(time.RFC3339Nano))
	if exists {
		if !sameDatasetParameters(old, config) {
			return fmt.Errorf("seed parameters differ: use a separate database")
		}
		if strings.HasPrefix(old, "v1/") {
			if e = upgradePayments(ctx, tx); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, "UPDATE analytics.seed SET config=$1 WHERE id=1", strings.Replace(old, "v1/", "v2-migrated/", 1)); e != nil {
				return e
			}
		}
		return tx.Commit(ctx)
	}
	d := core.Generate(seed, count, months, asof)
	if _, e = core.Prepare(d, asof); e != nil {
		return e
	}
	for _, u := range d.Users {
		b, _ := json.Marshal(u)
		if _, e = tx.Exec(ctx, "INSERT INTO users.users VALUES($1,$2,$3,$4)", u.ID, u.TelegramID, u.UpdatedAt, b); e != nil {
			return e
		}
	}
	for _, p := range d.Plans {
		b, _ := json.Marshal(p)
		if _, e = tx.Exec(ctx, "INSERT INTO subscriptions.plans VALUES($1,$2,$3)", p.ID, asof, b); e != nil {
			return e
		}
	}
	for _, s := range d.Subscriptions {
		b, _ := json.Marshal(s)
		if _, e = tx.Exec(ctx, "INSERT INTO subscriptions.subscriptions VALUES($1,$2,$3,$4,$5)", s.ID, s.UserID, s.PlanID, s.UpdatedAt, b); e != nil {
			return e
		}
	}
	for _, p := range d.Payments {
		b, _ := json.Marshal(p)
		var refund any
		if p.RefundOf != "" {
			refund = p.RefundOf
		}
		if _, e = tx.Exec(ctx, "INSERT INTO payments.payments VALUES($1,$2,$3,$4,$5,$6)", p.ID, p.UserID, p.SubscriptionID, refund, p.UpdatedAt, b); e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, "INSERT INTO analytics.seed VALUES(1,$1)", config)
	if e != nil {
		return e
	}
	slog.Info("dataset generated", "users", len(d.Users), "payments", len(d.Payments))
	return tx.Commit(ctx)
}
func (mock mockConfig) serve(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, schema, table string) {
	select {
	case <-r.Context().Done():
		return
	case <-time.After(mock.delay):
	}
	draw := rand.IntN(10000)
	if draw < mock.failBPS {
		http.Error(w, "injected failure", 500)
		return
	}
	if draw < mock.failBPS+mock.limitBPS {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "rate limit", http.StatusTooManyRequests)
		return
	}
	page, size := 1, 1000
	var e error
	if v := r.URL.Query().Get("page"); v != "" {
		page, e = strconv.Atoi(v)
	}
	if e != nil || page < 1 || page > 1000000 {
		http.Error(w, "invalid page", 400)
		return
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		size, e = strconv.Atoi(v)
	}
	if e != nil || size < 1 || size > 1000 {
		http.Error(w, "invalid page_size", 400)
		return
	}
	since := time.Time{}
	if v := r.URL.Query().Get("updated_since"); v != "" {
		since, e = time.Parse(time.RFC3339Nano, v)
		if e != nil {
			http.Error(w, "invalid updated_since", 400)
			return
		}
	}
	rows, e := db.Query(r.Context(), fmt.Sprintf("SELECT data FROM %s.%s WHERE updated_at >= $1 ORDER BY updated_at,id LIMIT $2 OFFSET $3", schema, table), since, size+1, (page-1)*size)
	if e != nil {
		http.Error(w, "database error", http.StatusServiceUnavailable)
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			http.Error(w, "database error", http.StatusServiceUnavailable)
			return
		}
		items = append(items, b)
	}
	if rows.Err() != nil {
		http.Error(w, "database error", http.StatusServiceUnavailable)
		return
	}
	more := len(items) > size
	if more {
		items = items[:size]
	}
	write(w, 200, map[string]any{"items": items, "has_more": more})
}

type service struct {
	ctx      context.Context
	obs      *observability
	db       *pgxpool.Pool
	redis    *redis.Client
	interval time.Duration
	flight   singleflight.Group
	running  atomic.Bool
}

func (s *service) loop(ctx context.Context, timeout time.Duration) {
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		if _, err := s.db.Exec(cleanupCtx, "UPDATE analytics.state SET running=false WHERE id=1"); err != nil {
			slog.Warn("cannot reset synchronization state on shutdown", "error", err)
		}
	}()
	next := time.Now()
	for {
		if ctx.Err() != nil {
			return
		}
		s.running.Store(true)
		s.obs.set("sync_in_progress", 1)
		cycleStart := time.Now()
		_, _ = s.db.Exec(ctx, "UPDATE analytics.state SET running=true,next_sync=$1 WHERE id=1", next.Add(s.interval))
		syncCtx, c := context.WithTimeout(ctx, timeout)
		e := s.sync(syncCtx)
		s.obs.syncFinished(e, cycleStart)
		c()
		s.running.Store(false)
		if ctx.Err() != nil {
			return
		}
		if now := time.Now(); !next.After(now) {
			next = now.Add(s.interval - now.Sub(next)%s.interval)
		}
		if e != nil {
			slog.Error("sync failed", "error", e)
			_, _ = s.db.Exec(ctx, "UPDATE analytics.state SET running=false,last_error=$1,next_sync=$2 WHERE id=1", e.Error(), next)
		} else {
			_, _ = s.db.Exec(ctx, "UPDATE analytics.state SET running=false,next_sync=$1 WHERE id=1", next)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
	}
}
func (s *service) sync(ctx context.Context) error {
	// PostgreSQL timestamptz stores microseconds; use the same precision in the
	// immutable JSON so /status metadata and metric bodies identify one instant.
	asof := time.Now().UTC().Truncate(time.Microsecond)
	var cursor *time.Time
	if e := s.db.QueryRow(ctx, "SELECT cursor_at FROM analytics.state WHERE id=1").Scan(&cursor); e != nil {
		return e
	}
	var lists [4][]json.RawMessage
	g, gctx := errgroup.WithContext(ctx)
	usersURL := strings.TrimRight(env("USERS_URL", "http://users-service:8080"), "/")
	subscriptionsURL := strings.TrimRight(env("SUBSCRIPTIONS_URL", "http://subscriptions-service:8080"), "/")
	paymentsURL := strings.TrimRight(env("PAYMENTS_URL", "http://payments-service:8080"), "/")
	urls := []string{usersURL + "/users", subscriptionsURL + "/plans", subscriptionsURL + "/subscriptions", paymentsURL + "/payments"}
	for i, url := range urls {
		i, url := i, url
		g.Go(func() error {
			var e error
			since := cursor
			if i == 1 {
				since = nil
			}
			fetchStart := time.Now()
			lists[i], e = fetchObserved(gctx, url, since, s.obs, []string{"users", "plans", "subscriptions", "payments"}[i])
			s.obs.sourceFetchFinished([]string{"users", "plans", "subscriptions", "payments"}[i], e, fetchStart)
			return e
		})
	}
	if e := g.Wait(); e != nil {
		return e
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(ctx) }()
	kinds := []string{"users", "plans", "subscriptions", "payments"}
	for i, list := range lists {
		for _, b := range list {
			var v struct {
				ID string `json:"id"`
			}
			if e = json.Unmarshal(b, &v); e != nil {
				return e
			}
			if _, e = tx.Exec(ctx, "INSERT INTO analytics.raw VALUES($1,$2,$3) ON CONFLICT(kind,id) DO UPDATE SET data=EXCLUDED.data", kinds[i], v.ID, []byte(b)); e != nil {
				return e
			}
		}
	}
	var d core.Dataset
	targets := map[string]any{"users": &d.Users, "plans": &d.Plans, "subscriptions": &d.Subscriptions, "payments": &d.Payments}
	for kind, target := range targets {
		var b []byte
		if e = tx.QueryRow(ctx, "SELECT COALESCE(jsonb_agg(data ORDER BY id),'[]'::jsonb) FROM analytics.raw WHERE kind=$1", kind).Scan(&b); e != nil {
			return e
		}
		if e = json.Unmarshal(b, target); e != nil {
			return e
		}
	}
	var seedConfig string
	if e = tx.QueryRow(ctx, "SELECT config FROM analytics.seed WHERE id=1").Scan(&seedConfig); e != nil {
		return e
	}
	snap, e := preparePublishedSnapshot(d, seedConfig, asof)
	if e != nil {
		return e
	}
	b, e := json.Marshal(snap)
	if e != nil {
		return e
	}
	var id int64
	if e = tx.QueryRow(ctx, "INSERT INTO analytics.snapshots(data_as_of,data,history_ready) VALUES($1,$2,true) RETURNING id", asof, b).Scan(&id); e != nil {
		return e
	}
	histories := indexUserHistories(d, *snap.Dataset.ObservedThrough, nil)
	if _, e = tx.CopyFrom(ctx, pgx.Identifier{"analytics", "user_ltv"}, []string{"snapshot_id", "telegram_id", "body", "history"}, pgx.CopyFromSlice(len(snap.Facts), func(i int) ([]any, error) {
		f := snap.Facts[i]
		profile := snap.UserLTV(f)
		body, err := json.Marshal(profile)
		if err != nil {
			return nil, err
		}
		history, err := histories.project(profile)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(history)
		return []any{id, f.TelegramID, body, encoded}, err
	})); e != nil {
		return e
	}
	for _, route := range []string{"summary", "by-plan", "by-utm", "plans"} {
		q := core.Filter{Periods: 12}
		if route == "by-utm" {
			q.Page, q.PageSize = 1, 100
		}
		body, e := snap.Response(route, q)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO analytics.responses VALUES($1,$2,$3)", id, cacheKey(route, q), body); e != nil {
			return e
		}
	}
	for _, p := range snap.Plans {
		q := core.Filter{Plan: p.ID, Periods: 12}
		body, e := snap.Response("retention", q)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO analytics.responses VALUES($1,$2,$3)", id, cacheKey("retention", q), body); e != nil {
			return e
		}
	}
	// Retirement starts at publication, not at the beginning of a potentially
	// long source poll. Old readers retain the full cleanup grace period.
	_, e = tx.Exec(ctx, "UPDATE analytics.snapshots SET retired_at=clock_timestamp() WHERE id=(SELECT current_id FROM analytics.state WHERE id=1)")
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE analytics.state SET current_id=$2,cursor_at=$1,last_success=clock_timestamp(),last_error=NULL WHERE id=1", asof, id)
	if e != nil {
		return e
	}
	if e = tx.Commit(ctx); e != nil {
		return e
	}
	s.obs.set("dataset_users", float64(len(d.Users)))
	s.obs.set("dataset_subscriptions", float64(len(d.Subscriptions)))
	s.obs.set("dataset_payments", float64(len(d.Payments)))
	s.obs.set("sync_last_success_timestamp_seconds", float64(time.Now().Unix()))
	s.obs.set("snapshot_data_timestamp_seconds", float64(asof.Unix()))
	s.obs.set("snapshot_available", 1)
	s.obs.observeCurrencies(snap.Dataset.Currencies)
	slog.Info("snapshot published", "snapshot_id", id, "users", len(d.Users), "duration", time.Since(asof))
	rows, gcErr := s.db.Query(ctx, "DELETE FROM analytics.snapshots WHERE retired_at < $1 RETURNING id", time.Now().Add(-30*time.Minute))
	if gcErr != nil {
		slog.Warn("snapshot cleanup failed", "error", gcErr)
		return nil
	}
	var retired []int64
	for rows.Next() {
		var old int64
		if rows.Scan(&old) == nil {
			retired = append(retired, old)
		}
	}
	rows.Close()
	for _, old := range retired {
		cleanupCtx, cancel := context.WithTimeout(ctx, time.Second)
		var cursor uint64
		for {
			keys, next, err := s.redis.Scan(cleanupCtx, cursor, fmt.Sprintf("snapshot:%d:*", old), 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = s.redis.Unlink(cleanupCtx, keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		cancel()
	}
	return nil
}
func cacheKey(route string, q core.Filter) string {
	b, _ := json.Marshal(q)
	h := sha256.Sum256(append([]byte("v3:"+route), b...))
	return fmt.Sprintf("%x", h)
}
func (s *service) status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Cache-Control", "no-store")
	var id *int64
	var asof, last, next *time.Time
	var message *string
	var running bool
	e := s.db.QueryRow(r.Context(), "SELECT current_id,(SELECT data_as_of FROM analytics.snapshots WHERE id=current_id),last_success,next_sync,last_error,running FROM analytics.state WHERE id=1").Scan(&id, &asof, &last, &next, &message, &running)
	if e != nil {
		write(w, 503, map[string]any{"error": "database unavailable", "data_as_of": nil})
		return
	}
	write(w, 200, map[string]any{"snapshot_id": id, "data_as_of": asof, "last_success_at": last, "next_sync_at": next, "last_error": message, "sync_in_progress": running})
}
func (s *service) metrics(w http.ResponseWriter, r *http.Request, route string) {
	var id *int64
	var asof, next *time.Time
	snapshotErr := s.db.QueryRow(r.Context(), "SELECT current_id,(SELECT data_as_of FROM analytics.snapshots WHERE id=current_id),next_sync FROM analytics.state WHERE id=1").Scan(&id, &asof, &next)
	q := core.Filter{Plan: r.URL.Query().Get("plan_id"), Periods: 12}
	var paramErr error
	q.UTMLink, paramErr = utmParameter(r)
	if paramErr != nil {
		write(w, 400, map[string]any{"error": paramErr.Error(), "data_as_of": asof})
		return
	}
	if route == "by-utm" {
		q.Page, q.PageSize = 1, 100
		for name, target := range map[string]*int{"page": &q.Page, "page_size": &q.PageSize} {
			if raw := r.URL.Query().Get(name); raw != "" {
				value, err := strconv.Atoi(raw)
				if err != nil {
					write(w, 400, map[string]any{"error": "invalid pagination", "data_as_of": asof})
					return
				}
				*target = value
			}
		}
		if q.Page < 1 || q.Page > 1000000 || q.PageSize < 1 || q.PageSize > 1000 {
			write(w, 400, map[string]any{"error": "invalid pagination", "data_as_of": asof})
			return
		}
	}
	for name, target := range map[string]**time.Time{"from": &q.From, "to": &q.To} {
		if v := r.URL.Query().Get(name); v != "" {
			t, e := time.Parse(time.RFC3339Nano, v)
			if e != nil {
				write(w, 400, map[string]any{"error": "invalid " + name, "data_as_of": asof})
				return
			}
			t = t.UTC()
			*target = &t
		}
	}
	if v := r.URL.Query().Get("max_periods"); v != "" {
		q.Periods, _ = strconv.Atoi(v)
	}
	if q.Periods < 1 || q.Periods > 24 || (q.From != nil && q.To != nil && !q.To.After(*q.From)) {
		write(w, 400, map[string]any{"error": "invalid range", "data_as_of": asof})
		return
	}
	if snapshotErr != nil || id == nil {
		w.Header().Set("Retry-After", "5")
		write(w, 503, map[string]any{"error": "snapshot unavailable", "data_as_of": nil})
		return
	}
	key := cacheKey(route, q)
	body, e := s.cachedResponse(r.Context(), *id, key, func(ctx context.Context) ([]byte, error) {
		snap, err := s.loadSnapshot(ctx, *id)
		if err != nil {
			return nil, err
		}
		return snap.Response(route, q)
	})
	if e != nil {
		status := 503
		if strings.Contains(e.Error(), "unknown plan") {
			status = 400
		}
		write(w, status, map[string]any{"error": "cannot serve requested metrics", "data_as_of": asof})
		return
	}
	etag := fmt.Sprintf("\"%d\"", *id)
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", asof.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Data-As-Of", asof.UTC().Format(time.RFC3339Nano))
	w.Header().Set("X-Snapshot-Version", etag)
	age := 0
	if next != nil && !s.running.Load() {
		age = int(time.Until(*next).Seconds())
		if age < 0 {
			age = 0
		}
		if age > 600 {
			age = 600
		}
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", age))
	for _, tag := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		if strings.TrimPrefix(strings.TrimSpace(tag), "W/") == etag || strings.TrimSpace(tag) == "*" {
			w.WriteHeader(304)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

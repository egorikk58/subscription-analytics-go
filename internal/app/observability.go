package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type observability struct {
	registry            *prometheus.Registry
	requests            *prometheus.CounterVec
	latency             *prometheus.HistogramVec
	responseSize        *prometheus.HistogramVec
	inflight            prometheus.Gauge
	syncs               *prometheus.CounterVec
	syncDuration        *prometheus.HistogramVec
	sourceRequests      *prometheus.CounterVec
	sourceLatency       *prometheus.HistogramVec
	sourceFetches       *prometheus.CounterVec
	sourceFetchDuration *prometheus.HistogramVec
	cache               *prometheus.CounterVec
	buildDuration       prometheus.Histogram
	gauges              map[string]prometheus.Gauge
	counters            map[string]*atomic.Int64
	loadedSnapshot      int64
	currencyRecords     *prometheus.GaugeVec
}

func newObservability(serviceName string, pool *pgxpool.Pool) *observability {
	o := &observability{registry: prometheus.NewRegistry(), gauges: map[string]prometheus.Gauge{}, counters: map[string]*atomic.Int64{}}
	reg := prometheus.WrapRegistererWith(prometheus.Labels{"service": serviceName}, o.registry)
	o.currencyRecords = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "subscope_dataset_payment_records", Help: "Published payment records by canonical currency; includes attempts and refunds."}, []string{"currency"})
	reg.MustRegister(o.currencyRecords)
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	o.requests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "subscope_http_requests_total", Help: "Completed HTTP requests; normalized routes only."}, []string{"method", "route", "status"})
	o.latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "subscope_http_request_duration_seconds", Help: "HTTP request wall-clock latency.", Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20}}, []string{"method", "route"})
	o.responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "subscope_http_response_size_bytes", Help: "HTTP response body size.", Buckets: prometheus.ExponentialBuckets(256, 4, 8)}, []string{"route"})
	o.inflight = prometheus.NewGauge(prometheus.GaugeOpts{Name: "subscope_http_requests_in_flight", Help: "HTTP requests currently running."})
	o.syncs = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "subscope_sync_total", Help: "Synchronization cycles completed by outcome."}, []string{"result"})
	o.syncDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "subscope_sync_duration_seconds", Help: "Synchronization cycle duration.", Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60, 120, 300}}, []string{"result"})
	o.sourceRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "subscope_source_requests_total", Help: "Upstream HTTP attempts, including retries."}, []string{"source", "status"})
	o.sourceLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "subscope_source_request_duration_seconds", Help: "Upstream attempt latency until response headers.", Buckets: prometheus.DefBuckets}, []string{"source"})
	o.sourceFetches = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "subscope_source_fetch_total", Help: "Complete paginated source fetch outcomes."}, []string{"source", "result"})
	o.sourceFetchDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "subscope_source_fetch_duration_seconds", Help: "Complete source fetch latency including all pages and retries.", Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60, 120, 300}}, []string{"source"})
	reg.MustRegister(o.sourceFetches, o.sourceFetchDuration)
	o.cache = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "subscope_cache_lookups_total", Help: "Cache lookups by layer and outcome; coalesced operations count once."}, []string{"layer", "result"})
	o.buildDuration = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "subscope_response_build_duration_seconds", Help: "Filtered response construction and persistence after cache miss.", Buckets: prometheus.DefBuckets})
	reg.MustRegister(o.requests, o.latency, o.responseSize, o.inflight, o.syncs, o.syncDuration, o.sourceRequests, o.sourceLatency, o.cache, o.buildDuration)
	for _, result := range []string{"success", "error"} {
		o.syncs.WithLabelValues(result).Add(0)
	}
	for _, definition := range []struct{ key, name, help, result string }{
		{"postgres_transactions_committed", "subscope_postgres_transactions_total", "Database transactions by outcome.", "commit"},
		{"postgres_transactions_rolled_back", "subscope_postgres_transactions_total", "Database transactions by outcome.", "rollback"},
		{"postgres_deadlocks", "subscope_postgres_deadlocks_total", "Database deadlocks.", ""},
		{"postgres_temp_bytes", "subscope_postgres_temp_bytes_total", "Temporary bytes written by PostgreSQL.", ""},
		{"redis_evicted_keys", "subscope_redis_evicted_keys_total", "Redis key evictions.", ""},
		{"redis_keyspace_hits", "subscope_redis_keyspace_hits_total", "Redis keyspace hits.", ""},
		{"redis_keyspace_misses", "subscope_redis_keyspace_misses_total", "Redis keyspace misses.", ""},
	} {
		value := &atomic.Int64{}
		o.counters[definition.key] = value
		labels := prometheus.Labels{}
		if definition.result != "" {
			labels["result"] = definition.result
		}
		reg.MustRegister(prometheus.NewCounterFunc(prometheus.CounterOpts{Name: definition.name, Help: definition.help, ConstLabels: labels}, func() float64 { return float64(value.Load()) }))
	}
	for name, help := range map[string]string{
		"postgres_up": "Last PostgreSQL ping succeeded.", "postgres_database_size_bytes": "Database size in bytes.", "postgres_connections": "Connections to this database.",
		"redis_up": "Last Redis poll succeeded.", "redis_memory_used_bytes": "Redis allocated memory.", "redis_memory_limit_bytes": "Redis maxmemory; zero means unlimited.", "redis_connected_clients": "Redis connected clients.",
		"sync_in_progress": "A synchronization cycle is running.", "sync_interval_seconds": "Configured synchronization interval.", "sync_last_success_timestamp_seconds": "Last successful synchronization, restored from PostgreSQL.", "snapshot_available": "A published snapshot exists.", "snapshot_data_timestamp_seconds": "Published snapshot data_as_of.", "dataset_users": "Users in the last published dataset.", "dataset_subscriptions": "Subscriptions in the last published dataset.", "dataset_payments": "Payment records in the last published dataset.", "dependency_last_check_timestamp_seconds": "Last dependency poll completion.",
	} {
		g := prometheus.NewGauge(prometheus.GaugeOpts{Name: "subscope_" + name, Help: help})
		o.gauges[name] = g
		reg.MustRegister(g)
	}
	for label, get := range map[string]func() float64{
		"total": func() float64 {
			if pool == nil {
				return 0
			}
			return float64(pool.Stat().TotalConns())
		},
		"idle": func() float64 {
			if pool == nil {
				return 0
			}
			return float64(pool.Stat().IdleConns())
		},
		"acquired": func() float64 {
			if pool == nil {
				return 0
			}
			return float64(pool.Stat().AcquiredConns())
		},
		"max": func() float64 {
			if pool == nil {
				return 0
			}
			return float64(pool.Stat().MaxConns())
		},
	} {
		reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "subscope_postgres_pool_connections", Help: "Application pgx pool connections.", ConstLabels: prometheus.Labels{"state": label}}, get))
	}
	return o
}
func (o *observability) set(name string, value float64) {
	if o != nil {
		if counter, ok := o.counters[name]; ok {
			counter.Store(int64(value))
			return
		}
		o.gauges[name].Set(value)
	}
}

func (o *observability) observeCurrencies(counts map[string]int) {
	if o == nil {
		return
	}
	for _, currency := range []string{"RUB", "BTC", "USDT"} {
		o.currencyRecords.WithLabelValues(currency).Set(float64(counts[currency]))
	}
}
func (o *observability) cacheResult(layer string, err error) {
	if o == nil {
		return
	}
	result := "hit"
	if errors.Is(err, redis.Nil) || errors.Is(err, pgx.ErrNoRows) {
		result = "miss"
	} else if err != nil {
		result = "error"
	}
	o.cache.WithLabelValues(layer, result).Inc()
}
func (o *observability) buildStarted() func() {
	start := time.Now()
	return func() {
		if o != nil {
			o.buildDuration.Observe(time.Since(start).Seconds())
		}
	}
}
func (o *observability) syncFinished(err error, start time.Time) {
	if o == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	o.syncs.WithLabelValues(result).Inc()
	o.syncDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
	o.set("sync_in_progress", 0)
}
func (o *observability) sourceFetchFinished(source string, err error, start time.Time) {
	if o == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	if errors.Is(err, context.Canceled) {
		result = "canceled"
	}
	o.sourceFetches.WithLabelValues(source, result).Inc()
	o.sourceFetchDuration.WithLabelValues(source).Observe(time.Since(start).Seconds())
}
func (o *observability) handler() http.Handler {
	return promhttp.HandlerFor(o.registry, promhttp.HandlerOpts{})
}

type measuredWriter struct {
	http.ResponseWriter
	status, bytes int
}

func (w *measuredWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *measuredWriter) WriteHeader(code int) {
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *measuredWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	n, e := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, e
}
func metricRoute(path string) string {
	if strings.HasPrefix(path, "/users/by-telegram/") && strings.HasSuffix(path, "/ltv") {
		return "/users/by-telegram/{telegram_id}/ltv"
	}
	for _, prefix := range []string{"/assets/", "/fonts/", "/docs/", "/grafana/", "/db/", "/prometheus/"} {
		if strings.HasPrefix(path, prefix) {
			return prefix + "*"
		}
	}
	switch path {
	case "/", "/healthz", "/status", "/metrics", "/api/dashboard", "/api/demo/preview", "/api/reports/export.xlsx", "/api/plans", "/metrics/ltv/summary", "/metrics/ltv/by-plan", "/metrics/ltv/by-utm", "/metrics/retention", "/users", "/plans", "/subscriptions", "/payments", "/openapi.json", "/openapi-mocks.json":
		return path
	}
	return "unmatched"
}
func (o *observability) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		o.inflight.Inc()
		defer o.inflight.Dec()
		mw := &measuredWriter{ResponseWriter: w}
		next.ServeHTTP(mw, r)
		if mw.status == 0 {
			mw.status = 200
		}
		method := r.Method
		switch method {
		case "GET", "POST", "HEAD", "OPTIONS":
		default:
			method = "OTHER"
		}
		route := metricRoute(r.URL.Path)
		o.requests.WithLabelValues(method, route, strconv.Itoa(mw.status)).Inc()
		o.latency.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
		o.responseSize.WithLabelValues(route).Observe(float64(mw.bytes))
		if route != "/healthz" && !strings.HasPrefix(route, "/assets/") && !strings.HasPrefix(route, "/fonts/") {
			slog.Info("http request completed", "method", method, "route", route, "status", mw.status, "duration_ms", time.Since(start).Milliseconds(), "response_bytes", mw.bytes)
		}
	})
}

type observedTransport struct {
	source  string
	metrics *observability
}

func (t observedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := http.DefaultTransport.RoundTrip(r)
	status := "network_error"
	if resp != nil {
		status = strconv.Itoa(resp.StatusCode)
	}
	t.metrics.sourceRequests.WithLabelValues(t.source, status).Inc()
	t.metrics.sourceLatency.WithLabelValues(t.source).Observe(time.Since(start).Seconds())
	return resp, err
}

func (o *observability) poll(ctx context.Context, pool *pgxpool.Pool, worker *service) {
	poll := func() {
		if pool == nil {
			return
		}
		check, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := pool.Ping(check); err != nil {
			o.set("postgres_up", 0)
		} else {
			o.set("postgres_up", 1)
		}
		var size, connections, commits, rollbacks, deadlocks, temp int64
		if err := pool.QueryRow(check, "SELECT pg_database_size(current_database()),numbackends,xact_commit,xact_rollback,deadlocks,temp_bytes FROM pg_stat_database WHERE datname=current_database()").Scan(&size, &connections, &commits, &rollbacks, &deadlocks, &temp); err == nil {
			for k, v := range map[string]int64{"postgres_database_size_bytes": size, "postgres_connections": connections, "postgres_transactions_committed": commits, "postgres_transactions_rolled_back": rollbacks, "postgres_deadlocks": deadlocks, "postgres_temp_bytes": temp} {
				o.set(k, float64(v))
			}
		}
		if worker != nil {
			o.set("sync_interval_seconds", worker.interval.Seconds())
			if worker.running.Load() {
				o.set("sync_in_progress", 1)
			} else {
				o.set("sync_in_progress", 0)
			}
			var asof, last *time.Time
			var exists bool
			var snapshotID *int64
			if err := pool.QueryRow(check, "SELECT current_id,current_id IS NOT NULL,(SELECT data_as_of FROM analytics.snapshots WHERE id=current_id),last_success FROM analytics.state WHERE id=1").Scan(&snapshotID, &exists, &asof, &last); err == nil {
				if exists {
					o.set("snapshot_available", 1)
				} else {
					o.set("snapshot_available", 0)
				}
				if asof != nil {
					o.set("snapshot_data_timestamp_seconds", float64(asof.Unix()))
				}
				if snapshotID != nil && *snapshotID != o.loadedSnapshot {
					var raw []byte
					if err := pool.QueryRow(check, "SELECT data->'dataset' FROM analytics.snapshots WHERE id=$1", *snapshotID).Scan(&raw); err == nil {
						var data struct {
							Users         int            `json:"users_count"`
							Subscriptions int            `json:"subscriptions_count"`
							Payments      int            `json:"payments_count"`
							Currencies    map[string]int `json:"currencies"`
						}
						if json.Unmarshal(raw, &data) == nil {
							o.set("dataset_users", float64(data.Users))
							o.set("dataset_subscriptions", float64(data.Subscriptions))
							o.set("dataset_payments", float64(data.Payments))
							o.observeCurrencies(data.Currencies)
							o.loadedSnapshot = *snapshotID
						}
					}
				}
				if last != nil {
					o.set("sync_last_success_timestamp_seconds", float64(last.Unix()))
				}
			}
			if worker.redis != nil {
				redisCtx, stop := context.WithTimeout(ctx, time.Second)
				defer stop()
				info, err := worker.redis.Info(redisCtx, "memory", "stats", "clients").Result()
				if err != nil {
					o.set("redis_up", 0)
				} else {
					o.set("redis_up", 1)
					mapping := map[string]string{"used_memory": "redis_memory_used_bytes", "maxmemory": "redis_memory_limit_bytes", "connected_clients": "redis_connected_clients", "evicted_keys": "redis_evicted_keys", "keyspace_hits": "redis_keyspace_hits", "keyspace_misses": "redis_keyspace_misses"}
					for _, line := range strings.Split(info, "\n") {
						parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
						if len(parts) == 2 {
							if key, ok := mapping[parts[0]]; ok {
								if v, e := strconv.ParseFloat(parts[1], 64); e == nil {
									o.set(key, v)
								}
							}
						}
					}
				}
			}
		}
		o.set("dependency_last_check_timestamp_seconds", float64(time.Now().Unix()))
	}
	poll()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	dto "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"
	"subscription-analytics/internal/core"
)

// This test creates and drops its own database. TEST_DATABASE_URL must name a
// disposable PostgreSQL instance and a role allowed to CREATE DATABASE/ROLE.
func TestIntegrationPersistenceAndSync(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	name := "subscription_test_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if _, dropErr := admin.Exec(cleanupCtx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); dropErr != nil {
			t.Errorf("drop integration database: %v", dropErr)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = name
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, key := range []string{"USERS_PASSWORD", "SUBSCRIPTIONS_PASSWORD", "PAYMENTS_PASSWORD", "ANALYTICS_PASSWORD"} {
		t.Setenv(key, "integration-test-password")
	}
	t.Setenv("DB_VIEWER_PASSWORD", "integration-viewer-password")
	t.Setenv("SEED", "42")
	t.Setenv("USERS_COUNT", "10000")
	t.Setenv("HISTORY_MONTHS", "24")
	t.Setenv("DATASET_AS_OF", "2020-11-01T00:00:00Z")
	t.Setenv("MOCK_DELAY", "1ns")
	t.Setenv("MOCK_500_BPS", "0")
	t.Setenv("MOCK_429_BPS", "0")
	if err = bootstrap(ctx, db); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var users, uniqueTelegram, plans, payments int
	if err = db.QueryRow(ctx, `SELECT (SELECT count(*) FROM users.users),
		(SELECT count(DISTINCT telegram_id) FROM users.users),
		(SELECT count(*) FROM subscriptions.plans),
		(SELECT count(*) FROM payments.payments)`).Scan(&users, &uniqueTelegram, &plans, &payments); err != nil {
		t.Fatal(err)
	}
	if users != 10000 || uniqueTelegram != users || plans != 4 || payments == 0 {
		t.Fatalf("unexpected dataset: users=%d telegram=%d plans=%d payments=%d", users, uniqueTelegram, plans, payments)
	}
	exerciseLegacyMigration(t, ctx, db)
	if err = bootstrap(ctx, db); err != nil {
		t.Fatalf("idempotent bootstrap: %v", err)
	}
	var countAfter int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM payments.payments").Scan(&countAfter); err != nil || countAfter != payments {
		t.Fatalf("bootstrap duplicated data: before=%d after=%d error=%v", payments, countAfter, err)
	}
	var analyticsCanReadSource, usersCanReadAnalytics bool
	if err = db.QueryRow(ctx, `SELECT has_table_privilege('analytics_app','users.users','SELECT'),
		has_table_privilege('users_reader','analytics.snapshots','SELECT')`).Scan(&analyticsCanReadSource, &usersCanReadAnalytics); err != nil {
		t.Fatal(err)
	}
	if analyticsCanReadSource || usersCanReadAnalytics {
		t.Fatal("runtime database roles cross schema boundaries")
	}

	var viewerSelect, viewerInsert, viewerAnalyticsWrite bool
	if err = db.QueryRow(ctx, `SELECT has_table_privilege('demo_viewer','users.users','SELECT'),has_table_privilege('demo_viewer','users.users','INSERT'),has_table_privilege('demo_viewer','analytics.state','UPDATE')`).Scan(&viewerSelect, &viewerInsert, &viewerAnalyticsWrite); err != nil {
		t.Fatal(err)
	}
	if !viewerSelect || viewerInsert || viewerAnalyticsWrite {
		t.Fatal("database viewer is not SELECT-only")
	}

	var unavailable atomic.Bool
	var malformed atomic.Bool
	var latePageFailure atomic.Bool
	var incrementalRequests atomic.Int64
	mock, err := loadMockConfig()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, route := range []struct{ schema, table string }{
		{"users", "users"}, {"subscriptions", "plans"},
		{"subscriptions", "subscriptions"}, {"payments", "payments"},
	} {
		schema, table := route.schema, route.table
		mux.HandleFunc("GET /"+table, func(w http.ResponseWriter, r *http.Request) {
			if latePageFailure.Load() && table == "users" && r.URL.Query().Get("page") == "2" {
				http.Error(w, "source failed after a complete first page", http.StatusInternalServerError)
				return
			}
			if malformed.Load() && table == "users" {
				_, _ = w.Write([]byte(`{"error":"invalid successful upstream response"}`))
				return
			}
			if unavailable.Load() && table == "users" {
				http.Error(w, "integration source outage", http.StatusInternalServerError)
				return
			}
			if r.URL.Query().Get("updated_since") != "" {
				incrementalRequests.Add(1)
			}
			mock.serve(w, r, db, schema, table)
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	for _, key := range []string{"USERS_URL", "SUBSCRIPTIONS_URL", "PAYMENTS_URL"} {
		t.Setenv(key, server.URL)
	}
	page := func(n int) []core.User {
		t.Helper()
		response, requestErr := http.Get(fmt.Sprintf("%s/users?page=%d&page_size=37", server.URL, n))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		defer func() { _ = response.Body.Close() }()
		var result struct {
			Items   []core.User `json:"items"`
			HasMore bool        `json:"has_more"`
		}
		if decodeErr := json.NewDecoder(response.Body).Decode(&result); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if response.StatusCode != http.StatusOK || len(result.Items) != 37 || !result.HasMore {
			t.Fatalf("invalid pagination response: status=%d count=%d more=%v", response.StatusCode, len(result.Items), result.HasMore)
		}
		return result.Items
	}
	firstPage, secondPage := page(1), page(2)
	seen := make(map[string]bool)
	for _, user := range append(firstPage, secondPage...) {
		if seen[user.ID] {
			t.Fatalf("duplicate user across adjacent pages: %s", user.ID)
		}
		seen[user.ID] = true
	}

	newOfflineRedis := func() *redis.Client {
		return redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1,
			DialTimeout: 10 * time.Millisecond, ReadTimeout: 10 * time.Millisecond,
			WriteTimeout: 10 * time.Millisecond, ContextTimeoutEnabled: true})
	}
	cache := newOfflineRedis()
	if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
		_ = cache.Close()
		cache = redis.NewClient(&redis.Options{Addr: addr, DB: 15, MaxRetries: -1,
			DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
			ContextTimeoutEnabled: true})
		if err = cache.Ping(ctx).Err(); err != nil {
			t.Fatalf("test Redis: %v", err)
		}
	}
	t.Cleanup(func() { _ = cache.Close() })
	svc := &service{db: db, redis: cache, interval: time.Minute}
	requestMetrics := func(route, query string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		svc.metrics(response, httptest.NewRequest(http.MethodGet, "/metrics?"+query, nil), route)
		return response
	}
	initial := requestMetrics("summary", "")
	if initial.Code != http.StatusServiceUnavailable || initial.Header().Get("Retry-After") == "" {
		t.Fatalf("before first snapshot: status=%d headers=%v", initial.Code, initial.Header())
	}
	noHistory := requestUserHistory(svc, "1000007919", "", "")
	if noHistory.Code != http.StatusServiceUnavailable || noHistory.Header().Get("Retry-After") == "" {
		t.Fatalf("history before first snapshot: status=%d headers=%v", noHistory.Code, noHistory.Header())
	}
	malformed.Store(true)
	if err = svc.sync(ctx); err == nil {
		t.Fatal("malformed successful source response published an incomplete first snapshot")
	}
	var noSnapshot bool
	var rawCount int
	if err = db.QueryRow(ctx, "SELECT current_id IS NULL,(SELECT count(*) FROM analytics.raw) FROM analytics.state WHERE id=1").Scan(&noSnapshot, &rawCount); err != nil || !noSnapshot || rawCount != 0 {
		t.Fatalf("failed first sync modified imported/published data: empty=%v raw=%d error=%v", noSnapshot, rawCount, err)
	}
	malformed.Store(false)
	latePageFailure.Store(true)
	if err = svc.sync(ctx); err == nil {
		t.Fatal("a source failure on page 2 published a partial snapshot")
	}
	if err = db.QueryRow(ctx, "SELECT current_id IS NULL,(SELECT count(*) FROM analytics.raw) FROM analytics.state WHERE id=1").Scan(&noSnapshot, &rawCount); err != nil || !noSnapshot || rawCount != 0 {
		t.Fatal("late-page failure changed raw data or publication state", err)
	}
	latePageFailure.Store(false)
	if err = svc.sync(ctx); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	var snapshotID int64
	var cursor time.Time
	if err = db.QueryRow(ctx, "SELECT current_id,cursor_at FROM analytics.state WHERE id=1").Scan(&snapshotID, &cursor); err != nil {
		t.Fatal(err)
	}
	redisKey := fmt.Sprintf("snapshot:%d:%s", snapshotID, cacheKey("summary", core.Filter{Periods: 12}))
	t.Run("Excel export pins the snapshot and applies filters", func(t *testing.T) { exerciseWorkbookExport(t, svc, snapshotID) })
	t.Run("User history pins the snapshot and reconciles payments", func(t *testing.T) { exerciseUserHistory(t, svc, snapshotID, cursor) })
	if os.Getenv("TEST_REDIS_ADDR") != "" {
		// DB 15 is reserved for this disposable integration fixture.
		if err = cache.Del(ctx, redisKey).Err(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cache.Del(context.Background(), redisKey).Err() })
	}
	response := requestMetrics("summary", "")
	var summary struct {
		UsersCount int       `json:"users_count"`
		DataAsOf   time.Time `json:"data_as_of"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || summary.UsersCount != 10000 || response.Header().Get("ETag") == "" {
		t.Fatalf("summary: status=%d users=%d body=%s", response.Code, summary.UsersCount, response.Body.String())
	}
	if !summary.DataAsOf.Equal(cursor) {
		t.Fatalf("snapshot JSON timestamp differs from its persisted metadata: json=%s metadata=%s", summary.DataAsOf, cursor)
	}
	var observedThrough time.Time
	var activePeriods int
	if err = db.QueryRow(ctx, `SELECT (data->'dataset'->>'observed_through')::timestamptz, (data->'dataset'->>'active_periods')::int FROM analytics.snapshots WHERE id=$1`, snapshotID).Scan(&observedThrough, &activePeriods); err != nil {
		t.Fatal(err)
	}
	seedCutoff, err := time.Parse(time.RFC3339, os.Getenv("DATASET_AS_OF"))
	if err != nil || !observedThrough.Equal(seedCutoff) || !observedThrough.Before(cursor) || activePeriods == 0 {
		t.Fatalf("static data must retain active periods at seed cutoff: observed=%v cursor=%v active=%d", observedThrough, cursor, activePeriods)
	}
	if os.Getenv("TEST_REDIS_ADDR") != "" {
		cached, cacheErr := cache.Get(ctx, redisKey).Bytes()
		if cacheErr != nil || !bytes.Equal(cached, response.Body.Bytes()) {
			t.Fatalf("Redis read-through: error=%v equal=%v", cacheErr, bytes.Equal(cached, response.Body.Bytes()))
		}
	}
	notModified := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics/ltv/summary", nil)
	req.Header.Set("If-None-Match", response.Header().Get("ETag"))
	svc.metrics(notModified, req, "summary")
	if notModified.Code != http.StatusNotModified || notModified.Body.Len() != 0 {
		t.Fatalf("conditional GET: status=%d bytes=%d", notModified.Code, notModified.Body.Len())
	}
	badPlan := httptest.NewRecorder()
	badPlanReq := httptest.NewRequest(http.MethodGet, "/metrics/retention?plan_id=missing", nil)
	badPlanReq.Header.Set("If-None-Match", response.Header().Get("ETag"))
	svc.metrics(badPlan, badPlanReq, "retention")
	badDate := requestMetrics("summary", "from=invalid")
	for _, invalid := range []*httptest.ResponseRecorder{badPlan, badDate} {
		var body struct {
			DataAsOf time.Time `json:"data_as_of"`
		}
		if err = json.Unmarshal(invalid.Body.Bytes(), &body); err != nil || invalid.Code != http.StatusBadRequest || !body.DataAsOf.Equal(cursor) {
			t.Fatalf("invalid requests must retain 400 and the snapshot timestamp: status=%d body=%s", invalid.Code, invalid.Body.String())
		}
	}
	// Redis unavailability must still allow creation and reuse of a filtered PG cache entry.
	offline := newOfflineRedis()
	t.Cleanup(func() { _ = offline.Close() })
	svc.redis = offline
	filtered := requestMetrics("summary", "from=2019-01-01T00:00:00Z&to=2020-01-01T00:00:00Z")
	filteredAgain := requestMetrics("summary", "to=2020-01-01T00:00:00Z&from=2019-01-01T00:00:00Z")
	if filtered.Code != http.StatusOK || filteredAgain.Code != http.StatusOK || !bytes.Equal(filtered.Body.Bytes(), filteredAgain.Body.Bytes()) {
		t.Fatalf("PG filtered fallback: first=%d second=%d", filtered.Code, filteredAgain.Code)
	}
	from, _ := time.Parse(time.RFC3339, "2019-01-01T00:00:00Z")
	to, _ := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	key := cacheKey("summary", core.Filter{From: &from, To: &to, Periods: 12})
	if err = db.QueryRow(ctx, "SELECT count(*) FROM analytics.responses WHERE snapshot_id=$1 AND key=$2", snapshotID, key).Scan(&countAfter); err != nil || countAfter != 1 {
		t.Fatalf("durable filtered response cache: count=%d error=%v", countAfter, err)
	}
	retention := requestMetrics("retention", "plan_id=monthly&max_periods=2")
	if retention.Code != http.StatusOK || !bytes.Contains(retention.Body.Bytes(), []byte("cumulative_eligible")) {
		t.Fatalf("retention contract: status=%d body=%s", retention.Code, retention.Body.String())
	}
	unavailable.Store(true)
	if err = svc.sync(ctx); err == nil {
		t.Fatal("sync succeeded despite permanent source failure")
	}
	var afterFailureID int64
	var afterFailureCursor time.Time
	if err = db.QueryRow(ctx, "SELECT current_id,cursor_at FROM analytics.state WHERE id=1").Scan(&afterFailureID, &afterFailureCursor); err != nil {
		t.Fatal(err)
	}
	stale := requestMetrics("summary", "")
	if afterFailureID != snapshotID || !afterFailureCursor.Equal(cursor) || stale.Code != http.StatusOK || !bytes.Equal(stale.Body.Bytes(), response.Body.Bytes()) {
		t.Fatalf("failed sync changed published data: before=%d after=%d status=%d", snapshotID, afterFailureID, stale.Code)
	}
	unavailable.Store(false)
	updated := firstPage[0]
	updated.UTMLink = "integration-updated"
	updated.UpdatedAt = time.Now().UTC()
	userJSON, err := json.Marshal(updated)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE users.users SET updated_at=$1,data=$2 WHERE id=$3", updated.UpdatedAt, userJSON, updated.ID); err != nil {
		t.Fatal(err)
	}
	if err = svc.sync(ctx); err != nil {
		t.Fatalf("incremental sync: %v", err)
	}
	var utm string
	if err = db.QueryRow(ctx, "SELECT data->>'utm_link' FROM analytics.raw WHERE kind='users' AND id=$1", updated.ID).Scan(&utm); err != nil || utm != updated.UTMLink {
		t.Fatalf("updated old record not merged: utm=%q error=%v", utm, err)
	}
	if err = db.QueryRow(ctx, `SELECT (data->'dataset'->>'observed_through')::timestamptz FROM analytics.snapshots WHERE id=(SELECT current_id FROM analytics.state WHERE id=1)`).Scan(&observedThrough); err != nil || !observedThrough.Equal(seedCutoff) {
		t.Fatalf("metadata update moved the observation cutoff: observed=%v error=%v", observedThrough, err)
	}
	if incrementalRequests.Load() == 0 {
		t.Fatal("sources were never requested with updated_since")
	}
	if err = svc.sync(ctx); err != nil {
		t.Fatalf("repeated incremental sync: %v", err)
	}
	if err = db.QueryRow(ctx, "SELECT count(*) FROM analytics.raw WHERE kind='users'").Scan(&countAfter); err != nil || countAfter != 10000 {
		t.Fatalf("upsert duplicated users: count=%d error=%v", countAfter, err)
	}
	t.Run("concurrent cache miss performs one aggregation", func(t *testing.T) {
		observed := &service{db: db, redis: offline, interval: time.Minute, obs: newObservability("analytics-service", nil)}
		start := make(chan struct{})
		var callers sync.WaitGroup
		for i := 0; i < 24; i++ {
			callers.Add(1)
			go func() {
				defer callers.Done()
				<-start
				recorder := httptest.NewRecorder()
				observed.metrics(recorder, httptest.NewRequest("GET", "/metrics/ltv/summary?from=2019-03-07T00:00:00Z&to=2019-09-12T00:00:00Z", nil), "summary")
				if recorder.Code != http.StatusOK {
					t.Errorf("concurrent request status=%d", recorder.Code)
				}
			}()
		}
		close(start)
		callers.Wait()
		metric := &dto.Metric{}
		if err := observed.obs.buildDuration.Write(metric); err != nil {
			t.Fatal(err)
		}
		if metric.GetHistogram().GetSampleCount() != 1 {
			t.Fatalf("identical requests aggregated %d times", metric.GetHistogram().GetSampleCount())
		}
	})
	t.Run("retirement grace starts after preparation not at the poll watermark", func(t *testing.T) {
		var retiredAt, currentAsOf time.Time
		if err := db.QueryRow(ctx, `SELECT old.retired_at,current.data_as_of FROM analytics.snapshots old
CROSS JOIN analytics.state state JOIN analytics.snapshots current ON current.id=state.current_id
WHERE old.retired_at IS NOT NULL ORDER BY old.retired_at DESC LIMIT 1`).Scan(&retiredAt, &currentAsOf); err != nil {
			t.Fatal(err)
		}
		if !retiredAt.After(currentAsOf) {
			t.Fatal("slow polls consume the old snapshot's cleanup grace", retiredAt, currentAsOf)
		}
	})
	t.Run("failed commit keeps snapshot cursor and durable responses", func(t *testing.T) {
		var beforeID int64
		var beforeCursor time.Time
		if err := db.QueryRow(ctx, "SELECT current_id,cursor_at FROM analytics.state WHERE id=1").Scan(&beforeID, &beforeCursor); err != nil {
			t.Fatal(err)
		}
		before := requestMetrics("summary", "")
		// Deferring the exception until COMMIT exercises the publication boundary,
		// after all source copies, facts and standard answers have been prepared.
		_, err := db.Exec(ctx, `CREATE FUNCTION analytics.reject_test_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected commit failure'; END $$;
CREATE CONSTRAINT TRIGGER test_commit_failure AFTER UPDATE ON analytics.state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION analytics.reject_test_commit()`)
		if err != nil {
			t.Fatal(err)
		}
		if err = svc.sync(ctx); err == nil {
			t.Fatal("deferred commit failure was ignored")
		}
		if _, err = db.Exec(ctx, "DROP TRIGGER test_commit_failure ON analytics.state; DROP FUNCTION analytics.reject_test_commit()"); err != nil {
			t.Fatal(err)
		}
		var afterID int64
		var afterCursor time.Time
		if err = db.QueryRow(ctx, "SELECT current_id,cursor_at FROM analytics.state WHERE id=1").Scan(&afterID, &afterCursor); err != nil {
			t.Fatal(err)
		}
		after := requestMetrics("summary", "")
		if beforeID != afterID || !beforeCursor.Equal(afterCursor) || !bytes.Equal(before.Body.Bytes(), after.Body.Bytes()) {
			t.Fatal("failed commit replaced published snapshot, cursor or cached body")
		}
	})
	t.Run("published version survives a service restart and headers are bounded", func(t *testing.T) {
		restarted := &service{db: db, redis: offline, interval: time.Minute}
		if _, err := db.Exec(ctx, "UPDATE analytics.state SET next_sync=clock_timestamp()+interval '90 seconds' WHERE id=1"); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		restarted.metrics(recorder, httptest.NewRequest("GET", "/metrics/ltv/summary", nil), "summary")
		if recorder.Code != 200 || recorder.Header().Get("Last-Modified") == "" || recorder.Header().Get("X-Data-As-Of") == "" {
			t.Fatalf("restart response status=%d headers=%v", recorder.Code, recorder.Header())
		}
		var age int
		if _, err := fmt.Sscanf(recorder.Header().Get("Cache-Control"), "public, max-age=%d", &age); err != nil || age < 0 || age > 90 {
			t.Fatalf("invalid max-age: %s", recorder.Header().Get("Cache-Control"))
		}
		restarted.running.Store(true)
		duringSync := httptest.NewRecorder()
		restarted.metrics(duringSync, httptest.NewRequest("GET", "/metrics/ltv/summary", nil), "summary")
		if duringSync.Header().Get("Cache-Control") != "public, max-age=0" {
			t.Fatal("in-progress update allowed stale browser freshness")
		}
	})
	// Bonus reads use the published index/cache even when all sources are down.
	unavailable.Store(true)
	bonusMux := http.NewServeMux()
	bonusMux.HandleFunc("GET /users/by-telegram/{telegram_id}/ltv", svc.telegramLTV)
	lookup := func(value string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		bonusMux.ServeHTTP(r, httptest.NewRequest("GET", "/users/by-telegram/"+value+"/ltv", nil))
		return r
	}
	profileResponse := lookup(strconv.FormatInt(updated.TelegramID, 10))
	var profile core.UserLTV
	if err = json.Unmarshal(profileResponse.Body.Bytes(), &profile); err != nil || profileResponse.Code != 200 || profile.UserID != updated.ID {
		t.Fatalf("indexed lookup: %d %s %v", profileResponse.Code, profileResponse.Body.String(), err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		missing := lookup("9223372036854775807")
		if missing.Code != 404 {
			t.Fatalf("negative lookup: %d %s", missing.Code, missing.Body.String())
		}
	}
	utmResponse := requestMetrics("by-utm", "utm_link=integration-updated")
	if utmResponse.Code != 200 || !bytes.Contains(utmResponse.Body.Bytes(), []byte(`"users_count":1`)) {
		t.Fatalf("UTM filter: %d %s", utmResponse.Code, utmResponse.Body.String())
	}
	// Missing required crypto FX must reject publication and preserve the cursor.
	unavailable.Store(false)
	var beforeFX int64
	if err = db.QueryRow(ctx, "SELECT current_id FROM analytics.state WHERE id=1").Scan(&beforeFX); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE payments.payments SET data=data-'fx',updated_at=clock_timestamp() WHERE id=(SELECT id FROM payments.payments WHERE data->>'currency'='BTC' AND data->>'status'='succeeded' LIMIT 1)"); err != nil {
		t.Fatal(err)
	}
	if err = svc.sync(ctx); err == nil {
		t.Fatal("snapshot published with missing FX")
	}
	var afterFX int64
	if err = db.QueryRow(ctx, "SELECT current_id FROM analytics.state WHERE id=1").Scan(&afterFX); err != nil || afterFX != beforeFX {
		t.Fatal("FX failure changed current snapshot", err)
	}
	t.Setenv("SEED", "43")
	if err = bootstrap(ctx, db); err == nil {
		t.Fatal("bootstrap accepted incompatible seed configuration")
	}
}

package app

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestPGAdminAssetCompressionPreservesContentAndNegotiation(t *testing.T) {
	t.Setenv("OPS_USERNAME", "operator")
	t.Setenv("OPS_PASSWORD", "test-only-password")
	payload := strings.Repeat("const example = 'pgAdmin';\n", 1000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("ETag", `"asset-version"`)
		_, _ = io.WriteString(w, payload)
	}))
	defer upstream.Close()
	proxy, err := toolProxy(upstream.URL, "/db/", true, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		path, encoding string
		compressed     bool
	}{
		{"/db/static/vendor.js", "gzip, deflate, br", true},
		{"/db/static/vendor.js", "gzip;q=0, identity", false},
		{"/db/static/vendor.js", "identity", false},
		{"/db/query/results", "gzip", false},
	} {
		t.Run(scenario.path+scenario.encoding, func(t *testing.T) {
			request := httptest.NewRequest("GET", scenario.path, nil)
			request.SetBasicAuth("operator", "test-only-password")
			request.Header.Set("Accept-Encoding", scenario.encoding)
			record := httptest.NewRecorder()
			operationsAuth(proxy).ServeHTTP(record, request)
			cacheControl := record.Header().Values("Cache-Control")
			wantCache := "no-store"
			if strings.HasPrefix(scenario.path, "/db/static/") {
				wantCache = "private, max-age=3600"
			}
			if len(cacheControl) != 1 || cacheControl[0] != wantCache {
				t.Fatalf("cache policy=%v expected=%s", cacheControl, wantCache)
			}
			body := record.Body.String()
			if got := record.Header().Get("Content-Encoding") == "gzip"; got != scenario.compressed {
				t.Fatalf("compression=%v expected=%v", got, scenario.compressed)
			}
			if scenario.compressed {
				reader, err := gzip.NewReader(record.Body)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := io.ReadAll(reader)
				_ = reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				body = string(decoded)
				if record.Header().Get("ETag") != `W/"asset-version"` {
					t.Fatal("transformed asset needs a weak validator")
				}
			}
			if body != payload {
				t.Fatal("proxy changed JavaScript content")
			}
		})
	}
}

func TestPGAdminTimeoutDoesNotExtendMetricsDeadline(t *testing.T) {
	handler := requestTimeouts(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("request deadline missing")
		}
		remaining := time.Until(deadline)
		if strings.HasPrefix(r.URL.Path, "/db/") {
			if remaining < 119*time.Second || remaining > 120*time.Second {
				t.Errorf("pgAdmin deadline=%s", remaining)
			}
		} else if r.URL.Path == "/api/reports/export.xlsx" {
			if remaining < 89*time.Second || remaining > 90*time.Second {
				t.Errorf("export deadline=%s", remaining)
			}
		} else if remaining < 14*time.Second || remaining > 15*time.Second {
			t.Errorf("API deadline=%s", remaining)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, path := range []string{"/db/static/vendor.js", "/api/reports/export.xlsx", "/metrics/ltv/summary"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
}

func TestToolsAndFrontendRoutesCanBeRegisteredTogether(t *testing.T) {
	mux := http.NewServeMux()
	if err := registerTools(mux, newObservability("analytics-service", nil)); err != nil {
		t.Fatal(err)
	}
	mux.Handle("/", frontend())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("frontend POST status = %d", response.Code)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("OpenAPI status = %d", response.Code)
	}
}

func TestMetricsUseBoundedLabelsAndExposeRealObservations(t *testing.T) {
	obs := newObservability("analytics-service", nil)
	handler := obs.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failure", 500) }))
	for _, path := range []string{"/unknown/private-user-123?token=secret", "/unknown/private-user-456"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	obs.cacheResult("redis", redis.Nil)
	obs.syncFinished(fmt.Errorf("source failed"), time.Now().Add(-time.Second))
	obs.sourceFetchFinished("users", context.Canceled, time.Now().Add(-time.Second))
	obs.set("postgres_transactions_committed", 12)
	recorder := httptest.NewRecorder()
	obs.handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `subscope_source_fetch_total{result="canceled",service="analytics-service",source="users"} 1`) {
		t.Fatal("source fetch cancellation not observed")
	}
	for _, want := range []string{`subscope_http_requests_total{method="GET",route="unmatched",service="analytics-service",status="500"} 2`, `subscope_cache_lookups_total{layer="redis",result="miss",service="analytics-service"} 1`, `subscope_sync_total{result="error",service="analytics-service"} 1`, `subscope_postgres_transactions_total{result="commit",service="analytics-service"} 12`, `subscope_http_request_duration_seconds_bucket`, `go_goroutines`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing exposition: %s", want)
		}
	}
	if strings.Contains(body, "private-user") || strings.Contains(body, "secret") {
		t.Fatal("high-cardinality or sensitive label exposed")
	}
}

func TestOperationsRequireCredentialsAndDisabledByDefault(t *testing.T) {
	t.Setenv("OPS_USERNAME", "operator")
	t.Setenv("OPS_PASSWORD", "")
	handler := operationsAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	record := httptest.NewRecorder()
	handler.ServeHTTP(record, httptest.NewRequest("GET", "/db/", nil))
	if record.Code != 503 {
		t.Fatalf("unconfigured operations = %d", record.Code)
	}
	t.Setenv("OPS_PASSWORD", "test-only-password")
	for _, password := range []string{"", "wrong", "test-only-password"} {
		req := httptest.NewRequest("GET", "/db/", nil)
		req.SetBasicAuth("operator", password)
		record = httptest.NewRecorder()
		handler.ServeHTTP(record, req)
		expected := 401
		if password == "test-only-password" {
			expected = 204
		}
		if record.Code != expected {
			t.Errorf("authentication status=%d expected=%d", record.Code, expected)
		}
	}
}

func TestToolsProxyPreservesSubpathsAndReplacesUntrustedIdentity(t *testing.T) {
	t.Setenv("OPS_USERNAME", "operator")
	t.Setenv("OPS_PASSWORD", "test-only-password")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("gateway credentials forwarded")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"path": r.URL.Path, "user": r.Header.Get("X-WEBAUTH-USER"), "query": r.URL.RawQuery, "script": r.Header.Get("X-Script-Name"), "scheme": r.Header.Get("X-Scheme"), "host": r.Host})
	}))
	defer upstream.Close()
	for _, grafana := range []bool{false, true} {
		prefix := "/db/"
		expectedPath := "/query"
		if grafana {
			prefix = "/grafana/"
			expectedPath = "/grafana/query"
		}
		proxy, err := toolProxy(upstream.URL, prefix, !grafana, grafana)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", prefix+"query?example=1", nil)
		req.SetBasicAuth("operator", "test-only-password")
		req.Header.Set("X-WEBAUTH-USER", "forged-admin")
		req.Header.Set("X-Script-Name", "/forged-prefix")
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Host = "analytics.example.test"
		response := httptest.NewRecorder()
		operationsAuth(proxy).ServeHTTP(response, req)
		var data map[string]string
		if err = json.Unmarshal(response.Body.Bytes(), &data); err != nil {
			t.Fatalf("proxy response: %s %v", response.Body.String(), err)
		}
		if data["path"] != expectedPath || data["query"] != "example=1" {
			t.Fatal(data)
		}
		if !grafana && (data["script"] != "/db" || data["scheme"] != "https" || data["host"] != "analytics.example.test") {
			t.Fatalf("pgAdmin proxy context: %v", data)
		}
		expectedUser := ""
		if grafana {
			expectedUser = "operator"
		}
		if data["user"] != expectedUser {
			t.Fatalf("forged identity trusted: %v", data)
		}
	}
}

func TestOpenAPIContractsServedWithoutFrontendBuild(t *testing.T) {
	mux := http.NewServeMux()
	if err := registerTools(mux, newObservability("analytics-service", nil)); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/openapi.json", "/openapi-mocks.json"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		var document struct {
			OpenAPI string         `json:"openapi"`
			Paths   map[string]any `json:"paths"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &document); err != nil || w.Code != 200 || document.OpenAPI != "3.0.3" || len(document.Paths) == 0 {
			t.Fatalf("invalid specification %s", path)
		}
	}
}

func TestPrivateMetricsPollStopsWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { newObservability("users-service", nil).poll(ctx, nil, nil); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poll did not stop")
	}
}

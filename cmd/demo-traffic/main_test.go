package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDemoTrafficUsesReadOnlyRequestsAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	seen := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || (!strings.HasPrefix(r.URL.Path, "/metrics/") && !strings.HasPrefix(r.URL.Path, "/api/dashboard")) {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(200)
		select {
		case seen <- struct{}{}:
		default:
		}
	}))
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		_, err := run(ctx, configuration{server.URL, time.Minute, 50, 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		done <- err
	}()
	select {
	case <-seen:
		cancel()
	case <-time.After(3 * time.Second):
		t.Fatal("no HTTP request")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("generator ignored cancellation")
	}
}

func TestDemoConfigurationAndVariantBound(t *testing.T) {
	for _, cfg := range []configuration{{"file:///etc/passwd", time.Second, 1, 1}, {"http://user:password@localhost", time.Second, 1, 1}, {"http://localhost", time.Hour, 10, 1}, {"http://localhost", time.Second, 51, 1}, {"http://localhost", time.Second, 1, 0}} {
		if cfg.validate() == nil {
			t.Fatal("accepted invalid configuration")
		}
	}
	paths := map[string]bool{}
	for i := 0; i < 10000; i++ {
		paths[requestPath(i)] = true
	}
	if len(paths) > 65 {
		t.Fatal("unbounded query variants", len(paths))
	}
}

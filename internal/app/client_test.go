package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchPaginationAndRetry(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.WriteHeader(500)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":"one"}],"has_more":true}`))
		} else {
			_, _ = w.Write([]byte(`{"items":[{"id":"two"}],"has_more":false}`))
		}
	}))
	defer server.Close()
	items, e := fetch(context.Background(), server.URL, nil)
	if e != nil || len(items) != 2 || calls.Load() != 3 {
		t.Fatalf("items=%v calls=%d error=%v", items, calls.Load(), e)
	}
}
func TestFetchRejectsPartialLoad(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":"one"}],"has_more":true}`))
			return
		}
		w.WriteHeader(500)
	}))
	defer server.Close()
	t.Setenv("RETRY_ATTEMPTS", "1")
	items, e := fetch(context.Background(), server.URL, nil)
	if e == nil || items != nil {
		t.Fatal("partial load returned as success")
	}
}
func TestFetchCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "120"); w.WriteHeader(429) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e := fetch(ctx, server.URL, nil)
	if e == nil || time.Since(started) > time.Second {
		t.Fatal("retry did not respect cancellation")
	}
}

func TestFetch429HonorsRetryAfterAndDoesNotRetryPermanent400(t *testing.T) {
	t.Setenv("RETRY_ATTEMPTS", "2")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"items":[],"has_more":false}`))
	}))
	defer server.Close()
	started := time.Now()
	if _, err := fetch(context.Background(), server.URL, nil); err != nil || calls.Load() != 2 || time.Since(started) < time.Second {
		t.Fatalf("Retry-After ignored: calls=%d elapsed=%s error=%v", calls.Load(), time.Since(started), err)
	}
	calls.Store(0)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer bad.Close()
	if _, err := fetch(context.Background(), bad.URL, nil); err == nil || calls.Load() != 1 {
		t.Fatal("permanent client error was retried")
	}
}

func TestFetchHTTPDateRetryAfterCanBeCanceled(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", time.Now().Add(time.Minute).UTC().Format(http.TimeFormat))
			w.WriteHeader(429)
			return
		}
		_, _ = w.Write([]byte(`{"items":[],"has_more":false}`))
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	defer cancel()
	if _, err := fetch(ctx, server.URL, nil); err == nil || calls.Load() != 1 {
		t.Fatal("HTTP-date Retry-After ignored or context cancellation lost")
	}
}

func TestFetchRejectsInvalidSuccessEnvelope(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"error":"source unavailable"}`,
		`{"items":[],"has_more":null}`,
		`{"items":null,"has_more":false}`,
		`{"items":[]}`,
		`{"items":[],"has_more":true}`,
		`{"items":[{"id":"one"}],"has_more":false} {"error":"trailing data"}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") != "1" {
					_, _ = w.Write([]byte(`{"items":[],"has_more":false}`))
					return
				}
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			items, err := fetch(context.Background(), server.URL, nil)
			if err == nil || items != nil {
				t.Fatalf("invalid successful source response accepted: items=%s error=%v", items, err)
			}
		})
	}
}

func TestFetchAcceptsEmptyIncrementalPage(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("updated_since") != since.Format(time.RFC3339Nano) {
			t.Error("incremental cursor missing from source request")
		}
		_, _ = w.Write([]byte(`{"items":[],"has_more":false}`))
	}))
	defer server.Close()
	items, err := fetch(context.Background(), server.URL, &since)
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("valid empty page rejected: items=%s error=%v", items, err)
	}
}

func TestFetchRetriesInterruptedSuccessBody(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, rw, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = fmt.Fprint(rw, "HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n{\"items\":[")
			_ = rw.Flush()
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"recovered"}],"has_more":false}`))
	}))
	defer server.Close()
	t.Setenv("RETRY_ATTEMPTS", "2")
	items, err := fetch(context.Background(), server.URL, nil)
	if err != nil || len(items) != 1 || calls.Load() != 2 {
		t.Fatalf("interrupted body not recovered: %v %v %d", items, err, calls.Load())
	}
}

func TestFetchRejectsOversizeBodyAfterValidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"items":[],"has_more":false}`)
		_, _ = fmt.Fprint(w, strings.Repeat(" ", maxSourceResponseBytes))
		_, _ = fmt.Fprint(w, "trailing corruption")
	}))
	defer server.Close()
	items, err := fetch(context.Background(), server.URL, nil)
	if err == nil || items != nil {
		t.Fatal("oversize response silently truncated and accepted")
	}
}

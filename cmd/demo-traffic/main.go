// demo-traffic sends bounded, read-only requests to the analytics API.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type configuration struct {
	URL              string
	Duration         time.Duration
	RPS, Concurrency int
}

func (c configuration) validate() error {
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("url must be an HTTP(S) origin without credentials, query or fragment")
	}
	if c.Duration <= 0 || c.Duration > 15*time.Minute || c.RPS < 1 || c.RPS > 50 || c.Concurrency < 1 || c.Concurrency > 16 {
		return fmt.Errorf("duration: 1ns..15m, rps: 1..50, concurrency: 1..16")
	}
	return nil
}

type counters struct{ requests, success, clientErrors, serverErrors, networkErrors, skipped, latency atomic.Int64 }

// At most 65 distinct query variants per snapshot; the demo cannot produce
// an unbounded stream of persistent response-cache keys.
func requestPath(index int) string {
	switch index % 8 {
	case 0:
		return "/metrics/ltv/summary"
	case 1:
		return "/metrics/ltv/by-plan"
	case 2:
		return "/metrics/ltv/by-utm"
	case 3:
		return "/metrics/retention?plan_id=monthly&max_periods=12"
	case 4:
		return "/metrics/retention?plan_id=quarterly&max_periods=8"
	case 5:
		return fmt.Sprintf("/api/dashboard?plan_id=all&page=%d&sort=ltv_desc", (index/8)%5+1)
	case 6:
		return fmt.Sprintf("/metrics/ltv/summary?utm_link=campaign-%d", (index/8)%5)
	default:
		return fmt.Sprintf("/api/dashboard?from=2025-%02d-01T00%%3A00%%3A00Z&page=1&sort=ltv_asc", (index/8)%12+1)
	}
}

func run(ctx context.Context, c configuration, logger *slog.Logger) (int64, error) {
	if err := c.validate(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.Duration)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()
	stats := &counters{}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for i := 0; i < c.Concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				started := time.Now()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.URL, "/")+requestPath(index), nil)
				if err != nil {
					stats.networkErrors.Add(1)
					continue
				}
				response, err := client.Do(req)
				if err != nil {
					if ctx.Err() == nil {
						stats.requests.Add(1)
						stats.networkErrors.Add(1)
					}
					continue
				}
				_, readErr := io.Copy(io.Discard, io.LimitReader(response.Body, 32<<20))
				_ = response.Body.Close()
				stats.requests.Add(1)
				stats.latency.Add(time.Since(started).Nanoseconds())
				if readErr != nil {
					stats.networkErrors.Add(1)
				} else if response.StatusCode < 400 {
					stats.success.Add(1)
				} else if response.StatusCode < 500 {
					stats.clientErrors.Add(1)
				} else {
					stats.serverErrors.Add(1)
				}
			}
		}()
	}
	started := time.Now()
	logger.Info("demo traffic started", "rps", c.RPS, "workers", c.Concurrency, "duration", c.Duration.String(), "mutates_source_data", false)
	requestTicker := time.NewTicker(time.Second / time.Duration(c.RPS))
	defer requestTicker.Stop()
	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()
	logStats := func(event string) {
		count := stats.requests.Load()
		mean := float64(0)
		if count > 0 {
			mean = float64(stats.latency.Load()) / float64(count) / float64(time.Millisecond)
		}
		logger.Info(event, "elapsed", time.Since(started).Round(time.Second).String(), "requests", count, "success", stats.success.Load(), "http_4xx", stats.clientErrors.Load(), "http_5xx", stats.serverErrors.Load(), "network_errors", stats.networkErrors.Load(), "skipped_busy", stats.skipped.Load(), "mean_http_ms", mean)
	}
	index := 0
	for {
		select {
		case <-ctx.Done():
			close(jobs)
			workers.Wait()
			logStats("demo traffic finished")
			return stats.success.Load(), nil
		case <-requestTicker.C:
			select {
			case jobs <- index:
				index++
			default:
				stats.skipped.Add(1)
			}
		case <-logTicker.C:
			logStats("demo traffic progress")
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	target := os.Getenv("TARGET_URL")
	if target == "" {
		target = "http://127.0.0.1:8080"
	}
	c := configuration{}
	flag.StringVar(&c.URL, "url", target, "analytics URL")
	flag.DurationVar(&c.Duration, "duration", 3*time.Minute, "duration, at most 15m")
	flag.IntVar(&c.RPS, "rps", 10, "offered requests per second, 1..50")
	flag.IntVar(&c.Concurrency, "concurrency", 4, "workers, 1..16")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	success, err := run(ctx, c, logger)
	if err != nil {
		logger.Error("invalid demo traffic configuration", "error", err)
		os.Exit(2)
	}
	if success == 0 && ctx.Err() == nil {
		logger.Error("no successful requests; start the application first")
		os.Exit(1)
	}
}

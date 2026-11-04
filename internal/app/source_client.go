package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const maxSourceResponseBytes = 16 << 20

func fetch(ctx context.Context, url string, since *time.Time) ([]json.RawMessage, error) {
	return fetchObserved(ctx, url, since, nil, "")
}

func fetchObserved(ctx context.Context, url string, since *time.Time, obs *observability, source string) ([]json.RawMessage, error) {
	items := []json.RawMessage{}
	client := &http.Client{Timeout: duration("HTTP_REQUEST_TIMEOUT", "5s")}
	if obs != nil {
		client.Transport = observedTransport{source: source, metrics: obs}
	}
	for page := 1; ; page++ {
		var result struct {
			Items *[]json.RawMessage `json:"items"`
			More  *bool              `json:"has_more"`
		}
		var last error
		attempts := integer("RETRY_ATTEMPTS", "4")
		if attempts < 1 || attempts > 10 {
			return nil, fmt.Errorf("invalid RETRY_ATTEMPTS")
		}
		for attempt := 0; attempt < attempts; attempt++ {
			req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
			if e != nil {
				return nil, e
			}
			q := req.URL.Query()
			q.Set("page", strconv.Itoa(page))
			q.Set("page_size", "1000")
			if since != nil {
				q.Set("updated_since", since.Format(time.RFC3339Nano))
			}
			req.URL.RawQuery = q.Encode()
			resp, e := client.Do(req)
			wait := time.Duration(200*(1<<attempt)+rand.IntN(100)) * time.Millisecond
			if e == nil {
				if resp.StatusCode == 200 {
					body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxSourceResponseBytes+1))
					_ = resp.Body.Close()
					if readErr != nil {
						last = fmt.Errorf("source body interrupted: %w", readErr)
						if attempt < attempts-1 {
							select {
							case <-ctx.Done():
								return nil, ctx.Err()
							case <-time.After(wait):
							}
						}
						continue
					}
					if len(body) > maxSourceResponseBytes {
						return nil, fmt.Errorf("source response too large")
					}
					decoder := json.NewDecoder(bytes.NewReader(body))
					e = decoder.Decode(&result)
					if e == nil {
						var extra any
						if trailing := decoder.Decode(&extra); trailing != io.EOF {
							e = fmt.Errorf("source response contains trailing data")
						}
					}
					if e != nil {
						return nil, e
					}
					if result.Items == nil || result.More == nil || (*result.More && len(*result.Items) == 0) || len(*result.Items) > 1000 {
						return nil, fmt.Errorf("invalid source pagination envelope")
					}
					last = nil
					break
				}
				last = fmt.Errorf("source status %d", resp.StatusCode)
				if v, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && v > 0 {
					wait = time.Duration(v) * time.Second
				} else if t, err := http.ParseTime(resp.Header.Get("Retry-After")); err == nil && time.Until(t) > 0 {
					wait = time.Until(t)
				}
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				_ = resp.Body.Close()
				if resp.StatusCode != 429 && resp.StatusCode < 500 {
					return nil, last
				}
			} else {
				last = e
			}
			if attempt < attempts-1 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
			}
		}
		if last != nil {
			return nil, last
		}
		items = append(items, (*result.Items)...)
		if !*result.More {
			return items, nil
		}
		if page >= 100000 {
			return nil, fmt.Errorf("pagination limit")
		}
	}
}

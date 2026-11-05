package app

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type config struct {
	port, metricsPort     int
	interval, syncTimeout time.Duration
	mock                  mockConfig
}

type mockConfig struct {
	delay             time.Duration
	failBPS, limitBPS int
}

func loadConfig(kind string) (config, error) {
	var c config
	var err error
	if kind == "bootstrap" {
		return c, nil
	}
	if c.port, err = boundedInt("PORT", "8080", 1, 65535); err != nil {
		return c, err
	}
	if c.metricsPort, err = boundedInt("METRICS_PORT", "9091", 1, 65535); err != nil {
		return c, err
	}
	if c.port == c.metricsPort {
		return c, fmt.Errorf("PORT and METRICS_PORT must be different")
	}
	if kind != "analytics-service" {
		c.mock, err = loadMockConfig()
		return c, err
	}
	if c.interval, err = positiveDuration("SYNC_INTERVAL", "10m"); err != nil {
		return c, err
	}
	if c.syncTimeout, err = positiveDuration("SYNC_TIMEOUT", "2m"); err != nil {
		return c, err
	}
	if _, _, err = sourceClientConfig(); err != nil {
		return c, err
	}
	for _, source := range []struct{ key, fallback string }{
		{"USERS_URL", "http://users-service:8080"},
		{"SUBSCRIPTIONS_URL", "http://subscriptions-service:8080"},
		{"PAYMENTS_URL", "http://payments-service:8080"},
	} {
		base := env(source.key, source.fallback)
		u, parseErr := url.Parse(base)
		if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(base, "?#") {
			return c, fmt.Errorf("%s must be an HTTP or HTTPS base URL", source.key)
		}
	}
	return c, nil
}

func loadMockConfig() (mockConfig, error) {
	var c mockConfig
	var err error
	c.delay, err = time.ParseDuration(env("MOCK_DELAY", "50ms"))
	if err != nil || c.delay < 0 {
		return c, fmt.Errorf("MOCK_DELAY must be a non-negative duration")
	}
	if c.failBPS, err = boundedInt("MOCK_500_BPS", "100", 0, 10000); err != nil {
		return c, err
	}
	if c.limitBPS, err = boundedInt("MOCK_429_BPS", "100", 0, 10000); err != nil {
		return c, err
	}
	if c.failBPS+c.limitBPS > 10000 {
		return c, fmt.Errorf("combined error probabilities exceed 100%%")
	}
	return c, nil
}

func sourceClientConfig() (time.Duration, int, error) {
	timeout, err := positiveDuration("HTTP_REQUEST_TIMEOUT", "5s")
	if err != nil {
		return 0, 0, err
	}
	attempts, err := boundedInt("RETRY_ATTEMPTS", "4", 1, 10)
	return timeout, attempts, err
}

package app

import (
	"context"
	"strings"
	"testing"
)

func TestInvalidConfigurationStopsBeforeConnecting(t *testing.T) {
	for _, tt := range []struct{ service, key, value string }{
		{"analytics-service", "PORT", "0"},
		{"analytics-service", "METRICS_PORT", "8080"},
		{"analytics-service", "SYNC_INTERVAL", "0"},
		{"analytics-service", "SYNC_TIMEOUT", "2"},
		{"analytics-service", "HTTP_REQUEST_TIMEOUT", "-5s"},
		{"analytics-service", "RETRY_ATTEMPTS", "0"},
		{"analytics-service", "PAYMENTS_URL", "file:///payments"},
		{"users-service", "MOCK_DELAY", "-1ms"},
		{"subscriptions-service", "MOCK_500_BPS", "10100"},
		{"payments-service", "MOCK_429_BPS", "10000"},
	} {
		t.Run(tt.key, func(t *testing.T) {
			t.Setenv("PORT", "8080")
			t.Setenv("MOCK_500_BPS", "100")
			t.Setenv("DATABASE_URL", "invalid database connection string")
			t.Setenv(tt.key, tt.value)
			err := run(context.Background(), tt.service)
			expected := tt.key
			if tt.key == "MOCK_429_BPS" {
				expected = "combined error probabilities"
			}
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("want configuration error containing %q, got %v", expected, err)
			}
		})
	}
}

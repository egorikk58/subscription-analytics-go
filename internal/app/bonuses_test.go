package app

import (
	"net/http/httptest"
	"testing"
)

func TestTelegramInputIsExactInt64AndMetricLabelIsBounded(t *testing.T) {
	for _, value := range []string{"0", "-1", "+1", "1.0", "1e9", "9223372036854775808", " 2"} {
		if _, err := parseTelegram(value); err == nil {
			t.Fatal("accepted", value)
		}
	}
	if got, err := parseTelegram("9223372036854775807"); err != nil || got != 9223372036854775807 {
		t.Fatal(got, err)
	}
	if got := metricRoute("/users/by-telegram/9223372036854775807/ltv"); got != "/users/by-telegram/{telegram_id}/ltv" {
		t.Fatal(got)
	}
	for _, query := range []string{"?utm_link=", "?utm_link=%20unknown%20"} {
		value, err := utmParameter(httptest.NewRequest("GET", "/"+query, nil))
		if err != nil || value == nil || *value != "unknown" {
			t.Fatal(value, err)
		}
	}
	if _, err := utmParameter(httptest.NewRequest("GET", "/?utm_link=a&utm_link=b", nil)); err == nil {
		t.Fatal("duplicate UTM accepted")
	}
}

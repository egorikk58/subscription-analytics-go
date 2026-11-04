package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDashboardQueryValidation(t *testing.T) {
	for _, query := range []string{"?page=-1", "?sort=invalid", "?selection=invalid", "?from=bad", "?from=2025-02-01T00:00:00Z&to=2025-01-01T00:00:00Z"} {
		if _, e := dashboardQuery(httptest.NewRequest(http.MethodGet, "/api/dashboard"+query, nil)); e == nil {
			t.Fatal("accepted", query)
		}
	}
	q, e := dashboardQuery(httptest.NewRequest(http.MethodGet, "/api/dashboard?plan_id=monthly&search=123", nil))
	if e != nil || q.Page != 1 || q.Sort != "ltv_desc" || q.Search != "123" || q.Selection != "first_plan" {
		t.Fatal(q, e)
	}
}

func TestPreviewRejectsInvalidGeneration(t *testing.T) {
	s := &service{}
	for _, body := range []string{`{"users_count":9999}`, `{"users_count":100001}`, `{"history_months":25}`, `{"data_as_of":"2099-01-01T00:00:00Z"}`, `{"seed":1.5}`, `{"unknown":true}`, `{} {}`, `{} trailing`} {
		w := httptest.NewRecorder()
		s.preview(w, httptest.NewRequest(http.MethodPost, "/api/demo/preview", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s returned %d", body, w.Code)
		}
	}
}

func TestPreviewDateValidation(t *testing.T) {
	p := previewRequest{Seed: 42, UsersCount: 10000, HistoryMonths: 24, DataAsOf: "2025-01-01T03:00:00+03:00"}
	asof, e := validatePreview(p, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if e != nil || asof.Location() != time.UTC || asof.Hour() != 0 {
		t.Fatal(asof, e)
	}
}

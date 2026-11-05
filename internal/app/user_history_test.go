package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func requestUserHistory(svc *service, telegramID, query, etag string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/users/by-telegram/"+telegramID+"/history?"+query, nil)
	req.SetPathValue("telegram_id", telegramID)
	req.Header.Set("If-None-Match", etag)
	w := httptest.NewRecorder()
	svc.userHistory(w, req)
	return w
}

func exerciseUserHistory(t *testing.T, svc *service, snapshotID int64, asof time.Time) {
	t.Helper()
	query := fmt.Sprintf("snapshot_id=%d", snapshotID)
	seen := make(map[string]bool)
	for _, id := range []string{"1000007919", "1000015838"} {
		w := requestUserHistory(svc, id, query, "")
		var data userHistory
		if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || w.Code != http.StatusOK || !data.DataAsOf.Equal(asof) || len(data.Subscriptions) == 0 {
			t.Fatalf("history: status=%d, decode=%v, body=%s", w.Code, err, w.Body.String())
		}
		var net int64
		var succeeded int
		for i, payment := range data.Payments {
			if payment.UserID != data.UserID || payment.AmountRUBMinor <= 0 || (i > 0 && payment.CreatedAt.After(data.Payments[i-1].CreatedAt)) {
				t.Fatalf("invalid history payment: %+v", payment)
			}
			seen[payment.Status] = true
			switch payment.Status {
			case "succeeded":
				net += payment.AmountRUBMinor
				succeeded++
			case "refunded":
				net -= payment.AmountRUBMinor
			}
		}
		if net != data.LTV || succeeded != data.PaymentsCount {
			t.Fatalf("history does not reconcile: net=%d LTV=%d succeeded=%d payments_count=%d", net, data.LTV, succeeded, data.PaymentsCount)
		}
		etag := w.Header().Get("ETag")
		conditional := requestUserHistory(svc, id, query, etag)
		if etag != fmt.Sprintf("%q", fmt.Sprint(snapshotID)) || conditional.Code != http.StatusNotModified || conditional.Body.Len() != 0 {
			t.Fatalf("unexpected conditional history response: etag=%q status=%d", etag, conditional.Code)
		}
	}
	for _, status := range []string{"succeeded", "failed", "refunded"} {
		if !seen[status] {
			t.Fatalf("history lacks guaranteed %s fixture", status)
		}
	}
	for _, tc := range []struct {
		id, query string
		want      int
	}{
		{"0", query, http.StatusBadRequest},
		{"1000007919", "snapshot_id=-1", http.StatusBadRequest},
		{"9223372036854775807", query, http.StatusNotFound},
		{"1000007919", "snapshot_id=9223372036854775807", http.StatusGone},
	} {
		if w := requestUserHistory(svc, tc.id, tc.query, ""); w.Code != tc.want {
			t.Errorf("history %s?%s status=%d want=%d", tc.id, tc.query, w.Code, tc.want)
		}
	}
}

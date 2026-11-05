package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"subscription-analytics/internal/core"
	"testing"
)

func TestPreviewSizeFollowsSuppliedSeedWhenCountOmitted(t *testing.T) {
	seed := int64(731)
	w := httptest.NewRecorder()
	(&service{}).preview(w, httptest.NewRequest(http.MethodPost, "/api/demo/preview", strings.NewReader(`{"seed":731}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		Dataset struct {
			UsersCount int `json:"users_count"`
		} `json:"dataset"`
		Users         core.UserPage          `json:"users"`
		UserHistories map[string]userHistory `json:"user_histories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Dataset.UsersCount != core.AutoUsersCount(seed) {
		t.Fatalf("omitted size was not derived from the actual seed: %d %v", result.Dataset.UsersCount, err)
	}
	if len(result.UserHistories) != len(result.Users.Items) || len(result.Users.Items) == 0 {
		t.Fatal("expected preview history for each visible user")
	}
	for _, row := range result.Users.Items {
		history, ok := result.UserHistories[row.ID]
		if !ok || history.UserID != row.ID || history.LTV != row.LTV || history.Payments == nil || history.Subscriptions == nil {
			t.Fatalf("invalid preview history for %s", row.ID)
		}
	}
}

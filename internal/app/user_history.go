package app

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"subscription-analytics/internal/core"
)

type userHistoryPayment struct {
	*core.Payment
	AmountRUBMinor int64 `json:"amount_rub_minor"`
}

type userHistory struct {
	core.UserLTV
	Subscriptions []*core.Subscription `json:"subscriptions"`
	Payments      []userHistoryPayment `json:"payments"`
}

type userHistoryIndex struct {
	subscriptions map[string][]*core.Subscription
	payments      map[string][]*core.Payment
}

// Build the grouping once for the entire sync. The index refers to the
// validated source records and lives only for the snapshot publication.
func indexUserHistories(d core.Dataset, observedThrough time.Time, selected map[string]struct{}) userHistoryIndex {
	capacity := len(d.Users)
	if selected != nil {
		capacity = len(selected)
	}
	index := userHistoryIndex{
		subscriptions: make(map[string][]*core.Subscription, capacity),
		payments:      make(map[string][]*core.Payment, capacity),
	}
	included := func(id string) bool {
		_, ok := selected[id]
		return selected == nil || ok
	}
	for i := range d.Subscriptions {
		sub := &d.Subscriptions[i]
		if included(sub.UserID) && !sub.StartedAt.After(observedThrough) {
			index.subscriptions[sub.UserID] = append(index.subscriptions[sub.UserID], sub)
		}
	}
	for i := range d.Payments {
		payment := &d.Payments[i]
		if included(payment.UserID) && !payment.CreatedAt.After(observedThrough) {
			index.payments[payment.UserID] = append(index.payments[payment.UserID], payment)
		}
	}
	for _, subscriptions := range index.subscriptions {
		slices.SortFunc(subscriptions, func(a, b *core.Subscription) int {
			if order := b.StartedAt.Compare(a.StartedAt); order != 0 {
				return order
			}
			return strings.Compare(b.ID, a.ID)
		})
	}
	for _, payments := range index.payments {
		slices.SortFunc(payments, func(a, b *core.Payment) int {
			if order := b.CreatedAt.Compare(a.CreatedAt); order != 0 {
				return order
			}
			return strings.Compare(b.ID, a.ID)
		})
	}
	return index
}

func (index userHistoryIndex) project(profile core.UserLTV) (userHistory, error) {
	subscriptions := index.subscriptions[profile.UserID]
	if subscriptions == nil {
		subscriptions = []*core.Subscription{}
	}
	sourcePayments := index.payments[profile.UserID]
	history := userHistory{UserLTV: profile, Subscriptions: subscriptions, Payments: make([]userHistoryPayment, 0, len(sourcePayments))}
	for _, payment := range sourcePayments {
		amount, err := core.ConvertedMinor(*payment)
		if err != nil {
			return userHistory{}, err
		}
		history.Payments = append(history.Payments, userHistoryPayment{Payment: payment, AmountRUBMinor: amount})
	}
	return history, nil
}

func previewUserHistories(d core.Dataset, snap core.Snapshot, users []core.UserRow) (map[string]userHistory, error) {
	result := make(map[string]userHistory, len(users))
	if len(users) == 0 {
		return result, nil
	}
	selected := make(map[string]struct{}, len(users))
	for _, user := range users {
		selected[user.ID] = struct{}{}
	}
	index := indexUserHistories(d, snap.DataAsOf, selected)
	for _, fact := range snap.Facts {
		if _, ok := selected[fact.ID]; !ok {
			continue
		}
		history, err := index.project(snap.UserLTV(fact))
		if err != nil {
			return nil, err
		}
		result[fact.ID] = history
	}
	return result, nil
}

func (s *service) userHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	telegramID, paramErr := parseTelegram(r.PathValue("telegram_id"))
	var requestedSnapshot *int64
	if raw := r.URL.Query().Get("snapshot_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			paramErr = fmt.Errorf("invalid snapshot_id")
		} else {
			requestedSnapshot = &id
		}
	}
	var snapshotID *int64
	var asof *time.Time
	var ready, userFound bool
	var history []byte
	err := s.db.QueryRow(r.Context(), `
SELECT snapshot.id, COALESCE(snapshot.data_as_of, current_snapshot.data_as_of),
       COALESCE(snapshot.history_ready, false), profile.telegram_id IS NOT NULL, profile.history
FROM analytics.state AS state
LEFT JOIN analytics.snapshots AS current_snapshot ON current_snapshot.id=state.current_id
LEFT JOIN analytics.snapshots AS snapshot ON snapshot.id=COALESCE($2::bigint, state.current_id)
LEFT JOIN analytics.user_ltv AS profile ON profile.snapshot_id=snapshot.id AND profile.telegram_id=$1
WHERE state.id=1`, telegramID, requestedSnapshot).Scan(&snapshotID, &asof, &ready, &userFound, &history)
	if paramErr != nil {
		write(w, http.StatusBadRequest, map[string]any{"error": paramErr.Error(), "data_as_of": asof})
		return
	}
	if err == nil && requestedSnapshot != nil && snapshotID == nil {
		write(w, http.StatusGone, map[string]any{"error": "requested snapshot unavailable", "data_as_of": asof})
		return
	}
	if err != nil || !ready || (userFound && history == nil) {
		w.Header().Set("Retry-After", "5")
		write(w, http.StatusServiceUnavailable, map[string]any{"error": "user history snapshot unavailable", "data_as_of": asof})
		return
	}
	if !userFound {
		write(w, http.StatusNotFound, map[string]any{"error": "user not found", "data_as_of": asof})
		return
	}

	etag := fmt.Sprintf("\"%d\"", *snapshotID)
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", asof.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Data-As-Of", asof.UTC().Format(time.RFC3339Nano))
	w.Header().Set("X-Snapshot-Version", etag)
	for _, candidate := range strings.Split(r.Header.Get("If-None-Match"), ",") {
		tag := strings.TrimSpace(candidate)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(history)
}

package app

import (
	"encoding/json"
	"testing"
	"time"

	"subscription-analytics/internal/core"
)

func TestPublishedSnapshotUsesObservationWindow(t *testing.T) {
	started := time.Date(2025, 10, 15, 0, 0, 0, 0, time.UTC)
	seedCutoff := started.AddDate(0, 0, 17)
	firstEnd := started.AddDate(0, 1, 0)
	secondEnd := started.AddDate(0, 2, 0)
	poll := started.AddDate(0, 3, 0)
	config := "v3/42/10000/24/" + seedCutoff.Format(time.RFC3339)
	d := core.Dataset{
		Users:         []core.User{{ID: "u", CreatedAt: started, UpdatedAt: poll}},
		Plans:         []core.Plan{{ID: "monthly", PeriodMonths: 1, Price: 100, Currency: "RUB"}},
		Subscriptions: []core.Subscription{{ID: "s", UserID: "u", PlanID: "monthly", StartedAt: started, CurrentPeriodEnd: firstEnd, Status: "active"}},
		Payments:      []core.Payment{{ID: "first", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: started, PeriodEnd: firstEnd, CreatedAt: started}},
	}
	check := func(poll, observed time.Time, eligible, renewed int, ltv int64) {
		t.Helper()
		snapshot, err := preparePublishedSnapshot(d, config, poll)
		if err != nil {
			t.Fatal(err)
		}
		// Read the same persisted facts used by filtered API and dashboard requests.
		stored, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		snapshot = core.Snapshot{}
		if err = json.Unmarshal(stored, &snapshot); err != nil {
			t.Fatal(err)
		}
		if !snapshot.DataAsOf.Equal(poll) || snapshot.Dataset.DatasetAsOf == nil || !snapshot.Dataset.DatasetAsOf.Equal(seedCutoff) || snapshot.Dataset.ObservedThrough == nil || !snapshot.Dataset.ObservedThrough.Equal(observed) {
			t.Fatalf("incorrect timestamps: publication=%v seed=%v observation=%v", snapshot.DataAsOf, snapshot.Dataset.DatasetAsOf, snapshot.Dataset.ObservedThrough)
		}
		body, err := snapshot.Response("retention", core.Filter{Plan: "monthly", Periods: 1})
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			DataAsOf time.Time        `json:"data_as_of"`
			Periods  []core.PeriodRow `json:"periods"`
		}
		if err = json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		row := result.Periods[0]
		if !result.DataAsOf.Equal(poll) || row.Eligible != eligible || row.Renewed != renewed || row.CumulativeEligible != eligible {
			t.Fatal(string(body))
		}
		dashboard, err := snapshot.Dashboard(core.DashboardQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if dashboard.Retention[0].Periods[0].Eligible != eligible || dashboard.Cohorts[0].Periods[0].Renewed != renewed {
			t.Fatal("dashboard and metric used different observation windows")
		}
		if dashboard.Dataset.ActivePeriods != 1 || snapshot.Facts[0].CurrentStatus != "active" || snapshot.Facts[0].LTV != ltv {
			t.Fatalf("source status or payment facts used the wrong cutoff: %+v", snapshot.Facts[0])
		}
	}
	check(poll, seedCutoff, 0, 0, 100)
	check(poll.AddDate(0, 6, 0), seedCutoff, 0, 0, 100)

	// A new billing event advances observation. A payment dated after the
	// poll is excluded from LTV even if returned by an older mock.
	d.Subscriptions[0].CurrentPeriodEnd = secondEnd
	d.Payments = append(d.Payments,
		core.Payment{ID: "renewal", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: firstEnd, PeriodEnd: secondEnd, CreatedAt: firstEnd},
		core.Payment{ID: "future", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: secondEnd, PeriodEnd: poll, CreatedAt: poll.Add(time.Hour)},
	)
	check(poll, firstEnd, 1, 1, 200)
}

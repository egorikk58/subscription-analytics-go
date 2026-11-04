package core

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
	"time"
)

func TestBoundary(t *testing.T) {
	start := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
	if got := Boundary(start, 1).Format("2006-01-02"); got != "2024-02-29" {
		t.Fatal(got)
	}
	if got := Boundary(start, 2).Format("2006-01-02"); got != "2024-03-31" {
		t.Fatal(got)
	}
}

func TestBoundaryNormalizesUTC(t *testing.T) {
	start, err := time.Parse(time.RFC3339, "2024-01-31T03:00:00+03:00")
	if err != nil {
		t.Fatal(err)
	}
	if got := Boundary(start, 1).Format(time.RFC3339); got != "2024-02-29T00:00:00Z" {
		t.Fatalf("calendar boundary changed the instant's UTC clock: %s", got)
	}
}

func TestPrepareEquivalentTimezonePeriods(t *testing.T) {
	parse := func(raw string) time.Time {
		t.Helper()
		v, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	start := parse("2025-01-31T03:00:00+03:00")
	next := parse("2025-02-28T03:00:00+03:00")
	end := parse("2025-03-31T00:00:00Z")
	d := Dataset{
		Users:         []User{{ID: "u"}},
		Plans:         []Plan{{ID: "m", PeriodMonths: 1, Price: 100, Currency: "RUB"}},
		Subscriptions: []Subscription{{ID: "s", UserID: "u", PlanID: "m", StartedAt: start}},
		Payments: []Payment{
			{ID: "first", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: start, PeriodEnd: next, CreatedAt: start},
			{ID: "renewal", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: next.UTC(), PeriodEnd: end, CreatedAt: next.UTC()},
		},
	}
	s, err := Prepare(d, next.UTC())
	if err != nil {
		t.Fatal(err)
	}
	// Two payments of 100 minor units form two adjacent monthly periods,
	// regardless of whether the same instant is encoded with +03:00 or Z.
	if s.Facts[0].LTV != 200 || s.Facts[0].Chains[0].Paid != 2 {
		t.Fatalf("equivalent timestamps broke the paid chain: %+v", s.Facts[0])
	}
	raw, err := s.Response("retention", Filter{Plan: "m", Periods: 1})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Periods []struct{ Eligible, Renewed int }
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.Periods[0].Eligible != 1 || result.Periods[0].Renewed != 1 {
		t.Fatal(string(raw))
	}
}

func TestRetentionAllTariffDurations(t *testing.T) {
	start := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		months int
		end    string
	}{{1, "2025-02-28"}, {3, "2025-04-30"}, {6, "2025-07-31"}, {12, "2026-01-31"}} {
		t.Run(tc.end, func(t *testing.T) {
			now, err := time.Parse("2006-01-02", tc.end)
			if err != nil {
				t.Fatal(err)
			}
			s := Snapshot{DataAsOf: now, Plans: []Plan{{ID: "p"}}, Facts: []Fact{{First: &start, Chains: []Chain{{"p", start, tc.months, 2}}}}}
			raw, err := s.Response("retention", Filter{Plan: "p", Periods: 2})
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Periods []struct {
					Eligible int
					Renewed  int
					Rate     *float64
				}
			}
			if err = json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if result.Periods[0].Eligible != 1 || result.Periods[0].Renewed != 1 || *result.Periods[0].Rate != 1 || result.Periods[1].Eligible != 0 || result.Periods[1].Rate != nil {
				t.Fatal(string(raw))
			}
		})
	}
}
func TestSummaryExact(t *testing.T) {
	m, e := summary([]Fact{{LTV: 0}, {LTV: 1}, {LTV: 2}, {LTV: 4}})
	if e != nil {
		t.Fatal(e)
	}
	if m["total_revenue_minor"] != int64(7) || m["mean_ltv_minor"] != "1.75" || m["median_ltv_minor"] != "1.50" {
		t.Fatal(m)
	}
	empty, _ := summary(nil)
	if empty["mean_ltv_minor"] != "0.00" {
		t.Fatal(empty)
	}
}

func TestPrepareRefundNetAtInt64Limit(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := Boundary(start, 1)
	d := Dataset{
		Users:         []User{{ID: "u"}},
		Plans:         []Plan{{ID: "m", PeriodMonths: 1, Price: math.MaxInt64, Currency: "RUB"}},
		Subscriptions: []Subscription{{ID: "s", UserID: "u", PlanID: "m", StartedAt: start}},
		Payments: []Payment{
			{ID: "original", UserID: "u", SubscriptionID: "s", Amount: math.MaxInt64, Currency: "RUB", Status: "succeeded", PeriodStart: start, PeriodEnd: end, CreatedAt: start},
			{ID: "refund", RefundOf: "original", UserID: "u", SubscriptionID: "s", Amount: math.MaxInt64, Currency: "RUB", Status: "refunded", PeriodStart: start, PeriodEnd: end, CreatedAt: start},
			{ID: "replacement", UserID: "u", SubscriptionID: "s", Amount: math.MaxInt64, Currency: "RUB", Status: "succeeded", PeriodStart: start, PeriodEnd: end, CreatedAt: start},
		},
	}
	// MaxInt64 - MaxInt64 + MaxInt64 is representable. Map iteration order
	// must not make this valid net amount fail on an intermediate addition.
	for attempt := 0; attempt < 100; attempt++ {
		s, err := Prepare(d, end)
		if err != nil {
			t.Fatal(err)
		}
		if s.Facts[0].LTV != math.MaxInt64 {
			t.Fatalf("incorrect net LTV: %d", s.Facts[0].LTV)
		}
	}
	// An actually unrepresentable final amount must still be rejected.
	d.Payments = append(d.Payments[:1], d.Payments[2])
	if _, err := Prepare(d, end); err == nil {
		t.Fatal("unrepresentable final LTV was accepted")
	}
}
func TestRetentionMaturity(t *testing.T) {
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	young := time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)
	s := Snapshot{DataAsOf: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), Plans: []Plan{{ID: "m"}}}
	for i := 0; i < 100; i++ {
		start, paid := old, 1
		if i >= 50 {
			start = young
		}
		if i < 30 {
			paid = 3
		} else if i < 40 {
			paid = 2
		}
		s.Facts = append(s.Facts, Fact{First: &start, Chains: []Chain{{"m", start, 1, paid}}})
	}
	b, e := s.Response("retention", Filter{Plan: "m", Periods: 2})
	if e != nil {
		t.Fatal(e)
	}
	var got struct {
		Periods []struct {
			Eligible, Renewed int
			Mature            int      `json:"cumulative_eligible"`
			Rate              *float64 `json:"rate"`
			Cumulative        *float64 `json:"cumulative_rate"`
		}
	}
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	p := got.Periods[1]
	if p.Eligible != 40 || p.Renewed != 30 || p.Mature != 50 || *p.Rate != .75 || *p.Cumulative != .6 {
		t.Fatal(string(b))
	}
}
func TestRefundSwitchAndLifetimeFilter(t *testing.T) {
	a := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	d := Dataset{Users: []User{{ID: "u"}, {ID: "zero"}}, Plans: []Plan{{"m", "Monthly", 1, 100, "RUB"}, {"y", "Year", 12, 1000, "RUB"}}, Subscriptions: []Subscription{{ID: "s", UserID: "u", PlanID: "m", StartedAt: a}, {ID: "new", UserID: "u", PlanID: "y", StartedAt: Boundary(a, 1)}}, Payments: []Payment{{ID: "p", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: a, PeriodEnd: Boundary(a, 1), CreatedAt: a}, {ID: "refund", UserID: "u", SubscriptionID: "s", Amount: 100, Currency: "RUB", Status: "refunded", RefundOf: "p", PeriodStart: a, PeriodEnd: Boundary(a, 1), CreatedAt: a}, {ID: "year", UserID: "u", SubscriptionID: "new", Amount: 1000, Currency: "RUB", Status: "succeeded", PeriodStart: Boundary(a, 1), PeriodEnd: Boundary(a, 13), CreatedAt: Boundary(a, 1)}}}
	s, e := Prepare(d, Boundary(a, 3))
	if e != nil {
		t.Fatal(e)
	}
	if s.Facts[0].LTV != 1000 || s.Facts[0].FirstPlan != "m" || s.Facts[0].Chains[0].Paid != 0 {
		t.Fatal(s)
	}
	to := Boundary(a, 1)
	b, e := s.Response("summary", Filter{From: &a, To: &to})
	if e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	if got["total_revenue_minor"] != float64(1000) || got["users_count"] != float64(1) {
		t.Fatal(string(b))
	}
}
func TestGeneratorReproducible(t *testing.T) {
	asof := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a := Generate(42, 10000, 24, asof)
	b := Generate(42, 10000, 24, asof)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("not deterministic")
	}
	if _, e := Prepare(a, asof); e != nil {
		t.Fatal(e)
	}
	ids := map[int64]bool{}
	registered := map[string]time.Time{}
	refunds, failed := 0, 0
	for _, u := range a.Users {
		registered[u.ID] = u.CreatedAt
		if ids[u.TelegramID] {
			t.Fatal("duplicate telegram id")
		}
		ids[u.TelegramID] = true
	}
	for _, p := range a.Payments {
		if p.CreatedAt.Before(registered[p.UserID]) {
			t.Fatal("payment predates registration")
		}
		if p.Status == "refunded" {
			refunds++
		}
		if p.Status == "failed" {
			failed++
		}
	}
	if refunds == 0 || failed == 0 {
		t.Fatal("missing edge cases")
	}
}

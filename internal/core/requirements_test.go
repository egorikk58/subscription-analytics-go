package core

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestGeneratedSourceContractsAndRequiredScenariosAcrossSeeds(t *testing.T) {
	cutoff := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for _, seed := range []int64{42, 137, 2026} {
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			d := Generate(seed, 10000, 18, cutoff)
			if len(d.Users) != 10000 || len(d.Plans) != 4 {
				t.Fatal("wrong source volume or tariff count")
			}
			oldest := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
			users, telegrams := map[string]bool{}, map[int64]bool{}
			for _, user := range d.Users {
				if !uuid.MatchString(user.ID) || users[user.ID] || telegrams[user.TelegramID] || user.TelegramID < 100000000 || user.CreatedAt.Before(oldest) || user.CreatedAt.After(cutoff) {
					t.Fatal("invalid user identity or history boundary")
				}
				users[user.ID], telegrams[user.TelegramID] = true, true
			}
			plans, durations := map[string]bool{}, map[int]bool{}
			for _, plan := range d.Plans {
				plans[plan.ID], durations[plan.PeriodMonths] = true, true
			}
			for _, months := range []int{1, 3, 6, 12} {
				if !durations[months] {
					t.Fatal("missing tariff duration", months)
				}
			}
			subscriptions := map[string]Subscription{}
			for _, subscription := range d.Subscriptions {
				if !users[subscription.UserID] || !plans[subscription.PlanID] {
					t.Fatal("orphan subscription")
				}
				subscriptions[subscription.ID] = subscription
			}
			payments, succeededPeriods := map[string]Payment{}, map[string]bool{}
			for _, payment := range d.Payments {
				if !uuid.MatchString(payment.ID) || payments[payment.ID].ID != "" || !users[payment.UserID] || subscriptions[payment.SubscriptionID].UserID != payment.UserID || payment.Amount <= 0 || payment.CreatedAt.After(cutoff) {
					t.Fatal("invalid payment identity, relation, amount or history")
				}
				payments[payment.ID] = payment
				if payment.Status == "succeeded" {
					succeededPeriods[payment.SubscriptionID+payment.PeriodStart.String()] = true
				}
			}
			retry := false
			for _, payment := range d.Payments {
				if payment.Status == "failed" && succeededPeriods[payment.SubscriptionID+payment.PeriodStart.String()] {
					retry = true
				}
				if payment.Status == "refunded" {
					original := payments[payment.RefundOf]
					if original.Status != "succeeded" || original.Amount != payment.Amount || original.Currency != payment.Currency {
						t.Fatal("refund is not linked to a complete original charge")
					}
				}
			}
			proof := DescribeDataset(d, cutoff)
			if !retry || proof.RefundedCount == 0 || proof.SwitchesCount == 0 || proof.UnpaidUsers == 0 || proof.ActivePeriods == 0 {
				t.Fatal("missing required generated scenario", proof)
			}
		})
	}
}

func TestTariffSwitchEndsOldRetentionChainButKeepsLifetimeRevenue(t *testing.T) {
	jan := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	d := Dataset{
		Users: []User{{ID: "u"}},
		Plans: []Plan{{"m", "Monthly", 1, 100, "RUB"}, {"y", "Annual", 12, 1000, "RUB"}},
		Subscriptions: []Subscription{
			{ID: "new", UserID: "u", PlanID: "y", StartedAt: feb},
			{ID: "old", UserID: "u", PlanID: "m", StartedAt: jan},
		},
		Payments: []Payment{
			{ID: "first", UserID: "u", SubscriptionID: "old", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: jan, PeriodEnd: feb, CreatedAt: jan},
			{ID: "old-charge-after-switch", UserID: "u", SubscriptionID: "old", Amount: 100, Currency: "RUB", Status: "succeeded", PeriodStart: feb, PeriodEnd: mar, CreatedAt: feb},
			{ID: "year", UserID: "u", SubscriptionID: "new", Amount: 1000, Currency: "RUB", Status: "succeeded", PeriodStart: feb, PeriodEnd: yearEnd, CreatedAt: feb},
		},
	}
	before := append([]Subscription(nil), d.Subscriptions...)
	s, err := Prepare(d, mar)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, d.Subscriptions) {
		t.Fatal("preparation changed the caller's subscription order")
	}
	if s.Facts[0].LTV != 1200 || s.Facts[0].FirstPlan != "m" {
		t.Fatal("all 100 + 100 + 1000 minor units must remain in the first-plan cohort", s.Facts[0])
	}
	if s.Facts[0].Chains[0].Paid != 1 || s.Facts[0].Chains[1].Paid != 1 {
		t.Fatalf("switch did not close the monthly chain: %+v", s.Facts[0].Chains)
	}
	encoded, err := s.Response("retention", Filter{Plan: "m", Periods: 2})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Periods []struct{ Eligible, Renewed int }
	}
	if err = json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	if result.Periods[0].Eligible != 1 || result.Periods[0].Renewed != 0 || result.Periods[1].Eligible != 0 {
		t.Fatal(string(encoded))
	}
}

func TestCumulativeMaturityIsNotProductOfConditionalRates(t *testing.T) {
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	young := time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC)
	s := Snapshot{DataAsOf: time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC), Plans: []Plan{{ID: "m"}}}
	for i := 0; i < 100; i++ {
		start, paid := old, 1
		switch {
		case i < 30:
			paid = 3
		case i < 40:
			paid = 2
		case i >= 50:
			start = young
			if i < 60 {
				paid = 2
			}
		}
		s.Facts = append(s.Facts, Fact{First: &start, Chains: []Chain{{"m", start, 1, paid}}})
	}
	body, err := s.Response("retention", Filter{Plan: "m", Periods: 2})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Periods []struct {
			Eligible   int
			Renewed    int
			Mature     int     `json:"cumulative_eligible"`
			Rate       float64 `json:"rate"`
			Cumulative float64 `json:"cumulative_rate"`
		}
	}
	if err = json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	// Period 1: 50/100. Period 2: 30/40 conditionally, 30/50 among
	// mature original participants; 0.5 * 0.75 = 0.375 is not 0.6.
	a, b := result.Periods[0], result.Periods[1]
	if a.Eligible != 100 || a.Renewed != 50 || a.Rate != .5 || b.Eligible != 40 || b.Renewed != 30 || b.Mature != 50 || b.Rate != .75 || b.Cumulative != .6 {
		t.Fatal(string(body))
	}
}

func TestSummaryOddCountAndHalfAwayRounding(t *testing.T) {
	got, err := summary([]Fact{{LTV: 0}, {LTV: 100}, {LTV: 101}})
	if err != nil || got["users_count"] != 3 || got["total_revenue_minor"] != int64(201) || got["mean_ltv_minor"] != "67.00" || got["median_ltv_minor"] != "100.00" {
		t.Fatal(got, err)
	}
	for _, total := range []int64{1, -1} {
		facts := make([]Fact, 200)
		facts[0].LTV = total
		got, err := summary(facts)
		want := "0.01"
		if total < 0 {
			want = "-0.01"
		}
		if err != nil || got["mean_ltv_minor"] != want {
			t.Fatalf("half-minor-unit hundredth rounding: %v %v", got, err)
		}
	}
}

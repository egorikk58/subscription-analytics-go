package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDashboardCohortRevenueAndSelection(t *testing.T) {
	jan := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	snap := Snapshot{DataAsOf: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC), Plans: []Plan{{ID: "monthly"}, {ID: "yearly"}}, Facts: []Fact{
		{ID: "one", TelegramID: 123, First: &jan, FirstPlan: "monthly", LTV: 100000, Chains: []Chain{{"monthly", jan, 1, 3}}},
		{ID: "two", TelegramID: 456, First: &jan, FirstPlan: "monthly", LTV: 300000, Chains: []Chain{{"monthly", jan, 1, 1}}},
		{ID: "three", TelegramID: 789, First: &feb, FirstPlan: "yearly", LTV: 900000, Chains: []Chain{{"yearly", feb, 12, 1}}},
		{ID: "zero", LTV: 0},
	}}
	report, e := snap.Dashboard(DashboardQuery{Plan: "all", Page: 1})
	if e != nil {
		t.Fatal(e)
	}
	if report.Summary["total_revenue_minor"] != int64(1300000) || report.Summary["users_count"] != 4 || len(report.Timeline) != 2 || report.Timeline[0]["total_revenue_minor"] != int64(400000) {
		t.Fatalf("incorrect aggregates: %+v", report)
	}
	if report.Cohorts[0].UsersCount != 2 || report.Cohorts[0].Periods[0].Eligible != 2 || report.Cohorts[0].Periods[0].Renewed != 1 || *report.Cohorts[0].Periods[0].CumulativeRate != .5 {
		t.Fatal(report.Cohorts)
	}
	if len(report.Retention[0].Periods) != 24 || len(report.Cohorts[0].Periods) != 12 {
		t.Fatal("retention must expose 24 periods and heatmap 12")
	}
	selected, e := snap.Dashboard(DashboardQuery{Plan: "monthly", Search: "123", Sort: "ltv_asc"})
	if e != nil {
		t.Fatal(e)
	}
	if selected.Summary["users_count"] != 2 || selected.Users.Total != 1 || selected.Users.Items[0].ID != "one" {
		t.Fatal(selected)
	}
	dateSelected, e := snap.Dashboard(DashboardQuery{From: &feb, Plan: "all"})
	if e != nil {
		t.Fatal(e)
	}
	if dateSelected.Summary["users_count"] != 1 || dateSelected.Summary["total_revenue_minor"] != int64(900000) {
		t.Fatal(dateSelected)
	}
}

func TestDatasetFingerprintAndSeedEvidence(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	d := Generate(42, 10000, 24, now)
	s, e := Prepare(d, now)
	if e != nil {
		t.Fatal(e)
	}
	if s.Dataset.UsersCount != 10000 || s.Dataset.FailedCount == 0 || s.Dataset.RefundedCount == 0 || s.Dataset.SwitchesCount == 0 || s.Dataset.UnpaidUsers == 0 {
		t.Fatal(s.Dataset)
	}
	shuffled := d
	shuffled.Users = append([]User(nil), d.Users...)
	for i, j := 0, len(shuffled.Users)-1; i < j; i, j = i+1, j-1 {
		shuffled.Users[i], shuffled.Users[j] = shuffled.Users[j], shuffled.Users[i]
	}
	if DescribeDataset(shuffled, now).Fingerprint != s.Dataset.Fingerprint {
		t.Fatal("fingerprint depends on source ordering")
	}
	if e = s.Dataset.SetConfig("v1/42/10000/24/2026-11-01T00:00:00Z"); e != nil {
		t.Fatal(e)
	}
	if *s.Dataset.Seed != 42 || s.Dataset.HistoryMonths != 24 {
		t.Fatal(s.Dataset)
	}
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored Snapshot
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	if restored.Facts[0].TelegramID == 0 || restored.Dataset.Fingerprint != s.Dataset.Fingerprint {
		t.Fatal("snapshot lost dashboard evidence")
	}
	changed := DescribeDataset(Generate(43, 10000, 24, now), now)
	if changed.Fingerprint == s.Dataset.Fingerprint {
		t.Fatal("seed does not affect dataset")
	}
	statuses := map[string]bool{}
	for _, p := range s.Dataset.Samples.Payments {
		statuses[p.Status] = true
	}
	if !statuses["succeeded"] || !statuses["failed"] || !statuses["refunded"] {
		t.Fatal("sample misses required payment cases")
	}
}

func TestDashboardEmptyAndInvalidPlan(t *testing.T) {
	s := Snapshot{Plans: []Plan{{ID: "monthly"}}}
	if _, e := s.Dashboard(DashboardQuery{Plan: "unknown"}); e == nil {
		t.Fatal("accepted invalid plan")
	}
	r, e := s.Dashboard(DashboardQuery{Plan: "all", Page: 900})
	if e != nil || len(r.Users.Items) != 0 || r.Users.Total != 0 || r.Summary["users_count"] != 0 {
		t.Fatal(r, e)
	}
	s.Facts = []Fact{{ID: "z"}, {ID: "a"}}
	r, e = s.Dashboard(DashboardQuery{Sort: "joined_desc"})
	if e != nil || r.Users.Items[0].ID != "a" || r.Users.Items[1].ID != "z" {
		t.Fatal("nil-date tie order is unstable", r, e)
	}
}

func TestDashboardChainDrilldownIncludesMigrators(t *testing.T) {
	jan := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	snap := Snapshot{DataAsOf: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Plans: []Plan{{ID: "monthly"}, {ID: "yearly"}}, Facts: []Fact{
		{ID: "migrator", First: &jan, FirstPlan: "monthly", LTV: 1000, Chains: []Chain{{"monthly", jan, 1, 1}, {"yearly", feb, 12, 2}}},
		{ID: "direct", First: &jan, FirstPlan: "yearly", LTV: 2000, Chains: []Chain{{"yearly", jan, 12, 2}}},
		{ID: "monthly-only", First: &jan, FirstPlan: "monthly", LTV: 300, Chains: []Chain{{"monthly", jan, 1, 2}}},
	}}
	direct, e := snap.Dashboard(DashboardQuery{Plan: "yearly", From: &jan, To: &feb})
	if e != nil {
		t.Fatal(e)
	}
	chain, e := snap.Dashboard(DashboardQuery{Plan: "yearly", Selection: "chain", From: &jan, To: &feb})
	if e != nil {
		t.Fatal(e)
	}
	if direct.Users.Total != 1 || chain.Users.Total != 2 || chain.Summary["users_count"] != 2 || chain.Summary["total_revenue_minor"] != int64(3000) || len(chain.Cohorts) != 1 || chain.Cohorts[0].UsersCount != chain.Users.Total {
		t.Fatal("drilldown lost a migrating user", direct, chain)
	}
	if chain.Plans[0]["users_count"] != 1 || chain.Plans[1]["users_count"] != 1 || chain.Plans[0]["total_revenue_minor"] != int64(1000) {
		t.Fatal("chain drilldown changed original LTV attribution", chain.Plans)
	}
}

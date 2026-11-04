package core

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestConversionUsesAtomicUnitsAndExactRounding(t *testing.T) {
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		currency string
		amount   int64
		rate     string
		want     int64
	}{
		{"RUB", 101, "", 101}, {"BTC", 1, "500000", 1}, {"BTC", 1, "499999", 0}, {"BTC", 12345678, "10000000", 123456780}, {"USDT", 1234567, "100", 12346},
	} {
		p := Payment{Currency: test.currency, Amount: test.amount, CreatedAt: at}
		if test.rate != "" {
			p.FX = &ExchangeRate{"test", test.rate, at, "manual-fixture"}
		}
		got, err := ConvertedMinor(p)
		if err != nil || got != test.want {
			t.Fatalf("%+v: %d %v", test, got, err)
		}
	}
	for _, rate := range []string{"0", "-1", "NaN", "1e8", "1/2", "1,5"} {
		_, err := ConvertedMinor(Payment{Currency: "BTC", Amount: 1, CreatedAt: at, FX: &ExchangeRate{"bad", rate, at, "test"}})
		if err == nil {
			t.Fatal("accepted", rate)
		}
	}
	for _, p := range []Payment{{Currency: "BTC", Amount: 1, CreatedAt: at}, {Currency: "ETH", Amount: 1}, {Currency: "USDT", Amount: math.MaxInt64, CreatedAt: at, FX: &ExchangeRate{"big", "999999999999", at, "test"}}} {
		if _, err := ConvertedMinor(p); err == nil {
			t.Fatal("accepted invalid/overflowing payment")
		}
	}
}

func TestMixedCurrenciesRefundAndTelegramProfile(t *testing.T) {
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	d := Dataset{Users: []User{{ID: "u", TelegramID: math.MaxInt64, UTMLink: " ads "}, {ID: "zero", TelegramID: 2}}, Plans: []Plan{{ID: "m", Price: 100, Currency: "RUB", PeriodMonths: 1}}, Subscriptions: []Subscription{{ID: "s", UserID: "u", PlanID: "m", StartedAt: at, Status: "active", CurrentPeriodEnd: Boundary(at, 3)}}}
	base := Payment{UserID: "u", SubscriptionID: "s", Currency: "RUB", Amount: 100, Status: "succeeded", PeriodStart: at, PeriodEnd: Boundary(at, 1), CreatedAt: at}
	rub := base
	rub.ID = "rub"
	btc := base
	btc.ID = "btc"
	btc.Currency = "BTC"
	btc.Amount = 2
	btc.FX = &ExchangeRate{"btc", "50000000", at, "manual"}
	usd := base
	usd.ID = "usdt"
	usd.Currency = "USDT"
	usd.Amount = 10000
	usd.FX = &ExchangeRate{"usd", "100", at, "manual"}
	refund := btc
	refund.ID = "refund"
	refund.Status = "refunded"
	refund.RefundOf = btc.ID
	refund.CreatedAt = Boundary(at, 1)
	refundRate := *btc.FX
	refundRate.AsOf = at.In(time.FixedZone("same-instant", 3*60*60))
	refund.FX = &refundRate
	d.Payments = []Payment{rub, btc, usd, refund}
	s, err := Prepare(d, Boundary(at, 2))
	if err != nil {
		t.Fatal(err)
	}
	if s.Facts[0].LTV != 200 || s.Facts[0].PaymentsCount != 3 || s.Facts[0].CurrentStatus != "active" {
		t.Fatal(s.Facts[0])
	}
	if len(s.ExchangeRates) != 2 || s.Conversion.Currency != "RUB" {
		t.Fatal("snapshot did not freeze FX metadata")
	}
	body, err := s.TelegramLTV(math.MaxInt64)
	if err != nil || !strings.Contains(string(body), `"telegram_id":9223372036854775807`) {
		t.Fatal(string(body), err)
	}
	var profile UserLTV
	if err = json.Unmarshal(body, &profile); err != nil || profile.LTV != 200 || profile.PaymentsCount != 3 || profile.FirstPlan == nil || *profile.FirstPlan != "m" {
		t.Fatal(profile, err)
	}
	zero := s.UserLTV(s.Facts[1])
	if zero.FirstPlan != nil || zero.LTV != 0 || zero.CurrentStatus != "none" {
		t.Fatal(zero)
	}
	if _, err = s.TelegramLTV(99); err == nil {
		t.Fatal("unknown ID accepted")
	}
	badRate := *btc.FX
	badRate.ID = "new-quote"
	d.Payments[3].FX = &badRate
	if _, err = Prepare(d, Boundary(at, 2)); err == nil {
		t.Fatal("refund at different quote accepted")
	}
	d.Payments = d.Payments[:3]
	d.Payments[1].FX = nil
	if _, err = Prepare(d, Boundary(at, 2)); err == nil {
		t.Fatal("missing FX accepted")
	}
	d.Payments = []Payment{btc}
	conflict := btc
	conflict.ID = "conflict"
	conflict.Amount = 4
	conflictRate := *btc.FX
	conflictRate.RUBPerUnit = "25000000"
	conflict.FX = &conflictRate
	d.Payments = append(d.Payments, conflict)
	if _, err = Prepare(d, Boundary(at, 2)); err == nil {
		t.Fatal("one FX ID accepted with conflicting values")
	}
}

func TestUTMGroupingFilteringAndPagination(t *testing.T) {
	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s := Snapshot{DataAsOf: Boundary(at, 2), Plans: []Plan{{ID: "m"}}, Facts: []Fact{{ID: "a", UTMLink: " ads ", First: &at, FirstPlan: "m", LTV: 100, Chains: []Chain{{"m", at, 1, 2}}}, {ID: "b", UTMLink: "ads", First: &at, FirstPlan: "m", LTV: 200}, {ID: "c", First: &at, LTV: 0}, {ID: "d", UTMLink: "social", LTV: 50}}}
	rows, err := UTMSummaries(s.Facts)
	if err != nil || len(rows) != 3 || rows[0]["total_revenue_minor"] != int64(300) || rows[0]["users_count"] != 2 {
		t.Fatal(rows, err)
	}
	utm := " ads "
	body, err := s.Response("summary", Filter{UTMLink: &utm})
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Count int   `json:"users_count"`
		Total int64 `json:"total_revenue_minor"`
	}
	_ = json.Unmarshal(body, &summary)
	if summary.Count != 2 || summary.Total != 300 {
		t.Fatal(string(body))
	}
	body, err = s.Response("retention", Filter{UTMLink: &utm, Plan: "m", Periods: 1})
	if err != nil || !strings.Contains(string(body), `"cohort_size":1`) {
		t.Fatal(string(body), err)
	}
	body, err = s.Response("by-utm", Filter{Page: 2, PageSize: 1})
	if err != nil || !strings.Contains(string(body), `"total":3`) || !strings.Contains(string(body), `"utm_link":"social"`) {
		t.Fatal(string(body), err)
	}
	report, err := s.Dashboard(DashboardQuery{UTMLink: &utm})
	if err != nil || len(report.UTMOptions) != 3 || len(report.UTMs) != 1 || report.Users.Total != 2 {
		t.Fatal(report, err)
	}
}

func TestGeneratorIncludesAllCurrenciesAndZeroUTM(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	d := Generate(42, 10000, 24, at)
	s, err := Prepare(d, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, currency := range []string{"RUB", "BTC", "USDT"} {
		if s.Dataset.Currencies[currency] == 0 {
			t.Fatal("missing", currency)
		}
	}
	unknown := false
	for _, u := range d.Users {
		if u.UTMLink == "" {
			unknown = true
		}
	}
	if !unknown {
		t.Fatal("missing unknown UTM")
	}
}

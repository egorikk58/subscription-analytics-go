package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"time"
)

type User struct {
	ID         string    `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	CreatedAt  time.Time `json:"created_at"`
	UTMLink    string    `json:"utm_link"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type Plan struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PeriodMonths int    `json:"period_months"`
	Price        int64  `json:"price"`
	Currency     string `json:"currency"`
}
type Subscription struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	PlanID           string     `json:"plan_id"`
	Status           string     `json:"status"`
	StartedAt        time.Time  `json:"started_at"`
	CurrentPeriodEnd time.Time  `json:"current_period_end"`
	CanceledAt       *time.Time `json:"canceled_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
type Payment struct {
	ID             string        `json:"id"`
	UserID         string        `json:"user_id"`
	SubscriptionID string        `json:"subscription_id"`
	Amount         int64         `json:"amount"`
	Currency       string        `json:"currency"`
	Status         string        `json:"status"`
	PeriodStart    time.Time     `json:"period_start"`
	PeriodEnd      time.Time     `json:"period_end"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	RefundOf       string        `json:"refund_of_payment_id,omitempty"`
	FX             *ExchangeRate `json:"fx,omitempty"`
}
type Dataset struct {
	Users         []User         `json:"users"`
	Plans         []Plan         `json:"plans"`
	Subscriptions []Subscription `json:"subscriptions"`
	Payments      []Payment      `json:"payments"`
}
type Chain struct {
	PlanID       string
	Start        time.Time
	Months, Paid int
}
type Fact struct {
	ID            string
	TelegramID    int64
	UTMLink       string
	First         *time.Time
	FirstPlan     string
	LTV           int64
	Chains        []Chain
	PaymentsCount int
	CurrentStatus string
}
type Snapshot struct {
	ExchangeRates []FrozenQuote    `json:"exchange_rates"`
	Conversion    ConversionPolicy `json:"conversion"`
	FormatVersion int              `json:"format_version"`
	DataAsOf      time.Time        `json:"data_as_of"`
	Plans         []Plan           `json:"plans"`
	Facts         []Fact           `json:"facts"`
	Dataset       DatasetInfo      `json:"dataset"`
}
type Filter struct {
	UTMLink        *string
	Page, PageSize int
	From, To       *time.Time
	Plan           string
	Periods        int
}

func Boundary(t time.Time, months int) time.Time {
	t = t.UTC()
	first := time.Date(t.Year(), t.Month()+time.Month(months), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	day := t.Day()
	if day > last {
		day = last
	}
	return first.AddDate(0, 0, day-1)
}
func ID(seed int64, label string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", seed, label)))
	h[6] = (h[6] & 15) | 64
	h[8] = (h[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[:4], h[4:6], h[6:8], h[8:10], h[10:16])
}
func Prepare(d Dataset, now time.Time) (Snapshot, error) {
	s := Snapshot{ExchangeRates: []FrozenQuote{}, Conversion: Policy(), FormatVersion: 2, DataAsOf: now, Plans: d.Plans}
	users := map[string]int{}
	plans := map[string]Plan{}
	subs := map[string]Subscription{}
	paid := map[string]bool{}
	refunds := map[string]bool{}
	payments := map[string]Payment{}
	quotes := map[string]FrozenQuote{}
	for _, p := range d.Plans {
		if p.Currency != "RUB" || p.Price <= 0 || (p.PeriodMonths != 1 && p.PeriodMonths != 3 && p.PeriodMonths != 6 && p.PeriodMonths != 12) {
			return s, fmt.Errorf("invalid plan")
		}
		plans[p.ID] = p
	}
	for _, u := range d.Users {
		if _, exists := users[u.ID]; exists {
			return s, fmt.Errorf("duplicate user")
		}
		users[u.ID] = len(s.Facts)
		s.Facts = append(s.Facts, Fact{ID: u.ID, TelegramID: u.TelegramID, UTMLink: u.UTMLink})
	}
	// Do not reorder a caller's source data while preparing immutable facts.
	d.Subscriptions = append([]Subscription(nil), d.Subscriptions...)
	sort.Slice(d.Subscriptions, func(i, j int) bool {
		a, b := d.Subscriptions[i], d.Subscriptions[j]
		if a.StartedAt.Equal(b.StartedAt) {
			return a.ID < b.ID
		}
		return a.StartedAt.Before(b.StartedAt)
	})
	for _, sub := range d.Subscriptions {
		idx, ok := users[sub.UserID]
		if !ok {
			return s, fmt.Errorf("unknown user")
		}
		if _, ok = plans[sub.PlanID]; !ok {
			return s, fmt.Errorf("unknown plan")
		}
		subs[sub.ID] = sub
		f := &s.Facts[idx]
		if f.First == nil {
			t := sub.StartedAt.UTC()
			f.First = &t
			f.FirstPlan = sub.PlanID
		}
		if !sub.StartedAt.After(now) {
			f.CurrentStatus = sub.Status
			if f.CurrentStatus == "active" && !sub.CurrentPeriodEnd.After(now) {
				f.CurrentStatus = "expired"
			}
			if f.CurrentStatus == "" {
				f.CurrentStatus = "unknown"
			}
		}
	}
	for _, p := range d.Payments {
		sub, ok := subs[p.SubscriptionID]
		if !ok || sub.UserID != p.UserID {
			return s, fmt.Errorf("invalid payment relation")
		}
		plan := plans[sub.PlanID]
		converted, err := ConvertedMinor(p)
		if err != nil {
			return s, err
		}
		if converted != plan.Price || !p.PeriodEnd.After(p.PeriodStart) {
			return s, fmt.Errorf("invalid payment amount/currency/period")
		}
		if p.CreatedAt.After(now) {
			continue
		}
		if p.FX != nil {
			key := p.Currency + "/" + p.FX.ID
			if prior, exists := quotes[key]; exists && !sameExchangeRate(&prior.Rate, p.FX) {
				return s, fmt.Errorf("conflicting historical exchange rate")
			}
			quotes[key] = FrozenQuote{Currency: p.Currency, Rate: *p.FX}
		}
		payments[p.ID] = p
	}
	quoteKeys := make([]string, 0, len(quotes))
	for key := range quotes {
		quoteKeys = append(quoteKeys, key)
	}
	sort.Strings(quoteKeys)
	for _, key := range quoteKeys {
		s.ExchangeRates = append(s.ExchangeRates, quotes[key])
	}
	for _, p := range payments {
		if p.Status == "refunded" {
			original, ok := payments[p.RefundOf]
			if !ok || original.Status != "succeeded" || original.Amount != p.Amount || original.Currency != p.Currency || original.UserID != p.UserID || original.SubscriptionID != p.SubscriptionID || !original.PeriodStart.Equal(p.PeriodStart) || !original.PeriodEnd.Equal(p.PeriodEnd) || p.CreatedAt.Before(original.CreatedAt) || refunds[p.RefundOf] {
				return s, fmt.Errorf("invalid refund")
			}
			if !sameExchangeRate(original.FX, p.FX) {
				return s, fmt.Errorf("refund must use original exchange rate")
			}
			refunds[p.RefundOf] = true
		}
	}
	totals := make([]big.Int, len(s.Facts))
	for _, p := range payments {
		idx := users[p.UserID]
		delta := int64(0)
		switch p.Status {
		case "succeeded":
			delta, _ = ConvertedMinor(p)
			s.Facts[idx].PaymentsCount++
		case "refunded":
			converted, _ := ConvertedMinor(p)
			delta = -converted
		case "failed":
		default:
			return s, fmt.Errorf("invalid payment status")
		}
		totals[idx].Add(&totals[idx], big.NewInt(delta))
		if p.Status == "succeeded" && !refunds[p.ID] {
			paid[p.SubscriptionID+"/"+p.PeriodStart.UTC().Format(time.RFC3339Nano)+"/"+p.PeriodEnd.UTC().Format(time.RFC3339Nano)] = true
		}
	}
	for i := range s.Facts {
		if !totals[i].IsInt64() {
			return s, fmt.Errorf("money overflow")
		}
		s.Facts[i].LTV = totals[i].Int64()
	}
	// A new subscription closes the preceding chain, including a tariff switch.
	// Later charges on an old subscription still contribute to lifetime revenue,
	// but cannot make that old chain appear to renew after the switch.
	chainEnds := map[string]time.Time{}
	nextSubscription := map[string]time.Time{}
	for i := len(d.Subscriptions) - 1; i >= 0; i-- {
		sub := d.Subscriptions[i]
		if next, exists := nextSubscription[sub.UserID]; exists {
			chainEnds[sub.ID] = next
		}
		nextSubscription[sub.UserID] = sub.StartedAt
	}
	seen := map[string]bool{}
	for _, sub := range d.Subscriptions {
		key := sub.UserID + "/" + sub.PlanID
		if seen[key] {
			continue
		}
		seen[key] = true
		plan := plans[sub.PlanID]
		c := Chain{PlanID: sub.PlanID, Start: sub.StartedAt.UTC(), Months: plan.PeriodMonths}
		for n := 0; n < 1000; n++ {
			a, b := Boundary(c.Start, n*c.Months), Boundary(c.Start, (n+1)*c.Months)
			if end, closed := chainEnds[sub.ID]; closed && !a.Before(end) {
				break
			}
			if !paid[sub.ID+"/"+a.Format(time.RFC3339Nano)+"/"+b.Format(time.RFC3339Nano)] {
				break
			}
			c.Paid++
		}
		idx := users[sub.UserID]
		s.Facts[idx].Chains = append(s.Facts[idx].Chains, c)
	}
	s.Dataset = DescribeDataset(d, now)
	return s, nil
}
func (s Snapshot) Response(route string, q Filter) ([]byte, error) {
	if q.Plan != "" {
		known := false
		for _, p := range s.Plans {
			if p.ID == q.Plan {
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown plan_id")
		}
	}
	facts := []Fact{}
	for _, f := range s.Facts {
		if q.UTMLink != nil && NormalizeUTM(f.UTMLink) != NormalizeUTM(*q.UTMLink) {
			continue
		}
		if route != "retention" && q.Plan != "" && f.FirstPlan != q.Plan {
			continue
		}
		if (q.From != nil && (f.First == nil || f.First.Before(*q.From))) || (q.To != nil && (f.First == nil || !f.First.Before(*q.To))) {
			continue
		}
		facts = append(facts, f)
	}
	if route == "plans" {
		return json.Marshal(map[string]any{"data_as_of": s.DataAsOf, "plans": s.Plans})
	}
	if route == "summary" {
		m, e := summary(facts)
		if e != nil {
			return nil, e
		}
		m["data_as_of"] = s.DataAsOf
		m["conversion"] = Policy()
		return json.Marshal(m)
	}
	if route == "by-utm" {
		rows, err := UTMSummaries(facts)
		if err != nil {
			return nil, err
		}
		page, size := max(1, q.Page), q.PageSize
		if size <= 0 {
			size = 100
		}
		start := min((page-1)*size, len(rows))
		end := min(start+size, len(rows))
		return json.Marshal(map[string]any{"data_as_of": s.DataAsOf, "currency": "RUB", "conversion": Policy(), "groups": rows[start:end], "total": len(rows), "page": page, "page_size": size})
	}
	if route == "by-plan" {
		rows := []any{}
		for _, p := range s.Plans {
			group := []Fact{}
			for _, f := range facts {
				if f.FirstPlan == p.ID {
					group = append(group, f)
				}
			}
			m, e := summary(group)
			if e != nil {
				return nil, e
			}
			m["plan_id"] = p.ID
			rows = append(rows, m)
		}
		return json.Marshal(map[string]any{"data_as_of": s.DataAsOf, "conversion": Policy(), "plans": rows})
	}
	if route != "retention" {
		return nil, fmt.Errorf("unknown route")
	}
	valid := false
	for _, p := range s.Plans {
		if p.ID == q.Plan {
			valid = true
		}
	}
	if !valid {
		return nil, fmt.Errorf("unknown plan_id")
	}
	chains := []Chain{}
	for _, f := range facts {
		for _, c := range f.Chains {
			if c.PlanID == q.Plan {
				chains = append(chains, c)
			}
		}
	}
	rows := []any{}
	for n := 1; n <= q.Periods; n++ {
		eligible, renewed, mature := 0, 0, 0
		for _, c := range chains {
			if Boundary(c.Start, n*c.Months).After(s.DataAsOf) {
				continue
			}
			mature++
			if c.Paid >= n {
				eligible++
				if c.Paid >= n+1 {
					renewed++
				}
			}
		}
		var rate, cumulative *float64
		if eligible > 0 {
			v := float64(renewed) / float64(eligible)
			rate = &v
		}
		if mature > 0 {
			v := float64(renewed) / float64(mature)
			cumulative = &v
		}
		rows = append(rows, map[string]any{"period": n, "eligible": eligible, "renewed": renewed, "cumulative_eligible": mature, "rate": rate, "cumulative_rate": cumulative})
	}
	return json.Marshal(map[string]any{"plan_id": q.Plan, "data_as_of": s.DataAsOf, "cohort_size": len(chains), "periods": rows})
}
func summary(facts []Fact) (map[string]any, error) {
	total := new(big.Int)
	values := []int64{}
	for _, f := range facts {
		total.Add(total, big.NewInt(f.LTV))
		values = append(values, f.LTV)
	}
	if !total.IsInt64() {
		return nil, fmt.Errorf("money overflow")
	}
	mean, median := "0.00", "0.00"
	n := len(values)
	if n > 0 {
		mean = new(big.Rat).SetFrac(total, big.NewInt(int64(n))).FloatString(2)
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		if n%2 == 1 {
			median = new(big.Rat).SetInt64(values[n/2]).FloatString(2)
		} else {
			sum := new(big.Int).Add(big.NewInt(values[n/2-1]), big.NewInt(values[n/2]))
			median = new(big.Rat).SetFrac(sum, big.NewInt(2)).FloatString(2)
		}
	}
	return map[string]any{"currency": "RUB", "users_count": n, "total_revenue_minor": total.Int64(), "mean_ltv_minor": mean, "median_ltv_minor": median}, nil
}

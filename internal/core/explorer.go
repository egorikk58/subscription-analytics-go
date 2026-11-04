package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DatasetInfo struct {
	Currencies         map[string]int   `json:"currencies"`
	Conversion         ConversionPolicy `json:"conversion"`
	Seed               *int64           `json:"seed"`
	GeneratorVersion   string           `json:"generator_version"`
	HistoryMonths      int              `json:"history_months"`
	DatasetAsOf        *time.Time       `json:"dataset_as_of"`
	UsersCount         int              `json:"users_count"`
	SubscriptionsCount int              `json:"subscriptions_count"`
	PaymentsCount      int              `json:"payments_count"`
	SucceededCount     int              `json:"succeeded_count"`
	FailedCount        int              `json:"failed_count"`
	RefundedCount      int              `json:"refunded_count"`
	SwitchesCount      int              `json:"switches_count"`
	UnpaidUsers        int              `json:"unpaid_users"`
	ActivePeriods      int              `json:"active_periods"`
	Fingerprint        string           `json:"fingerprint"`
	Samples            DatasetSamples   `json:"samples"`
}
type DatasetSamples struct {
	Users         []User         `json:"users"`
	Subscriptions []Subscription `json:"subscriptions"`
	Payments      []Payment      `json:"payments"`
}

// DescribeDataset records source evidence in the immutable snapshot, rather
// than querying mutable source tables while a dashboard request is running.
func DescribeDataset(d Dataset, now time.Time) DatasetInfo {
	info := DatasetInfo{Currencies: map[string]int{}, Conversion: Policy(), UsersCount: len(d.Users), SubscriptionsCount: len(d.Subscriptions), PaymentsCount: len(d.Payments)}
	paid := map[string]bool{}
	plans := map[string]map[string]bool{}
	for _, p := range d.Payments {
		info.Currencies[p.Currency]++
		switch p.Status {
		case "succeeded":
			info.SucceededCount++
			paid[p.UserID] = true
		case "failed":
			info.FailedCount++
		case "refunded":
			info.RefundedCount++
		}
	}
	for _, u := range d.Users {
		if !paid[u.ID] {
			info.UnpaidUsers++
		}
	}
	for _, s := range d.Subscriptions {
		if plans[s.UserID] == nil {
			plans[s.UserID] = map[string]bool{}
		}
		plans[s.UserID][s.PlanID] = true
		if s.StartedAt.Before(now) && s.CurrentPeriodEnd.After(now) {
			info.ActivePeriods++
		}
	}
	for _, p := range plans {
		if len(p) > 1 {
			info.SwitchesCount++
		}
	}
	users := append([]User(nil), d.Users...)
	subs := append([]Subscription(nil), d.Subscriptions...)
	payments := append([]Payment(nil), d.Payments...)
	p := append([]Plan(nil), d.Plans...)
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	sort.Slice(subs, func(i, j int) bool { return subs[i].ID < subs[j].ID })
	sort.Slice(payments, func(i, j int) bool { return payments[i].ID < payments[j].ID })
	sort.Slice(p, func(i, j int) bool { return p[i].ID < p[j].ID })
	canonical, _ := json.Marshal(Dataset{Users: users, Plans: p, Subscriptions: subs, Payments: payments})
	hash := sha256.Sum256(canonical)
	info.Fingerprint = fmt.Sprintf("%x", hash[:])
	paymentSamples := make([]Payment, 0, 12)
	for _, status := range []string{"succeeded", "failed", "refunded"} {
		added := 0
		for _, payment := range payments {
			if payment.Status == status {
				paymentSamples = append(paymentSamples, payment)
				added++
				if added == 4 {
					break
				}
			}
		}
	}
	info.Samples = DatasetSamples{Users: users[:min(12, len(users))], Subscriptions: subs[:min(12, len(subs))], Payments: paymentSamples}
	return info
}

func (i *DatasetInfo) SetConfig(config string) error {
	fields := strings.SplitN(config, "/", 5)
	if len(fields) != 5 {
		return fmt.Errorf("invalid seed configuration")
	}
	seed, e := strconv.ParseInt(fields[1], 10, 64)
	if e != nil {
		return e
	}
	months, e := strconv.Atoi(fields[3])
	if e != nil {
		return e
	}
	asof, e := time.Parse(time.RFC3339, fields[4])
	if e != nil {
		return e
	}
	i.Seed = &seed
	i.GeneratorVersion = fields[0]
	i.HistoryMonths = months
	i.DatasetAsOf = &asof
	return nil
}

type DashboardQuery struct {
	UTMLink      *string
	From, To     *time.Time
	Plan         string
	Selection    string
	Page         int
	Search, Sort string
}
type PeriodRow struct {
	Period             int      `json:"period"`
	Eligible           int      `json:"eligible"`
	Renewed            int      `json:"renewed"`
	CumulativeEligible int      `json:"cumulative_eligible"`
	Rate               *float64 `json:"rate"`
	CumulativeRate     *float64 `json:"cumulative_rate"`
}
type RetentionReport struct {
	PlanID     string      `json:"plan_id"`
	CohortSize int         `json:"cohort_size"`
	Periods    []PeriodRow `json:"periods"`
}
type CohortRow struct {
	Month      string      `json:"month"`
	UsersCount int         `json:"users_count"`
	Periods    []PeriodRow `json:"periods"`
}
type UserRow struct {
	ID         string     `json:"id"`
	TelegramID int64      `json:"telegram_id"`
	First      *time.Time `json:"first_subscription_at"`
	FirstPlan  string     `json:"first_plan_id"`
	LTV        int64      `json:"ltv_minor"`
	UTMLink    string     `json:"utm_link"`
}
type UserPage struct {
	Items    []UserRow `json:"items"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int       `json:"total"`
	Pages    int       `json:"pages"`
}
type DistributionRow struct {
	Label      string `json:"label"`
	MinMinor   int64  `json:"min_minor"`
	MaxMinor   *int64 `json:"max_minor"`
	UsersCount int    `json:"users_count"`
}
type Dashboard struct {
	UTMs         []map[string]any  `json:"utm_groups"`
	UTMOptions   []string          `json:"utm_options"`
	Conversion   ConversionPolicy  `json:"conversion"`
	DataAsOf     time.Time         `json:"data_as_of"`
	SnapshotID   *int64            `json:"snapshot_id"`
	Mode         string            `json:"mode"`
	Selection    string            `json:"selection"`
	Summary      map[string]any    `json:"summary"`
	Plans        []map[string]any  `json:"plans"`
	Timeline     []map[string]any  `json:"timeline"`
	Retention    []RetentionReport `json:"retention"`
	Cohorts      []CohortRow       `json:"cohorts"`
	Distribution []DistributionRow `json:"distribution"`
	Users        UserPage          `json:"users"`
	Dataset      DatasetInfo       `json:"dataset"`
}

func retain(chains []Chain, asof time.Time, count int) []PeriodRow {
	result := make([]PeriodRow, 0, count)
	for n := 1; n <= count; n++ {
		row := PeriodRow{Period: n}
		for _, c := range chains {
			if Boundary(c.Start, n*c.Months).After(asof) {
				continue
			}
			row.CumulativeEligible++
			if c.Paid >= n {
				row.Eligible++
				if c.Paid >= n+1 {
					row.Renewed++
				}
			}
		}
		if row.Eligible > 0 {
			rate := float64(row.Renewed) / float64(row.Eligible)
			row.Rate = &rate
		}
		if row.CumulativeEligible > 0 {
			rate := float64(row.Renewed) / float64(row.CumulativeEligible)
			row.CumulativeRate = &rate
		}
		result = append(result, row)
	}
	return result
}

func (s Snapshot) Dashboard(q DashboardQuery) (Dashboard, error) {
	if q.Selection == "" {
		q.Selection = "first_plan"
	}
	out := Dashboard{Conversion: Policy(), UTMOptions: []string{}, DataAsOf: s.DataAsOf, Mode: "live", Dataset: s.Dataset, Plans: []map[string]any{}, Timeline: []map[string]any{}, Retention: []RetentionReport{}, Cohorts: []CohortRow{}, Distribution: []DistributionRow{}}
	options := map[string]bool{}
	for _, f := range s.Facts {
		options[NormalizeUTM(f.UTMLink)] = true
	}
	for key := range options {
		out.UTMOptions = append(out.UTMOptions, key)
	}
	sort.Strings(out.UTMOptions)
	out.Selection = q.Selection
	dateFacts, selected, err := s.selectDashboardFacts(q)
	if err != nil {
		return out, err
	}
	out.Summary, err = summary(selected)
	if err != nil {
		return out, err
	}
	out.UTMs, err = UTMSummaries(selected)
	if err != nil {
		return out, err
	}
	for _, p := range s.Plans {
		group := []Fact{}
		for _, f := range selected {
			if f.FirstPlan == p.ID {
				group = append(group, f)
			}
		}
		m, e := summary(group)
		if e != nil {
			return out, e
		}
		m["plan_id"] = p.ID
		m["name"] = p.Name
		m["period_months"] = p.PeriodMonths
		out.Plans = append(out.Plans, m)
	}
	months := map[string][]Fact{}
	for _, f := range selected {
		if f.First != nil {
			month := f.First.UTC().Format("2006-01")
			months[month] = append(months[month], f)
		}
	}
	keys := make([]string, 0, len(months))
	for month := range months {
		keys = append(keys, month)
	}
	sort.Strings(keys)
	for _, month := range keys {
		m, e := summary(months[month])
		if e != nil {
			return out, e
		}
		m["month"] = month
		out.Timeline = append(out.Timeline, m)
	}
	heatmapPlan := q.Plan
	if heatmapPlan == "" || heatmapPlan == "all" {
		heatmapPlan = "monthly"
	}
	for _, p := range s.Plans {
		chains := []Chain{}
		for _, f := range dateFacts {
			for _, c := range f.Chains {
				if c.PlanID == p.ID {
					chains = append(chains, c)
				}
			}
		}
		out.Retention = append(out.Retention, RetentionReport{p.ID, len(chains), retain(chains, s.DataAsOf, 24)})
	}
	cohortChains := map[string][]Chain{}
	for _, f := range dateFacts {
		if f.First == nil {
			continue
		}
		for _, c := range f.Chains {
			if c.PlanID == heatmapPlan {
				month := f.First.UTC().Format("2006-01")
				cohortChains[month] = append(cohortChains[month], c)
			}
		}
	}
	keys = keys[:0]
	for month := range cohortChains {
		keys = append(keys, month)
	}
	sort.Strings(keys)
	for _, month := range keys {
		chains := cohortChains[month]
		out.Cohorts = append(out.Cohorts, CohortRow{month, len(chains), retain(chains, s.DataAsOf, 12)})
	}
	bounds := []int64{0, 1, 100000, 500000, 1000000}
	labels := []string{"Без выручки", "До 1 000 ₽", "1 000–5 000 ₽", "5 000–10 000 ₽", "От 10 000 ₽"}
	for i, minValue := range bounds {
		var maxValue *int64
		if i+1 < len(bounds) {
			v := bounds[i+1]
			maxValue = &v
		}
		row := DistributionRow{Label: labels[i], MinMinor: minValue, MaxMinor: maxValue}
		for _, f := range selected {
			if f.LTV >= minValue && (maxValue == nil || f.LTV < *maxValue) {
				row.UsersCount++
			}
		}
		out.Distribution = append(out.Distribution, row)
	}
	candidates := orderedUserFacts(selected, q.Search, q.Sort)
	page := max(1, q.Page)
	start := min((page-1)*20, len(candidates))
	end := min(start+20, len(candidates))
	out.Users = UserPage{Items: []UserRow{}, Page: page, PageSize: 20, Total: len(candidates), Pages: (len(candidates) + 19) / 20}
	for _, f := range candidates[start:end] {
		out.Users.Items = append(out.Users.Items, UserRow{f.ID, f.TelegramID, f.First, f.FirstPlan, f.LTV, f.UTMLink})
	}
	return out, nil
}

// SelectedFacts returns the entire dashboard user selection, without pagination.
// Exports share the same filtering and ordering rules as the interactive list.
func (s Snapshot) SelectedFacts(q DashboardQuery) ([]Fact, error) {
	_, selected, err := s.selectDashboardFacts(q)
	if err != nil {
		return nil, err
	}
	return orderedUserFacts(selected, q.Search, q.Sort), nil
}

func (s Snapshot) selectDashboardFacts(q DashboardQuery) ([]Fact, []Fact, error) {
	if q.Selection == "" {
		q.Selection = "first_plan"
	}
	if q.Selection != "first_plan" && q.Selection != "chain" {
		return nil, nil, fmt.Errorf("invalid selection")
	}
	if q.Plan != "" && q.Plan != "all" {
		valid := false
		for _, p := range s.Plans {
			if p.ID == q.Plan {
				valid = true
			}
		}
		if !valid {
			return nil, nil, fmt.Errorf("unknown plan_id")
		}
	}
	dateFacts := []Fact{}
	selected := []Fact{}
	for _, f := range s.Facts {
		if q.UTMLink != nil && NormalizeUTM(f.UTMLink) != NormalizeUTM(*q.UTMLink) {
			continue
		}
		if (q.From != nil && (f.First == nil || f.First.Before(*q.From))) || (q.To != nil && (f.First == nil || !f.First.Before(*q.To))) {
			continue
		}
		dateFacts = append(dateFacts, f)
		matches := q.Plan == "" || q.Plan == "all" || f.FirstPlan == q.Plan
		if q.Selection == "chain" && q.Plan != "" && q.Plan != "all" {
			matches = false
			for _, chain := range f.Chains {
				if chain.PlanID == q.Plan {
					matches = true
					break
				}
			}
		}
		if matches {
			selected = append(selected, f)
		}
	}

	return dateFacts, selected, nil
}

func orderedUserFacts(selected []Fact, searchText, sortBy string) []Fact {
	candidates := []Fact{}
	search := strings.ToLower(searchText)
	for _, f := range selected {
		if search == "" || strings.Contains(strings.ToLower(f.ID), search) || strings.Contains(strconv.FormatInt(f.TelegramID, 10), search) {
			candidates = append(candidates, f)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		switch sortBy {
		case "joined_desc":
			if a.First == nil && b.First == nil {
				return a.ID < b.ID
			}
			if a.First == nil {
				return false
			}
			if b.First == nil {
				return true
			}
			if !a.First.Equal(*b.First) {
				return a.First.After(*b.First)
			}
		case "ltv_asc":
			if a.LTV != b.LTV {
				return a.LTV < b.LTV
			}
		default:
			if a.LTV != b.LTV {
				return a.LTV > b.LTV
			}
		}
		return a.ID < b.ID
	})

	return candidates
}

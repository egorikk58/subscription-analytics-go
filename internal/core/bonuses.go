package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func NormalizeUTM(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func UTMSummaries(facts []Fact) ([]map[string]any, error) {
	groups := map[string][]Fact{}
	for _, f := range facts {
		key := NormalizeUTM(f.UTMLink)
		groups[key] = append(groups[key], f)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		m, err := summary(groups[key])
		if err != nil {
			return nil, err
		}
		m["utm_link"] = key
		rows = append(rows, m)
	}
	return rows, nil
}

type UserLTV struct {
	UserID        string    `json:"user_id"`
	TelegramID    int64     `json:"telegram_id"`
	FirstPlan     *string   `json:"first_plan_id"`
	LTV           int64     `json:"ltv"`
	PaymentsCount int       `json:"payments_count"`
	CurrentStatus string    `json:"current_status"`
	Currency      string    `json:"currency"`
	DataAsOf      time.Time `json:"data_as_of"`
}

func (s Snapshot) UserLTV(f Fact) UserLTV {
	var plan *string
	if f.FirstPlan != "" {
		p := f.FirstPlan
		plan = &p
	}
	status := f.CurrentStatus
	if status == "" {
		status = "none"
	}
	return UserLTV{f.ID, f.TelegramID, plan, f.LTV, f.PaymentsCount, status, "RUB", s.DataAsOf}
}

func (s Snapshot) TelegramLTV(id int64) ([]byte, error) {
	for _, f := range s.Facts {
		if f.TelegramID == id {
			return json.Marshal(s.UserLTV(f))
		}
	}
	return nil, fmt.Errorf("user not found")
}

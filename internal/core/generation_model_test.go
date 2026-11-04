package core

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestGeneratedMixAndHistoriesAreSeededIndependentlyOfVolume(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a, b := Generate(42, 10000, 24, at), Generate(42, 10001, 24, at)
	if !reflect.DeepEqual(a.Users, b.Users[:len(a.Users)]) || !reflect.DeepEqual(a.Subscriptions, b.Subscriptions[:len(a.Subscriptions)]) || !reflect.DeepEqual(a.Payments, b.Payments[:len(a.Payments)]) {
		t.Fatal("changing volume altered an existing user's history")
	}
	plans, utm := map[string]int{}, map[string]int{}
	for _, s := range a.Subscriptions {
		plans[s.PlanID]++
	}
	for _, u := range a.Users {
		utm[u.UTMLink]++
	}
	if plans["monthly"] <= plans["quarterly"] || plans["quarterly"] <= plans["half-year"] || plans["half-year"] <= plans["yearly"] || utm["campaign-0"] <= utm["campaign-4"] {
		t.Fatalf("weighted mix lost its asymmetry: plans=%v, utm=%v", plans, utm)
	}
	ordinaryLongTail := false
	counts := map[string]int{}
	for _, p := range a.Payments {
		if p.Status == "succeeded" {
			counts[p.SubscriptionID]++
		}
	}
	reserved := map[string]bool{}
	for i := 0; i < 10; i++ {
		reserved[ID(42, fmt.Sprintf("u%d", i))] = true
	}
	for _, s := range a.Subscriptions {
		if !reserved[s.UserID] && s.PlanID == "monthly" && counts[s.ID] > 8 {
			ordinaryLongTail = true
		}
	}
	if !ordinaryLongTail {
		t.Fatal("ordinary users must be able to renew beyond period 8")
	}
	if _, err := Prepare(a, at); err != nil {
		t.Fatal(err)
	}
}

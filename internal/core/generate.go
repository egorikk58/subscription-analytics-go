package core

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// Independent streams keep the size draw and each user's history reproducible
// without coupling them to how many random draws another user consumed.
func generationRandom(seed int64, label string) *rand.Rand {
	digest := sha256.Sum256([]byte(fmt.Sprintf("generation/%s/%d/%s", GeneratorVersion, seed, label)))
	return rand.New(rand.NewSource(int64(binary.BigEndian.Uint64(digest[:8]))))
}

func weightedChoice(r *rand.Rand, weights []float64) int {
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	draw := r.Float64() * total
	for i, weight := range weights {
		draw -= weight
		if draw < 0 {
			return i
		}
	}
	return len(weights) - 1
}

// A synthetic hazard model, not a fit to real billing observations. The tariff
// baseline, individual propensity and tenure all affect renewal. There is no
// arbitrary lifetime cap: observations stop at churn or the history boundary.
func renewalProbability(months, renewal int, propensity float64) float64 {
	baseline := map[int]float64{1: .60, 3: .70, 6: .76, 12: .82}[months]
	tenure := .10 * (1 - math.Exp(-float64(renewal-1)/3))
	return math.Max(.10, math.Min(.97, baseline+propensity+tenure))
}

func Generate(seed int64, count, months int, asof time.Time) Dataset {
	d := Dataset{Plans: []Plan{{"monthly", "Monthly", 1, 99000, "RUB"}, {"quarterly", "Quarterly", 3, 269000, "RUB"}, {"half-year", "Half year", 6, 499000, "RUB"}, {"yearly", "Yearly", 12, 899000, "RUB"}}}
	profile := generationRandom(seed, "profile")
	planWeights := []float64{.47, .28, .17, .08}
	utmWeights := []float64{.32, .24, .18, .12, .08, .06}
	for _, weights := range [][]float64{planWeights, utmWeights} {
		for i := range weights {
			weights[i] *= .8 + .4*profile.Float64()
		}
	}
	start := Boundary(asof, -months)
	for i := 0; i < count; i++ {
		r := generationRandom(seed, fmt.Sprintf("user/%d", i))
		uid := ID(seed, fmt.Sprintf("u%d", i))
		joined := start.Add(time.Duration(r.Int63n(int64(asof.Sub(start)))))
		if i < 10 {
			joined = start
		}
		if i == 8 {
			joined = asof.Add(-24 * time.Hour)
		}
		utm := weightedChoice(r, utmWeights)
		utmLink := ""
		if utm < 5 {
			utmLink = fmt.Sprintf("campaign-%d", utm)
		}
		d.Users = append(d.Users, User{uid, 1000000000 + int64(i)*7919, joined, utmLink, joined})
		// Fixed boundary cases guarantee coverage independently of the random seed.
		if i == 0 || (i >= 10 && r.Float64() < .08) {
			continue
		}
		planIndex := weightedChoice(r, planWeights)
		if i < 10 {
			planIndex = i % len(d.Plans)
		}
		p := d.Plans[planIndex]
		propensity := r.NormFloat64() * .09
		sid := ID(seed, fmt.Sprintf("s%d", i))
		sub := generateChain(&d, r, seed, uid, sid, p, joined, asof, propensity, i > 0 && i < 8, i == 1, i == 2, i == 9)
		d.Subscriptions = append(d.Subscriptions, sub)
		if !sub.CurrentPeriodEnd.After(asof) && (i == 9 || r.Float64() < .07) {
			nextWeights := append([]float64(nil), planWeights...)
			nextWeights[planIndex] = 0
			next := d.Plans[weightedChoice(r, nextWeights)]
			nextID := ID(seed, fmt.Sprintf("switch%d", i))
			d.Subscriptions = append(d.Subscriptions, generateChain(&d, r, seed, uid, nextID, next, sub.CurrentPeriodEnd, asof, propensity, false, false, false, false))
		}
	}
	if err := AddDemoCurrencies(d.Payments); err != nil {
		panic(err)
	}
	return d
}

func generateChain(d *Dataset, r *rand.Rand, seed int64, uid, sid string, plan Plan, began, asof time.Time, propensity float64, longTail, retry, refund, stopAfterFirst bool) Subscription {
	sub := Subscription{ID: sid, UserID: uid, PlanID: plan.ID, Status: "expired", StartedAt: began, UpdatedAt: began}
	last := began
	for n := 0; ; n++ {
		a, b := Boundary(began, n*plan.PeriodMonths), Boundary(began, (n+1)*plan.PeriodMonths)
		if a.After(asof) || (n > 0 && (stopAfterFirst || (!longTail && r.Float64() >= renewalProbability(plan.PeriodMonths, n, propensity)))) {
			break
		}
		id := ID(seed, fmt.Sprintf("payment/%s/%d", sid, n))
		payment := Payment{ID: id, UserID: uid, SubscriptionID: sid, Amount: plan.Price, Currency: "RUB", Status: "succeeded", PeriodStart: a, PeriodEnd: b, CreatedAt: a, UpdatedAt: a}
		if (n == 0 && retry) || r.Float64() < .10 {
			failed := payment
			failed.ID, failed.Status = ID(seed, "fail/"+id), "failed"
			d.Payments = append(d.Payments, failed)
		}
		d.Payments = append(d.Payments, payment)
		if (n == 0 && refund) || r.Float64() < .02 {
			returned := payment
			returned.ID, returned.Status, returned.RefundOf = ID(seed, "refund/"+id), "refunded", id
			d.Payments = append(d.Payments, returned)
		}
		last = b
	}
	sub.CurrentPeriodEnd = last
	if last.After(asof) {
		sub.Status = "active"
	} else if r.Float64() < .75 {
		sub.Status = "canceled"
		sub.CanceledAt = &last
	}
	return sub
}

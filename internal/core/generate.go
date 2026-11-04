package core

import (
	"fmt"
	"math/rand"
	"time"
)

func Generate(seed int64, count, months int, asof time.Time) Dataset {
	d := Dataset{Plans: []Plan{{"monthly", "Monthly", 1, 99000, "RUB"}, {"quarterly", "Quarterly", 3, 269000, "RUB"}, {"half-year", "Half year", 6, 499000, "RUB"}, {"yearly", "Yearly", 12, 899000, "RUB"}}}
	r := rand.New(rand.NewSource(seed))
	start := Boundary(asof, -months)
	for i := 0; i < count; i++ {
		uid := ID(seed, fmt.Sprintf("u%d", i))
		joined := start.Add(time.Duration(r.Int63n(int64(asof.Sub(start)))))
		if i < 8 {
			joined = start
		}
		if i == 8 {
			joined = asof.Add(-24 * time.Hour)
		}
		d.Users = append(d.Users, User{uid, 1000000000 + int64(i)*7919, joined, fmt.Sprintf("campaign-%d", i%5), joined})
		if i%11 == 0 {
			d.Users[len(d.Users)-1].UTMLink = ""
		}
		if i%13 == 0 {
			continue
		}
		p := d.Plans[i%4]
		began := joined
		if i < 8 {
			began = start
		}
		if i == 8 {
			began = asof.Add(-24 * time.Hour)
		}
		sid := ID(seed, fmt.Sprintf("s%d", i))
		sub := Subscription{ID: sid, UserID: uid, PlanID: p.ID, Status: "expired", StartedAt: began, UpdatedAt: began}
		limit := 1 + r.Intn(8)
		if i < 8 {
			limit = 100
		}
		last := began
		for n := 0; n < limit; n++ {
			a, b := Boundary(began, n*p.PeriodMonths), Boundary(began, (n+1)*p.PeriodMonths)
			if a.After(asof) {
				break
			}
			if n > 0 && i >= 8 && r.Float64() > []float64{.60, .70, .76, .82}[i%4] {
				break
			}
			id := ID(seed, fmt.Sprintf("p%d-%d", i, n))
			payment := Payment{ID: id, UserID: uid, SubscriptionID: sid, Amount: p.Price, Currency: "RUB", Status: "succeeded", PeriodStart: a, PeriodEnd: b, CreatedAt: a, UpdatedAt: a}
			if n == 0 && i%7 == 0 {
				fail := payment
				fail.ID = ID(seed, "fail"+id)
				fail.Status = "failed"
				d.Payments = append(d.Payments, fail)
			}
			d.Payments = append(d.Payments, payment)
			if i%17 == 0 && n == 0 {
				refund := payment
				refund.ID = ID(seed, "refund"+id)
				refund.Status = "refunded"
				refund.RefundOf = id
				d.Payments = append(d.Payments, refund)
			}
			last = b
		}
		sub.CurrentPeriodEnd = last
		sub.UpdatedAt = began
		if last.After(asof) {
			sub.Status = "active"
		} else if i%2 == 0 {
			sub.Status = "canceled"
			t := last
			sub.CanceledAt = &t
		}
		d.Subscriptions = append(d.Subscriptions, sub)
		if i%19 == 0 && !last.After(asof) && last.After(began) {
			p = d.Plans[3]
			sid = ID(seed, fmt.Sprintf("switch%d", i))
			end := Boundary(last, 12)
			status := "active"
			if !end.After(asof) {
				status = "expired"
			}
			d.Subscriptions = append(d.Subscriptions, Subscription{sid, uid, p.ID, status, last, end, nil, last})
			d.Payments = append(d.Payments, Payment{ID: ID(seed, sid+"pay"), UserID: uid, SubscriptionID: sid, Amount: p.Price, Currency: "RUB", Status: "succeeded", PeriodStart: last, PeriodEnd: end, CreatedAt: last, UpdatedAt: last})
		}
	}
	if err := AddDemoCurrencies(d.Payments); err != nil {
		panic(err)
	}
	return d
}

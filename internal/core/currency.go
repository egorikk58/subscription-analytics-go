package core

import (
	"crypto/sha256"
	"fmt"
	"math/big"
	"regexp"
	"time"
)

// Amount is always in the canonical asset's atomic units: RUB 2, BTC 8,
// USDT 6 decimals. These are ledger assets, not arbitrary on-chain tokens.
type ExchangeRate struct {
	ID         string    `json:"id"`
	RUBPerUnit string    `json:"rub_per_unit"`
	AsOf       time.Time `json:"as_of"`
	Source     string    `json:"source"`
}

type ConversionPolicy struct {
	Currency string `json:"currency"`
	Method   string `json:"method"`
	Rounding string `json:"rounding"`
}

type FrozenQuote struct {
	Currency string       `json:"currency"`
	Rate     ExchangeRate `json:"rate"`
}

func Policy() ConversionPolicy {
	return ConversionPolicy{"RUB", "historical_payment_rate; full_refund_at_original_rate", "half_away_from_zero_per_payment"}
}

func sameExchangeRate(a, b *ExchangeRate) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.ID == b.ID && a.RUBPerUnit == b.RUBPerUnit && a.Source == b.Source && a.AsOf.Equal(b.AsOf)
}

var decimalRate = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,12})?$`)

func AssetScale(currency string) (int, error) {
	switch currency {
	case "RUB":
		return 2, nil
	case "BTC":
		return 8, nil
	case "USDT":
		return 6, nil
	default:
		return 0, fmt.Errorf("unsupported currency %q", currency)
	}
}

func paymentRate(p Payment) (*big.Rat, error) {
	if _, err := AssetScale(p.Currency); err != nil {
		return nil, err
	}
	if p.Currency == "RUB" {
		if p.FX != nil {
			return nil, fmt.Errorf("RUB payment must not override exchange rate")
		}
		return big.NewRat(1, 1), nil
	}
	if p.FX == nil || p.FX.ID == "" || p.FX.Source == "" || p.FX.AsOf.IsZero() || p.FX.AsOf.After(p.CreatedAt) || !decimalRate.MatchString(p.FX.RUBPerUnit) {
		return nil, fmt.Errorf("missing or invalid historical exchange rate")
	}
	rate, ok := new(big.Rat).SetString(p.FX.RUBPerUnit)
	if !ok || rate.Sign() <= 0 {
		return nil, fmt.Errorf("invalid exchange rate")
	}
	return rate, nil
}

// ConvertedMinor uses exact rational arithmetic and rounds once per payment.
// A refund caller must supply its original payment rate, never a current quote.
func ConvertedMinor(p Payment) (int64, error) {
	if p.Amount <= 0 {
		return 0, fmt.Errorf("payment amount must be positive")
	}
	rate, err := paymentRate(p)
	if err != nil {
		return 0, err
	}
	scale, _ := AssetScale(p.Currency)
	value := new(big.Rat).Mul(new(big.Rat).SetInt64(p.Amount), rate)
	value.Mul(value, big.NewRat(100, 1))
	value.Quo(value, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)))
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(value.Num(), value.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(value.Denom()) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	if !whole.IsInt64() {
		return 0, fmt.Errorf("converted money overflow")
	}
	return whole.Int64(), nil
}

// AddDemoCurrencies upgrades payment fixtures without changing their IDs or
// RUB invoice values. Quotes are synthetic and immutable, never market prices.
func AddDemoCurrencies(payments []Payment) error {
	originals := make(map[string]Payment, len(payments))
	for i := range payments {
		p := &payments[i]
		if p.Status == "refunded" {
			continue
		}
		if p.Currency == "RUB" {
			hash := sha256.Sum256([]byte(p.ID))
			currency := []string{"RUB", "BTC", "USDT"}[int(hash[0])%3]
			if currency != "RUB" {
				month := int(p.CreatedAt.UTC().Month()) % 3
				rate := []int64{5000000, 8000000, 10000000}[month]
				if currency == "USDT" {
					rate = []int64{80, 100, 125}[month]
				}
				scale, _ := AssetScale(currency)
				native := new(big.Int).Mul(big.NewInt(p.Amount), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil))
				amount, remainder := new(big.Int), new(big.Int)
				amount.QuoRem(native, big.NewInt(rate*100), remainder)
				if remainder.Sign() != 0 || !amount.IsInt64() {
					return fmt.Errorf("demo invoice cannot be represented exactly")
				}
				p.Amount, p.Currency = amount.Int64(), currency
				date := time.Date(p.CreatedAt.Year(), p.CreatedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
				p.FX = &ExchangeRate{ID: "demo-fx-v1/" + date.Format("2006-01") + "/" + currency, RUBPerUnit: fmt.Sprint(rate), AsOf: date, Source: "deterministic-demo-fixture-v1"}
			}
		}
		originals[p.ID] = *p
	}
	for i := range payments {
		p := &payments[i]
		if p.Status != "refunded" {
			continue
		}
		original, ok := originals[p.RefundOf]
		if !ok {
			return fmt.Errorf("refund has no original payment")
		}
		p.Amount, p.Currency, p.FX = original.Amount, original.Currency, original.FX
	}
	return nil
}

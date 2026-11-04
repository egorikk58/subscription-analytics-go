package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"subscription-analytics/internal/core"
)

// Runs only inside the disposable database owned by the integration test.
func exerciseLegacyMigration(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	rows, err := db.Query(ctx, "SELECT data FROM payments.payments ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	amounts := map[string]int64{}
	batch := &pgx.Batch{}
	for rows.Next() {
		var raw []byte
		var p core.Payment
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		amount, convertErr := core.ConvertedMinor(p)
		if convertErr != nil {
			t.Fatal(convertErr)
		}
		amounts[p.ID] = amount
		p.Amount = amount
		p.Currency = "RUB"
		p.FX = nil
		body, _ := json.Marshal(p)
		batch.Queue("UPDATE payments.payments SET data=$1 WHERE id=$2", body, p.ID)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	batch.Queue("UPDATE analytics.seed SET config=regexp_replace(config,'^v2/','v1/') WHERE id=1")
	if err = db.SendBatch(ctx, batch).Close(); err != nil {
		t.Fatal(err)
	}
	if err = bootstrap(ctx, db); err != nil {
		t.Fatalf("upgrade legacy seed: %v", err)
	}
	rows, err = db.Query(ctx, "SELECT data FROM payments.payments ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count, crypto := 0, 0
	for rows.Next() {
		var raw []byte
		var p core.Payment
		if err = rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
		converted, convertErr := core.ConvertedMinor(p)
		if convertErr != nil {
			t.Fatal(convertErr)
		}
		original, ok := amounts[p.ID]
		if !ok || converted != original {
			t.Fatalf("legacy invoice changed: %s %d != %d", p.ID, converted, original)
		}
		if p.Currency != "RUB" {
			crypto++
		}
		count++
	}
	if err = rows.Err(); err != nil || count != len(amounts) || crypto == 0 {
		t.Fatal("migration lost records or crypto", count, len(amounts), crypto, err)
	}
}

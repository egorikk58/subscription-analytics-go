package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"subscription-analytics/internal/core"
)

// Transactional v1->v2 fixture migration. No IDs or source records are removed;
// original RUB invoice values are exactly preserved by the native conversion.
func upgradePayments(ctx context.Context, tx pgx.Tx) error {
	rows, err := tx.Query(ctx, "SELECT data FROM payments.payments ORDER BY id")
	if err != nil {
		return err
	}
	payments := []core.Payment{}
	for rows.Next() {
		var raw []byte
		var payment core.Payment
		if err = rows.Scan(&raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &payment); err != nil {
			rows.Close()
			return err
		}
		payments = append(payments, payment)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if err = core.AddDemoCurrencies(payments); err != nil {
		return err
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	batch := &pgx.Batch{}
	for _, p := range payments {
		p.UpdatedAt = at
		body, marshalErr := json.Marshal(p)
		if marshalErr != nil {
			return marshalErr
		}
		batch.Queue("UPDATE payments.payments SET data=$1,updated_at=$2 WHERE id=$3", body, at, p.ID)
	}
	results := tx.SendBatch(ctx, batch)
	if err = results.Close(); err != nil {
		return err
	}
	slog.Info("payment fixture migration prepared", "version", 2, "payments", len(payments))
	return nil
}

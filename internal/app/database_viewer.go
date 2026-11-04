package app

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"strings"
)

// Optional operations login: SELECT only; no write grants on source or analytics data.
func grantDatabaseViewer(ctx context.Context, tx pgx.Tx) error {
	password := os.Getenv("DB_VIEWER_PASSWORD")
	if password == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='demo_viewer')").Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := tx.Exec(ctx, "CREATE ROLE demo_viewer LOGIN PASSWORD '"+strings.ReplaceAll(password, "'", "''")+"'"); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "ALTER ROLE demo_viewer SET default_transaction_read_only=on; ALTER ROLE demo_viewer SET statement_timeout='10s'"); err != nil {
		return err
	}
	for _, schema := range []string{"users", "subscriptions", "payments", "analytics"} {
		if _, err := tx.Exec(ctx, fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO demo_viewer; GRANT SELECT ON ALL TABLES IN SCHEMA %s TO demo_viewer", schema, schema)); err != nil {
			return err
		}
	}
	return nil
}

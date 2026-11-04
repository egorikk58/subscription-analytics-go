package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"subscription-analytics/internal/core"
	"subscription-analytics/internal/reports"
)

// Bound CPU/memory without blocking ordinary metrics or the sync worker.
var exportGate = make(chan struct{}, 1)

func previewConfig(r *http.Request) (previewRequest, time.Time, error) {
	input := struct {
		Seed          int64  `json:"seed"`
		UsersCount    *int   `json:"users_count"`
		HistoryMonths int    `json:"history_months"`
		DataAsOf      string `json:"data_as_of"`
	}{Seed: 42, HistoryMonths: 24, DataAsOf: "2026-11-01T00:00:00Z"}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4097))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return previewRequest{}, time.Time{}, fmt.Errorf("invalid generation parameters")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return previewRequest{}, time.Time{}, fmt.Errorf("generation body must contain one JSON object")
	}
	p := previewRequest{Seed: input.Seed, UsersCount: core.AutoUsersCount(input.Seed), HistoryMonths: input.HistoryMonths, DataAsOf: input.DataAsOf}
	if input.UsersCount != nil {
		p.UsersCount = *input.UsersCount
	}
	asof, err := validatePreview(p, time.Now())
	return p, asof, err
}

func (s *service) exportWorkbook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	q, err := dashboardQuery(r)
	if err != nil {
		write(w, 400, map[string]any{"error": err.Error(), "data_as_of": nil})
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = "analytics"
	}
	ids := r.URL.Query()["user_id"]
	if (kind != "analytics" && kind != "users") || len(ids) > 1000 {
		write(w, 400, map[string]any{"error": "invalid export options", "data_as_of": nil})
		return
	}
	for _, id := range ids {
		if len(id) != 36 {
			write(w, 400, map[string]any{"error": "invalid user_id", "data_as_of": nil})
			return
		}
	}
	var snapshotID *int64
	if raw := r.URL.Query().Get("snapshot_id"); raw != "" {
		id, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || id < 1 {
			write(w, 400, map[string]any{"error": "invalid snapshot_id", "data_as_of": nil})
			return
		}
		snapshotID = &id
	}
	select {
	case exportGate <- struct{}{}:
		defer func() { <-exportGate }()
	default:
		w.Header().Set("Retry-After", "2")
		write(w, 429, map[string]any{"error": "Отчёт уже формируется. Повторите запрос через несколько секунд.", "data_as_of": nil})
		return
	}
	start := time.Now()
	var snapshot core.Snapshot
	mode, version := "live", ""
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		p, asof, e := previewConfig(r)
		if e != nil {
			write(w, 400, map[string]any{"error": e.Error(), "data_as_of": nil})
			return
		}
		// Preview and preview exports share the same deterministic generator. They
		// never write source tables, published snapshots, or source cursors.
		select {
		case previewGate <- struct{}{}:
			defer func() { <-previewGate }()
		default:
			w.Header().Set("Retry-After", "2")
			write(w, 429, map[string]any{"error": "generation already running", "data_as_of": nil})
			return
		}
		if r.Context().Err() != nil {
			return
		}
		snapshot, err = core.Prepare(core.Generate(p.Seed, p.UsersCount, p.HistoryMonths, asof), asof)
		if err == nil {
			err = snapshot.Dataset.SetConfig(fmt.Sprintf("%s/%d/%d/%d/%s", core.GeneratorVersion, p.Seed, p.UsersCount, p.HistoryMonths, asof.Format(time.RFC3339)))
		}
		if err != nil {
			write(w, 500, map[string]any{"error": "cannot prepare export", "data_as_of": nil})
			return
		}
		mode, version, snapshotID = "preview", fmt.Sprintf("seed-%d", p.Seed), nil
	} else {
		var raw []byte
		var id int64
		err = s.db.QueryRow(r.Context(), "SELECT id,data FROM analytics.snapshots WHERE id=COALESCE($1::bigint,(SELECT current_id FROM analytics.state WHERE id=1))", snapshotID).Scan(&id, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			if snapshotID != nil {
				write(w, 410, map[string]any{"error": "Снимок больше недоступен. Обновите данные и повторите экспорт.", "data_as_of": nil})
			} else {
				w.Header().Set("Retry-After", "5")
				write(w, 503, map[string]any{"error": "snapshot unavailable", "data_as_of": nil})
			}
			return
		}
		if err == nil {
			err = json.Unmarshal(raw, &snapshot)
		}
		if err != nil {
			write(w, 503, map[string]any{"error": "cannot load export snapshot", "data_as_of": nil})
			return
		}
		snapshotID, version = &id, fmt.Sprintf("snapshot-%d", id)
	}
	if fingerprint := r.URL.Query().Get("fingerprint"); fingerprint != "" && fingerprint != snapshot.Dataset.Fingerprint {
		write(w, 409, map[string]any{"error": "Набор данных изменился. Обновите данные и повторите экспорт.", "data_as_of": snapshot.DataAsOf})
		return
	}
	body, err := reports.Build(r.Context(), snapshot, reports.Options{SnapshotID: snapshotID, Mode: mode, Kind: kind, Query: q, UserIDs: ids})
	if err != nil {
		slog.Error("workbook export failed", "error", err, "duration", time.Since(start))
		status := 500
		if err.Error() == "unknown plan_id" {
			status = 400
		}
		write(w, status, map[string]any{"error": "Не удалось сформировать отчёт.", "data_as_of": snapshot.DataAsOf})
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="subscope-%s-%s.xlsx"`, kind, version))
	w.Header().Set("X-Data-As-Of", snapshot.DataAsOf.UTC().Format(time.RFC3339Nano))
	if snapshotID != nil {
		w.Header().Set("X-Snapshot-Version", fmt.Sprintf(`"%d"`, *snapshotID))
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
	slog.Info("workbook exported", "kind", kind, "mode", mode, "bytes", len(body), "duration", time.Since(start))
}

package app

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestWorkbookExportRejectsInvalidOptions(t *testing.T) {
	for _, query := range []string{"kind=csv", "snapshot_id=0", "snapshot_id=nope", "user_id=bad", "from=nope", "selection=invalid"} {
		w := httptest.NewRecorder()
		(&service{}).exportWorkbook(w, httptest.NewRequest(http.MethodGet, "/api/reports/export.xlsx?"+query, nil))
		if w.Code != 400 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatalf("%s: %d %s", query, w.Code, w.Body.String())
		}
	}
}

func TestWorkbookPreviewIsDeterministicAndRequiresNoDatabase(t *testing.T) {
	for _, kind := range []string{"analytics", "users"} {
		w := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/reports/export.xlsx?kind="+kind+"&search=1000000000", strings.NewReader(`{"seed":731,"users_count":10000}`))
		(&service{}).exportWorkbook(w, request)
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		value, err := f.GetCellValue("Пользователи", "C10")
		_ = f.Close()
		if err != nil || value != "1000000000" || !strings.Contains(w.Header().Get("Content-Disposition"), "seed-731.xlsx") {
			t.Fatal("wrong preview export", value, err)
		}
		if w.Header().Get("X-Snapshot-Version") != "" {
			t.Fatal("preview is not a published database snapshot")
		}
	}
}

func TestWorkbookExportCapacityAndMalformedPreview(t *testing.T) {
	exportGate <- struct{}{}
	w := httptest.NewRecorder()
	(&service{}).exportWorkbook(w, httptest.NewRequest(http.MethodGet, "/api/reports/export.xlsx", nil))
	<-exportGate
	if w.Code != 429 || w.Header().Get("Retry-After") != "2" {
		t.Fatal("busy export must not wait or access the database", w.Code)
	}
	for _, body := range []string{`{"seed":42,"unknown":true}`, `{"seed":42} {}`, `{"users_count":2}`, `{"seed":"text"}`} {
		w = httptest.NewRecorder()
		(&service{}).exportWorkbook(w, httptest.NewRequest(http.MethodPost, "/api/reports/export.xlsx", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal("malformed preview accepted", body, w.Code)
		}
	}
}

func exerciseWorkbookExport(t *testing.T, svc *service, snapshotID int64) {
	t.Helper()
	request := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		svc.exportWorkbook(w, httptest.NewRequest(http.MethodGet, "/api/reports/export.xlsx?"+query, nil))
		return w
	}
	w := request(fmt.Sprintf("snapshot_id=%d&search=1000000000", snapshotID))
	if w.Code != 200 || w.Header().Get("X-Snapshot-Version") != fmt.Sprintf(`"%d"`, snapshotID) {
		t.Fatalf("export pinned snapshot: %d %s", w.Code, w.Body.String())
	}
	f, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	count, err := f.GetCellValue("Сводка", "C10")
	_ = f.Close()
	if err != nil || count != "1" {
		t.Fatal("export must apply search", count, err)
	}
	w = request("snapshot_id=9223372036854775807")
	if w.Code != 410 {
		t.Fatal("missing pinned snapshot must never silently export current data", w.Code)
	}
	w = request(fmt.Sprintf("snapshot_id=%d&fingerprint=wrong", snapshotID))
	if w.Code != 409 {
		t.Fatal("fingerprint mismatch not detected", w.Code)
	}
	// A saved snapshot remains exportable independently of the current pointer.
	if _, err = svc.db.Exec(context.Background(), "UPDATE analytics.state SET current_id=NULL WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, restoreErr := svc.db.Exec(context.Background(), "UPDATE analytics.state SET current_id=$1 WHERE id=1", snapshotID); restoreErr != nil {
			t.Error(restoreErr)
		}
	}()
	if pinned := request(fmt.Sprintf("snapshot_id=%d&kind=users&search=1000000000", snapshotID)); pinned.Code != 200 {
		t.Fatal("pinned export depends on current pointer", pinned.Code)
	}
	if current := request(""); current.Code != 503 || current.Header().Get("Retry-After") != "5" {
		t.Fatal("no current snapshot must return retryable 503", current.Code)
	}
}

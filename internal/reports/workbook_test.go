package reports

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
	"subscription-analytics/internal/core"
)

func reportFixture() core.Snapshot {
	start := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	s := core.Snapshot{DataAsOf: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), Plans: []core.Plan{{ID: "monthly", Name: "Monthly", PeriodMonths: 1, Price: 99000, Currency: "RUB"}, {ID: "quarterly", Name: "Quarterly", PeriodMonths: 3, Price: 269000, Currency: "RUB"}}}
	for i := 0; i < 25; i++ {
		s.Facts = append(s.Facts, core.Fact{ID: core.ID(42, fmt.Sprint(i)), TelegramID: 1000000000 + int64(i), First: &start, FirstPlan: "monthly", UTMLink: "campaign-0", LTV: 99000, PaymentsCount: 1, CurrentStatus: "active", Chains: []core.Chain{{PlanID: "monthly", Start: start, Months: 1, Paid: 2}}})
	}
	s.Facts[0].UTMLink = "=HYPERLINK(\"https://example.invalid\")"
	s.Facts = append(s.Facts, core.Fact{ID: core.ID(42, "quarter"), TelegramID: 9007199254740993, First: &start, FirstPlan: "quarterly", UTMLink: "campaign-1", LTV: 269000, PaymentsCount: 1, CurrentStatus: "active", Chains: []core.Chain{{PlanID: "quarterly", Start: start, Months: 3, Paid: 1}}}, core.Fact{ID: core.ID(42, "unpaid"), TelegramID: 12345, CurrentStatus: "none"})
	return s
}

func openReport(t *testing.T, s core.Snapshot, opt Options) (*excelize.File, []byte) {
	t.Helper()
	data, err := Build(context.Background(), s, opt)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f, data
}

func raw(t *testing.T, f *excelize.File, sheet, address string) string {
	t.Helper()
	value, err := f.GetCellValue(sheet, address, excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestWorkbookFullSelectionAndCharts(t *testing.T) {
	s := reportFixture()
	id := int64(7)
	f, data := openReport(t, s, Options{Kind: "analytics", Mode: "live", SnapshotID: &id, Query: core.DashboardQuery{Page: 2, Sort: "ltv_desc"}})
	wantSheets := []string{"Сводка", "Тарифы", "Каналы", "По месяцам", "Продления", "Когорты", "Пользователи"}
	if fmt.Sprint(f.GetSheetList()) != fmt.Sprint(wantSheets) {
		t.Fatal(f.GetSheetList())
	}
	if raw(t, f, "Сводка", "C10") != "27" || raw(t, f, "Сводка", "C11") != "27440" {
		t.Fatal("summary must reconcile with all 27 users, including unpaid users")
	}
	if dimension, err := f.GetSheetDimension("Пользователи"); err != nil || dimension != "B2:J36" {
		t.Fatal("streamed sheet must declare its full used range", dimension, err)
	}
	rows, err := f.GetRows("Пользователи")
	if err != nil || len(rows) != 36 {
		t.Fatalf("expected every user, not page 2: rows=%d error=%v", len(rows), err)
	}
	if raw(t, f, "Пользователи", "C10") != "9007199254740993" {
		t.Fatal("Telegram identifier lost precision")
	}
	if date, err := f.GetCellValue("Пользователи", "D10"); err != nil || date != "2025-01-31" {
		t.Fatal("date must reopen with a readable date format", date, err)
	}
	cellType, err := f.GetCellType("Пользователи", "C10")
	if err != nil || cellType != excelize.CellTypeInlineString {
		t.Fatalf("Telegram ID must be text: %v %v", cellType, err)
	}
	formula, err := f.GetCellFormula("Пользователи", "J10")
	if err != nil || formula != "I10/100" || raw(t, f, "Пользователи", "J10") != "2690" {
		t.Fatal("LTV formula and its cached numeric value must survive reopening", formula, err)
	}
	var minor int64
	for n := 10; n <= 36; n++ {
		value, e := strconv.ParseInt(raw(t, f, "Пользователи", fmt.Sprintf("I%d", n)), 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		minor += value
		if strings.HasPrefix(raw(t, f, "Пользователи", fmt.Sprintf("F%d", n)), "=HYPERLINK") {
			formula, e = f.GetCellFormula("Пользователи", fmt.Sprintf("F%d", n))
			if e != nil || formula != "" {
				t.Fatal("UTM was evaluated as a formula")
			}
		}
	}
	if minor != 2744000 {
		t.Fatal("detail does not reconcile", minor)
	}
	if raw(t, f, "Продления", "G11") != "" || raw(t, f, "Продления", "I11") != "" {
		t.Fatal("immature rates must be blank, not zero")
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	charts := 0
	for _, file := range archive.File {
		if strings.HasPrefix(file.Name, "xl/charts/chart") && strings.HasSuffix(file.Name, ".xml") {
			charts++
			r, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			xml, e := io.ReadAll(r)
			_ = r.Close()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Contains(xml, []byte("<numRef>")) || !bytes.Contains(xml, []byte("<f>")) {
				t.Fatal("editable chart needs numeric source cell ranges", file.Name)
			}
			if !bytes.Contains(xml, []byte("<numCache>")) || !bytes.Contains(xml, []byte("<strCache>")) {
				t.Fatal("chart preview cache must include labels and numeric points", file.Name)
			}
		}
	}
	if charts != 4 {
		t.Fatalf("wanted 4 native charts, got %d", charts)
	}
	panes, err := f.GetPanes("Когорты")
	if err != nil || panes.YSplit != 9 {
		t.Fatal("cohort header must remain visible", panes, err)
	}
}

func TestWorkbookFiltersSelectionAndEmptyResults(t *testing.T) {
	s := reportFixture()
	utm := "campaign-0"
	opt := Options{Kind: "analytics", Mode: "preview", Query: core.DashboardQuery{Plan: "monthly", Selection: "chain", UTMLink: &utm, Search: "100000001", Sort: "ltv_desc"}}
	f, _ := openReport(t, s, opt)
	if raw(t, f, "Сводка", "C10") != "10" || raw(t, f, "Сводка", "C11") != "9900" {
		t.Fatal("search must scope every sheet to the same 10 users")
	}
	opt.UserIDs = []string{s.Facts[10].ID}
	f, _ = openReport(t, s, opt)
	if raw(t, f, "Сводка", "C10") != "1" || raw(t, f, "Сводка", "C11") != "990" {
		t.Fatal("checked IDs must scope summaries as well as detail")
	}
	opt.Query.Search = "not-a-user"
	f, _ = openReport(t, s, opt)
	if raw(t, f, "Сводка", "C10") != "0" || raw(t, f, "Сводка", "C11") != "0" || !strings.Contains(raw(t, f, "Пользователи", "B10"), "Нет пользователей") {
		t.Fatal("empty selection must still produce a valid workbook")
	}
	end := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	opt.Query = core.DashboardQuery{To: &end}
	opt.UserIDs = nil
	f, _ = openReport(t, s, opt)
	if raw(t, f, "Сводка", "C10") != "0" {
		t.Fatal("upper date boundary must be exclusive")
	}
}

func TestUsersWorkbookAndCancellation(t *testing.T) {
	f, _ := openReport(t, reportFixture(), Options{Kind: "users", Query: core.DashboardQuery{Search: "12345"}})
	if len(f.GetSheetList()) != 1 || raw(t, f, "Пользователи", "C10") != "12345" || raw(t, f, "Пользователи", "D10") != "" {
		t.Fatal("users-only report must retain the user without a subscription")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, reportFixture(), Options{Kind: "analytics"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled export must stop", err)
	}
}

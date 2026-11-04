// Package reports builds portable Excel reports from one immutable snapshot.
package reports

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
	"subscription-analytics/internal/core"
)

type Options struct {
	SnapshotID *int64
	Mode       string
	Kind       string // analytics or users
	Query      core.DashboardQuery
	UserIDs    []string
}

type builder struct {
	f                         *excelize.File
	err                       error
	body, title, header, note int
	money, count, date, rate  int
}

// Build exports all matching users, never just a dashboard page. Search and
// explicit user selection scope every metric and chart to those same users.
func Build(ctx context.Context, snapshot core.Snapshot, opt Options) ([]byte, error) {
	if opt.Kind != "analytics" && opt.Kind != "users" {
		return nil, fmt.Errorf("unknown report kind")
	}
	facts, err := snapshot.SelectedFacts(opt.Query)
	if err != nil {
		return nil, err
	}
	if len(opt.UserIDs) > 0 {
		ids := make(map[string]bool, len(opt.UserIDs))
		for _, id := range opt.UserIDs {
			ids[id] = true
		}
		selected := make([]core.Fact, 0, len(ids))
		for _, fact := range facts {
			if ids[fact.ID] {
				selected = append(selected, fact)
			}
		}
		facts = selected
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	snapshot.Facts = facts
	snapshot.Plans = append([]core.Plan(nil), snapshot.Plans...)
	sort.Slice(snapshot.Plans, func(i, j int) bool { return snapshot.Plans[i].PeriodMonths < snapshot.Plans[j].PeriodMonths })
	query := opt.Query
	query.Search, query.Page = "", 1
	dashboard, err := snapshot.Dashboard(query)
	if err != nil {
		return nil, err
	}
	dashboard.SnapshotID, dashboard.Mode = opt.SnapshotID, opt.Mode
	b := newBuilder()
	defer func() { _ = b.f.Close() }()
	sheets := []string{"Пользователи"}
	if opt.Kind == "analytics" {
		sheets = []string{"Сводка", "Тарифы", "Каналы", "По месяцам", "Продления", "Когорты", "Пользователи"}
	}
	b.check(b.f.SetSheetName("Sheet1", sheets[0]))
	for _, name := range sheets[1:] {
		_, err = b.f.NewSheet(name)
		b.check(err)
	}
	if opt.Kind == "analytics" {
		b.analytics(dashboard, opt, facts)
	}
	b.users(ctx, dashboard, opt, facts)
	if b.err != nil {
		return nil, b.err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	b.f.SetActiveSheet(0)
	buffer, err := b.f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return chartCaches(buffer.Bytes(), b.f)
}

func newBuilder() *builder {
	b := &builder{f: excelize.NewFile()}
	style := func(s *excelize.Style) int { id, err := b.f.NewStyle(s); b.check(err); return id }
	font := func() *excelize.Font { return &excelize.Font{Family: "Arial", Size: 10, Color: "182329"} }
	b.body = style(&excelize.Style{Font: font(), Alignment: &excelize.Alignment{Vertical: "center"}})
	b.title = style(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 16, Bold: true, Color: "182329"}})
	b.note = style(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 10, Color: "617079"}})
	b.header = style(&excelize.Style{Font: &excelize.Font{Family: "Arial", Size: 10, Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"233F46"}}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true}})
	format := func(code string) int {
		return style(&excelize.Style{Font: font(), CustomNumFmt: &code, Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"}})
	}
	b.money, b.count, b.date, b.rate = format(`#,##0.00 "₽";[Red](#,##0.00) "₽"`), format("#,##0"), format("yyyy-mm-dd"), format("0.0%")
	return b
}

func (b *builder) check(err error) {
	if err != nil && b.err == nil {
		b.err = err
	}
}
func cell(col, row int) string { name, _ := excelize.CoordinatesToCellName(col, row); return name }
func (b *builder) value(sheet, address string, value any) {
	b.check(b.f.SetCellValue(sheet, address, value))
}
func (b *builder) styled(value any, style int) excelize.Cell {
	return excelize.Cell{Value: value, StyleID: style}
}
func (b *builder) formula(formula string, value any, style int) excelize.Cell {
	return excelize.Cell{Formula: formula, Value: value, StyleID: style}
}

func planName(id string) string {
	switch id {
	case "monthly":
		return "Месячный"
	case "quarterly":
		return "Квартальный"
	case "half-year":
		return "Полугодовой"
	case "yearly":
		return "Годовой"
	case "":
		return "Без подписки"
	}
	return id
}
func utmName(value string) string {
	if core.NormalizeUTM(value) == core.NormalizeUTM("") {
		return "Без UTM"
	}
	return value
}
func asRUB(value any) float64 {
	r, ok := new(big.Rat).SetString(fmt.Sprint(value))
	if !ok {
		return 0
	}
	r.Quo(r, big.NewRat(100, 1))
	f, _ := r.Float64()
	return f
}
func countValue(value any) int { n, _ := strconv.Atoi(fmt.Sprint(value)); return n }
func bound(value *time.Time, fallback string) string {
	if value == nil {
		return fallback
	}
	return value.UTC().Format(time.RFC3339)
}

func contextLines(d core.Dashboard, opt Options) []string {
	version := "Предпросмотр"
	if opt.SnapshotID != nil {
		version = fmt.Sprintf("Снимок #%d", *opt.SnapshotID)
	}
	seed := "—"
	if d.Dataset.Seed != nil {
		seed = strconv.FormatInt(*d.Dataset.Seed, 10)
	}
	plan := "Все тарифы"
	if opt.Query.Plan != "" && opt.Query.Plan != "all" {
		plan = planName(opt.Query.Plan)
	}
	utm := "Все каналы"
	if opt.Query.UTMLink != nil {
		utm = utmName(*opt.Query.UTMLink)
	}
	selection := "Первый тариф"
	if opt.Query.Selection == "chain" {
		selection = "Участие в цепочке тарифа"
	}
	search := opt.Query.Search
	if search == "" {
		search = "Нет"
	}
	return []string{
		fmt.Sprintf("%s. Данные на %s UTC. Seed %s. Генератор %s.", version, d.DataAsOf.UTC().Format("02.01.2006 15:04:05"), seed, d.Dataset.GeneratorVersion),
		fmt.Sprintf("Первая подписка: от %s включительно до %s исключительно.", bound(opt.Query.From, "начала истории"), bound(opt.Query.To, "конца истории")),
		fmt.Sprintf("%s. %s. %s. Поиск: %s.", plan, selection, utm, search),
		fmt.Sprintf("В выгрузке %d пользователей. Выбрано по ID: %d. Выручка и LTV — за всю жизнь, после возвратов, в RUB.", countValue(d.Summary["users_count"]), len(opt.UserIDs)),
		fmt.Sprintf("История: %d мес. Граница данных: %s. Fingerprint: %s.", d.Dataset.HistoryMonths, bound(d.Dataset.DatasetAsOf, "не задана"), d.Dataset.Fingerprint),
	}
}

func (b *builder) setup(sheet, title string, d core.Dashboard, opt Options) {
	b.check(b.f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: boolPtr(false)}))
	b.check(b.f.SetColWidth(sheet, "A", "A", 3))
	b.check(b.f.SetColWidth(sheet, "B", "N", 17))
	b.value(sheet, "B2", title)
	b.check(b.f.SetCellStyle(sheet, "B2", "B2", b.title))
	b.check(b.f.SetRowHeight(sheet, 2, 27))
	for i, line := range contextLines(d, opt) {
		b.value(sheet, cell(2, i+3), line)
		b.check(b.f.SetCellStyle(sheet, cell(2, i+3), cell(2, i+3), b.note))
	}
}
func boolPtr(v bool) *bool { return &v }

func (b *builder) table(sheet, name string, row int, headers []string, rows [][]any, widths []float64) {
	for i, width := range widths {
		b.check(b.f.SetColWidth(sheet, cellColumn(i+2), cellColumn(i+2), width))
	}
	values := make([]any, len(headers))
	for i, h := range headers {
		values[i] = h
	}
	b.check(b.f.SetSheetRow(sheet, cell(2, row), &values))
	b.check(b.f.SetCellStyle(sheet, cell(2, row), cell(len(headers)+1, row), b.header))
	b.check(b.f.SetRowHeight(sheet, row, 32))
	for i, values := range rows {
		for j, value := range values {
			address := cell(j+2, row+i+1)
			if v, ok := value.(excelize.Cell); ok {
				b.value(sheet, address, v.Value)
				b.check(b.f.SetCellStyle(sheet, address, address, v.StyleID))
			} else {
				b.value(sheet, address, value)
				b.check(b.f.SetCellStyle(sheet, address, address, b.body))
			}
		}
		b.check(b.f.SetRowHeight(sheet, row+i+1, 21))
	}
	if len(rows) > 0 {
		b.check(b.f.AddTable(sheet, &excelize.Table{Range: fmt.Sprintf("%s:%s", cell(2, row), cell(len(headers)+1, row+len(rows))), Name: name, StyleName: "TableStyleMedium2", ShowRowStripes: boolPtr(false)}))
	}
	if len(rows) == 0 {
		b.value(sheet, cell(2, row+1), "Нет данных по выбранным условиям")
	}
	if row == 9 && sheet != "Сводка" {
		b.check(b.f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: row, XSplit: 2, TopLeftCell: cell(3, row+1), ActivePane: "bottomRight"}))
	}
}
func cellColumn(col int) string { value, _ := excelize.ColumnNumberToName(col); return value }

func (b *builder) analytics(d core.Dashboard, opt Options, facts []core.Fact) {
	b.setup("Сводка", "Аналитика подписок", d, opt)
	paid, active, payments := 0, 0, 0
	for _, fact := range facts {
		if fact.PaymentsCount > 0 {
			paid++
		}
		if fact.CurrentStatus == "active" {
			active++
		}
		payments += fact.PaymentsCount
	}
	metrics := [][]any{
		{"Пользователей", b.styled(len(facts), b.count)}, {"Выручка после возвратов", b.styled(asRUB(d.Summary["total_revenue_minor"]), b.money)},
		{"Средний LTV", b.styled(asRUB(d.Summary["mean_ltv_minor"]), b.money)}, {"Медианный LTV", b.styled(asRUB(d.Summary["median_ltv_minor"]), b.money)},
		{"С успешными оплатами", b.styled(paid, b.count)}, {"Без успешных оплат", b.styled(len(facts)-paid, b.count)}, {"Активных подписчиков", b.styled(active, b.count)}, {"Успешных оплат", b.styled(payments, b.count)},
	}
	b.table("Сводка", "SummaryMetrics", 9, []string{"Показатель", "Значение"}, metrics, []float64{34, 23})
	b.value("Сводка", "B21", "Проверка детализации")
	b.value("Сводка", "B22", "Число строк на листе «Пользователи»")
	b.value("Сводка", "C22", len(facts))
	b.value("Сводка", "B24", "Продления рассчитаны для пользователей этой выгрузки. Ненаблюдаемые периоды оставлены пустыми.")
	b.value("Сводка", "B25", "Месячные группы отражают первую подписку и lifetime-выручку когорты, а не денежный поток за месяц.")
	b.value("Сводка", "B26", "Криптовалютные оплаты приведены к RUB по сохранённому курсу оплаты; возвраты используют исходный курс.")
	for _, address := range []string{"B24", "B25", "B26"} {
		b.check(b.f.SetCellStyle("Сводка", address, address, b.note))
	}
	groupHeaders := []string{"Тариф", "Пользователей", "Выручка, ₽", "Средний LTV, ₽", "Медианный LTV, ₽", "Доля выручки"}
	total := asRUB(d.Summary["total_revenue_minor"])
	groupRows := func(groups []map[string]any, key string) [][]any {
		rows := make([][]any, 0, len(groups))
		for _, group := range groups {
			label := fmt.Sprint(group[key])
			if key == "plan_id" {
				label = planName(label)
			} else {
				label = utmName(label)
			}
			revenue := asRUB(group["total_revenue_minor"])
			var share any
			if total != 0 {
				share = revenue / total
			}
			rows = append(rows, []any{label, b.styled(group["users_count"], b.count), b.styled(revenue, b.money), b.styled(asRUB(group["mean_ltv_minor"]), b.money), b.styled(asRUB(group["median_ltv_minor"]), b.money), b.styled(share, b.rate)})
		}
		return rows
	}
	b.setup("Тарифы", "LTV по первому тарифу", d, opt)
	planRows := groupRows(d.Plans, "plan_id")
	without := []core.Fact{}
	for _, fact := range facts {
		if fact.FirstPlan == "" {
			without = append(without, fact)
		}
	}
	if len(without) > 0 {
		planRows = append(planRows, []any{"Без подписки", b.styled(len(without), b.count), b.styled(0, b.money), b.styled(0, b.money), b.styled(0, b.money), b.styled(0, b.rate)})
	}
	b.table("Тарифы", "PlanMetrics", 9, groupHeaders, planRows, []float64{23, 19, 24, 23, 23, 20})
	b.setup("Каналы", "LTV по каналам привлечения", d, opt)
	utmHeaders := append([]string(nil), groupHeaders...)
	utmHeaders[0] = "UTM-источник"
	b.table("Каналы", "ChannelMetrics", 9, utmHeaders, groupRows(d.UTMs, "utm_link"), []float64{28, 19, 24, 23, 23, 20})
	b.setup("По месяцам", "Когорты по месяцу первой подписки", d, opt)
	monthRows := make([][]any, 0, len(d.Timeline))
	for _, row := range d.Timeline {
		month, _ := time.Parse("2006-01", fmt.Sprint(row["month"]))
		monthRows = append(monthRows, []any{b.styled(month, b.date), b.styled(row["users_count"], b.count), b.styled(asRUB(row["total_revenue_minor"]), b.money), b.styled(asRUB(row["mean_ltv_minor"]), b.money), b.styled(asRUB(row["median_ltv_minor"]), b.money)})
	}
	b.table("По месяцам", "MonthlyCohorts", 9, []string{"Месяц первой подписки", "Пользователей", "Lifetime-выручка, ₽", "Средний LTV, ₽", "Медианный LTV, ₽"}, monthRows, []float64{27, 19, 25, 23, 23})
	b.retention(d, opt)
	b.cohorts(d, opt)
	if len(facts) > 0 {
		b.chart("Сводка", "F9", "Выручка по первому тарифу, ₽", excelize.Col, []excelize.ChartSeries{{Name: "'Тарифы'!$D$9", Categories: fmt.Sprintf("'Тарифы'!$B$10:$B$%d", 9+len(planRows)), Values: fmt.Sprintf("'Тарифы'!$D$10:$D$%d", 9+len(planRows)), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"527E6B"}}}}, false)
		if len(monthRows) > 0 {
			b.chart("Сводка", "B29", "Средний LTV по месяцу первой подписки, ₽", excelize.Line, []excelize.ChartSeries{{Name: "'По месяцам'!$E$9", Categories: fmt.Sprintf("'По месяцам'!$B$10:$B$%d", 9+len(monthRows)), Values: fmt.Sprintf("'По месяцам'!$E$10:$E$%d", 9+len(monthRows)), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"527E6B"}}, Line: excelize.ChartLine{Width: 2}}}, false)
		}
		if len(d.UTMs) > 0 {
			b.chart("Каналы", cell(2, max(20, len(d.UTMs)+13)), "Выручка по каналам, ₽", excelize.Bar, []excelize.ChartSeries{{Name: "'Каналы'!$D$9", Categories: fmt.Sprintf("'Каналы'!$B$10:$B$%d", 9+len(d.UTMs)), Values: fmt.Sprintf("'Каналы'!$D$10:$D$%d", 9+len(d.UTMs)), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"527E6B"}}}}, false)
		}
	}
}

func (b *builder) chart(sheet, anchor, title string, kind excelize.ChartType, series []excelize.ChartSeries, percent bool) {
	zero, one := 0.0, 1.0
	axisFont := excelize.Font{Family: "Arial", Size: 9, Color: "617079"}
	y := excelize.ChartAxis{Font: axisFont, Minimum: &zero, MajorGridLines: true, NumFmt: excelize.ChartNumFmt{CustomNumFmt: "#,##0"}}
	if percent {
		y.Maximum = &one
		y.NumFmt.CustomNumFmt = "0%"
	}
	legend := "none"
	if len(series) > 1 {
		legend = "bottom"
	}
	b.check(b.f.AddChart(sheet, anchor, &excelize.Chart{Type: kind, Series: series, VaryColors: boolPtr(false), Title: []excelize.RichTextRun{{Text: title, Font: &excelize.Font{Family: "Arial", Size: 12, Color: "182329"}}}, Dimension: excelize.ChartDimension{Width: 660, Height: 310}, Legend: excelize.ChartLegend{Position: legend}, XAxis: excelize.ChartAxis{Font: axisFont}, YAxis: y, ShowBlanksAs: "gap"}))
}

func (b *builder) retention(d core.Dashboard, opt Options) {
	const sheet = "Продления"
	b.setup(sheet, "Продления подписок по тарифам", d, opt)
	rows := [][]any{}
	for _, report := range d.Retention {
		for _, p := range report.Periods {
			var rate, cumulative any
			if p.Rate != nil {
				rate = *p.Rate
			}
			if p.CumulativeRate != nil {
				cumulative = *p.CumulativeRate
			}
			rows = append(rows, []any{planName(report.PlanID), b.styled(p.Period, b.count), b.styled(report.CohortSize, b.count), b.styled(p.Eligible, b.count), b.styled(p.Renewed, b.count), b.styled(rate, b.rate), b.styled(p.CumulativeEligible, b.count), b.styled(cumulative, b.rate)})
		}
	}
	b.table(sheet, "RenewalMetrics", 9, []string{"Тариф", "Период", "Исходная когорта", "Дошли и созрели", "Продлили", "Условная доля", "Созрели из исходной когорты", "Кумулятивная доля"}, rows, []float64{23, 12, 20, 20, 17, 20, 27, 24})
	series := []excelize.ChartSeries{}
	colors := []string{"527E6B", "486A96", "99733B", "8D627F"}
	for i, r := range d.Retention {
		if r.CohortSize == 0 {
			continue
		}
		start := 10 + i*24
		end := start + 23
		series = append(series, excelize.ChartSeries{Name: fmt.Sprintf("'%s'!$B$%d", sheet, start), Categories: fmt.Sprintf("'%s'!$C$%d:$C$%d", sheet, start, end), Values: fmt.Sprintf("'%s'!$I$%d:$I$%d", sheet, start, end), Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{colors[i%len(colors)]}}, Line: excelize.ChartLine{Width: 2}})
	}
	if len(series) > 0 {
		b.chart(sheet, "K9", "Кумулятивная доля продлений по номеру периода", excelize.Line, series, true)
	}
}

func (b *builder) cohorts(d core.Dashboard, opt Options) {
	const sheet = "Когорты"
	plan := opt.Query.Plan
	if plan == "" || plan == "all" {
		plan = "monthly"
	}
	b.setup(sheet, "Когорты продлений: "+planName(plan), d, opt)
	rows := [][]any{}
	headers := []string{"Месяц первой подписки", "Участников"}
	for n := 1; n <= 12; n++ {
		headers = append(headers, fmt.Sprintf("Период %d", n))
	}
	for _, cohort := range d.Cohorts {
		month, _ := time.Parse("2006-01", cohort.Month)
		row := []any{b.styled(month, b.date), b.styled(cohort.UsersCount, b.count)}
		for _, p := range cohort.Periods {
			var rate any
			if p.CumulativeRate != nil {
				rate = *p.CumulativeRate
			}
			row = append(row, b.styled(rate, b.rate))
		}
		rows = append(rows, row)
	}
	widths := []float64{27, 17}
	for n := 0; n < 12; n++ {
		widths = append(widths, 14)
	}
	b.table(sheet, "RetentionCohorts", 9, headers, rows, widths)
	if len(rows) > 0 {
		b.check(b.f.SetConditionalFormat(sheet, fmt.Sprintf("D10:O%d", 9+len(rows)), []excelize.ConditionalFormatOptions{{Type: "3_color_scale", Criteria: "=", MinType: "num", MinValue: "0", MidType: "num", MidValue: "0.5", MaxType: "num", MaxValue: "1", MinColor: "F4F0E8", MidColor: "C8DCCF", MaxColor: "527E6B"}}))
	}
	// Explicit denominators keep cohort maturity auditable alongside the heatmap.
	detail := [][]any{}
	for _, cohort := range d.Cohorts {
		for _, p := range cohort.Periods {
			detail = append(detail, []any{cohort.Month, p.Period, p.CumulativeEligible, p.Renewed})
		}
	}
	b.table(sheet, "CohortDenominators", 13+len(rows), []string{"Когорта", "Период", "Созрели", "Продлили"}, detail, nil)
}

func (b *builder) users(ctx context.Context, d core.Dashboard, opt Options, facts []core.Fact) {
	const sheet = "Пользователи"
	// Streaming avoids keeping hundreds of thousands of styled cells in memory.
	b.check(b.f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: boolPtr(false)}))
	b.check(b.f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 9, XSplit: 3, TopLeftCell: "D10", ActivePane: "bottomRight"}))
	b.check(b.f.SetSheetDimension(sheet, fmt.Sprintf("B2:J%d", max(10, 9+len(facts)))))
	sw, err := b.f.NewStreamWriter(sheet)
	if err != nil {
		b.check(err)
		return
	}
	widths := []float64{3, 39, 22, 25, 23, 28, 21, 22, 24, 24}
	for i, w := range widths {
		b.check(sw.SetColWidth(i+1, i+1, w))
	}
	b.check(sw.SetRow("B2", []any{b.styled("Реестр пользователей", b.title)}, excelize.RowOpts{Height: 27}))
	for i, line := range contextLines(d, opt) {
		b.check(sw.SetRow(cell(2, i+3), []any{b.styled(line, b.note)}))
	}
	headers := []string{"ID пользователя", "Telegram ID", "Первая подписка, UTC", "Первый тариф", "UTM-источник", "Текущий статус", "Успешных оплат", "LTV, копейки", "LTV, ₽"}
	row := make([]any, len(headers))
	for i, h := range headers {
		row[i] = b.styled(h, b.header)
	}
	b.check(sw.SetRow("B9", row, excelize.RowOpts{Height: 32}))
	for i, fact := range facts {
		if i%256 == 0 {
			if err = ctx.Err(); err != nil {
				b.check(err)
				return
			}
		}
		n := i + 10
		var first any
		if fact.First != nil {
			first = *fact.First
		}
		status := map[string]string{"active": "Активна", "canceled": "Отменена", "expired": "Истекла", "none": "Без подписки", "unknown": "Неизвестен"}[fact.CurrentStatus]
		if status == "" {
			status = fact.CurrentStatus
		}
		if fact.First == nil {
			status = "Без подписки"
		}
		values := []any{b.styled(fact.ID, b.body), b.styled(strconv.FormatInt(fact.TelegramID, 10), b.body), b.styled(first, b.date), b.styled(planName(fact.FirstPlan), b.body), b.styled(utmName(fact.UTMLink), b.body), b.styled(status, b.body), b.styled(fact.PaymentsCount, b.count), b.styled(fact.LTV, b.count), b.formula(fmt.Sprintf("I%d/100", n), asRUB(fact.LTV), b.money)}
		b.check(sw.SetRow(cell(2, n), values, excelize.RowOpts{Height: 20}))
	}
	if len(facts) > 0 {
		b.check(sw.AddTable(&excelize.Table{Range: fmt.Sprintf("B9:J%d", 9+len(facts)), Name: "Users", StyleName: "TableStyleMedium2", ShowRowStripes: boolPtr(false)}))
	} else {
		b.check(sw.SetRow("B10", []any{"Нет пользователей по выбранным условиям"}))
	}
	b.check(sw.Flush())
}

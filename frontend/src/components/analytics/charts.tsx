import { useId, useState, type KeyboardEvent, type ReactNode } from "react";
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceDot,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { Button } from "@/components/ui/button";
import {
  compact,
  money,
  monthName,
  number,
  percent,
} from "@/lib/format";
import type {
  DistributionRow,
  PeriodRow,
  PlanSummary,
  RetentionMetric,
  RetentionReport,
  TimelineRow,
  TrendMetric,
} from "@/types";
import { EmptyState, Panel, PanelNote } from "./panel";
import { planColor, planName } from "./plans";

const axisStyle = {
  fontSize: 12,
  fill: "#5d5f60",
  fontFamily: '"Geist", sans-serif',
};
const seriesDash = (index: number) =>
  [undefined, "6 3", "2 3", "10 3 2 3"][index % 4];

function MetricTabs<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: readonly (readonly [T, string])[];
  value: T;
  onChange: (value: T) => void;
  label: string;
}) {
  return (
    <div
      className="flex border-b border-border"
      role="group"
      aria-label={label}
    >
      {options.map(([key, text]) => (
        <Button
          key={key}
          type="button"
          variant="ghost"
          size="sm"
          aria-pressed={value === key}
          onClick={() => onChange(key)}
          className={`-mb-px min-h-10 rounded-none border-b-2 px-2 text-[13px] shadow-none sm:min-h-8 ${value === key ? "border-[#1d5fbf] font-medium text-foreground" : "border-transparent font-normal text-muted-foreground"}`}
        >
          {text}
        </Button>
      ))}
    </div>
  );
}

function ChartTooltipBox({ children }: { children: ReactNode }) {
  return (
    <div className="max-w-[280px] rounded-md border border-border bg-background px-3 py-2 text-[13px] leading-5 shadow-md">
      {children}
    </div>
  );
}

function moveIndex(
  event: KeyboardEvent<HTMLDivElement>,
  current: number,
  length: number,
  select?: (index: number) => void,
): number {
  let next = current;
  if (event.key === "ArrowLeft") next = Math.max(0, current - 1);
  else if (event.key === "ArrowRight") next = Math.min(length - 1, current + 1);
  else if (event.key === "Home") next = 0;
  else if (event.key === "End") next = length - 1;
  else if ((event.key === "Enter" || event.key === " ") && select)
    select(current);
  else return current;
  event.preventDefault();
  return next;
}

export interface TimelineChartProps {
  rows: TimelineRow[];
  metric: TrendMetric;
  onMetricChange: (metric: TrendMetric) => void;
  onMonthClick: (month: string) => void;
}

export function TimelineChart({
  rows,
  metric,
  onMetricChange,
  onMonthClick,
}: TimelineChartProps) {
  const descriptionId = useId();
  const [focused, setFocused] = useState(false);
  const [index, setIndex] = useState(0);
  const [width, setWidth] = useState(600);
  const moneyMetric = metric !== "users_count";
  const title =
    metric === "mean_ltv_minor"
      ? "Средний LTV по когортам"
      : metric === "users_count"
        ? "Размер когорт"
        : "Выручка когорт";
  const points = rows.map((row) => ({
    ...row,
    timestamp: Date.parse(`${row.month}-01T00:00:00Z`),
    value: Number(row[metric]),
  }));
  const active = points[Math.min(index, points.length - 1)];
  const tickEvery = Math.max(
    1,
    Math.ceil(points.length / (width < 480 ? 3 : 6)),
  );
  const ticks = points
    .filter((_, i) => i % tickEvery === 0)
    .map((point) => point.timestamp);
  const minimum = points[0]?.timestamp ?? 0;
  const maximum = points.at(-1)?.timestamp ?? 1;
  const yFormat = (value: number) =>
    moneyMetric ? compact(value / 100) : compact(value);
  const pointValue = (point: TimelineRow) =>
    moneyMetric
      ? money(point[metric], metric === "mean_ltv_minor" ? 2 : 0)
      : number(point.users_count);
  return (
    <Panel
      title={title}
      description={`По месяцу первой подписки · ${moneyMetric ? '₽' : 'пользователи'}`}
      className="timeline-panel"
      actions={
        <MetricTabs
          label="Показатель графика"
          options={
            [
              ["total_revenue_minor", "Выручка"],
              ["mean_ltv_minor", "LTV"],
              ["users_count", "Пользователи"],
            ] as const
          }
          value={metric}
          onChange={onMetricChange}
        />
      }
    >
      {!points.length ? (
        <EmptyState />
      ) : (
        <>
          <div
            className="timeline-plot mx-2 rounded-sm outline-offset-2 focus-visible:outline-2 focus-visible:outline-[#1d5fbf] sm:mx-3"
            data-testid="timeline-chart"
            tabIndex={0}
            role="group"
            aria-label="График когорт. Стрелки выбирают месяц, Enter открывает его пользователей."
            aria-describedby={descriptionId}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            onKeyDown={(event) =>
              setIndex(
                moveIndex(
                  event,
                  Math.min(index, points.length - 1),
                  points.length,
                  (next) => {
                    const row = points[next];
                    if (row) onMonthClick(row.month);
                  },
                ),
              )
            }
          >
            <ResponsiveContainer
              width="100%"
              height="100%"
              minWidth={0}
              onResize={(nextWidth) => setWidth(nextWidth)}
            >
              <LineChart
                data={points}
                margin={{ top: 12, right: 24, bottom: 4, left: 0 }}
                accessibilityLayer={false}
                onClick={(event) => {
                  if (event.activeTooltipIndex == null) return;
                  const row = points[Number(event.activeTooltipIndex)];
                  if (row) onMonthClick(row.month);
                }}
              >
                <CartesianGrid vertical={false} stroke="#edf0f0" />
                <XAxis
                  dataKey="timestamp"
                  type="number"
                  scale="time"
                  domain={
                    minimum === maximum
                      ? [minimum - 1296000000, maximum + 1296000000]
                      : [minimum, maximum]
                  }
                  ticks={ticks}
                  tickFormatter={(value) =>
                    monthName(new Date(Number(value)).toISOString().slice(0, 7))
                  }
                  tick={axisStyle}
                  axisLine={false}
                  tickLine={false}
                  minTickGap={20}
                  height={32}
                />
                <YAxis
                  domain={[0, "auto"]}
                  tickCount={5}
                  allowDecimals={moneyMetric}
                  tickFormatter={yFormat}
                  tick={axisStyle}
                  axisLine={false}
                  tickLine={false}
                  width={56}
                />
                <Tooltip
                  isAnimationActive={false}
                  cursor={{ stroke: "#bdc9cc", strokeWidth: 1 }}
                  content={({ active: visible, payload }) => {
                    const point = payload?.[0]?.payload as
                      | (typeof points)[number]
                      | undefined;
                    return visible && point ? (
                      <ChartTooltipBox>
                        <strong className="font-medium">
                          {monthName(point.month)}
                        </strong>
                        <p>{number(point.users_count)} пользователей</p>
                        <p>{pointValue(point)}</p>
                        <p className="mt-1 text-muted-foreground">
                          Нажмите, чтобы открыть выборку
                        </p>
                      </ChartTooltipBox>
                    ) : null;
                  }}
                />
                <Line
                  type="linear"
                  dataKey="value"
                  stroke="#1d5fbf"
                  strokeWidth={2.2}
                  dot={false}
                  activeDot={{ r: 4, strokeWidth: 2, fill: "white" }}
                  isAnimationActive={false}
                />
                {focused && active && (
                  <ReferenceDot
                    x={active.timestamp}
                    y={active.value}
                    r={4}
                    fill="white"
                    stroke="#1d5fbf"
                    strokeWidth={2}
                  />
                )}
              </LineChart>
            </ResponsiveContainer>
          </div>
          {focused && active && (
            <div
              className="mx-4 rounded-md bg-muted px-3 py-2 text-[13px] leading-5"
              aria-live="polite"
              data-testid="timeline-selection"
            >
              {monthName(active.month)}: {pointValue(active)} ·{" "}
              {number(active.users_count)} пользователей. Enter — открыть
              выборку.
            </div>
          )}
        </>
      )}
      <PanelNote>
        <span id={descriptionId}>
          Пожизненная выручка пользователей когорты, а не поступления за месяц.
          Нажмите на график для выборки.
        </span>
      </PanelNote>
    </Panel>
  );
}

export interface RetentionChartProps {
  reports: RetentionReport[];
  metric: RetentionMetric;
  onMetricChange: (metric: RetentionMetric) => void;
  maxPeriods?: number;
  compact?: boolean;
}

function periodEvidence(period: PeriodRow, metric: RetentionMetric) {
  return `${percent(period[metric])} · продлили ${number(period.renewed)} · ${metric === "rate" ? "дошли и завершили" : "зрелая исходная когорта"}: ${number(metric === "rate" ? period.eligible : period.cumulative_eligible)}`;
}

export function RetentionChart({
  reports,
  metric,
  onMetricChange,
  maxPeriods = 12,
  compact: compactView = false,
}: RetentionChartProps) {
  const [focused, setFocused] = useState(false);
  const [index, setIndex] = useState(0);
  const [width, setWidth] = useState(600);
  const count = compactView ? Math.min(maxPeriods, 8) : maxPeriods;
  const visible = reports.filter((report) =>
    report.periods.slice(0, count).some((period) => period[metric] != null),
  );
  const rows = Array.from({ length: count }, (_, i) => ({
    period: i + 1,
    ...Object.fromEntries(
      reports.map((report) => [
        report.plan_id,
        report.periods[i]?.[metric] ?? null,
      ]),
    ),
  }));
  const selectedPeriod = Math.min(index, count - 1) + 1;
  const tickStep = Math.max(1, Math.ceil((count - 1) / (width < 480 ? 3 : 6)));
  const ticks = Array.from({ length: count }, (_, i) => i + 1).filter(
    (_, i) => i % tickStep === 0,
  );
  return (
    <Panel
      title="Кривые продлений"
      description="Номер периода учитывает длительность тарифа"
      actions={
        <MetricTabs
          label="Доля продлений"
          options={
            [
              ["cumulative_rate", "Кумулятивная"],
              ["rate", "Условная"],
            ] as const
          }
          value={metric}
          onChange={onMetricChange}
        />
      }
    >
      <div
        className="mx-2 rounded-sm outline-offset-2 focus-visible:outline-2 focus-visible:outline-[#1d5fbf] sm:mx-3"
        data-testid="retention-chart"
        tabIndex={0}
        role="group"
        aria-label="График продлений. Стрелки выбирают период и показывают знаменатели."
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={(event) =>
          setIndex(moveIndex(event, Math.min(index, count - 1), count))
        }
      >
        <ResponsiveContainer
          width="100%"
          height={compactView ? 210 : 270}
          minWidth={0}
          onResize={(nextWidth) => setWidth(nextWidth)}
        >
          <LineChart
            data={rows}
            margin={{ top: 12, right: 20, bottom: 4, left: 0 }}
            accessibilityLayer={false}
          >
            <CartesianGrid vertical={false} stroke="#edf0f0" />
            <XAxis
              dataKey="period"
              type="number"
              domain={count === 1 ? [0.5, 1.5] : [1, count]}
              ticks={ticks}
              tick={axisStyle}
              axisLine={false}
              tickLine={false}
              height={30}
            />
            <YAxis
              domain={[0, 1]}
              ticks={[0, 0.25, 0.5, 0.75, 1]}
              tickFormatter={percent}
              tick={axisStyle}
              axisLine={false}
              tickLine={false}
              width={50}
            />
            <Tooltip
              isAnimationActive={false}
              cursor={{ stroke: "#bdc9cc", strokeWidth: 1 }}
              content={({ active, label }) => {
                const periodNumber = Number(label);
                return active && Number.isInteger(periodNumber) ? (
                  <ChartTooltipBox>
                    <strong className="font-medium">
                      Продление {periodNumber}
                    </strong>
                    {reports.map((report) => {
                      const period = report.periods[periodNumber - 1];
                      return period ? (
                        <p key={report.plan_id} className="mt-2">
                          <span className="font-medium">
                            {planName(report.plan_id)}
                          </span>
                          <br />
                          {periodEvidence(period, metric)}
                        </p>
                      ) : null;
                    })}
                  </ChartTooltipBox>
                ) : null;
              }}
            />
            {reports.map((report, i) => (
              <Line
                key={report.plan_id}
                type="linear"
                dataKey={report.plan_id}
                stroke={planColor(report.plan_id)}
                strokeDasharray={seriesDash(i)}
                strokeWidth={2.2}
                connectNulls={false}
                dot={false}
                activeDot={{ r: 4, fill: "white", strokeWidth: 2 }}
                isAnimationActive={false}
              />
            ))}
            {focused &&
              reports.map((report) => {
                const period = report.periods[selectedPeriod - 1];
                const value = period?.[metric];
                return value != null ? (
                  <ReferenceDot
                    key={report.plan_id}
                    x={selectedPeriod}
                    y={value}
                    r={4}
                    fill="white"
                    stroke={planColor(report.plan_id)}
                    strokeWidth={2}
                  />
                ) : null;
              })}
          </LineChart>
        </ResponsiveContainer>
      </div>
      {visible.length > 0 && (
        <div className="flex flex-wrap gap-x-4 gap-y-2 px-4 pt-1 text-[13px] text-muted-foreground sm:px-[18px]">
          {reports.map((report, i) =>
            visible.includes(report) ? (
              <span
                className="inline-flex items-center gap-1.5"
                key={report.plan_id}
              >
                <svg
                  width="18"
                  height="8"
                  viewBox="0 0 18 8"
                  aria-hidden="true"
                >
                  <line
                    x1="0"
                    x2="18"
                    y1="4"
                    y2="4"
                    stroke={planColor(report.plan_id)}
                    strokeWidth="2"
                    strokeDasharray={seriesDash(i)}
                  />
                </svg>
                {planName(report.plan_id)}
              </span>
            ) : null,
          )}
        </div>
      )}
      {focused && (
        <div
          className="mx-4 mt-2 rounded-md bg-muted px-3 py-2 text-[13px] leading-5"
          aria-live="polite"
          data-testid="retention-selection"
        >
          <p className="font-medium">Продление {selectedPeriod}</p>
          {reports.map((report) => {
            const period = report.periods[selectedPeriod - 1];
            return period ? (
              <p key={report.plan_id}>
                {planName(report.plan_id)}: {periodEvidence(period, metric)}
              </p>
            ) : null;
          })}
        </div>
      )}
      <PanelNote>
        {metric === "rate"
          ? "Условная доля: от дошедших до периода и завершивших его."
          : "Кумулятивная доля: от исходной когорты, достаточно зрелой для этого продления."}{" "}
        Незавершённые периоды не считаются оттоком.{" "}
        {visible.length
          ? "Знаменатели — в подсказке или при выборе периода клавиатурой."
          : "В выбранных периодах ещё нет наблюдений для расчёта."}
      </PanelNote>
    </Panel>
  );
}

export function RevenueRanking({
  plans,
  onPlanClick,
  onOpenAll,
}: {
  plans: PlanSummary[];
  onPlanClick: (planId: string) => void;
  onOpenAll?: () => void;
}) {
  const rows = [...plans].sort(
    (a, b) => b.total_revenue_minor - a.total_revenue_minor,
  );
  const total = rows.reduce((sum, row) => sum + row.total_revenue_minor, 0);
  const max = Math.max(...rows.map((row) => row.total_revenue_minor), 1);
  const best = [...rows].sort(
    (a, b) => Number(b.mean_ltv_minor) - Number(a.mean_ltv_minor),
  )[0];
  return (
    <Panel
      title="Выручка по тарифам"
      description="Тариф первой подписки · вся жизнь пользователя"
    >
      <div className="plan-rankings">
        {rows.map((row) => (
          <button
            type="button"
            key={row.plan_id}
            onClick={() => onPlanClick(row.plan_id)}
            className="plan-ranking"
            aria-label={`Открыть пользователей: ${planName(row.plan_id)}`}
          >
            <span className="plan-ranking-main"><span className="plan-ranking-name"><span className="plan-duration" title={`${row.period_months} месяцев`}>{row.period_months}м</span>{planName(row.plan_id)}</span><span>{money(row.total_revenue_minor)}</span></span>
            <span className="plan-ranking-meta"><span>{number(row.users_count)} пользователей</span><span>{percent(total ? row.total_revenue_minor / total : 0)} выручки</span></span>
            <span className="plan-ranking-track"><span style={{ width: `${Math.max(0, row.total_revenue_minor / max) * 100}%` }}/></span>
          </button>
        ))}
      </div>
      <div className="ranking-footer">
        {total && best ? (
          <>
            <span>Лучший LTV: <strong className="font-medium text-foreground">{planName(best.plan_id)}</strong></span>
          </>
        ) : (
          "В этом сегменте нет выручки."
        )}
      {onOpenAll && <Button variant="link" onClick={onOpenAll}>Сравнить тарифы</Button>}</div>
    </Panel>
  );
}

export function DistributionChart({
  rows,
  total,
}: {
  rows: DistributionRow[];
  total: number;
}) {
  const max = Math.max(...rows.map((row) => row.users_count), 1);
  return (
    <Panel
      title="Распределение LTV"
      description="Пользователи по диапазонам пожизненной выручки"
    >
      <div className="px-4 sm:px-[18px]">
        {rows.map((row) => (
          <div
            key={row.label}
            className="grid min-h-[52px] grid-cols-[1fr_auto] items-center gap-x-3 gap-y-1 border-b border-border/60 py-2 text-[13px] last:border-0"
          >
            <span>{row.label} ₽</span>
            <span className="text-right tabular-nums">
              {number(row.users_count)} чел.
            </span>
            <span className="h-[3px] bg-muted">
              <span
                className="block h-full bg-[#1d5fbf]"
                style={{ width: `${(row.users_count / max) * 100}%` }}
              />
            </span>
            <span className="text-right text-muted-foreground tabular-nums">
              {percent(total ? row.users_count / total : 0)}
            </span>
          </div>
        ))}
      </div>
      <PanelNote>Пользователи без оплат включены в нулевой диапазон.</PanelNote>
    </Panel>
  );
}

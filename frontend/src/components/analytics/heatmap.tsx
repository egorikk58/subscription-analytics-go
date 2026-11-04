import { useRef, useState, type KeyboardEvent } from "react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { monthName, number, percent } from "@/lib/format";
import type { CohortRow, PeriodRow, RetentionMetric } from "@/types";
import { EmptyState, Panel } from "./panel";
import { planName } from "./plans";

export interface CohortHeatmapProps {
  rows: CohortRow[];
  metric: RetentionMetric;
  onMetricChange?: (metric: RetentionMetric) => void;
  planId: string;
  compact?: boolean;
  onMonthClick: (month: string) => void;
  onOpenAll?: () => void;
}

function heatAppearance(value: number | null) {
  if (value == null) return { backgroundColor: "#f5f7f7", color: "#5d5f60" };
  const alpha = 0.06 + Math.max(0, Math.min(1, value)) * 0.94;
  const components = [29, 95, 191]
    .map((channel) => ((1 - alpha) * 255 + alpha * channel) / 255)
    .map((channel) =>
      channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
    );
  const luminance =
    (components[0] ?? 0) * 0.2126 +
    (components[1] ?? 0) * 0.7152 +
    (components[2] ?? 0) * 0.0722;
  return {
    backgroundColor: `rgba(29,95,191,${alpha})`,
    color: luminance < 0.179 ? "#fff" : "#000",
  };
}

function evidence(period: PeriodRow, metric: RetentionMetric) {
  return `${percent(period[metric])}. Продлили: ${number(period.renewed)}. ${metric === "rate" ? "Дошли и завершили" : "Зрелая исходная когорта"}: ${number(metric === "rate" ? period.eligible : period.cumulative_eligible)}.`;
}

export function CohortHeatmap({
  rows: allRows,
  metric,
  onMetricChange,
  planId,
  compact = false,
  onMonthClick,
  onOpenAll,
}: CohortHeatmapProps) {
  const rows = compact ? allRows.slice(-6) : allRows.slice(-24);
  const periods = compact ? 8 : 12;
  const [activeKey, setActiveKey] = useState("");
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const currentKey = rows.some((row) =>
    row.periods
      .slice(0, periods)
      .some((_, column) => `${row.month}:${column}` === activeKey),
  )
    ? activeKey
    : `${rows[0]?.month}:0`;
  const move = (
    event: KeyboardEvent<HTMLButtonElement>,
    rowIndex: number,
    column: number,
  ) => {
    let nextRow = rowIndex,
      nextColumn = column;
    if (event.key === "ArrowLeft") nextColumn = Math.max(0, column - 1);
    else if (event.key === "ArrowRight")
      nextColumn = Math.min(periods - 1, column + 1);
    else if (event.key === "ArrowUp") nextRow = Math.max(0, rowIndex - 1);
    else if (event.key === "ArrowDown")
      nextRow = Math.min(rows.length - 1, rowIndex + 1);
    else if (event.key === "Home") nextColumn = 0;
    else if (event.key === "End") nextColumn = periods - 1;
    else return;
    event.preventDefault();
    const row = rows[nextRow];
    if (!row) return;
    const key = `${row.month}:${nextColumn}`;
    setActiveKey(key);
    buttons.current.get(key)?.focus();
  };
  return (
    <Panel
      title="Матрица когорт"
      description={`${planName(planId === "all" ? "monthly" : planId)} · ${metric === "rate" ? "условное" : "кумулятивное"} удержание по периодам`}
      actions={
        compact && onOpenAll ? (
          <Button
            type="button"
            variant="link"
            size="sm"
            onClick={onOpenAll}
            className="h-auto px-0 text-[13px] text-foreground"
          >
            Все когорты ↗
          </Button>
        ) : onMetricChange ? (
          <div
            className="flex border-b border-border"
            role="group"
            aria-label="Доля матрицы когорт"
          >
            {(
              [
                ["cumulative_rate", "Кумулятивная"],
                ["rate", "Условная"],
              ] as const
            ).map(([key, label]) => (
              <Button
                key={key}
                type="button"
                variant="ghost"
                size="sm"
                aria-pressed={metric === key}
                onClick={() => onMetricChange(key)}
                className={`-mb-px min-h-10 rounded-none border-b-2 px-2 text-[13px] shadow-none sm:min-h-8 ${metric === key ? "border-[#1d5fbf] font-medium text-foreground" : "border-transparent font-normal text-muted-foreground"}`}
              >
                {label}
              </Button>
            ))}
          </div>
        ) : null
      }
    >
      {!rows.length ? (
        <EmptyState />
      ) : (
        <>
          <div
            className="overflow-x-auto overscroll-x-contain px-3"
            data-testid="cohort-matrix"
          >
            <table
              className="w-full table-fixed border-separate border-spacing-[3px] text-[13px]"
              style={{ minWidth: compact ? 490 : 720 }}
            >
              <caption className="sr-only">
                Продления по месяцу первой подписки. Стрелки перемещают фокус
                между ячейками, Enter открывает пользователей когорты.
              </caption>
              <thead>
                <tr>
                  <th
                    scope="col"
                    className="w-[100px] pb-2 text-left font-normal text-muted-foreground"
                  >
                    Когорта
                  </th>
                  <th
                    scope="col"
                    className="w-[52px] pb-2 pr-2 text-right font-normal text-muted-foreground"
                  >
                    Люди
                  </th>
                  {Array.from({ length: periods }, (_, i) => (
                    <th
                      key={i}
                      scope="col"
                      className="pb-2 text-center font-normal text-muted-foreground"
                    >
                      {i + 1}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((row, rowIndex) => (
                  <tr key={row.month}>
                    <th
                      scope="row"
                      className="sticky left-0 z-10 bg-background text-left font-normal"
                    >
                      <button
                        type="button"
                        className="min-h-8 whitespace-nowrap py-1 text-[13px] underline decoration-input underline-offset-4 focus-visible:outline-offset-2"
                        onClick={() => onMonthClick(row.month)}
                        data-cohort-month={row.month}
                      >
                        {monthName(row.month)}
                      </button>
                    </th>
                    <td className="pr-2 text-right text-muted-foreground tabular-nums">
                      {number(row.users_count)}
                    </td>
                    {row.periods.slice(0, periods).map((period, column) => {
                      const key = `${row.month}:${column}`;
                      const value = period[metric];
                      return (
                        <td
                          key={period.period}
                          className="h-8 rounded-[3px] p-0 text-center tabular-nums"
                          style={heatAppearance(value)}
                        >
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <button
                                type="button"
                                ref={(element) => {
                                  if (element)
                                    buttons.current.set(key, element);
                                  else buttons.current.delete(key);
                                }}
                                tabIndex={key === currentKey ? 0 : -1}
                                onFocus={() => setActiveKey(key)}
                                onKeyDown={(event) =>
                                  move(event, rowIndex, column)
                                }
                                onClick={() => onMonthClick(row.month)}
                                className="heat-cell h-full min-h-8 w-full rounded-[3px] p-0 text-[13px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#131416]"
                                aria-label={`${monthName(row.month)}, продление ${period.period}. ${evidence(period, metric)} Открыть когорту.`}
                                data-grid-cell={column}
                                data-cohort-month={row.month}
                              >
                                {value == null
                                  ? "—"
                                  : `${Math.round(value * 100)}%`}
                              </button>
                            </TooltipTrigger>
                            <TooltipContent
                              side="top"
                              className="max-w-[280px] border border-border bg-background px-3 py-2 text-[13px] leading-5 text-foreground shadow-md"
                            >
                              <p className="font-medium">
                                {monthName(row.month)} · продление{" "}
                                {period.period}
                              </p>
                              <p>
                                {value == null
                                  ? "Нет наблюдений для расчёта"
                                  : percent(value)}
                              </p>
                              <p>Продлили: {number(period.renewed)}</p>
                              <p>
                                {metric === "rate"
                                  ? "Дошли и завершили"
                                  : "Зрелая исходная когорта"}
                                :{" "}
                                {number(
                                  metric === "rate"
                                    ? period.eligible
                                    : period.cumulative_eligible,
                                )}
                              </p>
                            </TooltipContent>
                          </Tooltip>
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 pb-4 pt-3 text-[13px] text-muted-foreground sm:px-[18px]">
            <span>Нажмите на месяц или ячейку для выборки</span>
            <span className="inline-flex items-center gap-1 whitespace-nowrap">
              <span>0%</span>
              {[0, 0.2, 0.4, 0.6, 0.8, 1].map((value) => (
                <span
                  key={value}
                  className="h-2 w-3"
                  style={heatAppearance(value)}
                  aria-hidden="true"
                />
              ))}
              <span>100%</span>
            </span>
          </div>
        </>
      )}
    </Panel>
  );
}

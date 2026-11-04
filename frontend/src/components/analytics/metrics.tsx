import { money, number } from "@/lib/format";
import type { Summary } from "@/types";

export function MetricStrip({ summary }: { summary: Summary }) {
  const metrics = [
    [
      "Пожизненная выручка",
      money(summary.total_revenue_minor),
    ],
    ["Пользователи", number(summary.users_count)],
    [
      "Средний LTV",
      money(summary.mean_ltv_minor, 2),
    ],
    [
      "Медианный LTV",
      money(summary.median_ltv_minor, 2),
    ],
  ];
  return (
    <div
      className="metric-strip"
      aria-label="Показатели выбранного сегмента"
    >
      {metrics.map(([label, value], index) => (
        <article
          key={label}
          className="metric-cell"
        >
          <h2 className="text-[13px] font-normal text-muted-foreground">
            {label}
          </h2>
          <p
            className="kpi-value"
            data-testid={`metric-${index}`}
          >
            {value}
          </p>
        </article>
      ))}
    </div>
  );
}

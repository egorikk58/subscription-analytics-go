import { money, number } from "@/lib/format";
import type { Summary } from "@/types";

export function MetricStrip({ summary }: { summary: Summary }) {
  const metrics = [
    [
      "Пожизненная выручка",
      money(summary.total_revenue_minor),
      "Успешные оплаты − возвраты",
    ],
    ["Пользователи", number(summary.users_count), "В выбранном сегменте"],
    [
      "Средний LTV",
      money(summary.mean_ltv_minor, 2),
      "Включая пользователей без оплат",
    ],
    [
      "Медианный LTV",
      money(summary.median_ltv_minor, 2),
      "Середина распределения",
    ],
  ];
  return (
    <div
      className="metric-strip"
      aria-label="Показатели выбранного сегмента"
    >
      {metrics.map(([label, value, description], index) => (
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
          <p className="text-[13px] leading-5 text-muted-foreground">
            {description}
          </p>
        </article>
      ))}
    </div>
  );
}

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
      className="grid grid-cols-2 border-y border-border min-[801px]:grid-cols-[1.2fr_.8fr_1fr_1fr]"
      aria-label="Показатели выбранного сегмента"
    >
      {metrics.map(([label, value, description], index) => (
        <article
          key={label}
          className={`min-w-0 px-3 py-4 first:pl-0 last:pr-0 sm:px-5 min-[801px]:my-4 min-[801px]:py-0 ${index % 2 ? "border-l border-border" : ""} ${index > 1 ? "border-t border-border min-[801px]:border-t-0" : ""} ${index === 2 ? "pl-0 min-[801px]:border-l min-[801px]:pl-5" : ""}`}
        >
          <h2 className="text-[13px] font-normal text-muted-foreground">
            {label}
          </h2>
          <p
            className={`kpi-value my-2 whitespace-nowrap text-[clamp(21px,5.6vw,28px)] font-normal leading-tight tracking-[-.025em] tabular-nums ${index === 0 ? "min-[1101px]:text-[32px]" : "min-[1101px]:text-[27px]"}`}
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

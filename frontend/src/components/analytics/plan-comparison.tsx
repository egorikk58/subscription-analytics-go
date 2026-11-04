import { ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/native-select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { money, number, percent } from "@/lib/format";
import type { PlanSummary, RetentionReport } from "@/types";
import { Panel, PlanLabel } from "./panel";
import { planColor, planName } from "./plans";

export function PlanComparison({
  plans,
  onPlanClick,
}: {
  plans: PlanSummary[];
  onPlanClick: (planId: string) => void;
}) {
  const max = Math.max(...plans.map((plan) => Number(plan.mean_ltv_minor)), 1);
  return (
    <Panel
      title="Сравнение тарифов"
    >
      <Table
        className="min-w-[650px] text-[13px]"
        data-testid="plan-comparison"
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="px-4 font-normal">Первый тариф</TableHead>
            <TableHead className="px-4 text-right font-normal">
              Пользователи
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Средний LTV
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Медиана
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Выручка
            </TableHead>
            <TableHead className="w-10">
              <span className="sr-only">Действие</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {plans.map((plan) => (
            <TableRow key={plan.plan_id} className="hover:bg-muted/30">
              <TableCell className="px-4 py-3">
                <PlanLabel planId={plan.plan_id} />
              </TableCell>
              <TableCell className="px-4 text-right tabular-nums">
                {number(plan.users_count)}
              </TableCell>
              <TableCell className="px-4 text-right tabular-nums">
                <span className="font-medium">
                  {money(plan.mean_ltv_minor, 2)}
                </span>
                <span className="ml-auto mt-1 block h-[3px] max-w-[100px] bg-muted">
                  <span
                    className="block h-full"
                    style={{
                      width: `${(Number(plan.mean_ltv_minor) / max) * 100}%`,
                      background: planColor(plan.plan_id),
                    }}
                  />
                </span>
              </TableCell>
              <TableCell className="px-4 text-right tabular-nums">
                {money(plan.median_ltv_minor, 2)}
              </TableCell>
              <TableCell className="px-4 text-right font-medium tabular-nums">
                {money(plan.total_revenue_minor)}
              </TableCell>
              <TableCell className="pr-3">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => onPlanClick(plan.plan_id)}
                  aria-label={`Открыть пользователей: ${planName(plan.plan_id)}`}
                >
                  <ArrowUpRight className="size-4" />
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Panel>
  );
}

export function RetentionTable({
  reports,
  maxPeriods,
  onMaxPeriodsChange,
}: {
  reports: RetentionReport[];
  maxPeriods: number;
  onMaxPeriodsChange: (periods: number) => void;
}) {
  return (
    <Panel
      title="Продления по периодам"
      actions={
        <label
          className="flex items-center gap-2 text-[13px] text-muted-foreground"
          htmlFor="period-count"
        >
          Периодов
          <NativeSelect
            id="period-count"
            aria-label="Число периодов"
            value={maxPeriods}
            onChange={(event) => onMaxPeriodsChange(Number(event.target.value))}
            className="min-h-9 w-20 text-[13px]"
          >
            {[3, 6, 12, 24].map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </NativeSelect>
        </label>
      }
    >
      <Table
        className="min-w-[760px] text-[13px]"
        data-testid="retention-evidence"
      >
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="px-4 font-normal">Тариф / период</TableHead>
            <TableHead className="px-4 text-right font-normal">
              Дошли и завершили
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Продлили
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Зрелая когорта
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Условная
            </TableHead>
            <TableHead className="px-4 text-right font-normal">
              Кумулятивная
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {reports.flatMap((report) =>
            report.periods.slice(0, maxPeriods).map((period) => (
              <TableRow
                key={`${report.plan_id}-${period.period}`}
                className="hover:bg-muted/30"
              >
                <TableCell className="px-4 py-3">
                  <div className="flex items-center justify-between gap-4">
                    <PlanLabel planId={report.plan_id} />
                    <span className="text-xs text-muted-foreground">
                      Продление {period.period}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="px-4 text-right tabular-nums">
                  {number(period.eligible)}
                </TableCell>
                <TableCell className="px-4 text-right font-medium tabular-nums">
                  {number(period.renewed)}
                </TableCell>
                <TableCell className="px-4 text-right tabular-nums">
                  {number(period.cumulative_eligible)}
                </TableCell>
                <TableCell className="px-4 text-right tabular-nums">
                  {percent(period.rate)}
                </TableCell>
                <TableCell className="px-4 text-right font-medium tabular-nums">
                  {percent(period.cumulative_rate)}
                </TableCell>
              </TableRow>
            )),
          )}
        </TableBody>
      </Table>
    </Panel>
  );
}

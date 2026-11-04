import { useState, type ReactNode } from "react";
import { Check, Database } from "lucide-react";
import { Panel } from "@/components/analytics/panel";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { assetMoney, date, number, planColors, planNames } from "@/lib/format";
import type { DatasetInfo, DatasetSamples as DatasetSamplesType } from "@/types";

export function PlanName({ planId }: { planId: string }) {
  const key = planId as keyof typeof planNames;
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <span
        aria-hidden="true"
        className="size-1.5 shrink-0 rounded-full"
        style={{ backgroundColor: planColors[key] || "#9aa5a8" }}
      />
      {planNames[key] || "Без подписки"}
    </span>
  );
}

export function DatasetMetrics({ dataset }: { dataset: DatasetInfo }) {
  const metrics = [
    ["Пользователи", dataset.users_count],
    ["Подписки", dataset.subscriptions_count],
    ["Платёжные записи", dataset.payments_count],
    ["Seed", dataset.seed],
  ] as const;
  return (
    <section
      aria-label="Состав набора"
      className="mb-6 grid grid-cols-2 border-y border-border py-0 lg:grid-cols-4 lg:py-4"
    >
      {metrics.map(([label, value], index) => (
        <div
          key={label}
          className={`min-w-0 px-3 py-4 ${index % 2 ? "border-l border-border" : ""} ${index > 1 ? "border-t border-border lg:border-t-0" : ""} ${index > 0 ? "lg:border-l lg:border-border lg:px-5" : "lg:pl-0"}`}
        >
          <p className="text-[13px] text-muted-foreground">{label}</p>
          <p className="mt-2 text-[27px] leading-tight font-normal tabular-nums tracking-tight">
            {value == null ? "—" : number(value)}
          </p>
        </div>
      ))}
    </section>
  );
}

export function DatasetPassport({
  dataset,
  isPreview,
}: {
  dataset: DatasetInfo;
  isPreview: boolean;
}) {
  const metadata: [string, ReactNode][] = [
    ["Seed", dataset.seed ?? "—"],
    ["История", `${dataset.history_months || "—"} мес.`],
    ["Дата среза", dataset.dataset_as_of ? date(dataset.dataset_as_of) : "—"],
    ["Генератор", dataset.generator_version || "—"],
    [
      "Fingerprint",
      <span
        key="fingerprint"
        className="font-mono text-xs break-all"
        title={dataset.fingerprint}
      >
        {dataset.fingerprint.slice(0, 16)}…
      </span>,
    ],
    ["Режим", isPreview ? "Предпросмотр" : "Снимок"],
    ["Валюты оплат", Object.entries(dataset.currencies ?? { RUB: dataset.payments_count }).map(([currency, count]) => `${currency}: ${number(count)}`).join(' · ')],
    ["Валюта расчёта", "RUB"],
    ["Источник курсов", "Генератор"],
  ];
  const quality = [
    ["Успешные оплаты", dataset.succeeded_count],
    ["Неудачные попытки", dataset.failed_count],
    ["Полные возвраты", dataset.refunded_count],
    ["Сменили тариф", dataset.switches_count],
    ["Без оплат", dataset.unpaid_users],
    ["Незавершённые периоды", dataset.active_periods],
  ] as const;
  return (
    <Panel
      title="Параметры набора"
    >
      <dl className="grid grid-cols-2 gap-x-4 gap-y-5 px-4 pb-5 sm:px-5">
        {metadata.map(([label, value]) => (
          <div key={label}>
            <dt className="text-[13px] text-muted-foreground">{label}</dt>
            <dd className="mt-1 text-[13px] font-medium">{value}</dd>
          </div>
        ))}
      </dl>
      <ul className="grid gap-x-3 gap-y-2.5 border-t border-border px-4 py-4 text-[13px] sm:grid-cols-2 sm:px-5">
        {quality.map(([label, value]) => (
          <li key={label} className="flex items-baseline gap-1.5">
            <Check
              className="relative top-0.5 size-3.5 shrink-0 text-emerald-700"
              aria-hidden="true"
            />
            <span>
              {label}:{" "}
              <strong className="font-medium tabular-nums">
                {number(value)}
              </strong>
            </span>
          </li>
        ))}
      </ul>

    </Panel>
  );
}

type SampleKind = keyof DatasetSamplesType;
interface SampleRow {
  id: string;
  second: ReactNode;
  third: ReactNode;
  fourth: ReactNode;
  record: unknown;
}
function StatusBadge({ status }: { status: string }) {
  const positive = status === "succeeded" || status === "active";
  const caution = status === "failed" || status === "refunded";
  return (
    <Badge
      variant="secondary"
      className={
        positive
          ? "rounded-sm border-0 bg-emerald-50 font-normal text-emerald-800"
          : caution
            ? "rounded-sm border-0 bg-amber-50 font-normal text-amber-900"
            : "rounded-sm border-0 font-normal"
      }
    >
      {status}
    </Badge>
  );
}
const shortId = (value: string) => (
  <span className="font-mono text-xs" title={value}>
    {value.slice(0, 8)}…
  </span>
);

export interface DatasetSamplesProps {
  dataset: DatasetInfo;
  sampleKind?: SampleKind;
  onSampleKindChange?: (kind: SampleKind) => void;
}

export function DatasetSamples({ dataset, sampleKind, onSampleKindChange }: DatasetSamplesProps) {
  const [localKind, setLocalKind] = useState<SampleKind>("payments");
  const kind = sampleKind ?? localKind;
  const [selectedId, setSelectedId] = useState("");
  const samples = dataset.samples;
  const rows: SampleRow[] =
    kind === "payments"
      ? samples.payments.map((record) => ({
          id: record.id,
          second: <StatusBadge status={record.status} />,
          third: <span>{assetMoney(record.amount, record.currency)}{record.fx && <span className="mt-1 block text-xs text-muted-foreground">1 {record.currency} = {record.fx.rub_per_unit} RUB</span>}</span>,
          fourth: shortId(record.user_id),
          record,
        }))
      : kind === "subscriptions"
        ? samples.subscriptions.map((record) => ({
            id: record.id,
            second: <PlanName planId={record.plan_id} />,
            third: <StatusBadge status={record.status} />,
            fourth: shortId(record.user_id),
            record,
          }))
        : samples.users.map((record) => ({
            id: record.id,
            second: String(record.telegram_id),
            third: date(record.created_at),
            fourth: record.utm_link || "—",
            record,
          }));
  const active = rows.find((row) => row.id === selectedId) || rows[0];
  const headings =
    kind === "payments"
      ? ["ID платежа", "Статус", "Сумма", "Пользователь"]
      : kind === "subscriptions"
        ? ["ID подписки", "Тариф", "Статус", "Пользователь"]
        : ["ID пользователя", "Telegram ID", "Регистрация", "UTM"];
  return (
    <Panel
      title="Записи"
      actions={
        <Badge variant="secondary" className="rounded-sm font-normal">
          JSON
        </Badge>
      }
    >
      <Tabs
        value={kind}
        onValueChange={(value) => {
          if (
            value === "users" ||
            value === "subscriptions" ||
            value === "payments"
          ) {
            setLocalKind(value);
            onSampleKindChange?.(value);
            setSelectedId("");
          }
        }}
      >
        <TabsList
          className="mx-4 mb-3 h-10 bg-muted/50 sm:mx-5"
          aria-label="Тип записей"
        >
          <TabsTrigger value="users" data-sample="users">
            Пользователи
          </TabsTrigger>
          <TabsTrigger value="subscriptions" data-sample="subscriptions">
            Подписки
          </TabsTrigger>
          <TabsTrigger value="payments" data-sample="payments">
            Платежи
          </TabsTrigger>
        </TabsList>
        <TabsContent value={kind} className="mt-0">
          <Table>
            <TableHeader>
              <TableRow>
                {headings.map((label, index) => (
                  <TableHead
                    key={label}
                    className={`px-4 text-[13px] font-normal sm:px-5 ${kind === "payments" && index === 2 ? "text-right" : ""}`}
                  >
                    {label}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row, index) => (
                <TableRow
                  key={row.id}
                  data-state={active?.id === row.id ? "selected" : undefined}
                  className="sample-row cursor-pointer data-[state=selected]:bg-primary/5"
                  data-sample-index={index}
                  onClick={() => setSelectedId(row.id)}
                >
                  <TableCell className="px-4 sm:px-5">
                    <Button
                      variant="ghost"
                      className="h-auto px-0 py-1 font-mono text-xs hover:bg-transparent"
                      onClick={() => setSelectedId(row.id)}
                      aria-label={`Показать JSON ${row.id}`}
                    >
                      {shortId(row.id)}
                    </Button>
                  </TableCell>
                  <TableCell className="px-4 text-[13px] sm:px-5">
                    {row.second}
                  </TableCell>
                  <TableCell
                    className={`px-4 text-[13px] tabular-nums sm:px-5 ${kind === "payments" ? "text-right" : ""}`}
                  >
                    {row.third}
                  </TableCell>
                  <TableCell className="px-4 text-[13px] sm:px-5">
                    {row.fourth}
                  </TableCell>
                </TableRow>
              ))}
              {!rows.length && (
                <TableRow>
                  <TableCell
                    colSpan={4}
                    className="h-24 text-center text-muted-foreground"
                  >
                    В наборе нет записей этого типа.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
          <div className="border-t border-border bg-muted/40 p-4 sm:p-5">
            <div className="mb-3 flex items-center gap-2 text-[13px] text-muted-foreground">
              <Database className="size-3.5" aria-hidden="true" />
              Исходная запись JSON
            </div>
            <pre className="sample-json max-h-[460px] overflow-auto whitespace-pre-wrap font-mono text-xs leading-relaxed break-all">
              {active ? JSON.stringify(active.record, null, 2) : "Нет записей"}
            </pre>
          </div>
        </TabsContent>
      </Tabs>
    </Panel>
  );
}

import { OperationsLinks } from '@/components/operations-links';
import { ApiInspector } from '@/components/api-inspector';
import { ArrowRight, CreditCard, Layers, Users } from "lucide-react";
import { Panel } from "@/components/analytics/panel";
import { DatasetMetrics, DatasetSamples } from "@/components/dataset-samples";
import { Badge } from "@/components/ui/badge";
import { date, number, time } from "@/lib/format";
import type { Dashboard, ServiceStatus, DatasetSamples as DatasetSamplesType } from "@/types";

export interface SourcesPageProps {
  report: Dashboard;
  status: ServiceStatus | undefined;
  isPreview: boolean;
  sampleKind?: keyof DatasetSamplesType;
  onSampleKindChange?: (kind: keyof DatasetSamplesType) => void;
}

export function SourcesPage({ report, status, isPreview, sampleKind, onSampleKindChange }: SourcesPageProps) {
  const dataset = report.dataset;
  const sources = [
    {
      title: "Пользователи",
      route: "GET /users",
      count: dataset.users_count,
      Icon: Users,
    },
    {
      title: "Подписки и тарифы",
      route: "GET /plans · GET /subscriptions",
      count: dataset.subscriptions_count,
      Icon: Layers,
    },
    {
      title: "Платежи",
      route: "GET /payments",
      count: dataset.payments_count,
      Icon: CreditCard,
    },
  ];
  const metadata = [
    ["Версия", status?.snapshot_id ? `#${status.snapshot_id}` : "Ещё нет"],
    ["Данные на", status?.data_as_of ? date(status.data_as_of) : "—"],
    ["Последний успех", time(status?.last_success_at)],
    ["Следующий опрос", time(status?.next_sync_at)],
    [
      "Состояние",
      status?.sync_in_progress
        ? "Синхронизация"
        : status
          ? "Ожидание"
          : "Статус недоступен",
    ],
    [
      "Ошибка",
      status?.last_error
        ? "Есть ошибка источника"
        : status
          ? "Нет"
          : "Неизвестно",
    ],
  ];
  return (
    <div className="space-y-6">
      <DatasetMetrics dataset={dataset} />
      <div className="grid items-start gap-6 xl:grid-cols-2">
        <Panel
          title="Источники набора"
          description={
            isPreview
              ? "Предпросмотр получен генератором, источники рабочего сервиса не изменены"
              : "Данные собраны фоновым опросом"
          }
        >
          {sources.map(({ title, route, count, Icon }) => (
            <div
              key={title}
              className="source-card flex flex-wrap items-center gap-3 border-t border-border px-4 py-4 sm:flex-nowrap sm:px-5"
            >
              <Icon
                className="size-4 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              <div className="min-w-0">
                <h3 className="text-sm font-medium">{title}</h3>
                <p className="mt-1 text-xs text-muted-foreground">{route}</p>
              </div>
              <Badge
                variant="secondary"
                className="ml-7 rounded-sm font-normal tabular-nums sm:ml-auto"
              >
                {number(count)} записей
              </Badge>
            </div>
          ))}
          <ol
            aria-label="Путь данных"
            className="flex flex-wrap items-center gap-x-2 gap-y-3 border-t border-border px-4 py-4 text-xs text-muted-foreground sm:px-5"
          >
            {[
              "HTTP-источники",
              "Проверка и расчёт",
              "Снимок PostgreSQL",
              "Redis / API",
            ].map((step, index) => (
              <li key={step} className="flex items-center gap-2">
                {index > 0 && (
                  <ArrowRight className="size-3" aria-hidden="true" />
                )}
                {step}
              </li>
            ))}
          </ol>
        </Panel>
        <Panel
          title="Опубликованный снимок"
          description={
            isPreview
              ? "Дашборды показывают preview; статус относится к рабочему сервису"
              : "Одна согласованная версия во всех отчётах"
          }
        >
          <dl className="grid grid-cols-2 gap-x-4 gap-y-5 px-4 pb-5 sm:px-5">
            {metadata.map(([label, value]) => (
              <div key={label}>
                <dt className="text-[13px] text-muted-foreground">{label}</dt>
                <dd className="mt-1 text-[13px] font-medium">{value}</dd>
              </div>
            ))}
          </dl>
          <p className="mx-4 mb-4 rounded-md bg-muted/60 p-3 text-[13px] leading-relaxed text-muted-foreground sm:mx-5">
            При сбое источника последний успешный снимок сохраняется. Отказ
            Redis покрывается готовыми ответами PostgreSQL.
          </p>
          {status?.last_error && (
            <p
              className="mx-4 mb-4 rounded-md bg-destructive/5 p-3 text-[13px] break-words text-destructive sm:mx-5"
              role="alert"
            >
              {status.last_error}
            </p>
          )}
        </Panel>
      </div>
      <DatasetSamples dataset={dataset} sampleKind={sampleKind} onSampleKindChange={onSampleKindChange} />
    <OperationsLinks /><ApiInspector /></div>
  );
}

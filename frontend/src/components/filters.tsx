import { CalendarDays, ListFilter } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { number, planNames, time } from '@/lib/format'
import type { Dashboard, Filters, PlanFilter, View } from '@/types'

interface FilterBarProps { appliedFilters?: Filters; filters: Filters; onChange: (filters: Filters) => void; onReset: () => void; report?: Dashboard; view: View; fetching: boolean }
export function FilterBar({ appliedFilters, filters, onChange, onReset, report, view, fetching }: FilterBarProps) {
  const shown = appliedFilters ?? filters
  const pending = shown.from !== filters.from || shown.to !== filters.to || shown.plan_id !== filters.plan_id || shown.selection !== filters.selection || shown.utm_link !== filters.utm_link
  const retention = view === 'retention' || view === 'cohorts'
  const plan = shown.plan_id === 'all' && view === 'cohorts' ? 'monthly' : shown.plan_id
  const cohort = retention && plan !== 'all' ? report?.retention.find(row => row.plan_id === plan) : undefined
  const count = cohort?.cohort_size ?? report?.summary.users_count
  const description = retention ? cohort ? `Цепочки: ${planNames[plan]} · включая сменивших тариф` : 'Отбор по первой подписке · каждая линия — своя тарифная когорта' : shown.selection === 'chain' ? 'По цепочке тарифа · месяц первой подписки' : 'По дате первой подписки · выручка за всё время'
  const change = (patch: Partial<Filters>) => onChange({ ...filters, ...patch, selection: 'first_plan' })
  return <section aria-label="Фильтры аналитики" className="mb-6">
    <div className="flex flex-wrap items-center gap-2.5 sm:gap-3">
      <div className="flex w-full gap-0.5 rounded-md bg-muted p-0.5 sm:w-auto">{['all', '2025', '2026'].map(preset => {
        const active = preset === 'all' ? !filters.from && !filters.to : filters.from === `${preset}-01-01` && filters.to === `${preset}-12-31`
        return <Button key={preset} data-preset={preset} variant="ghost" aria-pressed={active} onClick={() => change({ from: preset === 'all' ? '' : `${preset}-01-01`, to: preset === 'all' ? '' : `${preset}-12-31` })} className={`min-h-10 flex-1 border px-3 text-[13px] font-normal shadow-none sm:min-h-8 ${active ? 'border-border bg-card hover:bg-card' : 'border-transparent text-muted-foreground'}`}>{preset === 'all' ? 'Вся история' : preset}</Button>
      })}</div>
      <div className="flex min-h-11 w-full items-center gap-1.5 rounded-md border border-input px-2 sm:min-h-9 sm:w-auto"><CalendarDays className="size-4 shrink-0 text-muted-foreground" aria-hidden="true"/><Input id="filter-from" type="date" aria-label="Первая подписка с даты" value={filters.from} onChange={event => change({ from: event.target.value })} className="h-9 min-w-0 flex-1 border-0 px-1 text-[13px] shadow-none sm:w-[121px]"/><span className="text-muted-foreground">—</span><Input id="filter-to" type="date" aria-label="Первая подписка по дату включительно" value={filters.to} onChange={event => change({ to: event.target.value })} className="h-9 min-w-0 flex-1 border-0 px-1 text-[13px] shadow-none sm:w-[121px]"/></div>
      <div className="flex min-h-11 min-w-0 flex-1 items-center gap-2 rounded-md border border-input px-2 sm:min-h-9 sm:flex-none"><ListFilter className="size-4 shrink-0 text-muted-foreground" aria-hidden="true"/><NativeSelect id="filter-plan" aria-label={retention ? 'Тариф продления' : 'Первый тариф'} value={filters.plan_id} onChange={event => change({ plan_id: event.target.value as PlanFilter })} className="min-h-10 w-full border-0 text-[13px] shadow-none sm:min-h-8 sm:w-[134px]"><NativeSelectOption value="all">Все тарифы</NativeSelectOption>{Object.entries(planNames).map(([id, label]) => <NativeSelectOption key={id} value={id}>{label}</NativeSelectOption>)}</NativeSelect></div>
      <NativeSelect aria-label="Источник трафика UTM" value={filters.utm_link ?? ''} onChange={event => change({ utm_link: event.target.value })} className="min-h-11 text-[13px] sm:min-h-9" wrapperClassName="max-w-full sm:max-w-56"><NativeSelectOption value="">Все источники</NativeSelectOption>{(report?.utm_options ?? []).map(utm => <NativeSelectOption key={utm} value={utm}>{utm === 'unknown' ? 'Без UTM' : utm}</NativeSelectOption>)}</NativeSelect>
      <Button id="reset-filters" variant="link" onClick={onReset} className="min-h-11 px-2 text-[13px] sm:min-h-9">Сбросить</Button>
    </div>
    {report && <p className={`mt-3 text-[13px] ${pending ? 'text-amber-800' : 'text-muted-foreground'}`} role="status">Показаны данные: {shown.from || 'начало истории'} — {shown.to || 'вся история'}, {planNames[shown.plan_id] || 'все тарифы'}, {shown.utm_link === 'unknown' ? 'без UTM' : shown.utm_link || 'все источники'}.{pending && (fetching ? ' Загружается новая выборка.' : ' Новые фильтры не применились.')}</p>}
    <div className="mt-3 flex flex-col gap-1 text-[13px] text-muted-foreground sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-4" aria-live="polite"><span><strong id="selection-count" className="font-medium text-foreground">{count == null ? '—' : number(count)}</strong> пользователей в выборке</span><span>{description}</span><span className="sm:ml-auto">{fetching ? 'Обновляем выборку…' : report ? `Снимок от ${time(report.data_as_of)}` : 'Ожидание данных'}</span></div>
  </section>
}

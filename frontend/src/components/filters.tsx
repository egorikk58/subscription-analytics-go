import { useState } from 'react'
import { CalendarDays, ChevronDown, Link2, ListFilter, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { number, planNames } from '@/lib/format'
import type { Dashboard, Filters, PlanFilter, View } from '@/types'

interface FilterBarProps { appliedFilters?: Filters; filters: Filters; onChange: (filters: Filters) => void; onReset: () => void; onShare?: () => void; report?: Dashboard; view: View; fetching: boolean }
export function FilterBar({ appliedFilters, filters, onChange, onReset, onShare, report, view, fetching }: FilterBarProps) {
  const [expanded, setExpanded] = useState(false)
  const shown = appliedFilters ?? filters
  const pending = shown.from !== filters.from || shown.to !== filters.to || shown.plan_id !== filters.plan_id || shown.selection !== filters.selection || shown.utm_link !== filters.utm_link
  const retention = view === 'retention' || view === 'cohorts'
  const plan = shown.plan_id === 'all' && view === 'cohorts' ? 'monthly' : shown.plan_id
  const cohort = retention && plan !== 'all' ? report?.retention.find(row => row.plan_id === plan) : undefined
  const count = cohort?.cohort_size ?? report?.summary.users_count
  const activeCount = Number(Boolean(filters.from || filters.to)) + Number(filters.plan_id !== 'all') + Number(Boolean(filters.utm_link))
  const change = (patch: Partial<Filters>) => onChange({ ...filters, ...patch, selection: 'first_plan' })
  return <section aria-label="Фильтры аналитики" className="filter-bar">
    <button className="mobile-filter-toggle" aria-expanded={expanded} aria-controls="filter-fields" onClick={() => setExpanded(open => !open)}><ListFilter size={16}/><span>Фильтры{activeCount ? ` · ${activeCount}` : ''}</span><span>{filters.plan_id === 'all' ? 'Все тарифы' : planNames[filters.plan_id]}</span><ChevronDown size={15}/></button>
    <div id="filter-fields" className={`filter-fields ${expanded ? 'is-expanded' : ''}`}>
      <div className="filter-presets">{['all', '2025', '2026'].map(preset => {
        const active = preset === 'all' ? !filters.from && !filters.to : filters.from === `${preset}-01-01` && filters.to === `${preset}-12-31`
        return <Button key={preset} data-preset={preset} variant="ghost" aria-pressed={active} onClick={() => change({ from: preset === 'all' ? '' : `${preset}-01-01`, to: preset === 'all' ? '' : `${preset}-12-31` })}>{preset === 'all' ? 'Вся история' : preset}</Button>
      })}</div>
      <div className="filter-dates"><CalendarDays size={14} aria-hidden="true"/><Input id="filter-from" type="date" aria-label="Первая подписка с даты" value={filters.from} onChange={event => change({ from: event.target.value })}/><span>—</span><Input id="filter-to" type="date" aria-label="Первая подписка по дату включительно" value={filters.to} onChange={event => change({ to: event.target.value })}/></div>
      <NativeSelect id="filter-plan" aria-label={retention ? 'Тариф продления' : 'Первый тариф'} value={filters.plan_id} onChange={event => change({ plan_id: event.target.value as PlanFilter })} wrapperClassName="filter-select"><NativeSelectOption value="all">Все тарифы</NativeSelectOption>{Object.entries(planNames).map(([id, label]) => <NativeSelectOption key={id} value={id}>{label}</NativeSelectOption>)}</NativeSelect>
      <NativeSelect aria-label="Источник трафика UTM" value={filters.utm_link ?? ''} onChange={event => change({ utm_link: event.target.value })} wrapperClassName="filter-select filter-utm"><NativeSelectOption value="">Все источники</NativeSelectOption>{(report?.utm_options ?? []).map(utm => <NativeSelectOption key={utm} value={utm}>{utm === 'unknown' ? 'Без UTM' : utm}</NativeSelectOption>)}</NativeSelect>
      <Button id="reset-filters" variant="ghost" title="Сбросить фильтры" aria-label="Сбросить фильтры" onClick={onReset}><RotateCcw size={13}/><span>Сбросить</span></Button>
      {onShare && <Button className="filter-share" variant="ghost" size="icon" title={report?.mode === 'preview' ? 'Ссылка доступна для рабочего снимка' : 'Скопировать ссылку на выборку'} aria-label="Скопировать ссылку на выборку" disabled={fetching || report?.mode !== 'live'} onClick={onShare}><Link2 size={15}/></Button>}
    </div>
    <div className="filter-scope" aria-live="polite"><span><strong id="selection-count">{count == null ? '—' : number(count)}</strong> пользователей{shown.selection === 'chain' ? ' в цепочке тарифа' : ' в выборке'}</span><span className="scope-explanation">Первая подписка · выручка за всё время</span>{report && <p className={pending ? 'scope-pending' : activeCount ? 'scope-description' : 'sr-only'} role="status">Показаны данные: {shown.from || 'начало истории'} — {shown.to || 'вся история'}, {planNames[shown.plan_id] || 'все тарифы'}{shown.utm_link ? `, ${shown.utm_link === 'unknown' ? 'без UTM' : shown.utm_link}` : ''}.{pending && (fetching ? ' Загружается новая выборка.' : ' Новые фильтры не применились.')}</p>}</div>
  </section>
}

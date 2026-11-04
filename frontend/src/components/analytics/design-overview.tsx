import { useState } from 'react'
import { ArrowRight, ArrowUpRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { NativeSelect } from '@/components/ui/native-select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { money, number, percent } from '@/lib/format'
import { heatAppearance } from '@/lib/visualization'
import type { Dashboard, PlanFilter, RetentionMetric } from '@/types'
import { Panel } from './panel'
import { planColor, planName } from './plans'
import { RetentionChart } from './charts'

interface DesignOverviewProps {
  report: Dashboard; plan: PlanFilter; metric: RetentionMetric
  onMetricChange: (metric: RetentionMetric) => void
  periods: number; onPeriodsChange: (value: number) => void
  onPlanClick: (id: string) => void; onOpenRetention: () => void
}
function LifetimeValue({ report }: { report: Dashboard }) {
  const summary = report.summary
  return <section className="lifetime-feature" aria-labelledby="lifetime-title"><h2 id="lifetime-title">Средний LTV пользователя</h2><p className="lifetime-value" data-testid="hero-ltv" aria-label={`Средний LTV ${money(summary.mean_ltv_minor, 2)}`}><span>{money(summary.mean_ltv_minor, 2).replace(/\s*₽$/, '')}</span><span className="lifetime-unit">₽</span></p><dl className="lifetime-details"><div><dt>Медиана</dt><dd>{money(summary.median_ltv_minor, 2)}</dd></div><div><dt>Выручка</dt><dd>{money(summary.total_revenue_minor)}</dd></div><div><dt>Пользователей</dt><dd>{number(summary.users_count)}</dd></div></dl></section>
}
function MetricToggle({ metric, onChange }: { metric: RetentionMetric; onChange: (metric: RetentionMetric) => void }) {
  return <div className="renewal-toggle" role="group" aria-label="Доля продлений по тарифам">{([['cumulative_rate', 'Кумулятивная'], ['rate', 'Условная']] as const).map(([id, label]) => <button key={id} type="button" aria-pressed={metric === id} onClick={() => onChange(id)}>{label}</button>)}</div>
}
function RevenueShare({ report, onPlanClick }: Pick<DesignOverviewProps, 'report' | 'onPlanClick'>) {
  const plans = [...report.plans].sort((a, b) => a.period_months - b.period_months)
  const total = plans.reduce((sum, plan) => sum + plan.total_revenue_minor, 0)
  const best = [...plans].filter(row => row.users_count > 0).sort((a, b) => Number(b.mean_ltv_minor) - Number(a.mean_ltv_minor))[0]
  return <section className="revenue-share" aria-labelledby="revenue-share-title"><div><h2 id="revenue-share-title">Выручка по тарифам</h2></div><div className="share-chart" role="img" aria-label={plans.map(plan => `${planName(plan.plan_id)}: ${percent(total ? plan.total_revenue_minor / total : 0)}`).join('; ')}>{plans.map(plan => <span key={plan.plan_id} style={{ width: `${total ? plan.total_revenue_minor / total * 100 : 0}%`, backgroundColor: planColor(plan.plan_id) }} />)}</div><div className="share-legend">{plans.map(plan => <button key={plan.plan_id} type="button" onClick={() => onPlanClick(plan.plan_id)} aria-label={`Открыть пользователей: ${planName(plan.plan_id)}`}><span className="share-legend-label"><i style={{ background: planColor(plan.plan_id) }}/>{plan.period_months} мес.</span><strong>{percent(total ? plan.total_revenue_minor / total : 0)}</strong></button>)}</div><div className="share-finding"><span>Лучший тариф по LTV</span>{best ? <button onClick={() => onPlanClick(best.plan_id)}>{best.period_months} мес. · {money(best.mean_ltv_minor)}<ArrowUpRight size={15}/></button> : <span>Нет пользователей</span>}</div></section>
}
export function PlanRenewalMatrix({ report, plan, metric, onMetricChange, periods, onPeriodsChange, onPlanClick }: DesignOverviewProps) {
  const plans = report.plans.filter(row => plan === 'all' || row.plan_id === plan).sort((a, b) => a.period_months - b.period_months)
  const [selected, setSelected] = useState<string | null>(null)
  return <section className="plan-renewal-panel" aria-labelledby="renewal-matrix-title"><header className="feature-heading"><div><h2 id="renewal-matrix-title">Продления по тарифам</h2></div><div className="matrix-controls"><label>Периодов<NativeSelect aria-label="Число периодов матрицы" value={periods} onChange={event => onPeriodsChange(Number(event.target.value))}>{[3, 6, 12, 24].map(value => <option key={value} value={value}>{value}</option>)}</NativeSelect></label><MetricToggle metric={metric} onChange={onMetricChange}/></div></header><div className="plan-matrix-scroll"><table className="plan-matrix" data-testid="plan-renewal-matrix"><thead><tr><th>Тариф / когорта</th><th className="plan-matrix-ltv">Средний LTV</th>{Array.from({ length: periods }, (_, i) => <th key={i}>#{i + 1}</th>)}</tr></thead><tbody>{plans.map(row => {
    const retention = report.retention.find(item => item.plan_id === row.plan_id)
    return <tr key={row.plan_id}><th scope="row"><button className="matrix-plan-link" onClick={() => onPlanClick(row.plan_id)}>{row.period_months} {row.period_months === 1 ? 'месяц' : row.period_months === 3 ? 'месяца' : 'месяцев'}<ArrowUpRight size={12}/></button><small>{number(retention?.cohort_size ?? 0)} участников цепочки</small></th><td className="plan-matrix-ltv">{row.users_count ? money(row.mean_ltv_minor) : '—'}</td>{Array.from({ length: periods }, (_, i) => {
      const period = retention?.periods[i], value = period?.[metric] ?? null
      const key = `${row.plan_id}-${i}`
      const evidence = `${planName(row.plan_id)} · продление ${i + 1}. ${value == null ? 'Нет наблюдений для расчёта' : percent(value)}. Продлили: ${number(period?.renewed)}. Дошли и завершили: ${number(period?.eligible)}. Зрелая исходная когорта: ${number(period?.cumulative_eligible)}.`
      return <td key={key}><Tooltip><TooltipTrigger asChild><button type="button" className={`plan-heat-cell ${value == null ? 'is-missing' : ''}`} style={heatAppearance(value)} aria-label={evidence} onClick={() => setSelected(selected === key ? null : key)} aria-pressed={selected === key} onKeyDown={event => { if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') { const next = event.key === 'ArrowRight' ? event.currentTarget.closest('td')?.nextElementSibling : event.currentTarget.closest('td')?.previousElementSibling; const button = next?.querySelector<HTMLButtonElement>('.plan-heat-cell'); if (button) { event.preventDefault(); button.focus() } } }} data-evidence={selected === key ? evidence : undefined}>{value == null ? '—' : `${Math.round(value * 100)}%`}</button></TooltipTrigger><TooltipContent className="max-w-72 bg-popover text-popover-foreground">{evidence}</TooltipContent></Tooltip></td>
    })}</tr>
  })}</tbody></table></div><div className="matrix-footer"><span className="heat-legend">0%{[0, 0.2, 0.4, 0.6, 0.8, 1].map(value => <i key={value} style={heatAppearance(value)}/>)}100%</span></div>{selected && <p className="matrix-selection" aria-live="polite">{(() => { const [id, index] = [selected.slice(0, selected.lastIndexOf('-')), Number(selected.slice(selected.lastIndexOf('-') + 1))]; const row = report.retention.find(item => item.plan_id === id)?.periods[index]; return row ? `${planName(id)} · продление ${index + 1}: ${percent(row[metric])}. Продлили ${number(row.renewed)}, дошли и завершили ${number(row.eligible)}, зрелая исходная когорта ${number(row.cumulative_eligible)}.` : 'В этом периоде нет наблюдений для расчёта.' })()}</p>}</section>
}
function TariffLedger({ report, onPlanClick }: Pick<DesignOverviewProps, 'report' | 'onPlanClick'>) {
  const total = report.plans.reduce((sum, row) => sum + row.total_revenue_minor, 0)
  return <section className="tariff-ledger"><header><h2>Тарифы по первой подписке</h2></header><div className="ledger-scroll"><table><thead><tr><th>Тариф</th><th>Пользователей</th><th>Средний LTV</th><th>Доля выручки</th><th>Первое продление</th><th aria-label="Открыть выборку"/></tr></thead><tbody>{[...report.plans].sort((a, b) => a.period_months - b.period_months).map(row => <tr key={row.plan_id}><th scope="row"><button onClick={() => onPlanClick(row.plan_id)}><i style={{ backgroundColor: planColor(row.plan_id) }}/>{row.period_months} мес.</button></th><td>{number(row.users_count)}</td><td>{money(row.mean_ltv_minor, 2)}</td><td>{percent(total ? row.total_revenue_minor / total : 0)}</td><td>{percent(report.retention.find(item => item.plan_id === row.plan_id)?.periods[0]?.cumulative_rate)}</td><td><button className="ledger-drilldown" onClick={() => onPlanClick(row.plan_id)} aria-label={`Открыть пользователей: ${planName(row.plan_id)}`}><ArrowUpRight size={18}/></button></td></tr>)}</tbody></table></div></section>
}
export function DarkOverview(props: DesignOverviewProps) {
  return <><div className="dark-lead-grid"><LifetimeValue report={props.report}/><RevenueShare report={props.report} onPlanClick={props.onPlanClick}/></div><PlanRenewalMatrix {...props}/></>
}
export function EditorialOverview(props: DesignOverviewProps) {
  const reports = props.report.retention.filter(row => props.plan === 'all' || row.plan_id === props.plan)
  return <><div className="editorial-lead-grid"><LifetimeValue report={props.report}/><div className="editorial-curves"><RetentionChart reports={reports} metric={props.metric} onMetricChange={props.onMetricChange} maxPeriods={props.periods} calendarMonths/><Button className="editorial-evidence-link" variant="link" onClick={props.onOpenRetention}>Продления<ArrowRight size={13}/></Button></div></div><TariffLedger report={props.report} onPlanClick={props.onPlanClick}/></>
}
export function SnapshotDetails({ report, onNavigate }: { report: Dashboard; onNavigate: () => void }) {
  return <Panel title={'Набор данных'} actions={<Button variant="link" onClick={onNavigate}>Источники<ArrowUpRight size={13}/></Button>}><dl className="snapshot-facts"><div><dt>Пользователи</dt><dd>{number(report.dataset.users_count)}</dd></div><div><dt>Подписки</dt><dd>{number(report.dataset.subscriptions_count)}</dd></div><div><dt>Платёжные попытки</dt><dd>{number(report.dataset.payments_count)}</dd></div><div><dt>Полные возвраты</dt><dd>{number(report.dataset.refunded_count)}</dd></div></dl></Panel>
}

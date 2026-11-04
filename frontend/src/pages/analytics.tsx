import { ArrowRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { UTMComparison } from '@/components/analytics/utm-comparison'
import { CohortHeatmap } from '@/components/analytics/heatmap'
import { MetricStrip } from '@/components/analytics/metrics'
import { TimelineChart, RevenueRanking, RetentionChart, DistributionChart } from '@/components/analytics/charts'
import { PlanComparison, RetentionTable } from '@/components/analytics/plan-comparison'
import { money, planNames } from '@/lib/format'
import type { Dashboard, PlanFilter, RetentionMetric, TrendMetric, View } from '@/types'

interface AnalyticsPageProps {
  view: View; report: Dashboard; plan: PlanFilter
  trend: TrendMetric; onTrendChange: (metric: TrendMetric) => void
  metric: RetentionMetric; onMetricChange: (metric: RetentionMetric) => void
  periods: number; onPeriodsChange: (periods: number) => void
  onMonthClick: (month: string, chain?: boolean) => void; onPlanClick: (plan: string) => void
  onNavigate: (view: View) => void; onUTMClick?: (utm: string) => void
}
export function AnalyticsPage({ view, report, plan, trend, onTrendChange, metric, onMetricChange, periods, onPeriodsChange, onMonthClick, onPlanClick, onNavigate, onUTMClick }: AnalyticsPageProps) {
  const reports = report.retention.filter(row => plan === 'all' || row.plan_id === plan)
  const best = [...report.plans].filter(row => row.users_count > 0).sort((a, b) => Number(b.mean_ltv_minor) - Number(a.mean_ltv_minor))[0]
  const timeline = <TimelineChart rows={report.timeline} metric={trend} onMetricChange={onTrendChange} onMonthClick={onMonthClick}/>
  const ranking = <RevenueRanking plans={report.plans} onPlanClick={onPlanClick} onOpenAll={view === 'overview' ? () => onNavigate('revenue') : undefined}/>
  const heatmap = (compact = false) => <CohortHeatmap rows={report.cohorts} metric={metric} onMetricChange={onMetricChange} planId={plan === 'all' ? 'monthly' : plan} compact={compact} onMonthClick={month => onMonthClick(month, true)} onOpenAll={compact ? () => onNavigate('cohorts') : undefined}/>
  const retention = (compact = false) => <RetentionChart reports={reports} metric={metric} onMetricChange={onMetricChange} maxPeriods={compact ? 8 : periods} compact={compact}/>
  if (view === 'retention') return <div className="analytics-layout"><p className="report-notice">В расчёт входят только завершённые периоды. Условная доля — от дошедших, кумулятивная — от зрелой исходной когорты. Переход на тариф начинает новую цепочку.</p>{retention()}<RetentionTable reports={reports} maxPeriods={periods} onMaxPeriodsChange={onPeriodsChange}/></div>
  if (view === 'cohorts') return <div className="analytics-layout"><p className="report-notice">Месяц первой подписки × номер продления. Нажмите ячейку, чтобы открыть пользователей этой когорты. Незавершённые периоды отмечены прочерком.</p>{heatmap()}<div className="analytics-pair">{retention(true)}{timeline}</div></div>
  return <div className="analytics-layout">
    <MetricStrip summary={report.summary}/>
    {best && <div className="insight-line"><span>Самый высокий средний LTV — <strong>{planNames[best.plan_id]?.toLocaleLowerCase('ru')} тариф, <span className="whitespace-nowrap">{money(best.mean_ltv_minor, 2)}</span></strong></span><Button variant="link" onClick={() => onPlanClick(best.plan_id)}>Посмотреть пользователей<ArrowRight size={13}/></Button></div>}
    <div className="overview-primary">{timeline}{ranking}</div>
    {view === 'revenue' ? <><div className="analytics-pair"><DistributionChart rows={report.distribution} total={report.summary.users_count}/><UTMComparison rows={report.utm_groups} onSelect={onUTMClick} compact/></div><PlanComparison plans={report.plans} onPlanClick={onPlanClick}/></> : <><div className="overview-secondary">{heatmap(true)}<UTMComparison rows={report.utm_groups} onSelect={onUTMClick} compact/></div><div className="analytics-section-title"><span className="text-xs text-muted-foreground">От общей картины к конкретной когорте</span><Button variant="link" onClick={() => onNavigate('retention')}>Исследовать продления<ArrowRight size={13}/></Button></div></>}
  </div>
}

import { ArrowUpRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { UTMComparison } from '@/components/analytics/utm-comparison'
import { CohortHeatmap } from '@/components/analytics/heatmap'
import { MetricStrip } from '@/components/analytics/metrics'
import { TimelineChart, RevenueRanking, RetentionChart, DistributionChart } from '@/components/analytics/charts'
import { PlanComparison, RetentionTable } from '@/components/analytics/plan-comparison'
import type { Dashboard, PlanFilter, RetentionMetric, TrendMetric, View } from '@/types'

interface AnalyticsPageProps {
  view: View; report: Dashboard; plan: PlanFilter
  trend: TrendMetric; onTrendChange: (metric: TrendMetric) => void
  metric: RetentionMetric; onMetricChange: (metric: RetentionMetric) => void
  periods: number; onPeriodsChange: (periods: number) => void
  onMonthClick: (month: string, chain?: boolean) => void; onPlanClick: (plan: string) => void
  onNavigate: (view: View) => void
  onUTMClick?: (utm: string) => void
}
export function AnalyticsPage({ view, report, plan, trend, onTrendChange, metric, onMetricChange, periods, onPeriodsChange, onMonthClick, onPlanClick, onNavigate, onUTMClick }: AnalyticsPageProps) {
  const reports = report.retention.filter(row => plan === 'all' || row.plan_id === plan)
  const timeline = <TimelineChart rows={report.timeline} metric={trend} onMetricChange={onTrendChange} onMonthClick={onMonthClick}/>
  const heatmap = (compact = false) => <CohortHeatmap rows={report.cohorts} metric={metric} onMetricChange={onMetricChange} planId={plan === 'all' ? 'monthly' : plan} compact={compact} onMonthClick={month => onMonthClick(month, true)} onOpenAll={compact ? () => onNavigate('cohorts') : undefined}/>
  const retention = (compact = false) => <RetentionChart reports={reports} metric={metric} onMetricChange={onMetricChange} maxPeriods={compact ? 8 : periods} compact={compact}/>
  if (view === 'retention') return <div className="space-y-6"><p className="rounded-md bg-muted/70 px-4 py-3 text-[13px] leading-relaxed text-muted-foreground">Цепочки выбранного тарифа включают переходы с других тарифов. Даты отбирают пользователей по самой первой подписке. Незавершённые периоды не считаются оттоком.</p>{retention()}<RetentionTable reports={reports} maxPeriods={periods} onMaxPeriodsChange={onPeriodsChange}/></div>
  if (view === 'cohorts') return <div className="space-y-6"><p className="rounded-md bg-muted/70 px-4 py-3 text-[13px] leading-relaxed text-muted-foreground">Строка — месяц первой подписки. Ячейка — продление выбранного тарифа. Откройте месяц или ячейку, чтобы увидеть состав когорты.</p>{heatmap()}<div className="grid gap-6 xl:grid-cols-2">{retention(true)}{timeline}</div></div>
  return <div className="space-y-6"><MetricStrip summary={report.summary}/><UTMComparison rows={report.utm_groups} onSelect={onUTMClick}/><div className="grid gap-6 xl:grid-cols-[1.65fr_1fr]">{timeline}<RevenueRanking plans={report.plans} onPlanClick={onPlanClick}/></div>{view === 'revenue' ? <><DistributionChart rows={report.distribution} total={report.summary.users_count}/><PlanComparison plans={report.plans} onPlanClick={onPlanClick}/></> : <><div className="grid gap-6 xl:grid-cols-2">{heatmap(true)}{retention(true)}</div><div className="flex flex-wrap items-center gap-x-6 gap-y-2 text-[13px] text-muted-foreground"><span>От сводных цифр — к конкретным пользователям.</span><Button variant="link" className="px-0 text-[13px]" onClick={() => onNavigate('users')}>Исследовать выборку<ArrowUpRight className="size-3.5"/></Button><Button variant="link" className="px-0 text-[13px]" onClick={() => onNavigate('revenue')}>Сравнение тарифов<ArrowUpRight className="size-3.5"/></Button></div></>}</div>
}

import { UTMComparison } from '@/components/analytics/utm-comparison'
import { CohortHeatmap } from '@/components/analytics/heatmap'
import { MetricStrip } from '@/components/analytics/metrics'
import { TimelineChart, RevenueRanking, RetentionChart, DistributionChart } from '@/components/analytics/charts'
import { PlanComparison, RetentionTable } from '@/components/analytics/plan-comparison'
import { DarkOverview, EditorialOverview, SnapshotDetails } from '@/components/analytics/design-overview'
import type { DesignVariant } from '@/lib/design'
import type { Dashboard, PlanFilter, RetentionMetric, TrendMetric, View } from '@/types'

interface AnalyticsPageProps {
  design: DesignVariant
  view: View; report: Dashboard; plan: PlanFilter
  trend: TrendMetric; onTrendChange: (metric: TrendMetric) => void
  metric: RetentionMetric; onMetricChange: (metric: RetentionMetric) => void
  periods: number; onPeriodsChange: (periods: number) => void
  onMonthClick: (month: string, chain?: boolean) => void; onPlanClick: (plan: string) => void
  onNavigate: (view: View) => void; onUTMClick?: (utm: string) => void
}
export function AnalyticsPage({ design, view, report, plan, trend, onTrendChange, metric, onMetricChange, periods, onPeriodsChange, onMonthClick, onPlanClick, onNavigate, onUTMClick }: AnalyticsPageProps) {
  const reports = report.retention.filter(row => plan === 'all' || row.plan_id === plan)
  const timeline = <TimelineChart rows={report.timeline} metric={trend} onMetricChange={onTrendChange} onMonthClick={onMonthClick}/>
  const ranking = <RevenueRanking plans={report.plans} onPlanClick={onPlanClick} onOpenAll={view === 'overview' ? () => onNavigate('revenue') : undefined}/>
  const heatmap = (compact = false) => <CohortHeatmap rows={report.cohorts} metric={metric} onMetricChange={onMetricChange} planId={plan === 'all' ? 'monthly' : plan} compact={compact} onMonthClick={month => onMonthClick(month, true)} onOpenAll={compact ? () => onNavigate('cohorts') : undefined}/>
  const retention = (compact = false) => <RetentionChart reports={reports} metric={metric} onMetricChange={onMetricChange} maxPeriods={compact ? 8 : periods} compact={compact}/>
  if (view === 'retention') return <div className="analytics-layout">{retention()}<RetentionTable reports={reports} maxPeriods={periods} onMaxPeriodsChange={onPeriodsChange}/></div>
  if (view === 'cohorts') return <div className="analytics-layout">{heatmap()}<div className="analytics-pair">{retention(true)}{timeline}</div></div>
  if (view === 'overview') {
    const Overview = design === 'a' ? DarkOverview : EditorialOverview
    return <div className="analytics-layout"><Overview report={report} plan={plan} metric={metric} onMetricChange={onMetricChange} periods={periods} onPeriodsChange={onPeriodsChange} onPlanClick={onPlanClick} onOpenRetention={() => onNavigate('retention')}/><div className="overview-support">{timeline}<UTMComparison rows={report.utm_groups} onSelect={onUTMClick} compact/></div><SnapshotDetails report={report} onNavigate={() => onNavigate('sources')}/></div>
  }
  return <div className="analytics-layout">
    <MetricStrip summary={report.summary}/>
    <div className="overview-primary">{timeline}{ranking}</div>
    <div className="analytics-pair"><DistributionChart rows={report.distribution} total={report.summary.users_count}/><UTMComparison rows={report.utm_groups} onSelect={onUTMClick} compact/></div><PlanComparison plans={report.plans} onPlanClick={onPlanClick}/>
  </div>
}

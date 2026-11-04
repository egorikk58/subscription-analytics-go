export type PlanId = 'monthly' | 'quarterly' | 'half-year' | 'yearly'
export type PlanFilter = PlanId | 'all'
export type Selection = 'first_plan' | 'chain'
export type UserSort = 'ltv_desc' | 'ltv_asc' | 'joined_desc'
export type RetentionMetric = 'rate' | 'cumulative_rate'
export type TrendMetric = 'total_revenue_minor' | 'mean_ltv_minor' | 'users_count'
export type View = 'overview' | 'revenue' | 'retention' | 'cohorts' | 'users' | 'generator' | 'sources'

export interface Summary {
  currency: string
  users_count: number
  total_revenue_minor: number
  mean_ltv_minor: string
  median_ltv_minor: string
}
export interface PlanSummary extends Summary { plan_id: PlanId; name: string; period_months: number }
export interface TimelineRow extends Summary { month: string }
export interface PeriodRow {
  period: number; eligible: number; renewed: number; cumulative_eligible: number
  rate: number | null; cumulative_rate: number | null
}
export interface RetentionReport { plan_id: PlanId; cohort_size: number; periods: PeriodRow[] }
export interface CohortRow { month: string; users_count: number; periods: PeriodRow[] }
export interface DistributionRow { label: string; min_minor: number; max_minor: number | null; users_count: number }
export interface UserRow {
  id: string; telegram_id: number; first_subscription_at: string | null
  first_plan_id: PlanId | ''; ltv_minor: number; utm_link: string
}
export interface UserPage { items: UserRow[]; page: number; page_size: number; total: number; pages: number }
export interface SourceUser { id: string; telegram_id: number; created_at: string; updated_at: string; utm_link: string }
export interface SourceSubscription {
  id: string; user_id: string; plan_id: PlanId; status: string; started_at: string
  current_period_end: string; canceled_at: string | null; updated_at: string
}
export interface SourcePayment {
	fx?: { id: string; rub_per_unit: string; as_of: string; source: string }
  id: string; user_id: string; subscription_id: string; amount: number; currency: string
  status: 'succeeded' | 'failed' | 'refunded'; period_start: string; period_end: string
  created_at: string; updated_at: string; refund_of_payment_id?: string
}
export interface DatasetSamples { users: SourceUser[]; subscriptions: SourceSubscription[]; payments: SourcePayment[] }
export interface DatasetInfo {
	currencies?: Record<string, number>
  seed: number | null; generator_version: string; history_months: number; dataset_as_of: string | null
  users_count: number; subscriptions_count: number; payments_count: number
  succeeded_count: number; failed_count: number; refunded_count: number; switches_count: number
  unpaid_users: number; active_periods: number; fingerprint: string; samples: DatasetSamples
}
export interface Dashboard {
	utm_groups?: (Summary & { utm_link: string })[]
	utm_options?: string[]
  data_as_of: string; snapshot_id: number | null; mode: 'live' | 'preview'; selection: Selection
  summary: Summary; plans: PlanSummary[]; timeline: TimelineRow[]; retention: RetentionReport[]
  cohorts: CohortRow[]; distribution: DistributionRow[]; users: UserPage; dataset: DatasetInfo
}
export interface ServiceStatus {
  snapshot_id: number | null; data_as_of: string | null; last_success_at: string | null
  next_sync_at: string | null; last_error: string | null; sync_in_progress: boolean
}
export interface SeedConfig { seed: number; users_count?: number; history_months: number; data_as_of: string }
export type DataSource = { mode: 'live' } | { mode: 'preview'; config: SeedConfig }
export interface Filters { from: string; to: string; plan_id: PlanFilter; selection: Selection; utm_link?: string }
export interface DashboardParams extends Filters { page: number; search: string; sort: UserSort }

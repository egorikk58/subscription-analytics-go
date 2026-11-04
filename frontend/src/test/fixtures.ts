import type { Dashboard, PlanId } from '@/types'

export function makeDashboard(): Dashboard {
  const summary = { currency: 'RUB', users_count: 10000, total_revenue_minor: 6567629000, mean_ltv_minor: '656762.90', median_ltv_minor: '499000.00' }
  const user = { id: '00000000-0000-4000-8000-000000000001', telegram_id: 1000000001, created_at: '2024-01-01T00:00:00Z', updated_at: '2024-01-01T00:00:00Z', utm_link: 'example' }
  const sub = { id: '00000000-0000-4000-8000-000000000002', user_id: user.id, plan_id: 'monthly' as const, status: 'active', started_at: '2024-01-01T00:00:00Z', current_period_end: '2024-02-01T00:00:00Z', canceled_at: null, updated_at: '2024-01-01T00:00:00Z' }
  const payment = { id: '00000000-0000-4000-8000-000000000003', user_id: user.id, subscription_id: sub.id, amount: 99000, currency: 'RUB', status: 'succeeded' as const, period_start: sub.started_at, period_end: sub.current_period_end, created_at: sub.started_at, updated_at: sub.started_at }
  const period = { period: 1, eligible: 4, renewed: 2, cumulative_eligible: 8, rate: 0.5, cumulative_rate: 0.25 }
  return {
    data_as_of: '2026-11-01T00:00:00Z', snapshot_id: 1, mode: 'live', selection: 'first_plan', summary,
    plans: (['monthly', 'quarterly', 'half-year', 'yearly'] as PlanId[]).map((plan_id, index) => ({ ...summary, plan_id, name: plan_id, period_months: [1, 3, 6, 12][index] ?? 1 })),
    timeline: [{ ...summary, month: '2024-01' }, { ...summary, month: '2024-02' }],
    retention: [{ plan_id: 'monthly', cohort_size: 8, periods: [period] }],
    cohorts: [{ month: '2024-01', users_count: 8, periods: [period, { ...period, period: 2, eligible: 0, renewed: 0, cumulative_eligible: 0, rate: null, cumulative_rate: null }] }],
    distribution: [{ label: 'Без выручки', min_minor: 0, max_minor: 1, users_count: 1 }],
    users: { page: 1, page_size: 20, total: 1, pages: 1, items: [{ id: user.id, telegram_id: user.telegram_id, first_subscription_at: sub.started_at, first_plan_id: 'monthly', ltv_minor: 99000, utm_link: user.utm_link }] },
    dataset: {
      seed: 42, generator_version: 'v1', history_months: 24, dataset_as_of: '2026-11-01T00:00:00Z', users_count: 10000, subscriptions_count: 9500, payments_count: 24000,
      succeeded_count: 22000, failed_count: 1500, refunded_count: 500, switches_count: 300, unpaid_users: 1000, active_periods: 5000, fingerprint: 'a'.repeat(64),
      samples: { users: [user], subscriptions: [sub], payments: [payment, { ...payment, id: '00000000-0000-4000-8000-000000000004', status: 'failed' }, { ...payment, id: '00000000-0000-4000-8000-000000000005', status: 'refunded', refund_of_payment_id: payment.id }] },
    },
  }
}

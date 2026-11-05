import { useQuery } from '@tanstack/react-query'
import { LoaderCircle } from 'lucide-react'
import { PlanName } from '@/components/dataset-samples'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ApiError, fetchUserHistory } from '@/lib/api'
import { assetMoney, date, money, number } from '@/lib/format'
import type { Dashboard, UserHistory, UserRow } from '@/types'

export interface UserDetails {
  user: UserRow
  mode: Dashboard['mode']
  snapshotID: Dashboard['snapshot_id']
  previewHistory?: UserHistory
}

const subscriptionStatus: Record<string, string> = {
  active: 'Активна', canceled: 'Отменена', expired: 'Завершена', none: 'Без подписки',
}

const paymentDate = new Intl.DateTimeFormat('ru-RU', {
  dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC',
})

function paymentLabel(payment: UserHistory['payments'][number], startedAt?: string) {
  if (payment.status === 'failed') return 'Неуспешно'
  if (payment.status === 'refunded') return 'Возврат'
  if (!startedAt) return 'Оплачено'
  return Date.parse(payment.period_start) === Date.parse(startedAt) ? 'Покупка' : 'Продление'
}

function HistoryLedger({ history }: { history: UserHistory }) {
  const subscriptions = new Map(history.subscriptions.map(subscription => [subscription.id, subscription]))
  return (
    <div className="min-w-0 space-y-5">
      <section aria-label="История платежей">
        <h3 className="border-b border-border pb-2 text-sm font-semibold">Платежи <span className="ml-1 font-normal text-muted-foreground">{number(history.payments.length)}</span></h3>
        {history.payments.length ? (
          <ul className="divide-y divide-border">
            {history.payments.map(payment => {
              const subscription = subscriptions.get(payment.subscription_id)
              const sign = payment.status === 'refunded' ? '−' : ''
              return (
                <li key={payment.id} className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-3 gap-y-1 py-3 text-sm sm:grid-cols-[minmax(0,1fr)_auto_auto] sm:gap-x-6">
                  <div className="min-w-0">
                    <div className="font-medium">{subscription ? <PlanName planId={subscription.plan_id}/> : 'Подписка'}</div>
                    <time dateTime={payment.created_at} className="mt-1 block text-xs text-muted-foreground">{paymentDate.format(new Date(payment.created_at))} UTC</time>
                  </div>
                  <div className="text-right tabular-nums">
                    <div className="font-medium">{sign}{assetMoney(payment.amount, payment.currency)}</div>
                    {payment.currency !== 'RUB' && <div className="mt-1 text-xs text-muted-foreground">≈ {sign}{money(payment.amount_rub_minor, 2)}</div>}
                  </div>
                  <Badge variant={payment.status === 'failed' ? 'destructive' : 'secondary'} className="col-span-2 rounded-sm font-normal sm:col-span-1">{paymentLabel(payment, subscription?.started_at)}</Badge>
                  <div className="col-span-full text-xs text-muted-foreground">Период {date(payment.period_start)} — {date(payment.period_end)}</div>
                </li>
              )
            })}
          </ul>
        ) : <p className="py-3 text-sm text-muted-foreground">Платежей пока нет</p>}
      </section>
      <section aria-label="История подписок">
        <h3 className="border-b border-border pb-2 text-sm font-semibold">Подписки <span className="ml-1 font-normal text-muted-foreground">{number(history.subscriptions.length)}</span></h3>
        {history.subscriptions.length ? (
          <ul className="divide-y divide-border">
            {history.subscriptions.map(subscription => (
              <li key={subscription.id} className="flex flex-wrap items-start justify-between gap-2 py-3 text-sm">
                <div className="space-y-1">
                  <div className="font-medium"><PlanName planId={subscription.plan_id}/></div>
                  <div className="text-xs text-muted-foreground">{date(subscription.started_at)} — {date(subscription.current_period_end)}</div>
                  {subscription.canceled_at && <div className="text-xs text-muted-foreground">Отмена {date(subscription.canceled_at)}</div>}
                </div>
                <Badge variant="secondary" className="rounded-sm font-normal">{subscriptionStatus[subscription.status] ?? subscription.status}</Badge>
              </li>
            ))}
          </ul>
        ) : <p className="py-3 text-sm text-muted-foreground">Подписок пока нет</p>}
      </section>
    </div>
  )
}

function historyError(error: Error) {
  if (error instanceof ApiError) {
    if (error.status === 410) return 'Снимок истёк. Обновите выборку.'
    if (error.status === 404) return 'Пользователь не найден в снимке.'
    if (error.status === 503) return 'История пока недоступна.'
  }
  return 'Не удалось загрузить историю.'
}

export function UserHistoryDialog({ detail, onClose, trigger }: {
  detail: UserDetails
  onClose: () => void
  trigger: HTMLButtonElement | null
}) {
  const { user, mode, snapshotID, previewHistory } = detail
  const history = useQuery({
    queryKey: ['user-history', snapshotID, user.telegram_id],
    queryFn: ({ signal }) => fetchUserHistory(user.telegram_id, snapshotID, signal),
    enabled: mode === 'live',
    staleTime: 30_000,
    retry: false,
  })
  const profile = mode === 'live' ? history.data : previewHistory
  const firstPlan = profile?.first_plan_id ?? user.first_plan_id
  return (
    <Dialog open onOpenChange={open => { if (!open) onClose() }}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] min-w-0 overflow-y-auto p-4 sm:max-h-[85dvh] sm:max-w-2xl sm:p-6"
        onCloseAutoFocus={event => {
          if (trigger?.isConnected) {
            event.preventDefault()
            trigger.focus()
          }
        }}
      >
        <DialogHeader className="pr-6 text-left">
          <DialogTitle>История покупок</DialogTitle>
          <DialogDescription className="flex flex-wrap gap-x-2 break-all text-xs">
            <span>Telegram <span>{user.telegram_id}</span></span>
            <span>{user.id}</span>
          </DialogDescription>
        </DialogHeader>
        <dl className="grid grid-cols-2 gap-3 border-y border-border py-3 text-sm sm:gap-x-6">
          <div><dt className="text-xs text-muted-foreground">Пожизненный LTV</dt><dd className="mt-1 font-semibold tabular-nums">{money(profile?.ltv ?? user.ltv_minor, 2)}</dd></div>
          <div><dt className="text-xs text-muted-foreground">Первый тариф</dt><dd className="mt-1"><PlanName planId={firstPlan ?? ''}/></dd></div>
          <div><dt className="text-xs text-muted-foreground">Первая подписка</dt><dd className="mt-1">{date(user.first_subscription_at)}</dd></div>
          <div><dt className="text-xs text-muted-foreground">UTM</dt><dd className="mt-1 break-words">{user.utm_link || '—'}</dd></div>
          {profile && <div><dt className="text-xs text-muted-foreground">Статус подписки</dt><dd className="mt-1">{subscriptionStatus[profile.current_status] ?? profile.current_status}</dd></div>}
          {profile && <div><dt className="text-xs text-muted-foreground">Успешные оплаты</dt><dd className="mt-1 tabular-nums">{number(profile.payments_count)}</dd></div>}
        </dl>
        {mode === 'live' && history.isPending && (
          <p role="status" className="flex items-center gap-2 py-3 text-sm text-muted-foreground"><LoaderCircle aria-hidden="true" className="size-4 animate-spin"/>Загружаем покупки…</p>
        )}
        {mode === 'live' && history.error && (
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p role="alert" className="text-sm text-destructive">{historyError(history.error)}</p>
            {!(history.error instanceof ApiError && history.error.status === 410) && (
              <Button variant="outline" size="sm" disabled={history.isFetching} onClick={() => { void history.refetch() }}>Повторить</Button>
            )}
          </div>
        )}
        {profile && <HistoryLedger history={profile}/>}
      </DialogContent>
    </Dialog>
  )
}

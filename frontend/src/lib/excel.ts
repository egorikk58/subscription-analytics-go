import { ApiError, queryString } from '@/lib/api'
import type { Dashboard, DashboardParams, DataSource } from '@/types'

export interface ExportContext {
  report: Dashboard
  params: DashboardParams
  source: DataSource
  selectedIds: string[]
}
export type ReportKind = 'analytics' | 'users'

export async function downloadWorkbook(context: ExportContext, kind: ReportKind, selected: boolean, signal: AbortSignal): Promise<void> {
  const query = new URLSearchParams(queryString(context.params))
  query.delete('page')
  query.set('kind', kind)
  query.set('fingerprint', context.report.dataset.fingerprint)
  if (context.report.snapshot_id != null) query.set('snapshot_id', String(context.report.snapshot_id))
  if (selected) for (const id of context.selectedIds) query.append('user_id', id)
  const preview = context.source.mode === 'preview'
  const response = await fetch('/api/reports/export.xlsx?' + query, {
    method: preview ? 'POST' : 'GET', signal, cache: 'no-store',
    ...(context.source.mode === 'preview' ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(context.source.config) } : {}),
  })
  if (!response.ok) {
    let message = 'Не удалось сформировать отчёт. Повторите запрос.'
    try { const error: unknown = await response.json(); if (error && typeof error === 'object' && 'error' in error && typeof error.error === 'string') message = error.error } catch { /* Keep the fallback for proxy errors. */ }
    throw new ApiError(message, response.status)
  }
  if (!response.headers.get('content-type')?.includes('spreadsheetml.sheet')) throw new ApiError('Сервис вернул файл в неизвестном формате.', response.status)
  const file = await response.blob()
  if (signal.aborted) return
  const filename = response.headers.get('content-disposition')?.match(/filename="([a-zA-Z0-9_.-]+)"/)?.[1] ?? 'subscope-report.xlsx'
  const url = URL.createObjectURL(file)
  const link = document.createElement('a')
  link.href = url; link.download = filename
  document.body.append(link); link.click(); link.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
}

import type { Dashboard, DashboardParams, DataSource, SeedConfig, ServiceStatus } from '@/types'

export const defaultFilters = { from: '', to: '', plan_id: 'all', selection: 'first_plan', utm_link: '' } as const
export const defaultParams: DashboardParams = { ...defaultFilters, page: 1, search: '', sort: 'ltv_desc' }
export class ApiError extends Error {
  constructor(message: string, public readonly status: number) { super(message); this.name = 'ApiError' }
}
export function queryString(params: DashboardParams): string {
  const query = new URLSearchParams({ plan_id: params.plan_id, page: String(params.page), sort: params.sort, selection: params.selection })
  if (params.search.trim()) query.set('search', params.search.trim())
  if (params.utm_link) query.set('utm_link', params.utm_link)
  if (params.from) query.set('from', params.from + 'T00:00:00Z')
  if (params.to) {
    const end = new Date(params.to + 'T00:00:00Z')
    end.setUTCDate(end.getUTCDate() + 1)
    query.set('to', end.toISOString())
  }
  return query.toString()
}
export const dashboardKey = (source: DataSource, params: DashboardParams) => ['dashboard', source, queryString(params)] as const
async function readJSON<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, { ...options, cache: 'no-store' })
  let payload: unknown
  try { payload = await response.json() } catch { throw new ApiError('Сервис вернул некорректный ответ. Повторите запрос.', response.status) }
  if (!response.ok) {
    const error = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string' ? payload.error : 'Не удалось получить данные'
    throw new ApiError(error === 'snapshot unavailable' ? 'Первый снимок ещё готовится. Повторите запрос через несколько секунд.' : error, response.status)
  }
  return payload as T
}
export function fetchDashboard(source: DataSource, params: DashboardParams, signal?: AbortSignal): Promise<Dashboard> {
  const query = queryString(params)
  return source.mode === 'live'
    ? readJSON<Dashboard>('/api/dashboard?' + query, { signal })
    : fetchPreview(source.config, params, signal)
}
export function fetchPreview(config: SeedConfig, params: DashboardParams, signal?: AbortSignal): Promise<Dashboard> {
  return readJSON<Dashboard>('/api/demo/preview?' + queryString(params), {
    method: 'POST', signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config),
  })
}
export const fetchStatus = (signal?: AbortSignal) => readJSON<ServiceStatus>('/status', { signal })

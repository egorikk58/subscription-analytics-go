import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertCircle, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Toaster } from '@/components/ui/sonner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { AppShell } from '@/components/app-shell'
import { views } from '@/lib/navigation'
import { FilterBar } from '@/components/filters'
const AnalyticsPage = lazy(() => import('@/pages/analytics').then(module => ({ default: module.AnalyticsPage })))
const UsersPage = lazy(() => import('@/pages/users').then(module => ({ default: module.UsersPage })))
const GeneratorPage = lazy(() => import('@/pages/generator').then(module => ({ default: module.GeneratorPage })))
const SourcesPage = lazy(() => import('@/pages/sources').then(module => ({ default: module.SourcesPage })))
import { ApiError, dashboardKey, defaultFilters, defaultParams, fetchDashboard, fetchPreview, fetchStatus } from '@/lib/api'
import { downloadCSV } from '@/lib/csv'
import type { Dashboard, DataSource, Filters, PlanId, RetentionMetric, SeedConfig, TrendMetric, UserSort, View } from '@/types'

function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => { const timeout = setTimeout(() => setDebounced(value), delay); return () => clearTimeout(timeout) }, [value, delay])
  return debounced
}
function LoadingDashboard() { return <div className="space-y-6" role="status" aria-label="Загрузка аналитики"><span className="sr-only">Загружаем опубликованный снимок…</span><div className="grid grid-cols-2 gap-4 md:grid-cols-4">{[1, 2, 3, 4].map(key => <Skeleton key={key} className="h-24 rounded-md"/>)}</div><div className="grid gap-6 xl:grid-cols-[1.65fr_1fr]"><Skeleton className="h-72 rounded-lg"/><Skeleton className="h-72 rounded-lg"/></div></div> }

export function App() {
  const navigate = useNavigate(), location = useLocation(), queryClient = useQueryClient()
  const path = location.pathname.replace(/^\//, '')
  const view: View = Object.hasOwn(views, path) ? path as View : 'overview'
  const [source, setSource] = useState<DataSource>({ mode: 'live' })
  const [filters, setFilters] = useState<Filters>({ ...defaultFilters })
  const [search, setSearch] = useState('')
  const delayedSearch = useDebouncedValue(search, 400)
  const debouncedSearch = search ? delayedSearch : ""
  const generationEpoch = useRef(0)
  const generationAbort = useRef<AbortController | null>(null)
  const [sampleKind, setSampleKind] = useState<"users" | "subscriptions" | "payments">("payments")
  const [sort, setSort] = useState<UserSort>('ltv_desc'), [page, setPage] = useState(1)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [trend, setTrend] = useState<TrendMetric>('total_revenue_minor')
  const [metric, setMetric] = useState<RetentionMetric>('cumulative_rate')
  const [periods, setPeriods] = useState(12)
  const [seedDraft, setSeedDraft] = useState<SeedConfig>()
  const params = { ...filters, page, sort, search: debouncedSearch }
  const key = dashboardKey(source, params)
  const reportQuery = useQuery({
    queryKey: key, queryFn: async ({ signal }) => ({ report: await fetchDashboard(source, params, signal), params }), staleTime: Infinity,
    placeholderData: keepPreviousData,
    retry: (count, error) => source.mode === 'live' && count < 1 && (!(error instanceof ApiError) || error.status === 429 || error.status >= 500),
  })
  const statusQuery = useQuery({ queryKey: ['status'], queryFn: ({ signal }) => fetchStatus(signal), staleTime: 15000, refetchInterval: 30000, retry: 1 })
  const report = reportQuery.data?.report
  const shownFilters = reportQuery.data?.params
  const status = statusQuery.data
  useEffect(() => { if (!Object.hasOwn(views, path)) void navigate('/overview', { replace: true }) }, [navigate, path])
  useEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }) }, [view])
  useEffect(() => {
    if (source.mode === 'live' && status?.snapshot_id && report?.mode === 'live' && report.snapshot_id !== status.snapshot_id) {
      void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    }
  }, [source.mode, status?.snapshot_id, report?.snapshot_id, report?.mode, queryClient])

  function clearSelection() { setPage(1); setSelectedIds(new Set()) }
  function changeFilters(value: Filters) { setFilters(value); clearSelection() }
  function resetFilters() { setFilters({ ...defaultFilters }); setSearch(''); clearSelection() }
  function openView(next: View) { void navigate('/' + next) }
  function openMonth(month: string, chain = false) {
    const [year = 0, monthIndex = 1] = month.split('-').map(Number)
    const to = new Date(Date.UTC(year, monthIndex, 0)).toISOString().slice(0, 10)
    setFilters(current => ({ ...current, from: month + '-01', to, selection: chain ? 'chain' : current.selection, plan_id: chain && current.plan_id === 'all' ? 'monthly' : current.plan_id }))
    setSearch(''); clearSelection(); openView('users')
  }
  function openPlan(plan: string) {
    if (!['monthly', 'quarterly', 'half-year', 'yearly'].includes(plan)) return
    setFilters(current => ({ ...current, plan_id: plan as PlanId, selection: 'first_plan' }))
    setSearch(''); clearSelection(); openView('users')
  }
  const generation = useMutation({
    mutationFn: ({config, signal}: {config: SeedConfig; epoch: number; signal: AbortSignal}) => fetchPreview(config, { ...defaultParams, sort }, signal),
    onSuccess: (nextReport, {config, epoch}) => {
      if (epoch !== generationEpoch.current) return
      const nextSource: DataSource = { mode: 'preview', config }
      queryClient.setQueryData(dashboardKey(nextSource, { ...defaultParams, sort }), {report: nextReport, params: {...defaultParams, sort}})
      setSource(nextSource); setFilters({ ...defaultFilters }); setSearch(''); clearSelection()
      toast.success(`Создан набор seed ${config.seed}. Откройте любой дашборд.`)
    },
  })
  async function generate(config: SeedConfig): Promise<void> {
    generationAbort.current?.abort()
    const controller = new AbortController()
    generationAbort.current = controller
    const epoch = ++generationEpoch.current
    try { await generation.mutateAsync({config, epoch, signal: controller.signal}) }
    catch (error) { if (!controller.signal.aborted) throw error }
  }
  function returnLive() {
    ++generationEpoch.current
    generationAbort.current?.abort()
    setSource({ mode: 'live' }); setSeedDraft(undefined); resetFilters(); generation.reset(); toast.dismiss()
    void queryClient.invalidateQueries({ queryKey: ['dashboard'] })
  }
  function refresh() {
    void reportQuery.refetch(); void statusQuery.refetch()
    toast.info('Читаем текущий снимок. Источники опрашиваются по расписанию.')
  }
  function exportCSV() {
    if (!report) return
    const filename = `subscope-${view}-${report.mode === 'preview' ? 'seed-' + report.dataset.seed : 'snapshot-' + report.snapshot_id}.csv`
    if (view === 'users') {
      const rows = report.users.items.filter(user => !selectedIds.size || selectedIds.has(user.id))
      downloadCSV(filename, ['user_id', 'telegram_id', 'first_subscription_at', 'first_plan_id', 'ltv_minor'], rows.map(user => [user.id, user.telegram_id, user.first_subscription_at, user.first_plan_id, user.ltv_minor]))
      toast.success(selectedIds.size ? 'Экспортированы выбранные записи текущей страницы' : 'Экспортирована текущая страница выборки')
    } else if (view === 'generator' || view === 'sources') {
      const rows = report.dataset.samples[sampleKind]
      const headers = [...new Set(rows.flatMap(row => Object.keys(row)))]
      downloadCSV(filename, headers, rows.map(row => headers.map(header => Object.entries(row).find(([key]) => key === header)?.[1])))
      toast.success('Экспортированы образцы выбранного типа записей')
    } else {
      downloadCSV(filename, ['plan_id', 'users_count', 'total_revenue_minor', 'mean_ltv_minor', 'median_ltv_minor'], report.plans.map(row => [row.plan_id, row.users_count, row.total_revenue_minor, row.mean_ltv_minor, row.median_ltv_minor]))
      toast.success('Экспортировано сравнение тарифов текущего сегмента')
    }
  }
  function content(data: Dashboard) {
    if (view === 'users') return <UsersPage report={data} search={search} sort={sort} onSearchChange={value => { setSearch(value); clearSelection() }} onSortChange={value => { setSort(value); clearSelection() }} onPageChange={value => { setPage(value); setSelectedIds(new Set()) }} selectedIds={selectedIds} onSelectionChange={setSelectedIds} isFetching={reportQuery.isFetching}/>
    if (view === 'generator') return <GeneratorPage report={data} onGenerate={generate} isGenerating={generation.isPending} isPreview={data.mode === 'preview'} onExplore={() => openView('overview')} initialConfig={seedDraft} onConfigChange={setSeedDraft} sampleKind={sampleKind} onSampleKindChange={setSampleKind}/>
    if (view === 'sources') return <SourcesPage report={data} status={status} isPreview={data.mode === 'preview'} sampleKind={sampleKind} onSampleKindChange={setSampleKind}/>
    return <AnalyticsPage view={view} report={data} plan={shownFilters?.plan_id ?? filters.plan_id} trend={trend} onTrendChange={setTrend} metric={metric} onMetricChange={setMetric} periods={periods} onPeriodsChange={setPeriods} onMonthClick={openMonth} onPlanClick={openPlan} onNavigate={openView} onUTMClick={utm => { setFilters(current => ({ ...current, utm_link: utm })); clearSelection() }}/>
  }
  return <><AppShell view={view} report={report} status={status} statusOffline={statusQuery.isError} isFetching={reportQuery.isFetching || generation.isPending} onRefresh={refresh} onExport={exportCSV} onReturnLive={returnLive}>
    {view !== 'generator' && view !== 'sources' && <FilterBar appliedFilters={shownFilters} filters={filters} onChange={changeFilters} onReset={resetFilters} report={report} view={view} fetching={reportQuery.isFetching}/>}
    {reportQuery.isError && <div id="error-banner" role="alert" className="mb-5 flex items-start gap-3 rounded-md bg-destructive/5 p-4 text-[13px] text-destructive"><AlertCircle className="size-4 shrink-0"/><div><p>{reportQuery.error.message}</p>{report && <p className="mt-1">Показан последний полученный снимок. Повторите запрос.</p>}<Button variant="link" className="mt-2 h-auto p-0 text-destructive" onClick={refresh}><RefreshCw className="size-3.5"/>Повторить</Button></div></div>}
    <div id="content" aria-busy={reportQuery.isFetching} inert={reportQuery.isPlaceholderData}>{report ? <Suspense fallback={<LoadingDashboard/>}>{content(report)}</Suspense> : reportQuery.isPending ? <LoadingDashboard/> : <div className="rounded-lg border border-dashed border-border px-4 py-12 text-center"><h2 className="font-medium">Данные пока недоступны</h2><p className="mt-2 text-sm text-muted-foreground">Дождитесь первого снимка или повторите запрос.</p></div>}</div>
  </AppShell><Toaster position="bottom-center"/></>
}

import { useState, type ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import { ArrowUpRight, Download, Menu, RefreshCw, Shuffle } from 'lucide-react'
import { views } from '@/lib/navigation'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { number } from '@/lib/format'
import type { Dashboard, ServiceStatus, View } from '@/types'

const reportViews: View[] = ['overview', 'revenue', 'retention', 'cohorts', 'users']
function Mark() { return <svg viewBox="0 0 24 24" fill="none" className="size-6 text-chart-1" aria-hidden="true"><path d="M3 5h18M3 12h13M3 19h7" stroke="currentColor" strokeWidth="3"/><circle cx="21" cy="12" r="1.5" fill="currentColor"/><circle cx="15" cy="19" r="1.5" fill="currentColor"/></svg> }
function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  const links = (items: View[]) => items.map(view => {
    const item = views[view], Icon = item.icon
    return <NavLink key={view} to={'/' + view} data-nav={view} onClick={onNavigate} className={({ isActive }) => `flex min-h-10 items-center gap-2.5 rounded-md px-3 py-2 text-sm ${isActive ? 'bg-sidebar-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground'}`}><Icon className="size-4 shrink-0" aria-hidden="true"/>{item.label}</NavLink>
  })
  return <div className="flex h-full min-h-0 flex-col px-4 pb-4 pt-6">
    <NavLink to="/overview" onClick={onNavigate} className="flex items-center gap-2.5 px-3 text-[22px] font-semibold tracking-tight"><Mark/>Subscope</NavLink>
    <div className="px-3 pb-5 pt-6"><p className="text-[13px] font-medium">Аналитика подписок</p><p className="mt-1 text-xs text-muted-foreground">Демонстрационный проект</p></div>
    <p className="px-3 pb-2 pt-3 text-[13px] text-muted-foreground">Отчёты</p><nav aria-label="Отчёты" className="grid gap-1">{links(reportViews)}</nav>
    <p className="px-3 pb-2 pt-7 text-[13px] text-muted-foreground">Данные</p><nav aria-label="Данные" className="grid gap-1">{links(['generator', 'sources'])}</nav>
    <div className="mt-auto px-3 pb-5 pt-8 text-[13px]"><Shuffle className="mb-3 size-4 text-chart-1" aria-hidden="true"/><p className="font-medium">Проверьте другой seed</p><p className="mb-3 mt-1.5 text-muted-foreground">Создайте набор и сравните его когорты с текущими.</p><NavLink to="/generator" onClick={onNavigate} className="inline-flex items-center gap-1 underline underline-offset-4">Открыть генератор<ArrowUpRight className="size-3.5"/></NavLink></div>
    <div className="mx-3 border-t border-border pt-4 text-xs"><p>Go · PostgreSQL · Redis</p><p className="mt-1 text-muted-foreground">Связанные тестовые данные</p></div>
  </div>
}
interface AppShellProps {
  view: View; report?: Dashboard; status?: ServiceStatus; statusOffline: boolean
  isFetching: boolean; onRefresh: () => void; onExport: () => void; onReturnLive: () => void; children: ReactNode
}
export function AppShell({ view, report, status, statusOffline, isFetching, onRefresh, onExport, onReturnLive, children }: AppShellProps) {
  const [menuOpen, setMenuOpen] = useState(false)
  const page = views[view], preview = report?.mode === 'preview'
  const connection = statusOffline ? 'Статус недоступен' : status?.last_error ? 'Сохранён последний снимок' : status?.sync_in_progress ? 'Обновление снимка' : status?.snapshot_id ? 'Данные обновлены' : 'Загрузка снимка'
  return <div className="min-h-svh bg-background font-sans text-foreground" data-frontend-stack="react-typescript">
    <a href="#main-content" onClick={event => {event.preventDefault(); document.getElementById('main-content')?.focus()}} className="fixed -top-20 left-4 z-50 rounded-md bg-primary px-4 py-3 text-primary-foreground focus:top-2">К аналитике</a>
    <aside className="fixed inset-y-0 left-0 hidden w-[228px] flex-col min-[1101px]:flex"><Navigation/></aside>
    <div className="min-[1101px]:ml-[228px] sm:px-2 min-[1101px]:pl-0">
      <header className="flex h-13 items-center justify-between gap-3 px-4 sm:h-12 sm:px-6">
        <div className="flex min-w-0 items-center gap-2 text-[13px] sm:gap-3">
          <Sheet open={menuOpen} onOpenChange={setMenuOpen}><SheetTrigger asChild><Button id="menu-button" variant="ghost" size="icon" className="-ml-2 size-10 min-[1101px]:hidden" aria-label="Открыть меню"><Menu className="size-5"/></Button></SheetTrigger><SheetContent side="left" className="w-[244px] gap-0 bg-background p-0"><SheetHeader className="sr-only"><SheetTitle>Навигация Subscope</SheetTitle><SheetDescription>Отчёты и управление тестовыми данными</SheetDescription></SheetHeader><Navigation onNavigate={() => setMenuOpen(false)}/></SheetContent></Sheet>
          <NavLink to="/overview" className="text-[15px] font-medium min-[1101px]:hidden">Subscope</NavLink>
          <span className="hidden text-muted-foreground sm:inline">Подписки</span><span className="hidden text-muted-foreground sm:inline">/</span><span id="breadcrumb-title" className="hidden font-medium sm:inline">{page.label}</span>
        </div>
        <span className="flex items-center gap-1.5 whitespace-nowrap text-xs text-muted-foreground"><i className={`size-1.5 rounded-full ${statusOffline || status?.last_error ? 'bg-amber-700' : 'bg-emerald-700'}`}/>{connection}</span>
      </header>
      <main className="min-h-[calc(100svh-56px)] bg-card px-4 pb-4 pt-6 sm:mb-2 sm:rounded-2xl sm:border sm:border-border sm:p-6" id="main-content" tabIndex={-1}>
        <div className="mb-6 flex flex-col justify-between gap-4 md:flex-row md:items-start">
          <div className="min-w-0"><div className="flex flex-wrap items-center gap-x-3 gap-y-2"><h1 id="page-title" className="text-2xl leading-tight font-medium tracking-tight">{page.title}</h1>{report && <NavLink to="/generator" id="mode-badge" className="rounded border border-border px-2 py-1 font-mono text-xs text-muted-foreground hover:border-input">{preview ? 'Предпросмотр' : 'Снимок'} · seed {report.dataset.seed ?? '—'}</NavLink>}</div><p className="mt-2 max-w-[78ch] text-sm leading-relaxed text-muted-foreground">{page.description}</p></div>
          <div className="flex shrink-0 gap-2"><Button id="export-button" variant="outline" className="min-h-11 flex-1 text-[13px] shadow-none sm:min-h-9" onClick={onExport} disabled={!report || isFetching}><Download className="size-4"/>Экспорт CSV</Button><Button id="refresh-button" variant="outline" className="min-h-11 flex-1 text-[13px] shadow-none sm:min-h-9" onClick={onRefresh} disabled={isFetching}><RefreshCw className={`size-4 ${isFetching ? 'animate-spin motion-reduce:animate-none' : ''}`}/>Обновить данные</Button></div>
        </div>
        {preview && <div id="preview-banner" className="mb-5 flex flex-wrap items-center justify-between gap-2 border-l-[3px] border-chart-1 bg-chart-1/5 px-4 py-3 text-[13px]"><p><strong className="font-medium">Предпросмотр набора</strong> · seed {report.dataset.seed}</p><Button id="return-live" variant="link" className="h-auto p-0 text-[13px]" onClick={onReturnLive}>Вернуться к снимку БД<ArrowUpRight className="size-3.5"/></Button></div>}
        {children}
        <footer className="mt-7 flex flex-wrap justify-between gap-2 border-t border-border pt-4 text-xs text-muted-foreground"><span>Subscope · Аналитика тестовых данных</span><span>{report ? preview ? `Предпросмотр · ${report.dataset.fingerprint.slice(0, 12)}` : `Снимок #${report.snapshot_id} · ${number(report.dataset.users_count)} пользователей` : 'Ожидание снимка'}</span></footer>
      </main>
    </div>
  </div>
}

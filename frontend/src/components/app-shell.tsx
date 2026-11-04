import { useEffect, useState, type ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { ArrowUpRight, BookOpen, ChartNoAxesCombined, ChevronRight, Database, Download, Menu, Moon, RefreshCw, Search, Shuffle, Sun } from 'lucide-react'
import { views } from '@/lib/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { number } from '@/lib/format'
import type { DesignVariant } from '@/lib/design'
import type { Dashboard, ServiceStatus, View } from '@/types'

const tools = [
  { label: 'Swagger', href: '/docs/', icon: BookOpen },
  { label: 'Grafana', href: '/grafana/', icon: ChartNoAxesCombined },
  { label: 'pgAdmin', href: '/db/', icon: Database },
]
function Mark() {
  return <svg viewBox="0 0 28 28" fill="none" className="brand-mark" aria-hidden="true"><path d="M5 5h18v5H10v4h13v9H5v-5h13v-4H5V5Z" fill="currentColor"/></svg>
}
function Navigation({ onNavigate }: { onNavigate?: () => void }) {
  return <nav className="report-navigation" aria-label="Разделы аналитики">{Object.entries(views).map(([id, item]) => <NavLink key={id} to={'/' + id} data-nav={id} onClick={onNavigate} className={({ isActive }) => `report-nav-link ${isActive ? 'is-active' : ''}`}><item.icon size={15} strokeWidth={1.6} aria-hidden="true"/><span>{item.label}</span></NavLink>)}</nav>
}
function SnapshotClock({ status, offline, preview }: { status?: ServiceStatus; offline: boolean; preview: boolean }) {
  const [now, setNow] = useState(Date.now)
  useEffect(() => { const interval = window.setInterval(() => setNow(Date.now()), 1000); return () => window.clearInterval(interval) }, [])
  const next = status?.next_sync_at ? Date.parse(status.next_sync_at) : NaN
  const last = status?.last_success_at ? Date.parse(status.last_success_at) : NaN
  const remaining = Math.max(0, Math.ceil((next - now) / 1000))
  const countdown = Number.isFinite(remaining) ? `${String(Math.floor(remaining / 60)).padStart(2, '0')}:${String(remaining % 60).padStart(2, '0')}` : '—'
  const fraction = Number.isFinite(next - last) && next > last ? Math.max(0, Math.min(1, (next - now) / (next - last))) : 0
  const message = offline ? 'Статус недоступен' : status?.sync_in_progress ? 'Обновляем рабочий снимок' : status?.last_error ? 'Сохранён последний успешный снимок' : next <= now ? 'Ожидаем следующий опрос' : `Следующий опрос через ${countdown}`
  const stamp = status?.data_as_of ? new Date(status.data_as_of).toLocaleString('ru-RU', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit', timeZone: 'UTC' }) + ' UTC' : 'Ожидаем первый снимок'
  return <div className={`snapshot-clock ${offline || status?.last_error ? 'has-warning' : ''}`} aria-label="Состояние синхронизации"><svg viewBox="0 0 44 44" aria-hidden="true"><circle cx="22" cy="22" r="18" fill="none" stroke="var(--border)" strokeWidth="3"/><circle cx="22" cy="22" r="18" fill="none" stroke="var(--signal)" strokeWidth="3" strokeLinecap="round" strokeDasharray="113.1" strokeDashoffset={113.1 * (1 - fraction)} transform="rotate(-90 22 22)"/></svg><span><strong>{preview ? 'Предпросмотр данных' : stamp}</strong><small>{message}</small></span></div>
}
interface AppShellProps {
  design: DesignVariant; onDesignChange: (variant: DesignVariant) => void
  view: View; report?: Dashboard; status?: ServiceStatus; statusOffline: boolean
  isFetching: boolean; onRefresh: () => void; onExport: () => void; onReturnLive: () => void; children: ReactNode
}
export function AppShell({ design, onDesignChange, view, report, status, statusOffline, isFetching, onRefresh, onExport, onReturnLive, children }: AppShellProps) {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [commandOpen, setCommandOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [commandIndex, setCommandIndex] = useState(0)
  const page = views[view], preview = report?.mode === 'preview'
  const commands = Object.entries(views).filter(([, item]) => `${item.label} ${item.title}`.toLocaleLowerCase('ru').includes(query.toLocaleLowerCase('ru')))
  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setQuery(''); setCommandIndex(0); setCommandOpen(open => !open) } }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [])
  const openCommand = () => { setMenuOpen(false); setCommandOpen(true); setQuery(''); setCommandIndex(0) }
  const chooseCommand = (id: string) => { void navigate('/' + id); setCommandOpen(false) }
  return <div className="design-app" data-frontend-stack="react-typescript" data-design-variant={design}>
    <a href="#main-content" onClick={event => { event.preventDefault(); document.getElementById('main-content')?.focus() }} className="skip-link">К аналитике</a>
    <div className="design-container">
      <header className="masthead"><NavLink to="/overview" className="design-brand"><Mark/><span>Subscope<small>Аналитика подписок</small></span></NavLink><SnapshotClock status={status} offline={statusOffline} preview={preview}/><div className="masthead-tools">{tools.map(tool => <a key={tool.href} href={tool.href} title={tool.label} target="_blank" rel="noopener noreferrer"><tool.icon size={15}/><span>{tool.label}</span><ArrowUpRight size={11}/></a>)}</div><Button className="appearance-toggle" variant="outline" size="icon" aria-label={design === 'a' ? 'Светлое оформление' : 'Тёмное оформление'} title={design === 'a' ? 'Светлое оформление' : 'Тёмное оформление'} onClick={() => onDesignChange(design === 'a' ? 'b' : 'a')}>{design === 'a' ? <Sun size={16}/> : <Moon size={16}/>}</Button></header>
      <div className="report-navigation-row"><div className="desktop-report-nav"><Navigation/></div><div className="phone-report-nav"><Sheet open={menuOpen} onOpenChange={setMenuOpen}><SheetTrigger asChild><Button id="menu-button" variant="outline" aria-label="Открыть меню"><Menu size={17}/><span>{page.label}</span></Button></SheetTrigger><SheetContent side="left" className="design-menu gap-0 bg-background p-6"><SheetHeader className="px-0 pb-6"><SheetTitle>Subscope</SheetTitle><SheetDescription>Аналитика подписок</SheetDescription></SheetHeader><Navigation onNavigate={() => setMenuOpen(false)}/><div className="menu-tools">{tools.map(tool => <a key={tool.href} href={tool.href} target="_blank" rel="noopener noreferrer">{tool.label}<ArrowUpRight size={14}/></a>)}</div></SheetContent></Sheet></div><button type="button" className="report-search" aria-label="Найти раздел" onClick={openCommand}><Search size={15}/><kbd>Ctrl K</kbd></button></div>
      <main className={`report-workspace ${view === 'overview' ? 'is-overview' : ''}`} id="main-content" tabIndex={-1}>
        <div className="report-title-row"><div><div className="report-edition">{report && <NavLink to="/generator" id="mode-badge" className="edition-snapshot">{preview ? `Предпросмотр · seed ${report.dataset.seed ?? '—'}` : `Снимок #${report.snapshot_id}`}</NavLink>}</div><h1 id="page-title">{page.title}</h1></div><div className="report-actions"><Button id="refresh-button" variant="outline" size="icon" aria-label="Обновить данные" title="Обновить данные" onClick={onRefresh} disabled={isFetching}><RefreshCw className={`size-4 ${isFetching ? 'animate-spin motion-reduce:animate-none' : ''}`}/></Button><Button id="export-button" onClick={onExport} disabled={!report || isFetching}><Download size={15}/><span>Excel-отчёт</span></Button></div></div>
        {preview && <div id="preview-banner" className="preview-notice"><span><Shuffle size={15}/><strong>Предпросмотр набора</strong><span>seed {report.dataset.seed}</span></span><Button id="return-live" variant="link" onClick={onReturnLive}>Вернуться к снимку БД<ArrowUpRight size={14}/></Button></div>}
        {children}
        <footer className="report-footer"><span>RUB</span><span>{report ? `${preview ? 'Предпросмотр' : 'Снимок #' + report.snapshot_id} · ${number(report.dataset.users_count)} пользователей` : 'Ожидание данных'}</span><NavLink to="/sources">Источники и синхронизация<ArrowUpRight size={13}/></NavLink></footer>
      </main>
    </div>
    <Dialog open={commandOpen} onOpenChange={setCommandOpen}><DialogContent className="command-dialog p-0" showCloseButton={false}><DialogHeader className="sr-only"><DialogTitle>Быстрый переход</DialogTitle><DialogDescription>Разделы аналитики</DialogDescription></DialogHeader><div className="command-search"><Search size={19}/><Input autoFocus placeholder="Поиск раздела" aria-label="Найти раздел" role="combobox" aria-expanded="true" aria-controls="command-options" aria-activedescendant={commands[commandIndex] ? `command-${commands[commandIndex][0]}` : undefined} value={query} onChange={event => { setQuery(event.target.value); setCommandIndex(0) }} onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); setCommandIndex(index => Math.min(index + 1, commands.length - 1)) } else if (event.key === 'ArrowUp') { event.preventDefault(); setCommandIndex(index => Math.max(index - 1, 0)) } else if (event.key === 'Enter' && commands[commandIndex]) chooseCommand(commands[commandIndex][0]) }}/><kbd>Esc</kbd></div><div id="command-options" role="listbox" className="command-options">{commands.map(([id, item], index) => <button key={id} id={`command-${id}`} role="option" aria-selected={index === commandIndex} onClick={() => chooseCommand(id)} onMouseMove={() => setCommandIndex(index)}><item.icon size={17}/><span>{item.label}</span><ChevronRight size={14}/></button>)}{!commands.length && <p className="command-empty">Нет результатов</p>}</div></DialogContent></Dialog>
  </div>
}

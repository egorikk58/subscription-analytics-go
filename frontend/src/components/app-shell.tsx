import { useEffect, useState, type ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { ArrowUpRight, BookOpen, ChartNoAxesCombined, Check, ChevronRight, Database, Download, Menu, RefreshCw, Search, Shuffle } from 'lucide-react'
import { views } from '@/lib/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { number, time } from '@/lib/format'
import type { Dashboard, ServiceStatus, View } from '@/types'

const reportViews: View[] = ['overview', 'revenue', 'retention', 'cohorts', 'users']
const tools = [
  { label: 'API и Swagger', href: '/docs/', icon: BookOpen },
  { label: 'Grafana', href: '/grafana/', icon: ChartNoAxesCombined },
  { label: 'База данных', href: '/db/', icon: Database },
]
function Mark() {
  return <svg viewBox="0 0 28 28" fill="none" className="brand-mark" aria-hidden="true"><path d="M5 5h18v5H10v4h13v9H5v-5h13v-4H5V5Z" fill="currentColor"/></svg>
}
function Navigation({ report, onNavigate, onSearch }: { report?: Dashboard; onNavigate?: () => void; onSearch: () => void }) {
  const links = (items: View[]) => items.map(view => {
    const item = views[view], Icon = item.icon
    return <NavLink key={view} to={'/' + view} data-nav={view} onClick={onNavigate} className={({ isActive }) => `nav-link ${isActive ? 'is-active' : ''}`}><Icon size={17} strokeWidth={1.65} aria-hidden="true"/><span>{item.label}</span>{view === 'generator' && <span className="nav-hint">seed</span>}</NavLink>
  })
  return <div className="navigation-body">
    <NavLink to="/overview" onClick={onNavigate} className="brand"><Mark/><span>Subscope<span className="brand-caption">Аналитика подписок</span></span></NavLink>
    <button className="navigation-search" onClick={onSearch}><Search size={15}/><span>Найти раздел</span><kbd>Ctrl K</kbd></button>
    <div className="nav-group-label">Аналитика</div><nav aria-label="Отчёты">{links(reportViews)}</nav>
    <div className="nav-group-label">Данные</div><nav aria-label="Данные">{links(['generator', 'sources'])}</nav>
    <div className="navigation-tools"><div className="nav-group-label">Инструменты</div>{tools.map(tool => <a key={tool.href} className="nav-link" href={tool.href} target="_blank" rel="noopener noreferrer"><tool.icon size={16} strokeWidth={1.65}/><span>{tool.label}</span><ArrowUpRight size={13} className="nav-external"/></a>)}</div>
    <NavLink to="/generator" onClick={onNavigate} className="dataset-switch"><span className="dataset-avatar"><Shuffle size={17}/></span><span><strong>{report?.mode === 'preview' ? 'Тестовый набор' : 'Рабочий набор'}</strong><small>{report ? `${number(report.dataset.users_count)} пользователей` : 'Ожидание данных'}</small></span><ChevronRight size={15}/></NavLink>
  </div>
}
interface AppShellProps {
  view: View; report?: Dashboard; status?: ServiceStatus; statusOffline: boolean
  isFetching: boolean; onRefresh: () => void; onExport: () => void; onReturnLive: () => void; children: ReactNode
}
export function AppShell({ view, report, status, statusOffline, isFetching, onRefresh, onExport, onReturnLive, children }: AppShellProps) {
  const navigate = useNavigate()
  const [menuOpen, setMenuOpen] = useState(false)
  const [commandOpen, setCommandOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [commandIndex, setCommandIndex] = useState(0)
  const page = views[view], preview = report?.mode === 'preview'
  const connection = statusOffline ? 'Статус недоступен' : status?.last_error ? 'Сохранён последний снимок' : status?.sync_in_progress ? 'Синхронизация' : status?.snapshot_id ? 'Данные обновлены' : 'Загрузка снимка'
  const commands = Object.entries(views).filter(([, item]) => `${item.label} ${item.title}`.toLocaleLowerCase('ru').includes(query.toLocaleLowerCase('ru')))
  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => { if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setCommandOpen(open => !open) } }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [])
  const openCommand = () => { setMenuOpen(false); setCommandOpen(true); setQuery(''); setCommandIndex(0) }
  const chooseCommand = (id: string) => { void navigate('/' + id); setCommandOpen(false) }
  return <div className="app-layout" data-frontend-stack="react-typescript">
    <a href="#main-content" onClick={event => { event.preventDefault(); document.getElementById('main-content')?.focus() }} className="skip-link">К аналитике</a>
    <aside className="desktop-navigation"><Navigation report={report} onSearch={openCommand}/></aside>
    <div className="mobile-topbar"><Sheet open={menuOpen} onOpenChange={setMenuOpen}><SheetTrigger asChild><Button id="menu-button" variant="ghost" size="icon" aria-label="Открыть меню"><Menu size={20}/></Button></SheetTrigger><SheetContent side="left" className="w-[280px] gap-0 bg-background p-0"><SheetHeader className="sr-only"><SheetTitle>Навигация Subscope</SheetTitle><SheetDescription>Отчёты и управление тестовыми данными</SheetDescription></SheetHeader><Navigation report={report} onNavigate={() => setMenuOpen(false)} onSearch={openCommand}/></SheetContent></Sheet><NavLink to="/overview" className="mobile-brand"><Mark/>Subscope</NavLink><span className="mobile-connection"><i className={statusOffline || status?.last_error ? 'status-warning' : ''}/>{preview ? 'Предпросмотр' : 'Снимок БД'}</span></div>
    <main className="workspace" id="main-content" tabIndex={-1}>
      <header className="page-header">
        <div className="page-heading"><div className="page-breadcrumb"><span>Аналитика</span><ChevronRight size={12}/><span id="breadcrumb-title">{page.label}</span></div><div className="page-title-row"><h1 id="page-title">{page.title}</h1>{report && <NavLink to="/generator" id="mode-badge" className="snapshot-tag">{preview ? 'Предпросмотр' : 'Снимок'} <span>· seed {report.dataset.seed ?? '—'}</span></NavLink>}</div><p className="page-description">{page.description}</p></div>
        <div className="page-actions"><div className="sync-status" title={status?.last_success_at ? `Успешный опрос в ${time(status.last_success_at)}` : undefined}><i className={statusOffline || status?.last_error ? 'status-warning' : ''}/>{connection}</div><div className="page-action-buttons"><Button id="refresh-button" variant="outline" size="icon" aria-label="Обновить данные" title="Обновить данные" onClick={onRefresh} disabled={isFetching}><RefreshCw className={`size-4 ${isFetching ? 'animate-spin motion-reduce:animate-none' : ''}`}/></Button><Button id="export-button" variant="default" className="export-action" onClick={onExport} disabled={!report || isFetching}><Download size={15}/><span>Экспорт CSV</span></Button></div></div>
      </header>
      {preview && <div id="preview-banner" className="preview-notice"><span><Shuffle size={15}/><strong>Предпросмотр набора</strong><span>seed {report.dataset.seed}</span></span><Button id="return-live" variant="link" onClick={onReturnLive}>Вернуться к снимку БД<ArrowUpRight size={14}/></Button></div>}
      {children}
      <footer className="workspace-footer"><span><Check size={12}/>Все суммы в RUB после конвертации и возвратов</span><span>{report ? preview ? `Предпросмотр · ${report.dataset.fingerprint.slice(0, 8)}` : `Снимок #${report.snapshot_id} · ${time(report.data_as_of)}` : 'Ожидание снимка'}</span></footer>
    </main>
    <Dialog open={commandOpen} onOpenChange={setCommandOpen}><DialogContent className="command-dialog p-0" showCloseButton={false}><DialogHeader className="sr-only"><DialogTitle>Быстрый переход</DialogTitle><DialogDescription>Найдите раздел. Стрелки выбирают результат, Enter открывает его.</DialogDescription></DialogHeader><div className="command-search"><Search size={19}/><Input autoFocus placeholder="Куда перейдём?" aria-label="Найти раздел" role="combobox" aria-expanded="true" aria-controls="command-options" aria-activedescendant={commands[commandIndex] ? `command-${commands[commandIndex][0]}` : undefined} value={query} onChange={event => { setQuery(event.target.value); setCommandIndex(0) }} onKeyDown={event => { if (event.key === 'ArrowDown') { event.preventDefault(); setCommandIndex(index => Math.min(index + 1, commands.length - 1)) } else if (event.key === 'ArrowUp') { event.preventDefault(); setCommandIndex(index => Math.max(index - 1, 0)) } else if (event.key === 'Enter' && commands[commandIndex]) chooseCommand(commands[commandIndex][0]) }}/><kbd>Esc</kbd></div><div id="command-options" role="listbox" className="command-options">{commands.map(([id, item], index) => <button key={id} id={`command-${id}`} role="option" aria-selected={index === commandIndex} onClick={() => chooseCommand(id)} onMouseMove={() => setCommandIndex(index)}><item.icon size={17}/><span>{item.label}<small>{item.description}</small></span><ChevronRight size={14}/></button>)}{!commands.length && <p className="command-empty">Такого раздела нет. Попробуйте «Когорты» или «Генератор».</p>}</div><div className="command-footer">↑ ↓ выбрать<span>Enter открыть</span></div></DialogContent></Dialog>
  </div>
}

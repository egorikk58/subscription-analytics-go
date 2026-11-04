import { useEffect, useRef, useState } from 'react'
import { Download, LoaderCircle } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { NativeSelect } from '@/components/ui/native-select'
import { downloadWorkbook, type ExportContext, type ReportKind } from '@/lib/excel'
import { date, number } from '@/lib/format'

export function ExportDialog({ context, onClose }: { context: ExportContext; onClose: () => void }) {
  const [kind, setKind] = useState<ReportKind>('analytics')
  const [selected, setSelected] = useState(context.selectedIds.length > 0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const controller = useRef<AbortController | null>(null)
  useEffect(() => () => controller.current?.abort(), [])
  async function download() {
    if (busy) return
    const operation = new AbortController()
    controller.current = operation; setBusy(true); setError('')
    try {
      await downloadWorkbook(context, kind, selected, operation.signal)
      if (!operation.signal.aborted) { toast.success('Excel-отчёт скачан'); onClose() }
    } catch (failure) {
      if (!operation.signal.aborted) { setError(failure instanceof Error ? failure.message : 'Не удалось скачать отчёт'); setBusy(false) }
    }
  }
  const total = selected ? context.selectedIds.length : context.report.users.total
  return <Dialog open onOpenChange={open => { if (!open) onClose() }}>
    <DialogContent className="export-dialog gap-5 border-border bg-background text-foreground shadow-none">
      <DialogHeader><DialogTitle>Экспорт в Excel</DialogTitle><DialogDescription className="sr-only">Параметры отчёта по подпискам</DialogDescription></DialogHeader>
      <div className="grid gap-4">
        <label className="grid gap-2 text-[13px]" htmlFor="export-kind">Отчёт<NativeSelect id="export-kind" wrapperClassName="w-full" disabled={busy} value={kind} onChange={event => setKind(event.target.value as ReportKind)}><option value="analytics">Аналитический отчёт — 7 листов</option><option value="users">Реестр пользователей</option></NativeSelect></label>
        <label className="grid gap-2 text-[13px]" htmlFor="export-scope">Пользователи<NativeSelect id="export-scope" wrapperClassName="w-full" disabled={busy} value={selected ? 'selected' : 'all'} onChange={event => setSelected(event.target.value === 'selected')}><option value="all">Вся выборка · {number(context.report.users.total)}</option>{context.selectedIds.length > 0 && <option value="selected">Отмеченные · {number(context.selectedIds.length)}</option>}</NativeSelect></label>
        <dl className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-3 border-y border-border py-4 text-[13px]"><dt className="text-muted-foreground">Набор</dt><dd className="text-right">{context.report.mode === 'preview' ? `Seed ${context.report.dataset.seed}` : `Снимок #${context.report.snapshot_id}`}</dd><dt className="text-muted-foreground">Данные на</dt><dd>{date(context.report.data_as_of)}</dd><dt className="text-muted-foreground">Пользователей</dt><dd className="text-right tabular-nums">{number(total)}</dd><dt className="text-muted-foreground">Формат</dt><dd className="text-right">.xlsx</dd></dl>
        {error && <p role="alert" className="text-[13px] text-destructive">{error}</p>}
      </div>
      <DialogFooter><Button variant="outline" onClick={onClose}>{busy ? 'Отменить' : 'Закрыть'}</Button><Button id="download-excel" onClick={() => { void download() }} disabled={busy} aria-busy={busy}>{busy ? <LoaderCircle size={16} className="animate-spin"/> : <Download size={16}/>}<span>{busy ? 'Формируем отчёт…' : 'Скачать Excel'}</span></Button></DialogFooter>
    </DialogContent>
  </Dialog>
}

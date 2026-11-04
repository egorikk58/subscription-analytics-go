import { Panel } from '@/components/analytics/panel'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { money, number } from '@/lib/format'
import type { Dashboard } from '@/types'

export function UTMComparison({ rows = [], onSelect }: { rows: Dashboard['utm_groups']; onSelect?: (utm: string) => void }) {
  return <Panel title="Источники трафика" description="UTM пользователя · пожизненная выручка в RUB после конвертации и возвратов">
    <div className="overflow-x-auto"><Table><TableHeader><TableRow><TableHead>Источник</TableHead><TableHead className="text-right">Пользователи</TableHead><TableHead className="text-right">Выручка</TableHead><TableHead className="text-right">Средний LTV</TableHead><TableHead className="text-right">Медиана</TableHead></TableRow></TableHeader><TableBody>{rows.map(row => <TableRow key={row.utm_link}><TableCell><Button variant="link" className="h-auto max-w-64 justify-start whitespace-normal p-0 text-left" onClick={() => onSelect?.(row.utm_link)} disabled={!onSelect}>{row.utm_link === 'unknown' ? 'Без UTM' : row.utm_link}</Button></TableCell><TableCell className="text-right tabular-nums">{number(row.users_count)}</TableCell><TableCell className="text-right tabular-nums">{money(row.total_revenue_minor, 2)}</TableCell><TableCell className="text-right tabular-nums">{money(row.mean_ltv_minor, 2)}</TableCell><TableCell className="text-right tabular-nums">{money(row.median_ltv_minor, 2)}</TableCell></TableRow>)}{!rows.length && <TableRow><TableCell colSpan={5} className="py-8 text-center text-muted-foreground">Нет источников в выбранной когорте</TableCell></TableRow>}</TableBody></Table></div>
  </Panel>
}

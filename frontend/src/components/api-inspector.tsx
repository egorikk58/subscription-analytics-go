import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Panel } from '@/components/analytics/panel'

const routes = ['/metrics/ltv/summary', '/metrics/ltv/by-plan', '/metrics/ltv/by-utm', '/metrics/retention?plan_id=monthly&max_periods=12', '/users/by-telegram/1000000000/ltv', '/status', '/healthz']
interface Result { route: string; status: number; headers: Record<string, string>; body: string }
export function ApiInspector() {
  const [route, setRoute] = useState(routes[0]!)
  const [result, setResult] = useState<Result>()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const controller = useRef<AbortController | null>(null)
  useEffect(() => () => controller.current?.abort(), [])
  async function run(conditional: boolean) {
    const request = new AbortController()
    controller.current = request
    setLoading(true); setError('')
    try {
      const etag = conditional && result?.route === route ? result.headers.etag : undefined
      const response = await fetch(route, { signal: request.signal, cache: 'no-store', headers: etag ? { 'If-None-Match': etag } : {} })
      const text = await response.text()
      const headers = Object.fromEntries(['etag', 'last-modified', 'cache-control', 'retry-after', 'x-data-as-of', 'x-snapshot-version'].flatMap(key => { const value = response.headers.get(key); return value ? [[key, value]] : [] }))
      let body = text
      try { body = text ? JSON.stringify(JSON.parse(text), null, 2) : 'Тело ответа отсутствует' } catch { /* Display the actual server body. */ }
      setResult({ route, status: response.status, headers, body })
    } catch (cause) { if (!request.signal.aborted) setError(cause instanceof Error ? cause.message : 'Не удалось выполнить запрос') }
    finally { if (!request.signal.aborted) setLoading(false) }
  }
  return <Panel title="HTTP API">
    <div className="space-y-4 px-4 pb-5 sm:px-5"><div className="flex flex-wrap items-center gap-3"><NativeSelect aria-label="Маршрут API" value={route} disabled={loading} onChange={event => setRoute(event.target.value)} className="max-w-full font-mono text-xs" wrapperClassName="max-w-full">{routes.map(path => <NativeSelectOption key={path} value={path}>{path}</NativeSelectOption>)}</NativeSelect><Button variant="outline" onClick={() => void run(false)} disabled={loading}>Выполнить GET</Button><Button variant="outline" onClick={() => void run(true)} disabled={loading || result?.route !== route || !result.headers.etag}>Проверить 304</Button></div>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    {result && <div className="space-y-3" aria-live="polite"><p className="text-sm font-medium">HTTP {result.status} <span className="break-all font-mono text-xs font-normal">{result.route}</span></p><dl className="grid gap-2 text-xs sm:grid-cols-[150px_1fr]">{Object.entries(result.headers).map(([key, value]) => <div key={key} className="contents"><dt className="font-mono text-muted-foreground">{key}</dt><dd className="break-all font-mono">{value}</dd></div>)}</dl><pre className="max-h-80 overflow-auto rounded-md bg-muted p-3 font-mono text-xs whitespace-pre-wrap break-all">{result.body}</pre></div>}
    </div>
  </Panel>
}

import { afterEach, expect, it, vi } from 'vitest'
import { downloadWorkbook, type ExportContext } from './excel'
import { defaultParams } from './api'
import { makeDashboard } from '@/test/fixtures'

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

it('pins the visible snapshot, exports checked users and preserves UTC filter bounds without pagination', async () => {
  const response = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'Снимок удалён' }), { status: 410, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', response)
  const context: ExportContext = { report: makeDashboard(), params: { ...defaultParams, page: 4, from: '2024-02-01', to: '2024-02-29', search: '1000', utm_link: 'campaign-1' }, source: { mode: 'live' }, selectedIds: ['first', 'second'] }
  await expect(downloadWorkbook(context, 'users', true, new AbortController().signal)).rejects.toThrow('Снимок удалён')
  const [url, options] = response.mock.calls[0] as [string, RequestInit]
  const query = new URL(url, 'http://localhost').searchParams
  expect(query.get('snapshot_id')).toBe('1')
  expect(query.get('fingerprint')).toBe(context.report.dataset.fingerprint)
  expect(query.getAll('user_id')).toEqual(['first', 'second'])
  expect(query.get('kind')).toBe('users')
  expect(query.has('page')).toBe(false)
  expect(query.get('to')).toBe('2024-03-01T00:00:00.000Z')
  expect(query.get('search')).toBe('1000')
  expect(query.get('utm_link')).toBe('campaign-1')
  expect(options.method).toBe('GET')
})

it('exports the preview configuration and never substitutes the database snapshot', async () => {
  const fetch = vi.fn().mockResolvedValue(new Response('wrong content', { status: 200, headers: { 'Content-Type': 'text/html' } }))
  vi.stubGlobal('fetch', fetch)
  const report = makeDashboard(); report.mode = 'preview'; report.snapshot_id = null
  const config = { seed: 731, history_months: 24, data_as_of: '2026-11-01T00:00:00Z' }
  await expect(downloadWorkbook({ report, params: defaultParams, source: { mode: 'preview', config }, selectedIds: [] }, 'analytics', false, new AbortController().signal)).rejects.toThrow('неизвестном формате')
  const [url, options] = fetch.mock.calls[0] as [string, RequestInit]
  expect(new URL(url, 'http://localhost').searchParams.has('snapshot_id')).toBe(false)
  expect(options.method).toBe('POST')
  expect(JSON.parse(options.body as string)).toEqual(config)
})

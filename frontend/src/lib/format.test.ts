import { describe, expect, it } from 'vitest'
import { money, percent } from './format'
import { csvText } from './csv'
import { dashboardKey, defaultParams, queryString } from './api'

const plain = (value: string) => value.replaceAll('\u00a0', ' ')
describe('money display preserves exact minor-unit rounding', () => {
  it('displays fractional mean, half ties and negative values without floating point money', () => {
    expect(plain(money('656762.90', 2))).toBe('6 567,63 ₽')
    expect(plain(money('150.50', 2))).toBe('1,51 ₽')
    expect(plain(money('-150.50', 2))).toBe('−1,51 ₽')
    expect(plain(money('150'))).toBe('2 ₽')
    expect(plain(money('-150'))).toBe('−2 ₽')
    expect(money('invalid')).toBe('—')
  })
  it('handles zero and maximum int64 encoded as a decimal string', () => {
    expect(money('-0.49', 2)).toBe('0,00 ₽')
    expect(plain(money('9223372036854775807', 2))).toBe('92 233 720 368 547 758,07 ₽')
    expect(percent(null)).toBe('—')
    expect(plain(percent(0))).toBe('0 %')
  })
})
describe('query boundaries and cache identity', () => {
  it('converts inclusive UTC calendar dates to exclusive upper boundary including leap February', () => {
    const query = new URLSearchParams(queryString({ ...defaultParams, from: '2024-02-01', to: '2024-02-29' }))
    expect(query.get('from')).toBe('2024-02-01T00:00:00Z')
    expect(query.get('to')).toBe('2024-03-01T00:00:00.000Z')
  })
  it('separates seed previews and cohort-chain selection from live first-plan queries', () => {
    const live = dashboardKey({ mode: 'live' }, defaultParams)
    expect(dashboardKey({ mode: 'live' }, { ...defaultParams, selection: 'chain' })).not.toEqual(live)
    expect(dashboardKey({ mode: 'preview', config: { seed: 731, users_count: 15000, history_months: 24, data_as_of: '2026-11-01T00:00:00Z' } }, defaultParams)).not.toEqual(live)
  })
})
it('escapes CSV values and prevents formula evaluation', () => {
  expect(csvText(['id', 'value'], [['=SUM(A1)', 'with "quotes"; and\nline']])).toBe('\uFEFF"id";"value"\r\n"\'=SUM(A1)";"with ""quotes""; and\nline"')
})

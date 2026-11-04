import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { TelegramLookup } from '@/components/telegram-lookup'
import { assetMoney } from '@/lib/format'
import { defaultParams, queryString } from '@/lib/api'

afterEach(() => { cleanup(); vi.unstubAllGlobals() })

describe('bonus contracts', () => {
  it('preserves native atomic units and encodes UTM without mixing filters', () => {
    expect(assetMoney(1, 'BTC')).toBe('0,00000001 BTC')
    expect(assetMoney(1234567, 'USDT')).toBe('1,234567 USDT')
    const params = new URLSearchParams(queryString({ ...defaultParams, utm_link: 'campaign/a?b=1' }))
    expect(params.get('utm_link')).toBe('campaign/a?b=1')
  })
  it('passes the full int64 Telegram ID as text and shows the exact JSON', async () => {
    const body = '{"telegram_id":9223372036854775807,"ltv":100}'
    const request = vi.fn().mockResolvedValue({ ok: true, text: async () => body })
    vi.stubGlobal('fetch', request)
    render(<TelegramLookup initialID="9223372036854775807" />)
    fireEvent.click(screen.getByRole('button', { name: 'Получить LTV' }))
    await waitFor(() => expect(screen.getByLabelText('Ответ LTV пользователя').textContent).toBe(body))
    expect(request.mock.calls[0]?.[0]).toBe('/users/by-telegram/9223372036854775807/ltv')
  })
  it('renders unknown-user feedback and does not submit overflowing IDs', async () => {
    const request = vi.fn().mockResolvedValue({ ok: false, status: 404, text: async () => '{}' })
    vi.stubGlobal('fetch', request)
    render(<TelegramLookup initialID="1" />)
    fireEvent.click(screen.getByRole('button', { name: 'Получить LTV' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Пользователь не найден')
    fireEvent.change(screen.getByLabelText('Telegram ID для LTV'), { target: { value: '9223372036854775808' } })
    fireEvent.click(screen.getByRole('button', { name: 'Получить LTV' }))
    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('int64'))
    expect(request).toHaveBeenCalledTimes(1)
  })
})

import { useState, type FormEvent } from 'react'
import { Panel } from '@/components/analytics/panel'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export function TelegramLookup({ initialID = '' }: { initialID?: string }) {
  const [id, setID] = useState(initialID)
  const [pending, setPending] = useState(false)
  const [body, setBody] = useState('')
  const [error, setError] = useState('')
  const lookup = async (value: string) => {
    if (!/^[0-9]{1,19}$/.test(value) || BigInt(value) <= 0n || BigInt(value) > 9223372036854775807n) throw new Error('Введите положительный Telegram ID в диапазоне int64')
    const response = await fetch(`/users/by-telegram/${encodeURIComponent(value)}/ltv`, { cache: 'no-store' })
    const body = await response.text()
    if (!response.ok) throw new Error(response.status === 404 ? 'Пользователь не найден' : response.status === 503 ? 'Опубликованный снимок ещё не готов' : 'Не удалось загрузить LTV')
    return body
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault(); setPending(true); setBody(''); setError('')
    try { setBody(await lookup(id.trim())) } catch (error) { setError(error instanceof Error ? error.message : 'Не удалось загрузить LTV') } finally { setPending(false) }
  }
  return <Panel title="LTV по Telegram ID" description="Опубликованный снимок · все оплаты и возвраты за жизнь пользователя · RUB в копейках">
    <form onSubmit={event => void submit(event)} className="flex flex-wrap gap-3 px-4 pb-5 sm:px-5"><Input aria-label="Telegram ID для LTV" value={id} onChange={event => setID(event.target.value)} inputMode="numeric" placeholder="Например, 1000000000" className="min-w-0 flex-1 sm:max-w-72"/><Button type="submit" disabled={pending}>{pending ? 'Загружаем…' : 'Получить LTV'}</Button></form>
    {error && <p role="alert" className="px-5 pb-5 text-sm text-destructive">{error}</p>}
    {body && <pre aria-label="Ответ LTV пользователя" className="mx-4 mb-5 whitespace-pre-wrap break-all rounded-md bg-muted p-4 text-xs leading-relaxed sm:mx-5">{body}</pre>}
  </Panel>
}

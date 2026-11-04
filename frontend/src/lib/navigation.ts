import { ChartNoAxesCombined, CircleDollarSign, Database, LayoutDashboard, Shuffle, Table2, Users, type LucideIcon } from 'lucide-react'
import type { View } from '@/types'

export const views: Record<View, { label: string; title: string; description: string; icon: LucideIcon }> = {
  overview: { label: 'Обзор', title: 'Обзор подписок', description: 'Выручка, ценность пользователей и продления — в одном месте.', icon: LayoutDashboard },
  revenue: { label: 'Выручка и LTV', title: 'Что приносит выручку', description: 'Сравнивайте тарифы и пожизненную ценность пользователей.', icon: CircleDollarSign },
  retention: { label: 'Продления', title: 'Как продлевают подписки', description: 'Условная и кумулятивная доли по оплаченным периодам.', icon: ChartNoAxesCombined },
  cohorts: { label: 'Когорты', title: 'Удержание в разрезе когорт', description: 'От первой подписки к каждому следующему продлению.', icon: Table2 },
  users: { label: 'Выборка', title: 'Пользователи за цифрами', description: 'Исследуйте сегмент, сортируйте по LTV и открывайте конкретные записи.', icon: Users },
  generator: { label: 'Генератор', title: 'Лаборатория тестовых данных', description: 'Создайте воспроизводимый набор и посмотрите, как меняется аналитика.', icon: Shuffle },
  sources: { label: 'Источники и снимки', title: 'От данных к аналитике', description: 'Состав набора, история опроса и согласованный снимок.', icon: Database },
}

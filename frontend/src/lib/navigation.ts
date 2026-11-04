import { ChartNoAxesCombined, CircleDollarSign, Database, LayoutDashboard, Shuffle, Table2, Users, type LucideIcon } from 'lucide-react'
import type { View } from '@/types'

export const views: Record<View, { label: string; title: string; icon: LucideIcon }> = {
  overview: { label: 'Обзор', title: 'Обзор подписок', icon: LayoutDashboard },
  revenue: { label: 'Выручка и LTV', title: 'Выручка и LTV', icon: CircleDollarSign },
  retention: { label: 'Продления', title: 'Продления подписок', icon: ChartNoAxesCombined },
  cohorts: { label: 'Когорты', title: 'Анализ когорт', icon: Table2 },
  users: { label: 'Выборка', title: 'Пользователи', icon: Users },
  generator: { label: 'Генератор', title: 'Генератор данных', icon: Shuffle },
  sources: { label: 'Источники и снимки', title: 'Источники и синхронизация', icon: Database },
}

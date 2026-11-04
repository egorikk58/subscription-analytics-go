import { ChartNoAxesCombined, CircleDollarSign, Database, LayoutDashboard, Shuffle, Table2, Users, type LucideIcon } from 'lucide-react'
import type { View } from '@/types'

export const views: Record<View, { label: string; title: string; description: string; icon: LucideIcon }> = {
  overview: { label: 'Обзор', title: 'Обзор подписок', description: 'Пожизненная выручка и продления четырёх подписочных тарифов.', icon: LayoutDashboard },
  revenue: { label: 'Выручка и LTV', title: 'Выручка и LTV', description: 'Каждый пользователь остаётся в когорте своего первого тарифа.', icon: CircleDollarSign },
  retention: { label: 'Продления', title: 'Продления подписок', description: 'Доли продлений среди пользователей с завершённым периодом.', icon: ChartNoAxesCombined },
  cohorts: { label: 'Когорты', title: 'Анализ когорт', description: 'Удержание по месяцу первой подписки и номеру оплаченного периода.', icon: Table2 },
  users: { label: 'Выборка', title: 'Пользователи', description: 'Пожизненная ценность, тариф и источник каждого пользователя.', icon: Users },
  generator: { label: 'Генератор', title: 'Генератор данных', description: 'Воспроизводимые наборы с оплатами, возвратами и переходами между тарифами.', icon: Shuffle },
  sources: { label: 'Источники и снимки', title: 'Источники и синхронизация', description: 'Состояние источников и актуальность опубликованного снимка.', icon: Database },
}

'use client'

import { ReactNode } from 'react'
import { StatCard } from '@/components/ui/StatCard'

type ColorVariant = 'primary' | 'success' | 'warning' | 'destructive' | 'accent' | 'muted'

export interface StatItem {
  /** Label for the stat */
  label: string
  /** Value to display */
  value: number | string
  /** Icon element */
  icon: ReactNode
  /** Optional link */
  href?: string
  /** Color variant */
  variant?: ColorVariant
}

interface StatsGridProps {
  /** Array of stats to display */
  stats: StatItem[]
  /** Number of columns on large screens (default: 4) */
  columns?: 3 | 4
}

/**
 * Grid layout for displaying stat cards.
 * Automatically handles responsive layout and animation delays.
 */
export function StatsGrid({ stats, columns = 4 }: StatsGridProps) {
  const gridClass =
    columns === 3
      ? 'grid grid-cols-1 gap-5 sm:grid-cols-3'
      : 'grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4'

  return (
    <div className={`mb-8 ${gridClass}`}>
      {stats.map((stat, index) => (
        <StatCard
          key={stat.label}
          label={stat.label}
          value={stat.value}
          icon={stat.icon}
          href={stat.href}
          variant={stat.variant}
          delay={index}
        />
      ))}
    </div>
  )
}

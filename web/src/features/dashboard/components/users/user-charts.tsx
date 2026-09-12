/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { Users, Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  ToggleGroup,
  ToggleGroupItem,
} from '@/components/ui/toggle-group'
import { useThemeCustomization } from '@/context/theme-customization-provider'
import { useTheme } from '@/context/theme-provider'
import { getUserQuotaDataByUsers } from '@/features/dashboard/api'
import {
  DASHBOARD_QUICK_RANGE_PRESETS,
  TIME_GRANULARITY_OPTIONS,
} from '@/features/dashboard/constants'
import {
  buildQuickRangeDashboardFilters,
  getDefaultDays,
  saveGranularity,
  processUserChartData,
} from '@/features/dashboard/lib'
import type {
  ProcessedUserChartData,
  UserChartsFilters,
} from '@/features/dashboard/types'
import { requireServerSuccess } from '@/lib/server-error-message'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'

let themeManagerPromise: Promise<
  (typeof import('@visactor/vchart'))['ThemeManager']
> | null = null

const USER_CHARTS: {
  value: string
  labelKey: string
  specKey: keyof ProcessedUserChartData
}[] = [
  {
    value: 'rank',
    labelKey: 'User Consumption Ranking',
    specKey: 'spec_user_rank',
  },
  {
    value: 'trend',
    labelKey: 'User Consumption Trend',
    specKey: 'spec_user_trend',
  },
]

const TOP_USER_LIMIT_OPTIONS = [5, 10, 20, 50]
const RANK_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const

function isRankPlaceholder(datum: Record<string, unknown>) {
  return Boolean(datum.__rankPlaceholder)
}

function getRankValues(spec: unknown): Record<string, unknown>[] {
  const values = (spec as { data?: Array<{ values?: unknown[] }> })?.data?.[0]
    ?.values
  return Array.isArray(values)
    ? (values.filter(
        (item): item is Record<string, unknown> =>
          Boolean(item) &&
          typeof item === 'object' &&
          !isRankPlaceholder(item as Record<string, unknown>)
      ) as Record<string, unknown>[])
    : []
}

function getRankValueLabel(
  spec: unknown,
  value: number,
  datum: Record<string, unknown>
) {
  const formatter = (spec as {
    label?: {
      formatMethod?: (
        value: number,
        datum?: Record<string, unknown>
      ) => string
    }
  })?.label?.formatMethod
  return formatter ? formatter(value, datum) : String(value)
}

function getRankBadgeClass(rank: number) {
  if (rank === 1) return 'border-primary/30 bg-primary/5 text-primary'
  if (rank === 2) return 'border-muted-foreground/25 bg-muted text-foreground'
  if (rank === 3) return 'border-accent/40 bg-accent/20 text-accent-foreground'
  return 'border-border bg-background text-muted-foreground'
}

function UserRankList({ spec }: { spec: unknown }) {
  const rows = getRankValues(spec)
  const maxValue = Math.max(
    ...rows.map((item) => Number(item.rawQuota) || 0),
    1
  )
  const colorMap =
    (spec as { color?: { specified?: Record<string, string> } })?.color
      ?.specified ?? {}

  return (
    <ScrollArea className='h-full pr-3'>
      <div className='flex flex-col gap-1.5 pb-2'>
        {rows.map((item, index) => {
          const name = String(item.User || '-')
          const rawValue = Number(item.rawQuota) || 0
          const width = `${Math.max((rawValue / maxValue) * 100, 2)}%`
          const color = colorMap[name] || RANK_COLORS[index % RANK_COLORS.length]

          return (
            <div
              key={name}
              className='grid min-h-8 items-center gap-2 rounded-md border border-transparent px-2 py-1 transition-[background-color,border-color] hover:border-border hover:bg-muted/35'
              style={{
                gridTemplateColumns:
                  'minmax(6.5rem, 9rem) minmax(5rem, 1fr) max-content',
              }}
            >
              <div className='flex min-w-0 items-center gap-2'>
                <span
                  className={`flex size-5 shrink-0 items-center justify-center rounded-md border text-[11px] font-medium tabular-nums ${getRankBadgeClass(index + 1)}`}
                >
                  {index + 1}
                </span>
                <span className='truncate text-sm font-medium' title={name}>
                  {name}
                </span>
              </div>
              <div className='bg-muted h-2.5 overflow-hidden rounded-full'>
                <div
                  className='h-full rounded-full'
                  style={{ width, backgroundColor: color }}
                />
              </div>
              <div className='text-muted-foreground min-w-16 text-right text-[11px] font-medium tabular-nums'>
                {getRankValueLabel(spec, rawValue, item)}
              </div>
            </div>
          )
        })}
      </div>
    </ScrollArea>
  )
}

interface UserChartsProps {
  filters: UserChartsFilters
  onFiltersChange: (filters: UserChartsFilters) => void
}

export function UserCharts(props: UserChartsProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { resolvedTheme } = useTheme()
  const { customization } = useThemeCustomization()
  const [themeReady, setThemeReady] = useState(false)
  const themeManagerRef = useRef<
    (typeof import('@visactor/vchart'))['ThemeManager'] | null
  >(null)

  // The selection is owned by the dashboard parent so it persists across
  // sub-section switches; the rolling window is derived from the chosen range.
  const timeGranularity = props.filters.timeGranularity
  const selectedRange = props.filters.selectedRange
  const topUserLimit = props.filters.topUserLimit
  const activeQuickRange = props.filters.activeQuickRange
  const onFiltersChange = props.onFiltersChange

  const timeRange = useMemo(() => {
    if (activeQuickRange) {
      const result = buildQuickRangeDashboardFilters(activeQuickRange, {
        time_granularity: timeGranularity,
      })
      const start = result.filters.start_timestamp as Date
      const end = result.filters.end_timestamp as Date
      return {
        start_timestamp: Math.floor(start.getTime() / 1000),
        end_timestamp: Math.floor(end.getTime() / 1000),
      }
    }

    const { start, end } = getRollingDateRange(selectedRange)
    return {
      start_timestamp: Math.floor(start.getTime() / 1000),
      end_timestamp: Math.floor(end.getTime() / 1000),
    }
  }, [activeQuickRange, selectedRange, timeGranularity])

  const handleQuickRangeChange = useCallback(
    (value: string[]) => {
      const next = value.find((item) => item !== activeQuickRange)
      if (!next) return

      const result = buildQuickRangeDashboardFilters(next, {
        time_granularity: timeGranularity,
      })

      const nextGranularity =
        result.filters.time_granularity || timeGranularity
      onFiltersChange({
        ...props.filters,
        activeQuickRange: result.presetKey,
        timeGranularity: nextGranularity,
        selectedRange: getDefaultDays(nextGranularity),
      })
    },
    [activeQuickRange, onFiltersChange, props.filters, timeGranularity]
  )

  const handleGranularityChange = useCallback(
    (g: TimeGranularity) => {
      saveGranularity(g)
      onFiltersChange({
        ...props.filters,
        activeQuickRange: null,
        timeGranularity: g,
        selectedRange: getDefaultDays(g),
      })
    },
    [onFiltersChange, props.filters]
  )

  const handleTopUserLimitChange = useCallback(
    (limit: number) => {
      onFiltersChange({ ...props.filters, topUserLimit: limit })
    },
    [onFiltersChange, props.filters]
  )

  useEffect(() => {
    const updateTheme = async () => {
      setThemeReady(false)
      if (!themeManagerPromise) {
        themeManagerPromise = import('@visactor/vchart').then(
          (m) => m.ThemeManager
        )
      }
      const ThemeManager = await themeManagerPromise
      themeManagerRef.current = ThemeManager
      ThemeManager.setCurrentTheme(resolvedTheme === 'dark' ? 'dark' : 'light')
      setThemeReady(true)
    }
    updateTheme()
  }, [resolvedTheme])

  const { data: userData, isLoading } = useQuery({
    queryKey: ['dashboard', 'user-quota', timeRange],
    queryFn: async () =>
      requireServerSuccess(await getUserQuotaDataByUsers(timeRange)),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const handleRefresh = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['dashboard', 'user-quota'] })
  }, [queryClient])

  const chartData = useMemo(
    () =>
      processUserChartData(
        isLoading ? [] : (userData ?? []),
        timeGranularity,
        t,
        topUserLimit,
        customization.preset
      ),
    [
      userData,
      isLoading,
      timeGranularity,
      t,
      topUserLimit,
      customization.preset,
    ]
  )

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-1.5 pb-1 sm:gap-2'>
        <ToggleGroup
          value={activeQuickRange ? [activeQuickRange] : []}
          onValueChange={handleQuickRangeChange}
          aria-label={t('Quick Range')}
          variant='outline'
          size='sm'
          spacing={0}
          className='max-w-full overflow-x-auto'
        >
          {DASHBOARD_QUICK_RANGE_PRESETS.map((preset) => (
            <ToggleGroupItem
              key={preset.key}
              value={preset.key}
              className={cn(
                'px-2.5 text-xs',
                activeQuickRange === preset.key && 'bg-muted'
              )}
            >
              {t(preset.label)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>

        <Button variant='outline' size='sm' onClick={handleRefresh}>
          <RefreshCw data-icon='inline-start' />
          {t('Refresh')}
        </Button>

        <Tabs
          value={timeGranularity}
          onValueChange={(value) =>
            handleGranularityChange(value as TimeGranularity)
          }
          className='shrink-0'
        >
          <TabsList>
            {TIME_GRANULARITY_OPTIONS.map((opt) => (
              <TabsTrigger
                key={opt.value}
                value={opt.value}
                className='px-2.5 text-xs'
              >
                {t(opt.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={String(topUserLimit)}
          onValueChange={(value) => handleTopUserLimitChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            <span className='text-muted-foreground px-2 text-xs font-medium whitespace-nowrap'>
              {t('Top Users')}
            </span>
            {TOP_USER_LIMIT_OPTIONS.map((limit) => (
              <TabsTrigger
                key={limit}
                value={String(limit)}
                className='px-2.5 text-xs'
              >
                {t('Top {{count}}', { count: limit })}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {isLoading && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>

      <div className='grid gap-3'>
        {USER_CHARTS.map((chart) => {
          const spec = chartData[chart.specKey]
          const isRankChart = chart.value === 'rank'
          const chartElement =
            themeReady &&
            spec && (
              <VChart
                key={`user-${chart.value}-${topUserLimit}-${resolvedTheme}-${customization.preset}`}
                spec={{
                  ...spec,
                  theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                  background: 'transparent',
                }}
                option={VCHART_OPTION}
              />
            )
          let chartContent = chartElement || null
          if (isLoading) {
            chartContent = <Skeleton className='h-full w-full' />
          } else if (isRankChart) {
            chartContent = <UserRankList spec={spec} />
          }

          return (
            <div
              key={chart.value}
              className='overflow-hidden rounded-lg border'
            >
              <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
                <IconBadge tone='info' size='sm'>
                  <Users />
                </IconBadge>
                <div className='text-sm font-semibold'>{t(chart.labelKey)}</div>
              </div>

              <div
                className={
                  isRankChart
                    ? 'h-[240px] p-1.5 sm:h-[280px] sm:p-2'
                    : 'h-[300px] p-1.5 sm:h-96 sm:p-2'
                }
              >
                {chartContent}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

import { useQuery } from '@tanstack/react-query'
import { KeyRound, Loader2 } from 'lucide-react'
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
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { getTokenRanking } from '@/features/dashboard/api'
import { buildQueryParams, getDefaultDays } from '@/features/dashboard/lib'
import type {
  DashboardFilters,
  DashboardTimeRange,
} from '@/features/dashboard/types'
import { requireServerSuccess } from '@/lib/server-error-message'
import { computeTimeRange } from '@/lib/time'

const RANK_COLORS = [
  'var(--chart-1)',
  'var(--chart-2)',
  'var(--chart-3)',
  'var(--chart-4)',
  'var(--chart-5)',
] as const

interface TokenRankingPanelProps {
  filters?: DashboardFilters
  timeRange?: DashboardTimeRange
}

function formatTokenMillions(value: number | null | undefined): string {
  const numeric = Number(value) || 0
  return `${(numeric / 1_000_000).toFixed(2)}M`
}

function maskUsername(username: string | null | undefined, fallback: string) {
  const value = username || fallback
  if (value.includes('***')) return value

  const chars = [...value]
  if (chars.length <= 1) return `${value}***`
  if (chars.length === 2) return `${chars.at(0)}***`
  return `${chars.at(0)}***${chars.at(-1)}`
}

function getRankBadgeClass(rank: number) {
  if (rank === 1) return 'border-primary/30 bg-primary/5 text-primary'
  if (rank === 2) return 'border-muted-foreground/25 bg-muted text-foreground'
  if (rank === 3) return 'border-accent/40 bg-accent/20 text-accent-foreground'
  return 'border-border bg-background text-muted-foreground'
}

export function TokenRankingPanel(props: TokenRankingPanelProps) {
  const { t } = useTranslation()

  const queryParams = useMemo(() => {
    const timeRange =
      props.timeRange ??
      computeTimeRange(
        getDefaultDays(props.filters?.time_granularity),
        props.filters?.start_timestamp,
        props.filters?.end_timestamp
      )
    const params = buildQueryParams(timeRange, props.filters)
    return {
      start_timestamp: params.start_timestamp,
      end_timestamp: params.end_timestamp,
      username: params.username,
    }
  }, [props.filters, props.timeRange])

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['dashboard', 'token-ranking', queryParams],
    queryFn: async () =>
      requireServerSuccess(await getTokenRanking(queryParams)),
    select: (res) => res.data,
    staleTime: 60_000,
  })

  const ranking = data?.ranking
  const isLimited = Boolean(data?.is_limited)

  const rankRows = useMemo(() => {
    const maskedNames = new Map<string, number>()
    return (ranking ?? []).map((item, index) => {
      let displayName =
        isLimited && !item.is_self
          ? maskUsername(item.username, t('User'))
          : item.username || t('User')

      if (isLimited && !item.is_self) {
        const count = (maskedNames.get(displayName) ?? 0) + 1
        maskedNames.set(displayName, count)
        if (count > 1) displayName = `${displayName} #${count}`
      }

      return {
        name: displayName,
        rank: item.rank ?? index + 1,
        tokens: item.token_used || 0,
        isSelf: item.is_self,
        color: RANK_COLORS[index % RANK_COLORS.length],
      }
    })
  }, [isLimited, ranking, t])

  const totalTokens = rankRows.reduce(
    (sum, item) => sum + (Number(item.tokens) || 0),
    0
  )
  const maxTokens = Math.max(...rankRows.map((item) => item.tokens), 1)

  let body: ReactNode
  if (isLoading) {
    body = (
      <div className='p-1.5 sm:p-2'>
        <Skeleton className='h-[240px] w-full sm:h-[280px]' />
      </div>
    )
  } else if (isError || !data) {
    body = (
      <ErrorState
        title={t('Failed to load token ranking')}
        className='min-h-40'
        onRetry={() => void refetch()}
      />
    )
  } else if ((ranking?.length ?? 0) === 0) {
    body = (
      <div className='text-muted-foreground flex h-40 items-center justify-center text-sm'>
        {t('No ranking data available')}
      </div>
    )
  } else {
    body = (
      <div className='p-1.5 sm:p-2'>
        <ScrollArea className='h-[240px] pr-3 sm:h-[280px]'>
          <div className='flex flex-col gap-1.5 pb-2'>
            <div className='text-muted-foreground flex items-center justify-between px-2 pb-1 text-xs'>
              <span>{t('Token Usage Ranking')}</span>
              <span>{`${t('Total:')} ${formatTokenMillions(totalTokens)}`}</span>
            </div>
            {rankRows.map((item) => {
              const width = `${Math.max((item.tokens / maxTokens) * 100, 2)}%`
              return (
                <div
                  key={`${item.rank}-${item.name}`}
                  className='hover:border-border hover:bg-muted/35 grid min-h-8 items-center gap-2 rounded-md border border-transparent px-2 py-1 transition-[background-color,border-color]'
                  style={{
                    gridTemplateColumns:
                      'minmax(6.5rem, 9rem) minmax(5rem, 1fr) max-content',
                  }}
                >
                  <div className='flex min-w-0 items-center gap-2'>
                    <span
                      className={`flex size-5 shrink-0 items-center justify-center rounded-md border text-[11px] font-medium tabular-nums ${getRankBadgeClass(item.rank)}`}
                    >
                      {item.rank}
                    </span>
                    <span
                      className='truncate text-sm font-medium'
                      title={item.name}
                    >
                      {item.name}
                    </span>
                  </div>
                  <div className='bg-muted h-2.5 overflow-hidden rounded-full'>
                    <div
                      className='h-full rounded-full'
                      style={{ width, backgroundColor: item.color }}
                    />
                  </div>
                  <div className='text-muted-foreground min-w-16 text-right text-[11px] font-medium tabular-nums'>
                    {formatTokenMillions(item.tokens)}
                  </div>
                </div>
              )
            })}
          </div>
        </ScrollArea>
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-col gap-2 border-b px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between sm:px-5 sm:py-3'>
        <div className='flex min-w-0 items-center gap-2'>
          <KeyRound className='text-muted-foreground/60 size-4 shrink-0' />
          <div className='truncate text-sm font-semibold'>
            {t('Token Usage Ranking')}
          </div>
          {isLoading && (
            <Loader2 className='text-muted-foreground size-3.5 animate-spin' />
          )}
        </div>
        <div className='flex flex-wrap items-center gap-1.5'>
          <Badge variant='outline'>
            {data?.is_limited
              ? t('Top {{count}}', { count: data.limit || 10 })
              : t('All Rankings')}
          </Badge>
          <Badge variant='secondary'>
            {t('Users: {{count}}', { count: data?.total_users ?? 0 })}
          </Badge>
          {data?.self_rank ? (
            <Badge variant='outline'>
              {t('My Rank: {{rank}}', { rank: data.self_rank })}
            </Badge>
          ) : null}
        </div>
      </div>

      {body}
    </div>
  )
}

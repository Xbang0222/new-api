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
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useModelStatCardsConfig } from '@/features/dashboard/hooks/use-dashboard-config'
import { calculateDashboardStats } from '@/features/dashboard/lib/stats'
import type {
  QuotaDataItem,
  DashboardTimeRange,
} from '@/features/dashboard/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatNumber, formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

interface LogStatCardsProps {
  data: QuotaDataItem[]
  timeRange: DashboardTimeRange
  dataUpdatedAt: number
  loading: boolean
  error: boolean
}

const MAX_INLINE_STAT_CHARS = 7

function formatStatNumber(value: number, locale: Intl.LocalesArgument) {
  const fullValue = formatNumber(value, locale)
  const displayValue =
    fullValue.length > MAX_INLINE_STAT_CHARS
      ? formatCompactNumber(value, locale)
      : fullValue

  return {
    displayValue,
    fullValue,
  }
}

export function LogStatCards(props: LogStatCardsProps) {
  const { t, i18n } = useTranslation()
  const statCardsConfig = useModelStatCardsConfig()
  const stats = calculateDashboardStats(props.data)
  // The inclusive query includes the whole last stored hour. Only the current
  // hour is partial, capped at the data snapshot so cached rates stay stable.
  const effectiveStart =
    Math.floor(props.timeRange.start_timestamp / 3600) * 3600
  const effectiveEnd = Math.min(
    (Math.floor(props.timeRange.end_timestamp / 3600) + 1) * 3600,
    props.dataUpdatedAt / 1000
  )
  const timeRangeMinutes = Math.max(0, effectiveEnd - effectiveStart) / 60

  const adaptedStats = {
    rpm: stats?.totalCount ?? 0,
    quota: stats?.totalQuota ?? 0,
    tpm: stats?.totalTokens ?? 0,
    cacheRead: stats?.totalCacheReadTokens ?? 0,
    cacheCreation: stats?.totalCacheCreationTokens ?? 0,
  }

  const items = statCardsConfig.map((config) => {
    const rawValue = config.getValue(adaptedStats, timeRangeMinutes)
    const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
    const formatted =
      config.key === 'quota'
        ? {
            displayValue: formatQuota(rawValue),
            fullValue: formatQuota(rawValue),
          }
        : formatStatNumber(rawValue, locale)

    return {
      key: config.key,
      title: config.title,
      value: formatted.displayValue,
      fullValue: formatted.fullValue,
      desc: config.description,
      icon: config.icon,
      iconTone: config.iconTone,
    }
  })

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid min-w-0 grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-7'>
        {items.map((it, idx) => {
          const Icon = it.icon
          let valueContent
          if (props.loading) {
            valueContent = (
              <div className='mt-1 flex flex-col gap-1 sm:mt-2 sm:gap-1.5'>
                <Skeleton className='h-5 w-16 sm:h-7 sm:w-20' />
                <Skeleton className='hidden h-3.5 w-28 md:block' />
              </div>
            )
          } else if (props.error) {
            valueContent = (
              <>
                <div className='text-muted-foreground mt-1 font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl sm:leading-normal'>
                  --
                </div>
                <div className='text-muted-foreground/40 mt-1 hidden text-xs md:block'>
                  {it.desc}
                </div>
              </>
            )
          } else {
            valueContent = (
              <>
                <div
                  className='text-foreground mt-1 max-w-full truncate font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl sm:leading-normal'
                  title={it.fullValue}
                >
                  {it.value}
                </div>
                <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                  {it.desc}
                </div>
              </>
            )
          }

          return (
            <div
              key={it.key}
              role='group'
              aria-label={it.title}
              aria-busy={props.loading}
              className={cn(
                'min-w-0 px-2.5 py-1.5 sm:px-3 sm:py-4 xl:px-4',
                idx === items.length - 1 &&
                  items.length % 2 !== 0 &&
                  'col-span-2 sm:col-span-1'
              )}
            >
              <div className='flex min-w-0 items-center gap-1.5 sm:gap-2'>
                <IconBadge
                  tone={it.iconTone}
                  size='stat'
                  className='size-4 rounded-sm sm:size-7 sm:rounded-md [&>svg]:size-2.5 sm:[&>svg]:size-3.5'
                >
                  <Icon />
                </IconBadge>
                <div
                  className='text-muted-foreground truncate text-[11px] leading-4 font-medium tracking-wide uppercase sm:text-xs sm:tracking-wider'
                  title={it.title}
                >
                  {it.title}
                </div>
              </div>

              {valueContent}
            </div>
          )
        })}
      </div>
      <p className='text-muted-foreground border-t px-3 py-2 text-xs'>
        {t(
          'Historical cache totals may be incomplete when older versions did not collect cache usage. Updates do not reset existing totals.'
        )}
      </p>
    </div>
  )
}

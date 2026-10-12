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
import { useQuery } from '@tanstack/react-query'
import { Trophy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { IconBadge } from '@/components/ui/icon-badge'
import { getInvitationRanking } from '@/features/dashboard/api'
import { toIntlLocale } from '@/i18n/languages'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { PanelWrapper } from '../ui/panel-wrapper'

const PODIUM_STYLES = [
  {
    position: 'col-start-3 row-start-1',
    step: 'h-24 border-amber-400/40 bg-amber-400/15 text-amber-800 dark:text-amber-200',
    avatar:
      'border-amber-400/50 bg-amber-400/15 text-amber-800 dark:text-amber-200',
  },
  {
    position: 'col-start-2 row-start-1',
    step: 'h-20 border-slate-400/40 bg-slate-400/15 text-slate-700 dark:text-slate-200',
    avatar:
      'border-slate-400/50 bg-slate-400/15 text-slate-700 dark:text-slate-200',
  },
  {
    position: 'col-start-4 row-start-1',
    step: 'h-16 border-orange-400/40 bg-orange-400/15 text-orange-800 dark:text-orange-200',
    avatar:
      'border-orange-400/50 bg-orange-400/15 text-orange-800 dark:text-orange-200',
  },
  {
    position: 'col-start-1 row-start-1',
    step: 'h-12 border-blue-400/30 bg-blue-400/10 text-blue-800 dark:text-blue-200',
    avatar:
      'border-blue-400/40 bg-blue-400/10 text-blue-800 dark:text-blue-200',
  },
  {
    position: 'col-start-5 row-start-1',
    step: 'h-8 border-violet-400/30 bg-violet-400/10 text-violet-800 dark:text-violet-200',
    avatar:
      'border-violet-400/40 bg-violet-400/10 text-violet-800 dark:text-violet-200',
  },
] as const

export function InvitationRankingPanel() {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const currency = useSystemConfigStore((state) => state.config.currency)
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const rankingQuery = useQuery({
    queryKey: [
      'dashboard',
      'overview',
      'invitation-ranking',
      user?.id,
      user?.role,
    ],
    queryFn: async () => requireServerSuccess(await getInvitationRanking()),
    staleTime: 60 * 1000,
    retry: false,
  })
  const ranking = rankingQuery.data?.data ?? []

  return (
    <PanelWrapper
      title={
        <span className='flex items-center gap-2'>
          <IconBadge tone='warning' size='sm'>
            <Trophy aria-hidden='true' />
          </IconBadge>
          {t('Invitation earnings leaderboard')}
        </span>
      }
      description={t('Top 5 by lifetime invitation earnings')}
      loading={rankingQuery.isPending}
      empty={!rankingQuery.isError && ranking.length === 0}
      emptyMessage={t('No invitation earnings yet')}
      contentClassName='flex flex-col'
    >
      {rankingQuery.isError ? (
        <ErrorState
          title={t('Unable to load invitation rankings')}
          onRetry={() => void rankingQuery.refetch()}
          className='min-h-0 p-0'
        />
      ) : (
        <ol
          aria-label={t('Invitation earnings leaderboard')}
          className='grid h-full grid-cols-5 items-end gap-1 sm:gap-2'
        >
          {ranking.slice(0, 5).map((entry, index) => {
            const style = PODIUM_STYLES[index]
            const earnings = formatBillingCurrencyFromUSD(
              entry.aff_history_quota / currency.quotaPerUnit,
              { abbreviate: false, locale }
            )

            return (
              <li
                key={entry.rank}
                className={cn(
                  'flex min-w-0 flex-col items-center',
                  style.position
                )}
              >
                <span className='sr-only'>
                  {t('Rank {{rank}}', { rank: index + 1 })}
                </span>
                {index === 0 && (
                  <Trophy
                    aria-hidden='true'
                    className='mb-1 size-4 text-amber-600 dark:text-amber-300'
                  />
                )}
                <Avatar
                  className={cn(
                    'mb-1 size-7 sm:size-8',
                    entry.is_self && 'ring-primary/50 ring-2'
                  )}
                >
                  <AvatarFallback
                    className={cn('border font-semibold', style.avatar)}
                  >
                    {[...entry.username].slice(0, 2).join('').toUpperCase()}
                  </AvatarFallback>
                </Avatar>
                <span
                  title={entry.username}
                  className='w-full truncate text-center text-xs font-medium'
                >
                  {entry.username}
                </span>
                {entry.is_self && (
                  <span className='text-primary text-[10px]'>{t('Me')}</span>
                )}
                <span
                  title={earnings}
                  className='text-muted-foreground mt-1 mb-2 w-full truncate text-center text-xs tabular-nums'
                >
                  {earnings}
                </span>
                <div
                  aria-hidden='true'
                  className={cn(
                    'flex w-full items-center justify-center rounded-t-xl border border-b-0 text-2xl font-bold tabular-nums',
                    style.step
                  )}
                >
                  {index + 1}
                </div>
              </li>
            )
          })}
        </ol>
      )}
    </PanelWrapper>
  )
}

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

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import type { UserSubscription } from '@/features/subscriptions/types'
import { formatQuota } from '@/lib/format'

interface SubscriptionSummaryProps {
  subscription: UserSubscription
  planTitle: string
}

export function SubscriptionSummary(props: SubscriptionSummaryProps) {
  const { t } = useTranslation()
  const subscription = props.subscription
  const totalAmount = Number(subscription.amount_total || 0)
  const usedAmount = Number(subscription.amount_used || 0)
  const remainAmount = Math.max(0, totalAmount - usedAmount)
  const usagePercent =
    totalAmount > 0 ? Math.round((usedAmount / totalAmount) * 100) : 0
  const now = Date.now() / 1000
  const isActive =
    subscription.status === 'active' && subscription.end_time > now
  const isCancelled = subscription.status === 'cancelled'
  const remainDays = Math.max(
    0,
    Math.ceil((subscription.end_time - now) / 86400)
  )
  const nextResetTime = subscription.next_reset_time ?? 0
  let statusLabel = t('Expired')
  let endTimeLabel = t('Expired at')
  if (isActive) {
    statusLabel = t('Active')
    endTimeLabel = t('Until')
  } else if (isCancelled) {
    statusLabel = t('Cancelled')
    endTimeLabel = t('Cancelled at')
  }

  return (
    <div className='min-w-0 flex-1 text-xs'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex min-w-0 flex-wrap items-center gap-2'>
          <h4 className='min-w-0 font-medium wrap-anywhere'>
            {props.planTitle && `${props.planTitle} · `}
            {t('Subscription')} #{subscription.id}
          </h4>
          <StatusBadge
            label={statusLabel}
            variant={isActive ? 'success' : 'neutral'}
            copyable={false}
          />
        </div>
        {isActive && (
          <span className='text-muted-foreground'>
            {t('{{count}} days remaining', { count: remainDays })}
          </span>
        )}
      </div>
      <div className='text-muted-foreground mt-1.5'>
        {endTimeLabel} {new Date(subscription.end_time * 1000).toLocaleString()}
      </div>
      {isActive && nextResetTime > 0 && (
        <div className='text-muted-foreground mt-1'>
          {t('Next reset')}: {new Date(nextResetTime * 1000).toLocaleString()}
        </div>
      )}
      <div className='text-muted-foreground mt-1'>
        {t('Total Quota')}:{' '}
        {totalAmount > 0 ? (
          <Tooltip>
            <TooltipTrigger render={<span className='cursor-help' />}>
              {formatQuota(usedAmount)}/{formatQuota(totalAmount)} ·{' '}
              {t('Remaining')} {formatQuota(remainAmount)}
            </TooltipTrigger>
            <TooltipContent>
              {t('Raw Quota')}: {usedAmount}/{totalAmount} · {t('Remaining')}{' '}
              {remainAmount}
            </TooltipContent>
          </Tooltip>
        ) : (
          t('Unlimited')
        )}
        {totalAmount > 0 && (
          <span className='ml-2'>
            {t('Used')} {usagePercent}%
          </span>
        )}
      </div>
      {totalAmount > 0 && isActive && (
        <Progress value={usagePercent} className='mt-2 h-1.5' />
      )}
    </div>
  )
}

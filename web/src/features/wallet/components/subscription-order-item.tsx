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
import ArrowDown01Icon from '@hugeicons/core-free-icons/ArrowDown01Icon'
import ArrowUp01Icon from '@hugeicons/core-free-icons/ArrowUp01Icon'
import Drag01Icon from '@hugeicons/core-free-icons/Drag01Icon'
import { HugeiconsIcon } from '@hugeicons/react'
import { Reorder, useDragControls } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import type { UserSubscription } from '@/features/subscriptions/types'

import { SubscriptionSummary } from './subscription-summary'

interface SubscriptionOrderItemProps {
  subscription: UserSubscription
  planTitle: string
  index: number
  count: number
  disabled: boolean
  onMove: (index: number, offset: number) => void
}

export function SubscriptionOrderItem(props: SubscriptionOrderItemProps) {
  const { t } = useTranslation()
  const dragControls = useDragControls()
  return (
    <Reorder.Item
      value={props.subscription.id}
      dragListener={false}
      dragControls={dragControls}
      drag={props.disabled ? false : 'y'}
      className='bg-background relative rounded-md border p-3'
    >
      <div className='mb-2 flex items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            className='text-muted-foreground cursor-grab touch-none active:cursor-grabbing'
            disabled={props.disabled || props.count < 2}
            aria-label={t('Drag subscription #{{id}} to reorder', {
              id: props.subscription.id,
            })}
            onPointerDown={(event) => {
              if (!props.disabled && props.count > 1) dragControls.start(event)
            }}
            onKeyDown={(event) => {
              if (props.disabled) return
              if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
                event.preventDefault()
                props.onMove(props.index, event.key === 'ArrowUp' ? -1 : 1)
              }
            }}
          >
            <HugeiconsIcon
              icon={Drag01Icon}
              strokeWidth={2}
              aria-hidden='true'
            />
          </Button>
          <span className='text-muted-foreground text-xs tabular-nums'>
            {t('Debit priority {{position}}', { position: props.index + 1 })}
          </span>
        </div>
        <div className='flex shrink-0 gap-1'>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            disabled={props.disabled || props.index === 0}
            aria-label={t('Move subscription #{{id}} up', {
              id: props.subscription.id,
            })}
            onClick={() => props.onMove(props.index, -1)}
          >
            <HugeiconsIcon
              icon={ArrowUp01Icon}
              strokeWidth={2}
              aria-hidden='true'
            />
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            disabled={props.disabled || props.index === props.count - 1}
            aria-label={t('Move subscription #{{id}} down', {
              id: props.subscription.id,
            })}
            onClick={() => props.onMove(props.index, 1)}
          >
            <HugeiconsIcon
              icon={ArrowDown01Icon}
              strokeWidth={2}
              aria-hidden='true'
            />
          </Button>
        </div>
      </div>
      <SubscriptionSummary
        subscription={props.subscription}
        planTitle={props.planTitle}
      />
    </Reorder.Item>
  )
}

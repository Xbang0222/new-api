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
import { useMutation } from '@tanstack/react-query'
import { Reorder } from 'motion/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import { Spinner } from '@/components/ui/spinner'
import { updateSubscriptionOrder } from '@/features/subscriptions/api'
import type {
  SelfSubscriptionData,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { handleServerError } from '@/lib/handle-server-error'

import { SubscriptionOrderItem } from './subscription-order-item'
import { SubscriptionSummary } from './subscription-summary'

interface SubscriptionOrderEditorProps {
  activeSubscriptions: UserSubscriptionRecord[]
  allSubscriptions: UserSubscriptionRecord[]
  nowSeconds?: number
  savedOrder: number[]
  planTitles: Map<number, string>
  onSaved: (data: SelfSubscriptionData) => void
}

export function SubscriptionOrderEditor(props: SubscriptionOrderEditorProps) {
  const { t } = useTranslation()
  // null follows the saved order; [] previews the default expiry order.
  const [draftOrder, setDraftOrder] = useState<number[] | null>(null)
  const now = props.nowSeconds ?? Date.now() / 1000
  const activeSubscriptions = props.activeSubscriptions.filter(
    ({ subscription }) =>
      subscription.status === 'active' && subscription.end_time > now
  )
  const subscriptionMap = new Map(
    activeSubscriptions.map((record) => [
      record.subscription.id,
      record.subscription,
    ])
  )
  const activeIds = activeSubscriptions.map((record) => record.subscription.id)
  let orderedIds = activeIds
  if (draftOrder?.length === 0) {
    orderedIds = [...activeSubscriptions]
      .sort(
        (a, b) =>
          a.subscription.end_time - b.subscription.end_time ||
          a.subscription.id - b.subscription.id
      )
      .map((record) => record.subscription.id)
  } else if (draftOrder) {
    const draftIds = new Set(draftOrder)
    orderedIds = [
      ...draftOrder.filter((id) => subscriptionMap.has(id)),
      ...activeIds.filter((id) => !draftIds.has(id)),
    ]
  }
  const inactiveSubscriptions = props.allSubscriptions.filter(
    (record) => !subscriptionMap.has(record.subscription.id)
  )
  const usesDefaultOrder = draftOrder
    ? draftOrder.length === 0
    : props.savedOrder.length === 0
  const hasChanges = draftOrder !== null
  const saveOrder = useMutation({
    mutationFn: async (ids: number[]) => {
      const response = await updateSubscriptionOrder(ids)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Update failed'))
      }
      return response.data
    },
    onSuccess: (data) => {
      props.onSaved(data)
      setDraftOrder(null)
      toast.success(t('Subscription debit order saved'))
    },
    onError: (error) => handleServerError(error),
  })

  const handleMove = (index: number, offset: number) => {
    if (saveOrder.isPending) return
    const target = index + offset
    if (target < 0 || target >= orderedIds.length) return
    const next = [...orderedIds]
    ;[next[index], next[target]] = [next[target], next[index]]
    setDraftOrder(next)
  }

  return (
    <div className='space-y-4'>
      <section aria-label={t('Subscription debit order')} className='space-y-3'>
        <div className='space-y-1'>
          <h3 className='text-sm font-medium'>
            {t('Subscription debit order')}
          </h3>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Drag subscriptions or use the arrow buttons to set debit priority. Save to apply. If a subscription cannot cover a request, the next one is tried.'
            )}
          </p>
          <p className='text-muted-foreground text-xs' role='status'>
            {usesDefaultOrder
              ? t('Earliest expiry first')
              : t('Custom debit order')}
            {hasChanges && ` · ${t('Unsaved changes')}`}
          </p>
        </div>
        {orderedIds.length > 0 ? (
          <Reorder.Group
            as='ol'
            axis='y'
            values={orderedIds}
            onReorder={(ids) => {
              if (!saveOrder.isPending) setDraftOrder(ids)
            }}
            layoutScroll
            aria-label={t('Active subscriptions in debit order')}
            className='flex max-h-96 flex-col gap-3 overflow-y-auto p-1'
          >
            {orderedIds.map((id, index) => {
              const subscription = subscriptionMap.get(id)
              if (!subscription) return null
              return (
                <SubscriptionOrderItem
                  key={id}
                  subscription={subscription}
                  planTitle={props.planTitles.get(subscription.plan_id) || ''}
                  index={index}
                  count={orderedIds.length}
                  disabled={saveOrder.isPending}
                  onMove={handleMove}
                />
              )
            })}
          </Reorder.Group>
        ) : (
          <Empty className='border p-3'>
            <EmptyHeader>
              <EmptyDescription>
                {t('No active subscriptions to reorder.')}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        {saveOrder.isError && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t(
                'Could not save the debit order. Your changes are still here; try again.'
              )}
            </AlertDescription>
          </Alert>
        )}
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            size='sm'
            disabled={!hasChanges || saveOrder.isPending}
            onClick={() =>
              saveOrder.mutate(draftOrder?.length === 0 ? [] : orderedIds)
            }
          >
            {saveOrder.isPending && <Spinner aria-label={t('Saving...')} />}
            {t('Save debit order')}
          </Button>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={usesDefaultOrder || saveOrder.isPending}
            onClick={() => setDraftOrder([])}
          >
            {t('Use earliest expiry first')}
          </Button>
          {hasChanges && (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              disabled={saveOrder.isPending}
              onClick={() => {
                setDraftOrder(null)
                saveOrder.reset()
              }}
            >
              {t('Discard changes')}
            </Button>
          )}
        </div>
      </section>
      {inactiveSubscriptions.length > 0 && (
        <section aria-label={t('Expired and cancelled subscriptions')}>
          <Accordion>
            <AccordionItem value='history'>
              <AccordionTrigger className='text-muted-foreground text-xs'>
                <span>
                  {t('Expired and cancelled subscriptions')} (
                  {inactiveSubscriptions.length})
                </span>
              </AccordionTrigger>
              <AccordionContent>
                <ul className='max-h-64 space-y-3 overflow-y-auto p-1'>
                  {inactiveSubscriptions.map(({ subscription }) => (
                    <li
                      key={subscription.id}
                      className='bg-background rounded-md border p-3'
                    >
                      <SubscriptionSummary
                        subscription={subscription}
                        planTitle={
                          props.planTitles.get(subscription.plan_id) || ''
                        }
                      />
                    </li>
                  ))}
                </ul>
              </AccordionContent>
            </AccordionItem>
          </Accordion>
        </section>
      )}
    </div>
  )
}

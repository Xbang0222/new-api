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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type {
  SubscriptionPlan,
  UserSubscription,
} from '@/features/subscriptions/types'
import { api } from '@/lib/api'

import { SubscriptionPlansCard } from '../subscription-plans-card'

const now = 1_800_000_000
const queryClients: QueryClient[] = []

const monthlyPlan: SubscriptionPlan = {
  id: 1,
  title: 'Monthly plan',
  price_amount: 10,
  currency: 'USD',
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  allow_balance_pay: true,
  allow_wallet_overflow: true,
  max_purchase_per_user: 1,
  total_amount: 0,
}

function subscription(
  overrides: Partial<UserSubscription> = {}
): UserSubscription {
  return {
    id: 1,
    user_id: 2,
    plan_id: monthlyPlan.id,
    status: 'active',
    start_time: now - 3600,
    end_time: now + 3600,
    amount_total: 0,
    amount_used: 0,
    ...overrides,
  }
}

async function renderPlans(
  subscriptions: UserSubscription[],
  limit = 1
): Promise<void> {
  const records = subscriptions.map((item) => ({ subscription: item }))
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/subscription/plans'
          ? [{ plan: { ...monthlyPlan, max_purchase_per_user: limit } }]
          : {
              billing_preference: 'subscription_first',
              subscriptions: records,
              all_subscriptions: records,
            },
    },
  }))
  const client = new QueryClient()
  queryClients.push(client)
  await act(async () => {
    render(
      <QueryClientProvider client={client}>
        <SubscriptionPlansCard topupInfo={null} userQuota={50_000_000} />
      </QueryClientProvider>
    )
  })
  expect(
    screen.getByRole('heading', { name: monthlyPlan.title })
  ).toBeInTheDocument()
}

describe('subscription purchase limits', () => {
  beforeEach(() => {
    vi.spyOn(Date, 'now').mockReturnValue(now * 1000)
  })

  afterEach(() => {
    for (const client of queryClients.splice(0)) client.clear()
    vi.useRealTimers()
  })

  it.each([
    ['an expired subscription', { status: 'expired' }],
    ['a cancelled subscription', { status: 'cancelled' }],
    ['an active subscription past its end time', { end_time: now - 1 }],
    ['an active subscription exactly at its end time', { end_time: now }],
    ['an active subscription for another plan', { plan_id: 2 }],
  ])(
    'allows buying after %s without counting purchase history',
    async (_, changes) => {
      await renderPlans([subscription(changes)])

      const subscribe = screen.getByRole('button', { name: 'Subscribe Now' })
      expect(subscribe).toBeEnabled()
      fireEvent.click(subscribe)

      const dialog = await screen.findByRole('dialog')
      expect(
        within(dialog).getByRole('button', { name: 'Pay with Balance' })
      ).toBeEnabled()
    }
  )

  it.each([
    ['a current active subscription', {}],
    [
      'an active subscription that starts in the future',
      { start_time: now + 60 },
    ],
  ])('blocks buying when %s fills the holding limit', async (_, changes) => {
    await renderPlans([subscription(changes)])

    expect(screen.getByRole('button', { name: 'Limit Reached' })).toBeDisabled()
    expect(
      screen.queryByRole('button', { name: 'Subscribe Now' })
    ).not.toBeInTheDocument()
  })

  it('allows another subscription when active holdings are below the limit despite older purchases', async () => {
    await renderPlans(
      [
        subscription(),
        subscription({ id: 2, end_time: now - 1 }),
        subscription({ id: 3, status: 'cancelled' }),
      ],
      2
    )

    expect(screen.getByRole('button', { name: 'Subscribe Now' })).toBeEnabled()
  })

  it('allows buying with active subscriptions when the holding limit is zero', async () => {
    await renderPlans([subscription()], 0)

    expect(screen.getByRole('button', { name: 'Subscribe Now' })).toBeEnabled()
  })

  it('enables buying when a held subscription expires while the page remains open', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    await renderPlans([subscription({ end_time: now + 60 })])
    expect(screen.getByRole('button', { name: 'Limit Reached' })).toBeDisabled()

    vi.mocked(Date.now).mockReturnValue((now + 60) * 1000)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })

    expect(screen.getByRole('button', { name: 'Subscribe Now' })).toBeEnabled()
  })

  it('keeps a long-lived subscription limited until expiry beyond the browser timer limit', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const validitySeconds = 30 * 86_400
    await renderPlans([subscription({ end_time: now + validitySeconds })])

    vi.mocked(Date.now).mockReturnValue(now * 1000 + 2_147_483_647)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2_147_483_647)
    })
    expect(screen.getByRole('button', { name: 'Limit Reached' })).toBeDisabled()

    vi.mocked(Date.now).mockReturnValue((now + validitySeconds) * 1000)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(validitySeconds * 1000 - 2_147_483_647)
    })
    expect(screen.getByRole('button', { name: 'Subscribe Now' })).toBeEnabled()
  })
})

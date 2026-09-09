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
              subscriptions: records.filter(
                ({ subscription }) =>
                  subscription.status === 'active' &&
                  subscription.end_time > now
              ),
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

  it('updates active and expired counts with the debit lists when a subscription expires on the open page', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    await renderPlans(
      [
        subscription({ end_time: now + 60 }),
        subscription({ id: 2 }),
        subscription({ id: 3 }),
        subscription({ id: 4, status: 'expired', end_time: now - 1 }),
      ],
      3
    )
    expect(screen.getByText('3 active')).toBeInTheDocument()
    expect(screen.getByText('1 expired')).toBeInTheDocument()

    vi.mocked(Date.now).mockReturnValue((now + 60) * 1000)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })

    expect(screen.getByText('2 active')).toBeInTheDocument()
    expect(screen.getByText('2 expired')).toBeInTheDocument()
    const activeList = screen.getByRole('list', {
      name: 'Active subscriptions in debit order',
    })
    expect(within(activeList).getAllByRole('listitem')).toHaveLength(2)
    const history = screen.getByRole('region', {
      name: 'Expired and cancelled subscriptions',
    })
    const historyToggle = within(history).getByRole('button', {
      name: 'Expired and cancelled subscriptions (2)',
    })
    expect(historyToggle).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(historyToggle)
    expect(within(history).getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Subscribe Now' })).toBeEnabled()
  })

  it('disables subscription billing preferences when the last active subscription expires on the open page', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    await renderPlans([subscription({ end_time: now + 60 })])
    fireEvent.click(screen.getByRole('combobox'))
    expect(
      screen.getByRole('option', { name: 'Subscription First' })
    ).not.toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('option', { name: 'Subscription Only' })
    ).not.toHaveAttribute('aria-disabled', 'true')

    vi.mocked(Date.now).mockReturnValue((now + 60) * 1000)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })

    expect(
      screen.getByRole('option', { name: 'Subscription First (No Active)' })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('option', { name: 'Subscription Only (No Active)' })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('option', { name: 'Wallet First' })
    ).not.toHaveAttribute('aria-disabled', 'true')
    expect(
      screen.getByRole('option', { name: 'Wallet Only' })
    ).not.toHaveAttribute('aria-disabled', 'true')
    expect(screen.getByText('No Active')).toBeInTheDocument()
    expect(screen.getByText('1 expired')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Preference saved as Subscription First, but no active subscription. Wallet will be used automatically.'
      )
    ).toBeInTheDocument()
    expect(
      screen.getByText('No active subscriptions to reorder.')
    ).toBeInTheDocument()
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

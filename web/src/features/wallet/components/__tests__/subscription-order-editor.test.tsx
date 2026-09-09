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
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type {
  SelfSubscriptionData,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { api } from '@/lib/api'

import { SubscriptionOrderEditor } from '../subscription-order-editor'

const early: UserSubscriptionRecord = {
  subscription: {
    id: 11,
    user_id: 7,
    plan_id: 1,
    status: 'active',
    start_time: 1,
    end_time: 4102444800,
    amount_total: 100,
    amount_used: 50,
  },
}
const late: UserSubscriptionRecord = {
  subscription: {
    id: 22,
    user_id: 7,
    plan_id: 2,
    status: 'active',
    start_time: 1,
    end_time: 4102531200,
    amount_total: 100,
    amount_used: 100,
  },
}
const expired: UserSubscriptionRecord = {
  subscription: {
    ...early.subscription,
    id: 33,
    status: 'active',
    end_time: 1,
  },
}
const cancelled: UserSubscriptionRecord = {
  subscription: { ...early.subscription, id: 44, status: 'cancelled' },
}
const defaultData: SelfSubscriptionData = {
  billing_preference: 'subscription_first',
  subscription_order: [],
  subscriptions: [early, late],
  all_subscriptions: [cancelled, late, expired, early],
}

function Harness(props: {
  data?: SelfSubscriptionData
  planTitles?: Map<number, string>
}) {
  const [data, setData] = useState(props.data ?? defaultData)
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  )
  return (
    <QueryClientProvider client={client}>
      <SubscriptionOrderEditor
        activeSubscriptions={data.subscriptions}
        allSubscriptions={data.all_subscriptions}
        savedOrder={data.subscription_order || []}
        planTitles={
          props.planTitles ??
          new Map([
            [1, 'Basic'],
            [2, 'Pro'],
          ])
        }
        onSaved={setData}
      />
    </QueryClientProvider>
  )
}

function debitOrder() {
  const list = screen.getByRole('list', {
    name: 'Active subscriptions in debit order',
  })
  return within(list)
    .getAllByRole('heading', { level: 4 })
    .map((heading) => heading.textContent)
}

beforeEach(() => {
  vi.spyOn(console, 'log').mockImplementation(() => undefined)
})

describe('subscription debit ordering', () => {
  test('uses the server debit order while expired and cancelled subscriptions remain outside the sortable list', () => {
    render(
      <Harness
        data={{
          ...defaultData,
          subscription_order: [22, 11],
          subscriptions: [late, early],
        }}
      />
    )

    expect(debitOrder()).toEqual([
      'Pro · Subscription #22',
      'Basic · Subscription #11',
    ])
    expect(screen.getByRole('status')).toHaveTextContent('Custom debit order')
    const history = screen.getByRole('region', {
      name: 'Expired and cancelled subscriptions',
    })
    expect(within(history).getAllByRole('heading', { level: 4 })).toHaveLength(
      2
    )
    expect(within(history).queryByRole('button')).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save debit order' })
    ).toBeDisabled()
  })

  test('arrow buttons and keyboard reorder with stable focus and boundary controls', async () => {
    const user = userEvent.setup()
    render(<Harness />)
    expect(
      screen.getByRole('button', { name: 'Move subscription #11 up' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Move subscription #22 down' })
    ).toBeDisabled()

    await user.click(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    )
    expect(debitOrder()).toEqual([
      'Pro · Subscription #22',
      'Basic · Subscription #11',
    ])
    const handle = screen.getByRole('button', {
      name: 'Drag subscription #11 to reorder',
    })
    handle.focus()
    await user.keyboard('{ArrowUp}')
    expect(debitOrder()).toEqual([
      'Basic · Subscription #11',
      'Pro · Subscription #22',
    ])
    expect(handle).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(debitOrder()).toEqual([
      'Basic · Subscription #11',
      'Pro · Subscription #22',
    ])
    expect(screen.getByRole('status')).toHaveTextContent('Unsaved changes')
  })

  test('dragging a subscription handle changes the visible debit order without saving', async () => {
    const put = vi.spyOn(api, 'put')
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
      function (this: HTMLElement) {
        if (this.tagName === 'LI' && this.parentElement?.tagName === 'OL') {
          const index = [...this.parentElement.children].indexOf(this)
          return new DOMRect(0, index * 120, 320, 120)
        }
        return new DOMRect(0, 0, 320, 240)
      }
    )
    render(<Harness />)
    const handle = screen.getByRole('button', {
      name: 'Drag subscription #11 to reorder',
    })
    fireEvent.pointerDown(handle, {
      pointerId: 1,
      pointerType: 'mouse',
      isPrimary: true,
      button: 0,
      buttons: 1,
      clientX: 20,
      clientY: 20,
    })
    fireEvent.pointerMove(window, {
      pointerId: 1,
      pointerType: 'mouse',
      isPrimary: true,
      buttons: 1,
      clientX: 20,
      clientY: 220,
    })
    await waitFor(() =>
      expect(debitOrder()).toEqual([
        'Pro · Subscription #22',
        'Basic · Subscription #11',
      ])
    )
    fireEvent.pointerUp(window, {
      pointerId: 1,
      pointerType: 'mouse',
      isPrimary: true,
      button: 0,
      clientX: 20,
      clientY: 220,
    })
    expect(
      screen.getByRole('button', { name: 'Save debit order' })
    ).toBeEnabled()
    expect(put).not.toHaveBeenCalled()
  })

  test('saves the displayed order explicitly and uses the returned server order', async () => {
    const put = vi.spyOn(api, 'put').mockResolvedValue({
      data: {
        success: true,
        data: {
          ...defaultData,
          subscription_order: [22, 11],
          subscriptions: [late, early],
        },
      },
    })
    render(<Harness />)
    fireEvent.click(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    )
    expect(put).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Save debit order' }))

    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Save debit order' })
      ).toBeDisabled()
    )
    await waitFor(() =>
      expect(screen.getByRole('status')).not.toHaveTextContent(
        'Unsaved changes'
      )
    )
    expect(put).toHaveBeenCalledWith('/api/subscription/self/order', {
      subscription_ids: [22, 11],
    })
    expect(debitOrder()).toEqual([
      'Pro · Subscription #22',
      'Basic · Subscription #11',
    ])
  })

  test('keeps the pending order after a failed save and retries that same order', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValueOnce({
        data: { success: false, message: 'Order update rejected' },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          data: {
            ...defaultData,
            subscription_order: [22, 11],
            subscriptions: [late, early],
          },
        },
      })
    render(<Harness />)
    fireEvent.click(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save debit order' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Your changes are still here'
    )
    expect(debitOrder()).toEqual([
      'Pro · Subscription #22',
      'Basic · Subscription #11',
    ])
    expect(
      screen.getByRole('button', { name: 'Save debit order' })
    ).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Save debit order' }))
    await waitFor(() =>
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    )
    expect(put.mock.calls.map((call) => call[1])).toEqual([
      { subscription_ids: [22, 11] },
      { subscription_ids: [22, 11] },
    ])
  })

  test('previews earliest expiry and saves an empty override when resetting', async () => {
    const put = vi
      .spyOn(api, 'put')
      .mockResolvedValue({ data: { success: true, data: defaultData } })
    render(
      <Harness
        data={{
          ...defaultData,
          subscriptions: [late, early],
          subscription_order: [22, 11],
        }}
      />
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Use earliest expiry first' })
    )
    expect(debitOrder()).toEqual([
      'Basic · Subscription #11',
      'Pro · Subscription #22',
    ])
    expect(put).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Save debit order' }))
    await waitFor(() =>
      expect(screen.getByRole('status')).not.toHaveTextContent(
        'Unsaved changes'
      )
    )
    expect(put).toHaveBeenCalledWith('/api/subscription/self/order', {
      subscription_ids: [],
    })
    expect(
      screen.getByRole('button', { name: 'Use earliest expiry first' })
    ).toBeDisabled()
  })

  test('locks all editing controls while a save is pending', async () => {
    let finish!: (value: unknown) => void
    vi.spyOn(api, 'put').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    render(<Harness />)
    fireEvent.click(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save debit order' }))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Discard changes' })
      ).toBeDisabled()
    )
    const editor = screen.getByRole('region', {
      name: 'Subscription debit order',
    })
    for (const button of within(editor).getAllByRole('button')) {
      expect(button).toBeDisabled()
    }
    fireEvent.keyDown(
      screen.getByRole('button', { name: 'Drag subscription #11 to reorder' }),
      { key: 'ArrowUp' }
    )
    expect(debitOrder()).toEqual([
      'Pro · Subscription #22',
      'Basic · Subscription #11',
    ])
    await act(async () =>
      finish({ data: { success: true, data: defaultData } })
    )
  })

  test('discard restores the saved sequence without a request', () => {
    const put = vi.spyOn(api, 'put')
    render(<Harness />)
    fireEvent.click(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    )
    fireEvent.click(screen.getByRole('button', { name: 'Discard changes' }))
    expect(debitOrder()).toEqual([
      'Basic · Subscription #11',
      'Pro · Subscription #22',
    ])
    expect(screen.getByRole('status')).not.toHaveTextContent('Unsaved changes')
    expect(put).not.toHaveBeenCalled()
  })

  test('an empty active list shows an empty state and leaves history non-sortable', () => {
    render(
      <Harness
        data={{
          ...defaultData,
          subscriptions: [],
          all_subscriptions: [expired, cancelled],
        }}
      />
    )
    expect(
      screen.getByText('No active subscriptions to reorder.')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('list', {
        name: 'Active subscriptions in debit order',
      })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /Drag subscription/ })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Save debit order' })
    ).toBeDisabled()
  })

  test('a single subscription disables movement and long plan names wrap without hiding controls', () => {
    const title = 'A'.repeat(240)
    render(
      <Harness
        data={{
          ...defaultData,
          subscriptions: [early],
          all_subscriptions: [early],
        }}
        planTitles={new Map([[1, title]])}
      />
    )
    expect(screen.getByRole('heading', { level: 4 })).toHaveTextContent(title)
    expect(screen.getByRole('heading', { level: 4 })).toHaveClass(
      'wrap-anywhere'
    )
    expect(screen.getByRole('list')).toHaveClass('overflow-y-auto')
    expect(
      screen.getByRole('button', { name: 'Drag subscription #11 to reorder' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Move subscription #11 up' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Move subscription #11 down' })
    ).toBeDisabled()
  })
})

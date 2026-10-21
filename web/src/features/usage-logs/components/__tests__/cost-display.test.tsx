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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import type React from 'react'
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  test,
} from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { LogCostDisplay } from '../log-cost-display'

function renderCost(
  props: React.ComponentProps<typeof LogCostDisplay>
): ReturnType<typeof render> {
  return render(<LogCostDisplay {...props} />)
}

describe('log cost display', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      Subscription: 'Subscription',
      'Deducted by subscription': 'Deducted by subscription',
      'Includes tool-call surcharge': 'Includes tool-call surcharge',
    })
  })

  beforeEach(() => {
    useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  })

  afterEach(() => {
    useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
    localStorage.clear()
  })

  test.each([
    { consumed: 12500, expected: '$0.025' },
    { consumed: 0, expected: '$0' },
    { consumed: 1, expected: '$0.000002' },
    { consumed: undefined, expected: '$0.01' },
  ])(
    'keeps the subscription label and reveals the actual deduction $consumed on hover',
    async ({ consumed, expected }) => {
      const user = userEvent.setup()
      renderCost({
        quota: 5000,
        other: {
          billing_source: 'subscription',
          subscription_consumed: consumed,
        },
        showBillingSource: true,
      })
      expect(screen.getByText('Subscription')).toBeVisible()
      expect(
        screen.queryByText(/Deducted by subscription:/)
      ).not.toBeInTheDocument()
      await user.hover(screen.getByLabelText('Subscription'))
      expect(
        await screen.findByText(`Deducted by subscription: ${expected}`)
      ).toBeVisible()
    }
  )

  test.each([true, false])(
    'keeps the wallet amount without a source icon when source display is %s',
    (showBillingSource) => {
      const rendered = renderCost({
        quota: 5000,
        other: { billing_source: 'wallet', subscription_consumed: 12500 },
        showBillingSource,
      })
      expect(rendered.container).toHaveTextContent('$0.01')
      expect(screen.queryByText('Subscription')).not.toBeInTheDocument()
      expect(
        screen.queryByRole('img', { name: 'Wallet' })
      ).not.toBeInTheDocument()
    }
  )

  test('keeps the subscription label when source display is hidden', () => {
    renderCost({
      quota: 5000,
      other: { billing_source: 'subscription' },
      showBillingSource: false,
    })
    expect(screen.getByText('Subscription')).toBeVisible()
  })

  test('keeps legacy costs and large amounts unabridged', () => {
    const rendered = renderCost({ quota: 2147483647, other: null })
    expect(rendered.container).toHaveTextContent('$4,294.9673')
    expect(screen.queryByText('Subscription')).not.toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  test('reveals the subscription deduction on keyboard focus', async () => {
    const user = userEvent.setup()
    renderCost({ quota: 5000, other: { billing_source: 'subscription' } })
    const marker = screen.getByLabelText('Subscription')
    await user.tab()
    expect(marker).toHaveFocus()
    expect(
      await screen.findByText('Deducted by subscription: $0.01')
    ).toBeVisible()
    await user.tab()
    await waitFor(() =>
      expect(
        screen.queryByText(/Deducted by subscription:/)
      ).not.toBeInTheDocument()
    )
  })

  test('keeps the wallet cost and accessible tool surcharge marker', () => {
    const rendered = renderCost({
      quota: 12500,
      other: {
        tool_surcharges: [{ name: 'lookup_customer', count: 1, price: 5 }],
      },
    })
    expect(rendered.container).toHaveTextContent('$0.025')
    const marker = screen.getByRole('img', {
      name: 'Includes tool-call surcharge',
    })
    expect(marker).toHaveAttribute('data-tool-surcharge-indicator', 'true')
    expect(marker).toHaveAttribute('tabindex', '0')
  })

  test('keeps the subscription label alongside a legacy tool surcharge', () => {
    renderCost({
      quota: 5000,
      other: {
        billing_source: 'subscription',
        web_search: true,
        web_search_call_count: 1,
        web_search_price: 10,
      },
    })
    expect(screen.getByText('Subscription')).toBeVisible()
    expect(
      screen.getByRole('img', { name: 'Includes tool-call surcharge' })
    ).toHaveAttribute('data-tool-surcharge-indicator', 'true')
  })
})

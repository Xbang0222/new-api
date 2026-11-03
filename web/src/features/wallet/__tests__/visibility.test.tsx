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
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { Wallet } from '..'

let client: QueryClient
const summaryOnly = {
  summary: true,
  recharge: false,
  subscriptions: false,
  affiliate: false,
}
function setModules(wallet: typeof summaryOnly) {
  client.setQueryData(['status'], {
    SidebarModulesAdmin: JSON.stringify({ wallet }),
  })
}
beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
  setModules(summaryOnly)
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/user/self') {
      return {
        data: {
          success: true,
          data: { quota: 100, used_quota: 0, request_count: 2 },
        },
      }
    }
    if (url === '/api/user/topup/info') {
      return {
        data: {
          success: true,
          data: {
            enable_online_topup: false,
            enable_stripe_topup: false,
            pay_methods: [],
            min_topup: 1,
            stripe_min_topup: 1,
            amount_options: [],
            discount: {},
          },
        },
      }
    }
    if (url === '/api/user/aff') {
      return { data: { success: true, data: 'alice-code' } }
    }
    if (String(url).startsWith('/api/user/topup/self')) {
      return { data: { success: true, data: { items: [], total: 0 } } }
    }
    throw new Error(`Unexpected GET ${url}`)
  })
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: '1' },
  })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})
function renderWallet(initialShowHistory = false) {
  return render(
    <QueryClientProvider client={client}>
      <Wallet initialShowHistory={initialShowHistory} />
    </QueryClientProvider>
  )
}

it('requests only balance data when only wallet summary is visible', async () => {
  renderWallet()
  expect(await screen.findByText('Current Balance')).toBeVisible()
  expect(vi.mocked(api.get).mock.calls.map(([url]) => url)).toEqual([
    '/api/user/self',
  ])
  expect(api.post).not.toHaveBeenCalled()
})

it('closes billing history when recharge is hidden and does not reopen it when recharge returns', async () => {
  setModules({ ...summaryOnly, recharge: true })
  renderWallet()
  await userEvent.click(
    await screen.findByRole('button', { name: 'Order History' })
  )
  expect(
    await screen.findByRole('dialog', { name: 'Billing History' })
  ).toBeVisible()
  await waitFor(() =>
    expect(
      vi
        .mocked(api.get)
        .mock.calls.some(([url]) =>
          String(url).startsWith('/api/user/topup/self')
        )
    ).toBe(true)
  )
  await act(async () => setModules(summaryOnly))
  await waitFor(() =>
    expect(
      screen.queryByRole('dialog', { name: 'Billing History' })
    ).not.toBeInTheDocument()
  )
  const historyCalls = vi
    .mocked(api.get)
    .mock.calls.filter(([url]) =>
      String(url).startsWith('/api/user/topup/self')
    ).length
  await act(async () => setModules({ ...summaryOnly, recharge: true }))
  expect(
    await screen.findByRole('button', { name: 'Order History' })
  ).toBeVisible()
  expect(
    screen.queryByRole('dialog', { name: 'Billing History' })
  ).not.toBeInTheDocument()
  expect(
    vi
      .mocked(api.get)
      .mock.calls.filter(([url]) =>
        String(url).startsWith('/api/user/topup/self')
      )
  ).toHaveLength(historyCalls)
})

it('ignores an initial history link when recharge is disabled, including after it becomes enabled', async () => {
  renderWallet(true)
  expect(await screen.findByText('Current Balance')).toBeVisible()
  expect(
    screen.queryByRole('dialog', { name: 'Billing History' })
  ).not.toBeInTheDocument()
  expect(
    vi
      .mocked(api.get)
      .mock.calls.some(([url]) =>
        String(url).startsWith('/api/user/topup/self')
      )
  ).toBe(false)
  await act(async () => setModules({ ...summaryOnly, recharge: true }))
  expect(
    await screen.findByRole('button', { name: 'Order History' })
  ).toBeVisible()
  expect(
    screen.queryByRole('dialog', { name: 'Billing History' })
  ).not.toBeInTheDocument()
})

it('does not request wallet data when every card is hidden', async () => {
  setModules({
    summary: false,
    recharge: false,
    subscriptions: false,
    affiliate: false,
  })
  await act(async () => renderWallet())
  expect(api.get).not.toHaveBeenCalled()
  expect(api.post).not.toHaveBeenCalled()
})

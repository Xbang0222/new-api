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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { OverviewDashboard } from '../overview-dashboard'

let client: QueryClient
const cards = [
  ['balance', 'Credit remaining'],
  ['todayUsage', 'Last 24h usage'],
  ['usage', 'Historical Usage'],
  ['requests', 'Request Count'],
] as const
const overview = {
  summary: true,
  community: false,
  invitationRanking: false,
  performanceHealth: false,
  announcements: false,
  apiInfo: false,
  faq: false,
  uptimeKuma: false,
  guide: false,
}
beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: [] } })
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'alice',
    role: 1,
    quota: 100,
    used_quota: 20,
    request_count: 4,
  })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
})
async function showOverview(summary: Record<string, boolean>, parent = true) {
  client.setQueryData(['status'], {
    SidebarModulesAdmin: JSON.stringify({
      overview: { ...overview, summary: parent },
      overviewSummary: summary,
    }),
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: OverviewDashboard }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

it.each(cards)(
  'hides only the %s metric and reacts immediately when it is enabled again',
  async (key, label) => {
    await showOverview({ [key]: false })
    expect(await screen.findByText('Usage at a glance')).toBeVisible()
    expect(screen.queryByText(label)).not.toBeInTheDocument()
    expect(
      screen
        .getByText('Usage at a glance')
        .closest('section')
        ?.querySelector('.grid')
    ).toHaveClass('xl:grid-cols-3')
    for (const [otherKey, otherLabel] of cards) {
      if (key !== otherKey) expect(screen.getByText(otherLabel)).toBeVisible()
    }
    act(() =>
      client.setQueryData(['status'], {
        SidebarModulesAdmin: JSON.stringify({
          overview,
          overviewSummary: { [key]: true },
        }),
      })
    )
    expect(await screen.findByText(label)).toBeVisible()
  }
)
it.each([false, true])(
  'does not mount or fetch summary when its parent is %s and every child is hidden',
  async (parent) => {
    await showOverview(
      { balance: false, todayUsage: false, usage: false, requests: false },
      parent
    )
    expect(screen.queryByText('Usage at a glance')).not.toBeInTheDocument()
    expect(
      vi.mocked(api.get).mock.calls.some(([url]) => url === '/api/data/self')
    ).toBe(false)
  }
)
it('keeps legacy summary=false authoritative over enabled child metrics', async () => {
  await showOverview(
    { balance: true, todayUsage: true, usage: true, requests: true },
    false
  )
  await waitFor(() =>
    expect(screen.queryByText('Usage at a glance')).not.toBeInTheDocument()
  )
  expect(
    vi.mocked(api.get).mock.calls.some(([url]) => url === '/api/data/self')
  ).toBe(false)
})

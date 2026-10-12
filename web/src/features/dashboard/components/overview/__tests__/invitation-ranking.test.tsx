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
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosAdapter } from 'axios'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import type { InvitationRankingEntry } from '@/features/dashboard/types'
import en from '@/i18n/locales/en.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { InvitationRankingPanel } from '../invitation-ranking-panel'
import { OverviewDashboard } from '../overview-dashboard'

let client: QueryClient
const i18n = createInstance()
const winners: InvitationRankingEntry[] = [
  {
    rank: 1,
    username: 'Champion-with-a-very-long-username',
    aff_history_quota: 3000000,
    is_self: false,
  },
  {
    rank: 2,
    username: 'Runner-up',
    aff_history_quota: 2000000,
    is_self: false,
  },
  {
    rank: 3,
    username: 'Third-place',
    aff_history_quota: 1000000,
    is_self: false,
  },
  { rank: 4, username: 'Fourth', aff_history_quota: 500000, is_self: false },
  { rank: 5, username: 'Fifth', aff_history_quota: 250000, is_self: true },
]

beforeEach(async () => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  await i18n.init({
    lng: 'en',
    resources: { en, zhTW },
    interpolation: { escapeValue: false },
  })
})

afterEach(() => {
  cleanup()
  client.clear()
  vi.restoreAllMocks()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  useAuthStore.setState(useAuthStore.getInitialState(), true)
})

function renderRanking() {
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <InvitationRankingPanel />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

it('shows all five users on one podium with lower steps for fourth and fifth, excluding sixth place', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        ...winners,
        {
          rank: 6,
          username: 'Sixth',
          aff_history_quota: 100000,
          is_self: false,
        },
      ],
    },
  })
  renderRanking()
  const list = await screen.findByRole('list', {
    name: 'Invitation earnings leaderboard',
  })
  const places = within(list).getAllByRole('listitem')
  expect(places).toHaveLength(5)
  expect(list).toHaveClass('grid-cols-5', 'items-end')
  const positions = [
    'col-start-3',
    'col-start-2',
    'col-start-4',
    'col-start-1',
    'col-start-5',
  ]
  const heights = ['h-24', 'h-20', 'h-16', 'h-12', 'h-8']
  for (const [index, place] of places.entries()) {
    expect(place).toHaveClass(positions[index], 'row-start-1')
    expect(within(place).getByText(String(index + 1))).toHaveClass(
      heights[index]
    )
  }
  expect(within(places[0]).getByText(winners[0].username)).toHaveAttribute(
    'title',
    winners[0].username
  )
  expect(within(places[0]).getByText('$6')).toBeVisible()
  expect(within(places[1]).getByText('$4')).toBeVisible()
  expect(within(places[2]).getByText('$2')).toBeVisible()
  expect(within(places[3]).getByText('Fourth')).toBeVisible()
  expect(within(places[4]).getByText('Fifth')).toBeVisible()
  expect(within(places[4]).getByText('Me')).toBeVisible()
  expect(screen.queryByText('Sixth')).not.toBeInTheDocument()
})

it('keeps a single winner centered without inventing other users', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: winners.slice(0, 1) },
  })
  renderRanking()
  const list = await screen.findByRole('list')
  const places = within(list).getAllByRole('listitem')
  expect(places).toHaveLength(1)
  expect(places[0]).toHaveClass('col-start-3')
})

it('shows the viewer’s full username and keeps identical masked names as separate places', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        {
          rank: 1,
          username: 'd***d',
          aff_history_quota: 3000000,
          is_self: false,
        },
        {
          rank: 2,
          username: 'my-account',
          aff_history_quota: 2000000,
          is_self: true,
        },
        {
          rank: 3,
          username: 'd***d',
          aff_history_quota: 1000000,
          is_self: false,
        },
      ],
    },
  })
  renderRanking()
  const list = await screen.findByRole('list')
  expect(within(list).getAllByRole('listitem')).toHaveLength(3)
  expect(within(list).getAllByText('d***d')).toHaveLength(2)
  const ownPlace = within(list).getAllByRole('listitem')[1]
  expect(within(ownPlace).getByText('my-account')).toHaveAttribute(
    'title',
    'my-account'
  )
  expect(within(ownPlace).getByText('Me')).toBeVisible()
})

it.each([
  { id: 1, role: 1, change: 'administrator becomes a regular user' },
  { id: 2, role: 1, change: 'switching to another account' },
])(
  'does not expose the previous viewer’s response when $change during a pending request',
  async (viewer) => {
    const previousAdapter = api.defaults.adapter
    let finishAdminRequest!: () => void
    const adapter = vi
      .fn<AxiosAdapter>()
      .mockImplementationOnce(
        (config) =>
          new Promise((resolve) => {
            finishAdminRequest = () =>
              resolve({
                data: {
                  success: true,
                  data: [
                    {
                      rank: 1,
                      username: 'private-alice',
                      aff_history_quota: 3000000,
                      is_self: false,
                    },
                  ],
                },
                status: 200,
                statusText: 'OK',
                headers: {},
                config,
              })
          })
      )
      .mockImplementation(async (config) => ({
        data: {
          success: true,
          data: [
            {
              rank: 1,
              username: 'p***e',
              aff_history_quota: 3000000,
              is_self: false,
            },
          ],
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }))
    api.defaults.adapter = adapter
    try {
      useAuthStore.getState().auth.setUser({
        id: 1,
        username: 'admin',
        role: 10,
        quota: 0,
        used_quota: 0,
        request_count: 0,
      })
      renderRanking()
      await waitFor(() => expect(adapter).toHaveBeenCalledTimes(1))
      act(() => {
        useAuthStore.getState().auth.setUser({
          id: viewer.id,
          username: 'viewer',
          role: viewer.role,
          quota: 0,
          used_quota: 0,
          request_count: 0,
        })
      })
      expect(await screen.findByText('p***e')).toBeVisible()
      await act(async () => finishAdminRequest())
      expect(screen.queryByText('private-alice')).not.toBeInTheDocument()
      expect(screen.queryByTitle('private-alice')).not.toBeInTheDocument()
    } finally {
      api.defaults.adapter = previousAdapter
    }
  }
)

it('shows an empty state when nobody has invitation earnings', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: [] } })
  renderRanking()
  expect(await screen.findByText('No invitation earnings yet')).toBeVisible()
  expect(screen.queryByRole('list')).not.toBeInTheDocument()
})

it('waits for the request before showing podium users', async () => {
  let finish!: (value: unknown) => void
  vi.spyOn(api, 'get').mockReturnValue(
    new Promise((resolve) => {
      finish = resolve
    })
  )
  renderRanking()
  expect(screen.queryByRole('list')).not.toBeInTheDocument()
  expect(
    screen.queryByText('No invitation earnings yet')
  ).not.toBeInTheDocument()
  await act(async () => finish({ data: { success: true, data: winners } }))
  expect(await screen.findByRole('list')).toBeVisible()
})

it('shows a retry action for a failed response and recovers when retry succeeds', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Ranking unavailable' },
    })
    .mockResolvedValueOnce({ data: { success: true, data: winners } })
  renderRanking()
  expect(
    await screen.findByText('Unable to load invitation rankings')
  ).toBeVisible()
  expect(
    screen.queryByText('No invitation earnings yet')
  ).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByRole('list')).toBeVisible()
})

it('updates translated labels and formats earnings when switching to Traditional Chinese', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: winners },
  })
  renderRanking()
  await screen.findByRole('list')
  await act(async () => {
    await i18n.changeLanguage('zhTW')
  })
  expect(
    screen.getByRole('list', {
      name: zhTW.translation['Invitation earnings leaderboard'],
    })
  ).toBeVisible()
  expect(
    screen.getByText(zhTW.translation['Rank {{rank}}'].replace('{{rank}}', '1'))
  ).toBeInTheDocument()
  expect(screen.getByText('$6')).toBeVisible()
  expect(screen.getByText(zhTW.translation.Me)).toBeVisible()
})

it.each([1, 10])(
  'shows the leaderboard for role %s and places it before performance health for administrators',
  async (role) => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'viewer',
      role,
      quota: 0,
      used_quota: 0,
      request_count: 0,
    })
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/status') {
        return {
          data: {
            success: true,
            data: {
              api_info_enabled: false,
              announcements_enabled: false,
              faq_enabled: false,
              uptime_kuma_enabled: false,
            },
          },
        }
      }
      if (url === '/api/data/invitation-ranking') {
        return { data: { success: true, data: winners } }
      }
      if (url === '/api/perf-metrics/summary') {
        return { data: { success: true, data: { models: [] } } }
      }
      return { data: { success: true, data: [] } }
    })
    const router = createRouter({
      routeTree: createRootRoute({ component: OverviewDashboard }),
      history: createMemoryHistory({ initialEntries: ['/'] }),
    })
    await router.load()
    render(
      <QueryClientProvider client={client}>
        <I18nextProvider i18n={i18n}>
          <RouterProvider router={router} />
        </I18nextProvider>
      </QueryClientProvider>
    )
    expect(
      await screen.findByRole('list', {
        name: 'Invitation earnings leaderboard',
      })
    ).toBeVisible()
    const informationRow = screen.getByTestId('overview-information-row')
    expect(informationRow.children[0]).toHaveTextContent(
      'Invitation earnings leaderboard'
    )
    if (role === 10) {
      expect(informationRow.children[1]).toHaveTextContent('Performance health')
    } else {
      expect(
        within(informationRow).queryByText('Performance health')
      ).not.toBeInTheDocument()
    }
  }
)

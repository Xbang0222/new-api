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
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { Dashboard } from '../index'

const chart = vi.hoisted(() =>
  vi.fn(
    (_props: {
      spec: { yField?: string; data?: { values: { Count?: number }[] }[] }
    }) => null
  )
)
vi.mock('@visactor/react-vchart', () => ({ VChart: chart }))
vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

const animationDescriptor = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  'getAnimations'
)
let client: QueryClient | undefined
afterEach(() => {
  client?.clear()
  useAuthStore.getState().auth.reset()
  localStorage.clear()
  vi.useRealTimers()
  chart.mockClear()
  if (animationDescriptor) {
    Object.defineProperty(
      HTMLElement.prototype,
      'getAnimations',
      animationDescriptor
    )
  } else Reflect.deleteProperty(HTMLElement.prototype, 'getAnimations')
})

describe('model analytics time selection', () => {
  it('updates cards, ranking, charts and site performance for all five presets', async () => {
    Object.defineProperty(HTMLElement.prototype, 'getAnimations', {
      configurable: true,
      value: () => [],
    })
    vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
    vi.useFakeTimers({ toFake: ['Date'] })
    const now = new Date(2026, 10, 2, 14, 30)
    vi.setSystemTime(now)
    useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
    const cases = [
      { label: 'Current Day', start: new Date(2026, 10, 2), count: 1 },
      { label: '24 Hours', start: new Date(2026, 10, 1, 14), count: 2 },
      { label: '7 Days', start: new Date(2026, 9, 26, 14), count: 3 },
      { label: '14 Days', start: new Date(2026, 9, 19, 14), count: 4 },
      { label: '30 Days', start: new Date(2026, 9, 3, 14), count: 5 },
    ]
    const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
      if (url === '/api/status') {
        return {
          data: {
            success: true,
            data: { display_token_ranking_enabled: true },
          },
        }
      }
      const start = (config?.params as { start_timestamp?: number } | undefined)
        ?.start_timestamp
      const sample = cases.find(
        (item) => Math.floor(item.start.getTime() / 3600000) * 3600 === start
      )
      if (!sample) throw new Error('Unexpected time range')
      if (url === '/api/data') {
        return {
          data: {
            success: true,
            data: [
              {
                created_at: start,
                model_name: 'model-a',
                count: sample.count,
                quota: sample.count * 500000,
                token_used: sample.count * 1000000,
              },
            ],
          },
        }
      }
      if (url === '/api/data/token-ranking') {
        return {
          data: {
            success: true,
            data: {
              ranking: [
                {
                  rank: 1,
                  username: 'alice',
                  token_used: sample.count * 1000000,
                  is_self: false,
                },
              ],
              total_users: 1,
              is_limited: false,
            },
          },
        }
      }
      if (url === '/api/perf-metrics/summary') {
        return {
          data: {
            success: true,
            data: {
              summary: {
                success_rate: 90 + sample.count,
                avg_latency_ms: 1000,
                avg_tps: 5,
              },
              models: [{ model_name: 'model-a', success_rate: 70 }],
            },
          },
        }
      }
      throw new Error('Unexpected endpoint')
    })
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const root = createRootRoute()
    const authenticated = createRoute({
      getParentRoute: () => root,
      id: '_authenticated',
    })
    const models = createRoute({
      getParentRoute: () => authenticated,
      path: 'dashboard/$section',
      component: Dashboard,
    })
    const router = createRouter({
      routeTree: root.addChildren([authenticated.addChildren([models])]),
      history: createMemoryHistory({ initialEntries: ['/dashboard/models'] }),
    })
    await router.load()
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    )
    for (const sample of cases) {
      const buttons = await screen.findAllByRole('button', {
        name: sample.label,
      })
      fireEvent.click(buttons[0])
      await waitFor(() =>
        expect(
          within(screen.getByRole('group', { name: 'Total Count' })).getByText(
            String(sample.count)
          )
        ).toBeVisible()
      )
      expect(await screen.findByText(`${90 + sample.count}.00%`)).toBeVisible()
      expect(
        (await screen.findAllByText(`${sample.count.toFixed(2)}M`))[0]
      ).toBeVisible()
      const expectedRange = {
        start_timestamp: Math.floor(sample.start.getTime() / 3600000) * 3600,
        end_timestamp: now.getTime() / 1000,
      }
      expect(get).toHaveBeenCalledWith(
        '/api/perf-metrics/summary',
        expect.objectContaining({ params: expectedRange })
      )
      await waitFor(() => {
        const call = [...chart.mock.calls]
          .reverse()
          .find(([props]) => props.spec.yField === 'Count')
        expect(
          call?.[0].spec.data?.[0].values.reduce(
            (sum, row) => sum + (row.Count ?? 0),
            0
          )
        ).toBe(sample.count)
      })
    }
  })
})

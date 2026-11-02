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
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { LogStatCards } from '@/features/dashboard/components/models/log-stat-cards'
import type { DashboardFilters } from '@/features/dashboard/types'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { useDashboardQuota } from '../use-dashboard-quota'

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 10 })
})
afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.reset()
})
function Wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}
const first: DashboardFilters = {
  start_timestamp: new Date('2026-11-02T10:15:00Z'),
  end_timestamp: new Date('2026-11-02T11:00:00Z'),
  time_granularity: 'hour',
}
const second: DashboardFilters = {
  ...first,
  start_timestamp: new Date('2026-10-26T10:15:00Z'),
}
function Cards(props: { filters: DashboardFilters }) {
  const query = useDashboardQuota(props.filters, true)
  return (
    <LogStatCards
      data={query.data}
      loading={query.loading}
      error={query.error}
      timeRange={query.timeRange}
    />
  )
}

describe('dashboard quota query', () => {
  it('keeps a late response for the old range from replacing the new selection', async () => {
    type Response = {
      data: { success: boolean; data: { created_at: number; count: number }[] }
    }
    let resolveOld!: (value: Response) => void
    const oldRequest = new Promise<Response>((resolve) => {
      resolveOld = resolve
    })
    vi.spyOn(api, 'get')
      .mockReturnValueOnce(oldRequest)
      .mockResolvedValueOnce({
        data: { success: true, data: [{ created_at: 1793070000, count: 9 }] },
      })
    const hook = renderHook(
      (props: { filters: DashboardFilters }) =>
        useDashboardQuota(props.filters, true),
      {
        initialProps: { filters: first },
        wrapper: Wrapper,
      }
    )
    hook.rerender({ filters: second })
    await waitFor(() => expect(hook.result.current.data[0]?.count).toBe(9))
    await act(async () => {
      resolveOld({
        data: { success: true, data: [{ created_at: 1793613600, count: 2 }] },
      })
      await oldRequest
    })
    expect(hook.result.current.data[0]?.count).toBe(9)
  })

  it('uses the same hourly query window for totals and per-minute rates', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: {
        success: true,
        data: [{ created_at: 1793613600, count: 120, token_used: 600 }],
      },
    })
    render(
      <Wrapper>
        <Cards filters={first} />
      </Wrapper>
    )
    await waitFor(() =>
      expect(
        within(screen.getByRole('group', { name: 'Total Count' })).getByText(
          '120'
        )
      ).toBeVisible()
    )
    expect(
      within(screen.getByRole('group', { name: 'Average RPM' })).getByText('2')
    ).toBeVisible()
    expect(
      within(screen.getByRole('group', { name: 'Average TPM' })).getByText('10')
    ).toBeVisible()
    expect(get).toHaveBeenCalledWith(
      '/api/data',
      expect.objectContaining({
        params: expect.objectContaining({
          start_timestamp: new Date('2026-11-02T10:00:00Z').getTime() / 1000,
        }),
      })
    )
  })

  it.each([
    {
      name: 'business failure',
      response: Promise.resolve({
        data: { success: false, message: 'unavailable' },
      }),
    },
    {
      name: 'empty response',
      response: Promise.resolve({ data: { success: true, data: [] } }),
    },
  ])('distinguishes $name from the other state', async ({ name, response }) => {
    vi.spyOn(api, 'get').mockReturnValue(response)
    render(
      <Wrapper>
        <Cards filters={first} />
      </Wrapper>
    )
    const count = within(screen.getByRole('group', { name: 'Total Count' }))
    expect(
      await count.findByText(name === 'business failure' ? '--' : '0')
    ).toBeVisible()
  })

  it('exposes a transport failure as an error without usable data', async () => {
    vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))
    const hook = renderHook(() => useDashboardQuota(first, true), {
      wrapper: Wrapper,
    })
    await waitFor(() => expect(hook.result.current.error).toBe(true))
    expect(hook.result.current.data).toEqual([])
  })
})

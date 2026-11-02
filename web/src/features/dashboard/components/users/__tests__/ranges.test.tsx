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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { UserChartsFilters } from '@/features/dashboard/types'
import { api } from '@/lib/api'

import { UserCharts } from '../user-charts'

vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))
vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-11-02T10:15:00Z'))
})
afterEach(() => {
  client.clear()
  localStorage.clear()
  vi.useRealTimers()
})
function Users() {
  const [filters, setFilters] = useState<UserChartsFilters>({
    timeGranularity: 'day',
    selectedRange: 7,
    topUserLimit: 10,
    activeQuickRange: 'last_7_days',
  })
  return (
    <QueryClientProvider client={client}>
      <UserCharts filters={filters} onFiltersChange={setFilters} />
    </QueryClientProvider>
  )
}

describe('user analytics time controls', () => {
  it('refreshes the rolling interval and keeps its duration when changing granularity', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: [] } })
    render(<Users />)
    await waitFor(() => expect(get).toHaveBeenCalled())
    vi.setSystemTime(new Date('2026-11-02T11:45:00Z'))
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    const refreshed = {
      start_timestamp: new Date('2026-10-26T11:00:00Z').getTime() / 1000,
      end_timestamp: new Date('2026-11-02T11:45:00Z').getTime() / 1000,
    }
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith('/api/data/users', {
        params: refreshed,
      })
    )
    vi.setSystemTime(new Date('2026-11-02T12:00:00Z'))
    fireEvent.click(screen.getByRole('tab', { name: 'Week' }))
    expect(screen.getByRole('button', { name: '7 Days' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(get).toHaveBeenLastCalledWith('/api/data/users', {
      params: refreshed,
    })
  })

  it('queries the chosen quick range and displays a business failure with retry', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: [] } })
    render(<Users />)
    await waitFor(() => expect(get).toHaveBeenCalled())
    vi.setSystemTime(new Date('2026-11-03T12:45:00Z'))
    get.mockResolvedValue({ data: { success: false, message: 'unavailable' } })
    fireEvent.click(screen.getByRole('button', { name: '14 Days' }))
    expect(await screen.findByRole('button', { name: 'Retry' })).toBeVisible()
    expect(get).toHaveBeenLastCalledWith('/api/data/users', {
      params: {
        start_timestamp: new Date('2026-10-20T12:00:00Z').getTime() / 1000,
        end_timestamp: new Date('2026-11-03T12:45:00Z').getTime() / 1000,
      },
    })
  })
})

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
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  buildQueryParams,
  buildQuickRangeDashboardFilters,
  refreshDashboardFilters,
} from '../filters'

afterEach(() => vi.useRealTimers())

describe('dashboard time ranges', () => {
  it('includes the partial first hour in both the query and rate denominator', () => {
    expect(
      buildQueryParams({ start_timestamp: 3661, end_timestamp: 7261 })
    ).toMatchObject({
      start_timestamp: 3600,
      end_timestamp: 7261,
    })
  })

  it.each([
    ['last_24_hours', 24],
    ['last_7_days', 7 * 24],
    ['last_14_days', 14 * 24],
    ['last_30_days', 30 * 24],
  ])('builds %s as a rolling range ending now', (preset, hours) => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-02T10:15:30Z'))
    const { filters } = buildQuickRangeDashboardFilters(String(preset))
    expect(filters.end_timestamp?.getTime()).toBe(Date.now())
    expect(filters.start_timestamp?.getTime()).toBe(
      Date.now() - Number(hours) * 3600000
    )
  })

  it('starts current day at local midnight and preserves user filters', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 10, 2, 14, 25))
    const { filters } = buildQuickRangeDashboardFilters('today', {
      username: 'alice',
    })
    expect(filters.start_timestamp).toEqual(new Date(2026, 10, 2))
    expect(filters.end_timestamp?.getTime()).toBe(Date.now())
    expect(filters.username).toBe('alice')
  })
})

describe('dashboard refresh', () => {
  it('advances a rolling range and keeps the selected user', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-11-02T10:00:00Z'))
    const initial = buildQuickRangeDashboardFilters('last_7_days', {
      username: 'alice',
    }).filters
    vi.setSystemTime(new Date('2026-11-02T11:30:00Z'))
    const refreshed = refreshDashboardFilters(initial, 'last_7_days')
    expect(refreshed.end_timestamp).toEqual(new Date('2026-11-02T11:30:00Z'))
    expect(refreshed.start_timestamp).toEqual(new Date('2026-10-26T11:30:00Z'))
    expect(refreshed.username).toBe('alice')
  })

  it('preserves a historical custom range on refresh', () => {
    const initial = {
      start_timestamp: new Date('2026-01-01'),
      end_timestamp: new Date('2026-01-08'),
      username: 'alice',
    }
    expect(refreshDashboardFilters(initial, null)).toEqual(initial)
  })
})

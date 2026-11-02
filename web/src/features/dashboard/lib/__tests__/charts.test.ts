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
import { describe, expect, test, vi } from 'vitest'

import type { QuotaDataItem } from '@/features/dashboard/types'
import type { TimeGranularity } from '@/lib/time'

import { processChartData, processUserChartData } from '../charts'

const cases: Array<{
  granularity: TimeGranularity
  start: string
  interval: number
  labels: string[]
}> = [
  {
    granularity: 'hour',
    start: '2025-12-31T20:00:00',
    interval: 3600,
    labels: [
      '12-31 20:00',
      '12-31 21:00',
      '12-31 22:00',
      '12-31 23:00',
      '01-01 00:00',
      '01-01 01:00',
      '01-01 02:00',
      '01-01 03:00',
    ],
  },
  {
    granularity: 'day',
    start: '2025-12-27T12:00:00',
    interval: 86400,
    labels: [
      '12-27',
      '12-28',
      '12-29',
      '12-30',
      '12-31',
      '01-01',
      '01-02',
      '01-03',
    ],
  },
  {
    granularity: 'week',
    start: '2025-11-29T12:00:00',
    interval: 604800,
    labels: [
      '11-24 - 11-30',
      '12-01 - 12-07',
      '12-08 - 12-14',
      '12-15 - 12-21',
      '12-22 - 12-28',
      '12-29 - 01-04',
      '01-05 - 01-11',
      '01-12 - 01-18',
    ],
  },
]

function unorderedUsage(start: string, interval: number): QuotaDataItem[] {
  const first = new Date(start).getTime() / 1000
  const rows = Array.from({ length: 8 }, (_, i) => ({
    created_at: first + i * interval,
    model_name: 'model-a',
    username: 'alice',
    quota: (i + 1) * 500000,
    count: i + 1,
  }))
  // Same hour, or a second hourly record within the same day/week label.
  const duplicate = {
    ...rows[0],
    created_at: first + (interval === 3600 ? 0 : 3600),
    quota: 5000000,
    count: 10,
  }
  return [
    rows[7],
    rows[2],
    duplicate,
    rows[5],
    rows[0],
    rows[6],
    rows[1],
    rows[4],
    rows[3],
  ]
}

describe.each(cases)('dashboard $granularity chart chronology', (scenario) => {
  test('orders model buckets across New Year while aggregating repeated labels', () => {
    const data = unorderedUsage(scenario.start, scenario.interval)
    const original = structuredClone(data)
    const result = processChartData(data, scenario.granularity)

    for (const key of ['spec_line', 'spec_area', 'spec_model_line'] as const) {
      const values: Array<{ Time: string; rawQuota: number; Count: number }> =
        result[key].data[0].values
      expect(values.map((row) => row.Time)).toEqual(scenario.labels)
      expect(
        values.map((row) =>
          key === 'spec_model_line' ? row.Count : row.rawQuota / 500000
        )
      ).toEqual([11, 2, 3, 4, 5, 6, 7, 8])
    }
    expect(data).toEqual(original)
  })

  test('orders user buckets across New Year while aggregating repeated labels', () => {
    const data = unorderedUsage(scenario.start, scenario.interval)
    const original = structuredClone(data)
    const result = processUserChartData(data, scenario.granularity)
    const values: Array<{ Time: string; rawQuota: number }> =
      result.spec_user_trend.data[0].values

    expect(values.map((row) => row.Time)).toEqual(scenario.labels)
    expect(values.map((row) => row.rawQuota / 500000)).toEqual([
      11, 2, 3, 4, 5, 6, 7, 8,
    ])
    expect(data).toEqual(original)
  })

  test('does not fabricate six earlier buckets for a single record without a selected range', () => {
    const data = [unorderedUsage(scenario.start, scenario.interval)[0]]
    const result = processChartData(data, scenario.granularity)

    for (const key of ['spec_line', 'spec_area', 'spec_model_line'] as const) {
      const values: Array<{ Time: string; rawQuota: number; Count: number }> =
        result[key].data[0].values
      expect(values.map((row) => row.Time)).toEqual(scenario.labels.slice(-1))
      expect(
        values.map((row) =>
          key === 'spec_model_line' ? row.Count : row.rawQuota / 500000
        )
      ).toEqual([8])
    }
  })
})

describe('dashboard range aggregation', () => {
  test('keeps both sparse endpoints and fills the selected daily window', () => {
    const start = new Date('2026-10-03T00:00:00').getTime() / 1000
    const end = new Date('2026-11-02T00:00:00').getTime() / 1000
    const data = [
      {
        created_at: start,
        model_name: 'a',
        username: 'alice',
        quota: 500000,
        count: 1,
      },
      {
        created_at: end,
        model_name: 'a',
        username: 'alice',
        quota: 1000000,
        count: 2,
      },
    ]
    const range = { start_timestamp: start, end_timestamp: end }
    const result = processChartData(
      data,
      'day',
      undefined,
      undefined,
      undefined,
      range
    )
    const trend = result.spec_model_line.data[0].values
    expect(trend).toHaveLength(31)
    expect(trend[0]).toMatchObject({ Time: '10-03', Count: 1 })
    expect(trend[30]).toMatchObject({ Time: '11-02', Count: 2 })
    expect(
      trend.slice(1, 30).every((row: { Count: number }) => row.Count === 0)
    ).toBe(true)
    for (const key of ['spec_line', 'spec_area'] as const) {
      expect(
        result[key].data[0].values.reduce(
          (sum: number, row: { rawQuota: number }) => sum + row.rawQuota,
          0
        )
      ).toBe(1500000)
    }
    const users = processUserChartData(
      data,
      'day',
      undefined,
      10,
      undefined,
      range
    ).spec_user_trend.data[0].values
    expect(users).toHaveLength(31)
    expect(
      users.reduce(
        (sum: number, row: { rawQuota: number }) => sum + row.rawQuota,
        0
      )
    ).toBe(1500000)
  })

  test('aggregates Monday and Sunday into one week and starts a new bucket next Monday', () => {
    const data = [
      '2026-10-26T12:00:00',
      '2026-11-01T12:00:00',
      '2026-11-02T12:00:00',
    ].map((date) => ({
      created_at: new Date(date).getTime() / 1000,
      model_name: 'a',
      username: 'alice',
      count: 1,
      quota: 500000,
    }))
    const models = processChartData(data, 'week').spec_model_line.data[0].values
    expect(models).toEqual([
      { Time: '10-26 - 11-01', Model: 'a', Count: 2 },
      { Time: '11-02 - 11-08', Model: 'a', Count: 1 },
    ])
    const users = processUserChartData(data, 'week').spec_user_trend.data[0]
      .values
    expect(users.map((row: { rawQuota: number }) => row.rawQuota)).toEqual([
      1000000, 500000,
    ])
  })

  test('keeps identical calendar labels from different years as different buckets', () => {
    const data = ['2025-11-02T12:00:00', '2026-11-02T12:00:00'].map((date) => ({
      created_at: new Date(date).getTime() / 1000,
      model_name: 'a',
      username: 'alice',
      count: 1,
      quota: 500000,
    }))
    const models = processChartData(data, 'day').spec_model_line.data[0].values
    expect(
      models.filter((row: { Count: number }) => row.Count > 0)
    ).toHaveLength(2)
    expect(new Set(models.map((row: { Time: string }) => row.Time)).size).toBe(
      models.length
    )
    const users = processUserChartData(data, 'day').spec_user_trend.data[0]
      .values
    expect(
      users.filter((row: { rawQuota: number }) => row.rawQuota > 0)
    ).toHaveLength(2)
  })
})

test('preserves Sunday usage and midnight day buckets through a midnight DST transition', () => {
  vi.stubEnv('TZ', 'America/Santiago')
  try {
    const data = [
      '2026-08-31T12:00:00',
      '2026-09-06T12:00:00',
      '2026-09-07T12:00:00',
      '2026-09-08T12:00:00',
    ].map((date) => ({
      created_at: new Date(date).getTime() / 1000,
      model_name: 'a',
      username: 'alice',
      count: 1,
      quota: 500000,
    }))
    const models = processChartData(data, 'week').spec_model_line.data[0].values
    expect(models.map((row: { Count: number }) => row.Count)).toEqual([2, 2])
    const users = processUserChartData(data, 'week').spec_user_trend.data[0]
      .values
    expect(users.map((row: { rawQuota: number }) => row.rawQuota)).toEqual([
      1000000, 1000000,
    ])
    const daily = processChartData(data.slice(1), 'day').spec_model_line.data[0]
      .values
    expect(
      daily.map((row: { Time: string; Count: number }) => [row.Time, row.Count])
    ).toEqual([
      ['09-06', 1],
      ['09-07', 1],
      ['09-08', 1],
    ])
  } finally {
    vi.unstubAllEnvs()
  }
})

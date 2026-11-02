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
import dayjs from '@/lib/dayjs'
import { formatChartTime, type TimeGranularity } from '@/lib/time'

export interface ChartTimeRange {
  start_timestamp: number
  end_timestamp: number
}

// Usage is stored in UTC-aligned hours; larger display buckets follow the
// viewer's calendar. setDate preserves calendar boundaries through DST.
export function dashboardBucketStart(
  timestamp: number,
  granularity: TimeGranularity
): number {
  if (granularity === 'hour') return Math.floor(timestamp / 3600) * 3600
  const date = new Date(timestamp * 1000)
  if (granularity === 'week') {
    date.setDate(date.getDate() - ((date.getDay() + 6) % 7))
  }
  date.setHours(0, 0, 0, 0)
  return date.getTime() / 1000
}

export function buildDashboardTimeAxis(
  timestamps: Iterable<number>,
  granularity: TimeGranularity,
  range?: ChartTimeRange
): Map<number, string> {
  const buckets = new Set<number>()
  let first = Infinity
  let last = -Infinity
  for (const timestamp of timestamps) {
    const bucket = dashboardBucketStart(timestamp, granularity)
    if (!Number.isFinite(bucket)) continue
    buckets.add(bucket)
    first = Math.min(first, bucket)
    last = Math.max(last, bucket)
  }
  if (range) {
    first = dashboardBucketStart(range.start_timestamp, granularity)
    last = dashboardBucketStart(range.end_timestamp, granularity)
  }
  if (Number.isFinite(first) && Number.isFinite(last)) {
    for (let timestamp = first; timestamp <= last;) {
      buckets.add(timestamp)
      if (granularity === 'hour') {
        timestamp += 3600
      } else {
        const next = new Date(timestamp * 1000)
        next.setDate(next.getDate() + (granularity === 'week' ? 7 : 1))
        next.setHours(0, 0, 0, 0)
        timestamp = next.getTime() / 1000
      }
    }
  }
  const axis = new Map<number, string>()
  const labelCounts = new Map<string, number>()
  for (const timestamp of [...buckets].sort((a, b) => a - b)) {
    const label =
      granularity === 'hour'
        ? dayjs(timestamp * 1000).format('MM-DD HH:mm')
        : formatChartTime(timestamp, granularity)
    axis.set(timestamp, label)
    labelCounts.set(label, (labelCounts.get(label) ?? 0) + 1)
  }
  // A band chart identifies points by their displayed category. Disambiguate
  // repeated dates across years and repeated clock hours at a DST rollback.
  for (const [timestamp, label] of axis) {
    if ((labelCounts.get(label) ?? 0) > 1) {
      axis.set(timestamp, dayjs(timestamp * 1000).format('YYYY-MM-DD HH:mm Z'))
    }
  }
  return axis
}

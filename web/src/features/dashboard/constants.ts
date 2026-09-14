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
import type { DashboardChartPreferences, DashboardFilters } from './types'

export const TIME_GRANULARITY_STORAGE_KEY = 'data_export_default_time'
export const DASHBOARD_CHART_PREFERENCES_STORAGE_KEY =
  'dashboard_models_chart_preferences'
export const DEFAULT_TIME_GRANULARITY = 'hour' as const
export const MAX_CHART_TREND_POINTS = 7
export const DASHBOARD_RANK_BASE_COUNT = 10
export const DASHBOARD_RANK_BAR_HEIGHT = 20
export const DASHBOARD_RANK_DESKTOP_ROW_HEIGHT =
  DASHBOARD_RANK_BAR_HEIGHT + 4
export const DASHBOARD_RANK_CHART_HEADER_HEIGHT = 60
export const DASHBOARD_RANK_DESKTOP_CHART_HEIGHT =
  DASHBOARD_RANK_CHART_HEADER_HEIGHT +
  DASHBOARD_RANK_DESKTOP_ROW_HEIGHT * DASHBOARD_RANK_BASE_COUNT

export const DEFAULT_DASHBOARD_CHART_PREFERENCES: DashboardChartPreferences = {
  consumptionDistributionChart: 'bar',
  modelAnalyticsChart: 'trend',
  defaultTimeRangeDays: 1,
  defaultTimeGranularity: DEFAULT_TIME_GRANULARITY,
}

export const TIME_RANGE_BY_GRANULARITY = {
  hour: 1,
  day: 7,
  week: 29,
} as const

export const TIME_GRANULARITY_OPTIONS = [
  { label: 'Hour', value: 'hour' },
  { label: 'Day', value: 'day' },
  { label: 'Week', value: 'week' },
] as const

export const TIME_RANGE_PRESETS = [
  { label: '24 Hours', days: 1 },
  { label: '7 Days', days: 7 },
  { label: '14 Days', days: 14 },
  { label: '30 Days', days: 30 },
] as const

export const DASHBOARD_QUICK_RANGE_PRESETS = [
  {
    key: 'today',
    label: 'Current Day',
    granularity: 'hour',
    mode: 'today',
    amount: 1,
    unit: 'day',
  },
  {
    key: 'last_24_hours',
    label: '24 Hours',
    granularity: 'hour',
    mode: 'rolling',
    amount: 24,
    unit: 'hour',
  },
  {
    key: 'last_7_days',
    label: '7 Days',
    granularity: 'day',
    mode: 'rolling',
    amount: 7,
    unit: 'day',
  },
  {
    key: 'last_14_days',
    label: '14 Days',
    granularity: 'day',
    mode: 'rolling',
    amount: 14,
    unit: 'day',
  },
  {
    key: 'last_30_days',
    label: '30 Days',
    granularity: 'day',
    mode: 'rolling',
    amount: 30,
    unit: 'day',
  },
] as const

export const DEFAULT_DASHBOARD_QUICK_RANGE = 'today'

export const CONSUMPTION_DISTRIBUTION_CHART_OPTIONS = [
  { value: 'bar', labelKey: 'Bar Chart' },
  { value: 'area', labelKey: 'Area Chart' },
] as const

export const MODEL_ANALYTICS_CHART_OPTIONS = [
  { value: 'trend', labelKey: 'Call Trend' },
  { value: 'proportion', labelKey: 'Call Count Distribution' },
  { value: 'top', labelKey: 'Call Count Ranking' },
] as const

export const EMPTY_DASHBOARD_FILTERS: DashboardFilters = {
  start_timestamp: undefined,
  end_timestamp: undefined,
  time_granularity: 'hour',
  username: '',
}

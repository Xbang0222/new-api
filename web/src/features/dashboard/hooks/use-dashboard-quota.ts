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
import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { getUserQuotaDates } from '@/features/dashboard/api'
import {
  buildQueryParams,
  getDefaultDays,
} from '@/features/dashboard/lib/filters'
import type {
  DashboardFilters,
  QuotaDataItem,
} from '@/features/dashboard/types'
import { ROLE } from '@/lib/roles'
import { requireServerSuccess } from '@/lib/server-error-message'
import { computeTimeRange } from '@/lib/time'
import { useAuthStore } from '@/stores/auth-store'

const EMPTY_DATA: QuotaDataItem[] = []

export function useDashboardQuota(filters: DashboardFilters, enabled: boolean) {
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)
  const params = useMemo(
    () =>
      buildQueryParams(
        computeTimeRange(
          getDefaultDays(filters.time_granularity),
          filters.start_timestamp,
          filters.end_timestamp ?? new Date()
        ),
        filters
      ),
    [filters]
  )
  const timeRange = useMemo(
    () => ({
      start_timestamp: params.start_timestamp,
      end_timestamp: params.end_timestamp,
    }),
    [params.start_timestamp, params.end_timestamp]
  )
  const query = useQuery({
    queryKey: ['dashboard', 'quota', user?.id, isAdmin, params],
    queryFn: async ({ signal }) =>
      requireServerSuccess(await getUserQuotaDates(params, isAdmin, signal)),
    enabled,
    staleTime: 60_000,
  })
  return {
    data: query.data?.data ?? EMPTY_DATA,
    timeRange,
    loading: query.isFetching || query.isPending,
    error: query.isError,
    refetch: query.refetch,
  }
}

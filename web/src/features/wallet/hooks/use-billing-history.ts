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
import i18next from 'i18next'
import { useState, useEffect, useCallback, useRef } from 'react'
import { toast } from 'sonner'

import { useIsAdmin } from '@/hooks/use-admin'
import { useDebounce } from '@/hooks/use-debounce'
import { handleServerError } from '@/lib/handle-server-error'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  getUserBillingHistory,
  getAllBillingHistory,
  getBillingHistoryExport,
  completeOrder,
  isApiSuccess,
} from '../api'
import type { TopupRecord } from '../types'

// ============================================================================
// Billing History Hook
// ============================================================================

interface UseBillingHistoryOptions {
  enabled?: boolean
  /** Initial page number */
  initialPage?: number
  /** Initial page size */
  initialPageSize?: number
}

export function useBillingHistory(options: UseBillingHistoryOptions = {}) {
  const { initialPage = 1, initialPageSize = 10, enabled = true } = options
  const isAdmin = useIsAdmin()

  const [records, setRecords] = useState<TopupRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword)
  const [userSearchField, setUserSearchField] = useState<
    'username' | 'user_id'
  >('username')
  const [userSearch, setUserSearch] = useState('')
  const debouncedUserSearch = useDebounce(userSearch)
  const invalidUserId =
    isAdmin &&
    userSearchField === 'user_id' &&
    userSearch.trim() !== '' &&
    (!/^[1-9]\d*$/.test(userSearch.trim()) ||
      !Number.isSafeInteger(Number(userSearch.trim())))
  const searchPending =
    keyword !== debouncedKeyword || userSearch !== debouncedUserSearch
  const requestIdRef = useRef(0)
  const [loading, setLoading] = useState(false)
  const [completing, setCompleting] = useState(false)
  const [exporting, setExporting] = useState(false)

  /**
   * Fetch billing history
   */
  const fetchBillingHistory = useCallback(async () => {
    const requestId = ++requestIdRef.current
    if (invalidUserId) {
      setRecords([])
      setTotal(0)
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      const response = isAdmin
        ? await getAllBillingHistory(page, pageSize, debouncedKeyword.trim(), {
            [userSearchField]: debouncedUserSearch.trim(),
          })
        : await getUserBillingHistory(page, pageSize, debouncedKeyword)

      if (requestId !== requestIdRef.current) return

      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        handleServerError(response, i18next.t('Failed to load billing history'))
        setRecords([])
        setTotal(0)
      }
    } catch (error) {
      if (requestId !== requestIdRef.current) return
      handleServerError(error, i18next.t('Failed to load billing history'))
      setRecords([])
      setTotal(0)
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false)
      }
    }
  }, [
    debouncedKeyword,
    debouncedUserSearch,
    userSearchField,
    invalidUserId,
    isAdmin,
    page,
    pageSize,
  ])

  const handleExport = async () => {
    if (exporting || loading || searchPending || invalidUserId || total === 0) {
      return
    }
    const filters = {
      keyword: keyword.trim(),
      ...(isAdmin ? { [userSearchField]: userSearch.trim() } : {}),
    }
    const language = i18next.language
    const translate = i18next.getFixedT(language)
    setExporting(true)
    try {
      const [response, { createBillingHistoryPdf }] = await Promise.all([
        getBillingHistoryExport(isAdmin, filters),
        import('../lib/billing-pdf'),
      ])
      if (!isApiSuccess(response) || !response.data) {
        handleServerError(
          response,
          translate('Failed to export top-up records')
        )
        return
      }
      if (response.data.items.length === 0) {
        toast.error(translate('No billing records found'))
        return
      }
      const bytes = await createBillingHistoryPdf(response.data, {
        siteName: useSystemConfigStore.getState().config.systemName,
        language,
        filters,
        t: translate,
      })
      const url = URL.createObjectURL(
        new Blob([bytes], { type: 'application/pdf' })
      )
      const link = document.createElement('a')
      link.href = url
      link.download = `topup-records-${response.data.generated_at}.pdf`
      document.body.append(link)
      link.click()
      link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
    } catch (error) {
      handleServerError(error, translate('Failed to export top-up records'))
    } finally {
      setExporting(false)
    }
  }

  /**
   * Complete a pending order (admin only)
   */
  const handleCompleteOrder = useCallback(
    async (tradeNo: string) => {
      if (!isAdmin) {
        toast.error(i18next.t('Admin access required'))
        return false
      }

      setCompleting(true)
      try {
        const response = await completeOrder({ trade_no: tradeNo })
        if (isApiSuccess(response)) {
          toast.success(i18next.t('Order completed successfully'))
          // Refresh the list
          await fetchBillingHistory()
          return true
        } else {
          handleServerError(response, i18next.t('Failed to complete order'))
          return false
        }
      } catch (error) {
        handleServerError(error, i18next.t('Failed to complete order'))
        return false
      } finally {
        setCompleting(false)
      }
    },
    [isAdmin, fetchBillingHistory]
  )

  /**
   * Change page
   */
  const handlePageChange = useCallback((newPage: number) => {
    requestIdRef.current += 1
    setLoading(true)
    setPage(newPage)
  }, [])

  /**
   * Change page size
   */
  const handlePageSizeChange = useCallback((newPageSize: number) => {
    requestIdRef.current += 1
    setLoading(true)
    setPageSize(newPageSize)
    setPage(1) // Reset to first page when changing page size
  }, [])

  /**
   * Search by keyword
   */
  const handleSearch = useCallback((newKeyword: string) => {
    requestIdRef.current += 1
    setLoading(true)
    setKeyword(newKeyword)
    setPage(1) // Reset to first page when searching
  }, [])

  const handleUserSearch = (value: string) => {
    requestIdRef.current += 1
    setLoading(true)
    setUserSearch(value)
    setPage(1)
  }

  const handleUserSearchField = (field: 'username' | 'user_id') => {
    requestIdRef.current += 1
    setLoading(true)
    setUserSearchField(field)
    setUserSearch('')
    setPage(1)
  }

  // Fetch data after the search draft has settled.
  useEffect(() => {
    if (!enabled || searchPending) return

    void fetchBillingHistory()
    return () => {
      requestIdRef.current += 1
    }
  }, [enabled, fetchBillingHistory, searchPending])

  return {
    records,
    total,
    page,
    pageSize,
    keyword,
    loading: loading || searchPending,
    completing,
    exporting,
    userSearch,
    userSearchField,
    invalidUserId,
    isAdmin,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleCompleteOrder,
    handleExport,
    handleUserSearch,
    handleUserSearchField,
    refresh: fetchBillingHistory,
  }
}

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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { useCallback, useEffect, useRef } from 'react'
import { toast } from 'sonner'

import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { updatePasskeyDomains, updateSystemOption } from '../api'
import type { UpdateOptionRequest, UpdatePasskeyDomainsRequest } from '../types'

// Configuration keys that require status refresh
const STATUS_RELATED_KEYS = new Set([
  'theme.frontend',
  'HeaderNavModules',
  'SidebarModulesAdmin',
  'Notice',
  'LogConsumeEnabled',
  'Logo',
  'log_setting.user_export_enabled',
  'log_setting.max_range_days',
  'QuotaPerUnit',
  'USDExchangeRate',
  'DisplayInCurrencyEnabled',
  'DisplayTokenStatEnabled',
  'DisplayTokenRankingEnabled',
  'ConsoleHomePage',
  'GroupChatQRCodeImageURL',
  'GroupChatQRCodeExpiresAt',
  'general_setting.quota_display_type',
  'general_setting.custom_currency_symbol',
  'general_setting.custom_currency_exchange_rate',
  'general_setting.docs_link',
  'oidc.display_name',
  'ServerAddress',
  'passkey.enabled',
  'passkey.rp_id',
  'passkey.legacy_rp_ids',
  'passkey.origins',
  'QuotaRemindEnabled',
  'QuotaRemindThreshold',
])

const SUCCESS_FLUSH_DELAY_MS = 120

type UpdateOptionMutationMeta = {
  silent?: boolean
  invalidate?: boolean
  successMessage?: string
}

type UpdateOptionMutationRequest = UpdateOptionRequest & {
  meta?: UpdateOptionMutationMeta
}

type UseUpdateOptionOptions = {
  silent?: boolean
  invalidate?: boolean
  successMessage?: string
}

export function useUpdateOption(options: UseUpdateOptionOptions = {}) {
  const queryClient = useQueryClient()
  const timerRef = useRef<number | null>(null)
  const pendingKeysRef = useRef<Set<string>>(new Set())
  const shouldInvalidateRef = useRef(false)
  const shouldRefreshStatusRef = useRef(false)
  const shouldToastRef = useRef(false)
  const successMessageRef = useRef<string | undefined>(undefined)

  const flushPendingSuccess = useCallback(() => {
    if (timerRef.current) {
      window.clearTimeout(timerRef.current)
      timerRef.current = null
    }

    if (pendingKeysRef.current.size === 0) {
      return
    }

    const shouldInvalidate = shouldInvalidateRef.current
    const shouldRefreshStatus = shouldRefreshStatusRef.current
    const shouldToast = shouldToastRef.current
    const successMessage =
      successMessageRef.current ?? 'Setting updated successfully'

    pendingKeysRef.current.clear()
    shouldInvalidateRef.current = false
    shouldRefreshStatusRef.current = false
    shouldToastRef.current = false
    successMessageRef.current = undefined

    if (shouldInvalidate) {
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
    }

    if (shouldRefreshStatus) {
      void queryClient.invalidateQueries({ queryKey: ['status'] })
      try {
        window.localStorage.removeItem('status')
      } catch {
        /* empty */
      }
    }

    if (shouldToast) {
      toast.success(i18next.t(successMessage))
    }
  }, [queryClient])

  const queueSuccessfulUpdate = useCallback(
    (variables: UpdateOptionMutationRequest) => {
      const shouldInvalidate = variables.meta?.invalidate ?? options.invalidate
      const shouldToast = !(variables.meta?.silent ?? options.silent ?? false)

      pendingKeysRef.current.add(variables.key)

      if (shouldInvalidate !== false) {
        shouldInvalidateRef.current = true

        if (STATUS_RELATED_KEYS.has(variables.key)) {
          shouldRefreshStatusRef.current = true
        }
      }

      if (shouldToast) {
        shouldToastRef.current = true
        successMessageRef.current =
          variables.meta?.successMessage ??
          options.successMessage ??
          successMessageRef.current
      }

      if (shouldInvalidate === false && !shouldToast) {
        return
      }

      if (timerRef.current) {
        window.clearTimeout(timerRef.current)
      }

      timerRef.current = window.setTimeout(
        flushPendingSuccess,
        SUCCESS_FLUSH_DELAY_MS
      )
    },
    [
      flushPendingSuccess,
      options.invalidate,
      options.silent,
      options.successMessage,
    ]
  )

  useEffect(() => {
    return () => flushPendingSuccess()
  }, [flushPendingSuccess])

  return useMutation({
    mutationFn: async (request: UpdateOptionMutationRequest) => {
      const { meta: _meta, ...payload } = request
      return requireServerSuccess(await updateSystemOption(payload))
    },
    onSuccess: (data, variables) => {
      if (data.success) {
        if (variables.key === 'DisplayTokenRankingEnabled') {
          const displayTokenRankingEnabled =
            variables.value === true || variables.value === 'true'
          useSystemConfigStore.getState().setConfig({
            displayTokenRankingEnabled,
          })
          queryClient.setQueryData<Record<string, unknown>>(
            ['status'],
            (current = {}) => ({
              ...current,
              display_token_ranking_enabled: displayTokenRankingEnabled,
            })
          )
        }
        if (variables.key === 'SidebarModulesAdmin') {
          queryClient.setQueryData<Record<string, unknown>>(
            ['status'],
            (current) =>
              current
                ? { ...current, SidebarModulesAdmin: variables.value }
                : current
          )
        }
        queueSuccessfulUpdate(variables)
      } else {
        toast.error(data.message || i18next.t('Failed to update setting'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Failed to update setting'))
    },
  })
}

export function useUpdatePasskeyDomains() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (request: UpdatePasskeyDomainsRequest) =>
      requireServerSuccess(await updatePasskeyDomains(request)),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
      void queryClient.invalidateQueries({ queryKey: ['status'] })
      try {
        window.localStorage.removeItem('status')
      } catch {
        /* Storage may be disabled. */
      }
      toast.success(i18next.t('Setting updated successfully'))
    },
    onError: (error: Error) =>
      handleServerError(error, i18next.t('Failed to update setting')),
  })
}

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

import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getSelf } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import {
  getServerMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

import { getAffiliateCode, transferAffiliateQuota } from '../api'
import { generateAffiliateLink } from '../lib'

// ============================================================================
// Affiliate Hook
// ============================================================================

export function useAffiliate(enabled = true) {
  const requestIdRef = useRef(0)
  const [affiliateCode, setAffiliateCode] = useState<string>('')
  const [affiliateLink, setAffiliateLink] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [transferring, setTransferring] = useState(false)
  const { copyToClipboard } = useCopyToClipboard()

  // Fetch affiliate code
  const fetchAffiliateCode = useCallback(async () => {
    if (!enabled) return
    const requestId = ++requestIdRef.current
    try {
      setLoading(true)
      const response = requireServerSuccess(await getAffiliateCode())
      if (requestId !== requestIdRef.current) return

      if (response.success && response.data) {
        setAffiliateCode(response.data)
        const link = generateAffiliateLink(response.data)
        setAffiliateLink(link)
      }
    } catch (error) {
      if (requestId === requestIdRef.current) handleServerError(error)
    } finally {
      if (requestId === requestIdRef.current) setLoading(false)
    }
  }, [enabled])

  // Copy affiliate link
  const copyAffiliateLink = useCallback(() => {
    copyToClipboard(affiliateLink)
  }, [affiliateLink, copyToClipboard])

  // Transfer affiliate quota to balance
  const transferQuota = useCallback(async (quota: number): Promise<boolean> => {
    try {
      setTransferring(true)
      const response = await transferAffiliateQuota({ quota })

      if (response.success) {
        toast.success(
          getServerMessage(response) || i18next.t('Transfer successful')
        )
        await getSelf()
        return true
      }

      handleServerError(response, i18next.t('Transfer failed'))
      return false
    } catch (_error) {
      handleServerError(_error, i18next.t('Transfer failed'))
      return false
    } finally {
      setTransferring(false)
    }
  }, [])

  useEffect(() => {
    if (!enabled) return
    void fetchAffiliateCode()
    return () => {
      requestIdRef.current += 1
    }
  }, [enabled, fetchAffiliateCode])

  return {
    affiliateCode,
    affiliateLink,
    loading: enabled && loading,
    transferring,
    copyAffiliateLink,
    transferQuota,
    refetch: fetchAffiliateCode,
  }
}

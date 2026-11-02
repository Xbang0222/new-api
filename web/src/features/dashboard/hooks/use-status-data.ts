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
import {
  isSidebarModuleContentVisibleForConfig,
  parseSidebarModuleLayers,
} from '@/hooks/use-sidebar-config'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

import type { AnnouncementItem, ApiInfoItem, FAQItem } from '../types'

/**
 * Get specific list from status data
 */
export function useStatusData<T = unknown>(
  enabledKey: string,
  dataKey: string
): { items: T[]; loading: boolean } {
  const { status, loading } = useStatus()
  const enabled = status ? status[enabledKey] !== false : false
  const items = (enabled ? status?.[dataKey] || [] : []) as T[]

  return { items, loading }
}

/**
 * Get API info list
 */
export function useApiInfo() {
  return useStatusData<ApiInfoItem>('api_info_enabled', 'api_info')
}

/**
 * Get announcements list
 */
export function useAnnouncements() {
  return useStatusData<AnnouncementItem>(
    'announcements_enabled',
    'announcements'
  )
}

/**
 * Get FAQ list
 */
export function useFAQ() {
  return useStatusData<FAQItem>('faq_enabled', 'faq')
}

/**
 * Get dashboard content panel visibility
 */
export function useDashboardContentVisibility() {
  const { status, ready } = useStatus()
  const user = useAuthStore((state) => state.auth.user)
  const { adminConfig, userConfig } = parseSidebarModuleLayers(status, user)
  const panels = Object.fromEntries(
    Object.keys(adminConfig.overview).map((key) => [
      key,
      ready &&
        isSidebarModuleContentVisibleForConfig(
          'overview',
          key,
          adminConfig,
          userConfig
        ),
    ])
  )

  return {
    summary: panels.summary,
    community: panels.community,
    performanceHealth: panels.performanceHealth,
    guide: panels.guide,
    invitationRanking: panels.invitationRanking,
    apiInfo: panels.apiInfo && status?.api_info_enabled !== false,
    announcements:
      panels.announcements && status?.announcements_enabled !== false,
    faq: panels.faq && status?.faq_enabled !== false,
    uptimeKuma: panels.uptimeKuma && status?.uptime_kuma_enabled !== false,
  }
}

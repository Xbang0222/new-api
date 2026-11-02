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
import { useLocation } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { resolveSidebarView } from '@/components/layout/lib/sidebar-view-registry'
import type { NavGroup, ResolvedSidebarView } from '@/components/layout/types'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { usePersonalSidebarRecoveryAllowed } from './use-personal-sidebar-recovery'
import { useSidebarConfig } from './use-sidebar-config'
import { useSidebarData } from './use-sidebar-data'

/** Sentinel key used for the root navigation in animation `key=` props */
const ROOT_VIEW_KEY = '__root'

/**
 * Resolve the active sidebar view for the current location.
 *
 * - Returns the matching nested {@link SidebarView} (with its nav
 *   groups) when the URL belongs to a registered drill-in workspace.
 * - Otherwise returns the root navigation, narrowed by:
 *     · admin-only group visibility (role-based);
 *     · `useSidebarConfig` (admin × user `sidebar_modules` overlay).
 *
 * Nested views use the same visibility preferences as the root navigation.
 * Route authorization remains enforced separately.
 */
export function useSidebarView(): ResolvedSidebarView {
  const { t } = useTranslation()
  const pathname = useLocation({ select: (l) => l.pathname })
  const userRole = useAuthStore((s) => s.auth.user?.role)
  const canRecoverPersonalSettings = usePersonalSidebarRecoveryAllowed()
  const rootSidebarData = useSidebarData()
  const configFilteredRoot = useSidebarConfig(rootSidebarData.navGroups)

  const rootNavGroups = useMemo<NavGroup[]>(() => {
    const role = userRole ?? ROLE.GUEST
    const isAdmin = role >= ROLE.ADMIN
    return configFilteredRoot
      .filter((group) => (group.id === 'admin' ? isAdmin : true))
      .map((group) => {
        const items = group.items.filter(
          (item) => item.requiredRole === undefined || role >= item.requiredRole
        )
        return items.length === group.items.length ? group : { ...group, items }
      })
  }, [configFilteredRoot, userRole])

  const view = resolveSidebarView(pathname)
  const nestedGroups = useMemo(() => view?.getNavGroups(t) ?? [], [view, t])
  const filteredNestedGroups = useSidebarConfig(nestedGroups)
  const navGroups = useMemo<NavGroup[]>(() => {
    const visibleGroups = view ? filteredNestedGroups : rootNavGroups
    const groups: NavGroup[] = canRecoverPersonalSettings
      ? [
          ...visibleGroups,
          {
            id: 'personal-module-settings-recovery',
            title: '',
            items: [
              {
                title: t('Sidebar Personal Settings'),
                url: '/profile/sidebar-modules',
              },
            ],
          },
        ]
      : visibleGroups
    const recoveryUrl = '/system-settings/site/sidebar-modules'
    if (
      userRole !== ROLE.SUPER_ADMIN ||
      groups.some((group) =>
        group.items.some((item) => 'url' in item && item.url === recoveryUrl)
      )
    ) {
      return groups
    }
    // The owner must always be able to restore visibility preferences.
    return [
      ...groups,
      {
        id: 'module-settings-recovery',
        title: '',
        items: [{ title: t('Sidebar modules'), url: recoveryUrl }],
      },
    ]
  }, [
    view,
    filteredNestedGroups,
    rootNavGroups,
    userRole,
    canRecoverPersonalSettings,
    t,
  ])

  if (view) {
    return {
      key: view.id,
      view,
      navGroups,
    }
  }

  return {
    key: ROOT_VIEW_KEY,
    view: null,
    navGroups,
  }
}

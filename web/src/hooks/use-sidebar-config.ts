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
import { useMemo } from 'react'

import type { NavGroup, NavItem } from '@/components/layout/types'
import {
  parseSidebarModulesAdmin,
  parseSidebarModulesUser,
  type SidebarModulesAdminConfig,
} from '@/features/system-settings/maintenance/config'
import { useStatus } from '@/hooks/use-status'
import { useAuthStore } from '@/stores/auth-store'

// User-layer config is shape-identical to admin, but may be null
// to signal "no narrowing" (empty/invalid/legacy users).
type SidebarModulesUserConfig = SidebarModulesAdminConfig | null

type SidebarConfigUser =
  | {
      sidebar_modules?: string
      permissions?: { sidebar_settings?: boolean }
    }
  | null
  | undefined

export type ConsoleHomePath =
  | '/dashboard/overview'
  | '/dashboard/models'
  | '/keys'
  | '/usage-logs/common'
  | '/usage-logs/task'
  | '/wallet'
  | '/playground'
  | '/profile'

const CONSOLE_HOME_CANDIDATES: ReadonlyArray<{
  section: string
  module: string
  path: ConsoleHomePath
}> = [
  { section: 'console', module: 'overview', path: '/dashboard/overview' },
  { section: 'console', module: 'dashboard', path: '/dashboard/models' },
  { section: 'console', module: 'token', path: '/keys' },
  { section: 'console', module: 'log', path: '/usage-logs/common' },
  { section: 'console', module: 'task', path: '/usage-logs/task' },
  { section: 'personal', module: 'topup', path: '/wallet' },
  { section: 'chat', module: 'playground', path: '/playground' },
  { section: 'personal', module: 'personal', path: '/profile' },
]

export function parseSidebarModuleLayers(
  status: unknown,
  user: SidebarConfigUser
): {
  adminConfig: SidebarModulesAdminConfig
  userConfig: SidebarModulesUserConfig
} {
  const rawAdminConfig =
    status &&
    typeof status === 'object' &&
    'SidebarModulesAdmin' in status &&
    (typeof status.SidebarModulesAdmin === 'string' ||
      status.SidebarModulesAdmin === null)
      ? status.SidebarModulesAdmin
      : undefined
  const adminConfig = parseSidebarModulesAdmin(rawAdminConfig)
  const userConfig =
    user?.permissions?.sidebar_settings === false
      ? null
      : parseSidebarModulesUser(user?.sidebar_modules)

  return { adminConfig, userConfig }
}

export function resolveConsoleHomePath(
  adminConfig: SidebarModulesAdminConfig,
  userConfig: SidebarModulesUserConfig,
  preferredPath?: string
): ConsoleHomePath {
  const preferred = CONSOLE_HOME_CANDIDATES.find(
    (candidate) => candidate.path === preferredPath
  )
  if (
    preferred &&
    isSidebarModuleEnabledForConfig(
      preferred.section,
      preferred.module,
      adminConfig,
      userConfig
    )
  ) {
    return preferred.path
  }

  const destination = CONSOLE_HOME_CANDIDATES.find((candidate) =>
    isSidebarModuleEnabledForConfig(
      candidate.section,
      candidate.module,
      adminConfig,
      userConfig
    )
  )

  return destination?.path ?? '/profile'
}

/**
 * Mapping from URL to configuration keys
 */
export const URL_TO_CONFIG_MAP: Record<
  string,
  { section: string; module: string }
> = {
  '/playground': { section: 'chat', module: 'playground' },
  '/dashboard': { section: 'console', module: 'overview' },
  '/dashboard/overview': { section: 'console', module: 'overview' },
  '/dashboard/models': { section: 'console', module: 'dashboard' },
  '/dashboard/users': { section: 'console', module: 'dashboard' },
  '/keys': { section: 'console', module: 'token' },
  '/usage-logs': { section: 'console', module: 'log' },
  '/usage-logs/common': { section: 'console', module: 'log' },
  '/usage-logs/audit': { section: 'console', module: 'audit' },
  '/usage-logs/drawing': { section: 'console', module: 'task' },
  '/usage-logs/task': { section: 'console', module: 'task' },
  '/wallet': { section: 'personal', module: 'topup' },
  '/invoices': { section: 'personal', module: 'invoice' },
  '/profile': { section: 'personal', module: 'personal' },
  '/security': { section: 'personal', module: 'security' },
  '/channels': { section: 'admin', module: 'channel' },
  '/channel-scheduling': { section: 'admin', module: 'channelScheduling' },
  '/models': { section: 'admin', module: 'models' },
  '/models/metadata': { section: 'admin', module: 'models' },
  '/models/deployments': { section: 'admin', module: 'models' },
  '/users': { section: 'admin', module: 'user' },
  '/redemption-codes': { section: 'admin', module: 'redemption' },
  '/subscriptions': { section: 'admin', module: 'subscription' },
  '/invoice-management': { section: 'admin', module: 'invoiceManagement' },
  '/system-info': { section: 'admin', module: 'systemInfo' },
  '/system-settings': { section: 'admin', module: 'setting' },
  '/system-settings/site': { section: 'admin', module: 'setting' },
}

/**
 * Check if a module is enabled. Admin config is the first (authoritative)
 * layer: if admin disables a section/module it is always hidden. User config
 * is a second narrower layer: it can only further hide what admin allowed.
 * A null user config means "do not narrow" (legacy/empty users).
 */
export function isSidebarModuleVisibleForConfig(
  url: string,
  adminConfig: SidebarModulesAdminConfig,
  userConfig: SidebarModulesUserConfig
): boolean {
  const mapping = URL_TO_CONFIG_MAP[url]
  if (!mapping) {
    // Non-sidebar/internal routes remain visible unless explicitly registered.
    return true
  }

  return isSidebarModuleEnabledForConfig(
    mapping.section,
    mapping.module,
    adminConfig,
    userConfig
  )
}

export function isSidebarModuleEnabledForConfig(
  section: string,
  module: string,
  adminConfig: SidebarModulesAdminConfig,
  userConfig: SidebarModulesUserConfig
): boolean {
  const adminSection = adminConfig[section]
  if (!adminSection?.enabled || adminSection[module] !== true) return false

  if (
    section === 'console' &&
    module === 'setupGuide' &&
    adminSection.overview !== true
  ) {
    return false
  }

  const userSection = userConfig?.[section]
  if (!userSection) return true
  if (userSection.enabled === false || userSection[module] === false) {
    return false
  }

  if (section === 'console' && module === 'setupGuide') {
    return userSection.overview !== false
  }

  return true
}

/**
 * Check if a navigation item should be visible
 */
function isNavItemVisible(
  item: NavItem,
  adminConfig: SidebarModulesAdminConfig,
  userConfig: SidebarModulesUserConfig
): boolean {
  // Handle dynamic chat presets type — also runs the admin × user AND gate
  if ('type' in item && item.type === 'chat-presets') {
    const adminChat = adminConfig.chat
    const adminAllowed = Boolean(adminChat?.enabled && adminChat.chat === true)
    if (!adminAllowed) return false
    if (!userConfig) return true
    const userChat = userConfig.chat
    if (!userChat) return true
    if (userChat.enabled === false) return false
    return userChat.chat !== false
  }

  // Handle direct link type
  if ('url' in item && item.url) {
    const configUrls = item.configUrls ?? [item.url]
    return configUrls.some((url) =>
      isSidebarModuleVisibleForConfig(url as string, adminConfig, userConfig)
    )
  }

  // Handle collapsible type (with sub-items)
  if ('items' in item && item.items) {
    // If has sub-items, show this collapsible item if at least one sub-item is visible
    return item.items.some((subItem) =>
      isSidebarModuleVisibleForConfig(
        subItem.url as string,
        adminConfig,
        userConfig
      )
    )
  }

  return true
}

/**
 * Filter navigation items
 */
function filterNavItems(
  items: NavItem[],
  adminConfig: SidebarModulesAdminConfig,
  userConfig: SidebarModulesUserConfig
): NavItem[] {
  return items
    .map((item) => {
      // If collapsible item, also filter its sub-items
      if ('items' in item && item.items) {
        const filteredSubItems = item.items.filter((subItem) =>
          isSidebarModuleVisibleForConfig(
            subItem.url as string,
            adminConfig,
            userConfig
          )
        )

        return {
          ...item,
          items: filteredSubItems,
        }
      }
      return item
    })
    .filter((item) => isNavItemVisible(item, adminConfig, userConfig))
}

/**
 * Filter sidebar navigation groups by admin × user sidebar_modules config.
 *
 * Two layers, AND-combined:
 *   1. Admin (status.SidebarModulesAdmin) — authoritative, falls back to
 *      DEFAULT_SIDEBAR_MODULES when empty/invalid. Disabling here hides the
 *      item for everyone regardless of user preference.
 *   2. User (auth.user.sidebar_modules) — narrower overlay, null sentinel
 *      means "don't narrow". A section/module is only hidden if the user
 *      explicitly set it to false; undefined fields default to visible so
 *      legacy users with empty sidebar_modules keep the full admin view.
 *      The overlay is also skipped entirely when the backend tells us the
 *      user cannot configure sidebar_settings (e.g. root accounts), so a
 *      stale historical value cannot lock them out of entries they have no
 *      UI to restore.
 */
export function useSidebarConfig(navGroups: NavGroup[]): NavGroup[] {
  const { status } = useStatus()
  const { auth } = useAuthStore()

  const { adminConfig, userConfig } = useMemo(
    () => parseSidebarModuleLayers(status, auth?.user),
    [status, auth?.user]
  )

  const filteredNavGroups = useMemo(
    () =>
      navGroups
        .map((group) => ({
          ...group,
          items: filterNavItems(group.items, adminConfig, userConfig),
        }))
        .filter((group) => group.items.length > 0), // Only show navigation groups with visible items
    [navGroups, adminConfig, userConfig]
  )

  return filteredNavGroups
}

/**
 * Check whether a single route is visible under the current sidebar_modules
 * config. Used by entries living outside the sidebar (e.g. the profile
 * dropdown's wallet link) so they honour the same "wallet display" toggle.
 */
export function useIsSidebarModuleVisible(url: string): boolean {
  const { status } = useStatus()
  const { auth } = useAuthStore()

  const { adminConfig, userConfig } = parseSidebarModuleLayers(
    status,
    auth?.user
  )

  return isSidebarModuleVisibleForConfig(url, adminConfig, userConfig)
}

export function useIsSidebarModuleEnabled(
  section: string,
  module: string
): boolean {
  const { status } = useStatus()
  const { auth } = useAuthStore()
  const { adminConfig, userConfig } = parseSidebarModuleLayers(
    status,
    auth?.user
  )
  return isSidebarModuleEnabledForConfig(
    section,
    module,
    adminConfig,
    userConfig
  )
}

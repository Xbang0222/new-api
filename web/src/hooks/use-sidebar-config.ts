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
import { useCallback, useMemo } from 'react'

import type { NavGroup, NavItem } from '@/components/layout/types'
import {
  parseSidebarModulesAdmin,
  type SidebarModulesAdminConfig,
} from '@/features/system-settings/maintenance/config'
import {
  SIDEBAR_CHILD_SECTIONS,
  getSidebarChildSection,
} from '@/features/system-settings/maintenance/sidebar-child-sections'
import { useStatus } from '@/hooks/use-status'

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

/** Sidebar module visibility is configured by administrators only. */
export function getSidebarModulesAdminConfig(
  status: unknown
): SidebarModulesAdminConfig {
  const rawAdminConfig =
    status &&
    typeof status === 'object' &&
    'SidebarModulesAdmin' in status &&
    (typeof status.SidebarModulesAdmin === 'string' ||
      status.SidebarModulesAdmin === null)
      ? status.SidebarModulesAdmin
      : undefined
  return parseSidebarModulesAdmin(rawAdminConfig)
}

export function resolveConsoleHomePath(
  adminConfig: SidebarModulesAdminConfig,
  preferredPath?: string
): ConsoleHomePath {
  const preferred = CONSOLE_HOME_CANDIDATES.find(
    (candidate) => candidate.path === preferredPath
  )
  if (
    preferred &&
    isSidebarModuleVisibleForConfig(preferred.path, adminConfig)
  ) {
    return preferred.path
  }

  const destination = CONSOLE_HOME_CANDIDATES.find((candidate) =>
    isSidebarModuleVisibleForConfig(candidate.path, adminConfig)
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
  '/chat': { section: 'chat', module: 'chat' },
  '/chat2link': { section: 'chat', module: 'chat' },
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
  '/task-plugins': { section: 'admin', module: 'taskPlugins' },
}

for (const [section, config] of Object.entries(SIDEBAR_CHILD_SECTIONS)) {
  for (const module of config.modules) {
    if (module.url) {
      URL_TO_CONFIG_MAP[module.url] = { section, module: module.key }
    }
  }
}

/**
 * Check whether the administrator's sidebar configuration shows a route.
 */
export function isSidebarModuleVisibleForConfig(
  url: string,
  adminConfig: SidebarModulesAdminConfig
): boolean {
  const normalizedUrl = url.split('?')[0].replace(/\/$/, '')
  const mapping =
    URL_TO_CONFIG_MAP[normalizedUrl] ??
    Object.entries(URL_TO_CONFIG_MAP)
      .filter(([path]) => normalizedUrl.startsWith(`${path}/`))
      .sort(([a], [b]) => b.length - a.length)[0]?.[1]
  if (!mapping) {
    // Non-sidebar/internal routes remain visible unless explicitly registered.
    return true
  }

  return isSidebarModuleContentVisibleForConfig(
    mapping.section,
    mapping.module,
    adminConfig
  )
}

/** A parent with no visible leaf content must not leave an empty page or card. */
export function isSidebarModuleContentVisibleForConfig(
  section: string,
  module: string,
  adminConfig: SidebarModulesAdminConfig
): boolean {
  if (!isSidebarModuleEnabledForConfig(section, module, adminConfig)) {
    return false
  }
  const childEntry = getSidebarChildSection(section, module)
  return (
    !childEntry ||
    childEntry[1].modules.some((child) =>
      isSidebarModuleContentVisibleForConfig(
        childEntry[0],
        child.key,
        adminConfig
      )
    )
  )
}

export function isSidebarModuleEnabledForConfig(
  section: string,
  module: string,
  adminConfig: SidebarModulesAdminConfig
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

  const parent = SIDEBAR_CHILD_SECTIONS[section]
  if (
    parent &&
    !isSidebarModuleEnabledForConfig(
      parent.parentSection,
      parent.parentModule,
      adminConfig
    )
  ) {
    return false
  }

  return true
}

/**
 * Check if a navigation item should be visible
 */
function isNavItemVisible(
  item: NavItem,
  adminConfig: SidebarModulesAdminConfig
): boolean {
  // Handle dynamic chat presets type
  if ('type' in item && item.type === 'chat-presets') {
    const adminChat = adminConfig.chat
    return Boolean(adminChat?.enabled && adminChat.chat === true)
  }

  // Handle direct link type
  if ('url' in item && item.url) {
    const configUrls = item.configUrls ?? [item.url]
    return configUrls.some((url) =>
      isSidebarModuleVisibleForConfig(url as string, adminConfig)
    )
  }

  // Handle collapsible type (with sub-items)
  if ('items' in item && item.items) {
    // If has sub-items, show this collapsible item if at least one sub-item is visible
    return item.items.some((subItem) =>
      isSidebarModuleVisibleForConfig(subItem.url as string, adminConfig)
    )
  }

  return true
}

/**
 * Filter navigation items
 */
function filterNavItems(
  items: NavItem[],
  adminConfig: SidebarModulesAdminConfig
): NavItem[] {
  return items
    .map((item) => {
      if (
        item.type !== 'chat-presets' &&
        typeof item.url === 'string' &&
        item.configUrls &&
        !isSidebarModuleVisibleForConfig(item.url as string, adminConfig)
      ) {
        const url = item.configUrls.find((candidate) =>
          isSidebarModuleVisibleForConfig(candidate as string, adminConfig)
        )
        if (url) return { ...item, url }
      }
      // If collapsible item, also filter its sub-items
      if ('items' in item && item.items) {
        const filteredSubItems = item.items.filter((subItem) =>
          isSidebarModuleVisibleForConfig(subItem.url as string, adminConfig)
        )

        return {
          ...item,
          items: filteredSubItems,
        }
      }
      return item
    })
    .filter((item) => isNavItemVisible(item, adminConfig))
}

/**
 * Filter sidebar navigation groups by the administrator's
 * status.SidebarModulesAdmin, which falls back to the defaults when
 * empty/invalid. Saved personal sidebar preferences are ignored.
 */
export function useSidebarConfig(navGroups: NavGroup[]): NavGroup[] {
  const { status, ready } = useStatus()
  const adminConfig = useMemo(
    () => getSidebarModulesAdminConfig(status),
    [status]
  )

  const filteredNavGroups = useMemo(
    () =>
      navGroups
        .map((group) => ({
          ...group,
          items: filterNavItems(group.items, adminConfig),
        }))
        .filter((group) => group.items.length > 0), // Only show navigation groups with visible items
    [navGroups, adminConfig]
  )

  return ready ? filteredNavGroups : []
}

/**
 * Check whether a single route is visible under the administrator's sidebar
 * configuration. Used by entries living outside the sidebar (e.g. the profile
 * dropdown's wallet link) so they honour the same "wallet display" toggle.
 */
export function useIsSidebarModuleVisible(url: string): boolean {
  const { status, ready } = useStatus()
  return (
    ready &&
    isSidebarModuleVisibleForConfig(url, getSidebarModulesAdminConfig(status))
  )
}

export function useIsSidebarModuleEnabled(
  section: string,
  module: string
): boolean {
  const { status, ready } = useStatus()
  return (
    ready &&
    isSidebarModuleContentVisibleForConfig(
      section,
      module,
      getSidebarModulesAdminConfig(status)
    )
  )
}

/** Visibility preferences only; server authorization remains authoritative. */
export function useSidebarModules() {
  const { status, ready } = useStatus()
  const adminConfig = useMemo(
    () => getSidebarModulesAdminConfig(status),
    [status]
  )
  return useCallback(
    (section: string, module: string) =>
      ready &&
      isSidebarModuleContentVisibleForConfig(section, module, adminConfig),
    [ready, adminConfig]
  )
}

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
export type HeaderNavAccessConfig = {
  enabled: boolean
  requireAuth: boolean
}

export type HeaderNavDocsConfig = {
  enabled: boolean
  openInNewTab: boolean
}

export type HeaderNavCanvasConfig = {
  enabled: boolean
  url: string
  openInNewTab: boolean
}

export type HeaderNavModulesConfig = {
  home: boolean
  console: boolean
  pricing: HeaderNavAccessConfig
  rankings: HeaderNavAccessConfig
  docs: HeaderNavDocsConfig
  canvas: HeaderNavCanvasConfig
  about: boolean
  [key: string]:
    | boolean
    | HeaderNavAccessConfig
    | HeaderNavDocsConfig
    | HeaderNavCanvasConfig
}

export type SidebarSectionConfig = {
  enabled: boolean
  [key: string]: boolean
}

export type SidebarModulesAdminConfig = Record<string, SidebarSectionConfig>

export const HEADER_NAV_DEFAULT: HeaderNavModulesConfig = {
  home: true,
  console: true,
  pricing: {
    enabled: true,
    requireAuth: false,
  },
  rankings: {
    enabled: true,
    requireAuth: false,
  },
  docs: {
    enabled: true,
    openInNewTab: false,
  },
  canvas: {
    enabled: false,
    url: '',
    openInNewTab: false,
  },
  about: true,
}

export const SIDEBAR_MODULES_DEFAULT: SidebarModulesAdminConfig = {
  chat: {
    enabled: true,
    playground: true,
    chat: true,
  },
  console: {
    enabled: true,
    overview: true,
    dashboard: true,
    token: true,
    log: true,
    audit: true,
    task: true,
  },
  personal: {
    enabled: true,
    invoice: true,
    topup: true,
    personal: true,
    security: true,
  },
  admin: {
    enabled: true,
    channel: true,
    channelScheduling: true,
    models: true,
    redemption: true,
    user: true,
    setting: true,
    subscription: true,
    invoiceManagement: true,
    systemInfo: true,
  },
}

const toBoolean = (value: unknown, fallback: boolean): boolean => {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value === 1
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return fallback
}

const cloneHeaderNavDefault = (): HeaderNavModulesConfig => ({
  ...HEADER_NAV_DEFAULT,
  pricing: { ...HEADER_NAV_DEFAULT.pricing },
  rankings: { ...HEADER_NAV_DEFAULT.rankings },
  docs: { ...HEADER_NAV_DEFAULT.docs },
  canvas: { ...HEADER_NAV_DEFAULT.canvas },
})

const parseAccessModule = (
  raw: unknown,
  fallback: HeaderNavAccessConfig
): HeaderNavAccessConfig => {
  if (
    typeof raw === 'boolean' ||
    typeof raw === 'string' ||
    typeof raw === 'number'
  ) {
    return {
      enabled: toBoolean(raw, fallback.enabled),
      requireAuth: fallback.requireAuth,
    }
  }
  if (raw && typeof raw === 'object') {
    const record = raw as Record<string, unknown>
    return {
      enabled: toBoolean(record.enabled, fallback.enabled),
      requireAuth: toBoolean(record.requireAuth, fallback.requireAuth),
    }
  }
  return { ...fallback }
}

const parseDocsModule = (
  raw: unknown,
  fallback: HeaderNavDocsConfig
): HeaderNavDocsConfig => {
  if (
    typeof raw === 'boolean' ||
    typeof raw === 'string' ||
    typeof raw === 'number'
  ) {
    return {
      enabled: toBoolean(raw, fallback.enabled),
      openInNewTab: fallback.openInNewTab,
    }
  }
  if (raw && typeof raw === 'object') {
    const record = raw as Record<string, unknown>
    return {
      enabled: toBoolean(record.enabled, fallback.enabled),
      openInNewTab: toBoolean(record.openInNewTab, fallback.openInNewTab),
    }
  }
  return { ...fallback }
}

const parseCanvasModule = (
  raw: unknown,
  fallback: HeaderNavCanvasConfig
): HeaderNavCanvasConfig => {
  if (raw && typeof raw === 'object') {
    const record = raw as Record<string, unknown>
    return {
      enabled: toBoolean(record.enabled, fallback.enabled),
      url: typeof record.url === 'string' ? record.url : fallback.url,
      openInNewTab: toBoolean(record.openInNewTab, fallback.openInNewTab),
    }
  }
  return { ...fallback }
}

const cloneSidebarDefault = (): SidebarModulesAdminConfig =>
  Object.entries(SIDEBAR_MODULES_DEFAULT).reduce<SidebarModulesAdminConfig>(
    (acc, [section, config]) => {
      acc[section] = { ...config }
      return acc
    },
    {}
  )

export function parseHeaderNavModules(
  value: string | null | undefined
): HeaderNavModulesConfig {
  const base = cloneHeaderNavDefault()
  if (!value) {
    return base
  }
  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    const result: HeaderNavModulesConfig = {
      ...base,
      pricing: { ...base.pricing },
      rankings: { ...base.rankings },
      docs: { ...base.docs },
      canvas: { ...base.canvas },
    }

    Object.entries(parsed).forEach(([key, raw]) => {
      if (key === 'pricing') {
        result.pricing = parseAccessModule(raw, base.pricing)
        return
      }
      if (key === 'rankings') {
        result.rankings = parseAccessModule(raw, base.rankings)
        return
      }
      if (key === 'docs') {
        result.docs = parseDocsModule(raw, base.docs)
        return
      }
      if (key === 'canvas') {
        result.canvas = parseCanvasModule(raw, base.canvas)
        return
      }
      if (typeof raw === 'boolean') {
        result[key] = raw
        return
      }
      if (typeof raw === 'string' || typeof raw === 'number') {
        result[key] = toBoolean(raw, Boolean(base[key]))
        return
      }
    })

    return result
  } catch {
    return base
  }
}

export function serializeHeaderNavModules(
  config: HeaderNavModulesConfig
): string {
  return JSON.stringify(config)
}

export function parseSidebarModulesAdmin(
  value: string | null | undefined
): SidebarModulesAdminConfig {
  const defaults = cloneSidebarDefault()
  // If empty string, null, or undefined, use default config
  if (!value || value.trim() === '') return defaults

  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    const result: SidebarModulesAdminConfig = {}

    Object.entries(parsed).forEach(([sectionKey, raw]) => {
      if (!raw || typeof raw !== 'object') return

      const defaultSection = defaults[sectionKey] ?? { enabled: true }
      const sectionConfig: SidebarSectionConfig = {
        enabled: toBoolean(
          (raw as Record<string, unknown>).enabled,
          defaultSection.enabled ?? true
        ),
      }

      Object.entries(raw as Record<string, unknown>).forEach(
        ([moduleKey, moduleValue]) => {
          if (moduleKey === 'enabled' || moduleKey === 'setupGuide') return
          sectionConfig[moduleKey] = toBoolean(
            moduleValue,
            defaultSection[moduleKey] ?? true
          )
        }
      )

      const rawSection = raw as Record<string, unknown>
      // custom: sidebar modules — migrate legacy combined keys without
      // exposing them as duplicate controls in the current settings UI.
      if (sectionKey === 'console') {
        if (!('overview' in rawSection) && 'detail' in rawSection) {
          sectionConfig.overview = toBoolean(rawSection.detail, true)
        }
        if (!('dashboard' in rawSection) && 'detail' in rawSection) {
          sectionConfig.dashboard = toBoolean(rawSection.detail, true)
        }
        if (!('task' in rawSection) && 'midjourney' in rawSection) {
          sectionConfig.task = toBoolean(rawSection.midjourney, true)
        }
        delete sectionConfig.detail
        delete sectionConfig.midjourney
      }

      result[sectionKey] = sectionConfig
    })

    // Merge defaults to ensure expected sections exist
    Object.entries(defaults).forEach(([sectionKey, config]) => {
      if (!result[sectionKey]) {
        result[sectionKey] = { ...config }
        return
      }

      Object.entries(config).forEach(([moduleKey, moduleValue]) => {
        if (!(moduleKey in result[sectionKey])) {
          result[sectionKey][moduleKey] = moduleValue
        }
      })
    })

    return result
  } catch {
    return defaults
  }
}

/** Parse the optional user overlay without adding default entries. */
export function parseSidebarModulesUser(
  value: string | null | undefined
): SidebarModulesAdminConfig | null {
  if (!value || value.trim() === '') return null

  try {
    const parsed = JSON.parse(value) as Record<string, unknown>
    if (!parsed || typeof parsed !== 'object') return null

    const result: SidebarModulesAdminConfig = {}
    Object.entries(parsed).forEach(([sectionKey, raw]) => {
      if (!raw || typeof raw !== 'object') return
      const rawSection = raw as Record<string, unknown>
      const sectionConfig: SidebarSectionConfig = {
        enabled: toBoolean(rawSection.enabled, true),
      }
      Object.entries(rawSection).forEach(([moduleKey, moduleValue]) => {
        if (moduleKey !== 'enabled' && moduleKey !== 'setupGuide') {
          sectionConfig[moduleKey] = toBoolean(moduleValue, true)
        }
      })
      if (sectionKey === 'console') {
        if (!('overview' in rawSection) && 'detail' in rawSection) {
          sectionConfig.overview = toBoolean(rawSection.detail, true)
        }
        if (!('dashboard' in rawSection) && 'detail' in rawSection) {
          sectionConfig.dashboard = toBoolean(rawSection.detail, true)
        }
        if (!('task' in rawSection) && 'midjourney' in rawSection) {
          sectionConfig.task = toBoolean(rawSection.midjourney, true)
        }
        delete sectionConfig.detail
        delete sectionConfig.midjourney
      }
      result[sectionKey] = sectionConfig
    })
    return result
  } catch {
    return null
  }
}

export function serializeSidebarModulesAdmin(
  config: SidebarModulesAdminConfig
): string {
  return JSON.stringify(config)
}

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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { checkIsActive } from '@/components/layout/lib/url-utils'
import { useDashboardContentVisibility } from '@/features/dashboard/hooks/use-status-data'
import {
  parseSidebarModulesAdmin,
  serializeSidebarModulesAdmin,
} from '@/features/system-settings/maintenance/config'
import { SIDEBAR_CHILD_SECTIONS } from '@/features/system-settings/maintenance/sidebar-child-sections'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import {
  useSidebarConfig,
  useSidebarModules,
  useIsSidebarModuleVisible,
  isSidebarModuleVisibleForConfig,
  isSidebarModuleEnabledForConfig,
} from '../use-sidebar-config'
import { useSidebarData } from '../use-sidebar-data'

beforeEach(() => {
  vi.stubGlobal('localStorage', {
    getItem: () => null,
    setItem: () => undefined,
    removeItem: () => undefined,
  })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  useAuthStore.getState().auth.reset()
})

function sidebarFor(admin?: object, user?: object, canConfigure = true) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['status'], {
    SidebarModulesAdmin: admin ? JSON.stringify(admin) : '',
  })
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'alice',
    role: 1,
    permissions: { sidebar_settings: canConfigure },
    sidebar_modules: user ? JSON.stringify(user) : '',
  })
  function Wrapper(props: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        {props.children}
      </QueryClientProvider>
    )
  }
  const result = renderHook(
    () => useSidebarConfig(useSidebarData().navGroups),
    { wrapper: Wrapper }
  )
  return result
}

describe('security sidebar visibility', () => {
  it('old configurations show Security & Access immediately after Profile and keep API Keys', () => {
    const { result } = sidebarFor(
      { personal: { enabled: true, personal: true, topup: true } },
      { personal: { enabled: true, personal: true } }
    )
    const personalItems =
      result.current
        .find((group) => group.id === 'personal')
        ?.items.map((item) => item.title) ?? []
    expect(personalItems).toContain('Security & Access')
    expect(personalItems.indexOf('Security & Access')).toBe(
      personalItems.indexOf('Profile') + 1
    )
    expect(
      result.current
        .flatMap((group) => group.items)
        .some((item) => item.title === 'API Keys')
    ).toBe(true)
  })
  it.each([
    [{ personal: { enabled: true, security: false } }, undefined],
    [{ personal: { enabled: false } }, { personal: { security: true } }],
    [undefined, { personal: { enabled: true, security: false } }],
    [undefined, { personal: { enabled: false } }],
  ])(
    'admin or user disablement hides Security & Access (%j, %j)',
    (admin, user) => {
      const { result } = sidebarFor(admin, user)
      expect(
        result.current
          .flatMap((group) => group.items)
          .some((item) => item.title === 'Security & Access')
      ).toBe(false)
    }
  )
  it('users without sidebar configuration permission retain the admin view', () => {
    const { result } = sidebarFor(
      undefined,
      { personal: { security: false } },
      false
    )
    expect(
      result.current
        .flatMap((group) => group.items)
        .some((item) => item.title === 'Security & Access')
    ).toBe(true)
  })
})

describe('audit log sidebar entry', () => {
  it('admin settings default Audit Logs to visible and preserve its independent toggle when saved', () => {
    const config = parseSidebarModulesAdmin(
      '{"console":{"enabled":true,"log":true}}'
    )
    expect(config.console.audit).toBe(true)
    config.console.audit = false
    const saved = parseSidebarModulesAdmin(serializeSidebarModulesAdmin(config))
    const { result } = sidebarFor(saved)
    const titles = result.current
      .flatMap((group) => group.items)
      .map((item) => item.title)
    expect(titles).not.toContain('Audit Logs')
    expect(titles).toContain('Usage Logs')
  })

  it('legacy configurations show a separate Audit Logs link immediately after Usage Logs', () => {
    const { result } = sidebarFor(
      { console: { enabled: true, log: true } },
      { console: { enabled: true, log: true } }
    )
    const items =
      result.current.find((group) => group.id === 'general')?.items ?? []
    const usageIndex = items.findIndex((item) => item.title === 'Usage Logs')
    expect(items[usageIndex + 1]).toMatchObject({
      title: 'Audit Logs',
      url: '/usage-logs/audit',
    })
    const selected = items.filter((item) =>
      checkIsActive('/usage-logs/audit', item)
    )
    expect(selected.map((item) => item.title)).toEqual(['Audit Logs'])
  })

  it.each([
    [{ console: { enabled: true, audit: false } }, undefined],
    [{ console: { enabled: false } }, { console: { audit: true } }],
    [undefined, { console: { enabled: true, audit: false } }],
    [undefined, { console: { enabled: false } }],
  ])(
    'admin and personal visibility rules can hide Audit Logs (%j, %j)',
    (admin, user) => {
      const { result } = sidebarFor(admin, user)
      expect(
        result.current
          .flatMap((group) => group.items)
          .some((item) => item.title === 'Audit Logs')
      ).toBe(false)
    }
  )

  it('hiding Usage Logs does not hide the independently configured Audit Logs entry', () => {
    const { result } = sidebarFor({ console: { enabled: true, log: false } })
    const titles = result.current
      .flatMap((group) => group.items)
      .map((item) => item.title)
    expect(titles).not.toContain('Usage Logs')
    expect(titles).toContain('Audit Logs')
  })
})

const overviewCards = [
  'summary',
  'community',
  'invitationRanking',
  'performanceHealth',
  'announcements',
  'apiInfo',
  'faq',
  'uptimeKuma',
  'guide',
] as const

it.each(overviewCards)(
  'hides only the %s overview card when its switch is off',
  (card) => {
    const client = new QueryClient()
    const overview = Object.fromEntries(
      overviewCards.map((key) => [key, key !== card])
    )
    client.setQueryData(['status'], {
      SidebarModulesAdmin: JSON.stringify({ overview }),
    })
    const { result, unmount } = renderHook(
      () => useDashboardContentVisibility(),
      {
        wrapper: ({ children }) => (
          <QueryClientProvider client={client}>{children}</QueryClientProvider>
        ),
      }
    )
    for (const key of overviewCards) {
      expect(result.current[key]).toBe(key !== card)
    }
    unmount()
    client.clear()
  }
)

it.each([
  { console: { enabled: false } },
  { console: { overview: false } },
  { overview: { enabled: false, invitationRanking: true } },
])('hides every overview card when its parent is disabled: %j', (config) => {
  const client = new QueryClient()
  client.setQueryData(['status'], {
    SidebarModulesAdmin: JSON.stringify(config),
  })
  const { result, unmount } = renderHook(
    () => useDashboardContentVisibility(),
    {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    }
  )
  for (const key of overviewCards) expect(result.current[key]).toBe(false)
  unmount()
  client.clear()
})

it.each(Object.entries(SIDEBAR_CHILD_SECTIONS))(
  'enforces administrator and parent switches for every %s child',
  (section, definition) => {
    const admin = parseSidebarModulesAdmin('')
    for (const child of definition.modules) {
      admin[section][child.key] = true
      expect(
        isSidebarModuleEnabledForConfig(section, child.key, admin, null)
      ).toBe(true)
      admin[section][child.key] = false
      const user = { [section]: { enabled: true, [child.key]: true } }
      expect(
        isSidebarModuleEnabledForConfig(section, child.key, admin, user)
      ).toBe(false)
      admin[section][child.key] = true
      admin[definition.parentSection][definition.parentModule] = false
      expect(
        isSidebarModuleEnabledForConfig(section, child.key, admin, user)
      ).toBe(false)
      admin[definition.parentSection][definition.parentModule] = true
    }
  }
)

it.each([
  ['/dashboard/flow', 'dashboard', 'flow'],
  ['/models/vendors', 'models', 'vendors'],
  ['/usage-logs/drawing', 'taskLogs', 'drawing'],
  ['/system-settings/billing/payment', 'systemSettings', 'billing'],
  ['/task-plugins', 'admin', 'taskPlugins'],
  ['/chat/123', 'chat', 'chat'],
])(
  'hides the disabled destination %s even when opened directly',
  (url, section, key) => {
    const admin = parseSidebarModulesAdmin('')
    expect(isSidebarModuleVisibleForConfig(url, admin, null)).toBe(true)
    admin[section][key] = false
    expect(isSidebarModuleVisibleForConfig(url, admin, null)).toBe(false)
  }
)

it('keeps the model management entry pointing to an enabled child when the default tab is hidden', () => {
  const { result } = sidebarFor({ models: { metadata: false, vendors: true } })
  const models = result.current
    .flatMap((group) => group.items)
    .find((item) => item.title === 'Models')
  expect(models).toHaveProperty('url', '/models/vendors')
})

it('hides wallet navigation when all of its cards are disabled', () => {
  const { result } = sidebarFor({
    wallet: {
      summary: false,
      recharge: false,
      subscriptions: false,
      affiliate: false,
    },
  })
  expect(
    result.current
      .flatMap((group) => group.items)
      .some((item) => item.title === 'Wallet')
  ).toBe(false)
})

it('does not enable modules before status resolves, and applies later settings immediately', async () => {
  let resolveStatus!: (value: unknown) => void
  const statusRequest = new Promise((resolve) => {
    resolveStatus = resolve
  })
  vi.spyOn(api, 'get').mockReturnValue(statusRequest)
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const { result, unmount } = renderHook(
    () => {
      const enabled = useSidebarModules()
      return {
        security: useIsSidebarModuleVisible('/security'),
        sessions: enabled('security', 'sessions'),
        profile: enabled('profile', 'details'),
      }
    },
    {
      wrapper: ({ children }) => (
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      ),
    }
  )
  expect(result.current).toEqual({
    security: false,
    sessions: false,
    profile: false,
  })
  await act(async () => {
    resolveStatus({
      data: {
        success: true,
        data: {
          SidebarModulesAdmin: JSON.stringify({
            security: { sessions: false },
          }),
        },
      },
    })
    await statusRequest
  })
  await waitFor(() =>
    expect(result.current).toEqual({
      security: true,
      sessions: false,
      profile: true,
    })
  )
  act(() =>
    client.setQueryData(['status'], {
      SidebarModulesAdmin: JSON.stringify({ personal: { security: false } }),
    })
  )
  await waitFor(() => expect(result.current.security).toBe(false))
  expect(result.current.sessions).toBe(false)
  unmount()
  client.clear()
})

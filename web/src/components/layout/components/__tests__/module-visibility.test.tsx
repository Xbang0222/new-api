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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { FontProvider } from '@/context/font-provider'
import { ThemeProvider } from '@/context/theme-provider'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { AuthenticatedLayout } from '../authenticated-layout'

const clients: QueryClient[] = []
beforeEach(() => {
  localStorage.clear()
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { success: true, data: url === '/api/notice' ? '' : [] },
  }))
})
afterEach(() => {
  for (const client of clients) client.clear()
  clients.length = 0
  useAuthStore.getState().auth.reset()
})

async function renderLayout(
  role: number,
  initialPath = '/wallet',
  pendingStatus = false,
  config?: {
    admin?: Record<string, Record<string, boolean> | undefined>
    user?: Record<string, Record<string, boolean>>
    canConfigure?: boolean
  }
) {
  useAuthStore
    .getState()
    .auth.setUser({
      id: 1,
      username: 'viewer',
      role,
      sidebar_modules: JSON.stringify(config?.user ?? {}),
      permissions: { sidebar_settings: config?.canConfigure ?? true },
    })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  if (!pendingStatus) {
    client.setQueryData(['status'], {
      SidebarModulesAdmin: JSON.stringify(
        config?.admin ?? {
          chat: { enabled: false },
          console: { enabled: false },
          personal: { enabled: false },
          admin: { enabled: false },
        }
      ),
    })
  }
  const root = createRootRoute({ component: AuthenticatedLayout })
  const wallet = createRoute({
    getParentRoute: () => root,
    path: '/wallet',
    component: () => <div>Protected wallet content</div>,
  })
  const settings = createRoute({
    getParentRoute: () => root,
    path: '/system-settings/site/sidebar-modules',
    component: () => <div>Module settings content</div>,
  })
  const personalSettings = createRoute({
    getParentRoute: () => root,
    path: '/profile/sidebar-modules',
    component: () => <div>Personal module preferences</div>,
  })
  const router = createRouter({
    routeTree: root.addChildren([wallet, settings, personalSettings]),
    history: createMemoryHistory({ initialEntries: [initialPath] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <FontProvider>
          <DirectionProvider>
            <RouterProvider router={router} />
          </DirectionProvider>
        </FontProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

it('gives the owner a working recovery link when every module is hidden', async () => {
  await renderLayout(100)
  expect(await screen.findByText('This module is hidden')).toBeVisible()
  expect(screen.queryByText('Protected wallet content')).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('link', { name: 'Sidebar modules' }))
  expect(await screen.findByText('Module settings content')).toBeVisible()
})

it.each([1, 10])(
  'lets role %s recover only personal preferences after hiding all own modules',
  async (role) => {
    await renderLayout(role, '/profile/sidebar-modules', false, {
      admin: {},
      user: {
        chat: { enabled: false },
        console: { enabled: false },
        personal: { enabled: false },
        admin: { enabled: false },
        profile: { sidebar: false },
      },
    })
    expect(await screen.findByText('Personal module preferences')).toBeVisible()
    expect(
      screen.queryByText('Protected wallet content')
    ).not.toBeInTheDocument()
    expect(
      screen.queryByText('Module settings content')
    ).not.toBeInTheDocument()
  }
)

it.each([
  { admin: { profile: { sidebar: false } } },
  { admin: { personal: { enabled: false } } },
  { admin: {}, canConfigure: false },
])(
  'keeps personal recovery behind administrator settings and user permission: %j',
  async (config) => {
    await renderLayout(1, '/profile/sidebar-modules', false, config)
    expect(await screen.findByText('This module is hidden')).toBeVisible()
    expect(
      screen.queryByText('Personal module preferences')
    ).not.toBeInTheDocument()
  }
)

it.each([1, 10])(
  'does not grant configuration recovery to role %s',
  async (role) => {
    await renderLayout(role, '/system-settings/site/sidebar-modules')
    expect(await screen.findByText('This module is hidden')).toBeVisible()
    expect(
      screen.queryByText('Module settings content')
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('link', { name: 'Sidebar modules' })
    ).not.toBeInTheDocument()
  }
)

it.each([false, true])(
  'waits for server settings without mounting a hidden page, persisted snapshot: %s',
  async (cached) => {
    if (cached) {
      localStorage.setItem(
        'status',
        JSON.stringify({
          SidebarModulesAdmin: JSON.stringify({ personal: { topup: true } }),
        })
      )
    }
    let resolveStatus!: (value: unknown) => void
    const statusRequest = new Promise((resolve) => {
      resolveStatus = resolve
    })
    vi.mocked(api.get).mockImplementation((url) =>
      url === '/api/status'
        ? statusRequest
        : Promise.resolve({
            data: { success: true, data: url === '/api/notice' ? '' : [] },
          })
    )
    await renderLayout(100, '/wallet', true)
    expect(await screen.findByText('Loading...')).toBeVisible()
    expect(
      screen.queryByText('Protected wallet content')
    ).not.toBeInTheDocument()
    await act(async () => {
      resolveStatus({
        data: {
          success: true,
          data: {
            SidebarModulesAdmin: JSON.stringify({ personal: { topup: false } }),
          },
        },
      })
      await statusRequest
    })
    expect(await screen.findByText('This module is hidden')).toBeVisible()
    expect(
      screen.queryByText('Protected wallet content')
    ).not.toBeInTheDocument()
  }
)

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
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { AuthenticatedLayout } from '@/components/layout/components/authenticated-layout'
import { DirectionProvider } from '@/context/direction-provider'
import { FontProvider } from '@/context/font-provider'
import { ThemeProvider } from '@/context/theme-provider'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { SidebarPreferences } from '../../sidebar-preferences'
import { SidebarModulesCard } from '../sidebar-modules-card'

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  client.setQueryData(['status'], {
    SidebarModulesAdmin: JSON.stringify({
      overview: { invitationRanking: true },
    }),
  })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.restoreAllMocks()
})
function renderSettings() {
  return render(
    <QueryClientProvider client={client}>
      <SidebarModulesCard />
    </QueryClientProvider>
  )
}

it('edits nested card preferences, preserves unknown preferences, and updates the current user after saving', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        sidebar_modules: JSON.stringify({
          overviewSummary: { balance: false },
          custom: { enabled: true, other: false },
        }),
      },
    },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  renderSettings()
  const balance = await screen.findByRole('switch', {
    name: 'Credit remaining',
  })
  await waitFor(() => expect(balance).not.toBeChecked())
  const user = userEvent.setup()
  await user.click(balance)
  await user.click(screen.getByRole('switch', { name: 'Historical Usage' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalled())
  const request = put.mock.calls[0][1] as { sidebar_modules: string }
  expect(JSON.parse(request.sidebar_modules)).toMatchObject({
    overviewSummary: { balance: true, usage: false },
    custom: { enabled: true, other: false },
  })
  await waitFor(() =>
    expect(useAuthStore.getState().auth.user?.sidebar_modules).toBe(
      request.sidebar_modules
    )
  )
})

it('prevents personal preferences from enabling an administrator-disabled card', async () => {
  client.setQueryData(['status'], {
    SidebarModulesAdmin: JSON.stringify({
      overview: { invitationRanking: false },
    }),
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        sidebar_modules: JSON.stringify({
          overview: { invitationRanking: true },
        }),
      },
    },
  })
  renderSettings()
  const ranking = await screen.findByRole('switch', {
    name: 'Invitation earnings leaderboard',
  })
  expect(ranking).toHaveAttribute('aria-disabled', 'true')
  expect(ranking).not.toBeChecked()
  await userEvent.click(ranking)
  expect(ranking).not.toBeChecked()
})

it('restores a personal disabled child group without overwriting its individual choices', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        sidebar_modules: JSON.stringify({
          overview: { enabled: false, community: false },
          overviewSummary: { enabled: false, balance: false },
        }),
      },
    },
  })
  renderSettings()
  const overview = await screen.findByRole('switch', { name: 'Overview' })
  await waitFor(() => expect(overview).not.toBeChecked())
  await userEvent.click(overview)
  await userEvent.click(
    screen.getByRole('switch', { name: 'Usage at a glance' })
  )
  expect(
    screen.getByRole('switch', { name: 'Credit remaining' })
  ).not.toBeChecked()
  expect(screen.getByRole('switch', { name: 'Last 24h usage' })).toBeChecked()
  expect(
    screen.getByRole('switch', { name: 'User community' })
  ).not.toBeChecked()
})

it('waits for administrator settings before exposing personal controls or requesting preferences', async () => {
  client.removeQueries({ queryKey: ['status'] })
  let resolveStatus!: (value: unknown) => void
  const pending = new Promise((resolve) => {
    resolveStatus = resolve
  })
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/status') return pending
    return { data: { success: true, data: {} } }
  })
  renderSettings()
  expect(screen.queryByRole('switch')).not.toBeInTheDocument()
  expect(get.mock.calls.some(([url]) => url === '/api/user/self')).toBe(false)
  await act(async () =>
    resolveStatus({
      data: { success: true, data: { SidebarModulesAdmin: '{}' } },
    })
  )
  expect(
    await screen.findByRole('switch', {
      name: 'Invitation earnings leaderboard',
    })
  ).not.toBeChecked()
})

it('keeps failed preference reads unavailable until a successful retry', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({ data: { success: false, message: 'unavailable' } })
    .mockResolvedValue({ data: { success: true, data: {} } })
  renderSettings()
  expect(await screen.findByText('Failed to load settings')).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Save Changes' })
  ).not.toBeInTheDocument()
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
  expect(
    await screen.findByRole('switch', { name: 'Credit remaining' })
  ).toBeChecked()
})

it('retains an unsaved choice and the previous user preferences when saving fails', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: {} } })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: false, message: 'unavailable' } })
  renderSettings()
  const balance = await screen.findByRole('switch', {
    name: 'Credit remaining',
  })
  await userEvent.click(balance)
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalled())
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  )
  expect(balance).not.toBeChecked()
  expect(useAuthStore.getState().auth.user?.sidebar_modules).toBeUndefined()
})

it('does not apply a late preference-save response to a different signed-in user', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data: {} } })
  let resolveSave!: (value: unknown) => void
  const pending = new Promise((resolve) => {
    resolveSave = resolve
  })
  const put = vi.spyOn(api, 'put').mockReturnValue(pending)
  renderSettings()
  await userEvent.click(
    await screen.findByRole('switch', { name: 'Credit remaining' })
  )
  await userEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalled())
  await act(async () =>
    useAuthStore.getState().auth.setUser({ id: 2, username: 'bob', role: 1 })
  )
  await act(async () => resolveSave({ data: { success: true } }))
  expect(useAuthStore.getState().auth.user?.sidebar_modules).toBeUndefined()
})

it('restores personal navigation after the user disables both the parent and the settings card', async () => {
  const hiddenPreferences = JSON.stringify({
    personal: { enabled: false, personal: false },
    profile: { enabled: false, sidebar: false },
  })
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'alice',
    role: 1,
    permissions: { sidebar_settings: true },
    sidebar_modules: hiddenPreferences,
  })
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/notice' ? '' : { sidebar_modules: hiddenPreferences },
    },
  }))
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const root = createRootRoute({ component: AuthenticatedLayout })
  const profile = createRoute({
    getParentRoute: () => root,
    path: '/profile',
    component: () => <div>Personal profile content</div>,
  })
  const recovery = createRoute({
    getParentRoute: () => root,
    path: '/profile/sidebar-modules',
    component: SidebarPreferences,
  })
  const router = createRouter({
    routeTree: root.addChildren([profile, recovery]),
    history: createMemoryHistory({ initialEntries: ['/profile'] }),
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
  expect(await screen.findByText('This module is hidden')).toBeVisible()
  const user = userEvent.setup()
  await user.click(
    screen.getByRole('link', { name: 'Sidebar Personal Settings' })
  )
  await user.click(
    await screen.findByRole('button', { name: 'Reset to Default' })
  )
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/user/self', {
      sidebar_modules: '{}',
    })
  )
  await waitFor(() =>
    expect(useAuthStore.getState().auth.user?.sidebar_modules).toBe('{}')
  )
  expect(useAuthStore.getState().auth.user?.role).toBe(1)
  await act(async () => {
    await router.navigate({ to: '/profile' })
  })
  expect(await screen.findByText('Personal profile content')).toBeVisible()
})

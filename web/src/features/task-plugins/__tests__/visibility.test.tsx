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
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { TaskPlugins } from '../index'
import type { TaskPluginListItem } from '../types'

let client: QueryClient
const item: TaskPluginListItem = {
  meta: {
    key: 'example',
    name: 'Example',
    version: '1.0.0',
    apiVersion: 1,
    author: { name: 'Example' },
    models: [],
    fetchMode: 'per_task',
  },
  source: 'factory',
  enabled: true,
  active: true,
  source_hash: '',
  remark: '',
  runtime_status: 'registered',
  channel_count: 0,
  in_flight_count: 0,
}
beforeEach(() => {
  localStorage.clear()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'owner', role: 100 })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    let data: unknown = []
    if (url === '/api/option/') {
      data = [{ key: 'TaskPluginEnabled', value: 'true' }]
    }
    if (url === '/api/plugin/task') data = [item]
    if (url === '/api/plugin/task/example') {
      data = { meta: item.meta, source: '', layer: 'factory' }
    }
    return { data: { success: true, data } }
  })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.reset()
})
function settings(installed: boolean, marketplace = true) {
  return {
    SidebarModulesAdmin: JSON.stringify({
      taskPlugins: { installed, marketplace },
    }),
  }
}
async function showPlugins(installed = true, marketplace = true) {
  client.setQueryData(['status'], settings(installed, marketplace))
  const router = createRouter({
    routeTree: createRootRoute({ component: TaskPlugins }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

it.each(['upload', 'details'])(
  'closes the open %s panel when Installed is disabled and does not reopen it later',
  async (panel) => {
    await showPlugins()
    const user = userEvent.setup()
    if (panel === 'upload') {
      await user.click(
        await screen.findByRole('button', { name: 'Upload plugin' })
      )
    } else {
      await screen.findByText('Example')
      await user.click(screen.getAllByRole('button', { name: 'Open menu' })[0])
      await user.click(await screen.findByRole('menuitem', { name: 'Details' }))
    }
    expect(await screen.findByRole('dialog')).toBeVisible()
    act(() => client.setQueryData(['status'], settings(false)))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(screen.getByRole('tab', { name: 'Marketplace' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(
      screen.queryByRole('tab', { name: 'Installed' })
    ).not.toBeInTheDocument()
    vi.mocked(api.get).mockClear()
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['task-plugin', 'example'] })
    })
    expect(
      vi
        .mocked(api.get)
        .mock.calls.some(([url]) => url === '/api/plugin/task/example')
    ).toBe(false)
    act(() => client.setQueryData(['status'], settings(true)))
    await screen.findByRole('tab', { name: 'Installed' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  }
)

it('does not query plugin data when every plugin area is disabled', async () => {
  await showPlugins(false, false)
  expect(screen.queryByRole('tab')).not.toBeInTheDocument()
  expect(
    vi
      .mocked(api.get)
      .mock.calls.some(
        ([url]) => url === '/api/plugin/task' || url === '/api/option/'
      )
  ).toBe(false)
})

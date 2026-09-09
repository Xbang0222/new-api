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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { ApiKey } from '../../types'
import { ApiKeysMutateDrawer } from '../api-keys-mutate-drawer'
import { ApiKeysProvider } from '../api-keys-provider'

const savedApiKey: ApiKey = {
  id: 14,
  name: 'Existing API key',
  key: 'test-key',
  status: 1,
  remain_quota: 0,
  used_quota: 0,
  unlimited_quota: true,
  expired_time: -1,
  created_time: 1,
  accessed_time: 0,
  group: 'auto',
  auto_groups: null,
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}
const queryClients: QueryClient[] = []

async function renderDrawer(
  options: { currentRow?: ApiKey; defaultAuto?: boolean } = {}
) {
  const defaultAuto = options.defaultAuto ?? true
  const groups = {
    auto: { desc: 'Automatic routing', ratio: 'auto' },
    default: { desc: 'Standard access', ratio: 1 },
    vip: { desc: 'Priority access', ratio: 2 },
  }
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    switch (url) {
      case '/api/status':
        return { data: { data: { default_use_auto_group: defaultAuto } } }
      case '/api/user/models':
        return { data: { success: true, data: [] } }
      case '/api/user/self/groups':
        return { data: { success: true, data: groups } }
      case '/api/token/auto-groups':
        return {
          data: {
            success: true,
            data: { groups: ['vip', 'default'], max_count: 3 },
          },
        }
      case '/api/token/14':
        return { data: { success: true, data: options.currentRow } }
      default:
        throw new Error(`Unexpected GET ${url}`)
    }
  })
  const create = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true, data: {} } })
  const update = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, data: {} } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClients.push(client)
  client.setQueryData(
    ['status'],
    { default_use_auto_group: defaultAuto },
    { updatedAt: Date.now() + 60_000 }
  )
  render(
    <QueryClientProvider client={client}>
      <ApiKeysProvider>
        <ApiKeysMutateDrawer
          open
          currentRow={options.currentRow}
          onOpenChange={() => undefined}
        />
      </ApiKeysProvider>
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  )
  return { create, update }
}

function selectGroup(name: 'Automatic routing' | 'Standard access') {
  const groupItem = screen
    .getByText('Group', { selector: 'label' })
    .closest<HTMLElement>('[data-slot="form-item"]')
  if (!groupItem) throw new Error('Expected Group form field')
  fireEvent.click(within(groupItem).getByRole('combobox'))
  fireEvent.click(screen.getByRole('option', { name: new RegExp(name) }))
}

afterEach(() => {
  for (const client of queryClients.splice(0)) client.clear()
  window.localStorage.clear()
})

describe('API key cross-group retry preference', () => {
  test('an explicit off choice survives Auto to ordinary to Auto changes and saves as false', async () => {
    const { create } = await renderDrawer()
    const retrySwitch = screen.getByRole('switch', {
      name: 'Cross-group retry',
    })
    expect(retrySwitch).toBeChecked()
    fireEvent.click(retrySwitch)
    expect(retrySwitch).not.toBeChecked()
    selectGroup('Standard access')
    expect(
      screen.queryByRole('switch', { name: 'Cross-group retry' })
    ).not.toBeInTheDocument()
    selectGroup('Automatic routing')
    expect(
      screen.getByRole('switch', { name: 'Cross-group retry' })
    ).not.toBeChecked()
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Cache preference' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        '/api/token/',
        expect.objectContaining({ group: 'auto', cross_group_retry: false })
      )
    )
  })

  test.each([false, true])(
    'editing a saved retry choice %s preserves it on load and submission',
    async (enabled) => {
      const { update } = await renderDrawer({
        currentRow: { ...savedApiKey, cross_group_retry: enabled },
      })
      expect(
        screen.getByRole('switch', { name: 'Cross-group retry' })
      ).toHaveAttribute('aria-checked', String(enabled))
      selectGroup('Standard access')
      selectGroup('Automatic routing')
      expect(
        screen.getByRole('switch', { name: 'Cross-group retry' })
      ).toHaveAttribute('aria-checked', String(enabled))
      fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
      await waitFor(() =>
        expect(update).toHaveBeenCalledWith(
          '/api/token/',
          expect.objectContaining({
            id: 14,
            group: 'auto',
            cross_group_retry: enabled,
          })
        )
      )
    }
  )

  test('turning a saved false preference on submits true', async () => {
    const { update } = await renderDrawer({ currentRow: savedApiKey })
    const retrySwitch = screen.getByRole('switch', {
      name: 'Cross-group retry',
    })
    expect(retrySwitch).not.toBeChecked()
    fireEvent.click(retrySwitch)
    expect(retrySwitch).toBeChecked()
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(
        '/api/token/',
        expect.objectContaining({
          id: 14,
          group: 'auto',
          cross_group_retry: true,
        })
      )
    )
  })

  test('first selecting Auto for a new ordinary key keeps the existing enabled default', async () => {
    const { create } = await renderDrawer({ defaultAuto: false })
    expect(
      screen.queryByRole('switch', { name: 'Cross-group retry' })
    ).not.toBeInTheDocument()
    selectGroup('Automatic routing')
    expect(
      screen.getByRole('switch', { name: 'Cross-group retry' })
    ).toBeChecked()
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Auto default' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        '/api/token/',
        expect.objectContaining({ group: 'auto', cross_group_retry: true })
      )
    )
  })

  test('saving an ordinary group still sends false even if Auto retry was enabled', async () => {
    const { create } = await renderDrawer()
    expect(
      screen.getByRole('switch', { name: 'Cross-group retry' })
    ).toBeChecked()
    selectGroup('Standard access')
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Ordinary key' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))
    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        '/api/token/',
        expect.objectContaining({ group: 'default', cross_group_retry: false })
      )
    )
  })

  test('the existing switch exposes a small conditional prompt-cache cost notice in its accessible description', async () => {
    await renderDrawer()
    const retrySwitch = screen.getByRole('switch', {
      name: 'Cross-group retry',
    })
    const notice =
      'Cross-group retries may reduce prompt cache hits and increase costs.'
    expect(screen.getByText(notice)).toHaveClass('text-xs')
    expect(retrySwitch).toHaveAccessibleDescription(
      expect.stringContaining(notice)
    )
    expect(
      screen.getAllByRole('switch', { name: 'Cross-group retry' })
    ).toHaveLength(1)
  })
})

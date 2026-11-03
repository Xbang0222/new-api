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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import {
  parseSidebarModulesAdmin,
  serializeSidebarModulesAdmin,
} from '../config'
import { SidebarModulesSection } from '../sidebar-modules-section'

const config = parseSidebarModulesAdmin('')
function Fixture(props: {
  config?: ReturnType<typeof parseSidebarModulesAdmin>
}) {
  const selectedConfig = props.config ?? config
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <SidebarModulesSection
          config={selectedConfig}
          initialSerialized={serializeSidebarModulesAdmin(selectedConfig)}
        />
      </SettingsPageProvider>
    </>
  )
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

it('saves individual overview switches in sidebar modules and immediately updates status', async () => {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  client.setQueryData(['status'], {
    SidebarModulesAdmin: serializeSidebarModulesAdmin(config),
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <Fixture />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  const ranking = screen.getByRole('switch', {
    name: 'Invitation earnings leaderboard',
  })
  expect(ranking).not.toBeChecked()
  await user.click(ranking)
  await user.click(screen.getByRole('switch', { name: 'Usage at a glance' }))
  await user.click(screen.getByRole('button', { name: 'Save sidebar modules' }))
  await waitFor(() => expect(put).toHaveBeenCalled())
  const request = put.mock.calls[0][1] as { key: string; value: string }
  expect(request.key).toBe('SidebarModulesAdmin')
  const saved = JSON.parse(request.value)
  expect(saved.overview.invitationRanking).toBe(true)
  expect(saved.overview.summary).toBe(false)
  expect(saved.overview.community).toBe(true)
  await waitFor(() =>
    expect(client.getQueryData(['status'])).toEqual({
      SidebarModulesAdmin: request.value,
    })
  )
  cleanup()
  client.clear()
})

it('disables overview card controls when the console overview parent is off', async () => {
  const client = new QueryClient()
  render(
    <QueryClientProvider client={client}>
      <Fixture />
    </QueryClientProvider>
  )
  await userEvent.click(screen.getByRole('switch', { name: 'Overview' }))
  expect(
    screen.getByRole('switch', { name: 'Invitation earnings leaderboard' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(
    screen.getByRole('switch', { name: 'Usage at a glance' })
  ).toHaveAttribute('aria-disabled', 'true')
  await userEvent.click(screen.getByRole('switch', { name: 'Overview' }))
  expect(
    screen.getByRole('switch', { name: 'Invitation earnings leaderboard' })
  ).not.toHaveAttribute('aria-disabled', 'true')
  cleanup()
  client.clear()
})

it('saves each overview indicator independently while preserving its summary parent', async () => {
  const client = new QueryClient()
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <Fixture />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('switch', { name: 'Credit remaining' }))
  await user.click(screen.getByRole('button', { name: 'Save sidebar modules' }))
  await waitFor(() => expect(put).toHaveBeenCalled())
  const request = put.mock.calls[0][1] as { value: string }
  const saved = JSON.parse(request.value)
  expect(saved.overview.summary).toBe(true)
  expect(saved.overviewSummary).toEqual({
    enabled: true,
    balance: false,
    todayUsage: true,
    usage: true,
    requests: true,
  })
  cleanup()
  client.clear()
})

it('restores disabled child groups through their parent switches without resetting child choices', async () => {
  const client = new QueryClient()
  const restoredConfig = parseSidebarModulesAdmin(
    JSON.stringify({
      overview: { enabled: false, invitationRanking: false },
      overviewSummary: { enabled: false, balance: false },
    })
  )
  render(
    <QueryClientProvider client={client}>
      <Fixture config={restoredConfig} />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  const overview = screen.getByRole('switch', { name: 'Overview' })
  expect(overview).not.toBeChecked()
  await user.click(overview)
  const summary = screen.getByRole('switch', { name: 'Usage at a glance' })
  expect(summary).not.toBeChecked()
  await user.click(summary)
  expect(
    screen.getByRole('switch', { name: 'Credit remaining' })
  ).not.toBeChecked()
  expect(screen.getByRole('switch', { name: 'Last 24h usage' })).toBeChecked()
  expect(
    screen.getByRole('switch', { name: 'Invitation earnings leaderboard' })
  ).not.toBeChecked()
  await user.click(overview)
  expect(
    screen.getByRole('switch', { name: 'Last 24h usage' })
  ).toHaveAttribute('aria-disabled', 'true')
  await user.click(overview)
  expect(screen.getByRole('switch', { name: 'Last 24h usage' })).toBeChecked()
  cleanup()
  client.clear()
})

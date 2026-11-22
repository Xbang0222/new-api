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
import { createFileRoute, redirect } from '@tanstack/react-router'

import {
  getSidebarModulesAdminConfig,
  resolveConsoleHomePath,
} from '@/hooks/use-sidebar-config'

export const Route = createFileRoute('/_authenticated/dashboard/')({
  beforeLoad: ({ context }) => {
    let status = context.queryClient.getQueryData<{
      SidebarModulesAdmin?: string | null
      console_home_page?: string
    }>(['status'])

    if (!status && typeof window !== 'undefined') {
      try {
        const cachedStatus = window.localStorage.getItem('status')
        status = cachedStatus ? JSON.parse(cachedStatus) : undefined
      } catch {
        status = undefined
      }
    }

    const destination = resolveConsoleHomePath(
      getSidebarModulesAdminConfig(status),
      status?.console_home_page
    )

    if (destination === '/dashboard/overview') {
      throw redirect({
        to: '/dashboard/$section',
        params: { section: 'overview' },
      })
    }
    if (destination === '/dashboard/models') {
      throw redirect({
        to: '/dashboard/$section',
        params: { section: 'models' },
      })
    }
    if (destination === '/usage-logs/common') {
      throw redirect({
        to: '/usage-logs/$section',
        params: { section: 'common' },
      })
    }
    if (destination === '/usage-logs/task') {
      throw redirect({
        to: '/usage-logs/$section',
        params: { section: 'task' },
      })
    }

    throw redirect({ to: destination })
  },
})

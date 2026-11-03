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
import { useAuthStore } from '@/stores/auth-store'

import {
  isSidebarModuleEnabledForConfig,
  parseSidebarModuleLayers,
} from './use-sidebar-config'
import { useStatus } from './use-status'

/** Recovery ignores personal hiding choices, never administrator restrictions. */
export function usePersonalSidebarRecoveryAllowed(): boolean {
  const { status, ready } = useStatus()
  const user = useAuthStore((state) => state.auth.user)
  const { adminConfig } = parseSidebarModuleLayers(status, user)
  return (
    ready &&
    Boolean(user) &&
    user?.permissions?.sidebar_settings !== false &&
    isSidebarModuleEnabledForConfig('profile', 'sidebar', adminConfig, null)
  )
}

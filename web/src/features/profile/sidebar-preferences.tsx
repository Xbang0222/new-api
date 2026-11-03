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
import { Main } from '@/components/layout'
import { usePersonalSidebarRecoveryAllowed } from '@/hooks/use-personal-sidebar-recovery'

import { SidebarModulesCard } from './components/sidebar-modules-card'

export function SidebarPreferences() {
  const allowed = usePersonalSidebarRecoveryAllowed()
  if (!allowed) return null
  return (
    <Main>
      <div className='min-h-0 flex-1 overflow-auto px-3 py-3 sm:px-4 sm:py-6'>
        <div className='mx-auto w-full max-w-7xl'>
          <SidebarModulesCard />
        </div>
      </div>
    </Main>
  )
}

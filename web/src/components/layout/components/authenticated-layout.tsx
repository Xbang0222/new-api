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
import { useQueryClient } from '@tanstack/react-query'
import { useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { LayoutProvider } from '@/context/layout-provider'
import { usePersonalSidebarRecoveryAllowed } from '@/hooks/use-personal-sidebar-recovery'
import { useIsSidebarModuleVisible } from '@/hooks/use-sidebar-config'
import { useStatus } from '@/hooks/use-status'
import { getCookie } from '@/lib/cookies'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const { t } = useTranslation()
  const pathname = useLocation({ select: (location) => location.pathname })
  const moduleVisible = useIsSidebarModuleVisible(pathname)
  const canRecoverPersonalSettings = usePersonalSidebarRecoveryAllowed()
  const userRole = useAuthStore((state) => state.auth.user?.role)
  const { ready, loading } = useStatus()
  const queryClient = useQueryClient()
  let canShowPage =
    moduleVisible ||
    (userRole === ROLE.SUPER_ADMIN &&
      pathname === '/system-settings/site/sidebar-modules')
  if (pathname === '/profile/sidebar-modules') {
    canShowPage = canRecoverPersonalSettings
  }
  const defaultOpen = getCookie('sidebar_state') !== 'false'
  let content = props.children ?? <AnimatedOutlet />
  if (!ready && !canShowPage) {
    content = loading ? (
      <LoadingState />
    ) : (
      <ErrorState
        onRetry={() => {
          void queryClient.invalidateQueries({ queryKey: ['status'] })
        }}
      />
    )
  } else if (!canShowPage) {
    content = (
      <EmptyState
        title={t('This module is hidden')}
        description={t('Choose another module from the sidebar.')}
      />
    )
  }

  return (
    <LayoutProvider>
      <SidebarProvider defaultOpen={defaultOpen} className='flex-col'>
        <SkipToMain />
        <AppHeader />
        <div className='flex min-h-0 w-full flex-1'>
          <AppSidebar />
          <SidebarInset
            className={cn(
              '@container/content',
              'h-[calc(100svh-var(--app-header-height,0px))]',
              'min-h-0 overflow-hidden',
              'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))]'
            )}
          >
            {content}
          </SidebarInset>
        </div>
      </SidebarProvider>
    </LayoutProvider>
  )
}

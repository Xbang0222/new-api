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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { LayoutDashboard } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import {
  SettingsControlChildren,
  SettingsControlGroup,
  SettingsSwitchField,
} from '@/features/system-settings/components/settings-form-layout'
import {
  parseSidebarModulesUser,
  isSidebarModuleSelected,
  type SidebarModulesAdminConfig,
} from '@/features/system-settings/maintenance/config'
import { SidebarChildControls } from '@/features/system-settings/maintenance/sidebar-child-controls'
import { getSidebarChildSection } from '@/features/system-settings/maintenance/sidebar-child-sections'
import {
  isSidebarModuleEnabledForConfig,
  parseSidebarModuleLayers,
} from '@/hooks/use-sidebar-config'
import { useStatus } from '@/hooks/use-status'
import { api } from '@/lib/api'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

type SectionDef = {
  key: string
  title: string
  description: string
  modules: { key: string; title: string; description: string }[]
}

export function SidebarModulesCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentUser = useAuthStore((state) => state.auth.user)
  const { status, error: statusError } = useStatus()
  const [draft, setDraft] = useState<SidebarModulesAdminConfig | null>(null)
  const { adminConfig } = parseSidebarModuleLayers(status, currentUser)
  const queryKey = ['user', 'sidebar-settings', currentUser?.id]
  const preferences = useQuery({
    queryKey,
    queryFn: async ({ signal }) =>
      requireServerSuccess(
        (
          await api.get<{
            success: boolean
            data: { sidebar_modules?: string | Record<string, unknown> }
          }>('/api/user/self', { signal })
        ).data
      ),
    enabled: Boolean(currentUser?.id && status),
  })
  const raw = preferences.data?.data.sidebar_modules
  const serialized = typeof raw === 'string' ? raw : JSON.stringify(raw)
  const config = draft ?? parseSidebarModulesUser(serialized) ?? {}
  const canEdit = currentUser?.permissions?.sidebar_settings !== false
  const save = useMutation({
    mutationFn: async (request: { userId: number; sidebarModules: string }) =>
      requireServerSuccess(
        (
          await api.put('/api/user/self', {
            sidebar_modules: request.sidebarModules,
          })
        ).data
      ),
    onSuccess: (_, request) => {
      const latestUser = useAuthStore.getState().auth.user
      if (latestUser?.id !== request.userId) return
      queryClient.setQueryData(['user', 'sidebar-settings', request.userId], {
        success: true,
        data: { sidebar_modules: request.sidebarModules },
      })
      useAuthStore.getState().auth.setUser({
        ...latestUser,
        sidebar_modules: request.sidebarModules,
      })
      setDraft(null)
      toast.success(t('Saved successfully'))
    },
    onError: (error) =>
      handleServerError(error, t('Save failed, please retry')),
  })
  const setModule = (section: string, module: string, checked: boolean) => {
    const child = getSidebarChildSection(section, module)
    setDraft((previous) => {
      const next = previous ?? config
      const updated = {
        ...next,
        [section]: {
          ...next[section],
          enabled: next[section]?.enabled ?? true,
          [module]: checked,
        },
      }
      if (checked && child) {
        updated[child[0]] = { ...next[child[0]], enabled: true }
      }
      return updated
    })
  }
  const isAllowed = (section: string, module: string) =>
    Boolean(status) &&
    canEdit &&
    isSidebarModuleEnabledForConfig(section, module, adminConfig, null)

  const sectionDefs: SectionDef[] = [
    {
      key: 'chat',
      title: t('Chat Area'),
      description: t('Playground and chat functions'),
      modules: [
        {
          key: 'playground',
          title: t('Playground'),
          description: t('AI model testing environment'),
        },
        {
          key: 'chat',
          title: t('Chat'),
          description: t('Chat session management'),
        },
      ],
    },
    {
      key: 'console',
      title: t('Console Area'),
      description: t('Data management and log viewing'),
      modules: [
        {
          key: 'overview',
          title: t('Overview'),
          description: t('Account summary and quick actions'),
        },
        {
          key: 'dashboard',
          title: t('Dashboard'),
          description: t('System data statistics'),
        },
        {
          key: 'token',
          title: t('Token Management'),
          description: t('API token management'),
        },
        {
          key: 'log',
          title: t('Usage Logs'),
          description: t('API usage records'),
        },
        {
          key: 'audit',
          title: t('Audit Logs'),
          description: t('Login, security and access records'),
        },
        {
          key: 'task',
          title: t('Task Logs'),
          description: t('Drawing and system task records'),
        },
      ],
    },
    {
      key: 'personal',
      title: t('Personal Center Area'),
      description: t('User personal functions'),
      modules: [
        {
          key: 'topup',
          title: t('Wallet Management'),
          description: t('Balance and top-up management'),
        },
        {
          key: 'invoice',
          title: t('Invoice applications'),
          description: t('Apply for and download invoices for paid orders.'),
        },
        {
          key: 'personal',
          title: t('Personal Settings'),
          description: t('Personal info settings'),
        },
        {
          key: 'security',
          title: t('Security & Access'),
          description: t('Manage your security settings and account access'),
        },
      ],
    },
  ]

  if ((currentUser?.role ?? 0) >= ROLE.ADMIN) {
    sectionDefs.push({
      key: 'admin',
      title: t('Admin area'),
      description: t('Global configuration and administrative tools.'),
      modules: [
        { key: 'channel', title: t('Channels'), description: '' },
        {
          key: 'channelScheduling',
          title: t('Intelligent Scheduling'),
          description: '',
        },
        { key: 'models', title: t('Models'), description: '' },
        { key: 'redemption', title: t('Redeem codes'), description: '' },
        { key: 'user', title: t('Users'), description: '' },
        { key: 'setting', title: t('System settings'), description: '' },
        {
          key: 'subscription',
          title: t('Subscription Management'),
          description: '',
        },
        {
          key: 'invoiceManagement',
          title: t('Invoice Management'),
          description: '',
        },
        { key: 'systemInfo', title: t('System Info'), description: '' },
        { key: 'taskPlugins', title: t('Task Plugins'), description: '' },
      ],
    })
  }

  if (preferences.isError || statusError) {
    return (
      <ErrorState
        title={t('Failed to load settings')}
        onRetry={() => {
          void preferences.refetch()
          void queryClient.invalidateQueries({ queryKey: ['status'] })
        }}
      />
    )
  }
  if (!status || preferences.isPending) {
    return <Skeleton className='h-48 w-full' />
  }

  return (
    <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
      <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
        <div className='flex items-center gap-3'>
          <IconBadge tone='info' size='title'>
            <LayoutDashboard />
          </IconBadge>
          <div className='min-w-0'>
            <CardTitle className='text-lg tracking-tight sm:text-xl'>
              {t('Sidebar Personal Settings')}
            </CardTitle>
            <CardDescription className='text-xs sm:text-sm'>
              {t('Customize sidebar display content')}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent className='space-y-4 p-3 sm:space-y-5 sm:p-5'>
        {sectionDefs.map((section) => {
          const adminEnabled = adminConfig[section.key]?.enabled === true
          const sectionEnabled =
            adminEnabled && config[section.key]?.enabled !== false
          return (
            <SettingsControlGroup key={section.key}>
              <SettingsSwitchField
                controlId={`sidebar-user-${section.key}`}
                label={section.title}
                description={section.description}
                checked={sectionEnabled}
                onCheckedChange={(value) =>
                  setModule(section.key, 'enabled', value)
                }
                disabled={!canEdit || !adminEnabled}
              />
              <SettingsControlChildren className='space-y-3'>
                {section.modules
                  .filter(
                    (module) =>
                      module.key !== 'audit' ||
                      (currentUser?.role ?? 0) >= ROLE.ADMIN
                  )
                  .map((module) => {
                    const allowed = isAllowed(section.key, module.key)
                    const checked =
                      allowed &&
                      isSidebarModuleSelected(config, section.key, module.key)
                    const disabled = !sectionEnabled || !allowed
                    const child = getSidebarChildSection(
                      section.key,
                      module.key
                    )
                    return (
                      <div key={module.key}>
                        <SettingsSwitchField
                          controlId={`sidebar-user-${section.key}-${module.key}`}
                          label={module.title}
                          description={module.description}
                          checked={checked}
                          onCheckedChange={(value) =>
                            setModule(section.key, module.key, value)
                          }
                          disabled={disabled}
                          className={disabled ? 'opacity-50' : undefined}
                        />
                        {child && (
                          <SidebarChildControls
                            section={child[0]}
                            config={config}
                            disabled={disabled || !checked}
                            controlPrefix='sidebar-user'
                            isAllowed={isAllowed}
                            onChange={setModule}
                          />
                        )}
                      </div>
                    )
                  })}
              </SettingsControlChildren>
            </SettingsControlGroup>
          )
        })}
        <div className='flex flex-col-reverse gap-2 border-t pt-4 sm:flex-row sm:justify-end'>
          <Button
            variant='outline'
            onClick={() => setDraft({})}
            disabled={!canEdit}
          >
            {t('Reset to Default')}
          </Button>
          <Button
            onClick={() => {
              if (currentUser) {
                save.mutate({
                  userId: currentUser.id,
                  sidebarModules: JSON.stringify(config),
                })
              }
            }}
            disabled={save.isPending || !canEdit}
          >
            {save.isPending ? t('Saving...') : t('Save Changes')}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

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
import { useQuery } from '@tanstack/react-query'
import { CircleAlert, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertAction, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useSidebarModules } from '@/hooks/use-sidebar-config'

import { getChannelSchedulingOverview } from './api'
import { OverviewCards } from './components/overview-cards'
import { PoolTable } from './components/pool-table'
import { SchedulingCharts } from './components/scheduling-charts'
import { SchedulingScopeSelector } from './components/scheduling-scope-selector'
import { SettingsPanel } from './components/settings-panel'
import {
  calculateSchedulingGroupMetrics,
  listConfiguredSchedulingGroups,
  listSchedulingPools,
} from './lib/scheduling-analytics'
import type { SchedulingFilters } from './types'

const POLL_INTERVAL_MS = 2000
const EMPTY_FILTERS: SchedulingFilters = { group: '', model: '', priority: '' }

export function ChannelScheduling() {
  const isModuleEnabled = useSidebarModules()
  const { t } = useTranslation()
  const [selectedTab, setActiveTab] = useState('overview')
  const visibleTabs = ['overview', 'pools', 'settings'].filter((key) =>
    isModuleEnabled('scheduling', key)
  )
  const activeTab = visibleTabs.includes(selectedTab)
    ? selectedTab
    : (visibleTabs[0] ?? '')
  const [selectedGroup, setSelectedGroup] = useState('')
  const [selectedPoolKey, setSelectedPoolKey] = useState('')
  const overviewQuery = useQuery({
    queryKey: ['channel-scheduling-overview'],
    queryFn: () => getChannelSchedulingOverview(EMPTY_FILTERS),
    enabled: activeTab !== '' && activeTab !== 'settings',
    refetchInterval: activeTab === 'settings' ? false : POLL_INTERVAL_MS,
    refetchIntervalInBackground: false,
  })
  const groups = listConfiguredSchedulingGroups(overviewQuery.data?.strategy)
  const activeGroup = groups.includes(selectedGroup)
    ? selectedGroup
    : groups[0] || ''
  const pools = listSchedulingPools(
    overviewQuery.data?.channels ?? [],
    activeGroup
  )
  const activePool =
    pools.find((pool) => pool.key === selectedPoolKey) ?? pools[0] ?? null
  const activeStrategy = overviewQuery.data
    ? overviewQuery.data.strategy.group_strategies[activeGroup] ||
      overviewQuery.data.strategy.default_strategy
    : null

  const handleGroupChange = (group: string) => {
    setSelectedGroup(group)
    setSelectedPoolKey('')
  }

  let overviewContent = (
    <div className='flex flex-col gap-4'>
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {Array.from({ length: 10 }, (_, index) => (
          <Skeleton key={index} className='h-32' />
        ))}
      </div>
      <Skeleton className='h-80' />
    </div>
  )
  if (overviewQuery.isError && !overviewQuery.data) {
    overviewContent = (
      <Alert variant='destructive'>
        <CircleAlert aria-hidden='true' />
        <AlertTitle>{t('Failed to load scheduling data')}</AlertTitle>
        <AlertAction>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={() => overviewQuery.refetch()}
          >
            {t('Retry')}
          </Button>
        </AlertAction>
      </Alert>
    )
  } else if (overviewQuery.data && activePool) {
    overviewContent = (
      <>
        <OverviewCards
          group={activeGroup}
          pool={activePool}
          metrics={calculateSchedulingGroupMetrics(
            pools,
            overviewQuery.data.generated_at
          )}
        />
        <SchedulingCharts pool={activePool} />
      </>
    )
  } else if (overviewQuery.data) {
    overviewContent = (
      <Empty className='rounded-lg border py-12'>
        <EmptyHeader>
          <EmptyTitle>
            {t('No active scheduling pool for this group')}
          </EmptyTitle>
          <EmptyDescription>
            {t(
              'Choose another service group, or wait until intelligent scheduling handles a request for this group.'
            )}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Intelligent Scheduling')}</span>
          <Badge variant='outline'>{t('Real-time')}</Badge>
          {activeStrategy && (
            <Badge
              variant={
                activeStrategy === 'intelligent' ? 'default' : 'secondary'
              }
            >
              {activeStrategy === 'intelligent'
                ? t('Intelligent round robin')
                : t('Legacy priority and weight')}
            </Badge>
          )}
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          variant='outline'
          size='sm'
          onClick={() => overviewQuery.refetch()}
          disabled={overviewQuery.isFetching}
        >
          <RefreshCw
            className={
              overviewQuery.isFetching ? 'size-4 animate-spin' : 'size-4'
            }
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs
          value={activeTab}
          onValueChange={setActiveTab}
          className='flex flex-col gap-4'
        >
          <TabsList>
            {isModuleEnabled('scheduling', 'overview') && (
              <TabsTrigger value='overview'>
                {t('Real-time overview')}
              </TabsTrigger>
            )}
            {isModuleEnabled('scheduling', 'pools') && (
              <TabsTrigger value='pools'>{t('Pool details')}</TabsTrigger>
            )}
            {isModuleEnabled('scheduling', 'settings') && (
              <TabsTrigger value='settings'>
                {t('Strategy settings')}
              </TabsTrigger>
            )}
          </TabsList>

          {activeTab !== '' && activeTab !== 'settings' && (
            <SchedulingScopeSelector
              groups={groups}
              selectedGroup={activeGroup}
              onGroupChange={handleGroupChange}
              pools={pools}
              selectedPoolKey={activePool?.key ?? ''}
              onPoolChange={setSelectedPoolKey}
              strategy={activeStrategy}
              showPool={activeTab === 'overview'}
            />
          )}

          {isModuleEnabled('scheduling', 'overview') && (
            <TabsContent value='overview' className='flex flex-col gap-4'>
              {overviewContent}
            </TabsContent>
          )}

          {isModuleEnabled('scheduling', 'pools') && (
            <TabsContent value='pools'>
              <PoolTable
                group={activeGroup}
                pools={pools}
                faults={overviewQuery.data?.faults ?? []}
                channelFaults={overviewQuery.data?.channel_faults ?? []}
              />
            </TabsContent>
          )}

          {isModuleEnabled('scheduling', 'settings') && (
            <TabsContent value='settings'>
              <SettingsPanel />
            </TabsContent>
          )}
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

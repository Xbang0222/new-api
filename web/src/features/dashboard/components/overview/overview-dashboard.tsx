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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { OnboardingTour } from '@/features/onboarding/components/onboarding-tour'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useDashboardContentVisibility } from '../../hooks/use-status-data'
import { AnnouncementsPanel } from './announcements-panel'
import { ApiInfoPanel } from './api-info-panel'
import { CommunityGroupCard } from './community-group-card'
import { FAQPanel } from './faq-panel'
import { InvitationRankingPanel } from './invitation-ranking-panel'
import { OverviewSectionLayout } from './overview-section-layout'
import { PerformanceHealthPanel } from './performance-health-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'

export function OverviewDashboard() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [tourOpen, setTourOpen] = useState(false)
  const {
    apiInfo: showApiInfoPanel,
    announcements: showAnnouncementsPanel,
    faq: showFAQPanel,
    uptimeKuma: showUptimePanel,
  } = useDashboardContentVisibility()
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)

  return (
    <>
      <OnboardingTour open={tourOpen} onOpenChange={setTourOpen} />
      {!isAdmin && (
        <div className='mb-3 flex justify-end'>
          <Button variant='ghost' size='sm' onClick={() => setTourOpen(true)}>
            {t('View guide')}
          </Button>
        </div>
      )}
      <OverviewSectionLayout
        summary={<SummaryCards />}
        community={<CommunityGroupCard />}
        informationPanels={
          <>
            <InvitationRankingPanel />
            {isAdmin && <PerformanceHealthPanel />}
            {showAnnouncementsPanel && <AnnouncementsPanel />}
            {showApiInfoPanel && <ApiInfoPanel />}
            {showFAQPanel && <FAQPanel />}
            {showUptimePanel && <UptimePanel />}
          </>
        }
      />
    </>
  )
}

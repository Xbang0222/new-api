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
import { CircleHelp, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useApiKeys } from './api-keys-provider'

type ApiKeysPrimaryButtonsProps = {
  onOpenTour?: () => void
}

export function ApiKeysPrimaryButtons(props: ApiKeysPrimaryButtonsProps) {
  const { t } = useTranslation()
  const { setOpen } = useApiKeys()
  const user = useAuthStore((state) => state.auth.user)
  const showGuide = user?.role === ROLE.USER
  return (
    <div className='flex gap-2'>
      {showGuide && (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='ghost'
                size='icon-sm'
                onClick={props.onOpenTour}
                aria-label={t('View API key guide')}
              />
            }
          >
            <CircleHelp />
          </TooltipTrigger>
          <TooltipContent>{t('View API key guide')}</TooltipContent>
        </Tooltip>
      )}
      <Button
        size='sm'
        data-tour='api-key-create'
        onClick={() => setOpen('create')}
      >
        <Plus className='h-4 w-4' />
        {t('Create API Key')}
      </Button>
    </div>
  )
}

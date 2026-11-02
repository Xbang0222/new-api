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
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'

import {
  SettingsControlChildren,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import {
  isSidebarModuleSelected,
  type SidebarModulesAdminConfig,
} from './config'
import {
  getSidebarChildSection,
  SIDEBAR_CHILD_SECTIONS,
} from './sidebar-child-sections'

type SidebarChildControlsProps = {
  section: string
  config: SidebarModulesAdminConfig
  disabled?: boolean
  controlPrefix: string
  isAllowed?: (section: string, module: string) => boolean
  onChange: (section: string, module: string, checked: boolean) => void
}

export function SidebarChildControls(props: SidebarChildControlsProps) {
  const { t } = useTranslation()
  const definition = SIDEBAR_CHILD_SECTIONS[props.section]
  if (!definition) return null

  return (
    <SettingsControlChildren className='grid gap-x-6 sm:grid-cols-2'>
      {definition.modules.map((module) => {
        const allowed = props.isAllowed?.(props.section, module.key) ?? true
        const checked =
          allowed &&
          isSidebarModuleSelected(props.config, props.section, module.key)
        const disabled = props.disabled || !allowed
        const child = getSidebarChildSection(props.section, module.key)
        return (
          <Fragment key={module.key}>
            <SettingsSwitchField
              controlId={`${props.controlPrefix}-${props.section}-${module.key}`}
              label={t(module.titleKey)}
              checked={checked}
              onCheckedChange={(value) =>
                props.onChange(props.section, module.key, value)
              }
              disabled={disabled}
              className={disabled ? 'opacity-50' : undefined}
            />
            {child && (
              <div className='col-span-full'>
                <SidebarChildControls
                  {...props}
                  section={child[0]}
                  disabled={disabled || !checked}
                />
              </div>
            )}
          </Fragment>
        )
      })}
    </SettingsControlChildren>
  )
}

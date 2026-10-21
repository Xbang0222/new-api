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
import { Combobox } from '@/components/ui/combobox'
import {
  AlertTriangle,
  ChevronDown,
  GripVertical,
  Info,
  Plus,
  Trash2,
} from 'lucide-react'
import {
  useState,
  useMemo,
  useEffect,
  useCallback,
  memo,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Dialog } from '@/components/dialog'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

import { safeJsonParse } from '../utils/json-parser'

type GroupRatioVisualEditorProps = {
  groupRatio: string
  topupGroupRatio: string
  userGroups?: string
  groupDescriptions?: string
  userUsableGroups: string
  groupGroupRatio: string
  autoGroups: string
  maxTokenAutoGroupsField: ReactNode
  groupSpecialUsableGroup: string
  onChange: (field: string, value: string) => void
  /** Legacy callers may supply section controls; the visual editor is always rendered. */
  section?: string
  onSectionChange?: (section: string) => void
  defaultUseAutoGroupField?: ReactNode
}

type GroupPricingRow = {
  _id: string
  name: string
  ratio: string
  selectable: boolean
  description: string
}

type UserGroupRow = {
  _id: string
  name: string
  description: string
  topupRatio: string
}

type RegistryEntry = {
  name: string
  ratio: number
}

const sectionCardClassName =
  'relative shadow-sm ring-0 before:pointer-events-none before:absolute before:inset-0 before:rounded-xl before:border before:border-border/90'
const sectionHeaderClassName = 'border-b bg-muted/20'

let groupPricingIdCounter = 0
function createGroupPricingId() {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

function normalizeRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

function parseRatioMap(value: string): Record<string, number> {
  return safeJsonParse<Record<string, number>>(value, {
    fallback: {},
    silent: true,
  })
}

function parseUsableMap(value: string): Record<string, string> {
  return safeJsonParse<Record<string, string>>(value, {
    fallback: {},
    silent: true,
  })
}

function parseNestedRatioMap(
  value: string
): Record<string, Record<string, number>> {
  return safeJsonParse<Record<string, Record<string, number>>>(value, {
    fallback: {},
    silent: true,
  })
}

function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string,
  groupDescriptions: string
): GroupPricingRow[] {
  const ratioMap = parseRatioMap(groupRatio)
  const usableMap = parseUsableMap(userUsableGroups)
  const descriptionMap = parseUsableMap(groupDescriptions)
  const names = new Set([
    ...Object.keys(ratioMap),
    ...Object.keys(usableMap),
    ...Object.keys(descriptionMap),
  ])
  names.delete('auto')

  return [...names].map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: String(normalizeRatio(ratioMap[name])),
    selectable: Object.hasOwn(usableMap, name),
    description: String(descriptionMap[name] ?? usableMap[name] ?? ''),
  }))
}

function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}
  const groupDescriptions: Record<string, string> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeRatio(row.ratio)
    groupDescriptions[name] = row.description
    if (row.selectable) {
      userUsableGroups[name] = row.description
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    GroupDescriptions: JSON.stringify(groupDescriptions, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  }
}

function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: parseRatioMap(serialized.GroupRatio),
    groupDescriptions: parseUsableMap(serialized.GroupDescriptions),
    userUsableGroups: parseUsableMap(serialized.UserUsableGroups),
  })
}

function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string,
  groupDescriptions: string
): string {
  const descriptions = parseUsableMap(groupDescriptions)
  const usableGroups = parseUsableMap(userUsableGroups)
  delete descriptions.auto
  delete usableGroups.auto
  return JSON.stringify({
    groupRatio: parseRatioMap(groupRatio),
    groupDescriptions: descriptions,
    userUsableGroups: usableGroups,
  })
}

function buildUserGroupRows(
  userGroups: string,
  topupGroupRatio: string
): UserGroupRow[] {
  const userGroupMap = parseUsableMap(userGroups)
  const topupMap = parseRatioMap(topupGroupRatio)
  const names = new Set([
    ...Object.keys(userGroupMap),
    ...Object.keys(topupMap),
  ])

  return [...names].map((name) => ({
    _id: createGroupPricingId(),
    name,
    description: String(userGroupMap[name] ?? ''),
    topupRatio: Object.hasOwn(topupMap, name) ? String(topupMap[name]) : '',
  }))
}

function serializeUserGroupRows(rows: UserGroupRow[]) {
  const userGroups: Record<string, string> = {}
  const topupGroupRatio: Record<string, number> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    userGroups[name] = row.description
    const topup = row.topupRatio.trim()
    if (topup !== '' && Number.isFinite(Number(topup))) {
      topupGroupRatio[name] = Number(topup)
    }
  }

  return {
    UserGroups: JSON.stringify(userGroups, null, 2),
    TopupGroupRatio: JSON.stringify(topupGroupRatio, null, 2),
  }
}

function userGroupSignature(rows: UserGroupRow[]): string {
  const serialized = serializeUserGroupRows(rows)
  return JSON.stringify({
    userGroups: parseUsableMap(serialized.UserGroups),
    topupGroupRatio: parseRatioMap(serialized.TopupGroupRatio),
  })
}

function sourceUserGroupSignature(
  userGroups: string,
  topupGroupRatio: string
): string {
  return JSON.stringify({
    userGroups: parseUsableMap(userGroups),
    topupGroupRatio: parseRatioMap(topupGroupRatio),
  })
}

function UnknownGroupBadge() {
  const { t } = useTranslation()
  return (
    <StatusBadge variant='danger' copyable={false}>
      <AlertTriangle className='mr-1 h-3 w-3' />
      {t('Not in pricing table')}
    </StatusBadge>
  )
}

type GroupNameSelectProps = {
  options: string[]
  value: string | null
  placeholder: string
  onValueChange: (value: string) => void
  className?: string
}

function GroupNameSelect(props: GroupNameSelectProps) {
  const options = useMemo(() => {
    if (props.value && !props.options.includes(props.value)) {
      return [props.value, ...props.options]
    }
    return props.options
  }, [props.options, props.value])

  return (
    <Combobox
  options={options.map((name) => ({ value: name, label: name }))}
  value={props.value}
  onValueChange={(value) => { if (value) props.onValueChange(value) }}
  className={props.className ?? 'w-48'}
  placeholder={props.placeholder}
  aria-label={props.placeholder}
/>
  )
}

export const GroupRatioVisualEditor = memo(function GroupRatioVisualEditor({
  groupRatio,
  topupGroupRatio,
  userGroups = '{}',
  groupDescriptions = '{}',
  userUsableGroups,
  groupGroupRatio,
  autoGroups,
  maxTokenAutoGroupsField,
  groupSpecialUsableGroup,
  onChange,
}: GroupRatioVisualEditorProps) {
  const { t } = useTranslation()
  const [detailGroup, setDetailGroup] = useState<string | null>(null)

  const registry = useMemo<RegistryEntry[]>(() => {
    const ratioMap = parseRatioMap(groupRatio)
    const usableMap = parseUsableMap(userUsableGroups)
    const descriptionMap = parseUsableMap(groupDescriptions)
    const names = new Set([
      ...Object.keys(ratioMap),
      ...Object.keys(usableMap),
      ...Object.keys(descriptionMap),
    ])
    return [...names]
      .filter((name) => name !== 'auto')
      .map((name) => ({
        name,
        ratio: normalizeRatio(ratioMap[name]),
      }))
  }, [groupRatio, userUsableGroups, groupDescriptions])

  const registryNames = useMemo(
    () => registry.map((entry) => entry.name),
    [registry]
  )
  const userGroupNames = useMemo(
    () => Object.keys(parseUsableMap(userGroups)),
    [userGroups]
  )

  // Auto groups
  const autoGroupsList = useMemo(() => {
    return safeJsonParse<string[]>(autoGroups, {
      fallback: [],
      context: 'auto groups',
    })
  }, [autoGroups])
  const autoSelectable = useMemo(
    () => Object.hasOwn(parseUsableMap(userUsableGroups), 'auto'),
    [userUsableGroups]
  )
  const autoDescription = useMemo(() => {
    const descriptions = parseUsableMap(groupDescriptions)
    const usableGroups = parseUsableMap(userUsableGroups)
    return descriptions.auto ?? usableGroups.auto ?? ''
  }, [groupDescriptions, userUsableGroups])

  const handleAutoSelectableChange = useCallback(
    (checked: boolean) => {
      const usableGroups = parseUsableMap(userUsableGroups)
      const descriptions = parseUsableMap(groupDescriptions)
      if (checked) {
        const description = descriptions.auto ?? usableGroups.auto ?? 'Auto'
        usableGroups.auto = description
        descriptions.auto = description
      } else {
        delete usableGroups.auto
      }
      onChange('UserUsableGroups', JSON.stringify(usableGroups, null, 2))
      onChange('GroupDescriptions', JSON.stringify(descriptions, null, 2))
    },
    [groupDescriptions, onChange, userUsableGroups]
  )

  const handleAutoGroupAdd = useCallback(
    (name: string) => {
      if (autoGroupsList.includes(name)) return
      onChange('AutoGroups', JSON.stringify([...autoGroupsList, name], null, 2))
    },
    [autoGroupsList, onChange]
  )

  const handleAutoGroupDelete = useCallback(
    (index: number) => {
      const list = autoGroupsList.filter((_, i) => i !== index)
      onChange('AutoGroups', JSON.stringify(list, null, 2))
    },
    [autoGroupsList, onChange]
  )

  const handleAutoGroupMove = useCallback(
    (index: number, direction: 'up' | 'down') => {
      const list = [...autoGroupsList]
      const newIndex = direction === 'up' ? index - 1 : index + 1
      if (newIndex < 0 || newIndex >= list.length) return
      ;[list[index], list[newIndex]] = [list[newIndex], list[index]]
      onChange('AutoGroups', JSON.stringify(list, null, 2))
    },
    [autoGroupsList, onChange]
  )

  const autoGroupCandidates = useMemo(
    () => registryNames.filter((name) => !autoGroupsList.includes(name)),
    [registryNames, autoGroupsList]
  )

  return (
    <div className='space-y-4'>
      <UserGroupTable
        userGroups={userGroups}
        topupGroupRatio={topupGroupRatio}
        onChange={onChange}
      />

      <GroupPricingTable
        groupRatio={groupRatio}
        userUsableGroups={userUsableGroups}
        groupDescriptions={groupDescriptions}
        autoSelectable={autoSelectable}
        autoDescription={autoDescription}
        onChange={onChange}
        onShowDetail={setDetailGroup}
      />

      <GroupOverrideRules
        registry={registry}
        userGroupNames={userGroupNames}
        groupGroupRatio={groupGroupRatio}
        onChange={onChange}
      />

      {/* Auto Groups */}
      <Card className={sectionCardClassName}>
        <CardHeader className={sectionHeaderClassName}>
          <CardTitle>{t('Auto assignment order')}</CardTitle>
          <CardDescription>
            {t(
              'Priority order for tokens in the auto group. The system tries groups from top to bottom.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <div className='flex items-start gap-3 rounded-md border p-3'>
              <Checkbox
                id='auto-group-selectable'
                checked={autoSelectable}
                onCheckedChange={(checked) =>
                  handleAutoSelectableChange(checked === true)
                }
                aria-label={t('Allow users to select Auto')}
              />
              <div className='space-y-1'>
                <Label htmlFor='auto-group-selectable'>
                  {t('Allow users to select Auto')}
                </Label>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Auto is a virtual routing group. It follows the assignment order below and has no billing ratio of its own.'
                  )}
                </p>
              </div>
            </div>
            {maxTokenAutoGroupsField}
            <GroupNameSelect
              options={autoGroupCandidates}
              value={null}
              placeholder={t('Add group')}
              onValueChange={handleAutoGroupAdd}
            />
            {autoGroupsList.length > 0 && (
              <div className='space-y-2'>
                {autoGroupsList.map((group, index) => (
                  <div
                    key={group}
                    className='flex items-center gap-2 rounded-md border p-3'
                  >
                    <GripVertical className='text-muted-foreground h-4 w-4' />
                    <span className='font-medium'>{group}</span>
                    {!registryNames.includes(group) && <UnknownGroupBadge />}
                    <div className='ml-auto flex gap-1'>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === 0}
                        onClick={() => handleAutoGroupMove(index, 'up')}
                      >
                        ↑
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        disabled={index === autoGroupsList.length - 1}
                        onClick={() => handleAutoGroupMove(index, 'down')}
                      >
                        ↓
                      </Button>
                      <Button
                        variant='ghost'
                        size='sm'
                        onClick={() => handleAutoGroupDelete(index)}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      <GroupDetailSheet
        groupName={detailGroup}
        onOpenChange={(open) => {
          if (!open) setDetailGroup(null)
        }}
        registry={registry}
        userUsableGroups={userUsableGroups}
        groupDescriptions={groupDescriptions}
        groupGroupRatio={groupGroupRatio}
        autoGroups={autoGroupsList}
        groupSpecialUsableGroup={groupSpecialUsableGroup}
      />
    </div>
  )
})

type UserGroupTableProps = {
  userGroups: string
  topupGroupRatio: string
  onChange: (field: string, value: string) => void
}

function UserGroupTable(props: UserGroupTableProps) {
  const { t } = useTranslation()
  const onChange = props.onChange
  const [rows, setRows] = useState<UserGroupRow[]>(() =>
    buildUserGroupRows(props.userGroups, props.topupGroupRatio)
  )

  useEffect(() => {
    const incomingSignature = sourceUserGroupSignature(
      props.userGroups,
      props.topupGroupRatio
    )
    setRows((currentRows) => {
      if (userGroupSignature(currentRows) === incomingSignature) {
        return currentRows
      }
      return buildUserGroupRows(props.userGroups, props.topupGroupRatio)
    })
  }, [props.userGroups, props.topupGroupRatio])

  const emitRows = useCallback(
    (nextRows: UserGroupRow[]) => {
      setRows(nextRows)
      const serialized = serializeUserGroupRows(nextRows)
      onChange('UserGroups', serialized.UserGroups)
      onChange('TopupGroupRatio', serialized.TopupGroupRatio)
    },
    [onChange]
  )

  const updateRow = useCallback(
    (id: string, field: Exclude<keyof UserGroupRow, '_id'>, value: string) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `user_group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `user_group_${index}`
    }
    emitRows([
      ...rows,
      {
        _id: createGroupPricingId(),
        name,
        description: '',
        topupRatio: '',
      },
    ])
  }, [emitRows, rows])

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (name) counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return new Set(
      [...counts.entries()]
        .filter(([, count]) => count > 1)
        .map(([name]) => name)
    )
  }, [rows])

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <CardTitle>{t('User groups')}</CardTitle>
        <CardDescription>
          {t(
            'User groups are assigned to users. They have recharge ratios and can define special pricing rules, but no base model billing ratio.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <Button size='sm' onClick={addRow}>
          <Plus className='mr-2 h-4 w-4' />
          {t('Add user group')}
        </Button>
        <StaticDataTable
          data={rows}
          getRowKey={(row) => row._id}
          emptyClassName='text-muted-foreground h-20 text-sm'
          emptyContent={t('No user groups configured')}
          columns={[
            {
              id: 'name',
              header: t('User group name'),
              className: 'min-w-48',
              cell: (row) => (
                <div className='space-y-1'>
                  <Input
                    value={row.name}
                    aria-invalid={duplicateNames.has(row.name.trim())}
                    onChange={(event) =>
                      updateRow(row._id, 'name', event.target.value)
                    }
                  />
                  {duplicateNames.has(row.name.trim()) && (
                    <p className='text-destructive text-xs'>
                      {t('Group names must be unique')}
                    </p>
                  )}
                </div>
              ),
            },
            {
              id: 'description',
              header: t('Description'),
              className: 'min-w-56',
              cell: (row) => (
                <Input
                  value={row.description}
                  placeholder={t('User group description')}
                  onChange={(event) =>
                    updateRow(row._id, 'description', event.target.value)
                  }
                />
              ),
            },
            {
              id: 'topup-ratio',
              header: t('Top-up ratio'),
              className: 'w-36',
              cell: (row) => (
                <Input
                  type='number'
                  min={0}
                  step={0.1}
                  value={row.topupRatio}
                  placeholder={t('Not set')}
                  onChange={(event) =>
                    updateRow(row._id, 'topupRatio', event.target.value)
                  }
                />
              ),
            },
            {
              id: 'actions',
              header: t('Actions'),
              className: 'text-right',
              cellClassName: 'text-right',
              cell: (row) => (
                <Button
                  variant='ghost'
                  size='sm'
                  aria-label={t('Delete')}
                  onClick={() =>
                    emitRows(rows.filter((item) => item._id !== row._id))
                  }
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              ),
            },
          ]}
        />
      </CardContent>
    </Card>
  )
}

type GroupPricingTableProps = {
  groupRatio: string
  userUsableGroups: string
  groupDescriptions: string
  autoSelectable: boolean
  autoDescription: string
  onChange: (field: string, value: string) => void
  onShowDetail: (name: string) => void
}

function GroupPricingTable({
  groupRatio,
  userUsableGroups,
  groupDescriptions,
  autoSelectable,
  autoDescription,
  onChange,
  onShowDetail,
}: GroupPricingTableProps) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<GroupPricingRow[]>(() =>
    buildGroupPricingRows(groupRatio, userUsableGroups, groupDescriptions)
  )

  useEffect(() => {
    const incomingSignature = sourceGroupPricingSignature(
      groupRatio,
      userUsableGroups,
      groupDescriptions
    )
    setRows((currentRows) => {
      if (groupPricingSignature(currentRows) === incomingSignature) {
        return currentRows
      }
      return buildGroupPricingRows(
        groupRatio,
        userUsableGroups,
        groupDescriptions
      )
    })
  }, [groupRatio, userUsableGroups, groupDescriptions])

  const emitRows = useCallback(
    (nextRows: GroupPricingRow[]) => {
      setRows(nextRows)
      const serialized = serializeGroupPricingRows(nextRows)
      const nextDescriptions = parseUsableMap(serialized.GroupDescriptions)
      if (autoDescription !== '') {
        nextDescriptions.auto = autoDescription
      }
      const nextUsableGroups = parseUsableMap(serialized.UserUsableGroups)
      if (autoSelectable) {
        nextUsableGroups.auto = autoDescription
      }
      onChange('GroupRatio', serialized.GroupRatio)
      onChange('GroupDescriptions', JSON.stringify(nextDescriptions, null, 2))
      onChange('UserUsableGroups', JSON.stringify(nextUsableGroups, null, 2))
    },
    [autoDescription, autoSelectable, onChange]
  )

  const updateRow = useCallback(
    (
      id: string,
      field: Exclude<keyof GroupPricingRow, '_id'>,
      value: string | number | boolean
    ) => {
      emitRows(
        rows.map((row) => (row._id === id ? { ...row, [field]: value } : row))
      )
    },
    [emitRows, rows]
  )

  const addRow = useCallback(() => {
    const existingNames = new Set(rows.map((row) => row.name))
    let index = 1
    let name = `group_${index}`
    while (existingNames.has(name)) {
      index += 1
      name = `group_${index}`
    }
    emitRows([
      {
        _id: createGroupPricingId(),
        name,
        ratio: '1',
        selectable: true,
        description: '',
      },
      ...rows,
    ])
  }, [emitRows, rows])

  const removeRow = useCallback(
    (id: string) => {
      emitRows(rows.filter((row) => row._id !== id))
    },
    [emitRows, rows]
  )

  const duplicateNames = useMemo(() => {
    const counts = new Map<string, number>()
    for (const row of rows) {
      const name = row.name.trim()
      if (!name) continue
      counts.set(name, (counts.get(name) ?? 0) + 1)
    }
    return [...counts.entries()]
      .filter(([, count]) => count > 1)
      .map(([name]) => name)
  }, [rows])

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
          <div>
            <CardTitle>{t('Service groups')}</CardTitle>
            <CardDescription>
              {t(
                'Service groups control channel routing and the base model billing ratio. Hidden groups are removed from API key choices and the model marketplace, while their descriptions are preserved.'
              )}
            </CardDescription>
          </div>
          <Button onClick={addRow} size='sm' className='sm:self-start'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add service group')}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className='space-y-3'>
          <StaticDataTable
            data={rows}
            getRowKey={(row) => row._id}
            emptyClassName='text-muted-foreground h-20 text-sm'
            emptyContent={t(
              'No service groups yet. Add a service group to get started.'
            )}
            columns={[
              {
                id: 'group',
                header: t('Service group name'),
                className: 'min-w-40',
                cell: (row) => (
                  <Input
                    value={row.name}
                    aria-label={t('Service group name')}
                    onChange={(event) =>
                      updateRow(row._id, 'name', event.target.value)
                    }
                    aria-invalid={duplicateNames.includes(row.name.trim())}
                  />
                ),
              },
              {
                id: 'ratio',
                header: t('Ratio'),
                className: 'w-28',
                cell: (row) => (
                  <Input
                    type='number'
                    min={0.000001}
                    step={0.1}
                    value={row.ratio}
                    onChange={(event) =>
                      updateRow(row._id, 'ratio', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'selectable',
                header: t('Visible and selectable'),
                className: 'w-28 text-center',
                cell: (row) => (
                  <div className='flex justify-center'>
                    <Checkbox
                      checked={row.selectable}
                      onCheckedChange={(checked) =>
                        updateRow(row._id, 'selectable', checked === true)
                      }
                      aria-label={t('Visible and selectable')}
                    />
                  </div>
                ),
              },
              {
                id: 'description',
                header: t('Description'),
                className: 'min-w-56',
                cell: (row) => (
                  <Input
                    value={row.description}
                    placeholder={t('Service group description')}
                    onChange={(event) =>
                      updateRow(row._id, 'description', event.target.value)
                    }
                  />
                ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (row) => (
                  <div className='flex justify-end gap-1'>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => onShowDetail(row.name.trim())}
                      disabled={!row.name.trim()}
                      aria-label={t('Details')}
                    >
                      <Info className='h-4 w-4' />
                    </Button>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => removeRow(row._id)}
                      aria-label={t('Delete')}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                ),
              },
            ]}
          />

          {duplicateNames.length > 0 && (
            <p className='text-destructive text-sm'>
              {t('Duplicate group names: {{names}}', {
                names: duplicateNames.join(', '),
              })}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

type GroupOverride = {
  targetGroup: string
  ratio: number
}

type GroupOverrideRulesProps = {
  registry: RegistryEntry[]
  userGroupNames: string[]
  groupGroupRatio: string
  onChange: (field: string, value: string) => void
}

function GroupOverrideRules({
  registry,
  userGroupNames,
  groupGroupRatio,
  onChange,
}: GroupOverrideRulesProps) {
  const { t } = useTranslation()
  const [userGroupDialogOpen, setUserGroupDialogOpen] = useState(false)
  const [userGroupInput, setUserGroupInput] = useState<string | null>(null)
  const [overrideDialogOpen, setOverrideDialogOpen] = useState(false)
  const [overrideUserGroup, setOverrideUserGroup] = useState<string | null>(
    null
  )
  const [overrideEditData, setOverrideEditData] =
    useState<GroupOverride | null>(null)

  const registryNames = useMemo(
    () => registry.map((entry) => entry.name),
    [registry]
  )

  const baseRatioByName = useMemo(() => {
    const map = new Map<string, number>()
    for (const entry of registry) map.set(entry.name, entry.ratio)
    return map
  }, [registry])

  const groupGroupRatioList = useMemo(() => {
    const map = parseNestedRatioMap(groupGroupRatio)
    return Object.entries(map).map(([userGroup, overrides]) => ({
      userGroup,
      overrides: Object.entries(overrides).map(([targetGroup, ratio]) => ({
        targetGroup,
        ratio,
      })),
    }))
  }, [groupGroupRatio])

  const emitMap = useCallback(
    (map: Record<string, Record<string, number>>) => {
      onChange('GroupGroupRatio', JSON.stringify(map, null, 2))
    },
    [onChange]
  )

  const handleUserGroupSave = useCallback(() => {
    if (!userGroupInput) return
    const map = parseNestedRatioMap(groupGroupRatio)
    if (!map[userGroupInput]) {
      map[userGroupInput] = {}
    }
    emitMap(map)
    setUserGroupDialogOpen(false)
    setUserGroupInput(null)
  }, [userGroupInput, groupGroupRatio, emitMap])

  const handleUserGroupDelete = useCallback(
    (userGroup: string) => {
      const map = parseNestedRatioMap(groupGroupRatio)
      delete map[userGroup]
      emitMap(map)
    },
    [groupGroupRatio, emitMap]
  )

  const handleOverrideAdd = useCallback((userGroup: string) => {
    setOverrideUserGroup(userGroup)
    setOverrideEditData(null)
    setOverrideDialogOpen(true)
  }, [])

  const handleOverrideEdit = useCallback(
    (userGroup: string, override: GroupOverride) => {
      setOverrideUserGroup(userGroup)
      setOverrideEditData(override)
      setOverrideDialogOpen(true)
    },
    []
  )

  const handleOverrideSave = useCallback(
    (targetGroup: string, ratio: number, oldTargetGroup?: string) => {
      if (!overrideUserGroup) return
      const map = parseNestedRatioMap(groupGroupRatio)
      if (!map[overrideUserGroup]) {
        map[overrideUserGroup] = {}
      }
      if (oldTargetGroup && oldTargetGroup !== targetGroup) {
        delete map[overrideUserGroup][oldTargetGroup]
      }
      map[overrideUserGroup][targetGroup] = ratio
      emitMap(map)
      setOverrideDialogOpen(false)
    },
    [overrideUserGroup, groupGroupRatio, emitMap]
  )

  const handleOverrideDelete = useCallback(
    (userGroup: string, targetGroup: string) => {
      const map = parseNestedRatioMap(groupGroupRatio)
      if (map[userGroup]) {
        delete map[userGroup][targetGroup]
        if (Object.keys(map[userGroup]).length === 0) {
          delete map[userGroup]
        }
      }
      emitMap(map)
    },
    [groupGroupRatio, emitMap]
  )

  return (
    <Card className={sectionCardClassName}>
      <CardHeader className={sectionHeaderClassName}>
        <CardTitle>{t('Special ratio rules')}</CardTitle>
        <CardDescription>
          {t(
            'Each rule links one user group to one service group. Without a matching rule, the service group base ratio applies.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className='space-y-4'>
          <Button
            onClick={() => {
              setUserGroupInput(null)
              setUserGroupDialogOpen(true)
            }}
            size='sm'
          >
            <Plus className='mr-2 h-4 w-4' />
            {t('Add user group')}
          </Button>
          {groupGroupRatioList.length > 0 && (
            <div className='space-y-3'>
              {groupGroupRatioList.map((userGroupData) => (
                <Collapsible key={userGroupData.userGroup}>
                  <div className='rounded-lg border'>
                    <div className='flex items-center justify-between p-4'>
                      <div className='flex items-center gap-2'>
                        <CollapsibleTrigger
                          render={<Button variant='ghost' size='sm' />}
                        >
                          <ChevronDown className='h-4 w-4' />
                        </CollapsibleTrigger>
                        <span className='font-semibold'>
                          {userGroupData.userGroup}
                        </span>
                        {!userGroupNames.includes(userGroupData.userGroup) && (
                          <AlertTriangle
                            className='text-destructive h-4 w-4'
                            aria-label={t('Unknown user group')}
                          />
                        )}
                        <span className='text-muted-foreground text-sm'>
                          {t('{{count}} override', {
                            count: userGroupData.overrides.length,
                          })}
                        </span>
                      </div>
                      <div className='flex gap-2'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() =>
                            handleOverrideAdd(userGroupData.userGroup)
                          }
                        >
                          <Plus className='h-4 w-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() =>
                            handleUserGroupDelete(userGroupData.userGroup)
                          }
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </div>
                    <CollapsibleContent>
                      {userGroupData.overrides.length > 0 && (
                        <div className='border-t'>
                          <StaticDataTable
                            className='rounded-none border-0'
                            data={userGroupData.overrides}
                            getRowKey={(override) => override.targetGroup}
                            columns={[
                              {
                                id: 'target-group',
                                header: t('Service group'),
                                cellClassName: 'font-medium',
                                cell: (override) => (
                                  <span className='inline-flex items-center gap-1.5'>
                                    {override.targetGroup}
                                    {!registryNames.includes(
                                      override.targetGroup
                                    ) && (
                                      <AlertTriangle
                                        className='text-destructive h-3.5 w-3.5'
                                        aria-label={t('Not in pricing table')}
                                      />
                                    )}
                                  </span>
                                ),
                              },
                              {
                                id: 'ratio',
                                header: t('Ratio'),
                                cell: (override) => {
                                  const baseRatio = baseRatioByName.get(
                                    override.targetGroup
                                  )
                                  return (
                                    <span className='inline-flex items-center gap-1.5'>
                                      {override.ratio}
                                      {baseRatio !== undefined &&
                                        baseRatio !== override.ratio && (
                                          <span className='text-muted-foreground text-xs'>
                                            {t('(instead of {{ratio}})', {
                                              ratio: baseRatio,
                                            })}
                                          </span>
                                        )}
                                    </span>
                                  )
                                },
                              },
                              {
                                id: 'actions',
                                header: t('Actions'),
                                className: 'text-right',
                                cellClassName: 'text-right',
                                cell: (override) => (
                                  <StaticRowActions
                                    editLabel={t('Edit')}
                                    deleteLabel={t('Delete')}
                                    menuLabel={t('Open menu')}
                                    onEdit={() =>
                                      handleOverrideEdit(
                                        userGroupData.userGroup,
                                        override
                                      )
                                    }
                                    onDelete={() =>
                                      handleOverrideDelete(
                                        userGroupData.userGroup,
                                        override.targetGroup
                                      )
                                    }
                                  />
                                ),
                              },
                            ]}
                          />
                        </div>
                      )}
                    </CollapsibleContent>
                  </div>
                </Collapsible>
              ))}
            </div>
          )}
        </div>
      </CardContent>

      {/* Add user group dialog */}
      <Dialog
        open={userGroupDialogOpen}
        onOpenChange={setUserGroupDialogOpen}
        title={t('Add user group')}
        description={t(
          'Create a new user group to configure ratio overrides for.'
        )}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setUserGroupDialogOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button onClick={handleUserGroupSave} disabled={!userGroupInput}>
              {t('Add')}
            </Button>
          </>
        }
      >
        <div className='space-y-4 py-4'>
          <div className='space-y-2'>
            <Label>{t('User group name')}</Label>
            <GroupNameSelect
              className='w-full'
              options={userGroupNames}
              value={userGroupInput}
              placeholder={t('Select a user group')}
              onValueChange={setUserGroupInput}
            />
          </div>
        </div>
      </Dialog>

      <GroupOverrideDialog
        open={overrideDialogOpen}
        onOpenChange={setOverrideDialogOpen}
        onSave={handleOverrideSave}
        editData={overrideEditData}
        userGroup={overrideUserGroup}
        groupOptions={registryNames}
        baseRatioByName={baseRatioByName}
      />
    </Card>
  )
}

type GroupOverrideDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (targetGroup: string, ratio: number, oldTargetGroup?: string) => void
  editData: GroupOverride | null
  userGroup: string | null
  groupOptions: string[]
  baseRatioByName: Map<string, number>
}

function GroupOverrideDialog({
  open,
  onOpenChange,
  onSave,
  editData,
  userGroup,
  groupOptions,
  baseRatioByName,
}: GroupOverrideDialogProps) {
  const { t } = useTranslation()
  const [targetGroup, setTargetGroup] = useState<string | null>(null)
  const [ratio, setRatio] = useState('')

  useEffect(() => {
    if (!open) {
      setTargetGroup(null)
      setRatio('')
      return
    }

    setTargetGroup(editData?.targetGroup ?? null)
    setRatio(editData ? String(editData.ratio) : '')
  }, [editData, open])

  const baseRatio = targetGroup ? baseRatioByName.get(targetGroup) : undefined

  const handleSave = () => {
    if (!targetGroup || !ratio.trim()) return
    const parsedRatio = Number.parseFloat(ratio)
    if (Number.isNaN(parsedRatio) || parsedRatio < 0) return

    onSave(targetGroup, parsedRatio, editData?.targetGroup)
    setTargetGroup(null)
    setRatio('')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={editData ? t('Edit ratio override') : t('Add ratio override')}
      description={
        userGroup
          ? t(
              'Configure a custom ratio for "{{userGroup}}" users when using a specific service group.',
              { userGroup }
            )
          : t('Configure a custom ratio for a specific service group.')
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSave}>
            {editData ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-4 py-4'>
        <div className='space-y-2'>
          <Label>{t('Service group')}</Label>
          <GroupNameSelect
            className='w-full'
            options={groupOptions}
            value={targetGroup}
            placeholder={t('Select a group')}
            onValueChange={setTargetGroup}
          />
          <p className='text-muted-foreground text-xs'>
            {t('The service group that will have a custom ratio')}
          </p>
        </div>
        <div className='space-y-2'>
          <Label>{t('Ratio')}</Label>
          <Input
            type='number'
            min={0}
            step='any'
            value={ratio}
            onChange={(e) => {
              const val = e.target.value
              if (val === '' || !Number.isNaN(Number.parseFloat(val))) {
                setRatio(val)
              }
            }}
            placeholder={baseRatio === undefined ? '0.9' : String(baseRatio)}
          />
          <p className='text-muted-foreground text-xs'>
            {baseRatio !== undefined
              ? t('(instead of {{ratio}})', { ratio: baseRatio })
              : t(
                  'Multiplier applied when {{userGroup}} uses {{targetGroup}}',
                  {
                    userGroup: userGroup || t('this user group'),
                    targetGroup: targetGroup || t('this service group'),
                  }
                )}
          </p>
        </div>
      </div>
    </Dialog>
  )
}

type GroupDetailSheetProps = {
  groupName: string | null
  onOpenChange: (open: boolean) => void
  registry: RegistryEntry[]
  userUsableGroups: string
  groupDescriptions: string
  groupGroupRatio: string
  autoGroups: string[]
  groupSpecialUsableGroup: string
}

type VisibilityRule = {
  userGroup: string
  visible: boolean
  description: string
}

function parseSpecialGroupKey(rawKey: string): {
  visible: boolean
  groupName: string
} {
  if (rawKey.startsWith('-:')) {
    return { visible: false, groupName: rawKey.slice(2) }
  }
  if (rawKey.startsWith('+:')) {
    return { visible: true, groupName: rawKey.slice(2) }
  }
  return { visible: true, groupName: rawKey }
}

function GroupDetailSheet(props: GroupDetailSheetProps) {
  const { t } = useTranslation()
  const name = props.groupName

  const detail = useMemo(() => {
    if (!name) return null

    const entry = props.registry.find((item) => item.name === name)
    const usableMap = parseUsableMap(props.userUsableGroups)
    const descriptionMap = parseUsableMap(props.groupDescriptions)
    const overrideMap = parseNestedRatioMap(props.groupGroupRatio)
    const specialMap = safeJsonParse<Record<string, Record<string, string>>>(
      props.groupSpecialUsableGroup,
      { fallback: {}, silent: true }
    )

    // Overrides that apply when other user groups bill as this group
    const incomingOverrides: { userGroup: string; ratio: number }[] = []
    for (const [userGroup, overrides] of Object.entries(overrideMap)) {
      if (Object.hasOwn(overrides, name)) {
        incomingOverrides.push({ userGroup, ratio: overrides[name] })
      }
    }

    // Overrides that apply when users of this group bill as other groups
    const outgoingOverrides = Object.entries(overrideMap[name] ?? {}).map(
      ([targetGroup, ratio]) => ({ targetGroup, ratio })
    )

    // Visibility rules targeting this group
    const visibilityRules: VisibilityRule[] = []
    for (const [userGroup, inner] of Object.entries(specialMap)) {
      if (typeof inner !== 'object' || inner === null) continue
      for (const [rawKey, desc] of Object.entries(inner)) {
        const parsed = parseSpecialGroupKey(rawKey)
        if (parsed.groupName !== name) continue
        visibilityRules.push({
          userGroup,
          visible: parsed.visible,
          description: typeof desc === 'string' ? desc : '',
        })
      }
    }

    const autoIndex = props.autoGroups.indexOf(name)

    return {
      ratio: entry?.ratio,
      selectable: Object.hasOwn(usableMap, name),
      description: String(descriptionMap[name] ?? usableMap[name] ?? ''),
      incomingOverrides,
      outgoingOverrides,
      visibilityRules,
      autoIndex,
    }
  }, [
    name,
    props.registry,
    props.userUsableGroups,
    props.groupDescriptions,
    props.groupGroupRatio,
    props.autoGroups,
    props.groupSpecialUsableGroup,
  ])

  return (
    <Sheet open={name !== null} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-lg')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {t('Group details')}
            {name ? `: ${name}` : ''}
          </SheetTitle>
          <SheetDescription>
            {t('Everything configured for this group, in one place.')}
          </SheetDescription>
        </SheetHeader>

        {detail && (
          <div className={sideDrawerFormClassName('gap-5')}>
            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>{t('Overview')}</h3>
              <dl className='space-y-1.5 text-sm'>
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>{t('Ratio')}</dt>
                  <dd className='font-medium'>{detail.ratio ?? '-'}</dd>
                </div>
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>
                    {t('Visible and selectable')}
                  </dt>
                  <dd className='font-medium'>
                    {detail.selectable ? t('Yes') : t('No')}
                  </dd>
                </div>
                {detail.description && (
                  <div className='flex justify-between gap-4'>
                    <dt className='text-muted-foreground'>
                      {t('Description')}
                    </dt>
                    <dd className='text-right font-medium'>
                      {detail.description}
                    </dd>
                  </div>
                )}
                <div className='flex justify-between'>
                  <dt className='text-muted-foreground'>
                    {t('Auto assignment order')}
                  </dt>
                  <dd className='font-medium'>
                    {detail.autoIndex >= 0
                      ? t('Position {{position}}', {
                          position: detail.autoIndex + 1,
                        })
                      : t('Not included')}
                  </dd>
                </div>
              </dl>
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Ratio overrides when billed as this group')}
              </h3>
              {detail.incomingOverrides.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.incomingOverrides.map((item) => (
                    <li
                      key={item.userGroup}
                      className='flex justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {t('Users in {{group}}', { group: item.userGroup })}
                      </span>
                      <span className='font-medium'>{item.ratio}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Ratio overrides for users of this group')}
              </h3>
              {detail.outgoingOverrides.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.outgoingOverrides.map((item) => (
                    <li
                      key={item.targetGroup}
                      className='flex justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {t('When billed as {{group}}', {
                          group: item.targetGroup,
                        })}
                      </span>
                      <span className='font-medium'>{item.ratio}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>

            <section className='space-y-2'>
              <h3 className='text-sm font-semibold'>
                {t('Special visibility rules')}
              </h3>
              {detail.visibilityRules.length === 0 ? (
                <p className='text-muted-foreground text-sm'>{t('None')}</p>
              ) : (
                <ul className='space-y-1 text-sm'>
                  {detail.visibilityRules.map((rule) => (
                    <li
                      key={`${rule.userGroup}-${rule.visible}`}
                      className='flex items-center justify-between rounded-md border px-3 py-1.5'
                    >
                      <span>
                        {rule.visible
                          ? t('Extra visible to {{group}}', {
                              group: rule.userGroup,
                            })
                          : t('Hidden from {{group}}', {
                              group: rule.userGroup,
                            })}
                      </span>
                      <StatusBadge
                        variant={rule.visible ? 'info' : 'danger'}
                        copyable={false}
                      >
                        {rule.visible ? t('Visible') : t('Hidden')}
                      </StatusBadge>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}

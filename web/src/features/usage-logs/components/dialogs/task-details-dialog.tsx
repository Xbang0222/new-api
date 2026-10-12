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
import { AlertTriangle, Clock3, Info, ReceiptText, Server } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Label } from '@/components/ui/label'
import { formatLogQuota, formatTimestampToDate } from '@/lib/format'
import { translateServerText } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { taskActionMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'

function DetailRow(props: {
  label: React.ReactNode
  value: React.ReactNode
  mono?: boolean
}) {
  return (
    <div className='grid min-w-0 grid-cols-[6rem_minmax(0,1fr)] gap-2 text-sm sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3'>
      <span className='text-muted-foreground min-w-0 text-xs'>
        {props.label}
      </span>
      <span
        className={cn(
          'max-w-full min-w-0 text-xs break-all sm:wrap-break-word',
          props.mono && 'font-mono'
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function DetailSection(props: {
  icon: React.ReactNode
  label: string
  danger?: boolean
  children: React.ReactNode
}) {
  return (
    <section className='min-w-0 space-y-1.5'>
      <Label
        className={cn(
          'flex items-center gap-1.5 text-xs font-semibold',
          props.danger && 'text-red-600 dark:text-red-400'
        )}
      >
        {props.icon}
        {props.label}
      </Label>
      <div
        className={cn(
          'min-w-0 space-y-1.5 overflow-hidden rounded-md border p-2.5 max-sm:p-2',
          props.danger
            ? 'border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-950/20'
            : 'bg-muted/30'
        )}
      >
        {props.children}
      </div>
    </section>
  )
}

function parseProperties(properties: TaskLog['properties']) {
  if (!properties) return null
  if (typeof properties !== 'string') return properties
  try {
    const parsed: unknown = JSON.parse(properties)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Exclude<TaskLog['properties'], string | undefined>
    }
  } catch {
    return null
  }
  return null
}

function prettyTaskData(data: unknown): string {
  if (data == null || data === '') return ''
  if (typeof data === 'string') {
    try {
      return JSON.stringify(JSON.parse(data), null, 2)
    } catch {
      return data
    }
  }
  try {
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}

export function TaskDetailsDialog(props: {
  log: TaskLog
  isAdmin: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const properties = parseProperties(props.log.properties)
  const taskData = props.isAdmin ? prettyTaskData(props.log.data) : ''
  const duration =
    props.log.finish_time && props.log.finish_time >= props.log.submit_time
      ? props.log.finish_time - props.log.submit_time
      : null

  if (!props.isAdmin) {
    return (
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={t('Details')}
        description={t('View the complete details for this log entry')}
        contentClassName='min-w-0 overflow-hidden sm:max-w-lg'
        contentHeight='auto'
      >
        <DetailSection
          icon={<Info className='size-3.5' aria-hidden='true' />}
          label={t('Overview')}
        >
          <DetailRow label={t('Task ID')} value={props.log.task_id} mono />
          <DetailRow
            label={t('Original Model')}
            value={properties?.origin_model_name || '-'}
            mono
          />
          <DetailRow
            label={t('Duration')}
            value={duration == null ? '-' : `${duration}s`}
            mono
          />
          <DetailRow
            label={t('Cost')}
            value={
              props.log.quota == null ? '-' : formatLogQuota(props.log.quota)
            }
            mono
          />
          <DetailRow label={t('Group')} value={props.log.group || '-'} mono />
        </DetailSection>
      </Dialog>
    )
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={
        <span className='flex items-center gap-2'>
          {t('Details')}
          <StatusBadge
            label={t(
              taskStatusMapper.getLabel(
                props.log.status,
                props.log.status || 'Submitting'
              )
            )}
            variant={taskStatusMapper.getVariant(props.log.status)}
            size='sm'
            copyable={false}
          />
        </span>
      }
      description={t('View the complete details for this log entry')}
      contentClassName='min-w-0 overflow-hidden sm:max-w-lg'
      contentHeight='min(78dvh, 720px)'
      bodyClassName='pr-2 sm:pr-4'
    >
      <div className='w-full max-w-full min-w-0 space-y-3 overflow-x-hidden py-1'>
        <DetailSection
          icon={<Info className='size-3.5' aria-hidden='true' />}
          label={t('Overview')}
        >
          <DetailRow label={t('Task ID')} value={props.log.task_id} mono />
          <DetailRow label='ID' value={String(props.log.id)} mono />
          <DetailRow label={t('Platform')} value={t(props.log.platform)} />
          <DetailRow
            label={t('Action')}
            value={t(
              taskActionMapper.getLabel(props.log.action, props.log.action)
            )}
          />
          <DetailRow
            label={t('Status')}
            value={t(
              taskStatusMapper.getLabel(
                props.log.status,
                props.log.status || 'Submitting'
              )
            )}
          />
          {props.log.progress ? (
            <DetailRow label={t('Progress')} value={props.log.progress} mono />
          ) : null}
          {properties?.origin_model_name ? (
            <DetailRow
              label={t('Original Model')}
              value={properties.origin_model_name}
              mono
            />
          ) : null}
          {properties?.upstream_model_name ? (
            <DetailRow
              label={t('Actual Model')}
              value={properties.upstream_model_name}
              mono
            />
          ) : null}
        </DetailSection>

        <DetailSection
          icon={<Clock3 className='size-3.5' aria-hidden='true' />}
          label={t('Timing')}
        >
          {props.log.created_at ? (
            <DetailRow
              label={t('Created At')}
              value={formatTimestampToDate(props.log.created_at, 'seconds')}
              mono
            />
          ) : null}
          <DetailRow
            label={t('Submit Time')}
            value={formatTimestampToDate(props.log.submit_time, 'seconds')}
            mono
          />
          {props.log.start_time ? (
            <DetailRow
              label={t('Start Time')}
              value={formatTimestampToDate(props.log.start_time, 'seconds')}
              mono
            />
          ) : null}
          {props.log.finish_time ? (
            <DetailRow
              label={t('Finish Time')}
              value={formatTimestampToDate(props.log.finish_time, 'seconds')}
              mono
            />
          ) : null}
          {duration != null ? (
            <DetailRow label={t('Duration')} value={`${duration}s`} mono />
          ) : null}
        </DetailSection>

        {props.log.quota != null || props.log.group || props.isAdmin ? (
          <DetailSection
            icon={<ReceiptText className='size-3.5' aria-hidden='true' />}
            label={t('Billing')}
          >
            {props.log.quota != null ? (
              <DetailRow
                label={t('Cost')}
                value={formatLogQuota(props.log.quota)}
                mono
              />
            ) : null}
            {props.log.group ? (
              <DetailRow label={t('Group')} value={props.log.group} mono />
            ) : null}
            {props.isAdmin ? (
              <DetailRow
                label={t('Channel')}
                value={`#${props.log.channel_id}`}
                mono
              />
            ) : null}
            {props.isAdmin && props.log.username ? (
              <DetailRow label={t('User')} value={props.log.username} />
            ) : null}
            {props.isAdmin ? (
              <DetailRow
                label={t('User ID')}
                value={String(props.log.user_id)}
                mono
              />
            ) : null}
          </DetailSection>
        ) : null}

        {props.log.fail_reason ? (
          <DetailSection
            icon={<AlertTriangle className='size-3.5' aria-hidden='true' />}
            label={t('Fail Reason')}
            danger
          >
            <p className='text-xs leading-relaxed break-all whitespace-pre-wrap'>
              {translateServerText(t, props.log.fail_reason)}
            </p>
          </DetailSection>
        ) : null}

        {taskData ? (
          <DetailSection
            icon={<Server className='size-3.5' aria-hidden='true' />}
            label={t('Upstream Response')}
          >
            <pre className='bg-background/60 max-h-72 min-w-0 overflow-auto rounded border p-2 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap'>
              {taskData}
            </pre>
          </DetailSection>
        ) : null}
      </div>
    </Dialog>
  )
}

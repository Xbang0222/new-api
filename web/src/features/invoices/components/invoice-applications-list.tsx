/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import Delete02Icon from '@hugeicons/core-free-icons/Delete02Icon'
import Invoice01Icon from '@hugeicons/core-free-icons/Invoice01Icon'
import RefreshIcon from '@hugeicons/core-free-icons/RefreshIcon'
import { HugeiconsIcon } from '@hugeicons/react'
import type {
  ColumnDef,
  ColumnFiltersState,
  OnChangeFn,
  PaginationState,
  Updater,
} from '@tanstack/react-table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { formatInvoiceDate, formatInvoiceMoney } from '../lib/format'
import {
  canCancelInvoiceApplication,
  canDeleteInvoiceApplication,
} from '../lib/invoice-delete-action'
import { getInvoiceEmailAction } from '../lib/invoice-email-action'
import type { InvoiceApplication, InvoiceStatus } from '../types'
import { InvoiceStatusBadge } from './invoice-status-badge'

type InvoiceApplicationsListProps = {
  applications: InvoiceApplication[]
  loading: boolean
  fetching: boolean
  page: number
  pageSize: number
  total: number
  keyword: string
  status?: InvoiceStatus
  isAdmin: boolean
  sendingId: number | null
  deletingId: number | null
  refreshing: boolean
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
  onKeywordChange: (keyword: string) => void
  onStatusChange: (status?: InvoiceStatus) => void
  onView: (application: InvoiceApplication) => void
  onSend: (application: InvoiceApplication) => void
  onDelete: (application: InvoiceApplication) => void
  onRefresh: () => void
}

function resolveUpdater<T>(updater: Updater<T>, current: T): T {
  return typeof updater === 'function'
    ? (updater as (value: T) => T)(current)
    : updater
}

export function InvoiceApplicationsList(props: InvoiceApplicationsListProps) {
  const { t } = useTranslation()
  const deletingId = props.deletingId
  const isAdmin = props.isAdmin
  const onDelete = props.onDelete
  const onSend = props.onSend
  const onView = props.onView
  const sendingId = props.sendingId
  const columns = useMemo<ColumnDef<InvoiceApplication>[]>(() => {
    const result: ColumnDef<InvoiceApplication>[] = [
      {
        accessorKey: 'id',
        header: t('ID'),
        cell: ({ row }) => (
          <span className='font-mono text-sm'>#{row.original.id}</span>
        ),
        size: 80,
        meta: { mobileOrder: 10 },
      },
      {
        accessorKey: 'invoice_title',
        header: t('Invoice title'),
        cell: ({ row }) => (
          <div className='flex min-w-44 flex-col gap-0.5'>
            <span className='truncate font-medium'>
              {row.original.invoice_title}
            </span>
            <span className='text-muted-foreground truncate text-xs'>
              {row.original.tax_number}
            </span>
          </div>
        ),
        size: 220,
        meta: { mobileTitle: true },
      },
      {
        id: 'orders',
        header: t('Paid orders'),
        cell: ({ row }) => (
          <div className='flex min-w-36 flex-col gap-0.5'>
            <span className='font-medium tabular-nums'>
              {formatInvoiceMoney(
                row.original.invoice_amount_cents,
                row.original.currency,
                row.original.currency_symbol
              )}
            </span>
            <span className='text-muted-foreground text-xs'>
              {t('{{count}} orders', { count: row.original.orders.length })}
            </span>
          </div>
        ),
        size: 160,
        meta: { mobileOrder: 30 },
      },
      {
        accessorKey: 'recipient_email',
        header: t('Recipient email'),
        cell: ({ row }) => (
          <span className='block max-w-56 truncate'>
            {row.original.recipient_email}
          </span>
        ),
        size: 220,
        meta: { mobileOrder: 40 },
      },
      {
        accessorKey: 'status',
        header: t('Status'),
        cell: ({ row }) => <InvoiceStatusBadge status={row.original.status} />,
        filterFn: (row, id, value: string[]) =>
          value.includes(String(row.getValue(id))),
        size: 140,
        meta: { mobileBadge: true },
      },
      {
        id: 'delivery',
        header: t('Email delivery'),
        cell: ({ row }) => {
          if (!row.original.invoice_email_sent_at) {
            return (
              <span className='text-muted-foreground'>{t('Not sent')}</span>
            )
          }
          return (
            <div className='flex min-w-36 flex-col gap-0.5'>
              <span>
                {formatInvoiceDate(row.original.invoice_email_sent_at)}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Sent {{count}} times', {
                  count: row.original.invoice_email_send_count,
                })}
              </span>
            </div>
          )
        },
        size: 180,
        meta: { mobileOrder: 50 },
      },
      {
        accessorKey: 'created_at',
        header: t('Submitted at'),
        cell: ({ row }) => (
          <span className='text-muted-foreground whitespace-nowrap'>
            {formatInvoiceDate(row.original.created_at)}
          </span>
        ),
        size: 180,
        meta: { mobileHidden: true },
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: ({ row }) => {
          const emailAction = getInvoiceEmailAction(row.original, isAdmin)
          const canDelete = isAdmin
            ? canDeleteInvoiceApplication(row.original, true)
            : canCancelInvoiceApplication(row.original)
          return (
            <div
              data-testid='invoice-row-actions'
              className='grid w-60 grid-cols-2 gap-2'
            >
              <Button
                className='w-full'
                size='sm'
                variant='outline'
                onClick={() => onView(row.original)}
              >
                {t('View details')}
              </Button>
              {emailAction ? (
                <Button
                  className='w-full'
                  size='sm'
                  disabled={sendingId === row.original.id}
                  onClick={() => onSend(row.original)}
                >
                  {emailAction === 'send'
                    ? t('Send invoice')
                    : t('Resend invoice')}
                </Button>
              ) : null}
              {canDelete ? (
                <Button
                  className='w-full'
                  size='sm'
                  variant='destructive'
                  disabled={deletingId === row.original.id}
                  onClick={() => onDelete(row.original)}
                >
                  <HugeiconsIcon icon={Delete02Icon} data-icon='inline-start' />
                  {isAdmin ? t('Delete') : t('Cancel application')}
                </Button>
              ) : null}
            </div>
          )
        },
        enableHiding: false,
        size: 240,
        meta: { pinned: 'right' as const },
      },
    ]

    if (isAdmin) {
      result.splice(2, 0, {
        accessorKey: 'user_id',
        header: t('User'),
        cell: ({ row }) => (
          <span className='font-mono text-sm'>{row.original.user_id}</span>
        ),
        size: 90,
        meta: { mobileOrder: 20 },
      })
    }
    return result
  }, [deletingId, isAdmin, onDelete, onSend, onView, sendingId, t])

  const pagination = useMemo<PaginationState>(
    () => ({ pageIndex: props.page - 1, pageSize: props.pageSize }),
    [props.page, props.pageSize]
  )
  const columnFilters = useMemo<ColumnFiltersState>(
    () => (props.status ? [{ id: 'status', value: [props.status] }] : []),
    [props.status]
  )
  const onPaginationChange: OnChangeFn<PaginationState> = (updater) => {
    const next = resolveUpdater(updater, pagination)
    if (next.pageSize !== props.pageSize) {
      props.onPageSizeChange(next.pageSize)
      props.onPageChange(1)
      return
    }
    props.onPageChange(next.pageIndex + 1)
  }
  const onColumnFiltersChange: OnChangeFn<ColumnFiltersState> = (updater) => {
    const next = resolveUpdater(updater, columnFilters)
    const statusValues = next.find((filter) => filter.id === 'status')
      ?.value as InvoiceStatus[] | undefined
    props.onStatusChange(statusValues?.[0])
    props.onPageChange(1)
  }
  const onGlobalFilterChange: OnChangeFn<string> = (updater) => {
    props.onKeywordChange(resolveUpdater(updater, props.keyword))
    props.onPageChange(1)
  }

  const { table } = useDataTable({
    data: props.applications,
    columns,
    pagination,
    columnFilters,
    globalFilter: props.keyword,
    onPaginationChange,
    onColumnFiltersChange,
    onGlobalFilterChange,
    manualFiltering: true,
    manualPagination: true,
    totalCount: props.total,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={props.loading}
      isFetching={props.fetching}
      emptyTitle={t('No invoice applications')}
      emptyDescription={t(
        'Invoice applications will appear here after they are submitted.'
      )}
      emptyIcon={<HugeiconsIcon icon={Invoice01Icon} />}
      skeletonKeyPrefix='invoice-applications-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t(
          'Search by ID, invoice title, tax number, or email...'
        ),
        searchClassName: 'sm:w-[360px] lg:w-[460px]',
        searchDebounceMs: 300,
        preActions: (
          <Button
            variant='outline'
            size='icon-sm'
            onClick={props.onRefresh}
            disabled={props.refreshing}
            title={t('Refresh')}
            aria-label={t('Refresh')}
          >
            <HugeiconsIcon
              icon={RefreshIcon}
              data-icon='inline-start'
              className={cn(props.refreshing && 'animate-spin')}
            />
          </Button>
        ),
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            singleSelect: true,
            options: [
              { value: 'pending_review', label: t('Review pending') },
              { value: 'pending_payment', label: t('Awaiting tax supplement') },
              { value: 'approved', label: t('Approved') },
              { value: 'issued', label: t('Issued') },
              { value: 'rejected', label: t('Rejected') },
            ],
          },
        ],
      }}
      pinnedColumns={[{ columnId: 'actions', side: 'right' }]}
    />
  )
}

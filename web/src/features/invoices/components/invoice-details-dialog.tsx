/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import Upload01Icon from '@hugeicons/core-free-icons/Upload01Icon'
import ViewIcon from '@hugeicons/core-free-icons/ViewIcon'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

import { formatInvoiceDate, formatInvoiceMoney } from '../lib/format'
import { getInvoiceEmailAction } from '../lib/invoice-email-action'
import type {
  InvoiceApplication,
  InvoiceOrder,
  ReviewInvoiceApplicationRequest,
} from '../types'
import { InvoiceStatusBadge } from './invoice-status-badge'

type InvoiceDetailsDialogProps = {
  application: InvoiceApplication | null
  isAdmin: boolean
  busy: boolean
  onOpenChange: (open: boolean) => void
  onReview: (request: ReviewInvoiceApplicationRequest) => void
  onCancel: () => void
  onUpload: (file: File) => void
  onViewFile: () => void
  onSend: () => void
}

type InvoiceFileActionProps = {
  busy: boolean
  onView: () => void
}

export function InvoiceFileAction(props: InvoiceFileActionProps) {
  const { t } = useTranslation()

  return (
    <Button variant='outline' disabled={props.busy} onClick={props.onView}>
      <HugeiconsIcon icon={ViewIcon} data-icon='inline-start' />
      {t('View document')}
    </Button>
  )
}

type InvoiceOrderListProps = {
  orders: InvoiceOrder[]
  currency: string
}

export function InvoiceOrderList(props: InvoiceOrderListProps) {
  const { t } = useTranslation()

  if (props.orders.length === 0) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('No eligible paid orders')}
      </p>
    )
  }

  return (
    <div className='divide-y border-y'>
      {props.orders.map((order) => (
        <div
          key={order.id || `${order.application_id}-${order.top_up_id}`}
          className='flex flex-col gap-2 p-3 sm:flex-row sm:items-center sm:justify-between'
        >
          <div className='min-w-0'>
            <div className='truncate font-mono text-xs' title={order.trade_no}>
              {order.trade_no || `#${order.top_up_id}`}
            </div>
            <div className='text-muted-foreground text-xs'>
              {formatInvoiceDate(order.completed_at)} {' / '}{' '}
              {order.payment_method || '-'}
            </div>
          </div>
          <span className='shrink-0 tabular-nums'>
            {formatInvoiceMoney(order.paid_amount_cents, props.currency)}
          </span>
        </div>
      ))}
    </div>
  )
}

export function InvoiceDetailsDialog(props: InvoiceDetailsDialogProps) {
  const { t } = useTranslation()
  const [rejectReason, setRejectReason] = useState('')
  const [note, setNote] = useState('')
  const [file, setFile] = useState<File | null>(null)

  useEffect(() => {
    const application = props.application
    if (!application) return
    setRejectReason('')
    setNote(application.admin_note || '')
    setFile(null)
  }, [props.application])

  const application = props.application
  if (!application) return null

  const canUpload =
    props.isAdmin &&
    (application.status === 'approved' || application.status === 'issued')
  const emailAction = getInvoiceEmailAction(application, props.isAdmin)
  const orders = application.orders ?? []

  return (
    <Dialog open onOpenChange={props.onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>
            {t('Invoice application details')} #{application.id}
          </DialogTitle>
          <DialogDescription>
            {application.invoice_title} ·{' '}
            {formatInvoiceDate(application.created_at)}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 md:grid-cols-2'>
          <section className='flex flex-col gap-3 rounded-lg border p-4'>
            <div className='flex items-center justify-between gap-3'>
              <h3 className='font-medium'>{t('Application information')}</h3>
              <InvoiceStatusBadge status={application.status} />
            </div>
            <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm'>
              <dt className='text-muted-foreground'>{t('User')}</dt>
              <dd>{application.user_id}</dd>
              <dt className='text-muted-foreground'>{t('Invoice title')}</dt>
              <dd>{application.invoice_title}</dd>
              <dt className='text-muted-foreground'>{t('Tax number')}</dt>
              <dd className='break-all'>{application.tax_number}</dd>
              <dt className='text-muted-foreground'>{t('Recipient email')}</dt>
              <dd className='break-all'>
                {application.recipient_email || '-'}
              </dd>
              <dt className='text-muted-foreground'>{t('Application note')}</dt>
              <dd className='whitespace-pre-wrap'>
                {application.applicant_note || '-'}
              </dd>
              <dt className='text-muted-foreground'>{t('Paid orders')}</dt>
              <dd>{orders.length}</dd>
            </dl>
          </section>

          <section className='flex flex-col gap-3 rounded-lg border p-4'>
            <h3 className='font-medium'>{t('Invoice information')}</h3>
            <dl className='grid grid-cols-[1fr_auto] gap-x-3 gap-y-2 text-sm'>
              <dt className='text-muted-foreground'>{t('Invoice item')}</dt>
              <dd>{application.invoice_item_name}</dd>
              <dt className='text-muted-foreground'>
                {t('Paid order amount')}
              </dt>
              <dd>
                {formatInvoiceMoney(
                  application.order_amount_cents,
                  application.currency
                )}
              </dd>
              <dt className='text-muted-foreground'>{t('Estimated VAT')}</dt>
              <dd>
                {formatInvoiceMoney(
                  application.estimated_vat_cents,
                  application.currency
                )}
              </dd>
              <dt className='text-muted-foreground'>
                {t('Urban maintenance and construction tax')}
              </dt>
              <dd>
                {formatInvoiceMoney(
                  application.estimated_urban_tax_cents,
                  application.currency
                )}
              </dd>
              <dt className='text-muted-foreground'>
                {t('Education surcharges')}
              </dt>
              <dd>
                {formatInvoiceMoney(
                  application.estimated_education_surcharge_cents +
                    application.estimated_local_education_surcharge_cents,
                  application.currency
                )}
              </dd>
              <dt className='text-muted-foreground'>
                {t('Estimated individual income tax withholding')}
              </dt>
              <dd>
                {formatInvoiceMoney(
                  application.estimated_pit_withholding_cents,
                  application.currency
                )}
              </dd>
              <dt className='font-medium'>{t('Estimated total tax')}</dt>
              <dd className='font-medium'>
                {formatInvoiceMoney(
                  application.estimated_total_tax_cents,
                  application.currency
                )}
              </dd>
              <dt className='font-medium'>{t('Suggested tax supplement')}</dt>
              <dd className='font-medium'>
                {formatInvoiceMoney(
                  application.suggested_supplement_cents,
                  application.currency
                )}
              </dd>
              <dt className='font-medium'>
                {t('Final tax supplement amount')}
              </dt>
              <dd className='font-medium'>
                {formatInvoiceMoney(
                  application.final_supplement_cents,
                  application.currency
                )}
              </dd>
              <dt className='font-medium'>{t('Final invoice amount')}</dt>
              <dd className='font-medium'>
                {formatInvoiceMoney(
                  application.invoice_amount_cents,
                  application.currency
                )}
              </dd>
            </dl>
          </section>
        </div>

        <section className='flex flex-col gap-3 rounded-lg border p-4'>
          <div className='flex items-center justify-between gap-3'>
            <h3 className='font-medium'>{t('Paid orders')}</h3>
            <span className='text-muted-foreground text-sm'>
              {orders.length}
            </span>
          </div>
          <InvoiceOrderList orders={orders} currency={application.currency} />
        </section>

        <Alert>
          <AlertTitle>
            {t('Tax estimate requires administrator verification')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'In customer-borne mode, all enabled estimated taxes are grossed up into the customer supplement. The final amount must follow the competent tax authority, withholding declaration, and tax payment certificate.'
            )}
          </AlertDescription>
        </Alert>

        {props.isAdmin && application.status === 'pending_review' ? (
          <section className='flex flex-col gap-4 rounded-lg border p-4'>
            <h3 className='font-medium'>{t('Review invoice application')}</h3>
            <div className='grid gap-4'>
              <Field>
                <FieldLabel htmlFor='invoice-admin-note'>
                  {t('Admin note')}
                </FieldLabel>
                <Textarea
                  id='invoice-admin-note'
                  value={note}
                  maxLength={2000}
                  onChange={(event) => setNote(event.target.value)}
                />
              </Field>
            </div>
            <Field>
              <FieldLabel htmlFor='invoice-reject-reason'>
                {t('Rejection reason')}
              </FieldLabel>
              <Textarea
                id='invoice-reject-reason'
                value={rejectReason}
                maxLength={2000}
                placeholder={t('Required only when rejecting the application.')}
                onChange={(event) => setRejectReason(event.target.value)}
              />
            </Field>
            <div className='flex flex-wrap justify-end gap-2'>
              <Button
                variant='destructive'
                disabled={props.busy || rejectReason.trim() === ''}
                onClick={() =>
                  props.onReview({
                    action: 'reject',
                    tax_adjustment_reason: '',
                    reason: rejectReason,
                    note,
                  })
                }
              >
                {t('Reject')}
              </Button>
              <Button
                disabled={props.busy}
                onClick={() =>
                  props.onReview({
                    action: 'approve',
                    tax_adjustment_reason: '',
                    reason: '',
                    note,
                  })
                }
              >
                {t('Approve')}
              </Button>
            </div>
          </section>
        ) : null}

        {!props.isAdmin && application.status === 'pending_review' ? (
          <div className='flex justify-end'>
            <Button
              variant='destructive'
              disabled={props.busy}
              onClick={props.onCancel}
            >
              {t('Withdraw application')}
            </Button>
          </div>
        ) : null}

        {canUpload ? (
          <section className='flex flex-col gap-3 rounded-lg border p-4'>
            <h3 className='font-medium'>
              {application.status === 'issued'
                ? t('Replace and send invoice')
                : t('Upload and send invoice')}
            </h3>
            <Field>
              <FieldLabel htmlFor='invoice-file'>
                {t('Invoice file')}
              </FieldLabel>
              <Input
                id='invoice-file'
                type='file'
                accept='application/pdf,image/png,image/jpeg'
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
              <FieldDescription>
                {t('PDF, PNG, or JPEG. Maximum file size: 20 MB.')}
              </FieldDescription>
            </Field>
            <div className='flex justify-end'>
              <Button
                disabled={props.busy || !file}
                onClick={() => file && props.onUpload(file)}
              >
                <HugeiconsIcon icon={Upload01Icon} data-icon='inline-start' />
                {t('Upload and send invoice')}
              </Button>
            </div>
          </section>
        ) : null}

        {application.invoice_file_name ? (
          <section className='flex flex-col gap-3 rounded-lg border p-4'>
            <h3 className='font-medium'>{t('Email delivery')}</h3>
            <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm'>
              <dt className='text-muted-foreground'>{t('Invoice file')}</dt>
              <dd className='break-all'>{application.invoice_file_name}</dd>
              <dt className='text-muted-foreground'>{t('Last sent at')}</dt>
              <dd>
                {application.invoice_email_sent_at
                  ? formatInvoiceDate(application.invoice_email_sent_at)
                  : t('Not sent')}
              </dd>
              <dt className='text-muted-foreground'>{t('Send count')}</dt>
              <dd>{application.invoice_email_send_count}</dd>
            </dl>
            <div className='flex flex-wrap gap-2'>
              <InvoiceFileAction busy={props.busy} onView={props.onViewFile} />
              {emailAction ? (
                <Button disabled={props.busy} onClick={props.onSend}>
                  {emailAction === 'send'
                    ? t('Send invoice')
                    : t('Resend invoice')}
                </Button>
              ) : null}
            </div>
          </section>
        ) : null}

        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

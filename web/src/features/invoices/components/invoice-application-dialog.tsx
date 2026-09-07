/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { zodResolver } from '@hookform/resolvers/zod'
import AddInvoiceIcon from '@hugeicons/core-free-icons/AddInvoiceIcon'
import { HugeiconsIcon } from '@hugeicons/react'
import { type ReactNode, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'

import { formatInvoiceDate, formatInvoiceMoney } from '../lib/format'
import {
  canSubmitInvoiceApplication,
  getInvoiceOrderSelectionState,
} from '../lib/invoice-application-availability'
import {
  createInvoiceApplicationSchema,
  invoiceApplicationFormDefaults,
  type InvoiceApplicationFormValues,
} from '../lib/invoice-application-form'
import type { EligibleInvoiceOrder, InvoiceConfig } from '../types'

type InvoiceApplicationDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  config?: InvoiceConfig
  orders: EligibleInvoiceOrder[]
  configLoading: boolean
  configError: boolean
  ordersLoading: boolean
  ordersError: boolean
  submitting: boolean
  userQuota?: number
  balanceError?: boolean
  submissionError?: string
  onRetry: () => void
  onSubmit: (values: InvoiceApplicationFormValues) => Promise<void>
}

export function InvoiceApplicationDialog(props: InvoiceApplicationDialogProps) {
  const { t } = useTranslation()
  const schema = useMemo(() => createInvoiceApplicationSchema(t), [t])
  const form = useForm<InvoiceApplicationFormValues>({
    resolver: zodResolver(schema),
    defaultValues: invoiceApplicationFormDefaults,
  })
  const selectedIds = form.watch('top_up_ids')
  const currency = props.config?.currency ?? 'USD'
  const currencySymbol = props.config?.currency_symbol
  const invoiceOrders = useMemo(
    () =>
      props.orders.filter(
        (order) =>
          Number.isSafeInteger(order.invoice_amount_cents) &&
          order.invoice_amount_cents > 0 &&
          order.currency === currency &&
          order.currency_symbol === currencySymbol
      ),
    [props.orders, currency, currencySymbol]
  )
  const selectedTotal = invoiceOrders.reduce((total, order) => {
    return selectedIds.includes(order.id)
      ? total + order.invoice_amount_cents
      : total
  }, 0)
  const hasUnavailableSelection = selectedIds.some(
    (id) => !invoiceOrders.some((order) => order.id === id)
  )
  const feeCents = Math.round(
    (selectedTotal * (props.config?.fee_rate_basis_points ?? 0)) / 10_000
  )
  const quotaPerUnit = props.config?.quota_per_currency_unit ?? 0
  const estimatedQuota = (feeCents * quotaPerUnit) / 100
  // Ignore floating-point noise at integer boundaries without hiding a missing
  // quota unit. The server calculates and charges the exact decimal amount.
  const roundingTolerance = Math.min(0.25, estimatedQuota * Number.EPSILON * 2)
  const requiredQuota = Math.ceil(estimatedQuota - roundingTolerance)
  const insufficientQuota =
    props.userQuota !== undefined && props.userQuota < requiredQuota
  const belowMinimum =
    selectedTotal < Math.round((props.config?.minimum_amount ?? 0) * 100)

  const handleOpenChange = (open: boolean) => {
    props.onOpenChange(open)
    if (!open) form.reset(invoiceApplicationFormDefaults)
  }

  const toggleOrder = (orderId: number, checked: boolean) => {
    const current = form.getValues('top_up_ids')
    const next = checked
      ? [...current, orderId]
      : current.filter((id) => id !== orderId)
    form.setValue('top_up_ids', next, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const submit = form.handleSubmit(async (values) => {
    try {
      await props.onSubmit(values)
      form.reset(invoiceApplicationFormDefaults)
    } catch {
      // The parent displays the server failure; preserve the entered application.
    }
  })
  const availability = {
    configEnabled: props.config?.enabled,
    configLoading: props.configLoading,
    configError: props.configError,
    ordersLoading: props.ordersLoading,
    ordersError: props.ordersError,
    orderCount: invoiceOrders.length,
  }
  const selectionState = getInvoiceOrderSelectionState(availability)
  const submitDisabled =
    hasUnavailableSelection ||
    insufficientQuota ||
    belowMinimum ||
    (feeCents > 0 && (props.userQuota === undefined || quotaPerUnit <= 0)) ||
    !canSubmitInvoiceApplication(
      availability,
      selectedIds.length,
      props.submitting
    )

  let orderSelectionContent: ReactNode
  if (selectionState === 'config_loading') {
    orderSelectionContent = (
      <div className='text-muted-foreground flex items-center gap-2 rounded-lg border p-4 text-sm'>
        <Spinner />
        {t('Loading invoice application settings...')}
      </div>
    )
  } else if (selectionState === 'config_error') {
    orderSelectionContent = (
      <Alert variant='destructive'>
        <AlertTitle>
          {t('Failed to load invoice application settings.')}
        </AlertTitle>
        <AlertDescription>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='mt-2'
            onClick={props.onRetry}
          >
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  } else if (selectionState === 'disabled') {
    orderSelectionContent = (
      <Alert>
        <AlertTitle>{t('Invoice applications are unavailable')}</AlertTitle>
        <AlertDescription>
          {t('The administrator has not enabled invoice applications.')}
        </AlertDescription>
      </Alert>
    )
  } else if (selectionState === 'orders_loading') {
    orderSelectionContent = (
      <div className='text-muted-foreground flex items-center gap-2 rounded-lg border p-4 text-sm'>
        <Spinner />
        {t('Loading eligible paid orders...')}
      </div>
    )
  } else if (selectionState === 'orders_error') {
    orderSelectionContent = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Failed to load eligible paid orders.')}</AlertTitle>
        <AlertDescription>
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='mt-2'
            onClick={props.onRetry}
          >
            {t('Retry')}
          </Button>
        </AlertDescription>
      </Alert>
    )
  } else if (selectionState === 'empty') {
    orderSelectionContent = (
      <Alert>
        <AlertTitle>{t('No eligible paid orders')}</AlertTitle>
        <AlertDescription>
          {t('There are no paid orders currently available for invoicing.')}
        </AlertDescription>
      </Alert>
    )
  } else {
    orderSelectionContent = (
      <div className='max-h-[22rem] overflow-y-auto rounded-lg border'>
        {invoiceOrders.map((order) => {
          const checked = selectedIds.includes(order.id)
          return (
            <Field
              key={order.id}
              orientation='horizontal'
              className='border-b p-3 last:border-b-0'
            >
              <Checkbox
                id={`invoice-order-${order.id}`}
                checked={checked}
                onCheckedChange={(value) =>
                  toggleOrder(order.id, value === true)
                }
              />
              <FieldLabel htmlFor={`invoice-order-${order.id}`}>
                <span className='flex min-w-0 flex-1 flex-col gap-0.5'>
                  <span className='truncate font-mono text-xs'>
                    {order.trade_no}
                  </span>
                  <span className='text-muted-foreground text-xs'>
                    {formatInvoiceDate(
                      order.complete_time || order.create_time
                    )}{' '}
                    · {order.payment_method}
                  </span>
                </span>
                <span className='tabular-nums'>
                  {formatInvoiceMoney(
                    order.invoice_amount_cents,
                    order.currency,
                    order.currency_symbol
                  )}
                </span>
              </FieldLabel>
            </Field>
          )
        })}
      </div>
    )
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('Apply for an invoice')}</DialogTitle>
          <DialogDescription>
            {t(
              'Select paid orders and provide the invoice and delivery information.'
            )}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit}>
          <div className='grid gap-6 md:grid-cols-2'>
            <FieldSet className='min-w-0'>
              <FieldLegend>{t('Paid orders')}</FieldLegend>
              <FieldDescription>
                {t(
                  'Only paid orders with a verified amount and currency are available.'
                )}
              </FieldDescription>
              <FieldDescription>
                {t('Selected amount')}:{' '}
                <span className='text-foreground font-medium tabular-nums'>
                  {formatInvoiceMoney(selectedTotal, currency, currencySymbol)}
                </span>
              </FieldDescription>
              {orderSelectionContent}
              <FieldError errors={[form.formState.errors.top_up_ids]} />
            </FieldSet>

            <FieldSet className='min-w-0'>
              <FieldLegend>{t('Invoice information')}</FieldLegend>
              <FieldGroup>
                <Field
                  data-invalid={Boolean(form.formState.errors.invoice_title)}
                >
                  <FieldLabel htmlFor='invoice-title'>
                    {t('Invoice title')}
                  </FieldLabel>
                  <Input
                    id='invoice-title'
                    autoComplete='organization'
                    aria-invalid={Boolean(form.formState.errors.invoice_title)}
                    {...form.register('invoice_title')}
                  />
                  <FieldError errors={[form.formState.errors.invoice_title]} />
                </Field>
                <Field data-invalid={Boolean(form.formState.errors.tax_number)}>
                  <FieldLabel htmlFor='invoice-tax-number'>
                    {t('Tax identification number')}
                  </FieldLabel>
                  <Input
                    id='invoice-tax-number'
                    aria-invalid={Boolean(form.formState.errors.tax_number)}
                    {...form.register('tax_number')}
                  />
                  <FieldError errors={[form.formState.errors.tax_number]} />
                </Field>
                <Field
                  data-invalid={Boolean(form.formState.errors.recipient_email)}
                >
                  <FieldLabel htmlFor='invoice-recipient-email'>
                    {t('Recipient email')}
                  </FieldLabel>
                  <Input
                    id='invoice-recipient-email'
                    type='email'
                    autoComplete='email'
                    aria-invalid={Boolean(
                      form.formState.errors.recipient_email
                    )}
                    {...form.register('recipient_email')}
                  />
                  <FieldDescription>
                    {t('The invoice will be sent to this email address.')}
                  </FieldDescription>
                  <FieldError
                    errors={[form.formState.errors.recipient_email]}
                  />
                </Field>
                <Field
                  data-invalid={Boolean(form.formState.errors.applicant_note)}
                >
                  <FieldLabel htmlFor='invoice-applicant-note'>
                    {t('Application note')}
                  </FieldLabel>
                  <Textarea
                    id='invoice-applicant-note'
                    maxLength={2000}
                    rows={4}
                    aria-invalid={Boolean(form.formState.errors.applicant_note)}
                    {...form.register('applicant_note')}
                  />
                  <FieldError errors={[form.formState.errors.applicant_note]} />
                </Field>
              </FieldGroup>
            </FieldSet>
          </div>

          {props.config ? (
            <Alert className='mt-6'>
              <AlertTitle>{t('Invoice fee')}</AlertTitle>
              <AlertDescription>
                <div className='flex flex-col gap-2'>
                  <dl className='grid grid-cols-[1fr_auto] gap-x-4 gap-y-2'>
                    <dt>{t('Invoice item')}</dt>
                    <dd>{props.config.invoice_item_name}</dd>
                    <dt>{t('Final invoice amount')}</dt>
                    <dd>
                      {formatInvoiceMoney(
                        selectedTotal,
                        currency,
                        currencySymbol
                      )}
                    </dd>
                    <dt>{t('Invoice fee rate (%)')}</dt>
                    <dd>{props.config.fee_rate_basis_points / 100}%</dd>
                    <dt>{t('Fee deducted on submission')}</dt>
                    <dd>
                      {formatInvoiceMoney(feeCents, currency, currencySymbol)}
                    </dd>
                    <dt>{t('Available balance')}</dt>
                    <dd>
                      {props.userQuota !== undefined && quotaPerUnit > 0
                        ? formatInvoiceMoney(
                            (props.userQuota * 100) / quotaPerUnit,
                            currency,
                            currencySymbol
                          )
                        : t('Unavailable')}
                    </dd>
                  </dl>
                  <p>
                    {t('Minimum invoice amount')}:{' '}
                    {formatInvoiceMoney(
                      Math.round(props.config.minimum_amount * 100),
                      currency,
                      currencySymbol
                    )}
                  </p>
                  <p>
                    {t(
                      'The fee is deducted from your balance when you apply and refunded if the application is rejected or cancelled.'
                    )}
                  </p>
                </div>
              </AlertDescription>
            </Alert>
          ) : null}

          {insufficientQuota || props.submissionError || props.balanceError ? (
            <Alert variant='destructive' className='mt-3'>
              <AlertDescription>
                {insufficientQuota
                  ? t('Insufficient quota')
                  : props.submissionError ||
                    t('Failed to load balance. Refresh and try again.')}
              </AlertDescription>
            </Alert>
          ) : null}

          <DialogFooter className='mt-6'>
            <Button
              type='button'
              variant='outline'
              onClick={() => handleOpenChange(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit' disabled={submitDisabled}>
              {props.submitting ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <HugeiconsIcon icon={AddInvoiceIcon} data-icon='inline-start' />
              )}
              {t('Submit application')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

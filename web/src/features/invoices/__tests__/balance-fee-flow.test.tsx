/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { InvoiceApplicationDialog } from '../components/invoice-application-dialog'
import { InvoiceDetailsDialog } from '../components/invoice-details-dialog'
import { canCancelInvoiceApplication } from '../lib/invoice-delete-action'
import { application, config } from './fixtures'

function renderApplicationDialog(
  userQuota = 1000,
  settings = config,
  invoiceAmountCents = 5000
) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  render(
    <InvoiceApplicationDialog
      open
      onOpenChange={vi.fn()}
      config={settings}
      orders={[
        {
          id: 1,
          money: 50,
          invoice_amount_cents: invoiceAmountCents,
          currency: settings.currency,
          currency_symbol: settings.currency_symbol,
          trade_no: 'paid-order-1',
          payment_method: 'balance',
          create_time: 1_800_000_000,
          complete_time: 1_800_000_000,
          status: 'success',
        },
      ]}
      configLoading={false}
      configError={false}
      ordersLoading={false}
      ordersError={false}
      submitting={false}
      onRetry={vi.fn()}
      onSubmit={onSubmit}
      userQuota={userQuota}
    />
  )
  return onSubmit
}

describe('invoice fees charged on application', () => {
  it('shows a 1.50 balance fee for a 50.00 invoice at 3 percent without increasing the invoice amount', () => {
    renderApplicationDialog()
    fireEvent.click(screen.getByRole('checkbox'))

    expect(screen.getByText('Invoice fee')).toBeInTheDocument()
    expect(screen.getByText(/1\.50/)).toBeInTheDocument()
    expect(screen.queryByText('VAT threshold')).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeEnabled()
  })

  it('shows insufficient quota and prevents an application when balance cannot cover the fee', () => {
    renderApplicationDialog(149)
    fireEvent.click(screen.getByRole('checkbox'))

    expect(screen.getByText('Insufficient quota')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeDisabled()
  })

  it('uses a verified 375 CNY invoice amount for a raw 50 USD payment and charges 11.25 CNY at 3 percent', () => {
    renderApplicationDialog(
      750_000,
      { ...config, quota_per_currency_unit: 500_000 / 7.5 },
      37_500
    )
    const order = screen.getByRole('checkbox', { name: /375\.00/ })
    fireEvent.click(order)

    expect(screen.getAllByText(/11\.25/)).toHaveLength(2)
    expect(screen.queryByText(/50\.00/)).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeEnabled()
  })

  it('does not offer an order without a verified invoice amount or fall back to its raw payment amount', () => {
    renderApplicationDialog(1000, config, Number.NaN)
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.getByText('No eligible paid orders')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeDisabled()
  })

  it('allows submission when the balance exactly covers the fee', () => {
    renderApplicationDialog(150)
    fireEvent.click(screen.getByRole('checkbox'))
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeEnabled()
  })

  it.each([
    [300, 100_000],
    [30, 10_000],
    [4, 1_334],
  ])(
    'allows an exact %i basis-point CNY fee balance of %i quota at the 7.5 exchange rate',
    (feeRate, exactQuota) => {
      renderApplicationDialog(exactQuota, {
        ...config,
        fee_rate_basis_points: feeRate,
        quota_per_currency_unit: 500_000 / 7.5,
      })
      fireEvent.click(screen.getByRole('checkbox'))

      expect(
        screen.getByRole('button', { name: 'Submit application' })
      ).toBeEnabled()
      expect(screen.queryByText('Insufficient quota')).not.toBeInTheDocument()
    }
  )

  it.each([
    [300, 99_999],
    [30, 9_999],
    [4, 1_333],
  ])(
    'rejects a %i basis-point CNY fee balance of %i quota that is one unit short',
    (feeRate, insufficientBalance) => {
      renderApplicationDialog(insufficientBalance, {
        ...config,
        fee_rate_basis_points: feeRate,
        quota_per_currency_unit: 500_000 / 7.5,
      })
      fireEvent.click(screen.getByRole('checkbox'))

      expect(
        screen.getByRole('button', { name: 'Submit application' })
      ).toBeDisabled()
      expect(screen.getByText('Insufficient quota')).toBeInTheDocument()
    }
  )

  it('allows a zero-fee application with a zero balance', () => {
    renderApplicationDialog(0, { ...config, fee_rate_basis_points: 0 })
    fireEvent.click(screen.getByRole('checkbox'))
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeEnabled()
    expect(screen.queryByText('Insufficient quota')).not.toBeInTheDocument()
  })

  it('blocks submission below the configured minimum invoice amount', () => {
    renderApplicationDialog(1000, { ...config, minimum_amount: 51 })
    fireEvent.click(screen.getByRole('checkbox'))
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeDisabled()
  })

  it('shows the configured custom currency symbol for the fee and selected order amount', () => {
    renderApplicationDialog(1000, {
      ...config,
      currency: 'CUSTOM',
      currency_symbol: '€',
    })
    fireEvent.click(screen.getByRole('checkbox'))
    expect(screen.getByText('€1.50')).toBeInTheDocument()
    expect(screen.getAllByText('€50.00')).toHaveLength(3)
    expect(screen.queryByText(/CN¥/)).not.toBeInTheDocument()
  })

  it('reviews a balance-funded invoice without editable amounts or supplement payment actions', async () => {
    const onReview = vi.fn()
    render(
      <InvoiceDetailsDialog
        application={application}
        isAdmin
        busy={false}
        onOpenChange={vi.fn()}
        onReview={onReview}
        onUpload={vi.fn()}
        onViewFile={vi.fn()}
        onSend={vi.fn()}
      />
    )

    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
    expect(screen.queryByText('Tax adjustment reason')).not.toBeInTheDocument()
    expect(screen.queryByText('Estimated VAT')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Approve' }))
    await waitFor(() =>
      expect(onReview).toHaveBeenCalledWith({
        action: 'approve',
        reason: '',
        note: '',
      })
    )
  })

  it('keeps legacy invoice details readable without offering a separate payment checkout', () => {
    render(
      <InvoiceDetailsDialog
        application={{
          ...application,
          fee_mode: '',
          status: 'pending_payment',
        }}
        isAdmin={false}
        busy={false}
        onOpenChange={vi.fn()}
        onReview={vi.fn()}
        onUpload={vi.fn()}
        onViewFile={vi.fn()}
        onSend={vi.fn()}
      />
    )

    expect(screen.getByText('Estimated VAT')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /Pay now|Deduct from balance/ })
    ).not.toBeInTheDocument()
  })

  it('shows a rejected application fee as refunded to the balance', () => {
    render(
      <InvoiceDetailsDialog
        application={{
          ...application,
          status: 'rejected',
          payment_status: 'refunded',
          refunded_quota: 150,
        }}
        isAdmin={false}
        busy={false}
        onOpenChange={vi.fn()}
        onReview={vi.fn()}
        onUpload={vi.fn()}
        onViewFile={vi.fn()}
        onSend={vi.fn()}
      />
    )
    expect(screen.getByText('Refunded to balance')).toBeInTheDocument()
    expect(screen.queryByText('Deducted from balance')).not.toBeInTheDocument()
  })

  it('shows a confirmed zero legacy supplement without replacing it with the historical estimate', () => {
    render(
      <InvoiceDetailsDialog
        application={{
          ...application,
          fee_mode: '',
          status: 'approved',
          final_supplement_cents: 0,
          suggested_supplement_cents: 150,
        }}
        isAdmin={false}
        busy={false}
        onOpenChange={vi.fn()}
        onReview={vi.fn()}
        onUpload={vi.fn()}
        onViewFile={vi.fn()}
        onSend={vi.fn()}
      />
    )
    expect(screen.getAllByText(/1\.50/)).toHaveLength(1)
    expect(screen.getByText('Historical tax estimate')).toBeInTheDocument()
  })
})

describe('invoice cancellation availability', () => {
  it.each(['pending_review', 'approved', 'rejected'] as const)(
    'allows cancelling a %s balance-funded application before an invoice is uploaded',
    (status) => {
      expect(canCancelInvoiceApplication({ ...application, status })).toBe(true)
    }
  )

  it.each([
    { status: 'issued' as const },
    { invoice_file_name: 'invoice.pdf' },
    { issued_at: 1_800_000_000 },
    { invoice_email_sent_at: 1_800_000_000 },
    { can_cancel: false },
  ])(
    'protects an issued, uploaded or server-blocked invoice from cancellation: %j',
    (changes) => {
      expect(canCancelInvoiceApplication({ ...application, ...changes })).toBe(
        false
      )
    }
  )

  it('does not offer cancellation for a legacy invoice with an unresolved payment attempt', () => {
    expect(
      canCancelInvoiceApplication({
        ...application,
        fee_mode: '',
        status: 'pending_payment',
        payment_status: 'pending',
        payment_confirmed_at: 0,
        payment_trade_no: 'epay-pending',
      })
    ).toBe(false)
  })
})

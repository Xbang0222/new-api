/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { InvoiceApplicationDialog } from '../components/invoice-application-dialog'
import { config } from './fixtures'

function renderApplicationDialog(orderCount: number) {
  const onSubmit = vi.fn().mockResolvedValue(undefined)
  render(
    <InvoiceApplicationDialog
      open
      onOpenChange={vi.fn()}
      config={config}
      orders={Array.from({ length: orderCount }, (_, index) => ({
        id: index + 1,
        money: 50,
        invoice_amount_cents: 5000,
        currency: config.currency,
        currency_symbol: config.currency_symbol,
        trade_no: `paid-order-${index + 1}`,
        payment_method: 'wxpay',
        create_time: 1_800_000_000,
        complete_time: 1_800_000_000,
        status: 'success',
      }))}
      configLoading={false}
      configError={false}
      ordersLoading={false}
      ordersError={false}
      submitting={false}
      onRetry={vi.fn()}
      onSubmit={onSubmit}
      userQuota={10_000_000}
    />
  )
  return onSubmit
}

function fillInvoiceInformation() {
  fireEvent.change(screen.getByLabelText('Invoice title'), {
    target: { value: 'Example Technology Co., Ltd.' },
  })
  fireEvent.change(screen.getByLabelText('Tax identification number'), {
    target: { value: '91310000EXAMPLE' },
  })
  fireEvent.change(screen.getByLabelText('Recipient email'), {
    target: { value: 'finance@example.com' },
  })
}

function orderRow(text: string) {
  const row = screen.getByText(text).closest<HTMLElement>('[role="group"]')
  if (!row) throw new Error(`No list row contains "${text}"`)
  return within(row)
}

describe('invoice paid order selection', () => {
  it('submits every listed order when Select all is clicked', async () => {
    const onSubmit = renderApplicationDialog(3)

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }))
    fillInvoiceInformation()
    fireEvent.click(screen.getByRole('button', { name: 'Submit application' }))

    expect(screen.getByText('Selected 3 / 3')).toBeInTheDocument()
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0].top_up_ids).toEqual([1, 2, 3])
  })

  it('shows the mixed state when only some orders are selected', () => {
    renderApplicationDialog(3)

    fireEvent.click(screen.getByRole('checkbox', { name: /paid-order-2/ }))

    expect(
      screen.getByRole('checkbox', { name: 'Select all' })
    ).toHaveAttribute('aria-checked', 'mixed')
    expect(screen.getByText('Selected 1 / 3')).toBeInTheDocument()
  })

  it('clears the selection when Select all is clicked while every order is selected', () => {
    renderApplicationDialog(3)
    const selectAll = screen.getByRole('checkbox', { name: 'Select all' })

    fireEvent.click(selectAll)
    fireEvent.click(selectAll)

    expect(selectAll).toHaveAttribute('aria-checked', 'false')
    expect(screen.getByText('Selected 0 / 3')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Submit application' })
    ).toBeDisabled()
  })

  it('selects only the first 300 orders and states the limit when 301 orders are listed', () => {
    renderApplicationDialog(301)

    // Role and label queries over the whole dialog compute the accessible name
    // of every row and take seconds on a list of this size, so each control is
    // looked up inside its own row.
    fireEvent.click(orderRow('Select all').getByRole('checkbox'))

    expect(screen.getByText('Selected 300 / 301')).toBeInTheDocument()
    expect(
      screen.getByText('You can select up to 300 paid orders.')
    ).toBeInTheDocument()
    expect(orderRow('paid-order-300').getByRole('checkbox')).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(orderRow('paid-order-301').getByRole('checkbox')).toHaveAttribute(
      'aria-checked',
      'false'
    )
  })
})

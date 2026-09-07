/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { Invoices } from '../index'
import { application, config } from './fixtures'

let queryClient: QueryClient

beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
})

afterEach(() => queryClient.clear())

async function renderInvoices(
  options: {
    withApplication?: boolean
    createFails?: boolean
    cancelFails?: boolean
  } = {}
) {
  let activeApplication = options.withApplication === true
  let quota = activeApplication ? 850 : 1000
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/invoice/config') {
      return { data: { success: true, data: config } }
    }
    if (url === '/api/user/self') {
      return { data: { success: true, data: { quota } } }
    }
    if (url === '/api/invoice/eligible-orders') {
      return {
        data: {
          success: true,
          data: activeApplication
            ? []
            : [
                {
                  id: 1,
                  money: 50,
                  invoice_amount_cents: 5000,
                  currency: 'CNY',
                  currency_symbol: '¥',
                  trade_no: 'paid-order-1',
                  payment_method: 'balance',
                  create_time: 1_800_000_000,
                  complete_time: 1_800_000_000,
                  status: 'success',
                },
              ],
        },
      }
    }
    if (url === '/api/invoice/self') {
      return {
        data: {
          success: true,
          data: {
            page: 1,
            page_size: 20,
            total: activeApplication ? 1 : 0,
            items: activeApplication
              ? [{ ...application, can_cancel: true }]
              : [],
          },
        },
      }
    }
    throw new Error(`Unexpected GET ${url}`)
  })
  const post = vi.spyOn(api, 'post').mockImplementation(async (url) => {
    if (url === '/api/invoice/applications') {
      if (options.createFails) {
        return { data: { success: false, message: '额度不足' } }
      }
      activeApplication = true
      quota = 850
      return { data: { success: true, data: application } }
    }
    if (url === '/api/invoice/applications/42/cancel') {
      if (options.cancelFails) {
        return { data: { success: false, message: 'Cancellation failed' } }
      }
      activeApplication = false
      quota = 1000
      return { data: { success: true, data: null } }
    }
    throw new Error(`Unexpected POST ${url}`)
  })
  await act(async () => {
    render(
      <QueryClientProvider client={queryClient}>
        <Invoices />
      </QueryClientProvider>
    )
  })
  await screen.findByRole('button', { name: 'Apply for an invoice' })
  return { get, post }
}

async function submitApplication() {
  fireEvent.click(screen.getByRole('button', { name: 'Apply for an invoice' }))
  const dialog = await screen.findByRole('dialog')
  fireEvent.click(within(dialog).getByRole('checkbox'))
  fireEvent.change(
    within(dialog).getByRole('textbox', { name: 'Invoice title' }),
    { target: { value: 'Example Company' } }
  )
  fireEvent.change(
    within(dialog).getByRole('textbox', { name: 'Tax identification number' }),
    { target: { value: '91310000TEST' } }
  )
  fireEvent.change(
    within(dialog).getByRole('textbox', { name: 'Recipient email' }),
    { target: { value: 'finance@example.com' } }
  )
  fireEvent.click(
    within(dialog).getByRole('button', { name: 'Submit application' })
  )
  return dialog
}

describe('invoice application actions', () => {
  it('submits only the selected paid order, refreshes the balance, and never starts a payment checkout', async () => {
    const { get, post } = await renderInvoices()
    await submitApplication()

    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    )
    expect(post).toHaveBeenCalledWith(
      '/api/invoice/applications',
      {
        top_up_ids: [1],
        invoice_title: 'Example Company',
        tax_number: '91310000TEST',
        recipient_email: 'finance@example.com',
        applicant_note: '',
      },
      { skipBusinessError: true }
    )
    expect(
      await screen.findByText(/Available balance:.*8\.50/)
    ).toBeInTheDocument()
    expect(get.mock.calls.map(([url]) => url)).not.toContain(
      '/api/invoice/payment-methods'
    )
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('keeps the entered application open and displays insufficient quota when the server rejects a stale balance', async () => {
    await renderInvoices({ createFails: true })
    const dialog = await submitApplication()

    expect(
      await within(dialog).findByText('Insufficient quota')
    ).toBeInTheDocument()
    expect(
      within(dialog).getByRole('textbox', { name: 'Invoice title' })
    ).toHaveValue('Example Company')
    expect(within(dialog).getByRole('checkbox')).toBeChecked()
  })

  it('removes a cancelled application and reloads the refunded balance and eligible orders', async () => {
    const { post } = await renderInvoices({ withApplication: true })
    fireEvent.click(
      await screen.findByRole('button', { name: 'Cancel application' })
    )
    const confirmation = await screen.findByRole('alertdialog')
    fireEvent.click(
      within(confirmation).getByRole('button', { name: 'Cancel application' })
    )

    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(post).toHaveBeenCalledWith(
      '/api/invoice/applications/42/cancel',
      undefined,
      { skipBusinessError: true }
    )
    expect(screen.queryByText('Example Company')).not.toBeInTheDocument()
    expect(
      await screen.findByText(/Available balance:.*10\.00/)
    ).toBeInTheDocument()
    fireEvent.click(
      screen.getByRole('button', { name: 'Apply for an invoice' })
    )
    expect(await screen.findByRole('checkbox')).toBeEnabled()
  })

  it('keeps the application and balance unchanged when cancellation fails', async () => {
    const { post } = await renderInvoices({
      withApplication: true,
      cancelFails: true,
    })
    fireEvent.click(
      await screen.findByRole('button', { name: 'Cancel application' })
    )
    const confirmation = await screen.findByRole('alertdialog')
    fireEvent.click(
      within(confirmation).getByRole('button', { name: 'Cancel application' })
    )

    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    await waitFor(() =>
      expect(
        within(confirmation).getByRole('button', { name: 'Cancel application' })
      ).toBeEnabled()
    )
    fireEvent.click(
      within(confirmation).getByRole('button', { name: 'Cancel' })
    )
    expect(await screen.findByText('Example Company')).toBeInTheDocument()
    expect(screen.getByText(/Available balance:.*8\.50/)).toBeInTheDocument()
  })
})

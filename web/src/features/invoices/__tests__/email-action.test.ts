/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import { getInvoiceEmailAction } from '../lib/invoice-email-action'
import type { InvoiceApplication } from '../types'

const application = {
  status: 'approved',
  invoice_file_name: 'invoice.pdf',
  invoice_email_send_count: 0,
} as InvoiceApplication

describe('invoice email actions', () => {
  test('allows an administrator to send an uploaded approved invoice', () => {
    assert.equal(getInvoiceEmailAction(application, true), 'send')
  })

  test('does not allow a user to send an invoice before it is issued', () => {
    assert.equal(getInvoiceEmailAction(application, false), null)
  })

  test('only offers resend after an invoice has been issued', () => {
    const issued = {
      ...application,
      status: 'issued',
      invoice_email_send_count: 1,
    } as InvoiceApplication

    assert.equal(getInvoiceEmailAction(issued, false), 'resend')
    assert.equal(getInvoiceEmailAction(issued, true), 'resend')
  })
})

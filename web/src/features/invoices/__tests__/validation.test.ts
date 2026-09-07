/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'

import type { TFunction } from 'i18next'
import { describe, test } from 'vitest'

import { createInvoiceApplicationSchema } from '../lib/invoice-application-form'

const t = ((key: string) => key) as TFunction

describe('invoice application validation', () => {
  test('accepts paid orders with required enterprise buyer information', () => {
    const result = createInvoiceApplicationSchema(t).safeParse({
      top_up_ids: [11, 12],
      invoice_title: 'Example Technology Co., Ltd.',
      tax_number: '91310000EXAMPLE',
      recipient_email: 'finance@example.com',
      applicant_note: 'Please include the project name on the invoice.',
    })

    assert.equal(result.success, true)
  })

  test('rejects an application without a paid order', () => {
    const result = createInvoiceApplicationSchema(t).safeParse({
      top_up_ids: [],
      invoice_title: 'Example Technology Co., Ltd.',
      tax_number: '91310000EXAMPLE',
      recipient_email: 'finance@example.com',
      applicant_note: '',
    })

    assert.equal(result.success, false)
  })

  test('rejects an invalid invoice recipient email', () => {
    const result = createInvoiceApplicationSchema(t).safeParse({
      top_up_ids: [11],
      invoice_title: 'Example Technology Co., Ltd.',
      tax_number: '91310000EXAMPLE',
      recipient_email: 'not-an-email',
      applicant_note: '',
    })

    assert.equal(result.success, false)
  })

  test('rejects an application note longer than 2000 characters', () => {
    const result = createInvoiceApplicationSchema(t).safeParse({
      top_up_ids: [11],
      invoice_title: 'Example Technology Co., Ltd.',
      tax_number: '91310000EXAMPLE',
      recipient_email: 'finance@example.com',
      applicant_note: 'a'.repeat(2001),
    })

    assert.equal(result.success, false)
  })

  test('rejects an application without a recipient email', () => {
    const result = createInvoiceApplicationSchema(t).safeParse({
      top_up_ids: [11],
      invoice_title: 'Example Technology Co., Ltd.',
      tax_number: '91310000EXAMPLE',
      recipient_email: '',
      applicant_note: '',
    })

    assert.equal(result.success, false)
  })
})

/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { canDeleteInvoiceApplication } from '../lib/invoice-delete-action'
import type { InvoiceApplication } from '../types'

const application = {
  status: 'pending_review',
} as InvoiceApplication

describe('invoice application deletion', () => {
  test('allows an administrator to delete a reviewable or unpaid application', () => {
    assert.equal(canDeleteInvoiceApplication(application, true), true)
    assert.equal(
      canDeleteInvoiceApplication({ ...application, status: 'rejected' }, true),
      true
    )
    assert.equal(
      canDeleteInvoiceApplication(
        {
          ...application,
          status: 'pending_payment',
          payment_status: 'pending',
          payment_confirmed_at: 0,
        },
        true
      ),
      true
    )
  })

  test('protects issued applications and hides deletion from regular users', () => {
    assert.equal(
      canDeleteInvoiceApplication({ ...application, status: 'issued' }, true),
      false
    )
    assert.equal(canDeleteInvoiceApplication(application, false), false)
    assert.equal(
      canDeleteInvoiceApplication(
        { ...application, quota_per_unit_snapshot: 500_000 },
        true
      ),
      false
    )
  })
})

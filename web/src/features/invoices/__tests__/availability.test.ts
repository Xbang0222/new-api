/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'

import { describe, test } from 'vitest'

import {
  canSubmitInvoiceApplication,
  getInvoiceOrderSelectionState,
  type InvoiceApplicationAvailability,
} from '../lib/invoice-application-availability'

const readyAvailability: InvoiceApplicationAvailability = {
  configEnabled: true,
  configLoading: false,
  configError: false,
  ordersLoading: false,
  ordersError: false,
  orderCount: 1,
}

describe('invoice application availability', () => {
  test('shows the empty state when there are no eligible paid orders', () => {
    const state = getInvoiceOrderSelectionState({
      ...readyAvailability,
      orderCount: 0,
    })

    assert.equal(state, 'empty')
  })

  test('shows the order error state when eligible orders fail to load', () => {
    const state = getInvoiceOrderSelectionState({
      ...readyAvailability,
      ordersError: true,
    })

    assert.equal(state, 'orders_error')
  })

  test('prevents submission until a paid order is selected', () => {
    assert.equal(
      canSubmitInvoiceApplication(readyAvailability, 0, false),
      false
    )
    assert.equal(canSubmitInvoiceApplication(readyAvailability, 1, false), true)
  })
})

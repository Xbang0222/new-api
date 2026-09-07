/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

import assert from 'node:assert/strict'

import { renderToStaticMarkup } from 'react-dom/server'
import { test } from 'vitest'

import { InvoiceOrderList } from '../components/invoice-details-dialog'

test('invoice details list each selected paid order', () => {
  const html = renderToStaticMarkup(
    <InvoiceOrderList
      currency='CNY'
      orders={[
        {
          id: 1,
          application_id: 7,
          top_up_id: 23,
          trade_no: 'topup-20260807-23',
          paid_amount_cents: 12_345,
          payment_method: 'alipay',
          completed_at: 1_759_500_000,
        },
      ]}
    />
  )

  assert.match(html, /topup-20260807-23/)
  assert.match(html, /alipay/)
  assert.match(html, /123\.45/)
})

test('invoice details show an empty state when selected orders are unavailable', () => {
  const html = renderToStaticMarkup(
    <InvoiceOrderList currency='CNY' orders={[]} />
  )

  assert.match(html, /No eligible paid orders/)
})

/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'

import { renderToStaticMarkup } from 'react-dom/server'
import { describe, test } from 'vitest'

import { InvoiceApplicationsList } from '../components/invoice-applications-list'
import type { InvoiceApplication } from '../types'

const application: InvoiceApplication = {
  id: 42,
  user_id: 7,
  status: 'pending_review',
  payment_status: 'not_required',
  invoice_title: 'Example Company',
  tax_number: '91310000TEST',
  recipient_email: 'finance@example.com',
  applicant_note: '',
  invoice_item_name: 'AI Agent services',
  currency: 'CNY',
  order_amount_cents: 10_000,
  invoice_amount_cents: 10_000,
  estimated_vat_cents: 0,
  estimated_urban_tax_cents: 0,
  estimated_education_surcharge_cents: 0,
  estimated_local_education_surcharge_cents: 0,
  estimated_pit_withholding_cents: 0,
  estimated_total_tax_cents: 0,
  suggested_supplement_cents: 0,
  final_supplement_cents: 0,
  tax_adjustment_reason: '',
  payment_trade_no: '',
  payment_method: '',
  payment_confirmed_at: 0,
  admin_note: '',
  reject_reason: '',
  reviewer_id: 0,
  reviewed_at: 0,
  invoice_file_name: '',
  invoice_file_content_type: '',
  invoice_email_sent_at: 0,
  invoice_email_send_count: 0,
  created_at: 1_722_038_400,
  updated_at: 1_722_038_400,
  issued_at: 0,
  orders: [],
}

function renderInvoiceList() {
  return renderToStaticMarkup(
    <InvoiceApplicationsList
      applications={[application]}
      loading={false}
      fetching={false}
      page={1}
      pageSize={20}
      total={1}
      keyword=''
      isAdmin
      sendingId={null}
      deletingId={null}
      refreshing={false}
      onPageChange={() => {}}
      onPageSizeChange={() => {}}
      onKeywordChange={() => {}}
      onStatusChange={() => {}}
      onView={() => {}}
      onSend={() => {}}
      onDelete={() => {}}
      onRefresh={() => {}}
    />
  )
}

describe('invoice applications list layout', () => {
  test('keeps the full search hint readable with a wider desktop input', () => {
    const html = renderInvoiceList()

    assert.match(
      html,
      /placeholder="Search by ID, invoice title, tax number, or email\.\.\."/
    )
    assert.match(html, /sm:w-\[360px\]/)
    assert.match(html, /lg:w-\[460px\]/)
  })

  test('renders refresh with view controls and exposes the admin delete action', () => {
    const html = renderInvoiceList()
    const refreshIndex = html.indexOf('aria-label="Refresh"')
    const viewIndex = html.indexOf('aria-label="View"')

    assert.ok(refreshIndex >= 0)
    assert.ok(viewIndex > refreshIndex)
    assert.match(html, />Delete<\/button>/)
  })
})

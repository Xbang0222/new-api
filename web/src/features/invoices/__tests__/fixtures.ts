/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { InvoiceApplication, InvoiceConfig } from '../types'

export const config: InvoiceConfig = {
  enabled: true,
  minimum_amount: 20,
  currency: 'CNY',
  currency_symbol: '¥',
  invoice_item_name: 'AI services',
  fee_rate_basis_points: 300,
  quota_per_currency_unit: 100,
}

export const application: InvoiceApplication = {
  id: 42,
  user_id: 7,
  status: 'pending_review',
  payment_status: 'paid',
  invoice_title: 'Example Company',
  tax_number: '91310000TEST',
  recipient_email: 'finance@example.com',
  applicant_note: '',
  invoice_item_name: 'AI services',
  currency: 'CNY',
  currency_symbol: '¥',
  fee_mode: 'balance_on_apply',
  fee_rate_basis_points: 300,
  charged_quota: 150,
  refunded_quota: 0,
  order_amount_cents: 5000,
  invoice_amount_cents: 5000,
  estimated_vat_cents: 0,
  estimated_urban_tax_cents: 0,
  estimated_education_surcharge_cents: 0,
  estimated_local_education_surcharge_cents: 0,
  estimated_pit_withholding_cents: 0,
  estimated_total_tax_cents: 0,
  suggested_supplement_cents: 150,
  final_supplement_cents: 150,
  tax_adjustment_reason: '',
  payment_trade_no: '',
  payment_method: 'balance',
  payment_confirmed_at: 1_800_000_000,
  admin_note: '',
  reject_reason: '',
  reviewer_id: 0,
  reviewed_at: 0,
  invoice_file_name: '',
  invoice_file_content_type: '',
  invoice_email_sent_at: 0,
  invoice_email_send_count: 0,
  created_at: 1_800_000_000,
  updated_at: 1_800_000_000,
  issued_at: 0,
  orders: [],
}

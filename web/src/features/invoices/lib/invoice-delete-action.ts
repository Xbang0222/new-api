/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { InvoiceApplication } from '../types'

export function canDeleteInvoiceApplication(
  application: InvoiceApplication,
  isAdmin: boolean
): boolean {
  if (!isAdmin) return false
  if (
    typeof application.can_cancel === 'boolean' ||
    application.fee_mode === 'balance_on_apply'
  ) {
    return canCancelInvoiceApplication(application)
  }
  if (application.issued_at || application.invoice_file_name) return false
  if (
    application.status === 'pending_review' ||
    application.status === 'rejected'
  ) {
    return true
  }
  return (
    application.status === 'pending_payment' &&
    application.payment_status === 'pending' &&
    application.payment_confirmed_at === 0
  )
}

export function canCancelInvoiceApplication(
  application: InvoiceApplication
): boolean {
  if (
    application.issued_at ||
    application.invoice_file_name ||
    application.invoice_email_sent_at ||
    application.status === 'issued'
  ) {
    return false
  }
  if (typeof application.can_cancel === 'boolean') return application.can_cancel
  if (application.fee_mode === 'balance_on_apply') {
    return ['pending_review', 'approved', 'rejected'].includes(
      application.status
    )
  }
  return false
}

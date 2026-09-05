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
    (application.quota_per_unit_snapshot ?? 0) > 0 ||
    (application.exchange_rate_snapshot ?? 0) > 0
  ) {
    return false
  }
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

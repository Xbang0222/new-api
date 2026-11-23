/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { TFunction } from 'i18next'
import { z } from 'zod'

// Keep in step with maxInvoiceOrdersPerApplication in service/invoice.go.
export const MAX_INVOICE_ORDERS_PER_APPLICATION = 300

export function createInvoiceApplicationSchema(t: TFunction) {
  return z.object({
    top_up_ids: z
      .array(z.number().int().positive())
      .min(1, t('Select at least one paid order.'))
      .max(
        MAX_INVOICE_ORDERS_PER_APPLICATION,
        t('You can select up to {{max}} paid orders.', {
          max: MAX_INVOICE_ORDERS_PER_APPLICATION,
        })
      ),
    invoice_title: z
      .string()
      .trim()
      .min(1, t('Invoice title is required.'))
      .max(255, t('Invoice title is too long.')),
    tax_number: z
      .string()
      .trim()
      .min(1, t('Tax identification number is required.'))
      .max(64, t('Tax identification number is too long.')),
    recipient_email: z
      .string()
      .trim()
      .min(1, t('Recipient email is required.'))
      .max(255)
      .email(t('Enter a valid recipient email address.')),
    applicant_note: z
      .string()
      .trim()
      .max(2000, t('Application note must be 2000 characters or fewer.')),
  })
}

export type InvoiceApplicationFormValues = z.infer<
  ReturnType<typeof createInvoiceApplicationSchema>
>

export const invoiceApplicationFormDefaults: InvoiceApplicationFormValues = {
  top_up_ids: [],
  invoice_title: '',
  tax_number: '',
  recipient_email: '',
  applicant_note: '',
}

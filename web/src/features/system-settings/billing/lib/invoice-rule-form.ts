/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { TFunction } from 'i18next'
import { z } from 'zod'

export function createInvoiceRuleSchema(t: TFunction) {
  return z.object({
    enabled: z.boolean(),
    minimumAmount: z.coerce
      .number()
      .finite()
      .min(0, t('Minimum invoice amount cannot be negative.')),
    invoiceItemName: z
      .string()
      .trim()
      .min(1, t('Invoice item name is required.'))
      .max(255),
    feeRatePercent: z.coerce
      .number()
      .finite()
      .min(0, t('Fee rate cannot be negative.'))
      .max(100, t('Fee rate cannot exceed 100%.')),
  })
}

export type InvoiceRuleFormValues = z.infer<
  ReturnType<typeof createInvoiceRuleSchema>
>

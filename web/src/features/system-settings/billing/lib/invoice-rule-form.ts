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
    minimumAmount: z
      .number({ error: t('Please enter a valid number') })
      .min(0, t('Minimum invoice amount cannot be negative.')),
    invoiceItemName: z
      .string()
      .trim()
      .min(1, t('Invoice item name is required.'))
      .max(255),
    feeRatePercent: z
      .number({ error: t('Please enter a valid number') })
      .min(0, t('Invoice fee rate cannot be negative.'))
      .max(100, t('Invoice fee rate cannot exceed 100%.'))
      .multipleOf(
        0.01,
        t('Invoice fee rate supports up to two decimal places.')
      ),
  })
}

export type InvoiceRuleFormValues = z.infer<
  ReturnType<typeof createInvoiceRuleSchema>
>

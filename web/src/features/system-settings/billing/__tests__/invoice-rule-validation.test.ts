/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import { createInvoiceRuleSchema } from '../lib/invoice-rule-form'

const t = ((key: string) => key) as TFunction

const baseRules = {
  enabled: true,
  minimumAmount: 50,
  invoiceItemName: 'AI Agent服务',
  feeRatePercent: 3,
}

describe('invoice rule validation', () => {
  test.each([0, 3.25, 100])(
    'accepts a fee rate of %s percent without legacy tax settings',
    (feeRatePercent) => {
      const result = createInvoiceRuleSchema(t).safeParse({
        ...baseRules,
        feeRatePercent,
      })

      expect(result.success).toBe(true)
    }
  )

  test.each([-0.01, 100.01, 3.001, Number.NaN, Number.POSITIVE_INFINITY])(
    'rejects an invalid fee rate of %s percent',
    (feeRatePercent) => {
      const result = createInvoiceRuleSchema(t).safeParse({
        ...baseRules,
        feeRatePercent,
      })

      expect(result.success).toBe(false)
      if (!result.success) {
        expect(result.error.issues.map((issue) => issue.path)).toContainEqual([
          'feeRatePercent',
        ])
      }
    }
  )

  test('rejects a negative minimum amount', () => {
    const result = createInvoiceRuleSchema(t).safeParse({
      ...baseRules,
      minimumAmount: -0.01,
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map((issue) => issue.path)).toContainEqual([
        'minimumAmount',
      ])
    }
  })

  test('requires a nonempty invoice item name', () => {
    const result = createInvoiceRuleSchema(t).safeParse({
      ...baseRules,
      invoiceItemName: '   ',
    })

    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues.map((issue) => issue.path)).toContainEqual([
        'invoiceItemName',
      ])
    }
  })

  test('trims the item name and ignores legacy currency and tax options', () => {
    const result = createInvoiceRuleSchema(t).safeParse({
      ...baseRules,
      invoiceItemName: '  AI Agent服务  ',
      currency: 'CNY',
      supplementPaymentMethod: 'epay',
      vatRatePercent: 1,
    })

    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data).toEqual(baseRules)
    }
  })
})

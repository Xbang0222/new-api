/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { TFunction } from 'i18next'

import { createInvoiceRuleSchema } from '../lib/invoice-rule-form'
const t = ((key: string) => key) as TFunction
const baseRules = {
  enabled: true,
  minimumAmount: 50,
  invoiceItemName: 'AI Agent服务',
  feeRatePercent: 3,
}
describe('invoice rule validation', () => {
  test('accepts the invoice item, minimum amount, and fee rate', () => {
    assert.equal(createInvoiceRuleSchema(t).safeParse(baseRules).success, true)
  })
  test('rejects a fee rate above one hundred percent', () => {
    const result = createInvoiceRuleSchema(t).safeParse({
      ...baseRules,
      feeRatePercent: 101,
    })
    assert.equal(result.success, false)
    if (!result.success)
      assert.deepEqual(result.error.issues[0]?.path, ['feeRatePercent'])
  })
  test('rejects non-finite amounts and fee rates', () => {
    assert.equal(
      createInvoiceRuleSchema(t).safeParse({
        ...baseRules,
        minimumAmount: Number.NaN,
      }).success,
      false
    )
    assert.equal(
      createInvoiceRuleSchema(t).safeParse({
        ...baseRules,
        feeRatePercent: Number.POSITIVE_INFINITY,
      }).success,
      false
    )
  })
})

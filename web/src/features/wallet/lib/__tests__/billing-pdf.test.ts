/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { readFileSync, mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'

import { createInstance } from 'i18next'
import { PDFDocument } from 'pdf-lib'
import { describe, expect, test } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'

import type { TopupRecord } from '../../types'
import { createBillingHistoryPdf, summarizeTopUpPayments } from '../billing-pdf'

const paidOrder: TopupRecord = {
  id: 1,
  user_id: 42,
  username: '张三',
  amount: 10,
  money: 999,
  trade_no: 'ORDER-20261116-001',
  payment_method: 'alipay',
  payment_provider: 'epay',
  status: 'success',
  paid_amount: '123.45',
  paid_currency: 'CNY',
  create_time: 1794787200,
  complete_time: 1794787260,
}

describe('top-up payment statement', () => {
  test('sums exact verified successful payments separately by currency', () => {
    const result = summarizeTopUpPayments([
      paidOrder,
      { ...paidOrder, paid_amount: '0.55' },
      { ...paidOrder, paid_amount: '90071992547409.91', paid_currency: 'USD' },
      { ...paidOrder, paid_amount: '90071992547409.91', paid_currency: 'USD' },
      { ...paidOrder, paid_amount: '4500', paid_currency: 'JPY' },
      { ...paidOrder, paid_amount: '', paid_currency: '' },
      { ...paidOrder, status: 'pending' },
      { ...paidOrder, status: 'failed' },
      { ...paidOrder, status: 'expired' },
    ])
    expect(result).toEqual({
      success: 6,
      unverified: 1,
      totals: [
        { currency: 'CNY', amount: '124.00' },
        { currency: 'JPY', amount: '4500' },
        { currency: 'USD', amount: '180143985094819.82' },
      ],
    })
  })

  test.each(['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'])(
    'exports a paginated, readable %s document',
    async (language) => {
      const i18n = createInstance()
      await i18n.init({
        lng: language,
        fallbackLng: 'en',
        nsSeparator: false,
        keySeparator: false,
        resources: { en, zh, 'zh-TW': zhTW, fr, ja, ru, vi },
        interpolation: { escapeValue: false },
      })
      const records = Array.from({ length: 18 }, (_, index) => ({
        ...paidOrder,
        id: index + 1,
        username:
          index === 1 ? '陈晓明-日本語-Русский-Đặng' : paidOrder.username,
        trade_no:
          index === 2
            ? `LONG-${'1234567890'.repeat(24)}`
            : `ORDER-20261116-${String(index + 1).padStart(3, '0')}`,
        ...(index === 3
          ? { paid_amount: '', paid_currency: '', complete_time: 0 }
          : {}),
        ...(index === 4
          ? {
              status: 'pending' as const,
              paid_amount: '',
              paid_currency: '',
              complete_time: 0,
            }
          : {}),
      }))
      const font = new Uint8Array(
        readFileSync(path.resolve('src/assets/fonts/NotoSansCJKsc-Regular.ttf'))
      )
      const data = {
        items: records,
        total: records.length,
        generated_at: 1794789000,
      }
      const bytes = await createBillingHistoryPdf(
        data,
        {
          siteName: 'new-api 测试服务',
          language,
          filters: { user_id: '42' },
          t: i18n.t.bind(i18n),
        },
        font
      )
      const pdf = await PDFDocument.load(bytes)
      expect(pdf.getPageCount()).toBeGreaterThan(1)
      expect(pdf.getTitle()).toBe(i18n.t('Top-up transaction record'))
      expect(bytes.length).toBeLessThan(500_000)
      const output = process.env.WALLET_PDF_OUTPUT_DIR
      if (output) {
        mkdirSync(output, { recursive: true })
        writeFileSync(path.join(output, `topup-record-${language}.pdf`), bytes)
      }
    }
  )
})

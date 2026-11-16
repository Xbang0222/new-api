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
import { readFileSync } from 'node:fs'
import path from 'node:path'

import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next, { createInstance } from 'i18next'
import { PDFDocument } from 'pdf-lib'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { BillingHistoryDialog } from '@/features/wallet/components/dialogs/billing-history-dialog'
import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { RechargeFormCard } from '../components/recharge-form-card'

const i18n = createInstance()

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
  vi.unstubAllGlobals()
})

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en, zh },
    interpolation: { escapeValue: false },
  })
})

it('translates billing history statuses and updates them when the language changes', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        total: 3,
        items: ['success', 'pending', 'expired'].map((status, id) => ({
          id,
          status,
          amount: 10,
          money: 10,
          trade_no: `order-${id}`,
          payment_method: 'stripe',
          create_time: 1,
        })),
      },
    },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <BillingHistoryDialog open onOpenChange={vi.fn()} />
    </I18nextProvider>
  )
  await screen.findByText('order-0')
  for (const label of ['成功', '待确认', '已过期']) {
    expect(screen.getByText(label)).toBeVisible()
  }
  await act(() => i18n.changeLanguage('en'))
  for (const label of ['Success', 'Pending', 'Expired']) {
    expect(screen.getByText(label)).toBeVisible()
  }
})

it('searches by username and user ID, then downloads all matching orders as a PDF', async () => {
  await i18n.changeLanguage('en')
  await i18next.init({
    lng: 'en',
    resources: { en },
    nsSeparator: false,
    keySeparator: false,
  })
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 10 })
  const target = {
    id: 11,
    user_id: 42,
    username: '张三',
    trade_no: 'TARGET-1',
    amount: 10,
    money: 20,
    paid_amount: '20.00',
    paid_currency: 'CNY',
    payment_method: 'alipay',
    status: 'success',
    create_time: 1794787200,
  }
  const get = vi.spyOn(api, 'get').mockImplementation(async (url, config) => {
    const params = config?.params as Record<string, unknown> | undefined
    if (params?.export === 'true') {
      return {
        data: {
          success: true,
          data: {
            items: [target, { ...target, id: 12, trade_no: 'TARGET-2' }],
            total: 2,
            generated_at: 1794789000,
          },
        },
      }
    }
    const selected =
      String(url).includes('user_id=42') || String(url).includes('username=')
    return {
      data: {
        success: true,
        data: {
          items: selected
            ? [target]
            : [
                {
                  ...target,
                  user_id: 7,
                  username: 'other',
                  trade_no: 'OTHER-ORDER',
                },
              ],
          total: selected ? 2 : 1,
        },
      },
    }
  })
  const font = new Uint8Array(
    readFileSync(path.resolve('src/assets/fonts/NotoSansCJKsc-Regular.ttf'))
  )
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(font))
  let downloadedBlob: Blob | undefined
  const OriginalURL = URL
  vi.stubGlobal(
    'URL',
    class extends OriginalURL {
      static createObjectURL(blob: Blob) {
        downloadedBlob = blob
        return 'blob:wallet-statement'
      }
      static revokeObjectURL() {}
    }
  )
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, 'click')
    .mockImplementation(() => {})
  const user = userEvent.setup()
  render(
    <I18nextProvider i18n={i18n}>
      <BillingHistoryDialog open onOpenChange={vi.fn()} />
    </I18nextProvider>
  )
  await screen.findByText('OTHER-ORDER')
  fireEvent.change(screen.getByRole('textbox', { name: 'Username' }), {
    target: { value: '张三' },
  })
  expect(screen.getByRole('button', { name: 'Export PDF' })).toBeDisabled()
  await screen.findByText('TARGET-1')
  expect(get).toHaveBeenCalledWith(
    expect.stringContaining('username=%E5%BC%A0%E4%B8%89')
  )
  await user.click(screen.getByRole('combobox', { name: 'Search user by' }))
  await user.click(screen.getByRole('option', { name: 'User ID' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'User ID' }), {
    target: { value: '42' },
  })
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith(expect.stringContaining('user_id=42'))
  )
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Export PDF' })).toBeEnabled()
  )
  await user.click(screen.getByRole('button', { name: 'Export PDF' }))
  await waitFor(() => expect(click).toHaveBeenCalledOnce())
  expect(get).toHaveBeenCalledWith(
    '/api/user/topup',
    expect.objectContaining({
      params: { keyword: '', user_id: '42', export: 'true' },
    })
  )
  expect(downloadedBlob?.type).toBe('application/pdf')
  const blob = downloadedBlob
  if (!blob) throw new Error('Expected a PDF download')
  const bytes = await new Promise<ArrayBuffer>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () => resolve(reader.result as ArrayBuffer),
      { once: true }
    )
    reader.addEventListener('error', reject, { once: true })
    reader.readAsArrayBuffer(blob)
  })
  const pdf = await PDFDocument.load(bytes)
  expect(pdf.getTitle()).toBe('Top-up transaction record')
  expect(pdf.getPageCount()).toBeGreaterThan(0)
})

it('keeps ordinary users on their own billing endpoint without account search controls', async () => {
  await i18n.changeLanguage('en')
  useAuthStore
    .getState()
    .auth.setUser({ id: 42, username: 'customer', role: 1 })
  const get = vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { items: [], total: 0 } },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <BillingHistoryDialog open onOpenChange={vi.fn()} />
    </I18nextProvider>
  )
  await screen.findByText('No billing records found')
  expect(get).toHaveBeenCalledWith(
    expect.stringContaining('/api/user/topup/self?')
  )
  expect(
    screen.queryByRole('combobox', { name: 'Search user by' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Export PDF' })).toBeDisabled()
})

it.each([
  { priceRatio: 7.3, paid: '584', saved: '146', full: '146' },
  { priceRatio: 0.000123, paid: '0.0098', saved: '0.0025', full: '0.0025' },
])(
  'translates recharge presets and minimum amount without changing precision at price ratio $priceRatio',
  async ({ priceRatio, paid, saved, full }) => {
    render(
      <I18nextProvider i18n={i18n}>
        <RechargeFormCard
          topupInfo={{
            enable_online_topup: true,
            enable_stripe_topup: false,
            pay_methods: [],
            min_topup: 1,
            stripe_min_topup: 1,
            amount_options: [100, 20],
            discount: {},
          }}
          presetAmounts={[{ value: 100, discount: 0.8 }, { value: 20 }]}
          selectedPreset={null}
          topupAmount={1}
          paymentAmount={priceRatio}
          priceRatio={priceRatio}
          calculating={false}
          paymentLoading={null}
          redemptionCode=''
          redeeming={false}
          onSelectPreset={vi.fn()}
          onTopupAmountChange={vi.fn()}
          onPaymentMethodSelect={vi.fn()}
          onRedemptionCodeChange={vi.fn()}
          onRedeem={vi.fn()}
        />
      </I18nextProvider>
    )
    expect(screen.getByRole('button', { name: /^100 / })).toHaveTextContent(
      `优惠 20%实付 ${paid} • 节省 ${saved}`
    )
    expect(screen.getByRole('button', { name: /^20 / })).toHaveTextContent(
      `实付 ${full}`
    )
    expect(screen.getByPlaceholderText('最低 1')).toBeVisible()
    await act(() => i18n.changeLanguage('en'))
    expect(screen.getByRole('button', { name: /^100 / })).toHaveTextContent(
      `20% OFFPay ${paid} • Save ${saved}`
    )
    expect(screen.getByRole('button', { name: /^20 / })).toHaveTextContent(
      `Pay ${full}`
    )
    expect(screen.getByPlaceholderText('Minimum 1')).toBeVisible()
  }
)

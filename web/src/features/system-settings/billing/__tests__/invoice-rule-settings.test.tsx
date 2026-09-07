/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { SettingsPageProvider } from '../../components/settings-page-context'
import {
  InvoiceManagementSection,
  type InvoiceRuleDefaults,
} from '../invoice-management-section'

const defaultValues: InvoiceRuleDefaults = {
  enabled: true,
  invoiceItemName: 'Technical services',
  minimumAmount: 0,
  feeRatePercent: 0,
}

const initialSystemConfig = useSystemConfigStore.getState().config
const initialStorage = useSystemConfigStore.persist.getOptions().storage
let queryClient: QueryClient
let actionsContainer: HTMLDivElement

function renderInvoiceSettings() {
  render(
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <InvoiceManagementSection defaultValues={defaultValues} />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

beforeEach(() => {
  useSystemConfigStore.persist.setOptions({
    storage: {
      getItem: () => null,
      setItem: () => undefined,
      removeItem: () => undefined,
    },
  })
  queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  actionsContainer = document.createElement('div')
  document.body.append(actionsContainer)
  useSystemConfigStore.getState().setConfig({
    currency: { ...DEFAULT_CURRENCY_CONFIG },
  })
})

afterEach(() => {
  queryClient.clear()
  actionsContainer.remove()
  useSystemConfigStore.getState().setConfig(initialSystemConfig)
  useSystemConfigStore.persist.setOptions({ storage: initialStorage })
})

describe('invoice rule settings', () => {
  test.each<[CurrencyDisplayType, string]>([
    ['USD', 'USD'],
    ['CNY', 'CNY'],
    ['CUSTOM', '€'],
    ['TOKENS', 'USD'],
  ])(
    'shows the system %s currency as read-only %s',
    (quotaDisplayType, expected) => {
      useSystemConfigStore.getState().setConfig({
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType,
          customCurrencySymbol: '€',
        },
      })

      renderInvoiceSettings()

      const currencyInput = screen.getByRole('textbox', { name: 'Currency' })
      expect(currencyInput).toHaveValue(expected)
      expect(currencyInput).toHaveAttribute('readonly')
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
      expect(screen.getAllByRole('spinbutton')).toHaveLength(2)
      expect(
        screen.getByRole('switch', { name: 'Enable invoice applications' })
      ).toBeChecked()
    }
  )

  test('updates the read-only currency when the system currency changes', () => {
    renderInvoiceSettings()

    act(() => {
      useSystemConfigStore.getState().setConfig({
        currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'CNY' },
      })
    })

    expect(screen.getByRole('textbox', { name: 'Currency' })).toHaveValue('CNY')
  })

  test('saves only the four invoice settings and converts the percent to basis points', async () => {
    const put = vi.spyOn(api, 'put').mockResolvedValue({
      data: { success: true, message: '' },
    })
    renderInvoiceSettings()

    fireEvent.change(
      screen.getByRole('textbox', { name: 'Invoice item name' }),
      {
        target: { value: '  AI services  ' },
      }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Minimum invoice amount' }),
      {
        target: { value: '50' },
      }
    )
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Invoice fee rate (%)' }),
      {
        target: { value: '3.25' },
      }
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save invoice rules' }))

    await waitFor(() => {
      expect(put).toHaveBeenCalledTimes(4)
      expect(
        screen.getByRole('button', { name: 'Save invoice rules' })
      ).toBeEnabled()
    })
    expect(put.mock.calls.map((call) => call[1])).toEqual([
      { key: 'invoice_setting.enabled', value: true },
      { key: 'invoice_setting.invoice_item_name', value: 'AI services' },
      { key: 'invoice_setting.minimum_amount', value: 50 },
      { key: 'invoice_setting.fee_rate_basis_points', value: 325 },
    ])
  })

  test.each([
    ['3.001', 'Invoice fee rate supports up to two decimal places.'],
    ['', 'Please enter a valid number'],
  ])(
    'blocks fee input %s with an accessible field error and no save request',
    async (value, message) => {
      const put = vi.spyOn(api, 'put')
      renderInvoiceSettings()
      const feeInput = screen.getByRole('spinbutton', {
        name: 'Invoice fee rate (%)',
      })

      fireEvent.change(feeInput, { target: { value } })
      fireEvent.click(
        screen.getByRole('button', { name: 'Save invoice rules' })
      )

      expect(await screen.findByText(message)).toBeInTheDocument()
      expect(feeInput).toHaveAttribute('aria-invalid', 'true')
      expect(put).not.toHaveBeenCalled()
    }
  )
})

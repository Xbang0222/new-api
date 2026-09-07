/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

export function formatInvoiceMoney(
  cents: number,
  currency: string,
  currencySymbol?: string
): string {
  if (currency === 'CUSTOM') {
    return `${currencySymbol || '¤'}${new Intl.NumberFormat(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(cents / 100)}`
  }
  const safeCurrency = /^[A-Z]{3}$/.test(currency) ? currency : 'CNY'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: safeCurrency,
  }).format(cents / 100)
}

export function formatInvoiceDate(timestamp: number): string {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(timestamp * 1000))
}

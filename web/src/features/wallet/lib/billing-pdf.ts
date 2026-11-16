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
import fontkit from '@pdf-lib/fontkit'
import type { TFunction } from 'i18next'
import { PDFDocument, rgb, type PDFFont, type PDFPage } from 'pdf-lib'

import statementFontUrl from '@/assets/fonts/NotoSansCJKsc-Regular.ttf'
import { toIntlLocale } from '@/i18n/languages'
import { formatRecordedCurrencyAmount } from '@/lib/currency'
import { formatNumber } from '@/lib/format'

import type {
  BillingHistoryExport,
  BillingHistoryFilters,
  TopupRecord,
} from '../types'
import { getPaymentMethodName, getStatusConfig } from './billing'

interface BillingPdfOptions {
  siteName: string
  language: string
  filters: BillingHistoryFilters
  t: TFunction
}

export function summarizeTopUpPayments(records: TopupRecord[]) {
  const amounts = new Map<string, { value: bigint; scale: number }>()
  let success = 0
  let unverified = 0
  for (const record of records) {
    if (record.status !== 'success') continue
    success += 1
    const currency = record.paid_currency
    if (
      !record.paid_amount ||
      !/^\d+(?:\.\d{1,4})?$/.test(record.paid_amount) ||
      !currency ||
      !/^[A-Z]{3}$/.test(currency)
    ) {
      unverified += 1
      continue
    }
    const [whole, fraction = ''] = record.paid_amount.split('.')
    const previous = amounts.get(currency) ?? {
      value: 0n,
      scale: 0,
    }
    const scale = Math.max(previous.scale, fraction.length)
    amounts.set(currency, {
      value:
        previous.value * 10n ** BigInt(scale - previous.scale) +
        BigInt(whole + fraction) * 10n ** BigInt(scale - fraction.length),
      scale,
    })
  }
  return {
    success,
    unverified,
    totals: [...amounts]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([currency, { value, scale }]) => {
        const digits = value.toString().padStart(scale + 1, '0')
        return {
          currency,
          amount: scale
            ? `${digits.slice(0, -scale)}.${digits.slice(-scale)}`
            : digits,
        }
      }),
  }
}

function wrapPdfText(
  text: string,
  font: PDFFont,
  size: number,
  width: number
): string[] {
  const lines: string[] = []
  for (const paragraph of text.split('\n')) {
    let line = ''
    for (const character of paragraph) {
      if (line && font.widthOfTextAtSize(line + character, size) > width) {
        const lastSpace = line.lastIndexOf(' ')
        if (character === ' ') {
          lines.push(line.trimEnd())
          line = ''
        } else if (lastSpace > 0) {
          lines.push(line.slice(0, lastSpace))
          line = line.slice(lastSpace + 1) + character
        } else {
          lines.push(line)
          line = character
        }
      } else {
        line += character
      }
    }
    lines.push(line)
  }
  return lines
}

function drawPdfLines(
  page: PDFPage,
  lines: string[],
  font: PDFFont,
  x: number,
  y: number,
  size = 9,
  lineHeight = 13
) {
  for (const [index, line] of lines.entries()) {
    page.drawText(line, {
      x,
      y: y - index * lineHeight,
      size,
      font,
      color: rgb(0.16, 0.2, 0.25),
    })
  }
}

function statementTimestamp(timestamp?: number): string {
  return timestamp && timestamp > 0
    ? new Date(timestamp * 1000)
        .toISOString()
        .replace('T', ' ')
        .replace('.000Z', ' UTC')
    : '-'
}

/** Builds a selectable-text PDF locally; account data never leaves this application. */
export async function createBillingHistoryPdf(
  data: BillingHistoryExport,
  options: BillingPdfOptions,
  fontBytes?: Uint8Array
): Promise<Uint8Array<ArrayBuffer>> {
  const { t, filters } = options
  if (!fontBytes) {
    const response = await fetch(statementFontUrl, {
      signal: AbortSignal.timeout(30_000),
    })
    if (!response.ok) throw new Error(t('Failed to load the PDF font'))
    fontBytes = new Uint8Array(await response.arrayBuffer())
  }
  const pdf = await PDFDocument.create()
  pdf.registerFontkit(fontkit)
  // The bundled font uses even-byte glyph padding for fontkit's subset encoder.
  // Preserve that preparation when updating it; see assets/fonts/NOTICE.txt.
  const font = await pdf.embedFont(fontBytes, { subset: true })
  const supported = new Set(font.getCharacterSet())
  // Preserve unusual account characters as explicit code points instead of
  // dropping them or allowing an unsupported glyph to abort the whole export.
  const printable = (text: string) =>
    [...text]
      .map((character) => {
        if (character === '\n') return character
        const code = character.codePointAt(0) ?? 0
        if (code < 32 || code === 127) return ' '
        return supported.has(code)
          ? character
          : `[U+${code.toString(16).toUpperCase()}]`
      })
      .join('')
  const number = (value: number) =>
    formatNumber(value, toIntlLocale(options.language))
  const summary = summarizeTopUpPayments(data.items)
  const title = t('Top-up transaction record')
  pdf.setTitle(title)
  pdf.setAuthor(options.siteName)
  pdf.setCreator('new-api')
  pdf.setCreationDate(new Date(data.generated_at * 1000))

  const margin = 40
  const width = 515.28
  const columns = [78, 168, 85, 112, 72.28]
  const headers = [
    t('User'),
    t('Order details'),
    t('Payment Method'),
    t('Payment amount'),
    t('Status'),
  ]
  let page = pdf.addPage([595.28, 841.89])
  let y = 798
  const siteLines = wrapPdfText(printable(options.siteName), font, 10, width)
  drawPdfLines(page, siteLines, font, margin, y, 10)
  y -= siteLines.length * 13 + 19
  drawPdfLines(page, [title], font, margin, y, 21, 27)
  y -= 28
  drawPdfLines(
    page,
    [
      t('Generated at: {{time}}', {
        time: statementTimestamp(data.generated_at),
      }),
    ],
    font,
    margin,
    y
  )
  y -= 23
  const scope =
    [
      filters.username && `${t('Username')}: ${filters.username}`,
      filters.user_id && `${t('User ID')}: ${filters.user_id}`,
      filters.keyword && `${t('Order Number')}: ${filters.keyword}`,
    ]
      .filter(Boolean)
      .join(' | ') || t('All matching orders')
  for (const paragraph of [
    scope,
    `${t('Total records')}: ${number(data.items.length)} | ${t('Success')}: ${number(summary.success)} | ${t('Other statuses')}: ${number(data.items.length - summary.success)}`,
    t('Verified payments by currency'),
    summary.totals.length
      ? summary.totals
          .map((total) =>
            formatRecordedCurrencyAmount(total.amount, total.currency)
          )
          .join(' | ')
      : '-',
    t(
      'Only successful orders with a verified payment amount are included in the payment totals.'
    ),
    ...(summary.unverified
      ? [
          t('Successful orders without a verified payment amount: {{count}}', {
            count: summary.unverified,
          }),
        ]
      : []),
  ]) {
    const lines = wrapPdfText(printable(paragraph), font, 9, width)
    drawPdfLines(page, lines, font, margin, y)
    y -= lines.length * 13 + 8
  }
  y -= 8
  const headerLines = headers.map((text, index) =>
    wrapPdfText(text, font, 8.5, columns[index] - 12)
  )
  const headerHeight =
    Math.max(...headerLines.map((lines) => lines.length)) * 12 + 16
  let needsHeader = true
  for (const record of data.items) {
    const status =
      record.status === 'pending'
        ? t('Pending', { context: 'payment' })
        : t(getStatusConfig(record.status).label)
    const verified = formatRecordedCurrencyAmount(
      record.paid_amount,
      record.paid_currency
    )
    const payment =
      verified !== '-'
        ? verified
        : `${t('Recorded payment amount')}: ${formatNumber(record.money, toIntlLocale(options.language))}\n${t('Currency not recorded')}`
    const cells = [
      `${record.username || t('Unavailable')}\n${t('User ID')}: ${record.user_id}`,
      `${record.trade_no}\n${t('Created')}: ${statementTimestamp(record.create_time)}\n${t('Completed')}: ${statementTimestamp(record.complete_time)}`,
      `${getPaymentMethodName(record.payment_method, t)}\n${record.payment_provider || '-'}`,
      payment,
      status,
    ].map((text, index) =>
      wrapPdfText(printable(text), font, 8.5, columns[index] - 12)
    )
    const height = Math.max(...cells.map((lines) => lines.length)) * 12 + 18
    if (y - height - (needsHeader ? headerHeight : 0) < 75) {
      page = pdf.addPage([595.28, 841.89])
      drawPdfLines(page, [title], font, margin, 800, 14, 18)
      y = 770
      needsHeader = true
    }
    if (needsHeader) {
      page.drawRectangle({
        x: margin,
        y: y - headerHeight,
        width,
        height: headerHeight,
        color: rgb(0.92, 0.95, 0.97),
      })
      let x = margin
      for (const [index, lines] of headerLines.entries()) {
        drawPdfLines(page, lines, font, x + 6, y - 13, 8.5, 12)
        x += columns[index]
      }
      y -= headerHeight
      needsHeader = false
    }
    let x = margin
    for (const [index, lines] of cells.entries()) {
      drawPdfLines(page, lines, font, x + 6, y - 14, 8.5, 12)
      x += columns[index]
    }
    y -= height
    page.drawLine({
      start: { x: margin, y },
      end: { x: margin + width, y },
      thickness: 0.5,
      color: rgb(0.84, 0.87, 0.9),
    })
  }
  const pages = pdf.getPages()
  for (const [index, target] of pages.entries()) {
    const note = wrapPdfText(
      t(
        'This document is a top-up record for reconciliation and is not an invoice.'
      ),
      font,
      8,
      width
    )
    drawPdfLines(target, note, font, margin, 46, 8, 11)
    const counter = t('Page {{page}} of {{total}}', {
      page: index + 1,
      total: pages.length,
    })
    drawPdfLines(target, [counter], font, margin, 16, 8, 11)
  }
  return new Uint8Array(await pdf.save())
}

/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/

import assert from 'node:assert/strict'

import { renderToStaticMarkup } from 'react-dom/server'
import { test } from 'vitest'

import { InvoiceFileAction } from '../components/invoice-details-dialog'

test('the administrator invoice file action only exposes viewing', () => {
  const html = renderToStaticMarkup(
    <InvoiceFileAction busy={false} onView={() => {}} />
  )

  assert.match(html, />View document<\/button>/)
  assert.doesNotMatch(html, /Download invoice/)
})

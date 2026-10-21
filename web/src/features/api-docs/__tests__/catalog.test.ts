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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { apiDocItems, apiEndpoints } from '../data/catalog'

describe('customer API catalog', () => {
  test('uses stable unique IDs for documentation deep links', () => {
    const ids = apiDocItems.map((item) => item.id)

    assert.equal(new Set(ids).size, ids.length)
  })

  test('gives every endpoint enough metadata for a complete reference', () => {
    for (const endpoint of apiEndpoints) {
      assert.ok(endpoint.title.trim(), endpoint.id)
      assert.ok(endpoint.description.trim(), endpoint.id)
      assert.ok(endpoint.path.startsWith('/'), endpoint.id)
      assert.ok(endpoint.parameters, endpoint.id)
      assert.ok(endpoint.requestExample, endpoint.id)
      assert.ok(endpoint.responseExample.trim(), endpoint.id)
      assert.ok(endpoint.responseDescription.trim(), endpoint.id)
    }
  })

  test('covers every public model API family exposed by the relay routers', () => {
    const documentedPaths = apiEndpoints.flatMap((endpoint) => [
      endpoint.path,
      ...(endpoint.relatedEndpoints?.map((related) => related.path) ?? []),
    ])
    const requiredPathPrefixes = [
      '/v1/models',
      '/v1/chat/completions',
      '/v1/responses',
      '/v1/completions',
      '/v1/embeddings',
      '/v1/rerank',
      '/v1/tools/search',
      '/v1/tools/fetch',
      '/v1/moderations',
      '/v1/images',
      '/v1/audio',
      '/v1/messages',
      '/v1beta/models',
      '/v1/realtime',
      '/v1/videos',
      '/v1/video/generations',
      '/kling/v1',
      '/jimeng',
      '/mj',
      '/suno',
    ]

    for (const prefix of requiredPathPrefixes) {
      assert.ok(
        documentedPaths.some((path) => path.startsWith(prefix)),
        `missing documentation for ${prefix}`
      )
    }
  })

  test('documents both GET and HEAD video content delivery methods', () => {
    const endpoint = apiEndpoints.find((item) => item.id === 'video-content')

    assert.ok(endpoint)
    assert.equal(endpoint.method, 'GET')
    assert.ok(
      endpoint.relatedEndpoints?.some(
        (related) =>
          related.method === 'HEAD' &&
          related.path === '/v1/videos/{task_id}/content'
      )
    )
  })

  test('documents the video resolution tier used by conditional per-second pricing', () => {
    const endpoint = apiEndpoints.find((item) => item.id === 'video-create')

    assert.ok(endpoint)
    assert.ok(
      endpoint.parameters.some(
        (parameter) =>
          parameter.name === 'resolution_name' && parameter.location === 'form'
      )
    )
  })
})

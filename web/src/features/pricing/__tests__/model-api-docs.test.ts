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

import { buildModelApiEndpoints } from '../lib/model-api-docs'
import type { PricingModel } from '../types'

function pricingModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'example-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('model API documentation', () => {
  test('documents DeepSeek tools without requiring a model in the body', () => {
    const endpoints = buildModelApiEndpoints(
      pricingModel({
        supported_endpoint_types: [
          'deepseek-tools-search',
          'deepseek-tools-fetch',
        ],
      }),
      {}
    )
    assert.deepEqual(
      endpoints.map((endpoint) => endpoint.path),
      ['/v1/tools/search', '/v1/tools/fetch']
    )
    for (const endpoint of endpoints) {
      assert.ok(
        !endpoint.parameters.some((parameter) => parameter.name === 'model')
      )
      assert.ok(!Object.hasOwn(endpoint.requestExample.json ?? {}, 'model'))
      assert.doesNotThrow(() => JSON.parse(endpoint.responseExample))
    }
  })
  test('uses each model configured endpoint instead of the global fallback', () => {
    const model = pricingModel({
      supported_endpoint_types: ['openai'],
      supported_endpoints: {
        openai: { path: '/custom/chat', method: 'PUT' },
      },
    })

    const endpoints = buildModelApiEndpoints(model, {
      openai: { path: '/v1/chat/completions', method: 'POST' },
    })

    assert.equal(endpoints.length, 1)
    assert.equal(endpoints[0].path, '/custom/chat')
    assert.equal(endpoints[0].method, 'PUT')
  })

  test('builds different parameter contracts for channel-derived protocols', () => {
    const model = pricingModel({
      model_name: 'multi-protocol-model',
      supported_endpoint_types: ['openai', 'anthropic', 'gemini'],
    })

    const endpoints = buildModelApiEndpoints(model, {})
    const openAI = endpoints.find(
      (endpoint) => endpoint.endpointType === 'openai'
    )
    const anthropic = endpoints.find(
      (endpoint) => endpoint.endpointType === 'anthropic'
    )
    const gemini = endpoints.find(
      (endpoint) => endpoint.endpointType === 'gemini'
    )

    assert.ok(
      openAI?.parameters.some((parameter) => parameter.name === 'messages')
    )
    assert.ok(
      anthropic?.parameters.some(
        (parameter) => parameter.name === 'anthropic-version'
      )
    )
    assert.ok(
      gemini?.parameters.some((parameter) => parameter.name === 'contents')
    )
    assert.equal(
      gemini?.path,
      '/v1beta/models/multi-protocol-model:generateContent'
    )
  })

  test('removes optional features that model metadata does not advertise', () => {
    const model = pricingModel({
      supported_endpoint_types: ['openai'],
      capabilities: ['system_prompt'],
    })

    const [endpoint] = buildModelApiEndpoints(model, {})
    const parameterNames = new Set(
      endpoint.parameters.map((parameter) => parameter.name)
    )

    assert.ok(!parameterNames.has('stream'))
    assert.ok(!parameterNames.has('tools'))
  })

  test('documents unknown configured endpoint types without inventing a schema', () => {
    const model = pricingModel({
      supported_endpoint_types: ['vendor-native'],
      supported_endpoints: {
        'vendor-native': { path: '/vendor/generate', method: 'PATCH' },
      },
    })

    const [endpoint] = buildModelApiEndpoints(model, {})

    assert.equal(endpoint.title, 'vendor-native')
    assert.deepEqual(
      endpoint.parameters.map((parameter) => parameter.name),
      ['model']
    )
  })
})

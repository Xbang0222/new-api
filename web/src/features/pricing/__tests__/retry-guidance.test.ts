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
import { describe, expect, test } from 'vitest'

import { buildModelApiEndpoints } from '../lib/model-api-docs'

const retryGuidance =
  'Automatic retries stop after cancellation or once response output begins. If a stream is interrupted, it will not be continued by another provider.'

describe('model API retry guidance', () => {
  test.each([
    'openai',
    'openai-response',
    'anthropic',
    'gemini',
    'image-generation',
    'audio-speech',
  ])(
    '%s API tab retains cancellation and partial response retry limits',
    (endpointType) => {
      const endpoints = buildModelApiEndpoints(
        {
          id: 1,
          model_name: 'example-model',
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          enable_groups: ['default'],
          supported_endpoint_types: [endpointType],
          capabilities: ['streaming'],
        },
        {}
      )
      expect(endpoints).toHaveLength(1)
      expect(endpoints[0].notes).toContain(retryGuidance)
    }
  )
})

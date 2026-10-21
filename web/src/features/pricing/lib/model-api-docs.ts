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
import { apiEndpoints } from '@/features/api-docs/data/catalog'
import type {
  ApiEndpoint,
  ApiExample,
  ApiMethod,
  ApiParameter,
} from '@/features/api-docs/types'

import type { PricingEndpointInfo, PricingModel } from '../types'
import { replaceModelInPath } from './model-helpers'

export type ModelApiEndpoint = ApiEndpoint & {
  endpointType: string
}

const CATALOG_ID_BY_ENDPOINT_TYPE: Record<string, string> = {
  openai: 'chat-completions',
  'deepseek-tools-search': 'deepseek-search',
  'deepseek-tools-fetch': 'deepseek-fetch',
  'openai-completions': 'completions',
  'openai-response': 'responses',
  'openai-response-compact': 'responses-compact',
  anthropic: 'claude-messages',
  gemini: 'gemini-generate-content',
  'jina-rerank': 'rerank',
  'image-generation': 'image-generation',
  'image-edits': 'image-edits',
  embeddings: 'embeddings',
  'audio-speech': 'audio-speech',
  'audio-transcriptions': 'audio-transcriptions',
  'audio-translations': 'audio-translations',
  moderations: 'moderations',
  realtime: 'realtime',
  'openai-video': 'video-create',
}

const CATALOG_ENDPOINT_BY_ID = new Map(
  apiEndpoints.map((endpoint) => [endpoint.id, endpoint])
)

const MODEL_PARAMETER: ApiParameter = {
  name: 'model',
  location: 'body',
  type: 'string',
  required: true,
  description: 'The model ID returned by the model list endpoint.',
}

const ALPHA_SEARCH_ENDPOINT: ApiEndpoint = {
  kind: 'endpoint',
  id: 'alpha-search',
  group: 'capabilities',
  title: 'Web Search',
  summary: 'Search the public web at inference time',
  description:
    'Only send optional fields when the selected model supports them.',
  method: 'POST',
  path: '/v1/alpha/search',
  auth: 'bearer',
  contentType: 'application/json',
  parameters: [
    MODEL_PARAMETER,
    {
      name: 'id',
      location: 'body',
      type: 'string',
      required: false,
      description: 'Request ID',
    },
  ],
  requestExample: {
    json: {
      model: 'your-search-model',
      id: 'search_request_example',
    },
  },
  responseExample: `{
  "type": "computer_initialize_state",
  "id": "search_request_example"
}`,
  responseDescription: 'Response',
}

function endpointMethod(
  method: string | undefined,
  fallback: ApiMethod
): ApiMethod {
  const normalized = method?.trim().toUpperCase()
  if (
    normalized === 'GET' ||
    normalized === 'HEAD' ||
    normalized === 'POST' ||
    normalized === 'PUT' ||
    normalized === 'PATCH' ||
    normalized === 'DELETE'
  ) {
    return normalized
  }
  return fallback
}

function replaceExampleModel(
  example: ApiExample,
  modelName: string
): ApiExample {
  const json = example.json ? { ...example.json } : undefined
  const form = example.form ? { ...example.form } : undefined

  if (json && Object.hasOwn(json, 'model')) {
    json.model = modelName
  }
  if (form && Object.hasOwn(form, 'model')) {
    form.model = modelName
  }

  return {
    ...example,
    path: example.path
      ? replaceModelInPath(example.path, modelName).replaceAll(
          /your-[a-z-]*model/g,
          modelName
        )
      : undefined,
    json,
    form,
  }
}

function filterParametersForCapabilities(
  parameters: ApiParameter[],
  model: PricingModel,
  endpointType: string
): ApiParameter[] {
  const capabilities = model.capabilities
  if (!capabilities || capabilities.length === 0) {
    return parameters
  }

  const supported = new Set(capabilities)
  return parameters.filter((parameter) => {
    if (parameter.name === 'stream') {
      return supported.has('streaming')
    }
    if (parameter.name === 'tools' || parameter.name === 'tool_choice') {
      return supported.has('tools') || supported.has('function_calling')
    }
    if (
      parameter.name === 'response_format' &&
      (endpointType === 'openai' || endpointType === 'openai-response')
    ) {
      return supported.has('json_mode') || supported.has('structured_output')
    }
    return true
  })
}

function genericEndpoint(
  endpointType: string,
  info: PricingEndpointInfo
): ApiEndpoint {
  return {
    kind: 'endpoint',
    id: `custom-${endpointType}`,
    group: 'capabilities',
    title: endpointType,
    summary: 'Provider-specific endpoint, account, and compatibility settings.',
    description:
      'Only send optional fields when the selected model supports them.',
    method: endpointMethod(info.method, 'POST'),
    path: info.path,
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [MODEL_PARAMETER],
    requestExample: { json: { model: 'your-model' } },
    responseExample: '{}',
    responseDescription: 'Response',
  }
}

function resolveEndpointInfo(
  model: PricingModel,
  endpointType: string,
  fallbackMap: Record<string, PricingEndpointInfo>,
  catalogEndpoint?: ApiEndpoint
): PricingEndpointInfo | undefined {
  return (
    model.supported_endpoints?.[endpointType] ??
    fallbackMap[endpointType] ??
    (catalogEndpoint
      ? { path: catalogEndpoint.path, method: catalogEndpoint.method }
      : undefined)
  )
}

export function buildModelApiEndpoints(
  model: PricingModel,
  fallbackMap: Record<string, PricingEndpointInfo>
): ModelApiEndpoint[] {
  const endpointTypes = [...new Set(model.supported_endpoint_types ?? [])]

  return endpointTypes.flatMap((endpointType) => {
    const catalogId = CATALOG_ID_BY_ENDPOINT_TYPE[endpointType]
    let catalogEndpoint: ApiEndpoint | undefined
    if (endpointType === 'openai-alpha-search') {
      catalogEndpoint = ALPHA_SEARCH_ENDPOINT
    } else if (catalogId) {
      catalogEndpoint = CATALOG_ENDPOINT_BY_ID.get(catalogId)
    }
    const info = resolveEndpointInfo(
      model,
      endpointType,
      fallbackMap,
      catalogEndpoint
    )
    if (!info?.path) {
      return []
    }

    const base = catalogEndpoint ?? genericEndpoint(endpointType, info)
    const configuredPath = replaceModelInPath(info.path, model.model_name)
    const usesConfiguredPath =
      configuredPath !== replaceModelInPath(base.path, model.model_name)

    return [
      {
        ...base,
        endpointType,
        method: endpointMethod(info.method, base.method),
        path: configuredPath,
        parameters: filterParametersForCapabilities(
          base.parameters,
          model,
          endpointType
        ),
        requestExample: replaceExampleModel(
          base.requestExample,
          model.model_name
        ),
        responseExample: base.responseExample.replaceAll(
          /your-[a-z-]*model/g,
          model.model_name
        ),
        relatedEndpoints: usesConfiguredPath
          ? undefined
          : base.relatedEndpoints?.map((related) => ({
              ...related,
              path: replaceModelInPath(related.path, model.model_name),
            })),
      },
    ]
  })
}

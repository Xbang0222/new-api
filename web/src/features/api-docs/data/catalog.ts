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
import type {
  ApiDocGroup,
  ApiDocItem,
  ApiEndpoint,
  ApiParameter,
} from '../types'

const modelParameter: ApiParameter = {
  name: 'model',
  location: 'body',
  type: 'string',
  required: true,
  description: 'The model ID returned by the model list endpoint.',
}

const streamParameter: ApiParameter = {
  name: 'stream',
  location: 'body',
  type: 'boolean',
  required: false,
  description:
    'When true, the response is delivered as a server-sent event stream.',
}

const relayRetryNote =
  'Automatic retries stop after cancellation or once response output begins. If a stream is interrupted, it will not be continued by another provider.'

const taskIdParameter: ApiParameter = {
  name: 'task_id',
  location: 'path',
  type: 'string',
  required: true,
  description: 'The task ID returned when the asynchronous job was created.',
}

export const apiDocGroups: ApiDocGroup[] = [
  { id: 'start', title: 'Get started' },
  { id: 'text', title: 'Text generation' },
  { id: 'capabilities', title: 'Model capabilities' },
  { id: 'media', title: 'Image and audio' },
  { id: 'native', title: 'Native provider APIs' },
  { id: 'tasks', title: 'Video and async tasks' },
]

const guides: ApiDocItem[] = [
  {
    kind: 'guide',
    id: 'overview',
    group: 'start',
    title: 'API overview',
    summary: 'Make your first request and understand how the gateway works.',
  },
  {
    kind: 'guide',
    id: 'authentication',
    group: 'start',
    title: 'Authentication',
    summary: 'Send API keys safely with each supported protocol.',
  },
  {
    kind: 'guide',
    id: 'errors',
    group: 'start',
    title: 'Errors and retries',
    summary:
      'Provider failures return a generic model-unavailable error without exposing upstream account details.',
  },
]

export const apiEndpoints: ApiEndpoint[] = [
  {
    kind: 'endpoint',
    id: 'deepseek-search',
    group: 'capabilities',
    title: 'DeepSeek Search',
    summary: 'Search the web with DeepSeek.',
    description: 'Search the web with DeepSeek.',
    method: 'POST',
    path: '/v1/tools/search',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'query',
        location: 'body',
        type: 'string',
        required: true,
        description: 'Search text: 1–128 characters, not only whitespace.',
      },
      {
        name: 'count',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Result limit: 1–50; defaults to 10.',
      },
      {
        name: 'ori_question',
        location: 'body',
        type: 'string | null',
        required: false,
        description: 'Original question: 1–128 characters, or null.',
      },
      {
        name: 'localization',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Optional IP address for localized results.',
      },
      {
        name: 'need_content',
        location: 'body',
        type: 'boolean',
        required: false,
        description: 'Include page content; defaults to false.',
      },
    ],
    requestExample: {
      json: { query: 'new-api documentation', count: 10, need_content: false },
    },
    responseExample: `{
  "results": [{
    "title": "new-api documentation",
    "url": "https://docs.newapi.ai/",
    "snippet": "Documentation for the new-api gateway.",
    "summary": "Setup and API reference for the new-api gateway.",
    "content": "",
    "rerank_score": 2.5,
    "authority": 3,
    "site_name": "new-api",
    "site_icon": null,
    "date": null
  }]
}`,
    responseDescription:
      'Returns results with URLs, summaries and relevance scores; results may be empty.',
    notes: [
      'Send JSON without model or stream. Retries are separate requests and may be billed again.',
      'Successful searches with results and fetches with page content are billed per request.',
      relayRetryNote,
    ],
    relatedEndpoints: [
      {
        method: 'POST',
        path: '/v1/tools/fetch',
        description: 'Fetch a web page as Markdown with DeepSeek.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'deepseek-fetch',
    group: 'capabilities',
    title: 'DeepSeek Fetch',
    summary: 'Fetch a web page as Markdown with DeepSeek.',
    description: 'Fetch a web page as Markdown with DeepSeek.',
    method: 'POST',
    path: '/v1/tools/fetch',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'url',
        location: 'body',
        type: 'string',
        required: true,
        description:
          'HTTP or HTTPS URL with a host, up to 8192 characters; no username or password.',
      },
    ],
    requestExample: { json: { url: 'https://docs.newapi.ai/' } },
    responseExample: `{
  "title": "new-api documentation",
  "content": "# new-api\\n\\nGateway setup and API reference."
}`,
    responseDescription: 'Returns the page title and Markdown content.',
    notes: [
      'Send JSON without model or stream. Retries are separate requests and may be billed again.',
      'Successful searches with results and fetches with page content are billed per request.',
      relayRetryNote,
    ],
    relatedEndpoints: [
      {
        method: 'POST',
        path: '/v1/tools/search',
        description: 'Search the web with DeepSeek.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'list-models',
    group: 'start',
    title: 'List available models',
    summary: 'Return the models this API key can use right now.',
    description:
      'The result is filtered by enabled channels, the key group, model restrictions, and billing configuration. Use supported_endpoint_types to choose the correct API for each model.',
    method: 'GET',
    path: '/v1/models',
    auth: 'bearer',
    parameters: [],
    requestExample: {},
    responseExample: `{
  "success": true,
  "object": "list",
  "data": [
    {
      "id": "your-chat-model",
      "object": "model",
      "created": 1626777600,
      "owned_by": "provider",
      "supported_endpoint_types": ["openai", "openai-response"]
    }
  ]
}`,
    responseDescription:
      'Model availability is dynamic. Query this endpoint instead of hard-coding a catalog in your application.',
    notes: [
      'Add x-api-key and anthropic-version to receive the Anthropic model-list shape.',
      'Use GET /v1beta/models for the native Gemini model-list shape.',
    ],
    relatedEndpoints: [
      {
        method: 'GET',
        path: '/v1/models/{model}',
        description: 'Retrieve one model in the OpenAI or Anthropic shape.',
      },
      {
        method: 'GET',
        path: '/v1beta/models',
        description: 'List models in the native Gemini shape.',
      },
      {
        method: 'GET',
        path: '/v1beta/openai/models',
        description:
          'List models in OpenAI shape under the Gemini-compatible prefix.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'chat-completions',
    group: 'text',
    title: 'Create a chat completion',
    summary: 'Generate a response from a conversation using the OpenAI format.',
    description:
      'This is the broadest compatibility endpoint and works with most chat models. Send the conversation in messages; use stream for incremental output and tools when the selected model supports function calling.',
    method: 'POST',
    path: '/v1/chat/completions',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'messages',
        location: 'body',
        type: 'array',
        required: true,
        description: 'The conversation so far, ordered from oldest to newest.',
      },
      streamParameter,
      {
        name: 'max_completion_tokens',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Maximum number of tokens the model may generate.',
      },
      {
        name: 'temperature',
        location: 'body',
        type: 'number',
        required: false,
        description:
          'Controls randomness. Lower values are more deterministic.',
      },
      {
        name: 'tools',
        location: 'body',
        type: 'array',
        required: false,
        description: 'Tools the model may call when tool calling is supported.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-chat-model',
        messages: [
          { role: 'system', content: 'You are a concise assistant.' },
          {
            role: 'user',
            content: 'Explain vector databases in one sentence.',
          },
        ],
        temperature: 0.2,
      },
    },
    responseExample: `{
  "id": "chatcmpl_example",
  "object": "chat.completion",
  "created": 1750000000,
  "model": "your-chat-model",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "A vector database stores and searches embeddings by semantic similarity."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 22,
    "completion_tokens": 13,
    "total_tokens": 35
  }
}`,
    responseDescription:
      'The response follows the OpenAI Chat Completions schema. Streaming requests return data events and end with data: [DONE].',
  },
  {
    kind: 'endpoint',
    id: 'responses',
    group: 'text',
    title: 'Create a response',
    summary:
      'Use the OpenAI Responses API for tools, reasoning, and multimodal input.',
    description:
      'Responses is the preferred API for newer OpenAI-style models. It supports plain text or structured input items, multi-turn continuation, built-in tools, reasoning controls, and typed streaming events.',
    method: 'POST',
    path: '/v1/responses',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'input',
        location: 'body',
        type: 'string | array',
        required: true,
        description: 'Text or structured input items for the model.',
      },
      {
        name: 'instructions',
        location: 'body',
        type: 'string',
        required: false,
        description: 'High-level instructions applied to this response.',
      },
      streamParameter,
      {
        name: 'previous_response_id',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Continues from an earlier response when the upstream supports it.',
      },
      {
        name: 'max_output_tokens',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'The maximum number of output tokens for this response.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-response-model',
        instructions: 'Answer in plain language.',
        input: 'What does an API gateway do?',
      },
    },
    responseExample: `{
  "id": "resp_example",
  "object": "response",
  "status": "completed",
  "model": "your-response-model",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {"type": "output_text", "text": "An API gateway provides one controlled entry point to multiple services."}
      ]
    }
  ],
  "usage": {
    "input_tokens": 10,
    "output_tokens": 16,
    "total_tokens": 26
  }
}`,
    responseDescription:
      'Non-streaming requests return a response object. For streaming, dispatch events by their type instead of treating every event as a text delta.',
  },
  {
    kind: 'endpoint',
    id: 'responses-compact',
    group: 'text',
    title: 'Compact a response context',
    summary:
      'Reduce a long Responses conversation into a smaller reusable context.',
    description:
      'Use compaction before a conversation grows beyond the practical context window. Availability depends on the selected model exposing the openai-response-compact endpoint type.',
    method: 'POST',
    path: '/v1/responses/compact',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'input',
        location: 'body',
        type: 'array',
        required: false,
        description: 'The response input items that should be compacted.',
      },
      {
        name: 'previous_response_id',
        location: 'body',
        type: 'string',
        required: false,
        description: 'The earlier response whose context should be compacted.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-response-model',
        input: [{ role: 'user', content: 'Long conversation content...' }],
      },
    },
    responseExample: `{
  "id": "resp_compact_example",
  "object": "response.compaction",
  "output": [
    {"type": "compaction", "encrypted_content": "..."}
  ]
}`,
    responseDescription:
      'The compacted output is intended to be passed back to a compatible Responses request, not displayed directly to an end user.',
  },
  {
    kind: 'endpoint',
    id: 'completions',
    group: 'text',
    title: 'Create a text completion',
    summary: 'Generate text from a prompt using the legacy Completions format.',
    description:
      'This endpoint exists for older SDKs and completion-only models. New integrations should prefer Chat Completions or Responses unless a model specifically requires this format.',
    method: 'POST',
    path: '/v1/completions',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'prompt',
        location: 'body',
        type: 'string | array',
        required: true,
        description: 'The prompt or prompts to complete.',
      },
      streamParameter,
      {
        name: 'max_tokens',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Maximum number of tokens the model may generate.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-completion-model',
        prompt: 'Once upon a time',
        max_tokens: 128,
      },
    },
    responseExample: `{
  "id": "cmpl_example",
  "object": "text_completion",
  "choices": [
    {"index": 0, "text": ", a small observatory watched the stars.", "finish_reason": "stop"}
  ],
  "usage": {"prompt_tokens": 4, "completion_tokens": 10, "total_tokens": 14}
}`,
    responseDescription:
      'The generated text is returned in choices[].text. Streaming uses the same SSE termination convention as Chat Completions.',
  },
  {
    kind: 'endpoint',
    id: 'embeddings',
    group: 'capabilities',
    title: 'Create embeddings',
    summary: 'Convert text into vectors for search, clustering, and retrieval.',
    description:
      'Send one string or an array of strings. Store the returned vectors together with the model name and dimensions because vectors from different models are not interchangeable.',
    method: 'POST',
    path: '/v1/embeddings',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'input',
        location: 'body',
        type: 'string | array',
        required: true,
        description: 'One text value or an array of text values to embed.',
      },
      {
        name: 'dimensions',
        location: 'body',
        type: 'integer',
        required: false,
        description:
          'Requested vector size when the selected model supports it.',
      },
      {
        name: 'encoding_format',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Vector encoding format, commonly float or base64.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-embedding-model',
        input: ['first document', 'second document'],
        encoding_format: 'float',
      },
    },
    responseExample: `{
  "object": "list",
  "data": [
    {"object": "embedding", "index": 0, "embedding": [0.012, -0.031, 0.008]},
    {"object": "embedding", "index": 1, "embedding": [0.019, -0.024, 0.011]}
  ],
  "model": "your-embedding-model",
  "usage": {"prompt_tokens": 4, "total_tokens": 4}
}`,
    responseDescription:
      'Each result keeps the input index, so batched vectors can be matched to the original text deterministically.',
    relatedEndpoints: [
      {
        method: 'POST',
        path: '/v1/engines/{model}/embeddings',
        description: 'Legacy model-in-path compatibility route.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'rerank',
    group: 'capabilities',
    title: 'Rerank documents',
    summary: 'Order candidate documents by relevance to a query.',
    description:
      'Reranking is usually the second stage of retrieval: first fetch a wider candidate set, then send those documents here to improve the final ordering before prompting a model.',
    method: 'POST',
    path: '/v1/rerank',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'query',
        location: 'body',
        type: 'string',
        required: true,
        description: 'The query used to score each document.',
      },
      {
        name: 'documents',
        location: 'body',
        type: 'array',
        required: true,
        description: 'Candidate strings or document objects to rerank.',
      },
      {
        name: 'top_n',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Return only the highest-ranked N documents.',
      },
      {
        name: 'return_documents',
        location: 'body',
        type: 'boolean',
        required: false,
        description: 'Include the original document content in each result.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-rerank-model',
        query: 'What is semantic search?',
        documents: [
          'Semantic search compares meaning using embeddings.',
          'A relational database stores rows and columns.',
        ],
        top_n: 2,
        return_documents: true,
      },
    },
    responseExample: `{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.98,
      "document": {"text": "Semantic search compares meaning using embeddings."}
    }
  ],
  "usage": {"total_tokens": 24}
}`,
    responseDescription:
      'Results are sorted from highest to lowest relevance. index points back to the original documents array.',
  },
  {
    kind: 'endpoint',
    id: 'moderations',
    group: 'capabilities',
    title: 'Moderate content',
    summary: 'Classify text or multimodal input for safety categories.',
    description:
      'Use moderation before displaying or forwarding untrusted content. If model is omitted, the gateway applies its default moderation model.',
    method: 'POST',
    path: '/v1/moderations',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      {
        ...modelParameter,
        required: false,
      },
      {
        name: 'input',
        location: 'body',
        type: 'string | array',
        required: true,
        description: 'The content to classify.',
      },
    ],
    requestExample: {
      json: {
        model: 'omni-moderation-latest',
        input: 'Content to classify',
      },
    },
    responseExample: `{
  "id": "modr_example",
  "model": "omni-moderation-latest",
  "results": [
    {"flagged": false, "categories": {}, "category_scores": {}}
  ]
}`,
    responseDescription:
      'Inspect flagged first, then use category-level scores only when the upstream model provides them.',
  },
  {
    kind: 'endpoint',
    id: 'image-generation',
    group: 'media',
    title: 'Generate images',
    summary: 'Create one or more images from a text prompt.',
    description:
      'Image options vary widely by provider. Start with model and prompt, then add size, quality, background, style, or streaming only after confirming the selected model supports them.',
    method: 'POST',
    path: '/v1/images/generations',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: true,
        description: 'A clear description of the image to create.',
      },
      {
        name: 'n',
        location: 'body',
        type: 'integer',
        required: false,
        description:
          'Number of images to generate. The gateway maximum is 128.',
      },
      {
        name: 'size',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Image dimensions written with a lowercase x, for example 1024x1024.',
      },
      {
        name: 'response_format',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'How generated images are returned, commonly url or b64_json.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-image-model',
        prompt: 'A clean isometric illustration of an API gateway',
        n: 1,
        size: '1024x1024',
        response_format: 'url',
      },
    },
    responseExample: `{
  "created": 1750000000,
  "data": [
    {"url": "https://cdn.example.com/generated/image.png"}
  ]
}`,
    responseDescription:
      'Depending on response_format and provider support, each data item contains either a URL or Base64 image data.',
    notes: [
      relayRetryNote,
      'Use 1024x1024, not 1024×1024. The multiplication symbol is rejected.',
      'Model-specific limits may be lower than the gateway limit.',
    ],
  },
  {
    kind: 'endpoint',
    id: 'image-edits',
    group: 'media',
    title: 'Edit an image',
    summary: 'Transform an input image using a natural-language instruction.',
    description:
      'Send multipart form data for local files. JSON requests are also accepted for providers that use image URLs or Base64 data. A mask is optional when the model supports inpainting.',
    method: 'POST',
    path: '/v1/images/edits',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'multipart/form-data',
    parameters: [
      { ...modelParameter, location: 'form' },
      {
        name: 'prompt',
        location: 'form',
        type: 'string',
        required: true,
        description: 'Describe the change to apply to the input image.',
      },
      {
        name: 'image',
        location: 'form',
        type: 'file',
        required: true,
        description:
          'The source image file, URL, or Base64 value supported by the provider.',
      },
      {
        name: 'mask',
        location: 'form',
        type: 'file',
        required: false,
        description: 'Optional mask identifying the region that may be edited.',
      },
      {
        name: 'size',
        location: 'form',
        type: 'string',
        required: false,
        description: 'Requested output dimensions.',
      },
    ],
    requestExample: {
      form: {
        model: 'your-image-edit-model',
        prompt: 'Replace the background with a modern office',
        image: '@input.png',
        n: '1',
        size: '1024x1024',
      },
    },
    responseExample: `{
  "created": 1750000000,
  "data": [
    {"url": "https://cdn.example.com/edited/image.png"}
  ]
}`,
    responseDescription:
      'The response uses the same data array as image generation. The provider determines whether URLs or Base64 content are available.',
    relatedEndpoints: [
      {
        method: 'POST',
        path: '/v1/edits',
        description:
          'Compatibility alias for clients that use the older edit path.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'audio-transcriptions',
    group: 'media',
    title: 'Transcribe audio',
    summary: 'Convert speech in an audio file into text.',
    description:
      'Upload the original audio as multipart form data. Choose json for simple text or a verbose format when timestamps and segments are needed and supported.',
    method: 'POST',
    path: '/v1/audio/transcriptions',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'multipart/form-data',
    parameters: [
      { ...modelParameter, location: 'form' },
      {
        name: 'file',
        location: 'form',
        type: 'file',
        required: true,
        description: 'The audio file to transcribe.',
      },
      {
        name: 'response_format',
        location: 'form',
        type: 'string',
        required: false,
        description: 'Desired output format, such as json or verbose_json.',
      },
      {
        name: 'language',
        location: 'form',
        type: 'string',
        required: false,
        description:
          'Optional input language hint supported by the selected model.',
      },
    ],
    requestExample: {
      form: {
        model: 'your-transcription-model',
        file: '@speech.mp3',
        response_format: 'json',
      },
    },
    responseExample: `{
  "text": "Welcome to the unified AI gateway."
}`,
    responseDescription:
      'Verbose formats may additionally contain language, duration, and timestamped segments.',
  },
  {
    kind: 'endpoint',
    id: 'audio-speech',
    group: 'media',
    title: 'Generate speech',
    summary: 'Turn text into speech and return audio bytes or an audio stream.',
    description:
      'The response is binary audio in the requested format. Set stream_format to sse only when the selected model and channel advertise streaming support.',
    method: 'POST',
    path: '/v1/audio/speech',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'input',
        location: 'body',
        type: 'string',
        required: true,
        description: 'The text that should be spoken.',
      },
      {
        name: 'voice',
        location: 'body',
        type: 'string',
        required: true,
        description: 'Voice ID supported by the selected model.',
      },
      {
        name: 'response_format',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Audio container or codec, for example mp3, wav, or opus.',
      },
      {
        name: 'speed',
        location: 'body',
        type: 'number',
        required: false,
        description: 'Playback speed when the voice model supports adjustment.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-tts-model',
        input: 'Welcome to the unified AI gateway.',
        voice: 'alloy',
        response_format: 'mp3',
        speed: 1,
      },
      binaryResponse: true,
      outputFilename: 'speech.mp3',
    },
    responseExample: 'Binary audio data (audio/mpeg)',
    responseLanguage: 'text',
    responseDescription:
      'Save the response body directly to a file. Do not parse a normal speech response as JSON.',
  },
  {
    kind: 'endpoint',
    id: 'audio-translations',
    group: 'media',
    title: 'Translate audio',
    summary: 'Transcribe audio and translate the spoken content into English.',
    description:
      'This endpoint uses the same multipart upload flow as transcription, but asks the selected audio model to translate the speech. Provider language support and accepted file formats may differ.',
    method: 'POST',
    path: '/v1/audio/translations',
    auth: 'bearer',
    notes: [relayRetryNote],
    contentType: 'multipart/form-data',
    parameters: [
      { ...modelParameter, location: 'form' },
      {
        name: 'file',
        location: 'form',
        type: 'file',
        required: true,
        description: 'The audio file to translate.',
      },
      {
        name: 'response_format',
        location: 'form',
        type: 'string',
        required: false,
        description: 'Desired output format, such as json or verbose_json.',
      },
    ],
    requestExample: {
      form: {
        model: 'your-translation-model',
        file: '@speech.mp3',
        response_format: 'json',
      },
    },
    responseExample: `{
  "text": "The translated English transcript appears here."
}`,
    responseDescription:
      'The simple JSON format returns translated text. Verbose formats may include duration and segment metadata.',
  },
  {
    kind: 'endpoint',
    id: 'claude-messages',
    group: 'native',
    title: 'Create a Claude message',
    summary:
      'Call Anthropic-compatible models with the native Messages format.',
    description:
      'Use this endpoint when your application already speaks the Anthropic protocol or needs Claude-native content blocks, thinking, tools, or cache controls.',
    method: 'POST',
    path: '/v1/messages',
    auth: 'claude',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      {
        name: 'anthropic-version',
        location: 'header',
        type: 'string',
        required: true,
        description:
          'Anthropic protocol version. 2023-06-01 is the common baseline.',
      },
      modelParameter,
      {
        name: 'messages',
        location: 'body',
        type: 'array',
        required: true,
        description:
          'Claude message objects containing user and assistant turns.',
      },
      {
        name: 'max_tokens',
        location: 'body',
        type: 'integer',
        required: true,
        description: 'Maximum number of tokens Claude may generate.',
      },
      streamParameter,
    ],
    requestExample: {
      json: {
        model: 'your-claude-model',
        max_tokens: 1024,
        messages: [
          { role: 'user', content: 'Give me three API security principles.' },
        ],
      },
    },
    responseExample: `{
  "id": "msg_example",
  "type": "message",
  "role": "assistant",
  "model": "your-claude-model",
  "content": [
    {"type": "text", "text": "1. Authenticate every request..."}
  ],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 12, "output_tokens": 38}
}`,
    responseDescription:
      'Claude errors retain the native type/error envelope. Provider failures use HTTP 500 with the generic current-model-unavailable message.',
  },
  {
    kind: 'endpoint',
    id: 'gemini-generate-content',
    group: 'native',
    title: 'Generate Gemini content',
    summary:
      'Call Gemini-compatible models using the native Google request shape.',
    description:
      'The model name lives in the URL. Send contents and parts exactly as a Gemini client would. The same route is available under both v1 and v1beta.',
    method: 'POST',
    path: '/v1beta/models/{model}:generateContent',
    auth: 'gemini',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      {
        name: 'model',
        location: 'path',
        type: 'string',
        required: true,
        description: 'The Gemini model ID selected for this request.',
      },
      {
        name: 'contents',
        location: 'body',
        type: 'array',
        required: true,
        description:
          'Conversation turns expressed as Gemini content and part objects.',
      },
      {
        name: 'generationConfig',
        location: 'body',
        type: 'object',
        required: false,
        description:
          'Generation limits and sampling controls in Gemini camelCase format.',
      },
      {
        name: 'tools',
        location: 'body',
        type: 'array',
        required: false,
        description:
          'Gemini tool declarations supported by the selected model.',
      },
    ],
    requestExample: {
      path: '/v1beta/models/your-gemini-model:generateContent',
      json: {
        contents: [
          {
            role: 'user',
            parts: [{ text: 'Explain zero trust security.' }],
          },
        ],
        generationConfig: {
          temperature: 0.2,
          maxOutputTokens: 512,
        },
      },
    },
    responseExample: `{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [{"text": "Zero trust verifies every access request..."}]
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 7,
    "candidatesTokenCount": 24,
    "totalTokenCount": 31
  }
}`,
    responseDescription:
      'For streaming, replace generateContent with streamGenerateContent and add alt=sse.',
    relatedEndpoints: [
      {
        method: 'POST',
        path: '/v1beta/models/{model}:streamGenerateContent?alt=sse',
        description: 'Stream typed Gemini content events over SSE.',
      },
      {
        method: 'POST',
        path: '/v1/models/{model}:generateContent',
        description:
          'Use the stable v1 prefix with the same native request shape.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'gemini-embeddings',
    group: 'native',
    title: 'Create Gemini embeddings',
    summary: 'Generate a vector with the native Gemini embedContent format.',
    description:
      'Use this endpoint for Gemini SDK compatibility. For a provider-neutral request shape, use POST /v1/embeddings instead.',
    method: 'POST',
    path: '/v1beta/models/{model}:embedContent',
    auth: 'gemini',
    notes: [relayRetryNote],
    contentType: 'application/json',
    parameters: [
      {
        name: 'model',
        location: 'path',
        type: 'string',
        required: true,
        description: 'The Gemini embedding model ID.',
      },
      {
        name: 'content',
        location: 'body',
        type: 'object',
        required: true,
        description:
          'The Gemini content object whose parts contain text to embed.',
      },
      {
        name: 'taskType',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Retrieval task hint used by Gemini embedding models.',
      },
      {
        name: 'outputDimensionality',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Requested vector size when supported.',
      },
    ],
    requestExample: {
      path: '/v1beta/models/your-embedding-model:embedContent',
      json: {
        content: { parts: [{ text: 'Text to embed' }] },
        taskType: 'RETRIEVAL_DOCUMENT',
        outputDimensionality: 768,
      },
    },
    responseExample: `{
  "embedding": {
    "values": [0.012, -0.031, 0.008]
  }
}`,
    responseDescription:
      'Use batchEmbedContents with a requests array when several Gemini-native embeddings are needed at once.',
  },
  {
    kind: 'endpoint',
    id: 'realtime',
    group: 'native',
    title: 'Open a Realtime connection',
    summary: 'Start a bidirectional OpenAI-compatible WebSocket session.',
    description:
      'Realtime is a stateful connection for low-latency audio and event exchange. Keep a client-side event log, handle reconnects, and restore only the state your application can safely replay.',
    method: 'GET',
    path: '/v1/realtime',
    auth: 'websocket',
    parameters: [
      {
        name: 'model',
        location: 'query',
        type: 'string',
        required: true,
        description: 'A model that advertises the realtime endpoint type.',
      },
      {
        name: 'Sec-WebSocket-Protocol',
        location: 'header',
        type: 'string',
        required: false,
        description:
          'Browser-compatible subprotocol carrying the API key and realtime protocol marker.',
      },
    ],
    requestExample: {
      query: { model: 'your-realtime-model' },
    },
    responseExample: `{
  "type": "session.created",
  "event_id": "event_example",
  "session": {
    "id": "sess_example",
    "model": "your-realtime-model"
  }
}`,
    responseDescription:
      'After the WebSocket upgrade, both directions carry JSON events rather than ordinary HTTP responses.',
    notes: [
      'Browser clients can use the openai-insecure-api-key.sk-... WebSocket subprotocol.',
      'Implement bounded reconnects and do not blindly replay side-effecting events.',
    ],
  },
  {
    kind: 'endpoint',
    id: 'video-create',
    group: 'tasks',
    title: 'Create a video',
    summary: 'Submit an OpenAI/Sora-compatible asynchronous video job.',
    description:
      'Video creation returns before rendering finishes. Persist the task ID immediately, then poll the retrieve endpoint at a reasonable interval until the task succeeds or fails.',
    method: 'POST',
    path: '/v1/videos',
    auth: 'bearer',
    contentType: 'multipart/form-data',
    parameters: [
      { ...modelParameter, location: 'form' },
      {
        name: 'prompt',
        location: 'form',
        type: 'string',
        required: true,
        description:
          'Describe the desired scene, motion, subject, and camera behavior.',
      },
      {
        name: 'seconds',
        location: 'form',
        type: 'integer',
        required: false,
        description:
          'Requested duration. The gateway safety maximum is 3600 seconds.',
      },
      {
        name: 'size',
        location: 'form',
        type: 'string',
        required: false,
        description: 'Requested frame size supported by the model.',
      },
      {
        name: 'resolution_name',
        location: 'form',
        type: 'string',
        required: false,
        description:
          'Requested resolution tier, such as 480p or 720p. When omitted, the configured model default is used.',
      },
      {
        name: 'input_reference',
        location: 'form',
        type: 'file | string',
        required: false,
        description: 'Optional reference image for image-to-video generation.',
      },
    ],
    requestExample: {
      form: {
        model: 'sora-2',
        prompt: 'A slow aerial shot above a futuristic coastal city',
        seconds: '4',
        size: '1280x720',
        resolution_name: '720p',
      },
    },
    responseExample: `{
  "id": "video_task_example",
  "object": "video",
  "status": "queued",
  "model": "sora-2",
  "progress": 0
}`,
    responseDescription:
      'The exact creation envelope follows the selected video adapter, but always retain the returned task identifier for later status and content requests.',
  },
  {
    kind: 'endpoint',
    id: 'video-retrieve',
    group: 'tasks',
    title: 'Retrieve a video task',
    summary: 'Check progress and read the final status of a video job.',
    description:
      'Poll this endpoint with a bounded interval. Stop polling on a terminal success or failure state, and surface the provider failure reason when it is safe to show to the customer.',
    method: 'GET',
    path: '/v1/videos/{task_id}',
    auth: 'bearer',
    parameters: [taskIdParameter],
    requestExample: {
      path: '/v1/videos/video_task_example',
    },
    responseExample: `{
  "id": "video_task_example",
  "object": "video",
  "status": "completed",
  "progress": 100,
  "model": "sora-2",
  "seconds": 4,
  "size": "1280x720"
}`,
    responseDescription:
      'When status is completed, request the content endpoint to download or stream the generated media.',
  },
  {
    kind: 'endpoint',
    id: 'video-remix',
    group: 'tasks',
    title: 'Remix a video',
    summary: 'Create a new video variation from an existing completed video.',
    description:
      'The original video ID binds the remix to the same task lineage and channel. Describe only the changes you want; the provider determines which source-video properties are retained.',
    method: 'POST',
    path: '/v1/videos/{video_id}/remix',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'video_id',
        location: 'path',
        type: 'string',
        required: true,
        description: 'The completed source video ID.',
      },
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: true,
        description: 'Describe how the source video should be changed.',
      },
    ],
    requestExample: {
      path: '/v1/videos/video_task_example/remix',
      json: {
        prompt:
          'Keep the camera movement, but change the scene to a snowy night.',
      },
    },
    responseExample: `{
  "id": "video_remix_example",
  "object": "video",
  "status": "queued",
  "progress": 0
}`,
    responseDescription:
      'A remix is a new asynchronous task. Poll it by its new ID rather than the source video ID.',
  },
  {
    kind: 'endpoint',
    id: 'video-content',
    group: 'tasks',
    title: 'Download video content',
    summary: 'Read the generated video after the task has completed.',
    description:
      'API Key authentication and an authenticated dashboard session are both accepted. When Cloudflare video delivery is enabled, the gateway returns a 307 redirect with a short-lived encrypted token; the Worker streams the upstream bytes without exposing the upstream URL or credentials. Clients must follow redirects and preserve Range headers for resumable downloads.',
    method: 'GET',
    path: '/v1/videos/{task_id}/content',
    auth: 'bearer',
    parameters: [taskIdParameter],
    requestExample: {
      path: '/v1/videos/video_task_example/content',
      binaryResponse: true,
      outputFilename: 'result.mp4',
    },
    responseExample: 'Binary video data (video/mp4)',
    responseLanguage: 'text',
    responseDescription:
      'Write the body to a file or media stream and enable redirect following. A HEAD request is also supported for content metadata and range probing.',
    relatedEndpoints: [
      {
        method: 'HEAD',
        path: '/v1/videos/{task_id}/content',
        description: 'Read content headers without downloading the body.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'kling-video',
    group: 'tasks',
    title: 'Create a Kling video',
    summary:
      'Submit text-to-video or image-to-video jobs with Kling-compatible paths.',
    description:
      'Use text2video when no reference image is needed and image2video when the first frame comes from an image. Both routes share the same API Key, routing, billing, and task tracking system.',
    method: 'POST',
    path: '/kling/v1/videos/text2video',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: true,
        description: 'The video prompt.',
      },
      {
        name: 'image',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Reference image URL or Base64 value for image2video.',
      },
      {
        name: 'duration',
        location: 'body',
        type: 'number',
        required: false,
        description:
          'Requested video duration supported by the configured Kling model.',
      },
      {
        name: 'metadata',
        location: 'body',
        type: 'object',
        required: false,
        description:
          'Provider-specific options such as negative prompt or quality level.',
      },
    ],
    requestExample: {
      json: {
        model: 'kling-v1',
        prompt: 'An astronaut walks across a quiet lunar base.',
        duration: 5,
      },
    },
    responseExample: `{
  "task_id": "kling_task_example",
  "status": "submitted"
}`,
    responseDescription:
      'Query GET /kling/v1/videos/text2video/{task_id}. The image2video route has the matching retrieve path.',
    notes: [
      'Image-to-video submit path: POST /kling/v1/videos/image2video.',
      'Image-to-video status path: GET /kling/v1/videos/image2video/{task_id}.',
    ],
    relatedEndpoints: [
      {
        method: 'GET',
        path: '/kling/v1/videos/text2video/{task_id}',
        description: 'Retrieve a Kling text-to-video task.',
      },
      {
        method: 'POST',
        path: '/kling/v1/videos/image2video',
        description: 'Create a Kling image-to-video task.',
      },
      {
        method: 'GET',
        path: '/kling/v1/videos/image2video/{task_id}',
        description: 'Retrieve a Kling image-to-video task.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'general-video',
    group: 'tasks',
    title: 'Create a general video task',
    summary:
      'Use the gateway-neutral JSON format for configured video providers.',
    description:
      'This route is useful when the client does not need a provider-native shape. Put portable fields at the top level and provider-specific controls in metadata.',
    method: 'POST',
    path: '/v1/video/generations',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      modelParameter,
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: true,
        description: 'The video prompt.',
      },
      {
        name: 'image',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Optional reference image URL or Base64 value.',
      },
      {
        name: 'duration',
        location: 'body',
        type: 'integer',
        required: false,
        description: 'Requested duration in seconds, subject to model limits.',
      },
      {
        name: 'metadata',
        location: 'body',
        type: 'object',
        required: false,
        description:
          'Provider-specific options such as negative prompt or quality level.',
      },
    ],
    requestExample: {
      json: {
        model: 'your-video-model',
        prompt: 'A robot walking through a rainy neon street',
        duration: 5,
        size: '1280x720',
        metadata: { negative_prompt: 'blur, distortion' },
      },
    },
    responseExample: `{
  "task_id": "video_task_example",
  "status": "submitted"
}`,
    responseDescription:
      'Query GET /v1/video/generations/{task_id}. Common result fields include status, progress, url, and a failure reason.',
  },
  {
    kind: 'endpoint',
    id: 'jimeng-video',
    group: 'tasks',
    title: 'Call the Jimeng video API',
    summary: 'Use the Jimeng official Action and Version query format.',
    description:
      'The gateway preserves Jimeng-style query actions for submission and result lookup while applying the same API Key, channel routing, and billing controls as other task APIs.',
    method: 'POST',
    path: '/jimeng/',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'Action',
        location: 'query',
        type: 'string',
        required: true,
        description:
          'Jimeng action, such as CVSync2AsyncSubmitTask or CVSync2AsyncGetResult.',
      },
      {
        name: 'Version',
        location: 'query',
        type: 'string',
        required: true,
        description:
          'Jimeng API version. The compatible route uses 2022-08-31.',
      },
      {
        name: 'req_key',
        location: 'body',
        type: 'string',
        required: true,
        description:
          'The Jimeng capability or model key configured for the request.',
      },
    ],
    requestExample: {
      query: {
        Action: 'CVSync2AsyncSubmitTask',
        Version: '2022-08-31',
      },
      json: {
        req_key: 'your-jimeng-model',
        prompt: 'A paper boat sailing through a miniature city',
      },
    },
    responseExample: `{
  "ResponseMetadata": {
    "RequestId": "request_example",
    "Action": "CVSync2AsyncSubmitTask",
    "Version": "2022-08-31"
  },
  "Result": {
    "TaskId": "jimeng_task_example"
  }
}`,
    responseDescription:
      'Use the get-result action with the returned task ID. The payload otherwise follows the configured Jimeng provider contract.',
  },
  {
    kind: 'endpoint',
    id: 'midjourney',
    group: 'tasks',
    title: 'Submit a Midjourney task',
    summary:
      'Create images and perform follow-up actions through Midjourney Proxy paths.',
    description:
      'The imagine route starts a task. Save result as the task ID, then query the fetch route. Follow-up endpoints cover actions, changes, describe, blend, edits, video, uploads, and InsightFace swap.',
    method: 'POST',
    path: '/mj/submit/imagine',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: true,
        description:
          'The Midjourney prompt including any supported command options.',
      },
      {
        name: 'botType',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Optional bot type understood by the configured Midjourney channel.',
      },
      {
        name: 'notifyHook',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Optional callback URL used by compatible upstream implementations.',
      },
      {
        name: 'state',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Opaque customer state returned with compatible task updates.',
      },
    ],
    requestExample: {
      json: {
        prompt: 'A calm library floating above the clouds --ar 16:9',
        botType: 'MID_JOURNEY',
      },
    },
    responseExample: `{
  "code": 1,
  "description": "Submit success",
  "properties": {},
  "result": "midjourney_task_example"
}`,
    responseDescription:
      'A successful submission returns the task ID in result. Query GET /mj/task/{id}/fetch until the task reaches a terminal state.',
    notes: [
      'The same route set is available under /{mode}/mj for compatible clients.',
      'GET /mj/image/{id} is a direct image route and does not pass through API Key middleware.',
    ],
    relatedEndpoints: [
      {
        method: 'GET',
        path: '/mj/image/{id}',
        description: 'Read the generated image bytes for a completed task.',
      },
      {
        method: 'GET',
        path: '/mj/task/{id}/fetch',
        description: 'Retrieve a Midjourney task and its progress.',
      },
      {
        method: 'GET',
        path: '/mj/task/{id}/image-seed',
        description: 'Read the image seed recorded for a completed task.',
      },
      {
        method: 'POST',
        path: '/mj/submit/action',
        description: 'Run a button action returned by a completed task.',
      },
      {
        method: 'POST',
        path: '/mj/submit/change',
        description: 'Submit a follow-up change for an existing task.',
      },
      {
        method: 'POST',
        path: '/mj/submit/simple-change',
        description: 'Submit a compact follow-up change request.',
      },
      {
        method: 'POST',
        path: '/mj/submit/describe',
        description: 'Describe an uploaded image.',
      },
      {
        method: 'POST',
        path: '/mj/submit/blend',
        description: 'Blend several source images.',
      },
      {
        method: 'POST',
        path: '/mj/submit/shorten',
        description: 'Ask Midjourney to analyze and shorten a prompt.',
      },
      {
        method: 'POST',
        path: '/mj/submit/modal',
        description: 'Submit the value requested by a Midjourney modal.',
      },
      {
        method: 'POST',
        path: '/mj/submit/edits',
        description: 'Edit a Midjourney image with a prompt and mask.',
      },
      {
        method: 'POST',
        path: '/mj/submit/video',
        description: 'Create a video from a supported Midjourney result.',
      },
      {
        method: 'POST',
        path: '/mj/insight-face/swap',
        description: 'Swap a face between two supplied images.',
      },
      {
        method: 'POST',
        path: '/mj/task/list-by-condition',
        description: 'Search tasks using Midjourney Proxy conditions.',
      },
      {
        method: 'POST',
        path: '/mj/submit/upload-discord-images',
        description: 'Upload images for a later Discord-backed task.',
      },
    ],
  },
  {
    kind: 'endpoint',
    id: 'suno',
    group: 'tasks',
    title: 'Submit a Suno task',
    summary: 'Create music tasks and retrieve their asynchronous results.',
    description:
      'The action segment selects the Suno operation supported by the configured channel. Keep the returned ID and use the fetch routes instead of resubmitting after a client timeout.',
    method: 'POST',
    path: '/suno/submit/{action}',
    auth: 'bearer',
    contentType: 'application/json',
    parameters: [
      {
        name: 'action',
        location: 'path',
        type: 'string',
        required: true,
        description: 'The Suno operation configured for the selected channel.',
      },
      {
        name: 'prompt',
        location: 'body',
        type: 'string',
        required: false,
        description:
          'Lyrics, style prompt, or generation instruction required by the action.',
      },
      {
        name: 'model',
        location: 'body',
        type: 'string',
        required: false,
        description: 'Optional Suno model or version selector.',
      },
    ],
    requestExample: {
      path: '/suno/submit/music',
      json: {
        prompt: 'A warm acoustic song about a quiet summer evening',
        model: 'your-suno-model',
      },
    },
    responseExample: `{
  "code": "success",
  "message": "",
  "data": "suno_task_example"
}`,
    responseDescription:
      'Use GET /suno/fetch/{id} for one task or POST /suno/fetch with an IDs array for a batch.',
    relatedEndpoints: [
      {
        method: 'GET',
        path: '/suno/fetch/{id}',
        description: 'Retrieve one Suno task.',
      },
      {
        method: 'POST',
        path: '/suno/fetch',
        description: 'Retrieve several Suno tasks by ID.',
      },
    ],
  },
]

export const apiDocItems: ApiDocItem[] = [...guides, ...apiEndpoints]

export const apiDocItemsById = new Map(
  apiDocItems.map((item) => [item.id, item] as const)
)

export const defaultApiDocItem = guides[0]

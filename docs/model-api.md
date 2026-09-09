# new-api 模型 API 接口文档

> 文档版本：1.0  
> 核对日期：2026-07-22  
> 适用范围：面向 API Key 客户端开放的模型查询、推理、媒体生成和异步任务接口

new-api 是统一的 AI 模型 API 网关。客户端通过一套 API Key，可以按 OpenAI、Anthropic Claude、Google Gemini 等兼容协议访问管理员已配置并启用的上游模型。

本文档以当前项目路由和请求 DTO 为准。实际可用的模型、模型能力、价格、分组权限和限流策略由部署实例的渠道及令牌配置决定，不应仅根据模型名称推断能力。

本文档不包含 `/api/...` 管理/面板接口，也不把仅供已登录面板使用的 `/pg/...` Playground 路由视为公共模型 API。

## 1. 接入信息

### 1.1 基础地址

将下文中的 `BASE_URL` 替换为部署实例地址，例如：

```text
https://api.example.com
```

OpenAI 兼容 SDK 的 `base_url` 通常配置为：

```text
https://api.example.com/v1
```

### 1.2 API Key 鉴权

公共模型接口默认使用 Bearer Token：

```http
Authorization: Bearer sk-your-api-key
```

不同原生协议还支持以下鉴权形式：

| 协议 | 推荐方式 | 兼容方式 |
| --- | --- | --- |
| OpenAI 兼容接口 | `Authorization: Bearer sk-...` | 无 |
| Claude Messages | `x-api-key: sk-...` | `Authorization: Bearer sk-...` |
| Gemini 原生接口 | `x-goog-api-key: sk-...` | `Authorization: Bearer sk-...` 或查询参数 `key` |
| Midjourney Proxy | `Authorization: Bearer sk-...` | `mj-api-secret: sk-...` |
| Realtime WebSocket | `Authorization: Bearer sk-...` | WebSocket 子协议中的 `openai-insecure-api-key.sk-...` |

安全建议：生产环境只使用 HTTPS/WSS；不要把 API Key 写入前端代码、日志或公开仓库。Gemini 的 `?key=` 会进入 URL 和访问日志，仅在客户端无法设置请求头时使用。

### 1.3 通用请求头

```http
Content-Type: application/json
Accept: application/json
Authorization: Bearer sk-your-api-key
```

文件上传接口使用 `multipart/form-data`，由 HTTP 客户端自动生成带 boundary 的 `Content-Type`，不要手工拼接 boundary。

每个 HTTP 响应都会包含：

```http
X-Oneapi-Request-Id: <request-id>
```

排障时应记录该请求 ID，但不要记录完整 API Key 或包含敏感内容的请求体。

## 2. 接口总览

### 2.1 模型与文本推理

| 方法 | 路径 | 协议/用途 | 状态 |
| --- | --- | --- | --- |
| `GET` | `/v1/models` | OpenAI/Claude 格式模型列表 | 可用 |
| `GET` | `/v1/models/{model}` | OpenAI/Claude 格式模型详情 | 可用 |
| `GET` | `/v1beta/models` | Gemini 格式模型列表 | 可用 |
| `GET` | `/v1beta/openai/models` | Gemini 路径下的 OpenAI 格式模型列表 | 可用 |
| `POST` | `/v1/chat/completions` | OpenAI Chat Completions | 可用，支持流式 |
| `POST` | `/v1/completions` | OpenAI Legacy Completions | 可用，支持流式 |
| `POST` | `/v1/responses` | OpenAI Responses API | 可用，支持流式 |
| `POST` | `/v1/responses/compact` | Responses 上下文压缩 | 可用 |
| `POST` | `/v1/messages` | Anthropic Claude Messages | 可用，支持流式 |
| `POST` | `/v1/models/{model}:{action}` | Gemini v1 原生动作 | 可用 |
| `POST` | `/v1beta/models/{model}:{action}` | Gemini v1beta 原生动作 | 可用 |
| `GET` | `/v1/realtime` | OpenAI Realtime WebSocket | 可用 |

Gemini 的 `{action}` 当前请求解析覆盖 `generateContent`、`streamGenerateContent`、`embedContent` 和 `batchEmbedContents`。具体模型是否支持对应动作，应以模型列表、渠道配置和实际调用结果为准。

### 2.2 向量、图像、音频与审核

| 方法 | 路径 | 用途 | 请求类型 |
| --- | --- | --- | --- |
| `POST` | `/v1/embeddings` | OpenAI 格式文本向量 | JSON |
| `POST` | `/v1/engines/{model}/embeddings` | 兼容旧式路径的文本向量 | JSON |
| `POST` | `/v1/rerank` | 文档重排序 | JSON |
| `POST` | `/v1/images/generations` | 图像生成 | JSON |
| `POST` | `/v1/images/edits` | 图像编辑 | JSON 或 multipart |
| `POST` | `/v1/edits` | 兼容图像编辑别名 | JSON 或 multipart |
| `POST` | `/v1/audio/transcriptions` | 语音转文本 | multipart |
| `POST` | `/v1/audio/translations` | 音频翻译 | multipart |
| `POST` | `/v1/audio/speech` | 文本转语音 | JSON |
| `POST` | `/v1/moderations` | 内容审核 | JSON |

### 2.3 视频和异步任务

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/v1/videos` | OpenAI/Sora 兼容视频任务 |
| `GET` | `/v1/videos/{task_id}` | 查询 OpenAI/Sora 视频任务 |
| `GET` | `/v1/videos/{task_id}/content` | 获取或代理视频内容 |
| `HEAD` | `/v1/videos/{task_id}/content` | 获取视频内容响应头，支持断点下载探测 |
| `POST` | `/v1/videos/{video_id}/remix` | 基于既有视频创建 remix |
| `POST` | `/v1/video/generations` | 通用视频生成任务 |
| `GET` | `/v1/video/generations/{task_id}` | 查询通用视频任务 |
| `POST` | `/kling/v1/videos/text2video` | Kling 文生视频 |
| `GET` | `/kling/v1/videos/text2video/{task_id}` | 查询 Kling 文生视频任务 |
| `POST` | `/kling/v1/videos/image2video` | Kling 图生视频 |
| `GET` | `/kling/v1/videos/image2video/{task_id}` | 查询 Kling 图生视频任务 |
| `POST` | `/jimeng/?Action=...&Version=2022-08-31` | 即梦官方格式任务提交/查询 |

### 2.4 Midjourney 与 Suno

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/mj/submit/imagine` | Midjourney 文生图 |
| `POST` | `/mj/submit/action` | 执行图片按钮动作 |
| `POST` | `/mj/submit/change` | 图片变化操作 |
| `POST` | `/mj/submit/simple-change` | 简化变化操作 |
| `POST` | `/mj/submit/describe` | 图片描述 |
| `POST` | `/mj/submit/blend` | 图片混合 |
| `POST` | `/mj/submit/shorten` | 提示词缩短 |
| `POST` | `/mj/submit/modal` | Modal 提交 |
| `POST` | `/mj/submit/edits` | 图片编辑 |
| `POST` | `/mj/submit/video` | 视频生成 |
| `POST` | `/mj/submit/upload-discord-images` | 上传 Discord 图片 |
| `POST` | `/mj/insight-face/swap` | 换脸 |
| `GET` | `/mj/task/{id}/fetch` | 查询任务 |
| `GET` | `/mj/task/{id}/image-seed` | 查询图片 seed |
| `POST` | `/mj/task/list-by-condition` | 按条件查询任务 |
| `GET` | `/mj/image/{id}` | 获取任务图片 |
| `POST` | `/suno/submit/{action}` | Suno 任务提交 |
| `POST` | `/suno/fetch` | 批量查询 Suno 任务 |
| `GET` | `/suno/fetch/{id}` | 查询单个 Suno 任务 |

Midjourney 路由还支持 `/{mode}/mj/...` 形式，接口集合与 `/mj/...` 相同，用于兼容带模式前缀的客户端。

`GET /mj/image/{id}` 在当前路由中位于 API Key 中间件之前，用于直接获取任务图片；部署方应结合对象存储权限、反向代理和业务隐私要求决定是否对公网暴露。其余 Midjourney 路由需要 API Key。

## 3. 查询可用模型

模型列表是运行时动态结果，会同时受到以下因素影响：

- 当前启用渠道及渠道支持的模型；
- API Key 所属用户分组或 Key 指定分组；
- API Key 的模型白名单；
- 模型是否存在有效计费配置；
- 当前部署是否启用允许未定价模型的自用模式。

因此，客户端应在运行时查询模型，不要把仓库中的静态模型名列表当作实例的服务承诺。

### 3.1 OpenAI 格式模型列表

```bash
curl "${BASE_URL}/v1/models" \
  -H "Authorization: Bearer ${API_KEY}"
```

响应示例：

```json
{
  "success": true,
  "object": "list",
  "data": [
    {
      "id": "example-chat-model",
      "object": "model",
      "created": 1626777600,
      "owned_by": "provider",
      "supported_endpoint_types": [
        "openai",
        "openai-response"
      ]
    }
  ]
}
```

`supported_endpoint_types` 表示该模型在当前实例中可路由的接口类型。常见值如下：

| 值 | 对应能力 |
| --- | --- |
| `openai` | `/v1/chat/completions` |
| `openai-completions` | `/v1/completions` |
| `openai-response` | `/v1/responses` |
| `openai-response-compact` | `/v1/responses/compact` |
| `anthropic` | `/v1/messages` |
| `gemini` | Gemini 原生模型动作 |
| `jina-rerank` | `/v1/rerank` |
| `image-generation` | `/v1/images/generations` |
| `image-edits` | `/v1/images/edits` |
| `embeddings` | `/v1/embeddings` |
| `audio-speech` | `/v1/audio/speech` |
| `audio-transcriptions` | `/v1/audio/transcriptions` |
| `audio-translations` | `/v1/audio/translations` |
| `moderations` | `/v1/moderations` |
| `realtime` | `/v1/realtime` |
| `openai-video` | `/v1/videos` |
| `openai-video-retrieve` | `/v1/videos/{task_id}` |
| `openai-video-content` | `/v1/videos/{task_id}/content` |
| `openai-video-remix` | `/v1/videos/{video_id}/remix` |

模型广场另外通过 `GET /api/pricing` 返回的模型级 `supported_endpoints` 显示实际配置的请求路径和 HTTP 方法。该字段不属于 `GET /v1/models` 响应；模型元数据中配置的自定义路径只覆盖对应模型，不会影响其他模型的接口文档。

### 3.2 Claude 格式模型列表

同时提供 `x-api-key` 和 `anthropic-version` 时，`GET /v1/models` 返回 Claude 模型列表格式：

```bash
curl "${BASE_URL}/v1/models" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01"
```

### 3.3 Gemini 格式模型列表

```bash
curl "${BASE_URL}/v1beta/models" \
  -H "x-goog-api-key: ${API_KEY}"
```

## 4. OpenAI Chat Completions

### `POST /v1/chat/completions`

必填字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `model` | string | 模型 ID |
| `messages` | array | 对话消息；FIM 模式使用 `prefix`/`suffix` 时可省略 |

常用可选字段包括 `temperature`、`top_p`、`max_tokens`、`max_completion_tokens`、`stream`、`stream_options`、`tools`、`tool_choice`、`response_format`、`reasoning_effort`、`modalities` 和 `audio`。字段是否生效取决于模型与上游渠道。

请求示例：

```bash
curl "${BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-chat-model",
    "messages": [
      {"role": "system", "content": "You are a concise assistant."},
      {"role": "user", "content": "Explain vector databases in one sentence."}
    ],
    "temperature": 0.2
  }'
```

响应采用 OpenAI Chat Completions 兼容结构：

```json
{
  "id": "chatcmpl-example",
  "object": "chat.completion",
  "created": 1750000000,
  "model": "example-chat-model",
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
}
```

### 流式响应

请求中设置：

```json
{
  "stream": true,
  "stream_options": {
    "include_usage": true
  }
}
```

服务端使用 `text/event-stream` 返回 SSE 数据。客户端应逐个处理 `data:` 事件，并在收到 `data: [DONE]` 或连接正常结束后完成聚合。并非所有上游都支持 `stream_options`；网关仅会在渠道声明支持时透传相关行为。

## 5. OpenAI Responses API

### 5.1 创建响应

`POST /v1/responses`

必填字段为 `model` 和 `input`。支持字符串或输入项数组，并可携带 `instructions`、`tools`、`tool_choice`、`reasoning`、`previous_response_id`、`max_output_tokens`、`temperature`、`top_p`、`stream`、`text` 和 `truncation` 等字段。

```bash
curl "${BASE_URL}/v1/responses" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-response-model",
    "instructions": "Answer in Simplified Chinese.",
    "input": "What is an API gateway?"
  }'
```

非流式响应采用 OpenAI Responses 兼容对象；`stream: true` 时返回 Responses 事件流。客户端应根据事件的 `type` 分发，而不是假定所有事件都是文本增量。

### 5.2 压缩上下文

`POST /v1/responses/compact`

必填字段为 `model`。请求还可包含 `input`、`instructions`、`previous_response_id`、`tools`、`parallel_tool_calls`、`reasoning` 和文本配置等字段。

```json
{
  "model": "example-response-model",
  "input": [
    {"role": "user", "content": "Long conversation content..."}
  ]
}
```

该接口是否可用取决于模型的 `openai-response-compact` 能力和上游实现。

## 6. Claude Messages API

### `POST /v1/messages`

推荐请求头：

```http
x-api-key: sk-your-api-key
anthropic-version: 2023-06-01
content-type: application/json
```

必填字段为 `model` 和 `messages`。生产调用通常还应明确传入 `max_tokens`。支持 `system`、`temperature`、`top_p`、`top_k`、`stop_sequences`、`stream`、`tools`、`tool_choice`、`thinking`、`context_management`、`output_config` 和 `metadata` 等 Claude 兼容字段。

```bash
curl "${BASE_URL}/v1/messages" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-claude-model",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Give me three API security principles."}
    ]
  }'
```

普通响应与流式事件采用 Claude Messages 兼容格式。Claude 请求发生网关错误时，错误结构为：

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "error message (request id: ...)"
  }
}
```

## 7. Gemini 原生 API

### 7.1 生成内容

```bash
curl "${BASE_URL}/v1beta/models/example-gemini-model:generateContent" \
  -H "x-goog-api-key: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "Explain zero trust security."}]
      }
    ],
    "generationConfig": {
      "temperature": 0.2,
      "maxOutputTokens": 512
    }
  }'
```

流式调用使用：

```text
POST /v1beta/models/{model}:streamGenerateContent?alt=sse
```

服务端也将 `generateContent?alt=sse` 识别为流式请求。原生 Gemini 请求还支持 `safetySettings`、`tools`、`toolConfig`、`systemInstruction` 和 `cachedContent` 等字段。

### 7.2 原生向量接口

单条向量：

```text
POST /v1beta/models/{model}:embedContent
```

```json
{
  "model": "models/example-embedding-model",
  "content": {
    "parts": [{"text": "Text to embed"}]
  },
  "taskType": "RETRIEVAL_DOCUMENT",
  "outputDimensionality": 768
}
```

批量向量：

```text
POST /v1beta/models/{model}:batchEmbedContents
```

请求体使用 `requests` 数组，每个元素与单条 `embedContent` 请求结构一致。

## 8. Embeddings 与 Rerank

### 8.1 OpenAI 格式向量

`POST /v1/embeddings`

```bash
curl "${BASE_URL}/v1/embeddings" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-embedding-model",
    "input": ["first document", "second document"],
    "encoding_format": "float"
  }'
```

`input` 可以是字符串或字符串数组。可选字段包括 `dimensions` 和 `user`。响应采用 OpenAI Embeddings 兼容结构，包含 `data[].embedding`、`model` 和 `usage`。

### 8.2 文档重排序

`POST /v1/rerank`

```bash
curl "${BASE_URL}/v1/rerank" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-rerank-model",
    "query": "What is semantic search?",
    "documents": [
      "Semantic search compares meaning using embeddings.",
      "A relational database stores rows and columns."
    ],
    "top_n": 2,
    "return_documents": true
  }'
```

响应核心字段：

```json
{
  "results": [
    {
      "index": 0,
      "relevance_score": 0.98,
      "document": {
        "text": "Semantic search compares meaning using embeddings."
      }
    }
  ],
  "usage": {
    "total_tokens": 24
  }
}
```

## 9. 图像接口

### 9.1 图像生成

`POST /v1/images/generations`

```bash
curl "${BASE_URL}/v1/images/generations" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-image-model",
    "prompt": "A clean isometric illustration of an API gateway",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }'
```

常用字段包括 `model`、`prompt`、`n`、`size`、`quality`、`style`、`response_format`、`background`、`output_format` 和 `stream`。`n` 的网关上限为 128；具体模型通常有更小的上限。

尺寸必须使用小写字母 `x`，例如 `1024x1024`，不能使用乘号 `×`。DALL·E 兼容模型还会执行对应模型的尺寸校验。

### 9.2 图像编辑

`POST /v1/images/edits`

multipart 示例：

```bash
curl "${BASE_URL}/v1/images/edits" \
  -H "Authorization: Bearer ${API_KEY}" \
  -F "model=example-image-edit-model" \
  -F "prompt=Replace the background with a modern office" \
  -F "image=@input.png" \
  -F "n=1" \
  -F "size=1024x1024"
```

接口也接受 JSON 形式的 `image`/`images`、`mask`、`prompt` 和其他兼容参数。上游模型可能要求 URL、Base64 或 multipart 文件中的特定一种形式。

## 10. 音频接口

### 10.1 音频转录

`POST /v1/audio/transcriptions`

```bash
curl "${BASE_URL}/v1/audio/transcriptions" \
  -H "Authorization: Bearer ${API_KEY}" \
  -F "model=example-transcription-model" \
  -F "file=@speech.mp3" \
  -F "response_format=json"
```

### 10.2 音频翻译

`POST /v1/audio/translations` 使用与转录相同的 multipart 结构，语义为将音频内容翻译成目标协议规定的语言。

### 10.3 文本转语音

`POST /v1/audio/speech`

```bash
curl "${BASE_URL}/v1/audio/speech" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "example-tts-model",
    "input": "Welcome to the unified AI gateway.",
    "voice": "alloy",
    "response_format": "mp3",
    "speed": 1.0
  }' \
  --output speech.mp3
```

若模型支持 SSE 音频流，可使用 `stream_format: "sse"`；否则响应体为对应音频媒体内容。

## 11. 内容审核与文本补全

### 11.1 内容审核

`POST /v1/moderations`

```json
{
  "model": "omni-moderation-latest",
  "input": "Content to classify"
}
```

`model` 可省略，此时网关使用默认审核模型。响应采用 OpenAI Moderations 兼容格式。

### 11.2 Legacy Completions

`POST /v1/completions`

```json
{
  "model": "example-completion-model",
  "prompt": "Once upon a time",
  "max_tokens": 128,
  "stream": false
}
```

新应用优先使用 Chat Completions 或 Responses API；该接口主要用于旧 SDK 和仅支持文本补全协议的模型。

## 12. Realtime WebSocket

连接地址：

```text
wss://api.example.com/v1/realtime?model=example-realtime-model
```

浏览器或不便设置 `Authorization` 的 WebSocket 客户端可使用子协议：

```text
realtime, openai-insecure-api-key.sk-your-api-key, openai-beta.realtime-v1
```

连接建立后，客户端与服务端交换 OpenAI Realtime 兼容事件。Realtime 是有状态长连接，客户端应实现心跳、断线重连、事件去重和会话状态恢复策略。

## 13. 视频与异步任务

### 13.1 OpenAI/Sora 兼容视频创建

`POST /v1/videos`

```bash
curl "${BASE_URL}/v1/videos" \
  -H "Authorization: Bearer ${API_KEY}" \
  -F "model=sora-2" \
  -F "prompt=A slow aerial shot above a futuristic coastal city" \
  -F "seconds=4" \
  -F "size=1280x720" \
  -F "resolution_name=720p"
```

Sora 兼容请求要求 `model` 和非空 `prompt`。`size` 表示画面宽高，`resolution_name` 表示计费和模型能力使用的分辨率档位（例如 `480p`、`720p`）；省略时使用模型或渠道默认值。支持 `image`、`images` 或 `input_reference` 作为参考图。视频时长的网关安全上限为 3600 秒；模型通常只允许更小的离散值。

查询任务：

```bash
curl "${BASE_URL}/v1/videos/${TASK_ID}" \
  -H "Authorization: Bearer ${API_KEY}"
```

获取视频：

```bash
curl -L "${BASE_URL}/v1/videos/${TASK_ID}/content" \
  -H "Authorization: Bearer ${API_KEY}" \
  --output result.mp4
```

`/content` 允许 API Key，也允许已登录面板会话访问。启用视频 Cloudflare Worker 后，网关只完成鉴权并返回 `307 Temporary Redirect`；视频字节由 Worker 直接从上游流式传给客户端，不经过网关。重定向地址使用 15 分钟有效的加密令牌，上游 URL 和下载凭据不会暴露给客户端。客户端必须跟随重定向，并应保留 `Range`、`If-Range` 等请求头以支持断点续传。未启用 Worker 时，网关保持兼容并直接代理媒体流。`HEAD` 与 `GET` 使用相同的鉴权和重定向规则。

### 13.2 通用视频任务

`POST /v1/video/generations`

```json
{
  "model": "example-video-model",
  "prompt": "A robot walking through a rainy neon street",
  "image": "https://example.com/reference.png",
  "duration": 5,
  "size": "1280x720",
  "metadata": {
    "negative_prompt": "blur, distortion"
  }
}
```

查询地址为 `GET /v1/video/generations/{task_id}`。异步任务的状态和结果字段会按任务适配器归一化，常见字段包括 `task_id`、`status`、`progress`、`url`/`result_url` 和失败原因。

### 13.3 Kling、即梦与 Suno

这些接口保持各自供应商或社区代理协议的路径结构。它们仍由同一 API Key、分组、模型路由、计费和限流体系控制。供应商扩展参数可通过请求体的 `metadata` 或对应原生字段传入；可用字段以所配置渠道版本为准。

## 14. 错误处理

### 14.1 OpenAI 兼容错误

除 Claude 和异步任务专用错误外，网关错误通常采用以下结构：

```json
{
  "error": {
    "message": "model is required (request id: ...)",
    "type": "new_api_error",
    "param": "",
    "code": "invalid_request"
  }
}
```

`code` 可能是字符串。上游渠道返回的错误状态、账户余额、鉴权信息和响应正文不会透传给客户；只要请求已经发送到上游且最终失败，网关统一返回 HTTP `500`：

```json
{
  "error": {
    "message": "当前模型不可用 (request id: ...)",
    "type": "server_error",
    "param": "",
    "code": "model_unavailable"
  }
}
```

Claude 请求仍使用原生 `type/error` 外层结构，但上游失败的消息同样固定为“当前模型不可用”。异步任务提交遇到上游失败时返回 HTTP `500`、`code=model_unavailable`，已落库的失败任务也只向客户展示该通用原因。管理员可在错误日志的私有诊断区域查看实际渠道、上游状态码和经过脱敏及长度限制的响应正文。

流式请求若在首个响应字节之前失败，会正常返回 HTTP `500`；如果流已经开始，HTTP 状态无法再修改，网关会追加协议兼容的通用错误事件并结束流，不会转发上游错误事件正文。

常见网关错误码包括：

| HTTP 状态 | 常见含义 | 客户端建议 |
| --- | --- | --- |
| `400` | JSON/字段无效、缺少模型、模型参数不受支持 | 修正请求，不要原样重试 |
| `401` | API Key 缺失或无效 | 检查鉴权配置 |
| `403` | 用户/Key 被禁用、IP 或模型权限受限 | 检查账户与令牌策略 |
| `404` | 路径不存在 | 检查基础地址和接口路径 |
| `413` | 请求体超过实例限制 | 缩小文件或请求体 |
| `429` | 网关自身限流或客户额度不足 | 指数退避，并检查客户账户额度 |
| `500` | 网关内部处理失败，或上游模型最终不可用 | 记录请求 ID 后进行有界重试或联系管理员 |
| `502`/`503` | 网关在建立上游请求前无法完成路由或服务暂不可用 | 有界指数退避 |
| `501` | 路由保留但尚未实现 | 不要重试，改用已支持接口 |

网关本地校验产生的异步任务错误采用：

```json
{
  "code": "invalid_request",
  "message": "prompt is required",
  "data": null
}
```

### 14.2 重试建议

- 仅对 `429`、`502`、`503` 和 `model_unavailable` 类型的 `500` 使用有界指数退避，并加入随机抖动。
- 尊重网关返回的 `Retry-After`（如有）；上游错误响应头不会直接暴露。
- 网关只会在请求仍有效、响应内容尚未开始时自动尝试其他渠道。流式响应仅发送保活注释时仍可能切换；一旦开始返回模型事件或内容，发生中断后不会拼接其他渠道的响应。实时连接升级后也不会自动重连到其他渠道。
- 使用 `auto` 分组时，网关按令牌中保存的分组顺序选择；只有启用跨分组重试才会在已尝试的分组失败或耗尽后切换。未开始请求前没有可用渠道的分组可以跳过。普通失败重试会排除本次请求已尝试的渠道；绑定原任务渠道的后续操作仍遵循原任务的渠道约束。
- 同步只读查询可以安全重试；创建图像、视频、音乐等任务时，连接中断后盲目重试可能产生重复任务和重复计费。
- 当前公开契约未声明通用幂等键。业务方应保存任务 ID，并在重试创建任务前先确认第一次请求结果。

## 15. 参数、计费与兼容性说明

- `max_tokens`、`max_completion_tokens`、`max_output_tokens` 等字段有网关安全上限，过大的值会在请求进入计费前被拒绝。
- 图像数量 `n` 最大为 128；视频时长最大为 3600 秒。上游模型可以施加更严格的约束。
- 可选参数的显式零值会尽量保留并转发；未提供的字段会按协议或渠道默认值处理。
- `service_tier` 默认会被过滤，除非渠道明确允许，以避免意外增加费用。
- Claude 的 `inference_geo` 和 `speed` 默认会被过滤，除非渠道明确允许。
- `safety_identifier` 默认会被过滤；`store` 默认允许透传，但管理员可以在渠道级禁用。
- 兼容协议不等于所有供应商功能完全一致。工具调用、结构化输出、推理、缓存、音视频、多模态和流式 usage 等能力都应以目标模型的实际能力为准。
- 网关可能根据管理员配置进行模型映射、参数覆盖、请求头覆盖及多渠道重试。上游失败统一收敛为 `500 model_unavailable`，客户端应保存请求 ID 供管理员关联私有上游诊断日志。

## 16. 尚未实现的保留路由

以下路由已注册，但固定返回 HTTP `501` 和 `api_not_implemented`，不得视为可用能力：

- `/v1/images/variations`
- `/v1/files`、`/v1/files/{id}`、`/v1/files/{id}/content`
- `/v1/fine-tunes`、`/v1/fine-tunes/{id}`、取消和事件接口
- `DELETE /v1/models/{model}`

示例响应：

```json
{
  "error": {
    "message": "API not implemented",
    "type": "new_api_error",
    "param": "",
    "code": "api_not_implemented"
  }
}
```

## 17. SDK 配置示例

### OpenAI 兼容 SDK

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",
    base_url="https://api.example.com/v1",
)

response = client.chat.completions.create(
    model="example-chat-model",
    messages=[{"role": "user", "content": "Hello"}],
)

print(response.choices[0].message.content)
```

### Anthropic SDK

```python
from anthropic import Anthropic

client = Anthropic(
    api_key="sk-your-api-key",
    base_url="https://api.example.com",
)

message = client.messages.create(
    model="example-claude-model",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}],
)

print(message.content)
```

不同 SDK 版本对 `base_url` 是否自动附加 `/v1` 的行为可能不同。首次接入时应通过抓包或服务端访问日志确认最终请求路径，避免出现 `/v1/v1/...`。

## 18. 机器可读规范

仓库同时提供 OpenAPI 3.0 规范：

- `docs/openapi/relay.json`：模型 Relay API；
- `docs/openapi/api.json`：面板与管理 API。

当前 `relay.json` 覆盖主要标准接口；Midjourney、Suno、部分 Gemini 动作和部分视频兼容路由以本文档及实际路由为准。发布 Swagger/Redoc 时，建议把部署域名作为运行时 Server URL 注入，不要在规范中硬编码生产域名。

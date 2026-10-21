package dto

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relaykit/routeexpr"
	"github.com/QuantumNous/new-api/relaykit/routejs"
	"github.com/QuantumNous/new-api/relaykit/types"
)

type ChannelSettings struct {
	TaskPluginKey             string   `json:"task_plugin_key,omitempty"`
	ForceFormat               bool     `json:"force_format,omitempty"`
	ThinkingToContent         bool     `json:"thinking_to_content,omitempty"`
	Proxy                     string   `json:"proxy"`
	PassThroughBodyEnabled    bool     `json:"pass_through_body_enabled,omitempty"`
	ResponsesWebSocketEnabled bool     `json:"responses_websocket_enabled,omitempty"`
	SystemPrompt              string   `json:"system_prompt,omitempty"`
	SystemPromptOverride      bool     `json:"system_prompt_override,omitempty"`
	TaskExtendPluginKeys      []string `json:"task_extend_plugin_keys,omitempty"`
	// HTTPProtocol controls outbound HTTP version negotiation for this channel.
	// Accepted values: "", "auto" (default), "http1".
	HTTPProtocol string `json:"http_protocol,omitempty"`
	// HTTP2ConnectionShards spreads HTTP/2 traffic across N independent transports
	// (1-8). Zero/unset means 1. Ignored when HTTPProtocol is "http1".
	HTTP2ConnectionShards int `json:"http2_connection_shards,omitempty"`
	// TLSInsecureSkipVerify accepts upstream certificates that fail
	// verification, such as self-signed ones, for this channel only.
	TLSInsecureSkipVerify bool `json:"tls_insecure_skip_verify,omitempty"`
}

// BindsTaskPlugin reports whether the channel is bound to the task plugin.
func (s ChannelSettings) BindsTaskPlugin(key string) bool {
	return key != "" && (s.TaskPluginKey == key || slices.Contains(s.TaskExtendPluginKeys, key))
}

// TaskPluginBindings returns the sorted, de-duplicated task plugin bindings.
func (s ChannelSettings) TaskPluginBindings() []string {
	bindings := make([]string, 0, len(s.TaskExtendPluginKeys)+1)
	if s.TaskPluginKey != "" {
		bindings = append(bindings, s.TaskPluginKey)
	}
	for _, key := range s.TaskExtendPluginKeys {
		if key != "" && !slices.Contains(bindings, key) {
			bindings = append(bindings, key)
		}
	}
	slices.Sort(bindings)
	return bindings
}

const (
	HTTPProtocolAuto                 = "auto"
	HTTPProtocolHTTP1                = "http1"
	MaxHTTP2ConnectionShards         = 8
	MaxChannelRateLimitCount         = 100_000
	MaxChannelRateLimitWindowSeconds = 24 * 60 * 60
)

type ChannelRequestRateLimit struct {
	MaxRequests   int `json:"max_requests"`
	WindowSeconds int `json:"window_seconds"`
}

func (l *ChannelRequestRateLimit) Validate() error {
	if l == nil {
		return nil
	}
	if l.MaxRequests <= 0 || l.MaxRequests > MaxChannelRateLimitCount {
		return fmt.Errorf("request_rate_limit.max_requests must be between 1 and %d", MaxChannelRateLimitCount)
	}
	if l.WindowSeconds <= 0 || l.WindowSeconds > MaxChannelRateLimitWindowSeconds {
		return fmt.Errorf("request_rate_limit.window_seconds must be between 1 and %d", MaxChannelRateLimitWindowSeconds)
	}
	return nil
}

// ValidateHTTPTransport validates save-time HTTP transport channel settings.
func (s *ChannelSettings) ValidateHTTPTransport() error {
	if s == nil {
		return nil
	}
	protocol := strings.ToLower(strings.TrimSpace(s.HTTPProtocol))
	switch protocol {
	case "", HTTPProtocolAuto, HTTPProtocolHTTP1:
	default:
		return fmt.Errorf("invalid http_protocol: %s", s.HTTPProtocol)
	}
	if s.HTTP2ConnectionShards < 0 || s.HTTP2ConnectionShards > MaxHTTP2ConnectionShards {
		return fmt.Errorf("invalid http2_connection_shards: %d", s.HTTP2ConnectionShards)
	}
	if protocol == HTTPProtocolHTTP1 && s.HTTP2ConnectionShards > 1 {
		return fmt.Errorf("http2_connection_shards must be 1 when http_protocol is http1")
	}
	return nil
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type AwsApiType string

const (
	AwsApiTypeInvokeModel AwsApiType = "invoke_model"
	AwsApiTypeMantle      AwsApiType = "mantle"
)

type ChannelOtherSettings struct {
	RequestRateLimit                      *ChannelRequestRateLimit `json:"request_rate_limit,omitempty"`
	AzureResponsesVersion                 string                   `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType            `json:"vertex_key_type,omitempty"`   // "json" or "api_key"
	VertexProjectID                       string                   `json:"vertex_project_id,omitempty"` // 仅当 vertex_key_type=api_key 时使用；JSON 模式下从 service account 解析
	OpenRouterEnterprise                  *bool                    `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool                     `json:"claude_beta_query,omitempty"`          // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool                     `json:"allow_service_tier,omitempty"`         // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool                     `json:"allow_inference_geo,omitempty"`        // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool                     `json:"allow_speed,omitempty"`                // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool                     `json:"allow_safety_identifier,omitempty"`    // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool                     `json:"disable_store,omitempty"`              // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool                     `json:"allow_include_obfuscation,omitempty"`  // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	DisableTaskPollingSleep               bool                     `json:"disable_task_polling_sleep,omitempty"` // 是否跳过异步任务轮询间隔
	AwsKeyType                            AwsKeyType               `json:"aws_key_type,omitempty"`
	AwsApiType                            AwsApiType               `json:"aws_api_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool                     `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool                     `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64                    `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string                 `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string                 `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string                 `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	AdvancedCustom                        *AdvancedCustomConfig    `json:"advanced_custom,omitempty"`
	// OllamaOpenAIChat routes Ollama chat completions to the OpenAI-compatible
	// /v1/chat/completions endpoint. When unset, chat completions keep using
	// the native /api/chat protocol.
	OllamaOpenAIChat bool `json:"ollama_openai_chat,omitempty"`
	// ToolLossPolicy is a channel-level opt-in for request-phase conversion
	// rejection. Empty follows the default allow policy. Accepted values:
	// "", "allow", "safe", "strict".
	ToolLossPolicy string `json:"tool_loss_policy,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}

// ValidateToolLossPolicy validates the channel-level request-phase tool-loss
// policy. Empty keeps the default allow policy.
func (s *ChannelOtherSettings) ValidateToolLossPolicy() error {
	if s == nil {
		return nil
	}
	switch strings.TrimSpace(s.ToolLossPolicy) {
	case "", string(types.ConversionLossPolicyAllow), string(types.ConversionLossPolicySafe), string(types.ConversionLossPolicyStrict):
		return nil
	default:
		return fmt.Errorf("invalid tool_loss_policy: %s", s.ToolLossPolicy)
	}
}

const (
	advancedCustomConverterNone                        = "none"
	AdvancedCustomConverterSGLangRerank                = "jina_rerank_to_sglang"
	advancedCustomConverterClaudeMessagesToOpenAIChat  = "anthropic_messages_to_openai_chat_completions"
	advancedCustomConverterOpenAIChatToClaudeMessages  = "openai_chat_completions_to_anthropic_messages"
	advancedCustomConverterOpenAIChatToOpenAIResponses = "openai_chat_completions_to_openai_responses"
	advancedCustomConverterOpenAIResponsesToOpenAIChat = "openai_responses_to_openai_chat_completions"
	advancedCustomConverterOpenAIResponsesToGemini     = "openai_responses_to_gemini_generate_content"
	advancedCustomConverterGeminiContentToOpenAIChat   = "gemini_generate_content_to_openai_chat_completions"
	advancedCustomConverterOpenAIChatToGeminiContent   = "openai_chat_completions_to_gemini_generate_content"
	// Same values as relayconvert.ConverterClaudeMessagesToOpenAIResponses and
	// relayconvert.ConverterGeminiContentToOpenAIResponses.
	advancedCustomConverterClaudeMessagesToOpenAIResponses = "claude_messages_to_openai_responses"
	advancedCustomConverterGeminiContentToOpenAIResponses  = "gemini_generate_content_to_openai_responses"
)

const (
	AdvancedCustomAuthTypeNone   = "none"
	AdvancedCustomAuthTypeHeader = "header"
	AdvancedCustomAuthTypeQuery  = "query"
)

type AdvancedCustomConfig struct {
	Routes []AdvancedCustomRoute `json:"advanced_routes,omitempty"`
}

type AdvancedCustomRoute struct {
	IncomingPath           string                   `json:"incoming_path,omitempty"`
	UpstreamPath           string                   `json:"upstream_path,omitempty"`
	Method                 string                   `json:"method,omitempty"`
	Converter              string                   `json:"converter,omitempty"`
	Models                 []string                 `json:"models,omitempty"`
	Auth                   *AdvancedCustomRouteAuth `json:"auth,omitempty"`
	Headers                map[string]string        `json:"headers,omitempty"`
	RequestBodyTemplate    json.RawMessage          `json:"request_body_template,omitempty"`
	ResponseBodyTemplate   json.RawMessage          `json:"response_body_template,omitempty"`
	Task                   *AdvancedCustomTask      `json:"task,omitempty"`
	PassThroughBodyEnabled bool                     `json:"pass_through_body_enabled,omitempty"`
}

// IsNative reports whether the route forwards without protocol conversion.
func (r AdvancedCustomRoute) IsNative() bool {
	converter := strings.TrimSpace(r.Converter)
	return converter == "" || converter == advancedCustomConverterNone
}

// SupportsPassThroughBody reports whether the route converter leaves the body untouched.
func (r AdvancedCustomRoute) SupportsPassThroughBody() bool {
	switch strings.TrimSpace(r.Converter) {
	case "", advancedCustomConverterNone, AdvancedCustomConverterSGLangRerank:
		return true
	default:
		return false
	}
}

type AdvancedCustomRouteAuth struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

const (
	AdvancedCustomTaskRequestModePassthrough = "passthrough"
	AdvancedCustomTaskRequestModeTemplate    = "template"
)

// AdvancedCustomTask turns an Advanced Custom route into a configurable
// asynchronous task protocol. The route's UpstreamPath/Auth/Headers describe
// submission; Poll describes subsequent status requests.
type AdvancedCustomTask struct {
	SubmitMethod   string                      `json:"submit_method,omitempty"`
	RequestMode    string                      `json:"request_mode,omitempty"`
	BodyTemplate   json.RawMessage             `json:"body_template,omitempty"`
	HeadersScript  string                      `json:"headers_script,omitempty"`
	BodyScript     string                      `json:"body_script,omitempty"`
	RequestScript  string                      `json:"request_script,omitempty"`
	SubmitResponse AdvancedCustomTaskResponse  `json:"submit_response"`
	Poll           AdvancedCustomTaskPoll      `json:"poll"`
	Download       *AdvancedCustomTaskDownload `json:"download,omitempty"`
}

type AdvancedCustomTaskPoll struct {
	Method        string                     `json:"method,omitempty"`
	UpstreamPath  string                     `json:"upstream_path"`
	Auth          *AdvancedCustomRouteAuth   `json:"auth,omitempty"`
	Headers       map[string]string          `json:"headers,omitempty"`
	BodyTemplate  json.RawMessage            `json:"body_template,omitempty"`
	HeadersScript string                     `json:"headers_script,omitempty"`
	BodyScript    string                     `json:"body_script,omitempty"`
	RequestScript string                     `json:"request_script,omitempty"`
	Response      AdvancedCustomTaskResponse `json:"response"`
}

// AdvancedCustomTaskDownload describes credentials that the media delivery
// worker must use when fetching the result URL. Nil means the result URL is
// already directly downloadable (for example, a presigned CDN URL).
type AdvancedCustomTaskDownload struct {
	Auth    *AdvancedCustomRouteAuth `json:"auth,omitempty"`
	Headers map[string]string        `json:"headers,omitempty"`
}

// AdvancedCustomTaskResponse uses gjson-compatible paths. StatusMap values
// are the gateway's stable task states: SUBMITTED, QUEUED, IN_PROGRESS,
// SUCCESS, or FAILURE.
type AdvancedCustomTaskResponse struct {
	TaskIDPath          string            `json:"task_id_path,omitempty"`
	StatusPath          string            `json:"status_path,omitempty"`
	ProgressPath        string            `json:"progress_path,omitempty"`
	ResultURLPath       string            `json:"result_url_path,omitempty"`
	ErrorPath           string            `json:"error_path,omitempty"`
	StatusMap           map[string]string `json:"status_map,omitempty"`
	ErrorCodePath       string            `json:"error_code_path,omitempty"`
	ErrorMessageMap     map[string]string `json:"error_message_map,omitempty"`
	DefaultErrorMessage string            `json:"default_error_message,omitempty"`
	ResponseScript      string            `json:"response_script,omitempty"`
	Script              string            `json:"script,omitempty"`
}

const (
	advancedCustomModelPlaceholder = "{model}"
	advancedCustomModelRegexPrefix = "re:"
)

const (
	advancedCustomEndpointPathOpenAIChat             = "/v1/chat/completions"
	advancedCustomEndpointPathOpenAIResponses        = "/v1/responses"
	advancedCustomEndpointPathOpenAIResponsesCompact = "/v1/responses/compact"
	advancedCustomEndpointPathOpenAIAlphaSearch      = "/v1/alpha/search"
	advancedCustomEndpointPathDeepSeekToolsSearch    = "/v1/tools/search"
	advancedCustomEndpointPathDeepSeekToolsFetch     = "/v1/tools/fetch"
	advancedCustomEndpointPathClaudeMessages         = "/v1/messages"
	advancedCustomEndpointPathJinaRerank             = "/v1/rerank"
	advancedCustomEndpointPathImageGeneration        = "/v1/images/generations"
	advancedCustomEndpointPathEmbeddings             = "/v1/embeddings"
)

const (
	// AdvancedCustomModelListPath identifies the optional OpenAI Models discovery route.
	AdvancedCustomModelListPath = "/v1/models"
	// AdvancedCustomBalancePath identifies the optional balance lookup route used by channel management.
	AdvancedCustomBalancePath = "/v1/dashboard/billing/credit_grants"
)

// MatchPath returns the first route whose IncomingPath matches requestPath.
// Matching mirrors the relay adaptor: exact match, {model} placeholder, and
// :generateContent <-> :streamGenerateContent equivalence.
func (c *AdvancedCustomConfig) MatchPath(requestPath string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// MatchPathForModel returns the first route whose IncomingPath and Models match.
// An empty Models list is a catch-all fallback for that incoming path.
func (c *AdvancedCustomConfig) MatchPathForModel(requestPath string, model string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	model = strings.TrimSpace(model)
	for _, route := range c.Routes {
		if matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) &&
			matchAdvancedCustomRouteModel(route.Models, model) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// MatchTaskPathForModel returns a configured asynchronous-task route for the
// incoming public path and model.
func (c *AdvancedCustomConfig) MatchTaskPathForModel(requestPath string, model string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	model = strings.TrimSpace(model)
	for _, route := range c.Routes {
		if route.Task != nil &&
			matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) &&
			matchAdvancedCustomRouteModel(route.Models, model) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// MatchTaskForModel is the legacy-task fallback when a task row predates route
// snapshots. New tasks should poll with the exact route saved at submission.
func (c *AdvancedCustomConfig) MatchTaskForModel(model string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	model = strings.TrimSpace(model)
	for _, route := range c.Routes {
		if route.Task != nil && matchAdvancedCustomRouteModel(route.Models, model) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// ModelListRoute returns the explicitly configured OpenAI Models discovery route.
// Template routes that merely happen to match /v1/models are not discovery routes.
func (c *AdvancedCustomConfig) ModelListRoute() (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if strings.TrimSpace(route.IncomingPath) == AdvancedCustomModelListPath {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// BalanceRoute returns the explicitly configured channel-management balance route.
func (c *AdvancedCustomConfig) BalanceRoute() (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if strings.TrimSpace(route.IncomingPath) == AdvancedCustomBalancePath {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// SupportsPath reports whether any route matches requestPath.
func (c *AdvancedCustomConfig) SupportsPath(requestPath string) bool {
	_, ok := c.MatchPath(requestPath)
	return ok
}

// SupportsPathForModel reports whether any route matches requestPath and model.
func (c *AdvancedCustomConfig) SupportsPathForModel(requestPath string, model string) bool {
	_, ok := c.MatchPathForModel(requestPath, model)
	return ok
}

func (c *AdvancedCustomConfig) SupportedEndpointTypesForModel(model string) []types.EndpointType {
	if c == nil {
		return nil
	}
	model = strings.TrimSpace(model)
	endpoints := make([]types.EndpointType, 0, len(c.Routes))
	seen := make(map[types.EndpointType]struct{}, len(c.Routes))
	for _, route := range c.Routes {
		if !matchAdvancedCustomRouteModel(route.Models, model) {
			continue
		}
		endpointType, ok := advancedCustomEndpointTypeFromIncomingPath(strings.TrimSpace(route.IncomingPath))
		if !ok {
			continue
		}
		if _, exists := seen[endpointType]; exists {
			continue
		}
		seen[endpointType] = struct{}{}
		endpoints = append(endpoints, endpointType)
	}
	return endpoints
}

func advancedCustomEndpointTypeFromIncomingPath(incomingPath string) (types.EndpointType, bool) {
	switch incomingPath {
	case advancedCustomEndpointPathOpenAIChat:
		return types.EndpointTypeOpenAI, true
	case advancedCustomEndpointPathOpenAIResponses:
		return types.EndpointTypeOpenAIResponse, true
	case advancedCustomEndpointPathOpenAIResponsesCompact:
		return types.EndpointTypeOpenAIResponseCompact, true
	case advancedCustomEndpointPathOpenAIAlphaSearch:
		return types.EndpointTypeOpenAIAlphaSearch, true
	case advancedCustomEndpointPathDeepSeekToolsSearch:
		return types.EndpointTypeDeepSeekToolsSearch, true
	case advancedCustomEndpointPathDeepSeekToolsFetch:
		return types.EndpointTypeDeepSeekToolsFetch, true
	case advancedCustomEndpointPathClaudeMessages:
		return types.EndpointTypeAnthropic, true
	case advancedCustomEndpointPathJinaRerank:
		return types.EndpointTypeJinaRerank, true
	case advancedCustomEndpointPathImageGeneration:
		return types.EndpointTypeImageGeneration, true
	case advancedCustomEndpointPathEmbeddings:
		return types.EndpointTypeEmbeddings, true
	case "/v1/videos", "/v1/video/generations":
		return types.EndpointTypeOpenAIVideo, true
	default:
		if isAdvancedCustomGeminiIncomingPath(incomingPath) {
			return types.EndpointTypeGemini, true
		}
		return "", false
	}
}

func isAdvancedCustomGeminiIncomingPath(incomingPath string) bool {
	if !strings.HasPrefix(incomingPath, "/v1beta/models/") {
		return false
	}
	return strings.Contains(incomingPath, ":generateContent") || strings.Contains(incomingPath, ":streamGenerateContent")
}

func matchAdvancedCustomRouteModel(models []string, model string) bool {
	normalizedModels := normalizeAdvancedCustomRouteModels(models)
	if len(normalizedModels) == 0 {
		return true
	}
	for _, allowedModel := range normalizedModels {
		if matchAdvancedCustomRouteModelRule(allowedModel, model) {
			return true
		}
	}
	return false
}

// advancedCustomModelRegexCache caches compiled route model patterns. Route model
// matching runs on the request hot path (distributor affinity, ability filtering,
// channel cache filtering, adaptor resolve), so patterns must not be recompiled per
// request. Invalid patterns are cached as nil to avoid recompiling them as well.
var advancedCustomModelRegexCache sync.Map // pattern string -> *regexp.Regexp (nil when invalid)

func compileAdvancedCustomModelRegex(pattern string) *regexp.Regexp {
	if cached, ok := advancedCustomModelRegexCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = nil
	}
	advancedCustomModelRegexCache.Store(pattern, re)
	return re
}

func matchAdvancedCustomRouteModelRule(rule string, model string) bool {
	if !strings.HasPrefix(rule, advancedCustomModelRegexPrefix) {
		return rule == model
	}
	pattern := strings.TrimPrefix(rule, advancedCustomModelRegexPrefix)
	if pattern == "" {
		return false
	}
	re := compileAdvancedCustomModelRegex(pattern)
	return re != nil && re.MatchString(model)
}

func matchAdvancedCustomIncomingPath(configuredPath string, requestPath string) bool {
	if matchAdvancedCustomIncomingPathTemplate(configuredPath, requestPath) {
		return true
	}
	if strings.Contains(configuredPath, ":generateContent") {
		streamPath := strings.Replace(configuredPath, ":generateContent", ":streamGenerateContent", 1)
		return matchAdvancedCustomIncomingPathTemplate(streamPath, requestPath)
	}
	return false
}

func matchAdvancedCustomIncomingPathTemplate(configuredPath string, requestPath string) bool {
	if !strings.Contains(configuredPath, advancedCustomModelPlaceholder) {
		return configuredPath == requestPath
	}

	parts := strings.Split(configuredPath, advancedCustomModelPlaceholder)
	if len(parts) != 2 {
		return false
	}
	if !strings.HasPrefix(requestPath, parts[0]) || !strings.HasSuffix(requestPath, parts[1]) {
		return false
	}

	model := strings.TrimSuffix(strings.TrimPrefix(requestPath, parts[0]), parts[1])
	return model != "" && !strings.Contains(model, "/")
}

func IsAdvancedCustomConverterAllowed(converter string) bool {
	switch converter {
	case advancedCustomConverterNone,
		AdvancedCustomConverterSGLangRerank,
		advancedCustomConverterClaudeMessagesToOpenAIChat,
		advancedCustomConverterOpenAIChatToClaudeMessages,
		advancedCustomConverterOpenAIChatToOpenAIResponses,
		advancedCustomConverterOpenAIResponsesToOpenAIChat,
		advancedCustomConverterOpenAIResponsesToGemini,
		advancedCustomConverterGeminiContentToOpenAIChat,
		advancedCustomConverterOpenAIChatToGeminiContent,
		advancedCustomConverterClaudeMessagesToOpenAIResponses,
		advancedCustomConverterGeminiContentToOpenAIResponses:
		return true
	default:
		return false
	}
}

func (c *AdvancedCustomConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("advanced_custom is required")
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("advanced_custom requires at least one route")
	}

	paths := make(map[string]*advancedCustomPathModelState, len(c.Routes))
	modelListRouteIndex := -1
	balanceRouteIndex := -1
	for i := range c.Routes {
		route := c.Routes[i]
		route.IncomingPath = strings.TrimSpace(route.IncomingPath)
		upstreamPath := strings.TrimSpace(route.UpstreamPath)
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = advancedCustomConverterNone
		}

		if route.IncomingPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path is required", i)
		}
		if !strings.HasPrefix(route.IncomingPath, "/") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must start with /", i)
		}
		if strings.Contains(route.IncomingPath, "?") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must not include query", i)
		}
		if route.IncomingPath == AdvancedCustomModelListPath || route.IncomingPath == AdvancedCustomBalancePath {
			managementRouteName := route.IncomingPath
			if route.PassThroughBodyEnabled {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].pass_through_body_enabled must be false for %s", i, managementRouteName)
			}
			previousIndex := modelListRouteIndex
			if route.IncomingPath == AdvancedCustomBalancePath {
				previousIndex = balanceRouteIndex
			}
			if previousIndex >= 0 {
				return fmt.Errorf("advanced_custom.advanced_routes[%d] duplicates the %s route at advanced_routes[%d]", i, managementRouteName, previousIndex)
			}
			if route.IncomingPath == AdvancedCustomModelListPath {
				modelListRouteIndex = i
			} else {
				balanceRouteIndex = i
			}
			if len(normalizeAdvancedCustomRouteModels(route.Models)) > 0 {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].models must be empty for %s", i, managementRouteName)
			}
			if route.Converter != advancedCustomConverterNone {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter must be none for %s", i, managementRouteName)
			}
			if strings.Contains(upstreamPath, advancedCustomModelPlaceholder) {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must not contain %s for %s", i, advancedCustomModelPlaceholder, managementRouteName)
			}
		}
		if err := validateAdvancedCustomRouteModels(i, route.IncomingPath, route.Models, paths); err != nil {
			return err
		}

		if upstreamPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path is required", i)
		}
		if err := validateAdvancedCustomUpstreamTarget(i, upstreamPath); err != nil {
			return err
		}
		if method := strings.ToUpper(strings.TrimSpace(route.Method)); method != "" && !isAdvancedCustomTaskMethodAllowed(method) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].method is invalid: %s", i, route.Method)
		}
		if route.IncomingPath == AdvancedCustomModelListPath && route.Method != "" && !strings.EqualFold(route.Method, http.MethodGet) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].method must be GET for /v1/models", i)
		}

		if route.Task != nil && route.Converter != advancedCustomConverterNone {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter must be none for task routes", i)
		}
		if route.Task != nil && strings.TrimSpace(route.Method) != "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].method is not used for task routes; use task.submit_method", i)
		}
		if (len(route.RequestBodyTemplate) > 0 || len(route.ResponseBodyTemplate) > 0) && route.Converter != advancedCustomConverterNone {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter must be none for JSON template routes", i)
		}
		if len(route.RequestBodyTemplate) > 0 && !json.Valid(route.RequestBodyTemplate) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].request_body_template must be valid JSON", i)
		}
		if len(route.ResponseBodyTemplate) > 0 && !json.Valid(route.ResponseBodyTemplate) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].response_body_template must be valid JSON", i)
		}
		if route.Task != nil && (len(route.RequestBodyTemplate) > 0 || len(route.ResponseBodyTemplate) > 0) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d] cannot combine synchronous JSON templates with a task protocol", i)
		}
		if !IsAdvancedCustomConverterAllowed(route.Converter) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter is not registered: %s", i, route.Converter)
		}
		if route.PassThroughBodyEnabled && !route.SupportsPassThroughBody() {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].pass_through_body_enabled requires converter none or %s", i, AdvancedCustomConverterSGLangRerank)
		}
		if err := validateAdvancedCustomConverterPath(i, route.IncomingPath, route.Converter); err != nil {
			return err
		}
		if err := validateAdvancedCustomRouteAuth(i, route.Auth); err != nil {
			return err
		}
		if err := validateAdvancedCustomHeaders(i, "headers", route.Headers); err != nil {
			return err
		}
		if route.Task != nil {
			if err := validateAdvancedCustomTask(i, route.Task); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateAdvancedCustomTask(index int, task *AdvancedCustomTask) error {
	if task == nil {
		return nil
	}
	submitMethod := strings.ToUpper(strings.TrimSpace(task.SubmitMethod))
	if submitMethod == "" {
		submitMethod = http.MethodPost
	}
	if !isAdvancedCustomTaskMethodAllowed(submitMethod) {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.submit_method is invalid: %s", index, task.SubmitMethod)
	}

	requestMode := strings.ToLower(strings.TrimSpace(task.RequestMode))
	if requestMode == "" {
		requestMode = AdvancedCustomTaskRequestModePassthrough
	}
	switch requestMode {
	case AdvancedCustomTaskRequestModePassthrough:
		if len(task.BodyTemplate) > 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.body_template requires request_mode template", index)
		}
	case AdvancedCustomTaskRequestModeTemplate:
		if len(task.BodyTemplate) == 0 || !json.Valid(task.BodyTemplate) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.body_template must be valid JSON", index)
		}
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.request_mode is invalid: %s", index, task.RequestMode)
	}

	if err := validateAdvancedCustomRouteExpression(index, "task.request_script", task.RequestScript); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.headers_script", task.HeadersScript, "header", "body"); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.body_script", task.BodyScript, "header", "body"); err != nil {
		return err
	}
	if strings.TrimSpace(task.BodyScript) != "" && len(task.BodyTemplate) > 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.body_script cannot be combined with body_template", index)
	}
	if strings.TrimSpace(task.SubmitResponse.TaskIDPath) == "" &&
		strings.TrimSpace(task.SubmitResponse.Script) == "" &&
		strings.TrimSpace(task.SubmitResponse.ResponseScript) == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.submit_response.task_id_path is required when no response script is configured", index)
	}
	if err := validateAdvancedCustomRouteExpression(index, "task.submit_response.script", task.SubmitResponse.Script); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.submit_response.response_script", task.SubmitResponse.ResponseScript, "row_response"); err != nil {
		return err
	}
	if err := validateAdvancedCustomTaskStatusMap(index, "task.submit_response.status_map", task.SubmitResponse.StatusMap); err != nil {
		return err
	}
	if err := validateAdvancedCustomTaskErrorMessages(index, "task.submit_response", task.SubmitResponse); err != nil {
		return err
	}

	pollMethod := strings.ToUpper(strings.TrimSpace(task.Poll.Method))
	if pollMethod == "" {
		pollMethod = http.MethodGet
	}
	if !isAdvancedCustomTaskMethodAllowed(pollMethod) {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.method is invalid: %s", index, task.Poll.Method)
	}
	pollPath := strings.TrimSpace(task.Poll.UpstreamPath)
	if pollPath == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.upstream_path is required", index)
	}
	if !strings.Contains(pollPath, "{task_id}") {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.upstream_path must contain {task_id}", index)
	}
	if err := validateAdvancedCustomTaskTarget(index, pollPath); err != nil {
		return err
	}
	if len(task.Poll.BodyTemplate) > 0 && !json.Valid(task.Poll.BodyTemplate) {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.body_template must be valid JSON", index)
	}
	if pollMethod == http.MethodGet && len(task.Poll.BodyTemplate) > 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.body_template is not allowed for GET", index)
	}
	if err := validateAdvancedCustomRouteExpression(index, "task.poll.request_script", task.Poll.RequestScript); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.poll.headers_script", task.Poll.HeadersScript, "header", "body"); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.poll.body_script", task.Poll.BodyScript, "header", "body"); err != nil {
		return err
	}
	if strings.TrimSpace(task.Poll.BodyScript) != "" && len(task.Poll.BodyTemplate) > 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.body_script cannot be combined with body_template", index)
	}
	if err := validateAdvancedCustomRouteAuth(index, task.Poll.Auth); err != nil {
		return err
	}
	if err := validateAdvancedCustomHeaders(index, "task.poll.headers", task.Poll.Headers); err != nil {
		return err
	}
	if task.Download != nil {
		if err := validateAdvancedCustomRouteAuth(index, task.Download.Auth); err != nil {
			return err
		}
		if err := validateAdvancedCustomHeaders(index, "task.download.headers", task.Download.Headers); err != nil {
			return err
		}
	}

	response := task.Poll.Response
	if err := validateAdvancedCustomRouteExpression(index, "task.poll.response.script", response.Script); err != nil {
		return err
	}
	if err := validateAdvancedCustomRouteJavaScript(index, "task.poll.response.response_script", response.ResponseScript, "row_response"); err != nil {
		return err
	}
	if strings.TrimSpace(response.Script) == "" && strings.TrimSpace(response.ResponseScript) == "" {
		if strings.TrimSpace(response.StatusPath) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.response.status_path is required when no response script is configured", index)
		}
		if strings.TrimSpace(response.ResultURLPath) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.response.result_url_path is required when no response script is configured", index)
		}
		if len(response.StatusMap) == 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.response.status_map is required when no response script is configured", index)
		}
	}
	if err := validateAdvancedCustomTaskStatusMap(index, "task.poll.response.status_map", response.StatusMap); err != nil {
		return err
	}
	return validateAdvancedCustomTaskErrorMessages(index, "task.poll.response", response)
}

func validateAdvancedCustomTaskStatusMap(index int, field string, statusMap map[string]string) error {
	for upstream, canonical := range statusMap {
		if strings.TrimSpace(upstream) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s contains an empty upstream status", index, field)
		}
		if !IsAdvancedCustomCanonicalTaskStatus(canonical) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s has invalid target status: %s", index, field, canonical)
		}
	}
	return nil
}

func IsAdvancedCustomCanonicalTaskStatus(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "SUBMITTED", "QUEUED", "IN_PROGRESS", "SUCCESS", "FAILURE":
		return true
	default:
		return false
	}
}

func validateAdvancedCustomTaskErrorMessages(index int, field string, response AdvancedCustomTaskResponse) error {
	hasMessages := len(response.ErrorMessageMap) > 0 || strings.TrimSpace(response.DefaultErrorMessage) != ""
	if hasMessages && strings.TrimSpace(response.ErrorCodePath) == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].%s.error_code_path is required when safe error messages are configured", index, field)
	}
	for code, message := range response.ErrorMessageMap {
		if strings.TrimSpace(code) == "" || strings.TrimSpace(message) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s.error_message_map contains an empty code or message", index, field)
		}
	}
	return nil
}

func validateAdvancedCustomRouteExpression(index int, field string, source string) error {
	if err := routeexpr.Validate(source); err != nil {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].%s is invalid: %w", index, field, err)
	}
	return nil
}

func validateAdvancedCustomRouteJavaScript(index int, field string, source string, parameterNames ...string) error {
	if err := routejs.ValidateFunction(source, parameterNames...); err != nil {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].%s is invalid: %w", index, field, err)
	}
	return nil
}

func isAdvancedCustomTaskMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func validateAdvancedCustomTaskTarget(index int, upstreamPath string) error {
	probe := strings.ReplaceAll(upstreamPath, "{task_id}", "task-id")
	probe = strings.ReplaceAll(probe, advancedCustomModelPlaceholder, "model")
	if strings.HasPrefix(probe, "/") {
		if strings.HasPrefix(probe, "//") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.upstream_path must be a full URL or a path starting with /", index)
		}
		return nil
	}
	parsedURL, err := url.Parse(probe)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.upstream_path must be a full URL or a path starting with /", index)
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].task.poll.upstream_path must use http or https", index)
	}
	return nil
}

func validateAdvancedCustomHeaders(index int, field string, headers map[string]string) error {
	for name, value := range headers {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s contains an empty header name", index, field)
		}
		if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].%s contains an invalid header", index, field)
		}
	}
	return nil
}

type advancedCustomPathModelState struct {
	catchAllIndex int
	modelIndexes  map[string]int
}

func validateAdvancedCustomRouteModels(index int, incomingPath string, models []string, paths map[string]*advancedCustomPathModelState) error {
	state := paths[incomingPath]
	if state == nil {
		state = &advancedCustomPathModelState{
			catchAllIndex: -1,
			modelIndexes:  make(map[string]int),
		}
		paths[incomingPath] = state
	}

	normalizedModels := normalizeAdvancedCustomRouteModels(models)
	if len(normalizedModels) == 0 {
		if state.catchAllIndex >= 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models catch-all already exists for incoming_path: %s", index, incomingPath)
		}
		state.catchAllIndex = index
		return nil
	}

	if state.catchAllIndex >= 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models catch-all route must be last for incoming_path: %s", index, incomingPath)
	}

	seenInRoute := make(map[string]struct{}, len(normalizedModels))
	for _, model := range normalizedModels {
		if err := validateAdvancedCustomRouteModelRule(index, incomingPath, model); err != nil {
			return err
		}
		if _, exists := seenInRoute[model]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models contains duplicate model for incoming_path %s: %s", index, incomingPath, model)
		}
		seenInRoute[model] = struct{}{}
		if existingIndex, exists := state.modelIndexes[model]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models overlaps with advanced_routes[%d] for incoming_path %s: %s", index, existingIndex, incomingPath, model)
		}
		state.modelIndexes[model] = index
	}
	return nil
}

func validateAdvancedCustomRouteModelRule(index int, incomingPath string, model string) error {
	if !strings.HasPrefix(model, advancedCustomModelRegexPrefix) {
		return nil
	}
	pattern := strings.TrimPrefix(model, advancedCustomModelRegexPrefix)
	if pattern == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models regex is empty for incoming_path %s: %s", index, incomingPath, model)
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models regex is invalid for incoming_path %s: %s", index, incomingPath, model)
	}
	return nil
}

func normalizeAdvancedCustomRouteModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			normalized = append(normalized, model)
		}
	}
	return normalized
}

func validateAdvancedCustomUpstreamTarget(index int, upstreamPath string) error {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
		}
		return nil
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must use http or https", index)
	}
	return nil
}

func validateAdvancedCustomConverterPath(index int, incomingPath string, converter string) error {
	if incomingPath == advancedCustomEndpointPathOpenAIAlphaSearch {
		if converter == advancedCustomConverterNone {
			return nil
		}
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter does not match incoming_path: %s", index, converter)
	}
	switch converter {
	case advancedCustomConverterNone:
		return nil
	case AdvancedCustomConverterSGLangRerank:
		if incomingPath == advancedCustomEndpointPathJinaRerank || incomingPath == "/rerank" {
			return nil
		}
	case advancedCustomConverterClaudeMessagesToOpenAIChat,
		advancedCustomConverterClaudeMessagesToOpenAIResponses:
		if incomingPath == "/v1/messages" {
			return nil
		}
	case advancedCustomConverterOpenAIChatToClaudeMessages,
		advancedCustomConverterOpenAIChatToOpenAIResponses,
		advancedCustomConverterOpenAIChatToGeminiContent:
		if incomingPath == "/v1/chat/completions" {
			return nil
		}
	case advancedCustomConverterOpenAIResponsesToOpenAIChat:
		if incomingPath == "/v1/responses" {
			return nil
		}
	case advancedCustomConverterOpenAIResponsesToGemini:
		if incomingPath == "/v1/responses" {
			return nil
		}
	case advancedCustomConverterGeminiContentToOpenAIChat,
		advancedCustomConverterGeminiContentToOpenAIResponses:
		if strings.Contains(incomingPath, ":generateContent") || strings.Contains(incomingPath, ":streamGenerateContent") {
			return nil
		}
	}
	return fmt.Errorf("advanced_custom.advanced_routes[%d].converter does not match incoming_path: %s", index, converter)
}

func validateAdvancedCustomRouteAuth(index int, auth *AdvancedCustomRouteAuth) error {
	if auth == nil {
		return nil
	}
	authType := strings.TrimSpace(auth.Type)
	switch authType {
	case AdvancedCustomAuthTypeNone:
		return nil
	case AdvancedCustomAuthTypeHeader, AdvancedCustomAuthTypeQuery:
		name := strings.TrimSpace(auth.Name)
		value := strings.TrimSpace(auth.Value)
		if name == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.name is required", index)
		}
		if value == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.value is required", index)
		}
		if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") ||
			(authType == AdvancedCustomAuthTypeHeader && strings.Contains(name, ":")) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth contains an invalid name or value", index)
		}
		return nil
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.type is invalid: %s", index, auth.Type)
	}
}

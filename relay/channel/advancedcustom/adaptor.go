package advancedcustom

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const ChannelName = "advanced_custom"

const advancedCustomModelPlaceholder = "{model}"

type Adaptor struct {
	openaiAdaptor openai.Adaptor
	claudeAdaptor claude.Adaptor
	geminiAdaptor gemini.Adaptor

	resolved  bool
	converted bool
	route     dto.AdvancedCustomRoute
	converter string
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.openaiAdaptor.Init(info)
	a.claudeAdaptor.Init(info)
	a.geminiAdaptor.Init(info)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter == relayconvert.ConverterNone {
		if len(a.route.RequestBodyTemplate) > 0 {
			return a.convertTemplatedJSONRequest(c, info)
		}
		return a.convertOpenAICompatibleRequest(c, info, request)
	}

	switch converter {
	case relayconvert.ConverterOpenAIChatToClaudeMessages,
		relayconvert.ConverterOpenAIChatToOpenAIResponses,
		relayconvert.ConverterOpenAIChatToGeminiContent:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		return result.Value, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI chat completions requests", converter)
	}
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		if len(a.route.RequestBodyTemplate) > 0 {
			return a.convertTemplatedJSONRequest(c, info)
		}
		return a.claudeAdaptor.ConvertClaudeRequest(c, info, request)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterClaudeMessagesToOpenAIResponses:
		return convertCrossProtocolResponsesRequest(c, info, converter, request)
	default:
		return nil, fmt.Errorf("converter %q does not support Anthropic Messages requests", converter)
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		if len(a.route.RequestBodyTemplate) > 0 {
			return a.convertTemplatedJSONRequest(c, info)
		}
		return a.geminiAdaptor.ConvertGeminiRequest(c, info, request)
	case relayconvert.ConverterGeminiContentToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterGeminiContentToOpenAIResponses:
		return convertCrossProtocolResponsesRequest(c, info, converter, request)
	default:
		return nil, fmt.Errorf("converter %q does not support Gemini generateContent requests", converter)
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	switch converter {
	case relayconvert.ConverterNone:
		if len(a.route.RequestBodyTemplate) > 0 {
			return a.convertTemplatedJSONRequest(c, info)
		}
		return a.convertOpenAICompatibleResponsesRequest(c, info, request)
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterOpenAIResponsesToGemini:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
		if !ok {
			return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
		}
		return geminiRequest, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI Responses requests", converter)
	}
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support embedding requests", converter)
	}
	if len(a.route.RequestBodyTemplate) > 0 {
		return a.convertTemplatedJSONRequest(c, info)
	}
	return a.convertOpenAICompatibleEmbeddingRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support audio requests", converter)
	}
	if len(a.route.RequestBodyTemplate) > 0 {
		if !isJSONRequest(c) {
			return nil, fmt.Errorf("advanced custom JSON request templates require application/json")
		}
		body, err := a.buildTemplatedJSONRequest(c, info)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(body), nil
	}
	return a.convertOpenAICompatibleAudioRequest(c, info, request)
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support image requests", converter)
	}
	if len(a.route.RequestBodyTemplate) > 0 {
		if !isJSONRequest(c) {
			return nil, fmt.Errorf("advanced custom JSON request templates require application/json")
		}
		return a.convertTemplatedJSONRequest(c, info)
	}
	return a.convertOpenAICompatibleImageRequest(c, info, request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	a.converted = true
	return a.openaiAdaptor.ConvertRerankRequest(c, relayMode, request)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(nil, info); err != nil {
		return "", err
	}
	return a.routeURL(info)
}

func (a *Adaptor) BuildModelListRequest(info *relaycommon.RelayInfo) (string, http.Header, error) {
	return a.buildManagementRequest(info, dto.AdvancedCustomModelListPath)
}

func (a *Adaptor) BuildBalanceRequest(info *relaycommon.RelayInfo) (string, http.Header, error) {
	return a.buildManagementRequest(info, dto.AdvancedCustomBalancePath)
}

func (a *Adaptor) buildManagementRequest(info *relaycommon.RelayInfo, managementPath string) (string, http.Header, error) {
	if info == nil {
		return "", nil, errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return "", nil, errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return "", nil, err
	}
	var route dto.AdvancedCustomRoute
	var ok bool
	switch managementPath {
	case dto.AdvancedCustomModelListPath:
		route, ok = config.ModelListRoute()
	case dto.AdvancedCustomBalancePath:
		route, ok = config.BalanceRoute()
	default:
		return "", nil, fmt.Errorf("unsupported advanced custom management path: %s", managementPath)
	}
	if !ok {
		return "", nil, fmt.Errorf("advanced custom channel does not configure a %s route", managementPath)
	}
	converter := strings.TrimSpace(route.Converter)
	if converter == "" {
		converter = relayconvert.ConverterNone
	}
	if converter != relayconvert.ConverterNone {
		return "", nil, fmt.Errorf("converter %q does not support %s requests", converter, managementPath)
	}

	requestURL, err := buildRouteURL(route, converter, info)
	if err != nil {
		return "", nil, err
	}

	header := http.Header{}
	auth := route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
		return requestURL, header, nil
	}

	switch strings.TrimSpace(auth.Type) {
	case dto.AdvancedCustomAuthTypeNone, dto.AdvancedCustomAuthTypeQuery:
	case dto.AdvancedCustomAuthTypeHeader:
		header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
	default:
		return "", nil, fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
	}
	return requestURL, header, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.resolve(c, info); err != nil {
		return err
	}

	channel.SetupApiRequestHeader(info, c, header)
	auth := a.route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
	} else {
		switch strings.TrimSpace(auth.Type) {
		case dto.AdvancedCustomAuthTypeNone:
		case dto.AdvancedCustomAuthTypeHeader:
			header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
		case dto.AdvancedCustomAuthTypeQuery:
		default:
			return fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
		}
	}

	if shouldApplyClaudeHeaders(a.converter, info) {
		applyClaudeHeaders(c, header, info)
	}
	applyConfiguredHeaders(*header, a.route.Headers, info.ApiKey, info.UpstreamModelName, "")

	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if err := a.resolve(c, info); err != nil {
		return nil, err
	}
	if info.IsStream && len(a.route.ResponseBodyTemplate) > 0 {
		return nil, errors.New("advanced custom JSON response templates do not support streaming")
	}
	if !a.converted && a.converter != relayconvert.ConverterNone {
		return nil, errors.New("advanced custom converter routes cannot be used with pass-through request body")
	}
	method := strings.ToUpper(strings.TrimSpace(a.route.Method))
	if method == "" {
		method = c.Request.Method
	}

	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation ||
		(info.RelayMode == relayconstant.RelayModeImagesEdits && !isJSONRequest(c)) {
		return channel.DoFormRequestWithMethod(a, c, info, method, requestBody)
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequestWithMethod(a, c, info, method, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if err := a.resolve(c, info); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(a.route.ResponseBodyTemplate) > 0 && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return a.doTemplatedJSONResponse(c, resp, info)
	}

	switch a.converter {
	case relayconvert.ConverterNone:
		return a.doNativeResponse(c, resp, info)
	case dto.AdvancedCustomConverterSGLangRerank:
		return a.doSGLangRerankResponse(c, resp, info)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat,
		relayconvert.ConverterGeminiContentToOpenAIChat:
		return a.openaiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToClaudeMessages:
		return a.claudeAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToGeminiContent:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIResponsesToGemini:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToOpenAIResponses,
		relayconvert.ConverterClaudeMessagesToOpenAIResponses,
		relayconvert.ConverterGeminiContentToOpenAIResponses:
		// These handlers convert the Responses upstream into info.RelayFormat.
		if info.IsStream {
			return openai.OaiResponsesToChatStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesToChatHandler(c, info, resp)
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		if info.IsStream {
			return openai.OaiChatToResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiChatToResponsesHandler(c, info, resp)
	default:
		return nil, types.NewOpenAIError(fmt.Errorf("unsupported advanced custom converter: %s", a.converter), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
}

func (a *Adaptor) buildTemplatedJSONRequest(c *gin.Context, info *relaycommon.RelayInfo) ([]byte, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	return applyAdvancedCustomTemplate(a.route.RequestBodyTemplate, advancedCustomTemplateValues{
		model:       info.UpstreamModelName,
		requestBody: rawBody,
	})
}

func (a *Adaptor) convertTemplatedJSONRequest(c *gin.Context, info *relaycommon.RelayInfo) (any, error) {
	body, err := a.buildTemplatedJSONRequest(c, info)
	if err != nil {
		return nil, err
	}
	var value any
	if err := common.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (a *Adaptor) doTemplatedJSONResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info.IsStream {
		return nil, types.NewOpenAIError(
			errors.New("advanced custom JSON response templates do not support streaming"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	_ = resp.Body.Close()
	mappedBody, err := applyAdvancedCustomTemplate(a.route.ResponseBodyTemplate, advancedCustomTemplateValues{
		model:        info.OriginModelName,
		responseBody: responseBody,
	})
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	c.Data(resp.StatusCode, "application/json", mappedBody)
	return nil, nil
}

func (a *Adaptor) GetModelList() []string {
	models := make([]string, 0, len(openai.ModelList)+len(claude.ModelList)+len(gemini.ModelList))
	models = append(models, openai.ModelList...)
	models = append(models, claude.ModelList...)
	models = append(models, gemini.ModelList...)
	return lo.Uniq(models)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) doNativeResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return a.claudeAdaptor.DoResponse(c, resp, info)
	case types.RelayFormatGemini:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	default:
		return a.openaiAdaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) resolveForConversion(c *gin.Context, info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(c, info); err != nil {
		return "", err
	}
	a.converted = true
	return a.converter, nil
}

func (a *Adaptor) resolve(c *gin.Context, info *relaycommon.RelayInfo) error {
	if a.resolved {
		return nil
	}
	if info == nil {
		return errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return err
	}

	incomingPath := incomingRequestPath(c, info)
	route, ok := config.MatchPathForModel(incomingPath, info.OriginModelName)
	if ok {
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = relayconvert.ConverterNone
		}
		a.route = route
		a.converter = route.Converter
		a.resolved = true
		return nil
	}
	return fmt.Errorf("advanced custom channel does not support request path %s for model %s", incomingPath, info.OriginModelName)
}

func incomingRequestPath(c *gin.Context, info *relaycommon.RelayInfo) string {
	if c != nil && c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	if info == nil {
		return ""
	}
	return strings.Split(info.RequestURLPath, "?")[0]
}

func (a *Adaptor) routeURL(info *relaycommon.RelayInfo) (string, error) {
	return buildRouteURL(a.route, a.converter, info)
}

func buildRouteURL(route dto.AdvancedCustomRoute, converter string, info *relaycommon.RelayInfo) (string, error) {
	parsedURL, err := resolveUpstreamTargetURL(applyUpstreamPathTemplate(strings.TrimSpace(route.UpstreamPath), info), info)
	if err != nil {
		return "", err
	}
	if shouldUseGeminiStreamURL(converter, info) {
		useGeminiStreamGenerateContentURL(parsedURL)
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeRealtime {
		switch parsedURL.Scheme {
		case "https":
			parsedURL.Scheme = "wss"
		case "http":
			parsedURL.Scheme = "ws"
		}
	}
	if route.Auth != nil && strings.TrimSpace(route.Auth.Type) == dto.AdvancedCustomAuthTypeQuery {
		query := parsedURL.Query()
		query.Set(strings.TrimSpace(route.Auth.Name), applyAuthTemplate(route.Auth.Value, info.ApiKey))
		parsedURL.RawQuery = query.Encode()
	}
	return parsedURL.String(), nil
}

func resolveUpstreamTargetURL(upstreamPath string, info *relaycommon.RelayInfo) (*url.URL, error) {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
		}
		if info == nil || strings.TrimSpace(info.ChannelBaseUrl) == "" {
			return nil, errors.New("channel base URL is required when advanced custom upstream path is relative")
		}
		return joinBaseURLAndUpstreamPath(info.ChannelBaseUrl, upstreamPath)
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return nil, errors.New("advanced custom upstream path must use http or https")
	}
	return parsedURL, nil
}

func joinBaseURLAndUpstreamPath(baseURL string, upstreamPath string) (*url.URL, error) {
	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("channel base URL must be a full URL when advanced custom upstream path is relative")
	}
	if !strings.EqualFold(parsedBaseURL.Scheme, "http") && !strings.EqualFold(parsedBaseURL.Scheme, "https") {
		return nil, errors.New("channel base URL must use http or https when advanced custom upstream path is relative")
	}

	parsedPath, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	parsedBaseURL.Path = strings.TrimRight(parsedBaseURL.Path, "/") + "/" + strings.TrimLeft(parsedPath.Path, "/")
	parsedBaseURL.RawPath = ""
	parsedBaseURL.RawQuery = parsedPath.RawQuery
	parsedBaseURL.Fragment = parsedPath.Fragment
	return parsedBaseURL, nil
}

func applyUpstreamPathTemplate(upstreamPath string, info *relaycommon.RelayInfo) string {
	if info == nil {
		return upstreamPath
	}
	return strings.ReplaceAll(upstreamPath, advancedCustomModelPlaceholder, info.UpstreamModelName)
}

// BuildTaskRouteURL resolves an asynchronous Advanced Custom operation URL.
// Relative paths are joined to the channel base URL; {model} and {task_id}
// placeholders are replaced before configured query authentication is applied.
func BuildTaskRouteURL(baseURL, upstreamPath, modelName, taskID, apiKey string, auth *dto.AdvancedCustomRouteAuth) (string, error) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl:    baseURL,
		UpstreamModelName: modelName,
		ApiKey:            apiKey,
	}}
	upstreamPath = strings.ReplaceAll(upstreamPath, "{task_id}", url.PathEscape(taskID))
	parsedURL, err := resolveUpstreamTargetURL(applyUpstreamPathTemplate(strings.TrimSpace(upstreamPath), info), info)
	if err != nil {
		return "", err
	}
	if auth != nil && strings.TrimSpace(auth.Type) == dto.AdvancedCustomAuthTypeQuery {
		query := parsedURL.Query()
		query.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, apiKey))
		parsedURL.RawQuery = query.Encode()
	}
	return parsedURL.String(), nil
}

// ApplyTaskRouteHeaders applies default/configured authentication and custom
// header templates for an asynchronous Advanced Custom operation.
func ApplyTaskRouteHeaders(header http.Header, headers map[string]string, auth *dto.AdvancedCustomRouteAuth, apiKey, modelName, taskID string) error {
	if auth == nil {
		header.Set("Authorization", "Bearer "+apiKey)
	} else {
		switch strings.TrimSpace(auth.Type) {
		case dto.AdvancedCustomAuthTypeNone, dto.AdvancedCustomAuthTypeQuery:
		case dto.AdvancedCustomAuthTypeHeader:
			header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, apiKey))
		default:
			return fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
		}
	}
	applyConfiguredHeaders(header, headers, apiKey, modelName, taskID)
	return nil
}

// ApplyTaskDownloadHeaders applies only explicitly configured download
// credentials. Unlike submit and poll operations, a missing auth block does
// not imply bearer authentication because result URLs are commonly presigned.
func ApplyTaskDownloadHeaders(header http.Header, download *dto.AdvancedCustomTaskDownload, apiKey, modelName, taskID string) error {
	if download == nil {
		return nil
	}
	if download.Auth != nil {
		switch strings.TrimSpace(download.Auth.Type) {
		case dto.AdvancedCustomAuthTypeNone, dto.AdvancedCustomAuthTypeQuery:
		case dto.AdvancedCustomAuthTypeHeader:
			header.Set(strings.TrimSpace(download.Auth.Name), applyAuthTemplate(download.Auth.Value, apiKey))
		default:
			return fmt.Errorf("invalid advanced custom download auth type: %s", download.Auth.Type)
		}
	}
	applyConfiguredHeaders(header, download.Headers, apiKey, modelName, taskID)
	return nil
}

// ApplyTaskDownloadURL applies explicitly configured query authentication to
// an upstream result URL. Result URLs must already be absolute.
func ApplyTaskDownloadURL(rawURL string, download *dto.AdvancedCustomTaskDownload, apiKey string) (string, error) {
	if download == nil || download.Auth == nil || strings.TrimSpace(download.Auth.Type) != dto.AdvancedCustomAuthTypeQuery {
		return rawURL, nil
	}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", fmt.Errorf("advanced custom result URL is invalid")
	}
	query := parsedURL.Query()
	query.Set(strings.TrimSpace(download.Auth.Name), applyAuthTemplate(download.Auth.Value, apiKey))
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func applyConfiguredHeaders(header http.Header, headers map[string]string, apiKey, modelName, taskID string) {
	for name, value := range headers {
		value = applyAuthTemplate(value, apiKey)
		value = strings.ReplaceAll(value, advancedCustomModelPlaceholder, modelName)
		value = strings.ReplaceAll(value, "{task_id}", taskID)
		header.Set(name, value)
	}
}

func shouldUseGeminiStreamURL(converter string, info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.IsStream &&
		(converter == relayconvert.ConverterOpenAIChatToGeminiContent ||
			converter == relayconvert.ConverterOpenAIResponsesToGemini)
}

func useGeminiStreamGenerateContentURL(parsedURL *url.URL) {
	if strings.Contains(parsedURL.Path, ":generateContent") {
		parsedURL.Path = strings.Replace(parsedURL.Path, ":generateContent", ":streamGenerateContent", 1)
	}
	if strings.Contains(parsedURL.Path, ":streamGenerateContent") {
		query := parsedURL.Query()
		query.Set("alt", "sse")
		parsedURL.RawQuery = query.Encode()
	}
}

func shouldApplyClaudeHeaders(converter string, info *relaycommon.RelayInfo) bool {
	return converter == relayconvert.ConverterOpenAIChatToClaudeMessages ||
		(converter == relayconvert.ConverterNone && info != nil && info.RelayFormat == types.RelayFormatClaude)
}

func applyClaudeHeaders(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) {
	anthropicVersion := ""
	if c != nil && c.Request != nil {
		anthropicVersion = c.Request.Header.Get("anthropic-version")
	}
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	header.Set("anthropic-version", anthropicVersion)
	if c != nil {
		claude.CommonClaudeHeadersOperation(c, header, info)
	}
}

func applyAuthTemplate(template string, apiKey string) string {
	return strings.ReplaceAll(template, "{api_key}", apiKey)
}

func isJSONRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json")
}

// convertCrossProtocolChatRequest converts a Claude, Gemini, or Responses request into an
// OpenAI chat completions request for an OpenAI-compatible upstream. Streaming requests
// always ask the upstream for usage, because the downstream protocol reports usage in its
// own format and never carries stream_options itself.
func (a *Adaptor) convertCrossProtocolChatRequest(c *gin.Context, info *relaycommon.RelayInfo, converter string, request any) (any, error) {
	result, err := service.ConvertRequestByID(c, info, converter, request)
	if err != nil {
		return nil, err
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	if info.SupportStreamOptions && info.IsStream {
		chatRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	return a.convertOpenAICompatibleRequest(c, info, chatRequest)
}

// convertCrossProtocolResponsesRequest converts a Claude or Gemini client
// request for an OpenAI Responses upstream.
func convertCrossProtocolResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, converter string, request any) (any, error) {
	result, err := service.ConvertRequestByID(c, info, converter, request)
	if err != nil {
		return nil, err
	}
	responsesRequest, ok := result.Value.(*dto.OpenAIResponsesRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI Responses request, got %T", result.Value)
	}
	return responsesRequest, nil
}

func (a *Adaptor) convertOpenAICompatibleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIResponsesRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertEmbeddingRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertAudioRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertImageRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

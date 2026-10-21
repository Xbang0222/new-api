package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	case relayconstant.RelayModeAlphaSearch:
		err = relay.AlphaSearchHelper(c, info)
	case relayconstant.RelayModeToolsSearch, relayconstant.RelayModeToolsFetch:
		err = relay.DeepSeekToolsHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func publicModelUnavailableError(requestId string) *types.NewAPIError {
	message := common.MessageWithRequestId("当前模型不可用", requestId)
	return types.WithOpenAIError(types.OpenAIError{
		Message: message,
		Type:    "server_error",
		Code:    "model_unavailable",
	}, http.StatusInternalServerError)
}

func writeRelayError(c *gin.Context, ws *websocket.Conn, relayFormat types.RelayFormat, relayErr *types.NewAPIError, requestId string) {
	clientErr := relayErr
	if relayErr.IsUpstream() {
		clientErr = publicModelUnavailableError(requestId)
	} else {
		clientErr.SetMessage(common.MessageWithRequestId(clientErr.Error(), requestId))
	}

	upstreamURLs := common.UpstreamAddresses(c)
	if relayFormat == types.RelayFormatOpenAIRealtime {
		helper.WssError(c, ws, clientErr.ToOpenAIError(upstreamURLs...))
		return
	}
	if c.Writer.Written() && strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
		writeRelayStreamError(c, relayFormat, clientErr)
		return
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	if relayFormat == types.RelayFormatClaude {
		c.JSON(clientErr.StatusCode, gin.H{
			"type":  "error",
			"error": clientErr.ToClaudeError(upstreamURLs...),
		})
		return
	}
	c.JSON(clientErr.StatusCode, gin.H{
		"error": clientErr.ToOpenAIError(upstreamURLs...),
	})
}

func writeRelayStreamError(c *gin.Context, relayFormat types.RelayFormat, relayErr *types.NewAPIError) {
	status, _ := common.GetContextKeyType[*relaycommon.StreamStatus](c, constant.ContextKeyResponseStreamStatus)
	if status == nil {
		status = relaycommon.NewStreamStatus()
		common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, status)
	}
	status.MarkFailed(string(relayErr.GetErrorCode()), string(relayErr.GetErrorType()), relayErr.StatusCode)
	upstreamURLs := common.UpstreamAddresses(c)
	helper.ExtendWriteDeadline(c)
	if relayFormat == types.RelayFormatClaude {
		body, err := common.Marshal(gin.H{
			"type":  "error",
			"error": relayErr.ToClaudeError(upstreamURLs...),
		})
		if err == nil {
			c.Render(-1, common.CustomEvent{Data: "event: error\ndata: " + string(body)})
			_ = helper.FlushWriter(c)
		}
		return
	}
	_ = helper.ObjectData(c, gin.H{"error": relayErr.ToOpenAIError(upstreamURLs...)})
	helper.Done(c)
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
		relayInfo   *relaycommon.RelayInfo
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			service.RecordRequestPolicyTermination(c, newAPIError)
			logger.LogError(c, common.LogText("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			writeRelayError(c, ws, relayFormat, newAPIError, requestId)
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		}
		return
	}

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	defer func() {
		recovered := recover()
		resultErr := newAPIError
		if recovered != nil {
			resultErr = types.NewError(fmt.Errorf("relay panic: %v", recovered), types.ErrorCodeBadResponse)
		}
		if relayFormat != types.RelayFormatOpenAIRealtime {
			perfmetrics.RecordRelayResult(c.Request.Context(), relayInfo, resultErr)
		}
		if recovered != nil {
			panic(recovered)
		}
	}()

	generalSettings := operation_setting.GetGeneralSetting()
	if relayFormat == types.RelayFormatClaude && relayInfo.IsStream && generalSettings.PingIntervalEnabled {
		stopKeepAlive := helper.StartStreamKeepAlive(c, time.Duration(generalSettings.PingIntervalSeconds)*time.Second)
		defer stopKeepAlive()
	}

	if newAPIError = relay.PrepareRequestBilling(c, relayInfo); newAPIError != nil {
		return
	}
	defer func() {
		newAPIError = relay.RefundFailedRequestBilling(c, relayInfo, newAPIError)
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil
	attemptLimit := service.GetRelayAttemptLimit(retryParam)
	for attempt := 0; attempt < attemptLimit; attempt++ {
		retryParam.ResetRateLimitState()
		relayInfo.StreamStatus = nil
		relayInfo.PerformanceBusinessRejection = false
		relayInfo.PerformanceOutputTokens = 0
		if attempt > 0 {
			retryParam.IncreaseRetry()
		}
		relayInfo.RetryIndex = retryParam.GetRetry()
		var channel *model.Channel
		var reservation *service.ChannelRateLimitReservation
		var channelErr *types.NewAPIError
		var waitCandidate *channelRateLimitWaitCandidate
		for {
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				if channelErr.GetErrorCode() == types.ErrorCodeChannelRateLimited && waitCandidate != nil {
					channel, reservation, channelErr = waitForRateLimitedChannel(c, relayInfo, retryParam, waitCandidate)
				}
				break
			}
			var decision service.ChannelRateLimitDecision
			var reserveErr error
			reservation, decision, reserveErr = service.ReserveChannelRequest(c.Request.Context(), channel)
			if reserveErr != nil {
				rateLimitErr := fmt.Errorf("check request rate limit for channel %d: %w", channel.Id, reserveErr)
				retryParam.MarkRateLimitUnavailable(channel.Id, rateLimitErr)
				if _, specificChannel := c.Get("specific_channel_id"); specificChannel {
					channelErr = types.NewErrorWithStatusCode(
						rateLimitErr,
						types.ErrorCodeGetChannelFailed,
						http.StatusServiceUnavailable,
						types.ErrOptionWithSkipRetry(),
					)
					break
				}
				relayInfo.ChannelMeta = nil
				continue
			}
			if decision.Allowed {
				break
			}
			retryParam.MarkRateLimited(channel.Id, decision.RetryAfterMillis)
			if _, specificChannel := c.Get("specific_channel_id"); specificChannel {
				candidate := &channelRateLimitWaitCandidate{channel: channel, group: relayInfo.UsingGroup, retryAfterMillis: decision.RetryAfterMillis, autoGroupRetryIndex: common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex)}
				channel, reservation, channelErr = waitForRateLimitedChannel(c, relayInfo, retryParam, candidate)
				break
			}
			if waitCandidate == nil || decision.RetryAfterMillis < waitCandidate.retryAfterMillis {
				waitCandidate = &channelRateLimitWaitCandidate{
					channel: channel, group: relayInfo.UsingGroup, retryAfterMillis: decision.RetryAfterMillis,
					autoGroupRetryIndex: common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex),
				}
			}
			relayInfo.ChannelMeta = nil
		}
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			if waitCandidate == nil && channelErr.GetErrorCode() != types.ErrorCodeChannelRateLimited && retryParam.RateLimitError() == nil &&
				relayInfo.LastError != nil {
				newAPIError = relayInfo.LastError
			} else {
				newAPIError = channelErr
			}
			break
		}
		addUsedChannel(c, channel.Id)
		retryParam.MarkChannelAttempted(channel)
		if relayInfo.Billing == nil && relayInfo.TieredBillingSnapshot == nil &&
			relayInfo.PriceData.FreeModel && relayInfo.PriceData.GroupRatioInfo.GroupRatio != 0 {
			// A free initial group skipped funding checks. Ordinary ratio and
			// per-call billing must create their session before a paid fallback,
			// preserving token limits and the user's subscription preference.
			billingMeta := &types.TokenCountMeta{}
			if relayInfo.Request != nil {
				billingMeta = relayInfo.Request.GetTokenCountMeta()
			}
			selectedPrice, priceErr := helper.ModelPriceHelper(
				c,
				relayInfo,
				relayInfo.GetEstimatePromptTokens(),
				billingMeta,
			)
			if priceErr != nil {
				service.ReleaseChannelRequestReservation(reservation)
				newAPIError = types.NewError(priceErr, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
				break
			}
			if !selectedPrice.FreeModel {
				if billingErr := service.PreConsumeBilling(c, selectedPrice.QuotaToPreConsume, relayInfo); billingErr != nil {
					service.ReleaseChannelRequestReservation(reservation)
					newAPIError = billingErr
					break
				}
			}
		}
		if billingErr := service.PrepareTieredBillingForSelectedGroup(c, relayInfo); billingErr != nil {
			service.ReleaseChannelRequestReservation(reservation)
			newAPIError = billingErr
			break
		}
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			service.ReleaseChannelRequestReservation(reservation)
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		relayInfo.UpstreamStreamError = nil
		relayInfo.ResetUpstreamTiming()
		service.BeginScheduledChannelAttempt(c, relayInfo, channel)
		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}
		if newAPIError == nil && relayInfo.UpstreamStreamError != nil {
			newAPIError = relayInfo.UpstreamStreamError
		}
		if newAPIError != nil && (newAPIError.IsUpstream() || !relayInfo.UpstreamStartTime.IsZero()) {
			newAPIError.MarkUpstream()
		}
		service.FinishScheduledChannelAttempt(relayInfo, channel, newAPIError == nil, newAPIError)
		if relayInfo.UpstreamStartTime.IsZero() {
			service.ReleaseChannelRequestReservation(reservation)
		}

		if newAPIError == nil {
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError, relayInfo)

		if !shouldRetry(c, relayInfo, newAPIError, attemptLimit-attempt-1) {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
}

type channelRateLimitWaitCandidate struct {
	channel             *model.Channel
	group               string
	retryAfterMillis    int64
	autoGroupRetryIndex int
}

func waitForRateLimitedChannel(
	c *gin.Context,
	info *relaycommon.RelayInfo,
	retryParam *service.RetryParam,
	candidate *channelRateLimitWaitCandidate,
) (*model.Channel, *service.ChannelRateLimitReservation, *types.NewAPIError) {
	if candidate == nil || candidate.channel == nil {
		return nil, nil, types.NewErrorWithStatusCode(
			errors.New("request rate limit wait candidate is unavailable"),
			types.ErrorCodeGetChannelFailed,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
		)
	}
	queueStart := time.Now()
	reservation, err := service.WaitForChannelRequest(c.Request.Context(), candidate.channel)
	info.AddChannelRateLimitQueueTime(time.Since(queueStart))
	if err != nil {
		return nil, nil, types.NewErrorWithStatusCode(
			fmt.Errorf("wait for channel %d request rate limit: %w", candidate.channel.Id, err),
			types.ErrorCodeGetChannelFailed,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
		)
	}
	channel, err := model.CacheGetChannel(candidate.channel.Id)
	if err != nil || channel.Status != common.ChannelStatusEnabled {
		service.ReleaseChannelRequestReservation(reservation)
		if err == nil {
			err = fmt.Errorf("channel %d became unavailable while waiting", candidate.channel.Id)
		}
		return nil, nil, types.NewErrorWithStatusCode(
			err,
			types.ErrorCodeGetChannelFailed,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if candidate.group != "" {
		info.UsingGroup = candidate.group
		common.SetContextKey(c, constant.ContextKeyUsingGroup, candidate.group)
	}
	if retryParam.TokenGroup == "auto" {
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		for index, group := range service.GetRequestAutoGroups(c, userGroup) {
			if group == candidate.group {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
				common.SetContextKey(c, constant.ContextKeyAutoGroupIndex, index)
				common.SetContextKey(c, constant.ContextKeyAutoGroupRetryIndex, candidate.autoGroupRetryIndex)
				break
			}
		}
	}
	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName); setupErr != nil {
		service.ReleaseChannelRequestReservation(reservation)
		return nil, nil, setupErr
	}
	retryParam.ResetRateLimitState()
	return channel, reservation, nil
}

// CountClaudeTokens implements Anthropic's token-counting utility endpoint.
// It deliberately skips upstream generation and billing; callers use this
// endpoint to size prompts before creating a Message.
func CountClaudeTokens(c *gin.Context) {
	request, err := helper.GetAndValidateClaudeRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	info := relaycommon.GenRelayInfoClaude(c, request)
	inputTokens, err := service.CountRequestToken(c, request.GetTokenCountMeta(), info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime", "responses"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if info.ChannelMeta == nil && !retryParam.HasRateLimited(c.GetInt("channel_id")) && !retryParam.HasAttempted(c.GetInt("channel_id")) {
		channelId := c.GetInt("channel_id")
		channel, err := model.CacheGetChannel(channelId)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
		}
		return channel, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryGetChannelFailed, map[string]any{"Group": selectGroup, "Model": info.OriginModelName, "Error": err.Error()})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		if rateLimitErr := retryParam.RateLimitError(); rateLimitErr != nil {
			return nil, types.NewErrorWithStatusCode(rateLimitErr, types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
		}
		if retryParam.HasRateLimitedChannels() {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("all available channels in group %s reached their request rate limits", selectGroup),
				types.ErrorCodeChannelRateLimited,
				http.StatusServiceUnavailable,
				types.ErrOptionWithSkipRetry(),
			)
		}
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryNoAvailableChannel, map[string]any{"Group": selectGroup, "Model": info.OriginModelName})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	if selectGroup != "" {
		info.UsingGroup = selectGroup
		common.SetContextKey(c, constant.ContextKeyUsingGroup, selectGroup)
	}
	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func shouldRetry(c *gin.Context, relayInfo *relaycommon.RelayInfo, openaiErr *types.NewAPIError, retryTimes int) bool {
	if openaiErr == nil || retryTimes <= 0 || !canRetryRelayResponse(c, relayInfo) {
		return false
	}
	// Provider-format error wrappers can preserve only the cancellation text.
	if errors.Is(openaiErr, context.Canceled) || strings.Contains(strings.ToLower(openaiErr.Error()), context.Canceled.Error()) {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if service.GetChannelConstraints(c).SuppressesRetry() {
		return false
	}
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if service.IsIntelligentSchedulingForGroup(usingGroup) {
		return service.ShouldRetryIntelligentSchedulingError(
			usingGroup, relayInfo, openaiErr,
		)
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

func canRetryRelayResponse(c *gin.Context, relayInfo *relaycommon.RelayInfo) bool {
	if c == nil || (c.Request != nil && c.Request.Context().Err() != nil) {
		return false
	}
	if c.Writer.Status() == http.StatusSwitchingProtocols {
		return false
	}
	if c.Writer.Written() && c.Writer.Header().Get("Content-Encoding") != "" {
		// Compressed writers may report encoded sizes, which cannot be
		// compared with the uncompressed keepalive byte count.
		return false
	}
	if relayInfo == nil {
		return !c.Writer.Written()
	}
	if relayInfo.IsStream {
		// SSE headers and keepalive comments may already be flushed before
		// model output. Only substantive response data ends failover there.
		return !relayInfo.HasSendResponse() && relayInfo.SendResponseCount == 0 && relayInfo.UpstreamFirstContentTime.IsZero() &&
			c.Writer.Size() <= common.GetContextKeyInt(c, constant.ContextKeyRelayKeepAliveBytes)
	}
	return !relayInfo.HasSendResponse() && !c.Writer.Written()
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, common.LocalLogPreview(err.Error())))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if relayInfo != nil && !relayInfo.UpstreamStartTime.IsZero() && service.IsIntelligentSchedulingFaultError(err) &&
		service.ShouldUseSessionChannelBinding(&service.RetryParam{
			Ctx:         c,
			ModelName:   relayInfo.OriginModelName,
			TokenGroup:  usingGroup,
			RequestPath: c.Request.URL.Path,
		}) {
		// A real upstream failure ends the current session assignment. The next
		// retry/new request re-enters intelligent scheduling and establishes a
		// replacement binding with first-writer-wins semantics.
		service.ClearChannelAffinityOnUpstreamFailure(c, channelError.ChannelId)
	}
	if !service.IsIntelligentSchedulingForGroup(usingGroup) && service.ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		modelName := c.GetString("original_model")
		tokenId := c.GetInt("token_id")
		other := model.NewLogOther()
		if c.Request != nil && c.Request.URL != nil {
			other.SetPublic("request_path", c.Request.URL.Path)
		}
		other.SetPublic("error_type", err.GetErrorType())
		other.SetPublic("error_code", err.GetErrorCode())
		if err.IsUpstream() {
			other.SetPublic("status_code", http.StatusInternalServerError)
		} else {
			other.SetPublic("status_code", err.StatusCode)
		}
		service.AppendRelayLogAdminInfo(c, relayInfo, other)
		usedChannels := c.GetStringSlice("use_channel")
		other.SetAdmin("channel_id", channelError.ChannelId)
		other.SetAdmin("channel_name", channelError.ChannelName)
		other.SetAdmin("channel_type", channelError.ChannelType)
		if err.IsUpstream() {
			details := err.UpstreamError()
			if details == nil || details.Body == "" {
				body, marshalErr := common.Marshal(err.RelayError)
				if marshalErr != nil || string(body) == "null" {
					body = []byte(err.Error())
				}
				service.MarkUpstreamError(err, err.StatusCode, "", string(body))
				details = err.UpstreamError()
			}
			upstreamError := map[string]interface{}{
				"attempt":      len(usedChannels),
				"status_code":  err.StatusCode,
				"channel_id":   channelError.ChannelId,
				"channel_name": channelError.ChannelName,
				"channel_type": channelError.ChannelType,
				"stream":       common.GetContextKeyBool(c, constant.ContextKeyIsStream),
			}
			if details != nil {
				if details.StatusCode != 0 {
					upstreamError["status_code"] = details.StatusCode
				}
				if details.ContentType != "" {
					upstreamError["content_type"] = details.ContentType
				}
				if details.Body != "" {
					upstreamError["body"] = details.Body
				}
			}
			other.SetAdmin("upstream_error", upstreamError)
		}
		service.AppendTaskPluginContextAuditInfo(c, other)
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		if relayInfo != nil {
			useTimeSeconds = int(relayInfo.ElapsedRequestSeconds(time.Now()))
		}
		content := err.MaskSensitiveErrorWithStatusCode()
		if err.IsUpstream() {
			content = "当前模型不可用"
		}
		model.RecordErrorLog(c, userId, channelError.ChannelId, modelName, tokenName, content, tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), usingGroup, other)
	}
}

func RelayMidjourney(c *gin.Context) {
	policy := service.RequestPolicy(c)
	defer func() {
		if policy.Attempts > 0 && !policy.Successful {
			service.RecordRequestPolicyTermination(c, types.NewErrorWithStatusCode(errors.New("Midjourney submission failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway, types.ErrOptionWithSkipRetry()))
		}
	}()
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *taskdto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		policy.Successful = false
		upstreamMessage := strings.TrimSpace(fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result))
		upstreamErr := types.NewOpenAIError(errors.New(upstreamMessage), types.ErrorCodeBadResponse, http.StatusBadRequest)
		body, _ := common.Marshal(mjErr)
		service.MarkUpstreamError(upstreamErr, http.StatusBadRequest, "application/json", string(body))
		processChannelError(c, *types.NewChannelError(
			c.GetInt("channel_id"),
			c.GetInt("channel_type"),
			c.GetString("channel_name"),
			common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey),
			common.GetContextKeyString(c, constant.ContextKeyChannelKey),
			c.GetBool("auto_ban"),
		), upstreamErr, relayInfo)
		statusCode := http.StatusInternalServerError
		description := "当前模型不可用"
		if mjErr.Code == 30 {
			description = i18n.T(c, i18n.MsgRelayGroupSaturated)
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": description,
			"type":        "upstream_error",
			"code":        4,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, common.LogText("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	// The web fallback may already have applied static-asset cache headers.
	// A missing API or asset can appear after an upgrade; never cache its 404.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

// RelayTaskPluginEndpoint keeps unclaimed shared-endpoint traffic on its
// existing handler while claimed requests enter the generation-pinned
// host-owned protocol bridge.
func RelayTaskPluginEndpoint(c *gin.Context, fallback gin.HandlerFunc) {
	pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint)
	if !exists {
		fallback(c)
		return
	}
	pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint)
	if !ok || pinned.Plugin == nil || pinned.Generation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Task protocol request failed",
				"type":    "new_api_error",
				"code":    "task_protocol_error",
			},
		})
		return
	}
	if pinned.Protocol != "openai_responses" {
		fallback(c)
		return
	}
	serveTaskPluginProtocol(c, pinned, defaultPluginProtocolBridgeDeps())
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

type taskSubmissionOutcome struct {
	Result    *relay.TaskSubmitResult
	Task      *model.Task
	RelayInfo *relaycommon.RelayInfo
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		respondTaskSubmissionError(c, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if action := c.GetString("task_action"); action != "" {
		relayInfo.Action = action
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	if taskErr := relay.ApplyOriginTaskAffinity(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}

	outcome, taskErr := executeTaskSubmission(c, relayInfo)
	if taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	presentTaskSubmission(c, outcome)
}

// executeTaskSubmission owns the retry, billing, and persistence lifecycle.
// It deliberately performs no client response writes so JSON and protocol
// presenters share the same durable task barrier. Its cancellation semantics
// come from c.Request.Context: native task endpoints use the client context,
// while the Responses bridge supplies an independently bounded context.
func executeTaskSubmission(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*taskSubmissionOutcome, *taskdto.TaskError) {
	return executeTaskSubmissionWith(c, relayInfo, relay.RelayTaskSubmit)
}

type taskSubmitAttempt func(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError)

func executeTaskSubmissionWith(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	submit taskSubmitAttempt,
) (*taskSubmissionOutcome, *taskdto.TaskError) {
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	diagnostics.start(relayInfo)
	var result *relay.TaskSubmitResult
	var taskErr *taskdto.TaskError
	durable := false
	stage := "start"
	defer func() {
		if !durable && relayInfo.Billing != nil {
			diagnostics.refund(stage)
			relayInfo.Billing.Refund(c)
		}
	}()
	stage = "before_attempt"
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_attempt", 0)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}

	attemptLimit := service.GetRelayAttemptLimit(retryParam)
	for attempt := 0; attempt < attemptLimit; attempt++ {
		retryParam.ResetRateLimitState()
		if attempt > 0 {
			retryParam.IncreaseRetry()
		}
		var channel *model.Channel
		var reservation *service.ChannelRateLimitReservation
		lockedChannel, channelLocked := relayInfo.LockedChannel.(*model.Channel)
		channelLocked = channelLocked && lockedChannel != nil
		_, specificChannel := c.Get("specific_channel_id")
		fixedChannel := channelLocked || specificChannel
		channelSelectionFailed := false
		var waitCandidate *channelRateLimitWaitCandidate
		for {
			if channelLocked {
				channel = lockedChannel
				if retryParam.GetRetry() > 0 {
					if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
						taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
						channelSelectionFailed = true
						break
					}
				}
			} else {
				var channelErr *types.NewAPIError
				channel, channelErr = getChannel(c, relayInfo, retryParam)
				if channelErr != nil {
					if channelErr.GetErrorCode() == types.ErrorCodeChannelRateLimited && waitCandidate != nil {
						channel, reservation, channelErr = waitForRateLimitedChannel(c, relayInfo, retryParam, waitCandidate)
						if channelErr == nil {
							break
						}
					}
					logger.LogError(c, channelErr.Error())
					if taskErr == nil ||
						channelErr.GetErrorCode() == types.ErrorCodeChannelRateLimited ||
						retryParam.RateLimitError() != nil {
						code := string(channelErr.GetErrorCode())
						if code == "" {
							code = "get_channel_failed"
						}
						taskErr = service.TaskErrorWrapperLocal(channelErr.Err, code, channelErr.StatusCode)
					}
					channelSelectionFailed = true
					break
				}
			}

			var decision service.ChannelRateLimitDecision
			var reserveErr error
			reservation, decision, reserveErr = service.ReserveChannelRequest(c.Request.Context(), channel)
			if reserveErr != nil {
				rateLimitErr := fmt.Errorf("check request rate limit for channel %d: %w", channel.Id, reserveErr)
				retryParam.MarkRateLimitUnavailable(channel.Id, rateLimitErr)
				if fixedChannel {
					taskErr = service.TaskErrorWrapperLocal(rateLimitErr, "channel_rate_limit_unavailable", http.StatusServiceUnavailable)
					channelSelectionFailed = true
					break
				}
				relayInfo.ChannelMeta = nil
				continue
			}
			if decision.Allowed {
				break
			}
			retryParam.MarkRateLimited(channel.Id, decision.RetryAfterMillis)
			if fixedChannel {
				candidate := &channelRateLimitWaitCandidate{channel: channel, group: relayInfo.UsingGroup, retryAfterMillis: decision.RetryAfterMillis, autoGroupRetryIndex: common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex)}
				var waitErr *types.NewAPIError
				channel, reservation, waitErr = waitForRateLimitedChannel(c, relayInfo, retryParam, candidate)
				if waitErr != nil {
					taskErr = service.TaskErrorWrapperLocal(waitErr.Err, string(waitErr.GetErrorCode()), waitErr.StatusCode)
					channelSelectionFailed = true
				}
				break
			}
			if waitCandidate == nil || decision.RetryAfterMillis < waitCandidate.retryAfterMillis {
				waitCandidate = &channelRateLimitWaitCandidate{
					channel: channel, group: relayInfo.UsingGroup, retryAfterMillis: decision.RetryAfterMillis,
					autoGroupRetryIndex: common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex),
				}
			}
			relayInfo.ChannelMeta = nil
		}
		if channelSelectionFailed {
			break
		}
		diagnostics.attempt(retryParam.GetRetry()+1, channel, relayInfo.LockedChannel != nil)

		addUsedChannel(c, channel.Id)
		retryParam.MarkChannelAttempted(channel)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			service.ReleaseChannelRequestReservation(reservation)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		relayInfo.ResetUpstreamTiming()
		service.BeginScheduledChannelAttempt(c, relayInfo, channel)
		result, taskErr = submit(c, relayInfo)
		if relayInfo.UpstreamStartTime.IsZero() {
			service.ReleaseChannelRequestReservation(reservation)
		}
		if taskErr == nil {
			service.FinishScheduledChannelAttempt(relayInfo, channel, true, nil)
			break
		}
		schedulingErr := types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
		if !taskErr.LocalError {
			if diagnostics := relayInfo.TaskUpstreamDiagnostics; diagnostics != nil {
				service.MarkUpstreamError(schedulingErr, diagnostics.HTTPStatus, diagnostics.ContentType, diagnostics.ResponseBody)
			} else {
				schedulingErr.MarkUpstream()
			}
		}
		service.FinishScheduledChannelAttempt(relayInfo, channel, false, schedulingErr)

		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				schedulingErr, relayInfo)
		}
		if relayInfo.LockedChannel != nil && service.IsHandledByIntelligentChannelScheduling(
			common.GetContextKeyString(c, constant.ContextKeyUsingGroup), schedulingErr,
		) {
			break
		}

		willRetry := shouldRetryTaskRelay(c, relayInfo, channel.Id, taskErr, attemptLimit-attempt-1)
		diagnostics.attemptFailed(retryParam.GetRetry()+1, channel, taskErr, willRetry)
		if !willRetry {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	if taskErr != nil {
		if requestErr := c.Request.Context().Err(); requestErr != nil {
			diagnostics.cancelled(stage, retryParam.GetRetry()+1)
			return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
		}
		diagnostics.failed(stage, "task_error", taskErr, false)
		return nil, taskErr
	}
	if result == nil {
		taskErr = service.TaskErrorWrapperLocal(errors.New("task submission returned no result"), "task_submit_failed", http.StatusInternalServerError)
		diagnostics.failed("submit", "missing_result", taskErr, false)
		return nil, taskErr
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_reserve", retryParam.GetRetry()+1)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	// Reserve any submit-time upward billing adjustment before persistence.
	// This keeps insertion failures fully refundable while ensuring settlement
	// after the barrier normally has a zero positive delta.
	if relayInfo.Billing != nil {
		stage = "reserve"
		diagnostics.reserve("reserve_start", result.Quota)
		if reserveErr := relayInfo.Billing.Reserve(result.Quota); reserveErr != nil {
			common.SysError(common.LogText("reserve adjusted task billing error: %s", reserveErr.Error()))
			taskErr = service.TaskErrorWrapperLocal(errors.New("insufficient quota for adjusted task cost"), string(types.ErrorCodeInsufficientUserQuota), http.StatusForbidden)
			diagnostics.failed("reserve", "insufficient_quota", taskErr, false)
			return nil, taskErr
		}
		diagnostics.reserve("reserve_complete", result.Quota)
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_insert", retryParam.GetRetry()+1)
		return nil, service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
	}

	stage = "insert"
	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.Execution = service.TaskExecutionSnapshotFromContext(c)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.BillingContext = service.NewTaskBillingContext(relayInfo)
	task.PrivateData.BillingContext.TieredSnapshot = relayInfo.TieredBillingSnapshot
	task.Quota = result.Quota
	task.Data = result.TaskData
	if len(result.PluginState) > 0 {
		task.PrivateData.PluginState = result.PluginState
	}
	task.Action = relayInfo.Action
	if immediate := result.Immediate; immediate != nil {
		task.Status = model.TaskStatus(immediate.Status)
		task.Progress = immediate.Progress
		if immediate.Status == model.TaskStatusSuccess || immediate.Status == model.TaskStatusFailure {
			task.FinishTime = time.Now().Unix()
		}
		if immediate.Status == model.TaskStatusFailure {
			task.FailReason = immediate.Reason
		}
		if immediate.Url != "" {
			task.PrivateData.ResultURL = immediate.Url
		} else if immediate.Status == model.TaskStatusSuccess {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
	}
	// A native submit route may declare retainResult: false. It applies only
	// to immediate terminal results: the client receives the complete response
	// once, the upstream snapshot is never persisted, and the task is not
	// retrievable afterwards. An asynchronous result on such a route keeps its
	// snapshot because polling and retrieval need it. The OpenAI Images
	// protocol delivers its images inline in the same HTTP response, so its
	// immediate results follow the same rule; an asynchronous image task is
	// expected there and is polled inside the request.
	var insertOmits []string
	immediateTerminal := result.Immediate != nil && (result.Immediate.Status == model.TaskStatusSuccess || result.Immediate.Status == model.TaskStatusFailure)
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		pinned, ok := pinnedValue.(pluginruntime.PinnedRoute)
		if ok && pinned.Route.RetainResult != nil && !*pinned.Route.RetainResult {
			if immediateTerminal {
				task.PrivateData.ResultDiscarded = true
				insertOmits = append(insertOmits, "data")
			} else {
				logger.LogWarn(c, common.LogText("task plugin route %s %s declares retainResult: false but returned an asynchronous result; retaining task %s", pinned.Route.Method, pinned.Route.Path, task.TaskID))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists && immediateTerminal {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == pluginruntime.ProtocolOpenAIImage {
			task.PrivateData.ResultDiscarded = true
			insertOmits = append(insertOmits, "data")
		}
	}
	diagnostics.insertStart(task)
	if insertErr := task.InsertWithContext(c.Request.Context(), insertOmits...); insertErr != nil {
		common.SysError(common.LogText("insert task error: %s", insertErr.Error()))
		taskErr = service.TaskErrorWrapperLocal(errors.New("failed to persist task"), "task_insert_failed", http.StatusInternalServerError)
		diagnostics.failed("insert", "database_error", taskErr, false)
		return nil, taskErr
	}
	durable = true
	stage = "settle"
	diagnostics.durable(task)
	diagnostics.settleStart(task, result.Quota)

	if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
		common.SysError(common.LogText("settle task billing error: %s", settleErr.Error()))
		taskErr = service.TaskErrorWrapperLocal(errors.New("failed to settle task billing"), "task_billing_settlement_failed", http.StatusInternalServerError)
		diagnostics.failed("settle", "billing_error", taskErr, true)
		return nil, taskErr
	}
	service.LogTaskConsumption(c, relayInfo, task)
	diagnostics.complete(task, result.Quota)

	return &taskSubmissionOutcome{Result: result, Task: task, RelayInfo: relayInfo}, nil
}

func presentTaskSubmission(c *gin.Context, outcome *taskSubmissionOutcome) {
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	otherRatios := outcome.RelayInfo.PriceData.OtherRatios()
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	if ratiosJSON, err := common.Marshal(otherRatios); err == nil {
		c.Header("X-New-Api-Other-Ratios", string(ratiosJSON))
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedRoute); ok && pinned.Plugin != nil && pinned.Route.Render != "" {
			view, err := service.BuildTaskPluginView(outcome.Task)
			requestValue, _ := c.Get(pluginruntime.ContextKeyRouteRequest)
			requestContext, _ := requestValue.(pluginruntime.RouteRequestContext)
			if err == nil {
				viewValue, valueErr := taskPluginProtocolJSONValue(view)
				if valueErr == nil {
					if body, callErr := pinned.Plugin.Engine.CallPath(c.Request.Context(), "native", []string{pinned.Route.Render}, requestContext.JSValue(), viewValue); callErr == nil {
						diagnostics.present(outcome.Task, "native_presenter")
						c.JSON(http.StatusOK, body)
						return
					} else {
						logger.LogError(c, common.LogText("task plugin native submit presenter failed: %s", callErr.Error()))
					}
				} else {
					logger.LogError(c, common.LogText("encode task plugin native submit view failed: %s", valueErr.Error()))
				}
			} else {
				logger.LogError(c, common.LogText("build task plugin native submit view failed: %s", err.Error()))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == "openai_video" && pinned.Operation.Name == "create" {
			diagnostics.present(outcome.Task, "openai_video_create")
			c.JSON(http.StatusOK, outcome.Task.ToOpenAIVideo())
			return
		}
	}
	createdAt := outcome.Task.CreatedAt
	if createdAt == 0 {
		createdAt = outcome.Task.SubmitTime
	}
	diagnostics.present(outcome.Task, "host_fallback")
	c.JSON(http.StatusOK, map[string]any{
		"id":         outcome.Task.TaskID,
		"task_id":    outcome.Task.TaskID,
		"status":     "queued",
		"model":      outcome.RelayInfo.OriginModelName,
		"created_at": createdAt,
	})
}

func respondTaskSubmissionError(c *gin.Context, taskErr *taskdto.TaskError) {
	newTaskPluginSubmitDiagnostics(c).presentError(taskErr)
	if middleware.RespondTaskPluginError(c, taskErr) {
		return
	}
	respondTaskError(c, taskErr)
}

func respondTaskError(c *gin.Context, taskErr *taskdto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = i18n.T(c, i18n.MsgRelayGroupUpstreamSaturated)
	} else if !taskErr.LocalError {
		c.JSON(http.StatusInternalServerError, &taskdto.TaskError{
			Code:       "model_unavailable",
			Message:    "当前模型不可用",
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func taskSubmissionAPIError(taskErr *taskdto.TaskError) *types.NewAPIError {
	if taskErr == nil {
		return types.NewError(errors.New("task submission failed"), types.ErrorCodeBadResponse)
	}
	err := taskErr.Error
	if err == nil {
		err = errors.New(taskErr.Message)
	}
	return types.NewOpenAIError(err, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
}

func shouldRetryTaskRelay(c *gin.Context, relayInfo *relaycommon.RelayInfo, channelId int, taskErr *taskdto.TaskError, retryTimes int) bool {
	if taskErr == nil || taskErr.LocalError || taskErr.NoRetry || retryTimes <= 0 || errors.Is(taskErr.Error, context.Canceled) || !canRetryRelayResponse(c, relayInfo) {
		return false
	}
	if _, specificChannel := c.Get("specific_channel_id"); specificChannel {
		return false
	}
	if service.GetChannelConstraints(c).SuppressesRetry() {
		return false
	}
	usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	if service.IsIntelligentSchedulingForGroup(usingGroup) {
		return service.ShouldRetryIntelligentSchedulingError(
			usingGroup,
			relayInfo,
			types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode),
		)
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}

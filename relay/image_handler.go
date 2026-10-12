package relay

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

func ImageHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	info.BillingImageCount = nil
	info.ImageRequestCount = 0

	imageReq, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ImageRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(imageReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to ImageRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	originalModel := request.Model
	exactSize := normalizeImageSize(request.Size)
	exactSizeRequired := requiresExactImageSize(info, *request, originalModel)
	if exactSizeRequired && exactSize != "" {
		if !isValidExactImageSize(exactSize) {
			return types.NewErrorWithStatusCode(fmt.Errorf("image-2 size must be 16-multiple dimensions, long edge <= 3840, ratio <= 3:1, and 655360-8294400 pixels: %s", request.Size), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		request.Size = exactSize
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	if exactSizeRequired && exactSize != "" {
		request.Size = exactSize
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	imageCount, err := request.ImageCount(false)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	var requestBody io.Reader
	var jsonData []byte

	shouldPassThroughBody := (model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled) &&
		!exactSizeRequired &&
		!(info.RelayMode == relayconstant.RelayModeImagesEdits && info.ApiType == constant.APITypeOpenAI)
	if shouldPassThroughBody {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			requestBody = common.NewReplayableBodyReader(storage)
		} else {
			jsonData, err = storage.Bytes()
			if err != nil {
				return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
		}
	} else {
		convertedRequest, err := adaptor.ConvertImageRequest(c, info, *request)
		if err != nil {
			// An adaptor that already classified its rejection (status code
			// and retry policy) keeps that classification instead of being
			// downgraded to a retryable conversion failure.
			var apiErr *types.NewAPIError
			if errors.As(err, &apiErr) {
				return apiErr
			}
			return types.NewError(err, types.ErrorCodeConvertRequestFailed)
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		switch convertedRequest.(type) {
		case *bytes.Buffer:
			requestBody = convertedRequest.(io.Reader)
		default:
			jsonData, err = common.Marshal(convertedRequest)
			if err != nil {
				return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
			}

			// apply param override
			if len(info.ParamOverride) > 0 {
				jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
				if err != nil {
					return newAPIErrorFromParamOverride(err)
				}
			}
			if exactSizeRequired && exactSize != "" {
				jsonData, err = preserveExactImageSize(jsonData, exactSize)
				if err != nil {
					return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
				}
			}

		}
	}
	if jsonData != nil {
		// This is a different trust boundary from ingress: channel overrides
		// and pass-through bodies can change the quantity actually submitted.
		var outbound struct {
			N *uint `json:"n"`
		}
		if err := common.Unmarshal(jsonData, &outbound); err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid image billing parameters: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		quantityRequest := dto.ImageRequest{N: outbound.N}
		if quantityRequest.N == nil {
			quantityRequest.N = common.GetPointer(uint(imageCount))
		}
		imageCount, err = quantityRequest.ImageCount(false)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		logger.LogDebug(c, "image request body: %s", jsonData)
		body, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		requestBody = body
	}
	if billingErr := service.PrepareImageBillingForRequest(c, info, imageCount); billingErr != nil {
		return billingErr
	}

	info.MarkUpstreamRequestStart()
	resp, err := adaptor.DoRequest(c, info, requestBody)
	info.MarkUpstreamResponse()
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			if httpResp.StatusCode == http.StatusCreated && info.ApiType == constant.APITypeReplicate {
				// replicate channel returns 201 Created when using Prefer: wait, treat it as success.
				httpResp.StatusCode = http.StatusOK
			} else {
				newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
				return newAPIError
			}
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		return newAPIError
	}

	// The log content shows the settled count: the count the handler derived
	// from the upstream response when it did, otherwise the reserved quantity.
	imageN := info.RequestedImageCount()
	if info.BillingImageCount != nil {
		imageN = *info.BillingImageCount
	} else if count, ok := info.PriceData.OtherRatios()["n"]; ok && info.PriceData.UsePrice && count >= 1 && count <= dto.MaxImageN {
		imageN = common.QuotaRound(count)
	}

	if usage.(*dto.Usage).TotalTokens == 0 {
		usage.(*dto.Usage).TotalTokens = 1
	}
	if usage.(*dto.Usage).PromptTokens == 0 {
		usage.(*dto.Usage).PromptTokens = 1
	}

	quality := request.Quality
	if quality == "" {
		quality = "standard"
	}

	var logContent []*common.Message

	if len(request.Size) > 0 {
		logContent = append(logContent, common.NewMessage("Size {{size}}", map[string]any{"size": request.Size}))
	}
	if len(quality) > 0 {
		logContent = append(logContent, common.NewMessage("Quality {{quality}}", map[string]any{"quality": quality}))
	}
	if imageN > 0 {
		logContent = append(logContent, common.NewMessage("Image count {{count}}", map[string]any{"count": imageN}))
	}

	service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), logContent)
	return nil
}

func requiresExactImageSize(info *relaycommon.RelayInfo, request dto.ImageRequest, modelNames ...string) bool {
	if isExactImageSizeModel(request.Model) {
		return true
	}
	for _, modelName := range modelNames {
		if isExactImageSizeModel(modelName) {
			return true
		}
	}
	if info == nil {
		return false
	}
	return isExactImageSizeModel(info.OriginModelName) || isExactImageSizeModel(info.UpstreamModelName)
}

func isExactImageSizeModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return normalized == "image-2" || normalized == "gpt-image-2" || strings.Contains(normalized, "gpt-image-2") || strings.Contains(normalized, "image-2")
}

func isValidExactImageSize(size string) bool {
	width, height, ok := parseImageSize(size)
	if !ok {
		return false
	}
	longEdge := width
	shortEdge := height
	if height > width {
		longEdge = height
		shortEdge = width
	}
	pixels := width * height
	return longEdge <= 3840 &&
		width%16 == 0 &&
		height%16 == 0 &&
		longEdge <= shortEdge*3 &&
		pixels >= 655360 &&
		pixels <= 8294400
}

func normalizeImageSize(size string) string {
	return strings.NewReplacer("*", "x", "×", "x").Replace(strings.TrimSpace(size))
}

func parseImageSize(size string) (int, int, bool) {
	normalized := normalizeImageSize(size)
	parts := strings.Split(normalized, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func preserveExactImageSize(jsonData []byte, requestedSize string) ([]byte, error) {
	var payload map[string]interface{}
	if err := common.Unmarshal(jsonData, &payload); err != nil {
		return nil, err
	}
	if setStringField(payload, "size", requestedSize) ||
		setStringField(payload, "image_size", requestedSize) ||
		setNestedStringField(payload, "parameters", "size", strings.ReplaceAll(requestedSize, "x", "*")) ||
		setNestedStringField(payload, "parameters", "image_size", requestedSize) ||
		setExactDimensionFields(payload, requestedSize) ||
		setNestedExactDimensionFields(payload, "input", requestedSize) {
		return common.Marshal(payload)
	}
	return nil, fmt.Errorf("image-2 exact custom size %q was not preserved by the selected channel; choose an OpenAI-compatible image channel that forwards the exact size field", requestedSize)
}

func setStringField(payload map[string]interface{}, key string, value string) bool {
	if _, ok := payload[key]; !ok {
		return false
	}
	payload[key] = value
	return true
}

func setNestedStringField(payload map[string]interface{}, parentKey string, key string, value string) bool {
	parent, ok := payload[parentKey].(map[string]interface{})
	if !ok {
		return false
	}
	if _, ok := parent[key]; !ok {
		return false
	}
	parent[key] = value
	payload[parentKey] = parent
	return true
}

func setExactDimensionFields(payload map[string]interface{}, requestedSize string) bool {
	width, height, ok := parseImageSize(requestedSize)
	if !ok {
		return false
	}
	hasWidth := hasJSONNumberField(payload, "width")
	hasHeight := hasJSONNumberField(payload, "height")
	if !hasWidth && !hasHeight {
		return false
	}
	payload["width"] = width
	payload["height"] = height
	return true
}

func setNestedExactDimensionFields(payload map[string]interface{}, parentKey string, requestedSize string) bool {
	parent, ok := payload[parentKey].(map[string]interface{})
	if !ok {
		return false
	}
	if !setExactDimensionFields(parent, requestedSize) {
		return false
	}
	payload[parentKey] = parent
	return true
}

func hasJSONNumberField(payload map[string]interface{}, key string) bool {
	switch payload[key].(type) {
	case float64, int, int64, uint, uint64:
		return true
	default:
		return false
	}
}

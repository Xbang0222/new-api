package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

const upstreamErrorBodyLimit = 8 * 1024

var (
	upstreamAuthorizationPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\r\n,;}\]]+)`)
	upstreamSecretQuotedPattern  = regexp.MustCompile(`(?i)((?:"?(?:api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|password|secret|token)"?)\s*[:=]\s*)(?:"[^"]*"|'[^']*')`)
	upstreamSecretBarePattern    = regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|password|secret|token)\s*[:=]\s*)[^"'\s,;}\]]+`)
	upstreamBearerPattern        = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`)
)

func UpstreamErrorBodyForAdmin(body string) string {
	body = common.MaskSensitiveInfo(body)
	body = upstreamAuthorizationPattern.ReplaceAllString(body, `${1}***`)
	body = upstreamSecretQuotedPattern.ReplaceAllString(body, `${1}"***"`)
	body = upstreamSecretBarePattern.ReplaceAllString(body, `${1}***`)
	body = upstreamBearerPattern.ReplaceAllString(body, `${1}***`)
	if len(body) <= upstreamErrorBodyLimit {
		return body
	}
	suffix := fmt.Sprintf("... [truncated, original_length=%d, limit=%d]", len(body), upstreamErrorBodyLimit)
	prefixLimit := upstreamErrorBodyLimit - len(suffix)
	for prefixLimit > 0 && !utf8.ValidString(body[:prefixLimit]) {
		prefixLimit--
	}
	return body[:prefixLimit] + suffix
}

func MarkUpstreamError(err *types.NewAPIError, statusCode int, contentType string, body string) *types.NewAPIError {
	if err != nil {
		err.SetUpstreamError(statusCode, contentType, UpstreamErrorBodyForAdmin(body))
	}
	return err
}

func MidjourneyErrorWrapper(code int, desc string) *taskdto.MidjourneyResponse {
	return &taskdto.MidjourneyResponse{
		Code:        code,
		Description: desc,
	}
}

func MidjourneyErrorWithStatusCodeWrapper(code int, desc string, statusCode int) *taskdto.MidjourneyResponseWithStatusCode {
	return &taskdto.MidjourneyResponseWithStatusCode{
		StatusCode: statusCode,
		Response:   *MidjourneyErrorWrapper(code, desc),
	}
}

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := dto.OpenAIError{
//		Message: text,
//		Type:    "new_api_error",
//		Code:    code,
//	}
//	return &dto.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(common.LogText("error: %s", text))
			text = "request to the upstream address failed"
		}
	}
	claudeError := types.ClaudeError{
		Message: text,
		Type:    "new_api_error",
	}
	return &dto.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (newApiErr *types.NewAPIError) {
	newApiErr = types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	responseBodyText := ""
	defer func() {
		MarkUpstreamError(newApiErr, resp.StatusCode, resp.Header.Get("Content-Type"), responseBodyText)
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	CloseResponseBodyGracefully(resp)
	var errResponse dto.GeneralErrorResponse
	responseBodyText = string(responseBody)
	responseBodyPreview := common.LocalLogPreview(responseBodyText)
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText)
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, responseBodyText)
	}

	err = common.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			newApiErr.Err = buildErrWithBody("")
			newApiErr.RelayError = types.OpenAIError{
				Message: newApiErr.Error(),
				Type:    string(types.ErrorCodeBadResponseStatusCode),
				Code:    types.ErrorCodeBadResponseStatusCode,
			}
		} else {
			logger.LogError(ctx, common.LogText("bad response status code %d, body: %s", resp.StatusCode, responseBodyPreview))
			newApiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}

	if common.GetJsonType(errResponse.Error) == "object" {
		// General format error (OpenAI, Anthropic, Gemini, etc.)
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			newApiErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				newApiErr.Err = buildErrWithBody(newApiErr.Error())
			}
			return
		}
	}
	message := errResponse.ToMessage()
	if message == "" {
		// The body parsed as JSON but carried no usable error message; log the
		// raw body so the upstream failure remains diagnosable.
		logger.LogError(ctx, common.LogText("bad response status code %d with empty error message, body: %s", resp.StatusCode, responseBodyPreview))
	}
	newApiErr = types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		newApiErr.Err = buildErrWithBody(newApiErr.Error())
	}
	return
}

func ResetStatusCode(newApiErr *types.NewAPIError, statusCodeMappingStr string) {
	if newApiErr == nil || statusCodeMappingStr == "" || statusCodeMappingStr == "{}" || newApiErr.StatusCode == http.StatusOK {
		return
	}
	statusCodeMapping := make(map[string]any)
	if err := common.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping); err != nil {
		return
	}
	value, ok := statusCodeMapping[strconv.Itoa(newApiErr.StatusCode)]
	if !ok {
		return
	}
	switch value := value.(type) {
	case string:
		code, err := strconv.Atoi(value)
		if err == nil {
			newApiErr.StatusCode = code
		}
	case float64:
		if value == math.Trunc(value) {
			newApiErr.StatusCode = int(value)
		}
	case json.Number:
		if code, err := strconv.Atoi(value.String()); err == nil {
			newApiErr.StatusCode = code
		}
	}
}

func TaskErrorWrapperLocal(err error, code string, statusCode int) *taskdto.TaskError {
	openaiErr := TaskErrorWrapper(err, code, statusCode)
	openaiErr.LocalError = true
	return openaiErr
}

func TaskErrorWrapper(err error, code string, statusCode int) *taskdto.TaskError {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
		common.SysLog(common.LogText("error: %s", text))
		//text = "请求上游地址失败"
		text = common.MaskSensitiveInfo(text)
	}
	//避免暴露内部错误
	taskError := &taskdto.TaskError{
		Code:       code,
		Message:    text,
		StatusCode: statusCode,
		Error:      err,
	}

	return taskError
}

// TaskErrorFromAPIError 将 PreConsumeBilling 返回的 NewAPIError 转换为 TaskError。
func TaskErrorFromAPIError(apiErr *types.NewAPIError) *taskdto.TaskError {
	if apiErr == nil {
		return nil
	}
	return &taskdto.TaskError{
		Code:       string(apiErr.GetErrorCode()),
		Message:    apiErr.Err.Error(),
		StatusCode: apiErr.StatusCode,
		Error:      apiErr.Err,
	}
}

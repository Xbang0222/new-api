package service

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// attachQuotaSaturationToOther nests a quota saturation marker under
// other.admin_info.quota_saturation. Nesting under admin_info makes it
// admin-only for free, since model.formatUserLogs strips the whole admin_info
// object for non-admin viewers. Creates admin_info if absent. No-op when the
// clamp is nil (the common case: no saturation happened).
func attachQuotaSaturationToOther(other *model.LogOther, clamp *common.QuotaClamp) {
	if clamp == nil || other == nil {
		return
	}
	other.SetAdmin("quota_saturation", clamp.AuditMap())
}

// attachQuotaSaturation records the request's quota clamp (if any) onto the
// consume log's other.admin_info and emits a request-correlated backend audit
// line. Called right before RecordConsumeLog on the text/audio/wss paths.
func attachQuotaSaturation(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil {
		return
	}
	clamp := relayInfo.QuotaClamp
	if clamp == nil {
		return
	}
	attachQuotaSaturationToOther(other, clamp)
	logger.LogWarn(ctx, fmt.Sprintf("quota saturation on consume log: op=%s kind=%s original=%g clamped=%d user=%d model=%s",
		clamp.Op, clamp.Kind, clamp.Original, clamp.Clamped, relayInfo.UserId, relayInfo.GetBillingModelName()))
}

func appendRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if other == nil {
		return
	}
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil {
		if path := ctx.Request.URL.Path; path != "" {
			other.SetPublic("request_path", path)
			return
		}
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		path := relayInfo.RequestURLPath
		if idx := strings.Index(path, "?"); idx != -1 {
			path = path[:idx]
		}
		other.SetPublic("request_path", path)
	}
}

// AppendChannelRateLimitQueueAdminInfo exposes queueing only inside admin_info.
func AppendChannelRateLimitQueueAdminInfo(adminInfo map[string]interface{}, relayInfo *relaycommon.RelayInfo) {
	if adminInfo == nil || relayInfo == nil || relayInfo.ChannelRateLimitQueueTime <= 0 {
		return
	}
	adminInfo["channel_rate_limit_queue_ms"] = relayInfo.ChannelRateLimitQueueTime.Milliseconds()
}

// AppendSmartProtectionReviewAdminInfo exposes safety-model diagnostics only to admins.
func AppendSmartProtectionReviewAdminInfo(adminInfo map[string]interface{}, relayInfo *relaycommon.RelayInfo) {
	if adminInfo == nil || relayInfo == nil {
		return
	}
	reviewed := relayInfo.SmartProtectionReviewTime > 0 || len(relayInfo.SmartProtectionSafeties) > 0 || len(relayInfo.SmartProtectionCategories) > 0 || len(relayInfo.SmartProtectionMatchedRules) > 0 || relayInfo.SmartProtectionReviewRaw != "" || relayInfo.SmartProtectionReviewError != "" || relayInfo.SmartProtectionReviewStatus != "" || relayInfo.SmartProtectionReviewReason != ""
	if relayInfo.SmartProtectionReviewTime > 0 {
		adminInfo["smart_protection_review_ms"] = relayInfo.SmartProtectionReviewTime.Milliseconds()
	}
	if reviewed {
		adminInfo["smart_protection_review_status"] = relayInfo.SmartProtectionReviewStatus
		adminInfo["smart_protection_review_reason"] = relayInfo.SmartProtectionReviewReason
		adminInfo["smart_protection_safeties"] = append([]string{}, relayInfo.SmartProtectionSafeties...)
		adminInfo["smart_protection_categories"] = append([]string{}, relayInfo.SmartProtectionCategories...)
		if relayInfo.SmartProtectionReviewRaw != "" {
			adminInfo["smart_protection_review_raw"] = relayInfo.SmartProtectionReviewRaw
		}
		adminInfo["smart_protection_matched_rules"] = append([]string{}, relayInfo.SmartProtectionMatchedRules...)
		if relayInfo.SmartProtectionReviewError != "" {
			adminInfo["smart_protection_review_error"] = relayInfo.SmartProtectionReviewError
		}
	}
}

func attachChannelRateLimitQueueTime(other interface{}, relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil || relayInfo.ChannelRateLimitQueueTime <= 0 {
		return
	}
	value := relayInfo.ChannelRateLimitQueueTime.Milliseconds()
	switch target := other.(type) {
	case *model.LogOther:
		target.SetAdmin("channel_rate_limit_queue_ms", value)
	case map[string]interface{}:
		adminInfo, ok := target["admin_info"].(map[string]interface{})
		if !ok {
			adminInfo = map[string]interface{}{}
			target["admin_info"] = adminInfo
		}
		AppendChannelRateLimitQueueAdminInfo(adminInfo, relayInfo)
	}
}

func attachSmartProtectionReviewTime(other interface{}, relayInfo *relaycommon.RelayInfo) {
	if relayInfo == nil {
		return
	}
	adminInfo := map[string]interface{}{}
	AppendSmartProtectionReviewAdminInfo(adminInfo, relayInfo)
	switch target := other.(type) {
	case *model.LogOther:
		for key, value := range adminInfo {
			target.SetAdmin(key, value)
		}
	case map[string]interface{}:
		target["admin_info"] = adminInfo
	}
}

// AppendRelayLogAdminInfo records relay routing and conversion diagnostics in
// the admin-only scope shared by successful and failed request logs.
func AppendRelayLogAdminInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if ctx == nil || other == nil {
		return
	}
	other.SetAdmin("use_channel", ctx.GetStringSlice("use_channel"))
	if relayInfo != nil {
		if billingModel := relayInfo.GetBillingModelName(); billingModel != "" && billingModel != relayInfo.OriginModelName {
			other.SetAdmin("billing_model", billingModel)
		}
		if diagnostics := relayInfo.ConversionDiagnostics(); len(diagnostics) > 0 {
			other.SetAdmin("conversion_diagnostics", diagnostics)
		}
		if relayInfo.ConversionDiagnosticsTruncated() {
			other.SetAdmin("conversion_diagnostics_truncated", true)
		}
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey) {
		other.SetAdmin("is_multi_key", true)
		other.SetAdmin("multi_key_index", common.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex))
	}
	if common.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens) {
		other.SetAdmin("local_count_tokens", true)
	}

	AppendChannelAffinityAdminInfo(ctx, other)
	attachChannelRateLimitQueueTime(other, relayInfo)
	attachSmartProtectionReviewTime(other, relayInfo)
}

func GenerateTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) *model.LogOther {
	other := model.NewLogOther()
	other.SetPublic("model_ratio", modelRatio)
	other.SetPublic("group_ratio", groupRatio)
	other.SetPublic("completion_ratio", completionRatio)
	other.SetPublic("cache_tokens", cacheTokens)
	other.SetPublic("cache_ratio", cacheRatio)
	other.SetPublic("model_price", modelPrice)
	other.SetPublic("user_group_ratio", userGroupRatio)
	if upstreamTTFT, ok := relayInfo.UpstreamFirstResponseDuration(); ok {
		other.SetPublic("frt", float64(upstreamTTFT))
	} else {
		other.SetPublic("frt", float64(0))
	}
	if relayInfo.ReasoningEffort != "" {
		other.SetPublic("reasoning_effort", relayInfo.ReasoningEffort)
	}
	if relayInfo.IsModelMapped {
		other.SetPublic("is_model_mapped", true)
		other.SetPublic("upstream_model_name", relayInfo.UpstreamModelName)
	}

	isSystemPromptOverwritten := common.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride)
	if isSystemPromptOverwritten {
		other.SetPublic("is_system_prompt_overwritten", true)
	}

	AppendRelayLogAdminInfo(ctx, relayInfo, other)
	appendRequestPath(ctx, relayInfo, other)
	appendRequestConversionChain(relayInfo, other)
	appendFinalRequestFormat(relayInfo, other)
	appendBillingInfo(relayInfo, other)
	appendParamOverrideInfo(relayInfo, other)
	appendStreamStatus(relayInfo, other)
	return other
}

func appendParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil || other == nil || len(relayInfo.ParamOverrideAudit) == 0 {
		return
	}
	other.SetPublic("po", relayInfo.ParamOverrideAudit)
}

func appendStreamStatus(relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil || other == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	streamInfo := map[string]any{
		"status":     status,
		"end_reason": string(ss.EndReason),
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.ErrorCount > 0 {
		streamInfo["error_count"] = ss.ErrorCount
		messages := make([]string, 0, len(ss.Errors))
		for _, e := range ss.Errors {
			messages = append(messages, e.Message)
		}
		streamInfo["errors"] = messages
	}
	other.SetPublic("stream_status", streamInfo)
}

func appendBillingInfo(relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil || other == nil {
		return
	}
	// billing_source: "wallet" or "subscription"
	if relayInfo.BillingSource != "" {
		other.SetPublic("billing_source", relayInfo.BillingSource)
	}
	if relayInfo.UserSetting.BillingPreference != "" {
		other.SetPublic("billing_preference", relayInfo.UserSetting.BillingPreference)
	}
	if relayInfo.BillingSource == "subscription" {
		if relayInfo.SubscriptionId != 0 {
			other.SetPublic("subscription_id", relayInfo.SubscriptionId)
		}
		if relayInfo.SubscriptionPreConsumed > 0 {
			other.SetPublic("subscription_pre_consumed", relayInfo.SubscriptionPreConsumed)
		}
		// post_delta: settlement delta applied after actual usage is known (can be negative for refund)
		if relayInfo.SubscriptionPostDelta != 0 {
			other.SetPublic("subscription_post_delta", relayInfo.SubscriptionPostDelta)
		}
		if relayInfo.SubscriptionPlanId != 0 {
			other.SetPublic("subscription_plan_id", relayInfo.SubscriptionPlanId)
		}
		if relayInfo.SubscriptionPlanTitle != "" {
			other.SetPublic("subscription_plan_title", relayInfo.SubscriptionPlanTitle)
		}
		// Compute "this request" subscription consumed + remaining
		consumed := relayInfo.SubscriptionPreConsumed + relayInfo.SubscriptionPostDelta
		usedFinal := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		if consumed < 0 {
			consumed = 0
		}
		if usedFinal < 0 {
			usedFinal = 0
		}
		if relayInfo.SubscriptionAmountTotal > 0 {
			remain := max(relayInfo.SubscriptionAmountTotal-usedFinal, 0)
			other.SetPublic("subscription_total", relayInfo.SubscriptionAmountTotal)
			other.SetPublic("subscription_used", usedFinal)
			other.SetPublic("subscription_remain", remain)
		}
		if consumed > 0 {
			other.SetPublic("subscription_consumed", consumed)
		}
		// A subscription settlement may charge its excess to the wallet.
		other.SetPublic("wallet_quota_deducted", relayInfo.SubscriptionWalletConsumed)
	}
}

func appendRequestConversionChain(relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil || other == nil {
		return
	}
	if len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) == 0 {
		return
	}
	other.SetPublic("request_conversion", chain)
}

func appendFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other *model.LogOther) {
	if relayInfo == nil || other == nil {
		return
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		// claude indicates the final upstream request format is Claude Messages.
		// Frontend log rendering uses this to keep the original Claude input display.
		other.SetPublic("claude", true)
	}
}

func GenerateWssOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) *model.LogOther {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info.SetPublic("ws", true)
	info.SetPublic("audio_input", usage.InputTokenDetails.AudioTokens)
	info.SetPublic("audio_output", usage.OutputTokenDetails.AudioTokens)
	info.SetPublic("text_input", usage.InputTokenDetails.TextTokens)
	info.SetPublic("text_output", usage.OutputTokenDetails.TextTokens)
	info.SetPublic("audio_ratio", audioRatio)
	info.SetPublic("audio_completion_ratio", audioCompletionRatio)
	return info
}

func GenerateAudioOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, modelRatio, groupRatio, completionRatio, audioRatio, audioCompletionRatio, modelPrice, userGroupRatio float64) *model.LogOther {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, 0, 0.0, modelPrice, userGroupRatio)
	info.SetPublic("audio", true)
	info.SetPublic("audio_input", usage.PromptTokensDetails.AudioTokens)
	info.SetPublic("audio_output", usage.CompletionTokenDetails.AudioTokens)
	info.SetPublic("text_input", usage.PromptTokensDetails.TextTokens)
	info.SetPublic("text_output", usage.CompletionTokenDetails.TextTokens)
	info.SetPublic("audio_ratio", audioRatio)
	info.SetPublic("audio_completion_ratio", audioCompletionRatio)
	return info
}

func GenerateClaudeOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64,
	cacheTokens int, cacheRatio float64,
	cacheCreationTokens int, cacheCreationRatio float64,
	cacheCreationTokens5m int, cacheCreationRatio5m float64,
	cacheCreationTokens1h int, cacheCreationRatio1h float64,
	modelPrice float64, userGroupRatio float64) *model.LogOther {
	info := GenerateTextOtherInfo(ctx, relayInfo, modelRatio, groupRatio, completionRatio, cacheTokens, cacheRatio, modelPrice, userGroupRatio)
	info.SetPublic("claude", true)
	info.SetPublic("cache_creation_tokens", cacheCreationTokens)
	info.SetPublic("cache_creation_ratio", cacheCreationRatio)
	if cacheCreationTokens5m != 0 {
		info.SetPublic("cache_creation_tokens_5m", cacheCreationTokens5m)
		info.SetPublic("cache_creation_ratio_5m", cacheCreationRatio5m)
	}
	if cacheCreationTokens1h != 0 {
		info.SetPublic("cache_creation_tokens_1h", cacheCreationTokens1h)
		info.SetPublic("cache_creation_ratio_1h", cacheCreationRatio1h)
	}
	return info
}

func GenerateMjOtherInfo(relayInfo *relaycommon.RelayInfo, priceData hosttypes.PriceData) *model.LogOther {
	other := model.NewLogOther()
	other.SetPublic("model_price", priceData.ModelPrice)
	other.SetPublic("group_ratio", priceData.GroupRatioInfo.GroupRatio)
	if priceData.GroupRatioInfo.HasSpecialRatio {
		other.SetPublic("user_group_ratio", priceData.GroupRatioInfo.GroupSpecialRatio)
	}
	attachChannelRateLimitQueueTime(other, relayInfo)
	attachSmartProtectionReviewTime(other, relayInfo)
	appendRequestPath(nil, relayInfo, other)
	return other
}

// InjectTieredBillingInfo overlays tiered billing fields onto an existing
// module-specific other map. Call this after GenerateTextOtherInfo /
// GenerateClaudeOtherInfo / etc. when the request used tiered_expr billing.
func InjectTieredBillingInfo(other *model.LogOther, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if relayInfo == nil || other == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil {
		return
	}
	other.SetPublic("billing_mode", "tiered_expr")
	other.SetPublic("expr_b64", base64.StdEncoding.EncodeToString([]byte(snap.ExprString)))
	if result != nil {
		if tokens := result.BillingTokens; tokens != nil && result.BillingUnit == billingexpr.BillingUnitToken {
			other.SetPublic("image_cache_tokens", tokens.ImgCR)
			other.SetPublic("billing_tokens", map[string]float64{
				"p": tokens.P, "c": tokens.C, "len": tokens.Len,
				"cr": tokens.CR, "cc": tokens.CC, "cc1h": tokens.CC1h,
				"img": tokens.Img, "img_cr": tokens.ImgCR, "img_o": tokens.ImgO,
				"ai": tokens.AI, "ao": tokens.AO,
			})
		}
		if result.ImageCount != nil {
			other.SetPublic("image_count", *result.ImageCount)
		}
		other.SetPublic("matched_tier", result.MatchedTier)
		if result.BillingUnit != "" {
			other.SetPublic("billing_unit", result.BillingUnit)
		}
		if result.FixedPrice != nil {
			other.SetPublic("fixed_price", *result.FixedPrice)
		}
		if len(result.RequestRules) > 0 {
			other.SetPublic("request_rules", result.RequestRules)
		}
	} else if snap.EstimatedBillingUnit != "" {
		if snap.EstimatedImageCount != nil {
			other.SetPublic("image_count", *snap.EstimatedImageCount)
		}
		other.SetPublic("matched_tier", snap.EstimatedTier)
		other.SetPublic("billing_unit", snap.EstimatedBillingUnit)
		if snap.EstimatedFixedPrice != nil {
			other.SetPublic("fixed_price", *snap.EstimatedFixedPrice)
		}
	}
}

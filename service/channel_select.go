package service

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

func GetChannelConstraints(c *gin.Context) *dto.ChannelConstraints {
	if c == nil {
		return &dto.ChannelConstraints{}
	}
	if existing, ok := common.GetContextKeyType[*dto.ChannelConstraints](c, constant.ContextKeyChannelConstraints); ok && existing != nil {
		return existing
	}
	constraints := &dto.ChannelConstraints{}
	common.SetContextKey(c, constant.ContextKeyChannelConstraints, constraints)
	return constraints
}

func AppendTaskPluginIdentityFilter(c *gin.Context, pluginKey string) {
	if c == nil {
		return
	}
	channelTypes, pluginKeys := pinnedTaskPluginIdentities(c, pluginKey)
	GetChannelConstraints(c).AddFilter(dto.ChannelFilter{
		Kind:                   dto.FilterTaskPluginIdentity,
		TaskPluginKey:          pluginKey,
		TaskPluginChannelTypes: channelTypes,
		TaskPluginKeys:         pluginKeys,
	})
}

type RetryParam struct {
	Ctx                   *gin.Context
	TokenGroup            string
	ModelName             string
	RequestPath           string
	Retry                 *int
	attempted             map[int]struct{}
	rateLimited           map[int]struct{}
	retryAfterMs          int64
	rateLimitErr          error
	lastAttemptedPriority map[string]int64
}

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) MarkAttempted(channelId int) {
	if channelId <= 0 {
		return
	}
	if p.attempted == nil {
		p.attempted = make(map[int]struct{})
	}
	p.attempted[channelId] = struct{}{}
}

func (p *RetryParam) MarkChannelAttempted(channel *model.Channel) {
	p.MarkAttempted(channel.Id)
	if p.lastAttemptedPriority == nil {
		p.lastAttemptedPriority = make(map[string]int64)
	}
	group := common.GetContextKeyString(p.Ctx, constant.ContextKeyUsingGroup)
	p.lastAttemptedPriority[group] = channel.GetPriority()
}

func (p *RetryParam) HasAttempted(channelId int) bool {
	if p == nil || p.attempted == nil {
		return false
	}
	_, ok := p.attempted[channelId]
	return ok
}

func (p *RetryParam) MarkRateLimited(channelId int, retryAfterMillis int64) {
	if p == nil || channelId <= 0 {
		return
	}
	if p.rateLimited == nil {
		p.rateLimited = make(map[int]struct{})
	}
	p.rateLimited[channelId] = struct{}{}
	if retryAfterMillis > 0 && (p.retryAfterMs == 0 || retryAfterMillis < p.retryAfterMs) {
		p.retryAfterMs = retryAfterMillis
	}
}

func (p *RetryParam) HasRateLimited(channelId int) bool {
	if p == nil || p.rateLimited == nil {
		return false
	}
	_, ok := p.rateLimited[channelId]
	return ok
}

func (p *RetryParam) HasRateLimitedChannels() bool {
	return p != nil && len(p.rateLimited) > 0
}

func (p *RetryParam) RateLimitRetryAfterMillis() int64 {
	if p == nil {
		return 0
	}
	return p.retryAfterMs
}

func (p *RetryParam) MarkRateLimitUnavailable(channelId int, err error) {
	p.MarkRateLimited(channelId, 0)
	if p != nil && p.rateLimitErr == nil {
		p.rateLimitErr = err
	}
}

func (p *RetryParam) RateLimitError() error {
	if p == nil {
		return nil
	}
	return p.rateLimitErr
}

func (p *RetryParam) ResetRateLimitState() {
	if p == nil {
		return
	}
	p.rateLimited = nil
	p.retryAfterMs = 0
	p.rateLimitErr = nil
}

// CacheGetRandomSatisfiedChannel keeps the request retry index monotonic.
// Auto groups share that index through context because Distributor selects the
// first channel before the relay controller creates its own RetryParam.
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*model.Channel, string, error) {
	if param.TokenGroup != "auto" {
		if IsIntelligentSchedulingForGroup(param.TokenGroup) {
			channel, err := selectIntelligentChannel(param, param.TokenGroup)
			return channel, param.TokenGroup, err
		}
		channel, err := selectLegacyChannel(param, param.TokenGroup, param.GetRetry())
		return channel, param.TokenGroup, err
	}

	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	autoGroups := GetRequestAutoGroups(param.Ctx, userGroup)
	if len(autoGroups) == 0 {
		return nil, param.TokenGroup, errors.New("auto groups is not enabled")
	}
	crossGroupRetry := common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)
	startGroupIndex, hasGroupIndex := common.GetContextKeyType[int](param.Ctx, constant.ContextKeyAutoGroupIndex)
	if !hasGroupIndex {
		// Legacy session affinity may have selected a group without invoking
		// this selector. Continue from that group instead of restarting auto.
		selectedGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyAutoGroup)
		for index, group := range autoGroups {
			if group == selectedGroup {
				startGroupIndex = index
				break
			}
		}
	}
	startRetryIndex, _ := common.GetContextKeyType[int](param.Ctx, constant.ContextKeyAutoGroupRetryIndex)
	selectGroup := param.TokenGroup
	for index := startGroupIndex; index < len(autoGroups); index++ {
		selectGroup = autoGroups[index]
		priorityRetry := param.GetRetry() - startRetryIndex
		if index > startGroupIndex {
			startRetryIndex = param.GetRetry()
			priorityRetry = 0
		}
		logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", selectGroup, priorityRetry)

		var channel *model.Channel
		var err error
		if IsIntelligentSchedulingForGroup(selectGroup) {
			channel, err = selectIntelligentChannel(param, selectGroup)
		} else if priorityRetry <= max(common.RetryTimes, 0) {
			channel, err = selectLegacyChannel(param, selectGroup, priorityRetry)
		}
		if err != nil {
			return nil, selectGroup, err
		}
		if channel != nil {
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, index)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, startRetryIndex)
			common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, selectGroup)
			return channel, selectGroup, nil
		}
		// Empty groups can be skipped on initial selection. Once an upstream
		// attempt failed, moving groups must honor the token's explicit opt-in.
		if !crossGroupRetry && (param.GetRetry() > 0 || len(param.attempted) > 0) {
			return nil, selectGroup, nil
		}
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, index+1)
		common.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, param.GetRetry())
	}
	return nil, selectGroup, nil
}

func pinnedTaskPluginIdentities(c *gin.Context, expected string) ([]int, []string) {
	if c == nil || expected == "" {
		return nil, nil
	}
	if value, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint); exists {
		pinned, ok := value.(jsplugin.PinnedEndpoint)
		if ok && pinned.Generation != nil && len(pinned.Candidates) > 1 {
			expectedFound := false
			channelTypes := make([]int, 0, len(pinned.Candidates))
			pluginKeys := make([]string, 0, len(pinned.Candidates))
			seen := make(map[int]struct{}, len(pinned.Candidates))
			for _, candidate := range pinned.Candidates {
				if candidate.Plugin == nil {
					continue
				}
				if candidate.Plugin.Meta.Key == expected {
					expectedFound = true
				}
				pluginKeys = append(pluginKeys, candidate.Plugin.Meta.Key)
				for _, channelType := range candidate.Plugin.Meta.ChannelTypes {
					if channelType == 0 || channelType == constant.ChannelTypeTaskPlugin {
						continue
					}
					if _, duplicate := seen[channelType]; duplicate {
						continue
					}
					if plugin, indexed := pinned.Generation.GetByChannelType(channelType); indexed && plugin == candidate.Plugin {
						seen[channelType] = struct{}{}
						channelTypes = append(channelTypes, channelType)
					}
				}
			}
			if expectedFound {
				return channelTypes, pluginKeys
			}
		}
	}
	value, exists := c.Get(jsplugin.ContextKeyPinnedPlugin)
	pinned, ok := value.(jsplugin.PinnedPlugin)
	if !exists || !ok || pinned.Generation == nil || pinned.Plugin == nil || pinned.Plugin.Meta.Key != expected {
		return nil, nil
	}
	channelTypes := make([]int, 0, len(pinned.Plugin.Meta.ChannelTypes))
	for _, channelType := range pinned.Plugin.Meta.ChannelTypes {
		if channelType == 0 || channelType == constant.ChannelTypeTaskPlugin {
			continue
		}
		channelTypes = append(channelTypes, channelType)
	}
	return channelTypes, []string{expected}
}

// ChannelSelectError explains why SelectChannelForRequest found no channel.
// Callers render it for their transport: the HTTP distributor localizes
// MessageID with its own helpers and the Responses WebSocket relay wraps it in
// a NewAPIError. Message is set instead of MessageID when the text is a fixed
// error code that clients match on.
type ChannelSelectError struct {
	StatusCode int
	Code       types.ErrorCode
	MessageID  string
	Params     map[string]any
	Message    string
	// FilterKind and Channel identify a candidate rejected by request filters.
	FilterKind dto.ChannelFilterKind
	Channel    *model.Channel
	// NoAvailableChannel marks the "no channel for this group and model"
	// outcome so the distributor can name the claiming task plugin.
	NoAvailableChannel bool
}

// SelectChannelForRequest resolves the channel for one attempt with the rules
// shared by the HTTP distributor and the Responses WebSocket relay: a pinned
// channel wins, then session affinity (first attempt only), then a random
// eligible channel; every candidate must satisfy the request's channel
// filters. The group the channel was chosen from is returned for auto-group
// callers. The caller still applies SetupContextForSelectedChannel.
func SelectChannelForRequest(c *gin.Context, modelName string, retry *RetryParam) (*model.Channel, string, *ChannelSelectError) {
	constraints := GetChannelConstraints(c)
	if pin, found, overridden := constraints.ResolvedPin(); found {
		for _, lost := range overridden {
			logger.LogWarn(c, common.LogText(
				"channel pin overridden: winning_source=%s winning_channel_id=%d overridden_source=%s overridden_channel_id=%d",
				pin.Source, pin.ChannelId, lost.Source, lost.ChannelId,
			))
		}
		channel, err := model.CacheGetChannel(pin.ChannelId)
		if err != nil {
			return nil, "", pinnedChannelUnavailable(pin, http.StatusBadRequest, i18n.MsgDistributorInvalidChannelId)
		}
		if channel.Status != common.ChannelStatusEnabled {
			return nil, "", pinnedChannelUnavailable(pin, http.StatusForbidden, i18n.MsgDistributorChannelDisabled)
		}
		if ok, kind := model.ChannelSatisfiesFilters(channel, modelName, constraints.Filters); !ok {
			return nil, "", &ChannelSelectError{
				StatusCode: http.StatusBadRequest, Code: types.ErrorCode(kind), MessageID: i18n.MsgDistributorNoAvailableChannel,
				Params:     map[string]any{"Group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup), "Model": modelName},
				FilterKind: kind, Channel: channel,
			}
		}
		return channel, "", nil
	}

	usingGroup := retry.TokenGroup
	var channel *model.Channel
	var selectGroup string
	if retry.GetRetry() == 0 {
		if preferredChannelID, found := GetPreferredChannelByAffinity(c, modelName, usingGroup); found {
			affinityUsable := false
			preferred, err := model.CacheGetChannel(preferredChannelID)
			affinitySatisfied := false
			if err == nil && preferred != nil && preferred.Status == common.ChannelStatusEnabled {
				affinitySatisfied, _ = model.ChannelSatisfiesFilters(preferred, modelName, constraints.Filters)
			}
			if affinitySatisfied {
				if usingGroup == "auto" {
					userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
					for _, g := range GetRequestAutoGroups(c, userGroup) {
						if model.IsChannelEnabledForGroupModel(g, modelName, preferred.Id) {
							selectGroup = g
							common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
							channel = preferred
							affinityUsable = true
							MarkChannelAffinityUsed(c, g, preferred.Id)
							break
						}
					}
				} else if model.IsChannelEnabledForGroupModel(usingGroup, modelName, preferred.Id) {
					channel = preferred
					selectGroup = usingGroup
					affinityUsable = true
					MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
				}
			}
			if !affinityUsable && !ShouldKeepChannelAffinityOnChannelDisabled() {
				ClearCurrentChannelAffinityCache(c)
			}
			if !affinityUsable && RequestPolicy(c).SessionMode == "strict" {
				return nil, "", &ChannelSelectError{StatusCode: http.StatusServiceUnavailable, Message: "strict_session_binding_unavailable"}
			}
		}
	}

	if channel == nil {
		var err error
		channel, selectGroup, err = CacheGetRandomSatisfiedChannel(retry)
		if err != nil {
			showGroup := usingGroup
			if usingGroup == "auto" {
				showGroup = fmt.Sprintf("auto(%s)", selectGroup)
			}
			return nil, selectGroup, &ChannelSelectError{
				StatusCode: http.StatusServiceUnavailable, Code: types.ErrorCodeModelNotFound, MessageID: i18n.MsgDistributorGetChannelFailed,
				Params: map[string]any{"Group": showGroup, "Model": modelName, "Error": err.Error()},
			}
		}
		if channel == nil {
			return nil, selectGroup, &ChannelSelectError{
				StatusCode: http.StatusServiceUnavailable, Code: types.ErrorCodeModelNotFound, MessageID: i18n.MsgDistributorNoAvailableChannel,
				Params: map[string]any{"Group": usingGroup, "Model": modelName}, NoAvailableChannel: true,
			}
		}
	}
	if ok, kind := model.ChannelSatisfiesFilters(channel, modelName, constraints.Filters); !ok {
		return nil, selectGroup, &ChannelSelectError{
			StatusCode: http.StatusServiceUnavailable, Code: types.ErrorCodeModelNotFound, MessageID: i18n.MsgDistributorNoAvailableChannel,
			Params:     map[string]any{"Group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup), "Model": modelName},
			FilterKind: kind, Channel: channel, NoAvailableChannel: true,
		}
	}
	return channel, selectGroup, nil
}

// Origin-task pins report a fixed code so task polling can tell a retired
// channel from a malformed request.
func pinnedChannelUnavailable(pin dto.ChannelPin, statusCode int, messageID string) *ChannelSelectError {
	if pin.Source == dto.PinSourceOriginTask {
		return &ChannelSelectError{StatusCode: http.StatusBadRequest, Code: "origin_task_channel_disabled", Message: "origin_task_channel_disabled"}
	}
	return &ChannelSelectError{StatusCode: statusCode, MessageID: messageID}
}

// AppendUsedChannel records an attempted channel in the request's channel
// trail, which the retry log and the consume log's admin_info both read.
func AppendUsedChannel(c *gin.Context, channelID int) {
	c.Set("use_channel", append(c.GetStringSlice("use_channel"), fmt.Sprintf("%d", channelID)))
}

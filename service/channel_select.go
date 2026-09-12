package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
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

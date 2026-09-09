package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type schedulingPoolKey struct {
	Group    string
	Model    string
	Priority int64
}

const intelligentSchedulingBaseWeight = 100

const channelFailureShardCount = 64

type channelSchedulingTtftSample struct {
	Ts     int64
	TtftMs int64
}

type channelSchedulingState struct {
	ChannelId         int
	ChannelName       string
	BaseWeight        int
	CurrentWeight     float64
	EffectiveWeight   float64
	PerformanceFactor float64
	EstimatedTtftMs   float64
	LastTtftMs        int64
	Inflight          int64
	RequestCount      int64
	SuccessCount      int64
	ErrorCount        int64
	AffinityHits      int64
	Samples           int64
	TtftSamples       []channelSchedulingTtftSample
	LastSelectedAt    int64
	LastCompletedAt   int64
	LastError         string
	Buckets           map[int64]*ChannelSchedulingBucket
}

type channelSchedulingPool struct {
	mu       sync.Mutex
	channels map[int]*channelSchedulingState
}

type channelModelKey struct {
	ChannelId int
	Model     string
}

type channelFailureStreak struct {
	Count         int
	LastFailureAt time.Time
	LastOutcomeAt time.Time
}

type channelFailureShard struct {
	mu      sync.Mutex
	streaks map[int]channelFailureStreak
}

type ChannelSchedulingBucket struct {
	Ts              int64   `json:"ts"`
	RequestCount    int64   `json:"request_count"`
	SuccessCount    int64   `json:"success_count"`
	ErrorCount      int64   `json:"error_count"`
	TtftSumMs       int64   `json:"-"`
	TtftCount       int64   `json:"-"`
	AvgTtftMs       int64   `json:"avg_ttft_ms"`
	EffectiveWeight float64 `json:"effective_weight"`
	Inflight        int64   `json:"inflight"`
}

type ChannelSchedulingChannelSnapshot struct {
	ChannelId         int                       `json:"channel_id"`
	ChannelName       string                    `json:"channel_name"`
	Group             string                    `json:"group"`
	Model             string                    `json:"model"`
	Priority          int64                     `json:"priority"`
	BaseWeight        int                       `json:"base_weight"`
	EffectiveWeight   float64                   `json:"effective_weight"`
	PerformanceFactor float64                   `json:"performance_factor"`
	EstimatedTtftMs   int64                     `json:"estimated_ttft_ms"`
	LastTtftMs        int64                     `json:"last_ttft_ms"`
	Inflight          int64                     `json:"inflight"`
	RequestCount      int64                     `json:"request_count"`
	SuccessCount      int64                     `json:"success_count"`
	ErrorCount        int64                     `json:"error_count"`
	AffinityHits      int64                     `json:"affinity_hits"`
	Samples           int64                     `json:"samples"`
	ActualShare       float64                   `json:"actual_share"`
	State             string                    `json:"state"`
	LastError         string                    `json:"last_error"`
	LastSelectedAt    int64                     `json:"last_selected_at"`
	LastCompletedAt   int64                     `json:"last_completed_at"`
	Series            []ChannelSchedulingBucket `json:"series"`
}

type ChannelSchedulingEvent struct {
	Ts          int64  `json:"ts"`
	Type        string `json:"type"`
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Group       string `json:"group"`
	Model       string `json:"model"`
	Priority    int64  `json:"priority"`
	Message     string `json:"message"`
}

type ChannelSchedulingChannelFault struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Reason      string `json:"reason"`
	DisabledAt  int64  `json:"disabled_at"`
}

type ChannelSchedulingOverview struct {
	Strategy         operation_setting.ChannelSchedulingSetting `json:"strategy"`
	GeneratedAt      int64                                      `json:"generated_at"`
	ActivePools      int                                        `json:"active_pools"`
	NormalChannels   int                                        `json:"normal_channels"`
	DegradedChannels int                                        `json:"degraded_channels"`
	DisabledModels   int                                        `json:"disabled_models"`
	DisabledChannels int                                        `json:"disabled_channels"`
	Inflight         int64                                      `json:"inflight"`
	Requests         int64                                      `json:"requests"`
	Rps              float64                                    `json:"rps"`
	Errors           int64                                      `json:"errors"`
	AvgTtftMs        int64                                      `json:"avg_ttft_ms"`
	Channels         []ChannelSchedulingChannelSnapshot         `json:"channels"`
	Faults           []model.ChannelModelState                  `json:"faults"`
	ChannelFaults    []ChannelSchedulingChannelFault            `json:"channel_faults"`
	Events           []ChannelSchedulingEvent                   `json:"events"`
}

var channelScheduler = struct {
	poolsMu          sync.RWMutex
	pools            map[schedulingPoolKey]*channelSchedulingPool
	disabledMu       sync.RWMutex
	disabled         map[channelModelKey]model.ChannelModelState
	disabledChannels map[int]struct{}
	failureShards    [channelFailureShardCount]channelFailureShard
	eventsMu         sync.Mutex
	events           []ChannelSchedulingEvent
}{
	pools:            make(map[schedulingPoolKey]*channelSchedulingPool),
	disabled:         make(map[channelModelKey]model.ChannelModelState),
	disabledChannels: make(map[int]struct{}),
}

var channelSchedulerSyncOnce sync.Once
var channelSchedulerDisableMu sync.Mutex

func InitChannelScheduler() {
	reloadChannelSchedulerFaults()
	channelSchedulerSyncOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				reloadChannelSchedulerFaults()
				now := time.Now()
				cleanupChannelSchedulingPools(now.Unix(), operation_setting.GetChannelSchedulingSetting().RealtimeRetentionMin)
				cleanupChannelFailureStreaks(now)
			}
		}()
	})
}

func reloadChannelSchedulerFaults() {
	states, err := model.ListChannelModelStates(true)
	if err != nil {
		common.SysError("failed to load channel scheduling faults: " + err.Error())
		return
	}
	disabledChannels, channelErr := model.ListAutoDisabledChannels()
	if channelErr != nil {
		common.SysError("failed to load disabled scheduling channels: " + channelErr.Error())
		return
	}
	channelScheduler.disabledMu.Lock()
	defer channelScheduler.disabledMu.Unlock()
	channelScheduler.disabled = make(map[channelModelKey]model.ChannelModelState, len(states))
	channelScheduler.disabledChannels = make(map[int]struct{}, len(disabledChannels))
	for _, state := range states {
		channelScheduler.disabled[channelModelKey{ChannelId: state.ChannelId, Model: state.Model}] = state
	}
	for _, channel := range disabledChannels {
		if model.IsChannelSchedulingFault(&channel) {
			channelScheduler.disabledChannels[channel.Id] = struct{}{}
		}
	}
}

func IsIntelligentSchedulingForGroup(group string) bool {
	return operation_setting.IsIntelligentChannelScheduling(group)
}

func ChannelUsesIntelligentScheduling(channel *model.Channel) bool {
	_, ok := GetIntelligentSchedulingGroupForChannel(channel)
	return ok
}

func GetIntelligentSchedulingGroupForChannel(channel *model.Channel) (string, bool) {
	if channel == nil {
		return "", false
	}
	for _, group := range strings.Split(channel.Group, ",") {
		group = strings.TrimSpace(group)
		if group != "" && IsIntelligentSchedulingForGroup(group) {
			return group, true
		}
	}
	return "", false
}

func ShouldRetryIntelligentSchedulingError(group string, info *relaycommon.RelayInfo, relayErr *types.NewAPIError) bool {
	return IsIntelligentSchedulingForGroup(group) && info != nil && !info.UpstreamStartTime.IsZero() && isIntelligentSchedulingUpstreamError(relayErr)
}

func IsHandledByIntelligentChannelScheduling(group string, relayErr *types.NewAPIError) bool {
	return IsIntelligentSchedulingForGroup(group) && isIntelligentSchedulingUpstreamError(relayErr)
}

func IsIntelligentSchedulingFaultError(relayErr *types.NewAPIError) bool {
	return isIntelligentSchedulingUpstreamError(relayErr)
}

func IsIntelligentSchedulingForRequest(param *RetryParam) bool {
	if param == nil {
		return false
	}
	if param.TokenGroup != "auto" {
		return IsIntelligentSchedulingForGroup(param.TokenGroup)
	}
	userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
	for _, group := range GetRequestAutoGroups(param.Ctx, userGroup) {
		if IsIntelligentSchedulingForGroup(group) {
			return true
		}
	}
	return false
}

func ShouldUseChannelAffinityForRequest(param *RetryParam) bool {
	return !IsIntelligentSchedulingForRequest(param)
}

// ShouldUseSoftChannelAffinityForRequest is the persisted compatibility name
// for intelligent session channel binding. It is deliberately opt-in and
// never affects legacy groups.
func ShouldUseSoftChannelAffinityForRequest(param *RetryParam) bool {
	return IsIntelligentSchedulingForRequest(param) && operation_setting.GetChannelSchedulingSetting().SoftAffinityEnabled
}

// ShouldUseSessionChannelBinding is the semantic name for the opt-in
// intelligent-scheduling affinity mode. The old function remains as a
// compatibility alias for callers and persisted configuration migrations.
func ShouldUseSessionChannelBinding(param *RetryParam) bool {
	return ShouldUseSoftChannelAffinityForRequest(param)
}

func GetRelayAttemptLimit(param *RetryParam) int {
	if IsIntelligentSchedulingForRequest(param) {
		return operation_setting.GetChannelSchedulingSetting().MaxAttempts
	}
	limit := common.RetryTimes + 1
	if limit < 1 {
		limit = 1
	}
	if param != nil && param.TokenGroup == "auto" && common.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry) {
		userGroup := common.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)
		groupCount := len(GetRequestAutoGroups(param.Ctx, userGroup))
		if groupCount > 1 {
			// Legacy retries are a per-group budget. Keep a separate bounded
			// request limit so exhausting one group can actually reach the next.
			if limit > math.MaxInt/groupCount {
				return math.MaxInt
			}
			limit *= groupCount
		}
	}
	return limit
}

func selectLegacyChannel(param *RetryParam, group string, retry int) (*model.Channel, error) {
	normalizedModel := ratio_setting.FormatMatchingModelName(param.ModelName)
	candidates, err := model.GetSatisfiedChannels(group, param.ModelName, param.RequestPath)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	available := make([]*model.Channel, 0, len(candidates))
	for _, channel := range candidates {
		if isChannelModelSchedulingDisabled(channel.Id, normalizedModel) || param.HasRateLimited(channel.Id) {
			continue
		}
		available = append(available, channel)
	}
	if len(available) == 0 {
		return nil, nil
	}

	prioritySnapshots, _ := common.GetContextKeyType[map[string][]int64](param.Ctx, constant.ContextKeyChannelRetryPriorities)
	if prioritySnapshots == nil {
		prioritySnapshots = make(map[string][]int64)
		common.SetContextKey(param.Ctx, constant.ContextKeyChannelRetryPriorities, prioritySnapshots)
	}
	priorities, hasSnapshot := prioritySnapshots[group]
	if !hasSnapshot {
		priorities = make([]int64, 0, len(available))
	}
	prioritySet := make(map[int64]bool, len(priorities))
	for _, priority := range priorities {
		prioritySet[priority] = true
	}
	for _, channel := range available {
		priority := channel.GetPriority()
		if !prioritySet[priority] {
			priorities = append(priorities, priority)
			prioritySet[priority] = true
		}
	}
	// Session affinity can choose the first channel outside this selector.
	// Keep its tier even if the failure removed that channel from the pool.
	if priority, attempted := param.lastAttemptedPriority[group]; attempted && !prioritySet[priority] {
		priorities = append(priorities, priority)
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	prioritySnapshots[group] = priorities
	if retry >= len(priorities) {
		retry = len(priorities) - 1
	}
	if retry < 0 {
		retry = 0
	}
	// Keep the original priority ranks when excluding attempts; deleting a
	// failed tier before indexing would skip the immediately lower priority.
	for _, targetPriority := range priorities[retry:] {
		tier := make([]*model.Channel, 0, len(available))
		for _, channel := range available {
			if channel.GetPriority() == targetPriority && !param.HasAttempted(channel.Id) {
				tier = append(tier, channel)
			}
		}
		if len(tier) > 0 {
			return selectLegacyWeightedChannel(tier), nil
		}
	}
	return nil, nil
}

func selectLegacyWeightedChannel(channels []*model.Channel) *model.Channel {
	if len(channels) == 0 {
		return nil
	}
	if len(channels) == 1 {
		return channels[0]
	}

	sumWeight := 0
	for _, channel := range channels {
		weight := channel.GetWeight()
		if weight > 0 {
			sumWeight += weight
		}
	}
	if sumWeight == 0 {
		return channels[common.GetRandomInt(len(channels))]
	}

	smoothingFactor := 1
	if sumWeight/len(channels) < 10 {
		smoothingFactor = 100
	}
	randomWeight := common.GetRandomInt(sumWeight * smoothingFactor)
	for _, channel := range channels {
		weight := channel.GetWeight()
		if weight <= 0 {
			continue
		}
		randomWeight -= weight * smoothingFactor
		if randomWeight < 0 {
			return channel
		}
	}
	return channels[len(channels)-1]
}

// selectBoundSessionChannel returns a previously assigned channel when it is
// still a valid candidate for this request. A binding is a routing decision,
// not a performance hint: once established it is reused until the channel is
// unavailable, rate-limited for this attempt, or excluded by retry state.
func selectBoundSessionChannel(param *RetryParam, group string, candidates []*model.Channel) (*model.Channel, bool) {
	if !ShouldUseSessionChannelBinding(param) || param == nil || param.Ctx == nil {
		return nil, false
	}
	preferredID, found := GetPreferredChannelByAffinity(param.Ctx, param.ModelName, group)
	if !found || preferredID <= 0 || param.HasAttempted(preferredID) || param.HasRateLimited(preferredID) {
		if found {
			ClearCurrentChannelAffinityCache(param.Ctx)
		}
		return nil, false
	}

	for _, candidate := range candidates {
		if candidate == nil || candidate.Id != preferredID || candidate.Status != common.ChannelStatusEnabled {
			continue
		}
		if !IsChannelUsableForScheduling(group, param.ModelName, candidate.Id) {
			break
		}
		MarkChannelAffinityUsed(param.Ctx, group, candidate.Id)
		return candidate, true
	}
	ClearCurrentChannelAffinityCache(param.Ctx)
	return nil, false
}

// bindSessionChannelIfAbsent reserves the first channel chosen for a new
// session. Set-if-absent makes the binding first-writer-wins across processes;
// a concurrent loser re-reads the established channel and uses it instead.
func bindSessionChannelIfAbsent(param *RetryParam, group string, selected *model.Channel, candidates []*model.Channel) *model.Channel {
	if !ShouldUseSessionChannelBinding(param) || param == nil || param.Ctx == nil || selected == nil {
		return selected
	}
	cacheKey, ttlSeconds, ok := getChannelAffinityContext(param.Ctx)
	if !ok || cacheKey == "" {
		return selected
	}
	if ttlSeconds <= 0 {
		setting := operation_setting.GetChannelAffinitySetting()
		if setting != nil {
			ttlSeconds = setting.DefaultTTLSeconds
		}
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	cache := getChannelAffinityCache()
	bound, err := cache.SetIfAbsentWithTTL(cacheKey, selected.Id, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		common.SysError(fmt.Sprintf("channel affinity initial bind failed: key=%s, err=%v", cacheKey, err))
		return selected
	}
	if bound {
		return selected
	}
	preferredID, found, err := cache.Get(cacheKey)
	if err != nil || !found || preferredID == selected.Id {
		return selected
	}
	if param.HasAttempted(preferredID) || param.HasRateLimited(preferredID) {
		return selected
	}
	for _, candidate := range candidates {
		if candidate != nil && candidate.Id == preferredID && candidate.Status == common.ChannelStatusEnabled && IsChannelUsableForScheduling(group, param.ModelName, candidate.Id) {
			MarkChannelAffinityUsed(param.Ctx, group, candidate.Id)
			return candidate
		}
	}
	return selected
}

func selectIntelligentChannel(param *RetryParam, group string) (*model.Channel, error) {
	if param == nil {
		return nil, nil
	}
	if param.Ctx != nil {
		param.Ctx.Set(ginKeyChannelAffinityUsed, false)
		param.Ctx.Set(ginKeyChannelAffinityLogInfo, nil)
	}
	candidates, err := model.GetSatisfiedChannels(group, param.ModelName, param.RequestPath)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	normalizedModel := ratio_setting.FormatMatchingModelName(param.ModelName)
	active := make([]*model.Channel, 0, len(candidates))
	for _, channel := range candidates {
		if isChannelSchedulingDisabled(channel.Id, normalizedModel) {
			continue
		}
		active = append(active, channel)
	}
	if len(active) == 0 {
		return nil, nil
	}

	// A session binding is a routing decision. Reuse it while the channel is
	// still healthy and eligible; the scheduler is only consulted for new
	// sessions or after a binding becomes unusable.
	if bound, ok := selectBoundSessionChannel(param, group, active); ok {
		return bound, nil
	}

	for len(active) > 0 {
		targetPriority, activeTier := highestPrioritySchedulingTier(active)
		eligibleTier := make([]*model.Channel, 0, len(activeTier))
		for _, channel := range activeTier {
			if param.HasAttempted(channel.Id) || param.HasRateLimited(channel.Id) {
				continue
			}
			eligibleTier = append(eligibleTier, channel)
		}
		if len(eligibleTier) > 0 {
			key := schedulingPoolKey{Group: group, Model: normalizedModel, Priority: targetPriority}
			selected := selectSmoothWeightedEligibleChannel(key, activeTier, eligibleTier)
			return bindSessionChannelIfAbsent(param, group, selected, active), nil
		}
		remaining := active[:0]
		for _, channel := range active {
			if channel.GetPriority() != targetPriority {
				remaining = append(remaining, channel)
			}
		}
		active = remaining
	}
	return nil, nil
}

func highestPrioritySchedulingTier(channels []*model.Channel) (int64, []*model.Channel) {
	if len(channels) == 0 {
		return 0, nil
	}
	targetPriority := channels[0].GetPriority()
	for _, channel := range channels[1:] {
		if channel.GetPriority() > targetPriority {
			targetPriority = channel.GetPriority()
		}
	}
	tier := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel.GetPriority() == targetPriority {
			tier = append(tier, channel)
		}
	}
	return targetPriority, tier
}

func selectSmoothWeightedChannel(key schedulingPoolKey, candidates []*model.Channel) *model.Channel {
	return selectSmoothWeightedEligibleChannel(key, candidates, candidates)
}

func selectSmoothWeightedEligibleChannel(key schedulingPoolKey, activeCandidates []*model.Channel, eligibleCandidates []*model.Channel) *model.Channel {
	if len(activeCandidates) == 0 || len(eligibleCandidates) == 0 {
		return nil
	}
	pool := getSchedulingPool(key)
	pool.mu.Lock()
	defer pool.mu.Unlock()

	now := time.Now().Unix()
	active := make(map[int]*model.Channel, len(activeCandidates))
	for _, channel := range activeCandidates {
		active[channel.Id] = channel
		state, ok := pool.channels[channel.Id]
		if !ok {
			state = &channelSchedulingState{ChannelId: channel.Id, Buckets: make(map[int64]*ChannelSchedulingBucket)}
			pool.channels[channel.Id] = state
		}
		state.ChannelName = channel.Name
		state.BaseWeight = intelligentSchedulingBaseWeight
	}
	for id := range pool.channels {
		if _, ok := active[id]; !ok {
			delete(pool.channels, id)
		}
	}

	setting := operation_setting.GetChannelSchedulingSetting()
	refreshSchedulingPoolWeights(pool, setting, now)
	var selected *model.Channel
	var selectedState *channelSchedulingState
	totalWeight := 0.0
	for _, channel := range eligibleCandidates {
		state := pool.channels[channel.Id]
		weight := state.EffectiveWeight
		state.CurrentWeight += weight
		totalWeight += weight
		if selectedState == nil || state.CurrentWeight > selectedState.CurrentWeight ||
			(state.CurrentWeight == selectedState.CurrentWeight && channel.Id < selected.Id) {
			selected = channel
			selectedState = state
		}
	}
	if selectedState != nil {
		selectedState.CurrentWeight -= totalWeight
		selectedState.LastSelectedAt = now
	}
	return selected
}

func getSchedulingPool(key schedulingPoolKey) *channelSchedulingPool {
	channelScheduler.poolsMu.RLock()
	pool := channelScheduler.pools[key]
	channelScheduler.poolsMu.RUnlock()
	if pool != nil {
		return pool
	}
	channelScheduler.poolsMu.Lock()
	defer channelScheduler.poolsMu.Unlock()
	if pool = channelScheduler.pools[key]; pool == nil {
		pool = &channelSchedulingPool{channels: make(map[int]*channelSchedulingState)}
		channelScheduler.pools[key] = pool
	}
	return pool
}

func BeginScheduledChannelAttempt(c *gin.Context, info *relaycommon.RelayInfo, channel *model.Channel) {
	if info == nil || channel == nil || !IsIntelligentSchedulingForGroup(info.UsingGroup) {
		return
	}
	key := schedulingPoolKey{
		Group:    info.UsingGroup,
		Model:    ratio_setting.FormatMatchingModelName(info.OriginModelName),
		Priority: channel.GetPriority(),
	}
	pool := getSchedulingPool(key)
	pool.mu.Lock()
	if len(pool.channels) == 0 {
		pool.mu.Unlock()
		requestPath := ""
		if c != nil && c.Request != nil && c.Request.URL != nil {
			requestPath = c.Request.URL.Path
		}
		candidates, err := model.GetSatisfiedChannels(info.UsingGroup, info.OriginModelName, requestPath)
		pool.mu.Lock()
		if err == nil {
			for _, candidate := range candidates {
				if candidate.GetPriority() != key.Priority || isChannelSchedulingDisabled(candidate.Id, key.Model) {
					continue
				}
				pool.channels[candidate.Id] = &channelSchedulingState{
					ChannelId: candidate.Id, ChannelName: candidate.Name,
					BaseWeight: intelligentSchedulingBaseWeight, Buckets: make(map[int64]*ChannelSchedulingBucket),
				}
			}
		}
	}
	state, ok := pool.channels[channel.Id]
	if !ok {
		state = &channelSchedulingState{
			ChannelId:   channel.Id,
			ChannelName: channel.Name,
			BaseWeight:  intelligentSchedulingBaseWeight,
			Buckets:     make(map[int64]*ChannelSchedulingBucket),
		}
		pool.channels[channel.Id] = state
	}
	state.Inflight++
	state.RequestCount++
	now := time.Now().Unix()
	state.LastSelectedAt = now
	if c != nil && c.GetBool(ginKeyChannelAffinityUsed) {
		state.AffinityHits++
		// The marker belongs to this selection only. Consume it so a later
		// retry on another channel cannot be counted as an affinity hit.
		c.Set(ginKeyChannelAffinityUsed, false)
	}
	setting := operation_setting.GetChannelSchedulingSetting()
	refreshSchedulingPoolWeights(pool, setting, now)
	bucket := schedulingBucket(state, now)
	bucket.RequestCount++
	bucket.Inflight = state.Inflight
	bucket.EffectiveWeight = state.EffectiveWeight
	pool.mu.Unlock()

	info.SchedulerGroup = key.Group
	info.SchedulerModel = key.Model
	info.SchedulerPriority = key.Priority
	info.SchedulerChannelId = channel.Id
	if c != nil && c.Request != nil && c.Request.URL != nil {
		info.SchedulerRequestPath = c.Request.URL.Path
	} else {
		info.SchedulerRequestPath = ""
	}
	info.SchedulerStartedAt = now
	info.SchedulerAttemptActive = true
	info.ResetUpstreamTiming()
}

func FinishScheduledChannelAttempt(info *relaycommon.RelayInfo, channel *model.Channel, success bool, relayErr *types.NewAPIError) {
	if info == nil || channel == nil || !info.SchedulerAttemptActive || info.SchedulerChannelId != channel.Id {
		return
	}
	info.SchedulerAttemptActive = false
	key := schedulingPoolKey{Group: info.SchedulerGroup, Model: info.SchedulerModel, Priority: info.SchedulerPriority}
	pool := getSchedulingPool(key)
	now := time.Now()
	ttftMs, hasTiming := info.UpstreamFirstResponseDuration()
	setting := operation_setting.GetChannelSchedulingSetting()
	recordAttempt := !info.UpstreamStartTime.IsZero()
	countFailure := recordAttempt && !success && isIntelligentSchedulingUpstreamError(relayErr)

	pool.mu.Lock()
	state := pool.channels[channel.Id]
	if state == nil {
		state = &channelSchedulingState{ChannelId: channel.Id, ChannelName: channel.Name, BaseWeight: intelligentSchedulingBaseWeight, Buckets: make(map[int64]*ChannelSchedulingBucket)}
		pool.channels[channel.Id] = state
	}
	if state.Inflight > 0 {
		state.Inflight--
	}
	state.LastCompletedAt = now.Unix()
	bucket := schedulingBucket(state, now.Unix())
	bucket.Inflight = state.Inflight
	if !recordAttempt {
		if state.RequestCount > 0 {
			state.RequestCount--
		}
		startedBucket := schedulingBucket(state, info.SchedulerStartedAt)
		if startedBucket.RequestCount > 0 {
			startedBucket.RequestCount--
		}
	}
	if success && recordAttempt {
		state.SuccessCount++
		bucket.SuccessCount++
		if hasTiming {
			addSchedulingTtftSample(state, now.Unix(), ttftMs, setting)
			bucket.TtftSumMs += ttftMs
			bucket.TtftCount++
			bucket.AvgTtftMs = bucket.TtftSumMs / bucket.TtftCount
		}
	} else if countFailure {
		state.ErrorCount++
		bucket.ErrorCount++
		if relayErr != nil {
			state.LastError = relayErr.MaskSensitiveErrorWithStatusCode()
		}
	}
	refreshSchedulingPoolWeights(pool, setting, now.Unix())
	bucket.EffectiveWeight = state.EffectiveWeight
	pruneSchedulingBuckets(state, now.Unix(), setting.RealtimeRetentionMin)
	pool.mu.Unlock()

	if success && recordAttempt {
		recordIntelligentSchedulingSuccess(channel.Id, now)
	} else if countFailure {
		recordIntelligentSchedulingFailure(info, channel, relayErr, now)
	}
}

func HandleIntelligentSchedulingChannelTestFault(channel *model.Channel, modelName string, relayErr *types.NewAPIError) bool {
	group, ok := GetIntelligentSchedulingGroupForChannel(channel)
	if !ok || !isIntelligentSchedulingUpstreamError(relayErr) {
		return false
	}
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	info := &relaycommon.RelayInfo{
		SchedulerGroup:    group,
		SchedulerModel:    normalizedModel,
		SchedulerPriority: channel.GetPriority(),
	}
	return recordIntelligentSchedulingFailure(info, channel, relayErr, time.Now())
}

func RecordIntelligentSchedulingChannelTestSuccess(channel *model.Channel) {
	if ChannelUsesIntelligentScheduling(channel) {
		recordIntelligentSchedulingSuccess(channel.Id, time.Now())
	}
}

func CancelScheduledChannelAttempt(info *relaycommon.RelayInfo, channel *model.Channel) {
	if info == nil || channel == nil || !info.SchedulerAttemptActive {
		return
	}
	info.SchedulerAttemptActive = false
	pool := getSchedulingPool(schedulingPoolKey{Group: info.SchedulerGroup, Model: info.SchedulerModel, Priority: info.SchedulerPriority})
	pool.mu.Lock()
	if state := pool.channels[channel.Id]; state != nil {
		if state.Inflight > 0 {
			state.Inflight--
		}
		if state.RequestCount > 0 {
			state.RequestCount--
		}
		startedBucket := schedulingBucket(state, info.SchedulerStartedAt)
		if startedBucket.RequestCount > 0 {
			startedBucket.RequestCount--
		}
		now := time.Now().Unix()
		refreshSchedulingPoolWeights(pool, operation_setting.GetChannelSchedulingSetting(), now)
		bucket := schedulingBucket(state, now)
		bucket.Inflight = state.Inflight
		bucket.EffectiveWeight = state.EffectiveWeight
	}
	pool.mu.Unlock()
}

func IsChannelUsableForScheduling(group string, modelName string, channelId int) bool {
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	if isChannelSchedulingDisabled(channelId, normalizedModel) {
		return false
	}
	return true
}

func IsChannelModelUsableForScheduling(modelName string, channelId int) bool {
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	return !isChannelModelSchedulingDisabled(channelId, normalizedModel)
}

func RestoreScheduledChannelModel(channelId int, modelName string) (bool, error) {
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	changed, err := model.RestoreChannelModel(channelId, normalizedModel)
	if err != nil {
		return false, err
	}
	channelScheduler.disabledMu.Lock()
	delete(channelScheduler.disabled, channelModelKey{ChannelId: channelId, Model: normalizedModel})
	channelScheduler.disabledMu.Unlock()
	ResetChannelSchedulingState(channelId, normalizedModel)
	appendSchedulingEvent(ChannelSchedulingEvent{Ts: time.Now().Unix(), Type: "restored", ChannelId: channelId, Model: normalizedModel, Message: "model capability restored manually"})
	return changed, nil
}

func MarkScheduledChannelModelManualDisabled(channelId int, modelName string) error {
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	changed, err := model.MarkChannelModelManualDisabled(channelId, normalizedModel)
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("model capability is not currently auto-disabled")
	}
	channelScheduler.disabledMu.Lock()
	if state, ok := channelScheduler.disabled[channelModelKey{ChannelId: channelId, Model: normalizedModel}]; ok {
		state.ManualDisabled = true
		channelScheduler.disabled[channelModelKey{ChannelId: channelId, Model: normalizedModel}] = state
	}
	channelScheduler.disabledMu.Unlock()
	appendSchedulingEvent(ChannelSchedulingEvent{Ts: time.Now().Unix(), Type: "manual_disabled", ChannelId: channelId, Model: normalizedModel, Message: "model capability held disabled manually"})
	return nil
}

func MarkScheduledChannelManualDisabled(channelId int) (bool, error) {
	changed, err := model.MarkChannelManualDisabledForScheduling(channelId)
	if err != nil || !changed {
		return changed, err
	}
	channelScheduler.disabledMu.Lock()
	delete(channelScheduler.disabledChannels, channelId)
	channelScheduler.disabledMu.Unlock()
	ResetChannelSchedulingState(channelId, "")
	return true, nil
}

func RestoreScheduledChannel(channelId int) (bool, error) {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return false, err
	}
	if channel.Status != common.ChannelStatusAutoDisabled || !model.IsChannelSchedulingFault(channel) {
		return false, nil
	}
	changed := model.UpdateChannelStatus(channelId, "", common.ChannelStatusEnabled, "manual scheduling recovery")
	if changed {
		channelScheduler.disabledMu.Lock()
		delete(channelScheduler.disabledChannels, channelId)
		channelScheduler.disabledMu.Unlock()
		ResetChannelSchedulingState(channelId, "")
		appendSchedulingEvent(ChannelSchedulingEvent{Ts: time.Now().Unix(), Type: "restored_channel", ChannelId: channelId, ChannelName: channel.Name, Message: "channel restored manually"})
	}
	return changed, nil
}

func ResetChannelSchedulingState(channelId int, modelName string) {
	normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
	if modelName == "" {
		channelScheduler.disabledMu.Lock()
		delete(channelScheduler.disabledChannels, channelId)
		channelScheduler.disabledMu.Unlock()
		clearIntelligentSchedulingFailure(channelId)
	}
	channelScheduler.poolsMu.RLock()
	defer channelScheduler.poolsMu.RUnlock()
	for key, pool := range channelScheduler.pools {
		if normalizedModel != "" && key.Model != normalizedModel {
			continue
		}
		pool.mu.Lock()
		delete(pool.channels, channelId)
		pool.mu.Unlock()
	}
}

func GetChannelSchedulingOverview(group string, modelName string, priority *int64, since int64) (ChannelSchedulingOverview, error) {
	overview := ChannelSchedulingOverview{
		Strategy:      operation_setting.GetChannelSchedulingSetting(),
		GeneratedAt:   time.Now().Unix(),
		Channels:      make([]ChannelSchedulingChannelSnapshot, 0),
		Faults:        make([]model.ChannelModelState, 0),
		ChannelFaults: make([]ChannelSchedulingChannelFault, 0),
		Events:        make([]ChannelSchedulingEvent, 0),
	}
	retentionStart := overview.GeneratedAt - int64(overview.Strategy.RealtimeRetentionMin*60)
	if since <= 0 || since < retentionStart {
		since = retentionStart
	}
	normalizedModel := ""
	if modelName != "" {
		normalizedModel = ratio_setting.FormatMatchingModelName(modelName)
	}

	channelScheduler.poolsMu.RLock()
	for key, pool := range channelScheduler.pools {
		if group != "" && key.Group != group {
			continue
		}
		if normalizedModel != "" && key.Model != normalizedModel {
			continue
		}
		if priority != nil && key.Priority != *priority {
			continue
		}
		pool.mu.Lock()
		if len(pool.channels) == 0 {
			pool.mu.Unlock()
			continue
		}
		refreshSchedulingPoolWeights(pool, overview.Strategy, overview.GeneratedAt)
		overview.ActivePools++
		poolRequests := int64(0)
		for _, state := range pool.channels {
			pruneSchedulingBuckets(state, overview.GeneratedAt, overview.Strategy.RealtimeRetentionMin)
			poolRequests += schedulingRequestsSince(state, since)
		}
		for _, state := range pool.channels {
			snapshot := snapshotSchedulingState(key, state, poolRequests, since, overview.Strategy.MinimumFactor)
			overview.Requests += snapshot.RequestCount
			overview.Errors += snapshot.ErrorCount
			overview.Inflight += snapshot.Inflight
			if snapshot.State == "degraded" {
				overview.DegradedChannels++
			} else {
				overview.NormalChannels++
			}
			overview.Channels = append(overview.Channels, snapshot)
		}
		pool.mu.Unlock()
	}
	channelScheduler.poolsMu.RUnlock()

	var ttftSum int64
	var ttftCount int64
	recentRequests := int64(0)
	recentCutoff := overview.GeneratedAt - 10
	for _, channel := range overview.Channels {
		for _, point := range channel.Series {
			ttftSum += point.TtftSumMs
			ttftCount += point.TtftCount
			if point.Ts > recentCutoff {
				recentRequests += point.RequestCount
			}
		}
	}
	if ttftCount > 0 {
		overview.AvgTtftMs = ttftSum / ttftCount
	}
	overview.Rps = math.Round(float64(recentRequests)/10*100) / 100
	faults, err := model.ListChannelModelStates(true)
	if err != nil {
		return ChannelSchedulingOverview{}, err
	}
	for _, fault := range faults {
		if normalizedModel != "" && fault.Model != normalizedModel {
			continue
		}
		overview.Faults = append(overview.Faults, fault)
	}
	overview.DisabledModels = len(overview.Faults)
	disabledChannels, err := model.ListAutoDisabledChannels()
	if err != nil {
		return ChannelSchedulingOverview{}, err
	}
	for _, channel := range disabledChannels {
		if !model.IsChannelSchedulingFault(&channel) {
			continue
		}
		info := channel.GetOtherInfo()
		disabledAt := int64(0)
		switch value := info["status_time"].(type) {
		case float64:
			disabledAt = int64(value)
		case int64:
			disabledAt = value
		case int:
			disabledAt = int64(value)
		}
		overview.ChannelFaults = append(overview.ChannelFaults, ChannelSchedulingChannelFault{
			ChannelId: channel.Id, ChannelName: channel.Name, Reason: model.GetChannelSchedulingFaultReason(&channel), DisabledAt: disabledAt,
		})
	}
	overview.DisabledChannels = len(overview.ChannelFaults)
	overview.Events = schedulingEventsSince(since, group, normalizedModel)
	sort.Slice(overview.Channels, func(i, j int) bool {
		left, right := overview.Channels[i], overview.Channels[j]
		if left.Group != right.Group {
			return left.Group < right.Group
		}
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		if left.Priority != right.Priority {
			return left.Priority > right.Priority
		}
		return left.ChannelId < right.ChannelId
	})
	return overview, nil
}

func recordIntelligentSchedulingFailure(info *relaycommon.RelayInfo, channel *model.Channel, relayErr *types.NewAPIError, now time.Time) bool {
	if info == nil || channel == nil || !isIntelligentSchedulingUpstreamError(relayErr) {
		return false
	}
	setting := operation_setting.GetChannelSchedulingSetting()
	failureThreshold := setting.FailureThreshold
	failureWindow := time.Duration(setting.FailureWindowSeconds) * time.Second

	shard := getChannelFailureShard(channel.Id)
	shard.mu.Lock()
	if isChannelSchedulingDisabled(channel.Id, info.SchedulerModel) {
		shard.mu.Unlock()
		return false
	}
	if shard.streaks == nil {
		shard.streaks = make(map[int]channelFailureStreak)
	}
	streak := shard.streaks[channel.Id]
	if !streak.LastOutcomeAt.IsZero() && now.Before(streak.LastOutcomeAt) {
		shard.mu.Unlock()
		return false
	}
	elapsed := now.Sub(streak.LastFailureAt)
	if streak.LastFailureAt.IsZero() || elapsed < 0 || elapsed > failureWindow {
		streak.Count = 0
	}
	streak.Count++
	streak.LastFailureAt = now
	streak.LastOutcomeAt = now
	shard.streaks[channel.Id] = streak
	if streak.Count < failureThreshold {
		shard.mu.Unlock()
		return false
	}

	lastError := relayErr.MaskSensitiveErrorWithStatusCode()
	reason := fmt.Sprintf("%d consecutive upstream failures with no gap longer than %s; last error: %s", streak.Count, failureWindow, lastError)
	shard.mu.Unlock()
	channelSchedulerDisableMu.Lock()
	shard.mu.Lock()
	current := shard.streaks[channel.Id]
	if current.Count < failureThreshold || !current.LastOutcomeAt.Equal(now) || isChannelSchedulingDisabled(channel.Id, info.SchedulerModel) {
		shard.mu.Unlock()
		channelSchedulerDisableMu.Unlock()
		return false
	}
	hasAlternative, err := hasAlternativeIntelligentSchedulingChannel(info, channel.Id)
	if err != nil {
		shard.mu.Unlock()
		channelSchedulerDisableMu.Unlock()
		common.SysError("failed to verify intelligent scheduling fallback channel: " + err.Error())
		return false
	}
	if !hasAlternative {
		shard.mu.Unlock()
		channelSchedulerDisableMu.Unlock()
		return false
	}
	if err := model.DisableChannelForScheduling(channel.Id, reason); err != nil {
		shard.mu.Unlock()
		channelSchedulerDisableMu.Unlock()
		common.SysError("failed to disable scheduling channel: " + err.Error())
		return false
	}
	channelScheduler.disabledMu.Lock()
	channelScheduler.disabledChannels[channel.Id] = struct{}{}
	channelScheduler.disabledMu.Unlock()
	delete(shard.streaks, channel.Id)
	shard.mu.Unlock()
	channelSchedulerDisableMu.Unlock()

	event := ChannelSchedulingEvent{
		Ts:          now.Unix(),
		Type:        "disabled_channel",
		ChannelId:   channel.Id,
		ChannelName: channel.Name,
		Group:       info.SchedulerGroup,
		Model:       info.SchedulerModel,
		Priority:    info.SchedulerPriority,
		Message:     reason,
	}
	appendSchedulingEvent(event)
	return true
}

func hasAlternativeIntelligentSchedulingChannel(info *relaycommon.RelayInfo, channelId int) (bool, error) {
	if info == nil || info.SchedulerGroup == "" || info.SchedulerModel == "" {
		return false, nil
	}
	candidates, err := model.GetSatisfiedChannels(info.SchedulerGroup, info.SchedulerModel, info.SchedulerRequestPath)
	if err != nil {
		return false, err
	}
	for _, candidate := range candidates {
		if candidate.Id != channelId && !isChannelSchedulingDisabled(candidate.Id, info.SchedulerModel) {
			return true, nil
		}
	}
	return false, nil
}

func isIntelligentSchedulingUpstreamError(relayErr *types.NewAPIError) bool {
	if relayErr == nil {
		return false
	}
	message := strings.ToLower(relayErr.Error())
	if errors.Is(relayErr, context.Canceled) || strings.Contains(message, context.Canceled.Error()) {
		return false
	}
	if (errors.Is(relayErr, context.DeadlineExceeded) || strings.Contains(message, context.DeadlineExceeded.Error())) && strings.Contains(message, "client") {
		return false
	}
	return true
}

func clearIntelligentSchedulingFailure(channelId int) {
	shard := getChannelFailureShard(channelId)
	shard.mu.Lock()
	delete(shard.streaks, channelId)
	shard.mu.Unlock()
}

func recordIntelligentSchedulingSuccess(channelId int, now time.Time) {
	shard := getChannelFailureShard(channelId)
	shard.mu.Lock()
	if isChannelSchedulingDisabled(channelId, "") {
		shard.mu.Unlock()
		return
	}
	if shard.streaks == nil {
		shard.streaks = make(map[int]channelFailureStreak)
	}
	streak := shard.streaks[channelId]
	if !streak.LastOutcomeAt.IsZero() && now.Before(streak.LastOutcomeAt) {
		shard.mu.Unlock()
		return
	}
	streak.Count = 0
	streak.LastFailureAt = time.Time{}
	streak.LastOutcomeAt = now
	shard.streaks[channelId] = streak
	shard.mu.Unlock()
}

func cleanupChannelFailureStreaks(now time.Time) {
	failureWindow := time.Duration(operation_setting.GetChannelSchedulingSetting().FailureWindowSeconds) * time.Second
	for index := range channelScheduler.failureShards {
		shard := &channelScheduler.failureShards[index]
		shard.mu.Lock()
		for channelId, streak := range shard.streaks {
			elapsed := now.Sub(streak.LastOutcomeAt)
			if streak.LastOutcomeAt.IsZero() || elapsed < 0 || elapsed > failureWindow {
				delete(shard.streaks, channelId)
			}
		}
		shard.mu.Unlock()
	}
}

func getChannelFailureShard(channelId int) *channelFailureShard {
	return &channelScheduler.failureShards[channelId%channelFailureShardCount]
}

func isChannelModelSchedulingDisabled(channelId int, modelName string) bool {
	channelScheduler.disabledMu.RLock()
	_, disabled := channelScheduler.disabled[channelModelKey{ChannelId: channelId, Model: modelName}]
	channelScheduler.disabledMu.RUnlock()
	return disabled
}

func hasChannelModelSchedulingFault(modelName string) bool {
	channelScheduler.disabledMu.RLock()
	defer channelScheduler.disabledMu.RUnlock()
	for key := range channelScheduler.disabled {
		if key.Model == modelName {
			return true
		}
	}
	return false
}

func isChannelSchedulingDisabled(channelId int, modelName string) bool {
	channelScheduler.disabledMu.RLock()
	_, channelDisabled := channelScheduler.disabledChannels[channelId]
	_, modelDisabled := channelScheduler.disabled[channelModelKey{ChannelId: channelId, Model: modelName}]
	channelScheduler.disabledMu.RUnlock()
	return channelDisabled || modelDisabled
}

func calculatePerformanceFactor(state *channelSchedulingState, median float64, setting operation_setting.ChannelSchedulingSetting) float64 {
	if state.EstimatedTtftMs <= 0 || median <= 0 {
		return 1
	}
	if state.LastTtftMs >= setting.SevereTtftMs {
		return setting.MinimumFactor
	}
	ratio := median / state.EstimatedTtftMs
	rawFactor := math.Pow(ratio, setting.PerformanceExponent)
	confidence := math.Min(1, float64(state.Samples)/float64(setting.WarmupSamples))
	factor := 1 + confidence*(rawFactor-1)
	return math.Max(setting.MinimumFactor, math.Min(setting.MaximumFactor, factor))
}

func refreshSchedulingPoolWeights(pool *channelSchedulingPool, setting operation_setting.ChannelSchedulingSetting, now int64) {
	observed := make([]float64, 0, len(pool.channels))
	for _, state := range pool.channels {
		pruneSchedulingTtftSamples(state, now, setting)
		if state.EstimatedTtftMs > 0 {
			observed = append(observed, state.EstimatedTtftMs)
		}
	}
	median := medianFloat(observed)
	if median <= 0 {
		median = 1000
	}
	for _, state := range pool.channels {
		state.PerformanceFactor = calculatePerformanceFactor(state, median, setting)
		loadFactor := 1 / (1 + setting.InflightPenalty*float64(state.Inflight))
		state.EffectiveWeight = math.Max(0.001, float64(state.BaseWeight)*state.PerformanceFactor*loadFactor)
	}
}

func addSchedulingTtftSample(state *channelSchedulingState, now int64, ttftMs int64, setting operation_setting.ChannelSchedulingSetting) {
	state.TtftSamples = append(state.TtftSamples, channelSchedulingTtftSample{Ts: now, TtftMs: ttftMs})
	pruneSchedulingTtftSamples(state, now, setting)
}

func pruneSchedulingTtftSamples(state *channelSchedulingState, now int64, setting operation_setting.ChannelSchedulingSetting) {
	cutoff := now - int64(setting.SampleMaxAgeMinutes*60)
	firstValid := 0
	for firstValid < len(state.TtftSamples) && state.TtftSamples[firstValid].Ts < cutoff {
		firstValid++
	}
	if firstValid > 0 {
		state.TtftSamples = append([]channelSchedulingTtftSample(nil), state.TtftSamples[firstValid:]...)
	}
	if len(state.TtftSamples) > setting.SampleWindowSize {
		state.TtftSamples = append([]channelSchedulingTtftSample(nil), state.TtftSamples[len(state.TtftSamples)-setting.SampleWindowSize:]...)
	}
	state.Samples = int64(len(state.TtftSamples))
	if len(state.TtftSamples) == 0 {
		state.EstimatedTtftMs = 0
		state.LastTtftMs = 0
		return
	}
	values := make([]int64, 0, len(state.TtftSamples))
	for _, sample := range state.TtftSamples {
		values = append(values, sample.TtftMs)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	state.EstimatedTtftMs = float64(values[(len(values)-1)/2])
	state.LastTtftMs = state.TtftSamples[len(state.TtftSamples)-1].TtftMs
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cloned := append([]float64(nil), values...)
	sort.Float64s(cloned)
	middle := len(cloned) / 2
	if len(cloned)%2 == 1 {
		return cloned[middle]
	}
	return (cloned[middle-1] + cloned[middle]) / 2
}

func schedulingBucket(state *channelSchedulingState, ts int64) *ChannelSchedulingBucket {
	bucketTs := ts - ts%2
	bucket := state.Buckets[bucketTs]
	if bucket == nil {
		bucket = &ChannelSchedulingBucket{Ts: bucketTs}
		state.Buckets[bucketTs] = bucket
	}
	return bucket
}

func pruneSchedulingBuckets(state *channelSchedulingState, now int64, retentionMinutes int) {
	cutoff := now - int64(retentionMinutes*60)
	for ts := range state.Buckets {
		if ts < cutoff {
			delete(state.Buckets, ts)
		}
	}
}

func schedulingRequestsSince(state *channelSchedulingState, since int64) int64 {
	requests := int64(0)
	for ts, bucket := range state.Buckets {
		if ts >= since {
			requests += bucket.RequestCount
		}
	}
	return requests
}

func cleanupChannelSchedulingPools(now int64, retentionMinutes int) {
	cutoff := now - int64(retentionMinutes*60)
	channelScheduler.poolsMu.Lock()
	defer channelScheduler.poolsMu.Unlock()
	for key, pool := range channelScheduler.pools {
		pool.mu.Lock()
		latestActivity := int64(0)
		hasInflight := false
		for _, state := range pool.channels {
			pruneSchedulingBuckets(state, now, retentionMinutes)
			latestActivity = max(latestActivity, state.LastSelectedAt, state.LastCompletedAt)
			hasInflight = hasInflight || state.Inflight > 0
		}
		refreshSchedulingPoolWeights(pool, operation_setting.GetChannelSchedulingSetting(), now)
		remove := len(pool.channels) == 0 || (!hasInflight && latestActivity > 0 && latestActivity < cutoff)
		pool.mu.Unlock()
		if remove {
			delete(channelScheduler.pools, key)
		}
	}
}

func snapshotSchedulingState(key schedulingPoolKey, state *channelSchedulingState, poolRequests int64, since int64, minimumFactor float64) ChannelSchedulingChannelSnapshot {
	status := "normal"
	if state.PerformanceFactor <= minimumFactor && state.Samples > 0 {
		status = "degraded"
	}
	series := make([]ChannelSchedulingBucket, 0, len(state.Buckets))
	channelRequests := int64(0)
	for ts, bucket := range state.Buckets {
		if since > 0 && ts < since {
			continue
		}
		series = append(series, *bucket)
		channelRequests += bucket.RequestCount
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Ts < series[j].Ts })
	actualShare := 0.0
	if poolRequests > 0 {
		actualShare = float64(channelRequests) / float64(poolRequests)
	}
	return ChannelSchedulingChannelSnapshot{
		ChannelId: state.ChannelId, ChannelName: state.ChannelName, Group: key.Group, Model: key.Model, Priority: key.Priority,
		BaseWeight: state.BaseWeight, EffectiveWeight: math.Round(state.EffectiveWeight*100) / 100,
		PerformanceFactor: math.Round(state.PerformanceFactor*1000) / 1000, EstimatedTtftMs: int64(math.Round(state.EstimatedTtftMs)),
		LastTtftMs: state.LastTtftMs, Inflight: state.Inflight, RequestCount: state.RequestCount, SuccessCount: state.SuccessCount,
		ErrorCount: state.ErrorCount, AffinityHits: state.AffinityHits, Samples: state.Samples, ActualShare: math.Round(actualShare*10000) / 10000,
		State: status, LastError: state.LastError, LastSelectedAt: state.LastSelectedAt, LastCompletedAt: state.LastCompletedAt, Series: series,
	}
}

func appendSchedulingEvent(event ChannelSchedulingEvent) {
	channelScheduler.eventsMu.Lock()
	defer channelScheduler.eventsMu.Unlock()
	channelScheduler.events = append(channelScheduler.events, event)
	if len(channelScheduler.events) > 500 {
		channelScheduler.events = append([]ChannelSchedulingEvent(nil), channelScheduler.events[len(channelScheduler.events)-500:]...)
	}
}

func schedulingEventsSince(since int64, group string, modelName string) []ChannelSchedulingEvent {
	channelScheduler.eventsMu.Lock()
	defer channelScheduler.eventsMu.Unlock()
	events := make([]ChannelSchedulingEvent, 0, len(channelScheduler.events))
	for _, event := range channelScheduler.events {
		if since > 0 && event.Ts < since {
			continue
		}
		if group != "" && event.Group != "" && event.Group != group {
			continue
		}
		if modelName != "" && event.Model != "" && event.Model != modelName {
			continue
		}
		events = append(events, event)
	}
	return events
}

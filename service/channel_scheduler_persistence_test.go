package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSchedulerPersistenceTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	testOptionMap := make(map[string]string, len(originalOptionMap))
	for key, value := range originalOptionMap {
		testOptionMap[key] = value
	}
	common.OptionMap = testOptionMap
	common.OptionMapRWMutex.Unlock()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelModelState{}, &model.Option{}))
	model.DB = db
	common.MemoryCacheEnabled = false

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func createSchedulerPersistenceChannel(t *testing.T, db *gorm.DB, id int, status int, reason string) {
	t.Helper()

	priority := int64(10)
	weight := uint(100)
	channel := &model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("scheduler-key-%d", id),
		Status:   status,
		Name:     fmt.Sprintf("scheduler-channel-%d", id),
		Weight:   &weight,
		Models:   "gpt-test",
		Group:    "default",
		Priority: &priority,
	}
	if reason != "" {
		channel.SetOtherInfo(map[string]interface{}{"status_reason": reason})
	}
	require.NoError(t, db.Create(channel).Error)
}

func createSchedulerCandidate(t *testing.T, db *gorm.DB, id int, group string, priority int64) {
	t.Helper()

	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("candidate-key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("candidate-%d", id),
		Weight:   &weight,
		Models:   "gpt-test",
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     "gpt-test",
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func finishSchedulerFailure(channel *model.Channel, group string, relayErr *types.NewAPIError) {
	info := &relaycommon.RelayInfo{UsingGroup: group, OriginModelName: "gpt-test"}
	BeginScheduledChannelAttempt(nil, info, channel)
	info.UpstreamStartTime = time.Now()
	FinishScheduledChannelAttempt(info, channel, false, relayErr)
}

func finishSchedulerSuccess(channel *model.Channel, group string) {
	info := &relaycommon.RelayInfo{UsingGroup: group, OriginModelName: "gpt-test"}
	BeginScheduledChannelAttempt(nil, info, channel)
	info.UpstreamStartTime = time.Now()
	info.UpstreamResponseTime = info.UpstreamStartTime.Add(time.Millisecond)
	FinishScheduledChannelAttempt(info, channel, true, nil)
}

func configureSchedulerGroupStrategyForTest(t *testing.T, group string, strategy string) {
	t.Helper()

	original := operation_setting.GetChannelSchedulingSetting()
	originalGroups, err := common.Marshal(original.GroupStrategies)
	require.NoError(t, err)
	strategies := original.GroupStrategies
	strategies[group] = strategy
	strategiesJSON, err := common.Marshal(strategies)
	require.NoError(t, err)
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
		"group_strategies": string(strategiesJSON),
	}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"group_strategies": string(originalGroups),
		}))
	})
}

func TestChannelSchedulingOverviewReturnsArraysWhenNoDataMatches(t *testing.T) {
	setupChannelSchedulerPersistenceTest(t)

	overview, err := GetChannelSchedulingOverview("missing-group", "missing-model", nil, 0)
	require.NoError(t, err)
	assert.NotNil(t, overview.Channels)
	assert.NotNil(t, overview.Faults)
	assert.NotNil(t, overview.ChannelFaults)
	assert.NotNil(t, overview.Events)

	payload, err := common.Marshal(overview)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"channels":[]`)
	assert.Contains(t, string(payload), `"faults":[]`)
	assert.Contains(t, string(payload), `"channel_faults":[]`)
	assert.Contains(t, string(payload), `"events":[]`)
}

func TestIntelligentSchedulingExhaustsCurrentPriorityBeforeFailover(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-priority-test"
	createSchedulerCandidate(t, db, 91101, group, 20)
	createSchedulerCandidate(t, db, 91102, group, 20)
	createSchedulerCandidate(t, db, 91103, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	original := operation_setting.GetChannelSchedulingSetting()
	originalGroups, err := common.Marshal(original.GroupStrategies)
	require.NoError(t, err)
	strategies := original.GroupStrategies
	strategies[group] = operation_setting.ChannelSchedulingStrategyIntelligent
	strategiesJSON, err := common.Marshal(strategies)
	require.NoError(t, err)
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
		"group_strategies": string(strategiesJSON),
	}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"group_strategies": string(originalGroups),
		}))
		ResetChannelSchedulingState(91101, "")
		ResetChannelSchedulingState(91102, "")
		ResetChannelSchedulingState(91103, "")
	})

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	param := &RetryParam{Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions"}
	selectedPriorities := make([]int64, 0, 3)
	selectedChannels := make(map[int]struct{}, 3)
	for range 3 {
		channel, selectedGroup, selectErr := CacheGetRandomSatisfiedChannel(param)
		require.NoError(t, selectErr)
		require.NotNil(t, channel)
		assert.Equal(t, group, selectedGroup)
		selectedPriorities = append(selectedPriorities, channel.GetPriority())
		selectedChannels[channel.Id] = struct{}{}
		param.MarkAttempted(channel.Id)
	}

	assert.Equal(t, []int64{20, 20, 10}, selectedPriorities)
	assert.Len(t, selectedChannels, 3)
}

func TestIntelligentSessionBindingReusesHealthyChannelAndMigratesAfterRetryExclusion(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-soft-affinity-boundary-test"
	createSchedulerCandidate(t, db, 91104, group, 20)
	createSchedulerCandidate(t, db, 91105, group, 20)
	createSchedulerCandidate(t, db, 91106, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)

	original := operation_setting.GetChannelSchedulingSetting()
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{"soft_affinity_enabled": "true"}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"soft_affinity_enabled": fmt.Sprintf("%t", original.SoftAffinityEnabled),
		}))
	})

	affinitySetting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, affinitySetting)
	var codexRule *operation_setting.ChannelAffinityRule
	for i := range affinitySetting.Rules {
		if strings.EqualFold(strings.TrimSpace(affinitySetting.Rules[i].Name), "codex cli trace") {
			codexRule = &affinitySetting.Rules[i]
			break
		}
	}
	require.NotNil(t, codexRule)

	affinityValue := fmt.Sprintf("scheduler-boundary-%d", time.Now().UnixNano())
	cacheKey := buildChannelAffinityCacheKeySuffix(*codexRule, "gpt-test", group, affinityValue)
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKey, 91106, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKey})
	})

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Session-Id", affinityValue)
	param := &RetryParam{Ctx: ctx, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/responses"}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, 91106, first.Id)
	assert.Equal(t, int64(10), first.GetPriority())
	assert.True(t, ctx.GetBool(ginKeyChannelAffinityUsed))

	require.NoError(t, cache.SetWithTTL(cacheKey, first.Id, time.Minute))
	param.MarkAttempted(first.Id)
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, int64(20), second.GetPriority())
	assert.NotEqual(t, first.Id, second.Id)
	assert.False(t, ctx.GetBool(ginKeyChannelAffinityUsed))
}

func TestLegacySchedulingSkipsRateLimitedChannelsBeforeFallingBackPriority(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-legacy-rate-limit-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyLegacy)
	createSchedulerCandidate(t, db, 91111, group, 20)
	createSchedulerCandidate(t, db, 91112, group, 20)
	createSchedulerCandidate(t, db, 91113, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	param := &RetryParam{Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions"}
	param.MarkRateLimited(91111, 15_000)

	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, 91112, selected.Id)

	param.MarkRateLimited(91112, 10_000)
	selected, selectedGroup, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, 91113, selected.Id)
	assert.Equal(t, int64(10_000), param.RateLimitRetryAfterMillis())
	param.MarkRateLimited(91113, 30_000)
	selected, selectedGroup, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	assert.Nil(t, selected)
	assert.Equal(t, group, selectedGroup)
	assert.True(t, param.HasRateLimitedChannels())

	for _, channelId := range []int{91111, 91112, 91113} {
		channel, getErr := model.GetChannelById(channelId, true)
		require.NoError(t, getErr)
		assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
	}
}

func TestIntelligentSchedulingKeepsRateLimitedChannelStateAndFallsBackPriority(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-intelligent-rate-limit-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91121, group, 20)
	createSchedulerCandidate(t, db, 91122, group, 20)
	createSchedulerCandidate(t, db, 91123, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	param := &RetryParam{Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions"}
	first, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 91121, first.Id)

	key := schedulingPoolKey{Group: group, Model: "gpt-test", Priority: 20}
	pool := getSchedulingPool(key)
	pool.mu.Lock()
	pool.channels[91121].RequestCount = 7
	pool.channels[91121].LastError = "preserve-rate-limited-state"
	pool.mu.Unlock()

	param.MarkRateLimited(91121, 20_000)
	selected, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 91122, selected.Id)

	pool.mu.Lock()
	preserved := *pool.channels[91121]
	pool.mu.Unlock()
	assert.Equal(t, int64(7), preserved.RequestCount)
	assert.Equal(t, "preserve-rate-limited-state", preserved.LastError)

	param.MarkRateLimited(91122, 10_000)
	selected, _, err = CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, 91123, selected.Id)

	limitedChannel, err := model.GetChannelById(91121, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, limitedChannel.Status)
}

func TestRedisRateLimitFailureCanFallBackToUnlimitedChannel(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-rate-limit-redis-fallback-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyLegacy)
	createSchedulerCandidate(t, db, 91131, group, 20)
	createSchedulerCandidate(t, db, 91132, group, 20)

	configured, err := model.GetChannelById(91131, true)
	require.NoError(t, err)
	configured.SetOtherSettings(dto.ChannelOtherSettings{RequestRateLimit: &dto.ChannelRequestRateLimit{
		MaxRequests: 30, WindowSeconds: 60,
	}})
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", configured.Id).Update("settings", configured.OtherSettings).Error)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = nil
	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
	})

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	param := &RetryParam{Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions"}
	first, err := model.CacheGetChannel(91131)
	require.NoError(t, err)

	reservation, _, reserveErr := ReserveChannelRequest(context, first)
	assert.Nil(t, reservation)
	require.Error(t, reserveErr)
	param.MarkRateLimitUnavailable(first.Id, reserveErr)

	fallback, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, fallback)
	assert.Equal(t, 91132, fallback.Id)
	reservation, decision, reserveErr := ReserveChannelRequest(context, fallback)
	require.NoError(t, reserveErr)
	assert.Nil(t, reservation)
	assert.True(t, decision.Allowed)
}

func TestLegacySchedulingAlsoSkipsPersistentlyDisabledModelCapability(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-legacy-fault-test"
	createSchedulerCandidate(t, db, 91201, group, 10)
	createSchedulerCandidate(t, db, 91202, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	require.NoError(t, model.DisableChannelModel(91201, "gpt-test", "unsupported model", "model_not_found", 404))
	reloadChannelSchedulerFaults()
	t.Cleanup(func() {
		_, restoreErr := model.RestoreChannelModel(91201, "gpt-test")
		require.NoError(t, restoreErr)
		reloadChannelSchedulerFaults()
	})

	original := operation_setting.GetChannelSchedulingSetting()
	originalGroups, err := common.Marshal(original.GroupStrategies)
	require.NoError(t, err)
	strategies := original.GroupStrategies
	strategies[group] = operation_setting.ChannelSchedulingStrategyLegacy
	strategiesJSON, err := common.Marshal(strategies)
	require.NoError(t, err)
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
		"group_strategies": string(strategiesJSON),
	}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"group_strategies": string(originalGroups),
		}))
	})

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	for range 10 {
		channel, selectedGroup, selectErr := CacheGetRandomSatisfiedChannel(&RetryParam{
			Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions",
		})
		require.NoError(t, selectErr)
		require.NotNil(t, channel)
		assert.Equal(t, group, selectedGroup)
		assert.Equal(t, 91202, channel.Id)
	}
}

func TestDeletingChannelRemovesPersistentModelFaults(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91301, common.ChannelStatusEnabled, "")
	require.NoError(t, model.DisableChannelModel(91301, "gpt-test", "unsupported model", "model_not_found", 404))

	rows, err := model.BatchDeleteChannels([]int{91301})
	require.NoError(t, err)
	assert.Equal(t, int64(1), rows)
	states, err := model.ListChannelModelStates(false)
	require.NoError(t, err)
	assert.Empty(t, states)
}

func TestChannelSchedulingOptionsPersistAndApplyAtRuntime(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	original := operation_setting.GetChannelSchedulingSetting()
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"default_strategy":      original.DefaultStrategy,
			"severe_ttft_ms":        fmt.Sprintf("%d", original.SevereTtftMs),
			"soft_affinity_enabled": fmt.Sprintf("%t", original.SoftAffinityEnabled),
		}))
		common.OptionMapRWMutex.Lock()
		delete(common.OptionMap, "channel_scheduling_setting.default_strategy")
		delete(common.OptionMap, "channel_scheduling_setting.severe_ttft_ms")
		delete(common.OptionMap, "channel_scheduling_setting.soft_affinity_enabled")
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		"channel_scheduling_setting.default_strategy":      operation_setting.ChannelSchedulingStrategyIntelligent,
		"channel_scheduling_setting.severe_ttft_ms":        "45000",
		"channel_scheduling_setting.soft_affinity_enabled": "true",
	}))

	setting := operation_setting.GetChannelSchedulingSetting()
	assert.Equal(t, operation_setting.ChannelSchedulingStrategyIntelligent, setting.DefaultStrategy)
	assert.Equal(t, int64(45_000), setting.SevereTtftMs)
	assert.True(t, setting.SoftAffinityEnabled)
	var options []model.Option
	require.NoError(t, db.Where("key LIKE ?", "channel_scheduling_setting.%").Order("key ASC").Find(&options).Error)
	require.Len(t, options, 3)
	assert.Equal(t, "channel_scheduling_setting.default_strategy", options[0].Key)
	assert.Equal(t, operation_setting.ChannelSchedulingStrategyIntelligent, options[0].Value)
	assert.Equal(t, "channel_scheduling_setting.severe_ttft_ms", options[1].Key)
	assert.Equal(t, "45000", options[1].Value)
	assert.Equal(t, "channel_scheduling_setting.soft_affinity_enabled", options[2].Key)
	assert.Equal(t, "true", options[2].Value)
}

func TestRateLimitFaultDisablesWholeChannelAndFailsOver(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-channel-fault-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91401, group, 10)
	createSchedulerCandidate(t, db, 91402, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91401, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("rate limited"), types.ErrorCodeBadResponseStatusCode, 429)
	for range operation_setting.ChannelSchedulingDefaultFailureCount - 1 {
		finishSchedulerFailure(channel, group, relayErr)
	}
	stillEnabled, err := model.GetChannelById(91401, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)

	finishSchedulerFailure(channel, group, relayErr)
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(91401); restoreErr == nil {
			assert.True(t, restored)
		}
	})

	disabled, err := model.GetChannelById(91401, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	assert.True(t, model.IsChannelSchedulingFault(disabled))
	assert.False(t, IsChannelUsableForScheduling(group, "gpt-test", 91401))

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions",
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, 91402, selected.Id)
}

func TestLastIntelligentSchedulingChannelIsNeverAutoDisabled(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-last-channel-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91411, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91411, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, 524)
	for range operation_setting.ChannelSchedulingDefaultFailureCount + 2 {
		finishSchedulerFailure(channel, group, relayErr)
	}

	stillEnabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)
	assert.False(t, model.IsChannelSchedulingFault(stillEnabled))
	streak, exists := getChannelFailureStreakForTest(channel.Id)
	assert.True(t, exists)
	assert.Equal(t, operation_setting.ChannelSchedulingDefaultFailureCount+2, streak.Count)
}

func TestLastChannelProtectionCountsLowerPriorityFallbackAsAvailable(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-last-channel-priority-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91421, group, 20)
	createSchedulerCandidate(t, db, 91422, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91421, true)
	require.NoError(t, err)
	for range operation_setting.ChannelSchedulingDefaultFailureCount {
		finishSchedulerFailure(channel, group, types.NewOpenAIError(
			fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden,
		))
	}

	disabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(channel.Id); restoreErr == nil {
			assert.True(t, restored)
		}
	})
}

func TestUnrelatedEnabledChannelDoesNotBypassLastPoolChannelProtection(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-last-pool-channel-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91423, group, 10)
	createSchedulerCandidate(t, db, 91424, "unrelated-group", 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91423, true)
	require.NoError(t, err)
	for range operation_setting.ChannelSchedulingDefaultFailureCount + 1 {
		finishSchedulerFailure(channel, group, types.NewOpenAIError(
			fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway,
		))
	}

	stillEnabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)
}

func TestConcurrentThresholdFailuresStillLeaveOneChannelEnabled(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-concurrent-last-channel-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91431, group, 10)
	createSchedulerCandidate(t, db, 91432, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channels := make([]*model.Channel, 0, 2)
	for _, channelId := range []int{91431, 91432} {
		channel, err := model.GetChannelById(channelId, true)
		require.NoError(t, err)
		channels = append(channels, channel)
		for range operation_setting.ChannelSchedulingDefaultFailureCount - 1 {
			finishSchedulerFailure(channel, group, types.NewOpenAIError(
				fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway,
			))
		}
	}

	var waitGroup sync.WaitGroup
	for _, channel := range channels {
		waitGroup.Add(1)
		go func(channel *model.Channel) {
			defer waitGroup.Done()
			finishSchedulerFailure(channel, group, types.NewOpenAIError(
				fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway,
			))
		}(channel)
	}
	waitGroup.Wait()

	enabledCount := 0
	for _, channel := range channels {
		current, err := model.GetChannelById(channel.Id, true)
		require.NoError(t, err)
		if current.Status == common.ChannelStatusEnabled {
			enabledCount++
			continue
		}
		require.Equal(t, common.ChannelStatusAutoDisabled, current.Status)
		currentId := current.Id
		t.Cleanup(func() {
			if restored, restoreErr := RestoreScheduledChannel(currentId); restoreErr == nil {
				assert.True(t, restored)
			}
		})
	}
	assert.Equal(t, 1, enabledCount)
}

func TestModelErrorDisablesWholeChannelOnlyAfterConsecutiveFailures(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-model-fault-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91501, group, 10)
	createSchedulerCandidate(t, db, 91502, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91501, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("model not found"), types.ErrorCodeModelNotFound, 404)
	for range operation_setting.ChannelSchedulingDefaultFailureCount {
		finishSchedulerFailure(channel, group, relayErr)
	}
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(91501); restoreErr == nil {
			assert.True(t, restored)
		}
	})

	disabled, err := model.GetChannelById(91501, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	assert.True(t, model.IsChannelSchedulingFault(disabled))
	assert.False(t, IsChannelUsableForScheduling(group, "gpt-test", 91501))
	states, err := model.ListChannelModelStates(true)
	require.NoError(t, err)
	assert.Empty(t, states)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx: context, ModelName: "gpt-test", TokenGroup: group, RequestPath: "/v1/chat/completions",
	})
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, group, selectedGroup)
	assert.Equal(t, 91502, selected.Id)
}

func TestSuccessfulAttemptResetsChannelFailureStreak(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-failure-reset-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91511, group, 10)
	createSchedulerCandidate(t, db, 91513, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91511, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, 524)
	for range operation_setting.ChannelSchedulingDefaultFailureCount - 1 {
		finishSchedulerFailure(channel, group, relayErr)
	}
	finishSchedulerSuccess(channel, group)

	streak, hasState := getChannelFailureStreakForTest(channel.Id)
	assert.True(t, hasState)
	assert.Zero(t, streak.Count)

	for range operation_setting.ChannelSchedulingDefaultFailureCount - 1 {
		finishSchedulerFailure(channel, group, relayErr)
	}
	stillEnabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)

	finishSchedulerFailure(channel, group, relayErr)
	disabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(channel.Id); restoreErr == nil {
			assert.True(t, restored)
		}
	})
}

func TestConfiguredFailureThresholdDelaysChannelDisable(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-configured-failure-threshold-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91521, group, 10)
	createSchedulerCandidate(t, db, 91522, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	original := operation_setting.GetChannelSchedulingSetting()
	const configuredThreshold = 7
	require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
		"failure_threshold": fmt.Sprintf("%d", configuredThreshold),
	}))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
			"failure_threshold": fmt.Sprintf("%d", original.FailureThreshold),
		}))
	})

	channel, err := model.GetChannelById(91521, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	for range configuredThreshold - 1 {
		finishSchedulerFailure(channel, group, relayErr)
	}
	stillEnabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)

	finishSchedulerFailure(channel, group, relayErr)
	disabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, disabled.Status)
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(channel.Id); restoreErr == nil {
			assert.True(t, restored)
		}
	})
}

func TestExpiredChannelFailureStreakRestartsFromOne(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-failure-window-test"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91512, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91512, true)
	require.NoError(t, err)
	t.Cleanup(func() { clearIntelligentSchedulingFailure(channel.Id) })
	shard := getChannelFailureShard(channel.Id)
	shard.mu.Lock()
	if shard.streaks == nil {
		shard.streaks = make(map[int]channelFailureStreak)
	}
	shard.streaks[channel.Id] = channelFailureStreak{
		Count: operation_setting.ChannelSchedulingDefaultFailureCount - 1,
		LastFailureAt: time.Now().Add(
			-time.Duration(operation_setting.ChannelSchedulingDefaultFailureSec)*time.Second - time.Second,
		),
	}
	shard.mu.Unlock()

	finishSchedulerFailure(channel, group, types.NewOpenAIError(
		fmt.Errorf("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden,
	))

	streak, _ := getChannelFailureStreakForTest(channel.Id)
	assert.Equal(t, 1, streak.Count)
	stillEnabled, err := model.GetChannelById(channel.Id, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, stillEnabled.Status)
}

func TestIntelligentChannelTestFaultUsesManualOnlySchedulerDisable(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	const group = "scheduler-health-test-fault"
	configureSchedulerGroupStrategyForTest(t, group, operation_setting.ChannelSchedulingStrategyIntelligent)
	createSchedulerCandidate(t, db, 91601, group, 10)
	createSchedulerCandidate(t, db, 91602, group, 10)
	common.MemoryCacheEnabled = true
	model.InitChannelCache()

	channel, err := model.GetChannelById(91601, true)
	require.NoError(t, err)
	relayErr := types.NewOpenAIError(fmt.Errorf("rate limited"), types.ErrorCodeBadResponseStatusCode, 429)
	for range operation_setting.ChannelSchedulingDefaultFailureCount - 1 {
		assert.False(t, HandleIntelligentSchedulingChannelTestFault(channel, "gpt-test", relayErr))
	}
	assert.True(t, HandleIntelligentSchedulingChannelTestFault(channel, "gpt-test", relayErr))
	t.Cleanup(func() {
		if restored, restoreErr := RestoreScheduledChannel(91601); restoreErr == nil {
			assert.True(t, restored)
		}
	})

	disabled, err := model.GetChannelById(91601, true)
	require.NoError(t, err)
	assert.True(t, model.IsChannelSchedulingFault(disabled))
	previousAutomaticEnable := common.AutomaticEnableChannelEnabled
	common.AutomaticEnableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticEnableChannelEnabled = previousAutomaticEnable })
	assert.False(t, ShouldEnableChannel(nil, disabled))
}

func TestChannelModelSchedulingFaultPersistsAndRequiresManualRestore(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91001, common.ChannelStatusEnabled, "")

	require.NoError(t, model.DisableChannelModel(91001, "gpt-test", "unsupported model", "model_not_found", 404))
	require.NoError(t, model.DisableChannelModel(91001, "gpt-test", "still unsupported", "model_not_found", 404))
	reloadChannelSchedulerFaults()

	states, err := model.ListChannelModelStates(true)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, "still unsupported", states[0].Reason)
	assert.False(t, IsChannelUsableForScheduling("default", "gpt-test", 91001))
	assert.False(t, IsChannelModelUsableForScheduling("gpt-test", 91001))

	changed, err := RestoreScheduledChannelModel(91001, "gpt-test")
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, IsChannelUsableForScheduling("default", "gpt-test", 91001))
	assert.True(t, IsChannelModelUsableForScheduling("gpt-test", 91001))

	states, err = model.ListChannelModelStates(true)
	require.NoError(t, err)
	assert.Empty(t, states)
}

func TestManualModelDisableStopsAutomaticRecoveryUntilRestore(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91021, common.ChannelStatusEnabled, "")
	require.NoError(t, model.DisableChannelModel(91021, "gpt-test", "unsupported model", "model_not_found", 404))
	reloadChannelSchedulerFaults()

	require.NoError(t, MarkScheduledChannelModelManualDisabled(91021, "gpt-test"))
	states, err := model.ListChannelModelStates(true)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.True(t, states[0].ManualDisabled)
	assert.False(t, IsChannelModelUsableForScheduling("gpt-test", 91021))

	changed, err := RestoreScheduledChannelModel(91021, "gpt-test")
	require.NoError(t, err)
	assert.True(t, changed)
	states, err = model.ListChannelModelStates(false)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.False(t, states[0].Disabled)
	assert.False(t, states[0].ManualDisabled)
}

func TestSchedulingStatusPreservesChannelMetadataAndSyncsManualChanges(t *testing.T) {
	db := useNodeSync(t)
	createSchedulerPersistenceChannel(t, db, 91024, common.ChannelStatusEnabled, "")
	require.NoError(t, model.DisableChannelForScheduling(91024, "initial failure"))
	_, err := model.UpdateChannelLocked(91024, func(channel *model.Channel) error {
		channel.Key = "refreshed-fixture-key"
		info := channel.GetOtherInfo()
		info["account_label"] = "preserved account"
		channel.SetOtherInfo(info)
		return nil
	})
	require.NoError(t, err)
	// A repeated failure updates the reason on an already disabled channel.
	require.NoError(t, model.DisableChannelForScheduling(91024, "repeated failure"))
	channel, err := model.GetChannelById(91024, true)
	require.NoError(t, err)
	assert.Equal(t, "refreshed-fixture-key", channel.Key)
	assert.Equal(t, "preserved account", channel.GetOtherInfo()["account_label"])
	assert.Equal(t, "repeated failure", model.GetChannelSchedulingFaultReason(channel))

	before := storedSyncTopics(t)[model.SyncTopicChannels]
	changed, err := model.RestoreChannelForScheduling(91024)
	require.NoError(t, err)
	require.True(t, changed)
	assert.Greater(t, storedSyncTopics(t)[model.SyncTopicChannels], before)
	channel, err = model.GetChannelById(91024, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)

	require.NoError(t, model.DisableChannelForScheduling(91024, "new failure"))
	before = storedSyncTopics(t)[model.SyncTopicChannels]
	changed, err = model.MarkChannelManualDisabledForScheduling(91024)
	require.NoError(t, err)
	require.True(t, changed)
	assert.Greater(t, storedSyncTopics(t)[model.SyncTopicChannels], before)
	changed, err = model.RestoreChannelForScheduling(91024)
	require.NoError(t, err)
	assert.False(t, changed, "manual disable must not be automatically restored")
}

func TestManualChannelDisableCannotBeAutomaticallyRestored(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91022, common.ChannelStatusEnabled, "")
	require.NoError(t, model.DisableChannelForScheduling(91022, "upstream failed"))

	changed, err := MarkScheduledChannelManualDisabled(91022)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, EnableChannelIfEligible(91022, "scheduler-channel-91022"))

	channel, err := model.GetChannelById(91022, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, channel.Status)
}

func TestSchedulingChannelCanRecoverWithoutGlobalAutoEnable(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91023, common.ChannelStatusEnabled, "")
	require.NoError(t, model.DisableChannelForScheduling(91023, "upstream failed"))
	original := common.AutomaticEnableChannelEnabled
	common.AutomaticEnableChannelEnabled = false
	t.Cleanup(func() { common.AutomaticEnableChannelEnabled = original })

	assert.True(t, EnableChannelIfEligible(91023, "scheduler-channel-91023"))

	channel, err := model.GetChannelById(91023, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
}

func TestRestoreScheduledChannelRejectsNonSchedulerDisabledChannels(t *testing.T) {
	db := setupChannelSchedulerPersistenceTest(t)
	createSchedulerPersistenceChannel(t, db, 91011, common.ChannelStatusEnabled, "")
	createSchedulerPersistenceChannel(t, db, 91012, common.ChannelStatusManuallyDisabled, "manual operation")
	createSchedulerPersistenceChannel(t, db, 91013, common.ChannelStatusAutoDisabled, "ordinary automatic disable")

	require.NoError(t, model.DisableChannelForScheduling(91011, "rate limited"))
	changed, err := RestoreScheduledChannel(91011)
	require.NoError(t, err)
	assert.True(t, changed)

	restored, err := model.GetChannelById(91011, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusEnabled, restored.Status)
	assert.False(t, model.IsChannelSchedulingFault(restored))

	for _, channelId := range []int{91012, 91013} {
		changed, err = RestoreScheduledChannel(channelId)
		require.NoError(t, err)
		assert.False(t, changed)
	}

	manuallyDisabled, err := model.GetChannelById(91012, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, manuallyDisabled.Status)
	ordinarilyDisabled, err := model.GetChannelById(91013, true)
	require.NoError(t, err)
	assert.Equal(t, common.ChannelStatusAutoDisabled, ordinarilyDisabled.Status)
}

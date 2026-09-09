package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectAutoGroupsTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMaxTokenAutoGroups := setting.GetMaxTokenAutoGroups()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMaxTokenAutoGroups)))

		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func createChannelSelectAutoGroupsChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func TestCacheGetRandomSatisfiedChannelUsesTokenAutoGroupsWhenGlobalAutoIsEmpty(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-groups-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2101, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2102, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "auto",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2101, first.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
	assert.Empty(t, setting.GetAutoGroups(), "the selection must not depend on the global Auto list")

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2102, second.Id)
	assert.Equal(t, "default", selectedGroup)
	assert.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

func TestAutoGroupRelayAttemptsRespectOrderBudgetAndCrossGroupSwitch(t *testing.T) {
	for _, test := range []struct {
		name          string
		retries       int
		crossGroup    bool
		smart         bool
		maxAttempts   int
		sharedChannel bool
		wantGroups    []string
	}{
		{name: "legacy zero retries still reaches next group", retries: 0, crossGroup: true, wantGroups: []string{"vip", "default"}},
		{name: "legacy each group has its own budget", retries: 1, crossGroup: true, wantGroups: []string{"vip", "vip", "default", "default"}},
		{name: "legacy failed channels are not reused", retries: 5, crossGroup: true, wantGroups: []string{"vip", "vip", "default", "default"}},
		{name: "legacy cross group disabled", retries: 5, wantGroups: []string{"vip", "vip"}},
		{name: "intelligent cross group enabled", retries: 0, crossGroup: true, smart: true, wantGroups: []string{"vip", "vip", "default", "default"}},
		{name: "intelligent cross group disabled", retries: 0, smart: true, wantGroups: []string{"vip", "vip"}},
		{name: "intelligent request budget stays bounded", crossGroup: true, smart: true, maxAttempts: 2, wantGroups: []string{"vip", "vip"}},
		{name: "failed channel shared by groups is excluded", retries: 5, crossGroup: true, sharedChannel: true, wantGroups: []string{"vip", "vip", "default", "default"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupChannelSelectAutoGroupsTest(t)
			common.RetryTimes = test.retries
			original := operation_setting.GetChannelSchedulingSetting()
			originalGroups, err := common.Marshal(original.GroupStrategies)
			require.NoError(t, err)
			strategies := `{"vip":"legacy","default":"legacy"}`
			if test.smart {
				strategies = `{"vip":"intelligent","default":"intelligent"}`
			}
			maxAttempts := 8
			if test.maxAttempts > 0 {
				maxAttempts = test.maxAttempts
			}
			require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{"group_strategies": strategies, "max_attempts": fmt.Sprint(maxAttempts)}))
			t.Cleanup(func() {
				require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
					"group_strategies": string(originalGroups), "max_attempts": fmt.Sprint(original.MaxAttempts),
				}))
			})
			const modelName = "auto-retry-path-model"
			createChannelSelectAutoGroupsChannel(t, db, 2201, "vip", modelName)
			createChannelSelectAutoGroupsChannel(t, db, 2202, "vip", modelName)
			createChannelSelectAutoGroupsChannel(t, db, 2203, "default", modelName)
			createChannelSelectAutoGroupsChannel(t, db, 2204, "default", modelName)
			if test.sharedChannel {
				require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 2201).Update("group", "vip,default").Error)
				require.NoError(t, db.Create(&model.Ability{Group: "default", Model: modelName, ChannelId: 2201, Enabled: true, Weight: 100}).Error)
			}
			model.InitChannelCache()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
			common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, test.crossGroup)

			// Distributor selects the first channel with a different RetryParam
			// than the controller, but they share the same request context.
			first, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName})
			require.NoError(t, err)
			require.NotNil(t, first)
			param := &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName}
			limit := GetRelayAttemptLimit(param)
			groups := []string{group}
			seen := map[int]bool{first.Id: true}
			param.MarkAttempted(first.Id)
			for attempt := 1; attempt < limit; attempt++ {
				param.IncreaseRetry()
				channel, group, err := CacheGetRandomSatisfiedChannel(param)
				require.NoError(t, err)
				if channel == nil {
					break
				}
				assert.False(t, seen[channel.Id], "a failed channel must not be retried")
				seen[channel.Id] = true
				groups = append(groups, group)
				param.MarkAttempted(channel.Id)
				assert.Equal(t, attempt, param.GetRetry(), "request attempt index must not reset when changing groups")
			}
			assert.Equal(t, test.wantGroups, groups)
		})
	}
}

func TestLegacyRetryDoesNotSkipTheNextPriorityAfterExcludingFailedChannel(t *testing.T) {
	for _, test := range []struct {
		name     string
		disable  bool
		affinity bool
	}{
		{name: "failed channel remains enabled"},
		{name: "failed channel removed from cache", disable: true},
		{name: "affinity channel removed from cache", disable: true, affinity: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := setupChannelSelectAutoGroupsTest(t)
			const modelName = "legacy-retry-priority-model"
			for index, priority := range []int64{30, 20, 10} {
				id := 2301 + index
				createChannelSelectAutoGroupsChannel(t, db, id, "default", modelName)
				require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", id).Update("priority", priority).Error)
				require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", id).Update("priority", priority).Error)
			}
			model.InitChannelCache()
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "default")
			param := &RetryParam{Ctx: ctx, TokenGroup: "default", ModelName: modelName}
			for index, wantID := range []int{2301, 2302, 2303} {
				if index > 0 {
					param.IncreaseRetry()
				}
				var channel *model.Channel
				var err error
				if test.affinity && index == 0 {
					channel, err = model.CacheGetChannel(wantID)
				} else {
					channel, _, err = CacheGetRandomSatisfiedChannel(param)
				}
				require.NoError(t, err)
				require.NotNil(t, channel)
				assert.Equal(t, wantID, channel.Id)
				param.MarkChannelAttempted(channel)
				if test.disable {
					require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("status", common.ChannelStatusAutoDisabled).Error)
					model.InitChannelCache()
				}
			}
			param.IncreaseRetry()
			channel, _, err := CacheGetRandomSatisfiedChannel(param)
			require.NoError(t, err)
			assert.Nil(t, channel, "exhausted priorities must not reuse the last failed channel")
		})
	}
}

func TestAutoGroupWithoutCrossGroupRetryStillSkipsInitiallyUnavailableGroup(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-initial-empty-group-model"
	createChannelSelectAutoGroupsChannel(t, db, 2401, "default", modelName)
	model.InitChannelCache()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	param := &RetryParam{Ctx: ctx, TokenGroup: "auto", ModelName: modelName}
	channel, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2401, channel.Id)
	assert.Equal(t, "default", group)
}

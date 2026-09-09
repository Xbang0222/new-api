package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAutoGroupRetryContinuesAfterWaitingForAnEarlierRateLimitedGroup(t *testing.T) {
	for _, strategy := range []string{"legacy", "intelligent"} {
		t.Run(strategy, func(t *testing.T) {
			previousDB := model.DB
			previousMemoryCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			previousRetries := common.RetryTimes
			previousGroups := setting.UserUsableGroups2JSONString()
			previousRatios := ratio_setting.GroupRatio2JSONString()
			previousMaxGroups := setting.GetMaxTokenAutoGroups()
			previousScheduling := operation_setting.GetChannelSchedulingSetting()
			previousStrategies, err := common.Marshal(previousScheduling.GroupStrategies)
			require.NoError(t, err)
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() {
				model.DB = previousDB
				common.MemoryCacheEnabled, common.RedisEnabled = previousMemoryCache, previousRedis
				common.SetDatabaseTypes(previousMainType, previousLogType)
				common.RetryTimes = previousRetries
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
				require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprint(previousMaxGroups)))
				require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{"group_strategies": string(previousStrategies)}))
				if previousMemoryCache && previousDB != nil && previousDB.Migrator().HasTable(&model.Channel{}) && previousDB.Migrator().HasTable(&model.Ability{}) {
					model.InitChannelCache()
				}
				require.NoError(t, sqlDB.Close())
			})
			model.DB = db
			common.MemoryCacheEnabled, common.RedisEnabled = true, false
			common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
			common.RetryTimes = 1
			require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
			require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))
			require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
				"group_strategies": fmt.Sprintf(`{"default":%q,"vip":%q}`, strategy, strategy),
			}))

			const modelName = "auto-wait-retry-model"
			channels := make([]model.Channel, 0, 2)
			for index, group := range []string{"vip", "default"} {
				priority := int64(10)
				channel := model.Channel{Id: 6401 + index, Type: constant.ChannelTypeOpenAI, Key: "test-key", Name: group, Group: group, Models: modelName, Status: common.ChannelStatusEnabled, Priority: &priority}
				channel.SetOtherSettings(dto.ChannelOtherSettings{RequestRateLimit: &dto.ChannelRequestRateLimit{MaxRequests: 1, WindowSeconds: 60}})
				require.NoError(t, db.Create(&channel).Error)
				require.NoError(t, db.Create(&model.Ability{ChannelId: channel.Id, Group: group, Model: modelName, Enabled: true, Priority: &priority, Weight: 100}).Error)
				channels = append(channels, channel)
			}
			model.InitChannelCache()
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			requestCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(requestCtx)
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
			common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, true)
			first, group, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: modelName})
			require.NoError(t, err)
			require.NotNil(t, first)
			require.Equal(t, channels[0].Id, first.Id)
			require.Nil(t, middleware.SetupContextForSelectedChannel(c, first, modelName))
			common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
			info := &relaycommon.RelayInfo{TokenGroup: "auto", OriginModelName: modelName, UsingGroup: group}
			param := &service.RetryParam{Ctx: c, TokenGroup: "auto", ModelName: modelName}

			blockers := make([]*service.ChannelRateLimitReservation, 0, len(channels))
			for index := range channels {
				reservation, decision, err := service.ReserveChannelRequest(requestCtx, &channels[index])
				require.NoError(t, err)
				require.True(t, decision.Allowed)
				require.NotNil(t, reservation)
				defer service.ReleaseChannelRequestReservation(reservation)
				blockers = append(blockers, reservation)
			}

			var candidate *channelRateLimitWaitCandidate
			for index := range channels {
				channel, channelErr := getChannel(c, info, param)
				require.Nil(t, channelErr)
				require.NotNil(t, channel)
				require.Equal(t, channels[index].Id, channel.Id)
				reservation, decision, err := service.ReserveChannelRequest(requestCtx, channel)
				require.NoError(t, err)
				require.False(t, decision.Allowed)
				require.Nil(t, reservation)
				if index == 0 {
					candidate = &channelRateLimitWaitCandidate{channel: channel, group: info.UsingGroup, retryAfterMillis: decision.RetryAfterMillis, autoGroupRetryIndex: common.GetContextKeyInt(c, constant.ContextKeyAutoGroupRetryIndex)}
				}
				param.MarkRateLimited(channel.Id, decision.RetryAfterMillis)
				info.ChannelMeta = nil
			}
			channel, channelErr := getChannel(c, info, param)
			require.Nil(t, channel)
			require.NotNil(t, channelErr)
			assert.Equal(t, types.ErrorCodeChannelRateLimited, channelErr.GetErrorCode())
			assert.Zero(t, param.GetRetry(), "rate-limit filtering must not count as an upstream attempt")

			// Make the earlier wait candidate immediately reservable; exercise the
			// real queue path without advancing time or sleeping in the test.
			service.ReleaseChannelRequestReservation(blockers[0])
			waited, reservation, waitErr := waitForRateLimitedChannel(c, info, param, candidate)
			require.Nil(t, waitErr)
			require.NotNil(t, waited)
			require.NotNil(t, reservation)
			defer service.ReleaseChannelRequestReservation(reservation)
			assert.Equal(t, channels[0].Id, waited.Id)
			assert.Equal(t, "vip", info.UsingGroup)
			assert.False(t, param.HasRateLimitedChannels())

			// The reserved channel now fails upstream. The second group becomes
			// available and must still be reachable after restoring the wait group.
			param.MarkChannelAttempted(waited)
			info.UpstreamStartTime = time.Now()
			upstreamErr := types.NewOpenAIError(errors.New("waited upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
			require.True(t, shouldRetry(c, info, upstreamErr, 1))
			param.IncreaseRetry()
			service.ReleaseChannelRequestReservation(blockers[1])
			channel, channelErr = getChannel(c, info, param)
			require.Nil(t, channelErr)
			require.NotNil(t, channel)
			assert.Equal(t, channels[1].Id, channel.Id)
			assert.Equal(t, "default", info.UsingGroup)
			assert.Equal(t, 1, param.GetRetry())
			nextReservation, decision, err := service.ReserveChannelRequest(requestCtx, channel)
			require.NoError(t, err)
			require.True(t, decision.Allowed)
			require.NotNil(t, nextReservation)
			service.ReleaseChannelRequestReservation(nextReservation)
		})
	}
}

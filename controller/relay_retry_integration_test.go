package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelayHTTPFailoverUsesAnotherChannelAndStopsAfterStreamContent(t *testing.T) {
	for _, test := range []struct {
		name                 string
		auto                 bool
		crossGroup           bool
		intelligent          bool
		stream               bool
		ollama               bool
		retries              int
		wantFallback         int32
		wantStatus           int
		paidFallback         bool
		perCall              bool
		walletQuota          *int
		limitedToken         bool
		subscriptionOnly     bool
		activeSubscription   bool
		wantWalletQuota      *int
		wantSubscriptionUsed int64
	}{
		{name: "legacy same group next priority", retries: 1, wantFallback: 1, wantStatus: 200},
		{name: "legacy auto next group with zero retries", auto: true, crossGroup: true, wantFallback: 1, wantStatus: 200},
		{name: "legacy auto respects cross group off", auto: true, retries: 3, wantStatus: 500},
		{name: "intelligent auto respects cross group off", auto: true, intelligent: true, wantStatus: 500},
		{name: "stream interrupted after content", stream: true, retries: 1, wantStatus: 200},
		{name: "ollama stream invalid JSON after content", stream: true, ollama: true, intelligent: true, retries: 1, wantStatus: 200},
		{name: "free to paid ratio blocks empty wallet", auto: true, crossGroup: true, paidFallback: true, walletQuota: common.GetPointer(0), wantStatus: 403, wantWalletQuota: common.GetPointer(0)},
		{name: "free to paid per call blocks empty wallet", auto: true, crossGroup: true, paidFallback: true, perCall: true, walletQuota: common.GetPointer(0), wantStatus: 403, wantWalletQuota: common.GetPointer(0)},
		{name: "free to paid blocks empty token", auto: true, crossGroup: true, paidFallback: true, limitedToken: true, wantStatus: 403, wantWalletQuota: common.GetPointer(1_000_000)},
		{name: "free to paid subscription only cannot charge wallet", auto: true, crossGroup: true, paidFallback: true, subscriptionOnly: true, wantStatus: 403, wantWalletQuota: common.GetPointer(1_000_000)},
		{name: "free to paid wallet settles once", auto: true, crossGroup: true, paidFallback: true, wantFallback: 1, wantStatus: 200, wantWalletQuota: common.GetPointer(999_998)},
		{name: "free to paid uses active subscription", auto: true, crossGroup: true, paidFallback: true, subscriptionOnly: true, activeSubscription: true, wantFallback: 1, wantStatus: 200, wantWalletQuota: common.GetPointer(1_000_000), wantSubscriptionUsed: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			service.InitHttpClient()
			require.NoError(t, i18n.Init())
			previousDB := model.DB
			previousMemoryCache := common.MemoryCacheEnabled
			previousRedis := common.RedisEnabled
			previousBatchUpdate := common.BatchUpdateEnabled
			previousLogConsume := common.LogConsumeEnabled
			previousErrorLog := constant.ErrorLogEnabled
			previousCountToken := constant.CountToken
			previousStreamingTimeout := constant.StreamingTimeout
			previousAutomaticDisable := common.AutomaticDisableChannelEnabled
			previousRetries := common.RetryTimes
			previousFreePreConsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
			previousModelRatios := ratio_setting.ModelRatio2JSONString()
			previousModelPrices := ratio_setting.ModelPrice2JSONString()
			previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
			previousGroupRatios := ratio_setting.GroupRatio2JSONString()
			previousUsableGroups := setting.UserUsableGroups2JSONString()
			previousMaxGroups := setting.GetMaxTokenAutoGroups()
			scheduling := operation_setting.GetChannelSchedulingSetting()
			previousStrategies, err := common.Marshal(scheduling.GroupStrategies)
			require.NoError(t, err)
			initModelListColumnNames(t)
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Token{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
			model.DB = db
			common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
			common.MemoryCacheEnabled = true
			common.RedisEnabled = false
			common.BatchUpdateEnabled = false
			common.LogConsumeEnabled = false
			constant.ErrorLogEnabled = false
			constant.CountToken = false
			constant.StreamingTimeout = 30
			common.AutomaticDisableChannelEnabled = false
			common.RetryTimes = test.retries
			operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
			const modelName = "relay-http-failover-test"
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"relay-http-failover-test":0}`))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
			if test.paidFallback {
				require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"relay-http-failover-test":1}`))
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0}`))
				if test.perCall {
					require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"relay-http-failover-test":0.01}`))
				}
			}
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
			require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))
			strategies := `{"default":"legacy","vip":"legacy"}`
			if test.intelligent {
				strategies = `{"default":"intelligent","vip":"intelligent"}`
			}
			require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{"group_strategies": strategies, "failure_threshold": "100"}))
			t.Cleanup(func() {
				// Relay schedules metrics and notifications after responding.
				// Join that work before restoring globals or closing its database.
				deadline := time.Now().Add(5 * time.Second)
				for gopool.WorkerCount() != 0 {
					require.True(t, time.Now().Before(deadline), "relay background work did not finish")
					runtime.Gosched()
				}
				model.DB = previousDB
				common.SetDatabaseTypes(previousMainType, previousLogType)
				common.MemoryCacheEnabled = previousMemoryCache
				if previousMemoryCache && previousDB != nil && previousDB.Migrator().HasTable(&model.Channel{}) && previousDB.Migrator().HasTable(&model.Ability{}) {
					model.InitChannelCache()
				}
				common.RedisEnabled = previousRedis
				common.BatchUpdateEnabled = previousBatchUpdate
				common.LogConsumeEnabled = previousLogConsume
				constant.ErrorLogEnabled = previousErrorLog
				constant.CountToken = previousCountToken
				constant.StreamingTimeout = previousStreamingTimeout
				common.AutomaticDisableChannelEnabled = previousAutomaticDisable
				common.RetryTimes = previousRetries
				operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = previousFreePreConsume
				require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
				require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousModelPrices))
				require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
				require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
				require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprint(previousMaxGroups)))
				require.NoError(t, operation_setting.ApplyChannelSchedulingConfig(map[string]string{
					"group_strategies": string(previousStrategies), "failure_threshold": fmt.Sprint(scheduling.FailureThreshold),
				}))
				sqlDB, err := db.DB()
				require.NoError(t, err)
				require.NoError(t, sqlDB.Close())
			})
			walletQuota := 1_000_000
			if test.walletQuota != nil {
				walletQuota = *test.walletQuota
			}
			require.NoError(t, db.Create(&model.User{Id: 1, Username: "retry-test-user", Group: "default", Quota: walletQuota}).Error)
			require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "retry-test-token", Status: common.TokenStatusEnabled, UnlimitedQuota: !test.limitedToken}).Error)
			if test.activeSubscription {
				plan := model.SubscriptionPlan{Id: 9901, Title: "Retry subscription", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, QuotaResetPeriod: model.SubscriptionResetNever}
				require.NoError(t, db.Create(&plan).Error)
				model.InvalidateSubscriptionPlanCache(plan.Id)
				t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
				require.NoError(t, db.Create(&model.UserSubscription{Id: 9901, UserId: 1, PlanId: plan.Id, Status: "active", EndTime: time.Now().Unix() + 3600, AmountTotal: 1_000_000}).Error)
			}

			var firstCalls, fallbackCalls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/primary/") {
					firstCalls.Add(1)
					if test.ollama {
						w.Header().Set("Content-Type", "application/x-ndjson")
						_, _ = io.WriteString(w, `{"model":"relay-http-failover-test","message":{"role":"assistant","content":"partial answer"},"done":false}`+"\n{broken-json\n")
						return
					}
					if test.stream {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","model":"relay-http-failover-test","choices":[{"index":0,"delta":{"content":"partial answer"},"finish_reason":null}]}`+"\n\n")
						_, _ = io.WriteString(w, `data: {"error":{"message":"stream interrupted","type":"server_error","code":"upstream_failure"}}`+"\n\n")
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = io.WriteString(w, `{"error":{"message":"primary failed","type":"server_error","code":"upstream_failure"}}`)
					return
				}
				fallbackCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"chatcmpl-success","object":"chat.completion","model":"relay-http-failover-test","choices":[{"index":0,"message":{"role":"assistant","content":"fallback success"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			}))
			defer upstream.Close()
			for index := range 2 {
				group, baseURL, priority := "default", upstream.URL+"/fallback", int64(10)
				if index == 0 {
					baseURL, priority = upstream.URL+"/primary", 20
					if test.auto {
						group = "vip"
					}
				}
				channel := model.Channel{Id: 3101 + index, Type: constant.ChannelTypeOpenAI, Key: "test-upstream-key", Status: common.ChannelStatusEnabled, Name: fmt.Sprintf("retry-%d", index), BaseURL: &baseURL, Models: modelName, Group: group, Priority: &priority}
				if index == 0 && test.ollama {
					channel.Type = constant.ChannelTypeOllama
				}
				require.NoError(t, db.Create(&channel).Error)
				require.NoError(t, db.Create(&model.Ability{Group: group, Model: modelName, ChannelId: channel.Id, Enabled: true, Priority: &priority, Weight: 100}).Error)
			}
			model.InitChannelCache()
			engine := gin.New()
			engine.Use(func(c *gin.Context) {
				group := "default"
				if test.auto {
					group = "auto"
				}
				common.SetContextKey(c, constant.ContextKeyUserId, 1)
				common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
				common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
				common.SetContextKey(c, constant.ContextKeyTokenGroup, group)
				common.SetContextKey(c, constant.ContextKeyTokenId, 1)
				common.SetContextKey(c, constant.ContextKeyTokenKey, "retry-test-token")
				common.SetContextKey(c, constant.ContextKeyTokenUnlimited, !test.limitedToken)
				common.SetContextKey(c, constant.ContextKeyUserQuota, walletQuota)
				preference := "wallet_only"
				if test.subscriptionOnly {
					preference = "subscription_only"
				}
				common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: preference})
				c.Set(common.RequestIdKey, "retry-funding-contract")
				common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
				common.SetContextKey(c, constant.ContextKeyTokenCrossGroupRetry, test.crossGroup)
				defer common.CleanupBodyStorage(c)
				c.Next()
			})
			engine.POST("/v1/chat/completions", middleware.Distribute(), func(c *gin.Context) { Relay(c, types.RelayFormatOpenAI) })
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}],"stream":%t}`, modelName, test.stream)))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, int32(1), firstCalls.Load(), recorder.Body.String())
			assert.Equal(t, test.wantFallback, fallbackCalls.Load(), recorder.Body.String())
			assert.Equal(t, test.wantStatus, recorder.Code, recorder.Body.String())
			if test.wantFallback > 0 {
				assert.Contains(t, recorder.Body.String(), "fallback success")
			} else if test.stream {
				assert.Contains(t, recorder.Body.String(), "partial answer")
				assert.Contains(t, recorder.Body.String(), "model_unavailable")
			} else if !test.paidFallback {
				assert.Contains(t, recorder.Body.String(), "model_unavailable")
			}
			if test.wantWalletQuota != nil {
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				assert.Equal(t, *test.wantWalletQuota, user.Quota, "fallback must preserve the selected funding contract")
			}
			if test.activeSubscription {
				var subscription model.UserSubscription
				require.NoError(t, db.First(&subscription, 9901).Error)
				assert.Equal(t, test.wantSubscriptionUsed, subscription.AmountUsed, "successful fallback must settle the subscription exactly once")
			}
			if test.limitedToken {
				var token model.Token
				require.NoError(t, db.First(&token, 1).Error)
				assert.Zero(t, token.RemainQuota, "a rejected fallback must leave the token quota intact")
				assert.Zero(t, token.UsedQuota)
			}
		})
	}
}

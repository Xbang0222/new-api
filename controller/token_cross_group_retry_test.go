package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateTokenCrossGroupRetryReplacesCachedValueForAuthentication(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousRDB := common.RedisEnabled, common.RDB
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled, common.RDB = previousRedis, previousRDB
	})
	configureTokenAutoGroupsTest(t, "2", `["default","vip"]`)
	initModelListColumnNames(t)
	user := setupTokenAutoGroupsControllerTest(t)
	token := seedToken(t, model.DB, user.Id, "cross-group-switch", "cross-group-switch-test-key")
	token.Group = "auto"
	token.CrossGroupRetry = true
	token.ExpiredTime = -1
	token.UnlimitedQuota = true
	require.NoError(t, token.SetAutoGroups([]string{"vip", "default"}))
	require.NoError(t, model.DB.Save(token).Error)

	// Exercise the Redis protocol and the production cache Lua with miniredis.
	// This test does not claim compatibility with an external Redis deployment.
	cache := miniredis.RunT(t)
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { require.NoError(t, common.RDB.Close()) })
	cacheKey := fmt.Sprintf("token:%s", common.GenerateHMAC(token.Key))
	current := true
	for _, enabled := range []bool{false, true} {
		// Advance only the in-process cache clock so the previous mutation fence
		// expires and both switch directions begin with a preheated token hash.
		cache.FastForward(time.Hour)
		preheated, err := model.GetTokenByKey(token.Key, false)
		require.NoError(t, err)
		require.Equal(t, current, preheated.CrossGroupRetry)
		require.True(t, cache.Exists(cacheKey))
		require.Equal(t, fmt.Sprint(current), cache.HGet(cacheKey, "CrossGroupRetry"))

		request := baseAutoTokenRequest("cross-group-switch")
		request["id"] = token.Id
		request["status"] = common.TokenStatusEnabled
		request["cross_group_retry"] = enabled
		request["auto_groups"] = []string{"vip", "default"}
		c, recorder := newTokenAutoGroupsAuthenticatedContext(t, http.MethodPut, "/api/token/", request, user.Id)
		UpdateToken(c)
		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var responseData struct {
			CrossGroupRetry bool     `json:"cross_group_retry"`
			AutoGroups      []string `json:"auto_groups"`
		}
		require.NoError(t, common.Unmarshal(response.Data, &responseData))
		assert.Equal(t, enabled, responseData.CrossGroupRetry)
		assert.Equal(t, []string{"vip", "default"}, responseData.AutoGroups)
		assert.False(t, cache.Exists(cacheKey), "saving the switch must invalidate the previously cached routing policy")

		authenticated, err := model.ValidateUserToken(token.Key)
		require.NoError(t, err)
		require.Equal(t, enabled, authenticated.CrossGroupRetry)
		require.Equal(t, "auto", authenticated.Group)
		requestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
		requestContext.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		common.SetContextKey(requestContext, constant.ContextKeyUserGroup, "default")
		require.NoError(t, middleware.SetupContextForToken(requestContext, authenticated))
		assert.Equal(t, enabled, common.GetContextKeyBool(requestContext, constant.ContextKeyTokenCrossGroupRetry))
		assert.Equal(t, []string{"vip", "default"}, service.GetRequestAutoGroups(requestContext, "default"))
		current = enabled
	}
}

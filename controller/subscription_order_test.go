package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type subscriptionOrderAPIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    *struct {
		BillingPreference string                      `json:"billing_preference"`
		SubscriptionOrder []int                       `json:"subscription_order"`
		Subscriptions     []model.SubscriptionSummary `json:"subscriptions"`
		AllSubscriptions  []model.SubscriptionSummary `json:"all_subscriptions"`
	} `json:"data"`
}

func setupSubscriptionOrderControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		model.InvalidateSubscriptionPlanCache(501)
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainType, previousLogType)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{},
	))

	setting, err := common.Marshal(dto.UserSetting{SubscriptionOrder: []int{103, 101}, BillingPreference: "subscription_only"})
	require.NoError(t, err)
	users := []model.User{
		{Id: 41, Username: "subscription-owner", Password: "unused", AffCode: "order-owner", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Setting: string(setting)},
		{Id: 42, Username: "subscription-other", Password: "unused", AffCode: "order-other", Status: common.UserStatusEnabled, Role: common.RoleCommonUser},
	}
	require.NoError(t, db.Create(&users).Error)
	plan := model.SubscriptionPlan{Id: 501, Title: "Order test plan", QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := time.Now().Unix()
	subscriptions := []model.UserSubscription{
		{Id: 101, UserId: 41, PlanId: plan.Id, Status: "active", StartTime: now - 3600, EndTime: now + 3600, AmountTotal: 100},
		{Id: 102, UserId: 41, PlanId: plan.Id, Status: "active", StartTime: now - 3600, EndTime: now + 7200, AmountTotal: 100},
		{Id: 103, UserId: 41, PlanId: plan.Id, Status: "active", StartTime: now - 3600, EndTime: now + 10800, AmountTotal: 100},
		{Id: 104, UserId: 41, PlanId: plan.Id, Status: "active", StartTime: now - 3600, EndTime: now - 60, AmountTotal: 100},
		{Id: 105, UserId: 41, PlanId: plan.Id, Status: "cancelled", StartTime: now - 3600, EndTime: now + 10800, AmountTotal: 100},
		{Id: 106, UserId: 42, PlanId: plan.Id, Status: "active", StartTime: now - 3600, EndTime: now + 10800, AmountTotal: 100},
	}
	require.NoError(t, db.Create(&subscriptions).Error)
	return db
}

func performSubscriptionOrderRequest(t *testing.T, method, path, body string, handler gin.HandlerFunc) subscriptionOrderAPIResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 41)
	c.Set("role", common.RoleCommonUser)
	handler(c)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response subscriptionOrderAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestGetSubscriptionSelfReturnsDebitOrderAndFiltersUnavailablePreferences(t *testing.T) {
	for _, tc := range []struct {
		name      string
		saved     []int
		wantOrder []int
		wantIDs   []int
	}{
		{name: "default earliest expiry", wantOrder: []int{}, wantIDs: []int{101, 102, 103}},
		{name: "saved order with unavailable IDs", saved: []int{104, 103, 106, 101, 105, 999}, wantOrder: []int{103, 101}, wantIDs: []int{103, 101, 102}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionOrderControllerTestDB(t)
			setting, err := common.Marshal(dto.UserSetting{SubscriptionOrder: tc.saved, BillingPreference: "subscription_only"})
			require.NoError(t, err)
			require.NoError(t, db.Model(&model.User{}).Where("id = ?", 41).Update("setting", string(setting)).Error)

			response := performSubscriptionOrderRequest(t, http.MethodGet, "/api/subscription/self", "", GetSubscriptionSelf)
			require.True(t, response.Success, response.Message)
			require.NotNil(t, response.Data)
			assert.Equal(t, "subscription_only", response.Data.BillingPreference)
			assert.Equal(t, tc.wantOrder, response.Data.SubscriptionOrder)
			activeIDs := make([]int, 0, len(response.Data.Subscriptions))
			for _, summary := range response.Data.Subscriptions {
				require.NotNil(t, summary.Subscription)
				activeIDs = append(activeIDs, summary.Subscription.Id)
			}
			assert.Equal(t, tc.wantIDs, activeIDs)
			allIDs := make([]int, 0, len(response.Data.AllSubscriptions))
			for _, summary := range response.Data.AllSubscriptions {
				require.NotNil(t, summary.Subscription)
				allIDs = append(allIDs, summary.Subscription.Id)
			}
			assert.ElementsMatch(t, []int{101, 102, 103, 104, 105}, allIDs)
			consumed, err := model.PreConsumeUserSubscription(t.Name(), 41, "test-model", 0, 5)
			require.NoError(t, err)
			require.NotNil(t, consumed)
			assert.Equal(t, tc.wantIDs[0], consumed.UserSubscriptionId)
		})
	}
}

func TestUpdateSubscriptionOrderRejectsInvalidRequestsWithoutChangingSettings(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing field", body: `{}`},
		{name: "null field", body: `{"subscription_ids":null}`},
		{name: "duplicate IDs", body: `{"subscription_ids":[101,101]}`},
		{name: "another user subscription", body: `{"subscription_ids":[101,106]}`},
		{name: "expired subscription", body: `{"subscription_ids":[101,104]}`},
		{name: "cancelled subscription", body: `{"subscription_ids":[101,105]}`},
		{name: "unknown subscription", body: `{"subscription_ids":[999]}`},
		{name: "nonpositive ID", body: `{"subscription_ids":[0]}`},
		{name: "wrong field type", body: `{"subscription_ids":"101"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionOrderControllerTestDB(t)
			var before model.User
			require.NoError(t, db.First(&before, 41).Error)

			response := performSubscriptionOrderRequest(t, http.MethodPut, "/api/subscription/self/order", tc.body, UpdateSubscriptionOrder)
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
			assert.Nil(t, response.Data)
			var after model.User
			require.NoError(t, db.First(&after, 41).Error)
			assert.Equal(t, before.Setting, after.Setting)
		})
	}
}

func TestUpdateSubscriptionOrderReturnsSavedOrderAndResetRestoresExpiryOrder(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		wantOrder []int
		wantIDs   []int
	}{
		{name: "save new order", body: `{"subscription_ids":[102,103,101]}`, wantOrder: []int{102, 103, 101}, wantIDs: []int{102, 103, 101}},
		{name: "restore default", body: `{"subscription_ids":[]}`, wantOrder: []int{}, wantIDs: []int{101, 102, 103}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionOrderControllerTestDB(t)
			response := performSubscriptionOrderRequest(t, http.MethodPut, "/api/subscription/self/order", tc.body, UpdateSubscriptionOrder)
			require.True(t, response.Success, response.Message)
			require.NotNil(t, response.Data)
			assert.Equal(t, tc.wantOrder, response.Data.SubscriptionOrder)
			activeIDs := make([]int, 0, len(response.Data.Subscriptions))
			for _, summary := range response.Data.Subscriptions {
				require.NotNil(t, summary.Subscription)
				activeIDs = append(activeIDs, summary.Subscription.Id)
			}
			assert.Equal(t, tc.wantIDs, activeIDs)
			var user model.User
			require.NoError(t, db.First(&user, 41).Error)
			assert.Equal(t, "subscription_only", user.GetSetting().BillingPreference)
			assert.Equal(t, tc.wantOrder, append([]int{}, user.GetSetting().SubscriptionOrder...))
			readback := performSubscriptionOrderRequest(t, http.MethodGet, "/api/subscription/self", "", GetSubscriptionSelf)
			require.True(t, readback.Success, readback.Message)
			assert.Equal(t, response.Data, readback.Data)
		})
	}
}

func TestUpdateUserNotificationSettingsPreservesSubscriptionOrder(t *testing.T) {
	db := setupSubscriptionOrderControllerTestDB(t)
	body := `{"notify_type":"email","quota_warning_threshold":100,"notification_email":"billing@example.com","record_ip_log":true}`
	response := performSubscriptionOrderRequest(t, http.MethodPut, "/api/user/setting", body, UpdateUserSetting)
	require.True(t, response.Success, response.Message)
	var user model.User
	require.NoError(t, db.First(&user, 41).Error)
	settings := user.GetSetting()
	assert.Equal(t, []int{103, 101}, settings.SubscriptionOrder)
	assert.Equal(t, dto.NotifyTypeEmail, settings.NotifyType)
	assert.Equal(t, float64(100), settings.QuotaWarningThreshold)
	assert.Equal(t, "billing@example.com", settings.NotificationEmail)
	assert.True(t, settings.RecordIpLog)
}

func TestGetSubscriptionSelfReportsReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table any
	}{
		{name: "user settings unavailable", table: &model.User{}},
		{name: "subscriptions unavailable", table: &model.UserSubscription{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := setupSubscriptionOrderControllerTestDB(t)
			require.NoError(t, db.Migrator().DropTable(tc.table))
			response := performSubscriptionOrderRequest(t, http.MethodGet, "/api/subscription/self", "", GetSubscriptionSelf)
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
			assert.Nil(t, response.Data)
		})
	}
}

func TestUpdateSubscriptionOrderRejectsOversizedBodiesBeforeChangingSettings(t *testing.T) {
	for _, knownLength := range []bool{true, false} {
		t.Run(fmt.Sprintf("content length known=%t", knownLength), func(t *testing.T) {
			db := setupSubscriptionOrderControllerTestDB(t)
			var before model.User
			require.NoError(t, db.First(&before, 41).Error)
			// A valid order after oversized whitespace must not reach persistence,
			// even when a streamed request has no Content-Length header.
			body := strings.Repeat(" ", 64<<10) + `{"subscription_ids":[102,101,103]}`
			reader := strings.NewReader(body)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/subscription/self/order", reader)
			c.Request.Header.Set("Content-Type", "application/json")
			if !knownLength {
				c.Request.ContentLength = -1
			}
			c.Set("id", 41)
			UpdateSubscriptionOrder(c)
			var response subscriptionOrderAPIResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
			assert.LessOrEqual(t, len(body)-reader.Len(), (32<<10)+1)
			var after model.User
			require.NoError(t, db.First(&after, 41).Error)
			assert.Equal(t, before.Setting, after.Setting)
		})
	}
}

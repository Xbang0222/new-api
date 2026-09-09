package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionRewardWebhook(t *testing.T, provider string) (*gin.Engine, model.SubscriptionOrder) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "subscription.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.TopUp{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}))
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldQuota, oldRedis, oldMemory := common.QuotaPerUnit, common.RedisEnabled, common.MemoryCacheEnabled
	oldType, oldValue, oldRate := common.InviterRewardType, common.InviterRewardValue, operation_setting.USDExchangeRate
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.QuotaPerUnit, common.RedisEnabled, common.MemoryCacheEnabled = 500000, false, false
	common.InviterRewardType, common.InviterRewardValue, operation_setting.USDExchangeRate = "percentage", 10, 7.3
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.QuotaPerUnit, common.RedisEnabled, common.MemoryCacheEnabled = oldQuota, oldRedis, oldMemory
		common.InviterRewardType, common.InviterRewardValue, operation_setting.USDExchangeRate = oldType, oldValue, oldRate
	})
	confirmPaymentComplianceForTest(t)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "subscription-inviter", AffCode: "sub-inviter"}).Error)
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "subscription-buyer", AffCode: "sub-buyer", InviterId: 2}).Error)
	plan := model.SubscriptionPlan{Title: "Webhook monthly", PriceAmount: 999, Currency: "USD", DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, Enabled: true, TotalAmount: 900000000}
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	order := model.SubscriptionOrder{UserId: 1, PlanId: plan.Id, Money: 999, TradeNo: "WAFFO_PANCAKE_SUB-reward-local", PaymentMethod: provider, PaymentProvider: provider, Status: common.TopUpStatusPending}
	require.NoError(t, db.Create(&order).Error)
	return gin.New(), order
}

func assertSubscriptionWebhookReward(t *testing.T, order model.SubscriptionOrder, paidMinor int64, currency string, reward int) {
	t.Helper()
	require.NoError(t, model.DB.First(&order, order.Id).Error)
	assert.Equal(t, common.TopUpStatusSuccess, order.Status)
	var receipt model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", order.TradeNo).First(&receipt).Error)
	assert.Equal(t, paidMinor, receipt.PaidAmountMinor)
	assert.Equal(t, currency, receipt.PaidCurrency)
	assert.Zero(t, receipt.Amount, "subscription payment does not represent wallet credit")
	assert.True(t, receipt.InviterRewardSent)
	assert.Equal(t, reward, receipt.InviterRewardQuota)
	assert.Equal(t, reward*10, receipt.InviterRewardBaseQuota)
	var inviter, buyer model.User
	require.NoError(t, model.DB.First(&inviter, 2).Error)
	require.NoError(t, model.DB.First(&buyer, 1).Error)
	assert.Equal(t, reward, inviter.AffQuota)
	assert.Equal(t, reward, inviter.AffHistoryQuota)
	assert.Zero(t, buyer.Quota, "buying a subscription must never credit the buyer wallet")
	var count int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 1).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestSubscriptionVerifiedPaymentWebhookRewards(t *testing.T) {
	for _, test := range []struct{ provider, eventType string }{
		{model.PaymentProviderStripe, "checkout.session.completed"},
		{model.PaymentProviderStripe, "checkout.session.async_payment_succeeded"},
		{model.PaymentProviderCreem, "checkout.completed"},
	} {
		t.Run(test.provider+"/"+test.eventType, func(t *testing.T) {
			provider := test.provider
			router, order := setupSubscriptionRewardWebhook(t, provider)
			stripeSecret, stripeWebhook, stripePrice := setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId
			creemKey, creemProducts, creemWebhook := setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret
			t.Cleanup(func() {
				setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId = stripeSecret, stripeWebhook, stripePrice
				setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret = creemKey, creemProducts, creemWebhook
			})
			setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId = "sk_test_local", "whsec_local", "price_local"
			setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret = "creem_local", `[{"productId":"prod_local"}]`, "creem_secret_local"
			if provider == model.PaymentProviderStripe {
				router.POST("/payment", StripeWebhook)
			} else {
				router.POST("/payment", CreemWebhook)
			}

			response := topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 1900, "usd", false, test.eventType)
			assert.GreaterOrEqual(t, response.Code, http.StatusBadRequest)
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusPending, order.Status)
			var count int64
			require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&count).Error)
			assert.Zero(t, count, "an invalid signature cannot create a payment or reward record")

			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 0, "", true, test.eventType)
			assert.Equal(t, http.StatusInternalServerError, response.Code, "missing payment evidence must be retryable, not silently acknowledged")
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusPending, order.Status)
			require.NoError(t, model.DB.Model(&model.UserSubscription{}).Count(&count).Error)
			assert.Zero(t, count, "the subscription and reward must roll back together")

			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 1900, "usd", true, test.eventType)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assertSubscriptionWebhookReward(t, order, 1900, "USD", 950000)
			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 9999, "usd", true, test.eventType)
			require.Equal(t, http.StatusInternalServerError, response.Code, "a conflicting signed receipt must not replace the original payment")
			assertSubscriptionWebhookReward(t, order, 1900, "USD", 950000)
			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 1900, "usd", true, test.eventType)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assertSubscriptionWebhookReward(t, order, 1900, "USD", 950000)
		})
	}
}

func TestSubscriptionEpayVerifiedRewardUsesCNYAndReplaysOnce(t *testing.T) {
	for _, firstPath := range []string{"notify", "return"} {
		t.Run(firstPath+"_first", func(t *testing.T) {
			router, order := setupSubscriptionRewardWebhook(t, model.PaymentProviderEpay)
			oldAddress, oldID, oldKey := operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey
			t.Cleanup(func() {
				operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = oldAddress, oldID, oldKey
			})
			operation_setting.PayAddress, operation_setting.EpayId, operation_setting.EpayKey = "https://pay.example.test", "local-id", "local-key"
			router.POST("/notify", SubscriptionEpayNotify)
			router.GET("/return", SubscriptionEpayReturn)
			params := epay.GenerateParams(map[string]string{
				"pid": "local-id", "out_trade_no": order.TradeNo, "trade_no": "epay-local", "type": "alipay", "money": "73.00", "trade_status": epay.StatusTradeSuccess,
			}, operation_setting.EpayKey)
			values := url.Values{}
			for key, value := range params {
				values.Set(key, value)
			}
			values.Set("money", "730.00")
			request := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(values.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, "fail", response.Body.String(), "tampering with the signed amount must not complete payment")
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusPending, order.Status)
			values.Set("money", "73.00")

			paths := []string{"notify", "return"}
			if firstPath == "return" {
				paths = []string{"return", "notify"}
			}
			for _, path := range paths {
				request = httptest.NewRequest(http.MethodGet, "/return?"+values.Encode(), nil)
				if path == "notify" {
					request = httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(values.Encode()))
					request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				}
				response = httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if path == "notify" {
					require.Equal(t, "success", response.Body.String())
				} else {
					require.Equal(t, http.StatusFound, response.Code)
					assert.Contains(t, response.Header().Get("Location"), "pay=success")
				}
				assertSubscriptionWebhookReward(t, order, 7300, "CNY", 500000)
			}
		})
	}
}

func TestSubscriptionPancakeVerifiedPaymentReward(t *testing.T) {
	router, order := setupSubscriptionRewardWebhook(t, model.PaymentProviderWaffoPancake)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	t.Setenv("WAFFO_WEBHOOK_TEST_PUBLIC_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey})))
	t.Setenv("WAFFO_WEBHOOK_PUBLIC_KEY", "")
	merchant, private, product, store := setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID = merchant, private, product, store
	})
	setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID = "merchant_local", "unused_local_key", "product_local", "STO_local"
	router.POST("/pancake/:env", WaffoPancakeWebhook)
	response := pancakeStoreWebhookRequest(t, router, key, order.TradeNo, "STO_other", "test")
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.NoError(t, model.DB.First(&order, order.Id).Error)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	for _, malformed := range []map[string]any{
		{"total": ""},
		{"total": "invalid"},
		{"total": nil, "taxAmount": "1.00"},
	} {
		response = pancakeStoreWebhookRequest(t, router, key, order.TradeNo, "STO_local", "test", malformed)
		require.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
		require.NoError(t, model.DB.First(&order, order.Id).Error)
		assert.Equal(t, common.TopUpStatusPending, order.Status, "ambiguous payment cannot complete the order or reward")
	}
	response = pancakeStoreWebhookRequest(t, router, key, order.TradeNo, "STO_local", "test")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assertSubscriptionWebhookReward(t, order, 1900, "USD", 950000)
	response = pancakeStoreWebhookRequest(t, router, key, order.TradeNo, "STO_local", "test")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assertSubscriptionWebhookReward(t, order, 1900, "USD", 950000)
}

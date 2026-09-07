package controller

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPancakeWebhookStoreBindingProtectsPaymentState(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey}))
	t.Setenv("WAFFO_WEBHOOK_TEST_PUBLIC_KEY", publicPEM)
	t.Setenv("WAFFO_WEBHOOK_PROD_PUBLIC_KEY", publicPEM)
	t.Setenv("WAFFO_WEBHOOK_PUBLIC_KEY", "")
	merchant, private, product, store := setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID = merchant, private, product, store
	})
	setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeProductID, setting.WaffoPancakeStoreID = "merchant_local", "unused_local_key", "product_local", "STO_local"
	for _, test := range []struct {
		name, eventStore, mode string
		subscription           bool
		wantStatus             int
		paid                   bool
	}{
		{name: "foreign wallet payment", eventStore: "STO_other", mode: "test", wantStatus: http.StatusUnauthorized},
		{name: "missing wallet store", mode: "test", wantStatus: http.StatusUnauthorized},
		{name: "foreign subscription payment", eventStore: "STO_other", mode: "test", subscription: true, wantStatus: http.StatusUnauthorized},
		{name: "missing subscription store", mode: "test", subscription: true, wantStatus: http.StatusUnauthorized},
		{name: "wrong environment", eventStore: "STO_local", mode: "prod", wantStatus: http.StatusOK},
		{name: "matching wallet payment", eventStore: "STO_local", mode: "test", wantStatus: http.StatusOK, paid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, _ := setupInvoiceController(t, 0)
			confirmPaymentComplianceForTest(t)
			router.POST("/pancake/:env", WaffoPancakeWebhook)
			tradeNo := "WAFFO_PANCAKE-local-order"
			var order model.TopUp
			var subscription model.SubscriptionOrder
			if test.subscription {
				require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionOrder{}))
				tradeNo = "WAFFO_PANCAKE_SUB-local-order"
				subscription = model.SubscriptionOrder{UserId: 1, PlanId: 1, TradeNo: tradeNo, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending}
				require.NoError(t, model.DB.Create(&subscription).Error)
			} else {
				order = model.TopUp{UserId: 1, Amount: 10, Money: 25, TradeNo: tradeNo, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending}
				require.NoError(t, model.DB.Create(&order).Error)
			}
			response := pancakeStoreWebhookRequest(t, router, key, tradeNo, test.eventStore, test.mode)
			require.Equal(t, test.wantStatus, response.Code, response.Body.String())
			var user model.User
			require.NoError(t, model.DB.First(&user, 1).Error)
			if test.subscription {
				require.NoError(t, model.DB.First(&subscription, subscription.Id).Error)
				assert.Equal(t, common.TopUpStatusPending, subscription.Status)
				assert.Empty(t, subscription.ProviderPayload)
				assert.Zero(t, user.Quota)
				return
			}
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			if !test.paid {
				assert.Equal(t, common.TopUpStatusPending, order.Status)
				assert.Zero(t, order.PaidAmountMinor)
				assert.Empty(t, order.PaidCurrency)
				assert.Zero(t, user.Quota)
				return
			}
			assert.Equal(t, common.TopUpStatusSuccess, order.Status)
			assert.EqualValues(t, 1900, order.PaidAmountMinor)
			assert.Equal(t, "USD", order.PaidCurrency)
			assert.Equal(t, 5000000, user.Quota)
			response = pancakeStoreWebhookRequest(t, router, key, tradeNo, test.eventStore, test.mode)
			require.Equal(t, http.StatusOK, response.Code)
			require.NoError(t, model.DB.First(&user, 1).Error)
			assert.Equal(t, 5000000, user.Quota, "a valid replay cannot credit twice")
		})
	}
}

func pancakeStoreWebhookRequest(t *testing.T, router *gin.Engine, key *rsa.PrivateKey, tradeNo, store, mode string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(map[string]any{
		"id": "event_local", "eventType": "order.completed", "storeId": store, "mode": mode,
		"data": map[string]any{"orderId": "ORD_local", "orderMerchantExternalId": tradeNo, "merchantProvidedBuyerIdentity": "new-api-user-1", "amount": "19.00", "total": "19.00", "taxAmount": "0.00", "currency": "USD"},
	})
	require.NoError(t, err)
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	digest := sha256.Sum256(append([]byte(timestamp+"."), body...))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/pancake/test", bytes.NewReader(body))
	request.Header.Set("X-Waffo-Signature", "t="+timestamp+",v1="+base64.StdEncoding.EncodeToString(signature))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestPancakeSubscriptionCheckoutRequiresStoreBeforeCreatingOrder(t *testing.T) {
	router, _ := setupInvoiceController(t, 0)
	confirmPaymentComplianceForTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPlan{}, &model.SubscriptionOrder{}))
	plan := model.SubscriptionPlan{Title: "Monthly", Enabled: true, WaffoPancakeProductId: "product_local"}
	require.NoError(t, model.DB.Create(&plan).Error)
	merchant, private, store := setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeStoreID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeStoreID = merchant, private, store
	})
	setting.WaffoPancakeMerchantID, setting.WaffoPancakePrivateKey, setting.WaffoPancakeStoreID = "merchant_local", "invalid_local_private_key", ""
	router.POST("/subscribe", SubscriptionRequestWaffoPancakePay)
	body, err := common.Marshal(SubscriptionWaffoPancakePayRequest{PlanId: plan.Id})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/subscribe", bytes.NewReader(body))
	request.Header.Set("Test-Actor", "1")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	assert.False(t, result.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.SubscriptionOrder{}).Count(&count).Error)
	assert.Zero(t, count, "missing callback ownership configuration must fail before an order is created")
}

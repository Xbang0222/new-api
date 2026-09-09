package controller

import (
	"bytes"
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
	"github.com/stripe/stripe-go/v81/webhook"
)

func TestTopUpVerifiedPaymentWebhookReceipts(t *testing.T) {
	for _, provider := range []string{model.PaymentProviderStripe, model.PaymentProviderCreem} {
		t.Run(provider, func(t *testing.T) {
			router, _ := setupInvoiceController(t, 0)
			require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionOrder{}))
			confirmPaymentComplianceForTest(t)
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

			order := model.TopUp{UserId: 1, Amount: 10, Money: 25, TradeNo: "receipt-verified", PaymentProvider: provider, Status: common.TopUpStatusPending}
			require.NoError(t, model.DB.Create(&order).Error)
			response := topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 1900, "eur", false)
			assert.GreaterOrEqual(t, response.Code, http.StatusBadRequest)
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusPending, order.Status)
			assert.Zero(t, order.PaidAmountMinor)
			var user model.User
			require.NoError(t, model.DB.First(&user, 1).Error)
			assert.Zero(t, user.Quota, "an unsigned request cannot add quota or a payment snapshot")

			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 1900, "eur", true)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.Equal(t, common.TopUpStatusSuccess, order.Status)
			assert.EqualValues(t, 1900, order.PaidAmountMinor)
			assert.Equal(t, "EUR", order.PaidCurrency)
			assert.Equal(t, float64(25), order.Money, "credit calculation and actual payment are independent")
			require.NoError(t, model.DB.First(&user, 1).Error)
			creditedQuota := 10
			if provider == model.PaymentProviderStripe {
				creditedQuota = 12500000
			}
			assert.Equal(t, creditedQuota, user.Quota)

			response = topUpReceiptWebhookRequest(t, router, provider, order.TradeNo, 9999, "usd", true)
			require.Equal(t, http.StatusOK, response.Code)
			require.NoError(t, model.DB.First(&order, order.Id).Error)
			assert.EqualValues(t, 1900, order.PaidAmountMinor, "a successful replay cannot replace the first receipt")
			assert.Equal(t, "EUR", order.PaidCurrency)
			require.NoError(t, model.DB.First(&user, 1).Error)
			assert.Equal(t, creditedQuota, user.Quota)

			missing := model.TopUp{UserId: 1, Amount: 10, Money: 25, TradeNo: "receipt-missing", PaymentProvider: provider, Status: common.TopUpStatusPending}
			require.NoError(t, model.DB.Create(&missing).Error)
			response = topUpReceiptWebhookRequest(t, router, provider, missing.TradeNo, 0, "", true)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.NoError(t, model.DB.First(&missing, missing.Id).Error)
			assert.Equal(t, common.TopUpStatusSuccess, missing.Status, "missing receipt limits invoicing but must not stop valid recharge")
			assert.Zero(t, missing.PaidAmountMinor)
			assert.Empty(t, missing.PaidCurrency)
			require.NoError(t, model.DB.First(&user, 1).Error)
			assert.Equal(t, creditedQuota*2, user.Quota)
		})
	}
}

// topUpReceiptWebhookRequest exercises each provider's real signature boundary;
// these locally signed fixtures never call a payment service.
func topUpReceiptWebhookRequest(t *testing.T, router *gin.Engine, provider, tradeNo string, amountMinor int64, currency string, validSignature bool, eventTypes ...string) *httptest.ResponseRecorder {
	t.Helper()
	var event any
	if provider == model.PaymentProviderStripe {
		object := map[string]any{"id": "cs_local", "object": "checkout.session", "client_reference_id": tradeNo, "status": "complete", "payment_status": "paid", "customer": "cus_local"}
		if currency != "" {
			object["amount_total"], object["currency"] = amountMinor, currency
		}
		eventType := "checkout.session.completed"
		if len(eventTypes) > 0 {
			eventType = eventTypes[0]
		}
		event = map[string]any{"id": "evt_local", "object": "event", "type": eventType, "data": map[string]any{"object": object}}
	} else {
		event = map[string]any{"id": "evt_local", "eventType": "checkout.completed", "object": map[string]any{
			"request_id": tradeNo, "order": map[string]any{"id": "ord_local", "status": "paid", "type": "onetime", "amount_paid": amountMinor, "currency": currency},
			"customer": map[string]any{"email": "payer@example.test", "name": "Local payer"},
		}}
	}
	body, err := common.Marshal(event)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/payment", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if provider == model.PaymentProviderStripe {
		secret := setting.StripeWebhookSecret
		if !validSignature {
			secret = "untrusted_secret"
		}
		timestamp := time.Now()
		request.Header.Set("Stripe-Signature", fmt.Sprintf("t=%d,v1=%x", timestamp.Unix(), webhook.ComputeSignature(timestamp, body, secret)))
	} else {
		secret := setting.CreemWebhookSecret
		if !validSignature {
			secret = "untrusted_secret"
		}
		request.Header.Set(CreemSignatureHeader, generateCreemSignature(string(body), secret))
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

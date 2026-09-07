package service

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Locally signed events exercise the real SDK verifier without contacting a
// payment provider. The SDK public-key override is limited to this test.
func TestWaffoPancakeWebhookRequiresConfiguredStore(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
	t.Setenv("WAFFO_WEBHOOK_PROD_PUBLIC_KEY", pubPEM)
	t.Setenv("WAFFO_WEBHOOK_TEST_PUBLIC_KEY", pubPEM)
	t.Setenv("WAFFO_WEBHOOK_PUBLIC_KEY", "")

	original := setting.WaffoPancakeStoreID
	t.Cleanup(func() { setting.WaffoPancakeStoreID = original })
	for _, tc := range []struct {
		name, storeField, config, mode, expectedStore string
		tamper, wantErr                               bool
	}{
		{name: "matching_store_prod", storeField: `"storeId":"STO_local",`, config: "STO_local", mode: "prod", expectedStore: "STO_local"},
		{name: "foreign_store_prod_rejected", storeField: `"storeId":"STO_foreign",`, config: "STO_local", mode: "prod", expectedStore: "STO_foreign", wantErr: true},
		{name: "matching_store_test", storeField: `"storeId":"STO_local",`, config: "STO_local", mode: "test", expectedStore: "STO_local"},
		{name: "foreign_store_test_rejected", storeField: `"storeId":"STO_foreign",`, config: "STO_local", mode: "test", expectedStore: "STO_foreign", wantErr: true},
		{name: "missing_event_store_rejected", config: "STO_local", mode: "prod", wantErr: true},
		{name: "empty_event_store_rejected", storeField: `"storeId":"",`, config: "STO_local", mode: "prod", wantErr: true},
		{name: "missing_config_rejected", storeField: `"storeId":"STO_foreign",`, mode: "prod", expectedStore: "STO_foreign", wantErr: true},
		{name: "numeric_json_store_rejected_by_sdk", storeField: `"storeId":123,`, config: "123", mode: "prod", wantErr: true},
		{name: "numeric_string_store_preserved", storeField: `"storeId":"123",`, config: "123", mode: "prod", expectedStore: "123"},
		{name: "padded_event_store_rejected", storeField: `"storeId":" STO_local ",`, config: "STO_local", mode: "prod", wantErr: true},
		{name: "case_changed_store_rejected", storeField: `"storeId":"sto_local",`, config: "STO_local", mode: "prod", wantErr: true},
		{name: "trimmed_config", storeField: `"storeId":"STO_local",`, config: " STO_local ", mode: "prod", expectedStore: "STO_local"},
		{name: "bad_signature_rejected", storeField: `"storeId":"STO_local",`, config: "STO_local", mode: "prod", tamper: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setting.WaffoPancakeStoreID = tc.config
			payload := fmt.Sprintf(`{"id":"evt_local_validation","eventType":"order.completed",%s"mode":"%s","data":{"orderId":"ORD_local_validation","orderMerchantExternalId":"WAFFO_PANCAKE-7-known-order","merchantProvidedBuyerIdentity":"new-api-user-7","amount":"1.00","taxAmount":"0.00","total":"1.00","currency":"USD"}}`, tc.storeField, tc.mode)
			timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
			digest := sha256.Sum256([]byte(timestamp + "." + payload))
			signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
			require.NoError(t, err)
			header := "t=" + timestamp + ",v1=" + base64.StdEncoding.EncodeToString(signature)
			if tc.tamper {
				payload += " "
			}
			event, err := VerifyConfiguredWaffoPancakeWebhook(payload, header)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, event)
				t.Logf("rejected: %v", err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, event)
			assert.Equal(t, tc.expectedStore, event.StoreID)
			assert.Equal(t, "new-api-user-7", event.Data.MerchantProvidedBuyerIdentity)
			assert.Equal(t, "WAFFO_PANCAKE-7-known-order", event.Data.OrderMerchantExternalID)
			t.Logf("accepted configured_store=%q event_store=%q mode=%q", tc.config, event.StoreID, event.Mode)
		})
	}
}

package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopUpPaymentReceiptUsesExactCurrencyMinorUnits(t *testing.T) {
	for _, test := range []struct {
		amount, currency string
		minor            int64
		invalid          bool
	}{
		{"12.34", "usd", 1234, false},
		{"50.00", "CNY", 5000, false},
		{"4500", "JPY", 4500, false},
		{"12.345", "KWD", 12345, false},
		{"12.345", "USD", 0, true},
		{"1.1", "JPY", 0, true},
		{"-1", "USD", 0, true},
		{"0", "USD", 0, true},
		{"NaN", "USD", 0, true},
		{"1e100000000", "USD", 0, true},
		{"999999999999999999999", "USD", 0, true},
		{"1", "XXX", 0, true},
		{"1", "CUSTOM", 0, true},
	} {
		t.Run(test.amount+"_"+test.currency, func(t *testing.T) {
			receipt, err := ParseTopUpPaymentReceipt(test.amount, test.currency)
			if test.invalid {
				require.Error(t, err)
				assert.Zero(t, receipt.AmountMinor)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.minor, receipt.AmountMinor)
		})
	}
}

func TestTopUpPaymentReceiptsCommitWithCreditAndRemainImmutable(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	withCommerceDatabases(t, func(t *testing.T) {
		for _, provider := range []string{PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffo, PaymentProviderWaffoPancake} {
			t.Run(provider, func(t *testing.T) {
				user := User{Username: "receipt-" + provider, AffCode: provider, Group: "default"}
				require.NoError(t, DB.Create(&user).Error)
				topup := TopUp{UserId: user.Id, Amount: 10, Money: 25, TradeNo: "receipt-" + provider, PaymentMethod: provider, PaymentProvider: provider, Status: common.TopUpStatusPending}
				require.NoError(t, DB.Create(&topup).Error)
				receipt := TopUpPaymentReceipt{AmountMinor: 1900, Currency: "EUR"}
				require.NoError(t, completeTopUpReceiptForTest(provider, topup.TradeNo, receipt))
				var stored TopUp
				require.NoError(t, DB.First(&stored, topup.Id).Error)
				assert.Equal(t, common.TopUpStatusSuccess, stored.Status)
				assert.Equal(t, int64(1900), stored.PaidAmountMinor)
				assert.Equal(t, "EUR", stored.PaidCurrency)
				assert.Equal(t, topup.Money, stored.Money, "paid amount must not overwrite nominal credit money")
				assert.Equal(t, topup.Amount, stored.Amount)
				var credited User
				require.NoError(t, DB.First(&credited, user.Id).Error)
				expectedQuota := 5000000
				if provider == PaymentProviderStripe {
					expectedQuota = 12500000
				}
				if provider == PaymentProviderCreem {
					expectedQuota = 10
				}
				assert.Equal(t, expectedQuota, credited.Quota)
				_ = completeTopUpReceiptForTest(provider, topup.TradeNo, TopUpPaymentReceipt{AmountMinor: 9999, Currency: "USD"})
				var replayed TopUp
				require.NoError(t, DB.First(&replayed, topup.Id).Error)
				assert.Equal(t, stored, replayed, "replayed callbacks must not replace paid snapshots")
				require.NoError(t, DB.First(&credited, user.Id).Error)
				assert.Equal(t, expectedQuota, credited.Quota)

				invalid := TopUp{UserId: user.Id, Amount: 10, Money: 25, TradeNo: "invalid-" + provider, PaymentMethod: provider, PaymentProvider: provider, Status: common.TopUpStatusPending}
				require.NoError(t, DB.Create(&invalid).Error)
				require.Error(t, completeTopUpReceiptForTest(provider, invalid.TradeNo, TopUpPaymentReceipt{AmountMinor: -1, Currency: "USD"}))
				var rejected TopUp
				require.NoError(t, DB.First(&rejected, invalid.Id).Error)
				assert.Equal(t, common.TopUpStatusPending, rejected.Status)
				assert.Zero(t, rejected.PaidAmountMinor)
				assert.Empty(t, rejected.PaidCurrency)
				require.NoError(t, DB.First(&credited, user.Id).Error)
				assert.Equal(t, expectedQuota, credited.Quota)

				require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", common.MaxWalletQuota).Error)
				require.Error(t, completeTopUpReceiptForTest(provider, invalid.TradeNo, receipt))
				require.NoError(t, DB.First(&rejected, invalid.Id).Error)
				assert.Equal(t, common.TopUpStatusPending, rejected.Status)
				assert.Zero(t, rejected.PaidAmountMinor, "failed wallet credit must roll back receipt and payment state")
				assert.Empty(t, rejected.PaidCurrency)
			})
		}
	})
}

func completeTopUpReceiptForTest(provider, tradeNo string, receipt TopUpPaymentReceipt) error {
	switch provider {
	case PaymentProviderStripe:
		return Recharge(tradeNo, "", "", receipt)
	case PaymentProviderCreem:
		return RechargeCreem(tradeNo, "", "", "", receipt)
	case PaymentProviderWaffo:
		return RechargeWaffo(tradeNo, "", receipt)
	default:
		return RechargeWaffoPancake(tradeNo, receipt)
	}
}

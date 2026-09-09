package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var ErrSubscriptionPaymentReceiptRequired = errors.New("subscription inviter reward requires a verified USD or CNY payment receipt")

func lockSubscriptionRewardUsersTx(tx *gorm.DB, buyerId, inviterId int) (bool, error) {
	inviterExists := false
	ids := []int{buyerId}
	if inviterId > 0 && inviterId != buyerId {
		if inviterId < buyerId {
			ids = []int{inviterId, buyerId}
		} else {
			ids = append(ids, inviterId)
		}
	}
	for _, id := range ids {
		var user User
		err := lockForUpdate(tx).Select("id", "inviter_id").First(&user, id).Error
		if id != buyerId && errors.Is(err, gorm.ErrRecordNotFound) {
			continue // Deleted inviters do not prevent the buyer from subscribing.
		}
		if err != nil {
			return false, err
		}
		if id == buyerId && user.InviterId != inviterId {
			return false, errors.New("subscription inviter changed during completion; retry")
		}
		if id == inviterId && id != buyerId {
			inviterExists = true
		}
	}
	return inviterExists, nil
}

// A subscription grants consumable quota on its own schedule. Its rebate basis
// is the verified external payment, never the plan's quota or today's price.
func subscriptionPaymentRewardQuota(topUp *TopUp) (int, error) {
	if topUp.PaidAmountMinor <= 0 || topUp.PaidAmountMinor > int64(common.MaxWalletQuota) ||
		common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return 0, ErrSubscriptionPaymentReceiptRequired
	}
	rate := 1.0
	switch topUp.PaidCurrency {
	case "USD":
	case "CNY":
		rate = operation_setting.USDExchangeRate
	default:
		return 0, ErrSubscriptionPaymentReceiptRequired
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0, errors.New("invalid subscription reward exchange rate")
	}
	// QuoRem avoids precision-dependent rounding upward before truncation.
	quota, _ := decimal.NewFromInt(topUp.PaidAmountMinor).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		QuoRem(decimal.NewFromFloat(rate).Mul(decimal.NewFromInt(100)), 0)
	return common.WalletQuotaFromDecimalStrict(quota)
}

func processSubscriptionInviterRewardTx(tx *gorm.DB, order *SubscriptionOrder, topUp *TopUp, inviterId int) (int, string, error) {
	if topUp.InviterRewardSent || !operation_setting.IsPaymentComplianceConfirmed() ||
		common.InviterRewardType == "" || common.InviterRewardValue <= 0 ||
		inviterId <= 0 || inviterId == order.UserId {
		return 0, "", nil
	}
	if _, err := NewTopUpPaymentReceipt(topUp.PaidAmountMinor, topUp.PaidCurrency); err != nil {
		return 0, "", fmt.Errorf("subscription %s: %w", order.TradeNo, ErrSubscriptionPaymentReceiptRequired)
	}
	quota, conversionErr := subscriptionPaymentRewardQuota(topUp)
	if conversionErr != nil {
		if common.InviterRewardType != "fixed" {
			return 0, "", fmt.Errorf("subscription %s: %w", order.TradeNo, conversionErr)
		}
		// A fixed rebate needs a positive verified payment, not an FX rate.
		// Zero records an unavailable basis; the credited reward stays exact.
		quota = 0
	}
	creditedInviter, message, err := processInviterRewardTx(tx, order.UserId, quota, topUp.Id)
	if err != nil {
		return 0, "", err
	}
	message = strings.ReplaceAll(message, "充值", "订阅购买")
	if creditedInviter > 0 {
		message += fmt.Sprintf("，订单号: %s", order.TradeNo)
	}
	return creditedInviter, message, nil
}

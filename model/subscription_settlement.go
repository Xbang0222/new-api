package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrSubscriptionQuotaExceeded = errors.New("subscription used exceeds total")

// SettleSubscriptionQuota applies a settlement delta to its original subscription.
// Only the positive excess may use the wallet, when both the request preference
// and the subscription snapshot allow it. Both debits commit together.
func SettleSubscriptionQuota(subscriptionId, userId int, delta int64, allowWallet bool) (int64, int64, error) {
	if subscriptionId <= 0 || userId <= 0 || delta > int64(common.MaxWalletQuota) || delta < -int64(common.MaxWalletQuota) {
		return 0, 0, errors.New("invalid subscription settlement")
	}
	if delta == 0 {
		return 0, 0, nil
	}
	var subscriptionDelta, walletDelta int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
			return err
		}
		if sub.AmountUsed < 0 || delta > 0 && sub.AmountUsed > math.MaxInt64-delta {
			return errors.New("invalid subscription usage for settlement")
		}
		subscriptionDelta = delta
		if delta > 0 && sub.AmountTotal > 0 {
			remaining := max(sub.AmountTotal-sub.AmountUsed, 0)
			if delta > remaining {
				if !allowWallet || !sub.AllowWalletOverflow {
					return fmt.Errorf("%w, used=%d total=%d", ErrSubscriptionQuotaExceeded, sub.AmountUsed+delta, sub.AmountTotal)
				}
				subscriptionDelta = remaining
				walletDelta = delta - remaining
			}
		}
		if subscriptionDelta != 0 {
			if err := tx.Model(&sub).Update("amount_used", max(sub.AmountUsed+subscriptionDelta, 0)).Error; err != nil {
				return err
			}
		}
		if walletDelta == 0 {
			return nil
		}
		// Usage already happened. Match wallet settlement's arrears semantics,
		// but bound the balance and never queue one side of this transaction.
		result := tx.Model(&User{}).
			Where("id = ? AND quota >= ?", userId, -int64(common.MaxWalletQuota)+walletDelta).
			Update("quota", gorm.Expr("quota - ?", walletDelta))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("wallet missing or settlement exceeds wallet limit")
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	if walletDelta > 0 {
		if err := cacheDecrUserQuota(userId, walletDelta); err != nil {
			// The transaction committed. Returning an error here could retry a debit.
			common.SysError(fmt.Sprintf("failed to sync subscription settlement wallet cache for user %d: %s", userId, err))
		}
	}
	return subscriptionDelta, walletDelta, nil
}

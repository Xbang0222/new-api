package model

import (
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// TransferSubscriptionReservationToWallet moves an entire in-flight reservation
// before an upstream attempt. Releasing the subscription and reserving the wallet
// are atomic; unlike final settlement, a reservation must not create wallet debt.
func TransferSubscriptionReservationToWallet(requestId string, userId, subscriptionId, targetQuota, extraReserved int) error {
	if requestId == "" || userId <= 0 || subscriptionId <= 0 || targetQuota <= 0 || targetQuota > common.MaxWalletQuota || extraReserved < 0 || extraReserved > targetQuota {
		return errors.New("invalid subscription reservation transfer")
	}
	// Reuse the guarded live-balance reservation also used by invoice transactions.
	cacheReserved, err := ReserveInvoiceFeeCache(userId, targetQuota)
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ? AND user_id = ? AND user_subscription_id = ?", requestId, userId, subscriptionId).First(&record).Error; err != nil {
			return err
		}
		if record.Status != "consumed" || record.PreConsumed <= 0 || record.PreConsumed > int64(targetQuota-extraReserved) {
			return errors.New("subscription reservation is not transferable")
		}
		var sub UserSubscription
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
			return err
		}
		if !sub.AllowWalletOverflow {
			return errors.New("subscription wallet overflow is disabled")
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, targetQuota).Update("quota", gorm.Expr("quota - ?", targetQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrWalletQuotaInsufficient
		}
		if err := postConsumeUserSubscriptionDeltaTx(tx, subscriptionId, -record.PreConsumed-int64(extraReserved)); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
	if err != nil {
		if cacheReserved {
			if restoreErr := SyncUserQuotaCacheDelta(userId, targetQuota); restoreErr != nil {
				common.SysError(fmt.Sprintf("failed to restore wallet reservation cache for user %d: %s", userId, restoreErr))
			}
		}
		return err
	}
	if !cacheReserved {
		if cacheErr := cacheDecrUserQuota(userId, int64(targetQuota)); cacheErr != nil {
			common.SysError(fmt.Sprintf("failed to sync wallet reservation transfer cache for user %d: %s", userId, cacheErr))
		}
	}
	return nil
}

package model

import (
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// SortSubscriptionsForDebit puts saved preferences first and otherwise uses
// earliest expiry, then ID. Stale IDs have no effect on eligibility.
func SortSubscriptionsForDebit(subs []UserSubscription, order []int) {
	ranks := make(map[int]int, len(order))
	for i, id := range order {
		ranks[id] = i + 1
	}
	sort.SliceStable(subs, func(i, j int) bool {
		a, b := ranks[subs[i].Id], ranks[subs[j].Id]
		if a != b {
			if a == 0 {
				return false
			}
			if b == 0 {
				return true
			}
			return a < b
		}
		if subs[i].EndTime != subs[j].EndTime {
			return subs[i].EndTime < subs[j].EndTime
		}
		return subs[i].Id < subs[j].Id
	})
}

func getSubscriptionDebitOrder(tx *gorm.DB, userId int) ([]int, error) {
	var user User
	if err := tx.Select("setting").Where("id = ?", userId).Find(&user).Error; err != nil {
		return nil, err
	}
	return user.GetSetting().SubscriptionOrder, nil
}

// UpdateSubscriptionDebitOrder only accepts the authenticated user's active
// subscriptions. An empty order restores earliest-expiry-first billing.
func UpdateSubscriptionDebitOrder(userId int, ids []int) error {
	if userId <= 0 || len(ids) > 1000 {
		return errors.New("invalid subscription order")
	}
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return errors.New("invalid subscription order")
		}
		seen[id] = true
	}
	now := GetDBTimestamp()
	var settingValue string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id", "setting").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if len(ids) > 0 {
			var count int64
			if err := tx.Model(&UserSubscription{}).Where("user_id = ? AND status = ? AND end_time > ? AND id IN ?", userId, "active", now, ids).Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(ids)) {
				return errors.New("subscription order contains unavailable subscriptions; refresh and try again")
			}
		}
		settings := user.GetSetting()
		settings.SubscriptionOrder = ids
		encoded, err := common.Marshal(settings)
		if err != nil {
			return err
		}
		settingValue = string(encoded)
		return tx.Model(&User{}).Where("id = ?", userId).Update("setting", settingValue).Error
	})
	if err != nil {
		return err
	}
	if err := updateUserSettingCache(userId, settingValue); err != nil {
		common.SysError("subscription order saved but user cache refresh failed: " + err.Error())
	}
	return nil
}

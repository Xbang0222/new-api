package model

import (
	"errors"

	"gorm.io/gorm"
)

// custom: subscription deduction order
func orderUserSubscriptionsForDeduction(query *gorm.DB) *gorm.DB {
	return query.
		Order("CASE WHEN COALESCE(deduction_order, 0) > 0 THEN 0 ELSE 1 END ASC").
		Order("CASE WHEN COALESCE(deduction_order, 0) > 0 THEN COALESCE(deduction_order, 0) ELSE 0 END ASC").
		Order("end_time ASC").
		Order("id ASC")
}

// custom: subscription deduction order
func UpdateUserSubscriptionDeductionOrder(userId int, subscriptionIds []int) error {
	if userId <= 0 {
		return errors.New("invalid userId")
	}
	seen := make(map[int]struct{}, len(subscriptionIds))
	for _, id := range subscriptionIds {
		if id <= 0 {
			return errors.New("invalid subscription id")
		}
		if _, exists := seen[id]; exists {
			return errors.New("订阅排序包含重复订阅")
		}
		seen[id] = struct{}{}
	}

	now := GetDBTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var activeSubs []UserSubscription
		query := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now)
		if err := orderUserSubscriptionsForDeduction(query).Find(&activeSubs).Error; err != nil {
			return err
		}
		if len(activeSubs) != len(subscriptionIds) {
			return errors.New("订阅列表已变化，请刷新后重试")
		}

		activeSet := make(map[int]struct{}, len(activeSubs))
		for _, sub := range activeSubs {
			activeSet[sub.Id] = struct{}{}
		}
		for _, id := range subscriptionIds {
			if _, ok := activeSet[id]; !ok {
				return errors.New("订阅列表已变化，请刷新后重试")
			}
		}
		for index, id := range subscriptionIds {
			res := tx.Model(&UserSubscription{}).
				Where("id = ? AND user_id = ? AND status = ? AND end_time > ?", id, userId, "active", now).
				Update("deduction_order", index+1)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return errors.New("订阅列表已变化，请刷新后重试")
			}
		}
		return nil
	})
}

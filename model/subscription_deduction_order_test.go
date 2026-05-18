package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertPlanForDeductionTest(t *testing.T, id int, sortOrder int) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:               id,
		Title:            "Deduction Plan",
		PriceAmount:      1.00,
		Currency:         "USD",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		SortOrder:        sortOrder,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func insertUserSubscriptionForDeductionTest(
	t *testing.T,
	userId int,
	planId int,
	endTime int64,
	total int64,
	used int64,
	deductionOrder int,
	status string,
) *UserSubscription {
	t.Helper()
	now := GetDBTimestamp()
	sub := &UserSubscription{
		UserId:         userId,
		PlanId:         planId,
		AmountTotal:    total,
		AmountUsed:     used,
		StartTime:      now - 3600,
		EndTime:        endTime,
		Status:         status,
		Source:         "order",
		DeductionOrder: deductionOrder,
	}
	require.NoError(t, DB.Create(sub).Error)
	return sub
}

func subscriptionAmountUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func subscriptionDeductionOrder(t *testing.T, id int) int {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", id).First(&sub).Error)
	return sub.DeductionOrder
}

func TestPreConsumeUserSubscriptionUsesCustomDeductionOrder(t *testing.T) {
	truncateTables(t)

	userId := 7201
	now := GetDBTimestamp()
	planA := insertPlanForDeductionTest(t, 9201, 0)
	planB := insertPlanForDeductionTest(t, 9202, 999)

	laterSub := insertUserSubscriptionForDeductionTest(t, userId, planA.Id, now+7200, 1000, 0, 1, "active")
	earlierSub := insertUserSubscriptionForDeductionTest(t, userId, planB.Id, now+3600, 1000, 0, 2, "active")

	result, err := PreConsumeUserSubscription("deduction-custom-order", userId, "gpt-test", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, laterSub.Id, result.UserSubscriptionId)
	assert.Equal(t, int64(100), subscriptionAmountUsed(t, laterSub.Id))
	assert.Equal(t, int64(0), subscriptionAmountUsed(t, earlierSub.Id))
}

func TestPreConsumeUserSubscriptionDefaultsToExpiryOrder(t *testing.T) {
	truncateTables(t)

	userId := 7202
	now := GetDBTimestamp()
	highSortPlan := insertPlanForDeductionTest(t, 9203, 999)
	lowSortPlan := insertPlanForDeductionTest(t, 9204, 0)

	laterSub := insertUserSubscriptionForDeductionTest(t, userId, highSortPlan.Id, now+7200, 1000, 0, 0, "active")
	earlierSub := insertUserSubscriptionForDeductionTest(t, userId, lowSortPlan.Id, now+3600, 1000, 0, 0, "active")

	result, err := PreConsumeUserSubscription("deduction-default-order", userId, "gpt-test", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, earlierSub.Id, result.UserSubscriptionId)
	assert.Equal(t, int64(0), subscriptionAmountUsed(t, laterSub.Id))
	assert.Equal(t, int64(100), subscriptionAmountUsed(t, earlierSub.Id))
}

func TestPreConsumeUserSubscriptionSkipsInsufficientPreferredSubscription(t *testing.T) {
	truncateTables(t)

	userId := 7203
	now := GetDBTimestamp()
	planA := insertPlanForDeductionTest(t, 9205, 0)
	planB := insertPlanForDeductionTest(t, 9206, 0)

	insufficientSub := insertUserSubscriptionForDeductionTest(t, userId, planA.Id, now+3600, 100, 50, 1, "active")
	nextSub := insertUserSubscriptionForDeductionTest(t, userId, planB.Id, now+7200, 1000, 0, 2, "active")

	result, err := PreConsumeUserSubscription("deduction-skip-insufficient", userId, "gpt-test", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, nextSub.Id, result.UserSubscriptionId)
	assert.Equal(t, int64(50), subscriptionAmountUsed(t, insufficientSub.Id))
	assert.Equal(t, int64(100), subscriptionAmountUsed(t, nextSub.Id))
}

func TestUpdateUserSubscriptionDeductionOrder(t *testing.T) {
	truncateTables(t)

	userId := 7204
	now := GetDBTimestamp()
	plan := insertPlanForDeductionTest(t, 9207, 0)
	first := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+3600, 1000, 0, 0, "active")
	second := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+7200, 1000, 0, 0, "active")
	third := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+10800, 1000, 0, 0, "active")

	err := UpdateUserSubscriptionDeductionOrder(userId, []int{third.Id, first.Id, second.Id})
	require.NoError(t, err)
	assert.Equal(t, 1, subscriptionDeductionOrder(t, third.Id))
	assert.Equal(t, 2, subscriptionDeductionOrder(t, first.Id))
	assert.Equal(t, 3, subscriptionDeductionOrder(t, second.Id))
}

func TestUpdateUserSubscriptionDeductionOrderRejectsInvalidLists(t *testing.T) {
	truncateTables(t)

	userId := 7205
	now := GetDBTimestamp()
	plan := insertPlanForDeductionTest(t, 9208, 0)
	first := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+3600, 1000, 0, 0, "active")
	second := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+7200, 1000, 0, 0, "active")
	cancelled := insertUserSubscriptionForDeductionTest(t, userId, plan.Id, now+10800, 1000, 0, 0, "cancelled")
	otherUser := insertUserSubscriptionForDeductionTest(t, userId+1, plan.Id, now+10800, 1000, 0, 0, "active")

	require.Error(t, UpdateUserSubscriptionDeductionOrder(userId, []int{first.Id, first.Id}))
	require.Error(t, UpdateUserSubscriptionDeductionOrder(userId, []int{first.Id}))
	require.Error(t, UpdateUserSubscriptionDeductionOrder(userId, []int{first.Id, second.Id, cancelled.Id}))
	require.Error(t, UpdateUserSubscriptionDeductionOrder(userId, []int{first.Id, second.Id, otherUser.Id}))
}

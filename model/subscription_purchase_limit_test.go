package model

import (
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPurchaseLimitAppliesOnlyToCurrentActiveCycle(t *testing.T) {
	truncateTables(t)
	user := insertUserForPaymentGuardTest(t, 801, 100000000)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 802)
	plan.MaxPurchasePerUser = 1
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	require.NoError(t, DB.Create(&UserSubscription{
		UserId: user.Id, PlanId: plan.Id, Status: "expired",
		StartTime: time.Now().Add(-2 * time.Hour).Unix(), EndTime: time.Now().Add(-time.Hour).Unix(),
	}).Error)

	require.NoError(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id))
	var active int64
	require.NoError(t, DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ? AND status = ?", user.Id, plan.Id, "active").Count(&active).Error)
	assert.EqualValues(t, 1, active)
	require.ErrorIs(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id), ErrSubscriptionPurchaseLimit)
}

func TestSubscriptionPendingOrderReservesPurchaseSlotUntilTimeout(t *testing.T) {
	truncateTables(t)
	user := insertUserForPaymentGuardTest(t, 811, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 812)
	plan.MaxPurchasePerUser = 1
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	first := &SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: plan.PriceAmount,
		TradeNo: "reservation-current", PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe}
	require.NoError(t, ReserveSubscriptionOrder(first))
	second := &SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: plan.PriceAmount,
		TradeNo: "reservation-blocked", PaymentMethod: PaymentMethodCreem, PaymentProvider: PaymentProviderCreem}
	require.ErrorIs(t, ReserveSubscriptionOrder(second), ErrSubscriptionPurchaseLimit)

	require.NoError(t, DB.Model(first).Update("create_time", time.Now().Unix()-SubscriptionOrderHoldSeconds-1).Error)
	require.NoError(t, ReserveSubscriptionOrder(second))
}

func TestPaidSubscriptionConflictIsRetainedForAdminResolution(t *testing.T) {
	truncateTables(t)
	user := insertUserForPaymentGuardTest(t, 821, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 822)
	plan.MaxPurchasePerUser = 1
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	require.NoError(t, DB.Create(&UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active",
		StartTime: time.Now().Unix(), EndTime: time.Now().Add(time.Hour).Unix()}).Error)
	insertSubscriptionOrderForPaymentGuardTest(t, "paid-conflict", user.Id, plan.Id, PaymentProviderStripe)

	err := CompleteSubscriptionOrder("paid-conflict", `{"paid":true}`, PaymentProviderStripe, PaymentMethodStripe)
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)
	order := GetSubscriptionOrderByTradeNo("paid-conflict")
	require.NotNil(t, order)
	assert.Equal(t, SubscriptionOrderStatusConflict, order.Status)
	assert.Equal(t, ErrSubscriptionPurchaseLimit.Error(), order.ConflictReason)
	assert.Contains(t, order.ProviderPayload, "paid")
	require.ErrorIs(t, CompleteSubscriptionOrder("paid-conflict", `{"paid":true}`, PaymentProviderStripe, PaymentMethodStripe), ErrSubscriptionPurchaseLimit)
	assert.EqualValues(t, 1, countUserSubscriptionsForPaymentGuardTest(t, user.Id))

	require.NoError(t, ResolveSubscriptionConflict(order.Id, 1, "refunded"))
	require.ErrorIs(t, ResolveSubscriptionConflict(order.Id, 1, "refunded"), ErrSubscriptionOrderStatusInvalid)
}

func TestExpiredReservationLateCallbackCanFulfillUnlessAnotherPurchaseWon(t *testing.T) {
	truncateTables(t)
	user := insertUserForPaymentGuardTest(t, 831, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 832)
	plan.MaxPurchasePerUser = 1
	require.NoError(t, DB.Save(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	late := &SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: plan.PriceAmount, TradeNo: "late-paid",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusPending,
		CreateTime: time.Now().Unix() - SubscriptionOrderHoldSeconds - 1}
	require.NoError(t, late.Insert())
	require.NoError(t, CompleteSubscriptionOrder(late.TradeNo, `{"paid":true}`, PaymentProviderStripe, PaymentMethodStripe))
	require.NoError(t, CompleteSubscriptionOrder(late.TradeNo, `{"paid":true}`, PaymentProviderStripe, PaymentMethodStripe))
	assert.EqualValues(t, 1, countUserSubscriptionsForPaymentGuardTest(t, user.Id))
}

func TestSubscriptionBalancePriceRejectsNonFiniteValues(t *testing.T) {
	for _, price := range []float64{-1, math.Inf(1), math.Inf(-1), math.NaN()} {
		_, err := calcSubscriptionBalanceQuota(price)
		require.Error(t, err)
	}
}

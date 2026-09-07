package model

import (
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionPurchaseLimitCountsOnlyUnexpiredActiveHoldings(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		now := common.GetTimestamp()
		for _, sub := range []UserSubscription{
			{UserId: 1, PlanId: 1, Status: "active", EndTime: now + 3600, AmountTotal: 100, AmountUsed: 100},
			{UserId: 1, PlanId: 1, Status: "active", StartTime: now + 60, EndTime: now + 7200},
			{UserId: 1, PlanId: 1, Status: "active", EndTime: now},
			{UserId: 1, PlanId: 1, Status: "active", EndTime: now - 1},
			{UserId: 1, PlanId: 1, Status: "expired", EndTime: now + 3600},
			{UserId: 1, PlanId: 1, Status: "cancelled", EndTime: now + 3600},
			{UserId: 2, PlanId: 1, Status: "active", EndTime: now + 3600},
			{UserId: 1, PlanId: 2, Status: "active", EndTime: now + 3600},
		} {
			require.NoError(t, DB.Create(&sub).Error)
		}
		count, err := CountActiveUserSubscriptionsByPlan(1, 1)
		require.NoError(t, err)
		assert.EqualValues(t, 2, count, "used-up quota and future starts do not release an unexpired holding")
	})
}

func TestSubscriptionPurchaseLimitAcrossPurchasePaths(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = previousQuotaPerUnit })
	withCommerceDatabases(t, func(t *testing.T) {
		for _, source := range []string{PaymentProviderBalance, PaymentProviderEpay, PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffoPancake, "admin"} {
			t.Run(source, func(t *testing.T) {
				user := User{Username: "limit-" + source, AffCode: source, Group: "default", Quota: 10000000}
				require.NoError(t, DB.Create(&user).Error)
				plan := SubscriptionPlan{Title: "Monthly " + source, PriceAmount: 1, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, MaxPurchasePerUser: 1, Enabled: true}
				require.NoError(t, DB.Create(&plan).Error)
				t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
				expired := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: common.GetTimestamp() - 1}
				require.NoError(t, DB.Create(&expired).Error)
				var order SubscriptionOrder
				purchase := func() error {
					switch source {
					case "balance":
						return PurchaseSubscriptionWithBalance(user.Id, plan.Id)
					case "admin":
						_, err := AdminBindSubscription(user.Id, plan.Id, "test")
						return err
					default:
						order = SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: 1, TradeNo: common.GetUUID(), PaymentProvider: source, PaymentMethod: source, Status: common.TopUpStatusPending}
						if err := order.Insert(); err != nil {
							return err
						}
						return CompleteSubscriptionOrder(order.TradeNo, "", source, "")
					}
				}
				require.NoError(t, purchase(), "expired holdings must allow another purchase")
				var afterPurchase User
				require.NoError(t, DB.First(&afterPurchase, user.Id).Error)
				if source == "balance" {
					assert.Equal(t, user.Quota-500000, afterPurchase.Quota)
				}
				if source != "balance" && source != "admin" {
					require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "repeated callback", source, ""))
				}
				require.EqualError(t, purchase(), "已达到该套餐购买上限")
				var afterRejection User
				require.NoError(t, DB.First(&afterRejection, user.Id).Error)
				assert.Equal(t, afterPurchase.Quota, afterRejection.Quota, "a rejected purchase must not debit the wallet")
				if source != "balance" && source != "admin" {
					require.NoError(t, DB.First(&order, order.Id).Error)
					assert.Equal(t, common.TopUpStatusPending, order.Status)
				}
				count, err := CountActiveUserSubscriptionsByPlan(user.Id, plan.Id)
				require.NoError(t, err)
				assert.EqualValues(t, 1, count)
				require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Update("end_time", common.GetTimestamp()-1).Error)
				require.NoError(t, purchase(), "expiry must release the slot without waiting for the cleanup task")
				plan.MaxPurchasePerUser = 0
				require.NoError(t, DB.Model(&plan).Update("max_purchase_per_user", 0).Error)
				InvalidateSubscriptionPlanCache(plan.Id)
				require.NoError(t, purchase(), "zero remains unlimited")
			})
		}
	})
}

func TestSubscriptionPurchaseLimitSerializesConcurrentBalancePurchases(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		user := User{Username: "concurrent-limit", Group: "default", Quota: 10000000}
		require.NoError(t, DB.Create(&user).Error)
		plan := SubscriptionPlan{Title: "Concurrent monthly", PriceAmount: 1, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, MaxPurchasePerUser: 1, Enabled: true}
		require.NoError(t, DB.Create(&plan).Error)
		t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
		start := make(chan struct{})
		results := make(chan error, 2)
		var workers sync.WaitGroup
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				results <- PurchaseSubscriptionWithBalance(user.Id, plan.Id)
			}()
		}
		close(start)
		workers.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			} else if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
				// SQLite may reject a competing writer instead of waiting for it.
				assert.True(t, err.Error() == "已达到该套餐购买上限" || (strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked")), "%v", err)
			} else {
				assert.EqualError(t, err, "已达到该套餐购买上限")
			}
		}
		assert.Equal(t, 1, successes)
		count, err := CountActiveUserSubscriptionsByPlan(user.Id, plan.Id)
		require.NoError(t, err)
		assert.EqualValues(t, 1, count)
		var after User
		require.NoError(t, DB.First(&after, user.Id).Error)
		requiredQuota, err := calcSubscriptionBalanceQuota(plan.PriceAmount)
		require.NoError(t, err)
		assert.Equal(t, user.Quota-requiredQuota, after.Quota)
	})
}

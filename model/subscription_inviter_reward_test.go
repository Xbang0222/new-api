package model

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type subscriptionRewardFixture struct {
	buyer   User
	inviter User
	plan    SubscriptionPlan
	order   SubscriptionOrder
}

func newSubscriptionRewardFixture(t *testing.T, provider string) subscriptionRewardFixture {
	t.Helper()
	tag := common.GetUUID()[:12]
	fixture := subscriptionRewardFixture{
		inviter: User{Username: "inviter-" + tag, AffCode: "i-" + tag, Status: common.UserStatusEnabled},
		buyer:   User{Username: "buyer-" + tag, AffCode: "b-" + tag, Status: common.UserStatusEnabled, Group: "default", Quota: 100000000},
		plan: SubscriptionPlan{Title: "Reward monthly", PriceAmount: 99, TotalAmount: 9000000,
			DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true},
	}
	require.NoError(t, DB.Create(&fixture.inviter).Error)
	fixture.buyer.InviterId = fixture.inviter.Id
	require.NoError(t, DB.Create(&fixture.buyer).Error)
	require.NoError(t, DB.Create(&fixture.plan).Error)
	fixture.order = SubscriptionOrder{UserId: fixture.buyer.Id, PlanId: fixture.plan.Id,
		Money: 17, TradeNo: "subscription-reward-" + tag, PaymentProvider: provider,
		PaymentMethod: provider, CreateTime: common.GetTimestamp() - 60, Status: common.TopUpStatusPending}
	require.NoError(t, fixture.order.Insert())
	return fixture
}

func configureSubscriptionRewardForTest(t *testing.T, rewardType string, rewardValue int) {
	t.Helper()
	configureInviterRewardForTest(t, rewardType, rewardValue, true)
	previousUnit, previousRate := common.QuotaPerUnit, operation_setting.USDExchangeRate
	common.QuotaPerUnit, operation_setting.USDExchangeRate = 500000, 7.3
	t.Cleanup(func() {
		common.QuotaPerUnit, operation_setting.USDExchangeRate = previousUnit, previousRate
	})
}

func assertSubscriptionRewardNotCommitted(t *testing.T, fixture subscriptionRewardFixture, topUpCount int64) {
	t.Helper()
	var order SubscriptionOrder
	require.NoError(t, DB.First(&order, fixture.order.Id).Error)
	assert.Equal(t, fixture.order.Status, order.Status)
	assert.Equal(t, fixture.order.CompleteTime, order.CompleteTime)
	assert.Equal(t, fixture.order.ProviderPayload, order.ProviderPayload)
	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", fixture.buyer.Id).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ?", fixture.order.TradeNo).Count(&count).Error)
	assert.Equal(t, topUpCount, count)
	buyer := loadInviterRewardUser(t, fixture.buyer.Id)
	inviter := loadInviterRewardUser(t, fixture.inviter.Id)
	assert.Equal(t, fixture.buyer.Quota, buyer.Quota)
	assert.Equal(t, fixture.inviter.AffQuota, inviter.AffQuota)
	assert.Equal(t, fixture.inviter.AffHistoryQuota, inviter.AffHistoryQuota)
	require.NoError(t, DB.Model(&Log{}).Where("user_id IN ?", []int{fixture.buyer.Id, fixture.inviter.Id}).Count(&count).Error)
	assert.Zero(t, count, "a rolled-back payment must not publish a success log")
}

func TestSubscriptionInviterRewardUsesVerifiedPaymentAndFreezesHistory(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, test := range []struct {
			provider string
			receipt  TopUpPaymentReceipt
			typeName string
			value    int
			base     int
			reward   int
		}{
			{PaymentProviderEpay, TopUpPaymentReceipt{730, "CNY"}, "percentage", 15, 500000, 75000},
			{PaymentProviderStripe, TopUpPaymentReceipt{125, "USD"}, "percentage", 15, 625000, 93750},
			{PaymentProviderCreem, TopUpPaymentReceipt{250, "USD"}, "fixed", 321, 1250000, 321},
			{PaymentProviderWaffoPancake, TopUpPaymentReceipt{1095, "CNY"}, "percentage", 15, 750000, 112500},
			{PaymentProviderEpay, TopUpPaymentReceipt{1, "CNY"}, "percentage", 15, 684, 102},
		} {
			t.Run(test.provider, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, test.typeName, test.value)
				fixture := newSubscriptionRewardFixture(t, test.provider)
				require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "verified payload", test.provider, "", test.receipt))
				common.InviterRewardType, common.InviterRewardValue = "fixed", 888
				common.QuotaPerUnit, operation_setting.USDExchangeRate = 100000, 9
				require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "replayed payload", test.provider, "", test.receipt))

				inviter := loadInviterRewardUser(t, fixture.inviter.Id)
				assert.Equal(t, test.reward, inviter.AffQuota)
				assert.Equal(t, test.reward, inviter.AffHistoryQuota)
				assert.Equal(t, fixture.buyer.Quota, loadInviterRewardUser(t, fixture.buyer.Id).Quota)
				topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
				require.NotNil(t, topUp)
				assert.Zero(t, topUp.Amount, "a subscription receipt must not become a wallet recharge")
				assert.True(t, topUp.InviterRewardSent)
				assert.Equal(t, test.base, topUp.InviterRewardBaseQuota)
				assert.Equal(t, test.reward, topUp.InviterRewardQuota)
				assert.Equal(t, test.receipt.AmountMinor, topUp.PaidAmountMinor)
				assert.Equal(t, test.receipt.Currency, topUp.PaidCurrency)
				var subscriptions []UserSubscription
				require.NoError(t, DB.Where("user_id = ?", fixture.buyer.Id).Find(&subscriptions).Error)
				require.Len(t, subscriptions, 1)
				assert.Equal(t, fixture.plan.TotalAmount, subscriptions[0].AmountTotal)
				var order SubscriptionOrder
				require.NoError(t, DB.First(&order, fixture.order.Id).Error)
				assert.Equal(t, common.TopUpStatusSuccess, order.Status)
				assert.Equal(t, "verified payload", order.ProviderPayload)
				rebates, total, err := GetInviterRebateDetails(fixture.inviter.Id, 20)
				require.NoError(t, err)
				require.EqualValues(t, 1, total)
				require.Len(t, rebates, 1)
				assert.Equal(t, test.base, rebates[0].RechargeQuota)
				assert.Equal(t, test.reward, rebates[0].RewardQuota)
				assert.Equal(t, fixture.order.TradeNo, rebates[0].TradeNo)
				assert.Equal(t, test.provider, rebates[0].PaymentProvider)
				var logs []Log
				require.NoError(t, DB.Where("user_id = ? AND type = ?", fixture.inviter.Id, LogTypeSystem).Find(&logs).Error)
				require.Len(t, logs, 1)
				assert.Contains(t, logs[0].Content, "订阅购买")
				assert.Contains(t, logs[0].Content, fixture.order.TradeNo)
			})
		}
	})
}

func TestSubscriptionInviterRewardRepairsLegacySuccessWithoutCreatingSubscription(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		configureSubscriptionRewardForTest(t, "percentage", 10)
		fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		fixture.order.Status = common.TopUpStatusSuccess
		fixture.order.CompleteTime = common.GetTimestamp() - 30
		require.NoError(t, DB.Save(&fixture.order).Error)
		legacy := TopUp{UserId: fixture.buyer.Id, TradeNo: fixture.order.TradeNo, Money: fixture.order.Money,
			PaymentMethod: fixture.order.PaymentMethod, Status: common.TopUpStatusSuccess, CompleteTime: fixture.order.CompleteTime}
		require.NoError(t, DB.Create(&legacy).Error)
		existing := UserSubscription{UserId: fixture.buyer.Id, PlanId: fixture.plan.Id, Status: "expired", Source: "order", AmountTotal: 123, EndTime: common.GetTimestamp() - 5}
		require.NoError(t, DB.Create(&existing).Error)
		// Historical prices and even deleted plans are irrelevant to the receipt.
		require.NoError(t, DB.Delete(&fixture.plan).Error)
		receipt := TopUpPaymentReceipt{123, "USD"}
		require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "verified replay", PaymentProviderStripe, "", receipt))
		require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "verified replay", PaymentProviderStripe, "", receipt))
		var subscriptions []UserSubscription
		require.NoError(t, DB.Where("user_id = ?", fixture.buyer.Id).Find(&subscriptions).Error)
		require.Len(t, subscriptions, 1)
		assert.Equal(t, existing, subscriptions[0])
		var afterOrder SubscriptionOrder
		require.NoError(t, DB.First(&afterOrder, fixture.order.Id).Error)
		assert.Equal(t, fixture.order, afterOrder)
		topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
		require.NotNil(t, topUp)
		assert.Equal(t, legacy.Id, topUp.Id)
		assert.Equal(t, PaymentProviderStripe, topUp.PaymentProvider)
		assert.Equal(t, fixture.order.CompleteTime, topUp.CompleteTime)
		assert.True(t, topUp.InviterRewardSent)
		assert.Equal(t, 615000, topUp.InviterRewardBaseQuota)
		assert.Equal(t, 61500, topUp.InviterRewardQuota)
		inviter := loadInviterRewardUser(t, fixture.inviter.Id)
		assert.Equal(t, 61500, inviter.AffQuota)
		assert.Equal(t, 61500, inviter.AffHistoryQuota)
	})
}

func TestSubscriptionInviterRewardSkipsIneligiblePurchases(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, scenario := range []string{"disabled", "zero_reward", "unconfirmed", "no_inviter", "self_inviter", "deleted_inviter", "balance", "admin"} {
			t.Run(scenario, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, "percentage", 10)
				fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
				switch scenario {
				case "disabled":
					common.InviterRewardType = ""
				case "zero_reward":
					common.InviterRewardValue = 0
				case "unconfirmed":
					operation_setting.GetPaymentSetting().ComplianceConfirmed = false
				case "no_inviter":
					require.NoError(t, DB.Model(&fixture.buyer).Update("inviter_id", 0).Error)
				case "self_inviter":
					require.NoError(t, DB.Model(&fixture.buyer).Update("inviter_id", fixture.buyer.Id).Error)
				case "deleted_inviter":
					require.NoError(t, DB.Delete(&fixture.inviter).Error)
				}
				if scenario == "balance" {
					require.NoError(t, PurchaseSubscriptionWithBalance(fixture.buyer.Id, fixture.plan.Id))
				} else if scenario == "admin" {
					_, err := AdminBindSubscription(fixture.buyer.Id, fixture.plan.Id, "reward exclusion")
					require.NoError(t, err)
				} else {
					require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderStripe, ""))
					topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
					require.NotNil(t, topUp)
					assert.False(t, topUp.InviterRewardSent)
					assert.Zero(t, topUp.InviterRewardQuota)
				}
				var inviter User
				require.NoError(t, DB.Unscoped().First(&inviter, fixture.inviter.Id).Error)
				buyer := loadInviterRewardUser(t, fixture.buyer.Id)
				assert.Zero(t, inviter.AffQuota)
				assert.Zero(t, inviter.AffHistoryQuota)
				assert.Zero(t, buyer.AffQuota)
				assert.Zero(t, buyer.AffHistoryQuota)
				var subscriptions int64
				require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", fixture.buyer.Id).Count(&subscriptions).Error)
				assert.EqualValues(t, 1, subscriptions)
				rebates, total, err := GetInviterRebateDetails(fixture.inviter.Id, 20)
				require.NoError(t, err)
				assert.Zero(t, total)
				assert.Empty(t, rebates)
			})
		}
	})
}

func TestSubscriptionInviterRewardRejectsUnusableReceiptsAtomically(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, test := range []struct {
			name     string
			receipts []TopUpPaymentReceipt
			rate     float64
			unit     float64
		}{
			{"missing", nil, 7.3, 500000},
			{"zero", []TopUpPaymentReceipt{{0, "USD"}}, 7.3, 500000},
			{"negative", []TopUpPaymentReceipt{{-1, "USD"}}, 7.3, 500000},
			{"unknown_currency", []TopUpPaymentReceipt{{100, "invalid"}}, 7.3, 500000},
			{"unsupported_currency", []TopUpPaymentReceipt{{100, "EUR"}}, 7.3, 500000},
			{"multiple_receipts", []TopUpPaymentReceipt{{100, "USD"}, {100, "USD"}}, 7.3, 500000},
			{"minor_units_overflow", []TopUpPaymentReceipt{{int64(common.MaxWalletQuota) + 1, "USD"}}, 7.3, 500000},
			{"quota_overflow", []TopUpPaymentReceipt{{int64(common.MaxWalletQuota), "USD"}}, 7.3, 500000},
			{"zero_exchange_rate", []TopUpPaymentReceipt{{100, "CNY"}}, 0, 500000},
			{"negative_exchange_rate", []TopUpPaymentReceipt{{100, "CNY"}}, -1, 500000},
			{"nan_exchange_rate", []TopUpPaymentReceipt{{100, "CNY"}}, math.NaN(), 500000},
			{"infinite_exchange_rate", []TopUpPaymentReceipt{{100, "CNY"}}, math.Inf(1), 500000},
			{"zero_quota_unit", []TopUpPaymentReceipt{{100, "USD"}}, 7.3, 0},
			{"nan_quota_unit", []TopUpPaymentReceipt{{100, "USD"}}, 7.3, math.NaN()},
			{"infinite_quota_unit", []TopUpPaymentReceipt{{100, "USD"}}, 7.3, math.Inf(1)},
		} {
			t.Run(test.name, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, "percentage", 10)
				operation_setting.USDExchangeRate, common.QuotaPerUnit = test.rate, test.unit
				fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
				err := CompleteSubscriptionOrder(fixture.order.TradeNo, "not committed", PaymentProviderStripe, "", test.receipts...)
				require.Error(t, err)
				if test.name == "missing" {
					assert.ErrorIs(t, err, ErrSubscriptionPaymentReceiptRequired)
				}
				assertSubscriptionRewardNotCommitted(t, fixture, 0)
			})
		}
	})
}

func TestSubscriptionInviterRewardRejectsMismatchedPaymentRecord(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, scenario := range []string{"callback_provider", "owner", "recharge_amount", "provider", "payment_method", "changed_receipt"} {
			t.Run(scenario, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, "percentage", 10)
				fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
				receipt := TopUpPaymentReceipt{100, "USD"}
				if scenario == "callback_provider" {
					err := CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderEpay, "", receipt)
					require.ErrorIs(t, err, ErrPaymentMethodMismatch)
					assertSubscriptionRewardNotCommitted(t, fixture, 0)
					return
				}
				topUp := TopUp{UserId: fixture.buyer.Id, TradeNo: fixture.order.TradeNo, Money: fixture.order.Money,
					PaymentProvider: PaymentProviderStripe, PaymentMethod: PaymentMethodStripe, Status: common.TopUpStatusSuccess}
				switch scenario {
				case "owner":
					topUp.UserId = fixture.inviter.Id
				case "recharge_amount":
					topUp.Amount = 1
				case "provider":
					topUp.PaymentProvider = PaymentProviderCreem
				case "payment_method":
					topUp.PaymentMethod = PaymentMethodBalance
				case "changed_receipt":
					topUp.PaidAmountMinor, topUp.PaidCurrency = 99, "USD"
				}
				require.NoError(t, DB.Create(&topUp).Error)
				require.Error(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderStripe, "", receipt))
				assertSubscriptionRewardNotCommitted(t, fixture, 1)
				after := GetTopUpByTradeNo(fixture.order.TradeNo)
				require.NotNil(t, after)
				assert.Equal(t, topUp, *after)
			})
		}
	})
}

func TestSubscriptionInviterRewardRollsBackWriteFailuresAndCanRetry(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, stage := range []string{"inviter_balance", "reward_marker", "subscription_order"} {
			t.Run(stage, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, "percentage", 10)
				fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
				forcedError := fmt.Errorf("forced %s write failure", stage)
				callbackName := "test:subscription_reward_failure"
				require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
					if tx.Statement == nil {
						return
					}
					updates, _ := tx.Statement.Dest.(map[string]interface{})
					_, writesBalance := updates["aff_quota"]
					_, writesMarker := updates["inviter_reward_sent"]
					if stage == "inviter_balance" && tx.Statement.Table == "users" && writesBalance ||
						stage == "reward_marker" && tx.Statement.Table == "top_ups" && writesMarker ||
						stage == "subscription_order" && tx.Statement.Table == "subscription_orders" {
						tx.AddError(forcedError)
					}
				}))
				t.Cleanup(func() { assert.NoError(t, DB.Callback().Update().Remove(callbackName)) })
				receipt := TopUpPaymentReceipt{100, "USD"}
				err := CompleteSubscriptionOrder(fixture.order.TradeNo, "rollback payload", PaymentProviderStripe, "", receipt)
				require.ErrorIs(t, err, forcedError)
				assertSubscriptionRewardNotCommitted(t, fixture, 0)
				require.NoError(t, DB.Callback().Update().Remove(callbackName))
				require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "retry payload", PaymentProviderStripe, "", receipt))
				inviter := loadInviterRewardUser(t, fixture.inviter.Id)
				assert.Equal(t, 50000, inviter.AffQuota)
				assert.Equal(t, 50000, inviter.AffHistoryQuota)
				topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
				require.NotNil(t, topUp)
				assert.True(t, topUp.InviterRewardSent)
			})
		}
	})
}

func TestSubscriptionInviterRewardConcurrentCallbacksCreditExactlyOnce(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		configureSubscriptionRewardForTest(t, "percentage", 10)
		fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		receipt := TopUpPaymentReceipt{100, "USD"}
		completeConcurrentSubscriptionRewards(t, []SubscriptionOrder{fixture.order, fixture.order}, receipt)
		inviter := loadInviterRewardUser(t, fixture.inviter.Id)
		assert.Equal(t, 50000, inviter.AffQuota)
		assert.Equal(t, 50000, inviter.AffHistoryQuota)
		var count int64
		require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", fixture.buyer.Id).Count(&count).Error)
		assert.EqualValues(t, 1, count)
		require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ? AND inviter_reward_sent = ?", fixture.order.TradeNo, true).Count(&count).Error)
		assert.EqualValues(t, 1, count)
		require.NoError(t, DB.Model(&Log{}).Where("user_id = ? AND type = ?", fixture.inviter.Id, LogTypeSystem).Count(&count).Error)
		assert.EqualValues(t, 1, count)
	})
}

func TestSubscriptionInviterRewardCyclicInvitersCompleteWithoutDeadlock(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		configureSubscriptionRewardForTest(t, "percentage", 10)
		first := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		second := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		require.NoError(t, DB.Model(&first.buyer).Update("inviter_id", second.buyer.Id).Error)
		require.NoError(t, DB.Model(&second.buyer).Update("inviter_id", first.buyer.Id).Error)
		completeConcurrentSubscriptionRewards(t, []SubscriptionOrder{first.order, second.order}, TopUpPaymentReceipt{100, "USD"})
		for _, fixture := range []subscriptionRewardFixture{first, second} {
			buyer := loadInviterRewardUser(t, fixture.buyer.Id)
			assert.Equal(t, fixture.buyer.Quota, buyer.Quota)
			assert.Equal(t, 50000, buyer.AffQuota)
			assert.Equal(t, 50000, buyer.AffHistoryQuota)
			var count int64
			require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", buyer.Id).Count(&count).Error)
			assert.EqualValues(t, 1, count)
			topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
			require.NotNil(t, topUp)
			assert.True(t, topUp.InviterRewardSent)
			assert.Equal(t, 50000, topUp.InviterRewardQuota)
		}
	})
}

func completeConcurrentSubscriptionRewards(t *testing.T, orders []SubscriptionOrder, receipt TopUpPaymentReceipt) {
	t.Helper()
	start := make(chan struct{})
	results := make([]error, len(orders))
	var workers sync.WaitGroup
	for index, order := range orders {
		workers.Add(1)
		go func(index int, order SubscriptionOrder) {
			defer workers.Done()
			<-start
			results[index] = CompleteSubscriptionOrder(order.TradeNo, "concurrent receipt", order.PaymentProvider, "", receipt)
		}(index, order)
	}
	close(start)
	workers.Wait()
	for index, err := range results {
		if err != nil && common.UsingMainDatabase(common.DatabaseTypeSQLite) &&
			(strings.Contains(err.Error(), "SQLITE_BUSY") || strings.Contains(err.Error(), "database is locked")) {
			// SQLite may reject a competing writer. Retry only that transient
			// outcome after both callbacks finish; server dialects must wait.
			order := orders[index]
			err = CompleteSubscriptionOrder(order.TradeNo, "concurrent receipt", order.PaymentProvider, "", receipt)
		}
		require.NoError(t, err)
	}
}

func TestSubscriptionInviterRewardRejectsBalanceOverflowAtomically(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		configureSubscriptionRewardForTest(t, "fixed", common.MaxWalletQuota)
		fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		fixture.inviter.AffQuota, fixture.inviter.AffHistoryQuota = 1, 1
		require.NoError(t, DB.Model(&fixture.inviter).Updates(map[string]interface{}{
			"aff_quota": fixture.inviter.AffQuota, "aff_history": fixture.inviter.AffHistoryQuota,
		}).Error)
		err := CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderStripe, "", TopUpPaymentReceipt{100, "USD"})
		require.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
		assertSubscriptionRewardNotCommitted(t, fixture, 0)
	})
}

func TestSubscriptionInviterRewardFixedAmountDoesNotDependOnCurrencyConversion(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, test := range []struct {
			name     string
			currency string
			rate     float64
		}{
			{"verified_eur", "EUR", 7.3},
			{"cny_with_unavailable_rate", "CNY", 0},
		} {
			t.Run(test.name, func(t *testing.T) {
				configureSubscriptionRewardForTest(t, "fixed", 321)
				operation_setting.USDExchangeRate = test.rate
				fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
				receipt := TopUpPaymentReceipt{100, test.currency}
				require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderStripe, "", receipt))
				common.InviterRewardType, common.InviterRewardValue = "percentage", 100
				operation_setting.USDExchangeRate = 7.3
				require.NoError(t, CompleteSubscriptionOrder(fixture.order.TradeNo, "", PaymentProviderStripe, "", receipt))
				inviter := loadInviterRewardUser(t, fixture.inviter.Id)
				assert.Equal(t, 321, inviter.AffQuota)
				assert.Equal(t, 321, inviter.AffHistoryQuota)
				topUp := GetTopUpByTradeNo(fixture.order.TradeNo)
				require.NotNil(t, topUp)
				assert.True(t, topUp.InviterRewardSent)
				assert.Zero(t, topUp.InviterRewardBaseQuota)
				assert.Equal(t, 321, topUp.InviterRewardQuota)
				rebates, total, err := GetInviterRebateDetails(fixture.inviter.Id, 20)
				require.NoError(t, err)
				require.EqualValues(t, 1, total)
				require.Len(t, rebates, 1)
				assert.Zero(t, rebates[0].RechargeQuota)
				assert.Equal(t, 321, rebates[0].RewardQuota)
			})
		}
	})
}

func TestSubscriptionInviterRewardPreservesRechargeZeroBaseExclusion(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		configureSubscriptionRewardForTest(t, "fixed", 321)
		fixture := newSubscriptionRewardFixture(t, PaymentProviderStripe)
		topUp := TopUp{UserId: fixture.buyer.Id, Amount: 1, Money: 1, TradeNo: "ordinary-recharge-" + fixture.order.TradeNo,
			PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess}
		require.NoError(t, DB.Create(&topUp).Error)
		require.NoError(t, ProcessInviterReward(fixture.buyer.Id, 0, topUp.Id))
		inviter := loadInviterRewardUser(t, fixture.inviter.Id)
		assert.Zero(t, inviter.AffQuota)
		assert.Zero(t, inviter.AffHistoryQuota)
		after := GetTopUpByTradeNo(topUp.TradeNo)
		require.NotNil(t, after)
		assert.False(t, after.InviterRewardSent)
		assert.Zero(t, after.InviterRewardQuota)

		// Legacy subscription rows can carry a sent marker without a credited
		// snapshot. Today's fixed policy must not invent a historical reward.
		require.NoError(t, DB.Model(&topUp).Updates(map[string]interface{}{
			"amount": 0, "payment_provider": "", "inviter_reward_sent": true,
		}).Error)
		rebates, total, err := GetInviterRebateDetails(fixture.inviter.Id, 20)
		require.NoError(t, err)
		require.EqualValues(t, 1, total)
		require.Len(t, rebates, 1)
		assert.Zero(t, rebates[0].RechargeQuota)
		assert.Zero(t, rebates[0].RewardQuota)
	})
}

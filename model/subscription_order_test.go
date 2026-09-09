package model

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// External DSNs must point to a disposable codex_subscription_order_* database.
// Run this same contract against each engine; SQLite runs without configuration.
func setupSubscriptionOrderDB(t *testing.T) {
	t.Helper()
	var dialector gorm.Dialector
	kind := common.DatabaseTypeSQLite
	if dsn := os.Getenv("SUBSCRIPTION_TEST_MYSQL_DSN"); dsn != "" {
		require.Contains(t, dsn, "/codex_subscription_order_")
		dialector, kind = mysql.Open(dsn), common.DatabaseTypeMySQL
	} else if dsn := os.Getenv("SUBSCRIPTION_TEST_POSTGRES_DSN"); dsn != "" {
		require.Contains(t, dsn, "dbname=codex_subscription_order_")
		dialector, kind = postgres.Open(dsn), common.DatabaseTypePostgreSQL
	} else {
		dialector = sqlite.Open(filepath.Join(t.TempDir(), "subscription.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate")
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	originalDB, originalLogDB := DB, LOG_DB
	originalMain, originalLog := common.MainDatabaseType(), common.LogDatabaseType()
	originalRedis := common.RedisEnabled
	DB, LOG_DB = db, db
	common.RedisEnabled = false
	common.SetDatabaseTypes(kind, kind)
	initCol()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// A single connection also catches accidental DB calls outside a transaction.
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
		DB, LOG_DB = originalDB, originalLogDB
		common.SetDatabaseTypes(originalMain, originalLog)
		common.RedisEnabled = originalRedis
		initCol()
	})
	for _, table := range []any{&SubscriptionPreConsumeRecord{}, &UserSubscription{}, &SubscriptionPlan{}, &User{}} {
		require.NoError(t, db.Migrator().DropTable(table))
	}
	require.NoError(t, db.AutoMigrate(&User{}, &SubscriptionPlan{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}))
	var version string
	if kind == common.DatabaseTypeSQLite {
		require.NoError(t, db.Raw("SELECT sqlite_version()").Scan(&version).Error)
	} else {
		require.NoError(t, db.Raw("SELECT version()").Scan(&version).Error)
	}
	t.Logf("database=%s version=%s", kind, version)
}

func TestSubscriptionDebitOrderDatabaseContract(t *testing.T) {
	setupSubscriptionOrderDB(t)
	now := GetDBTimestamp()
	// An existing settings document without the new key keeps legacy debit order.
	user := User{Username: "debit-order", AffCode: "order1", Setting: `{"language":"zh","billing_preference":"subscription_only","notification_email":"saved@example.com"}`}
	require.NoError(t, DB.Create(&user).Error)
	other := User{Username: "other-debit-order", AffCode: "order2"}
	require.NoError(t, DB.Create(&other).Error)
	plan := SubscriptionPlan{Title: "Order test", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, QuotaResetPeriod: SubscriptionResetNever}
	require.NoError(t, DB.Create(&plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })
	subs := []UserSubscription{
		{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now + 10000, AmountTotal: 1000},
		{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now + 20000, AmountTotal: 1000},
		{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now + 5000, AmountTotal: 1000},
		{UserId: user.Id, PlanId: plan.Id, Status: "cancelled", EndTime: now + 10000, AmountTotal: 1000},
		{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now - 1, AmountTotal: 1000},
		{UserId: other.Id, PlanId: plan.Id, Status: "active", EndTime: now + 10000, AmountTotal: 1000},
	}
	for i := range subs {
		require.NoError(t, DB.Create(&subs[i]).Error)
	}
	first, preferred, earliest := subs[0].Id, subs[1].Id, subs[2].Id
	list, err := GetAllActiveUserSubscriptions(user.Id)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, earliest, list[0].Subscription.Id)
	result, err := PreConsumeUserSubscription("default-order", user.Id, "test", 0, 100)
	require.NoError(t, err)
	assert.Equal(t, earliest, result.UserSubscriptionId)

	t.Run("saved order is used for display and debit", func(t *testing.T) {
		require.NoError(t, UpdateSubscriptionDebitOrder(user.Id, []int{preferred, first}))
		list, err := GetAllActiveUserSubscriptions(user.Id)
		require.NoError(t, err)
		assert.Equal(t, []int{preferred, first, earliest}, []int{list[0].Subscription.Id, list[1].Subscription.Id, list[2].Subscription.Id})
		settings, err := GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.Equal(t, "saved@example.com", settings.NotificationEmail)
		assert.Equal(t, "subscription_only", settings.BillingPreference)
		result, err := PreConsumeUserSubscription("saved-order", user.Id, "test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, preferred, result.UserSubscriptionId)
		// General settings cannot remove or overwrite the latest debit preference.
		require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{Language: "en", SubscriptionOrder: []int{earliest}}))
		settings, err = GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.Equal(t, []int{preferred, first}, settings.SubscriptionOrder)
		assert.Equal(t, "en", settings.Language)
	})

	t.Run("cache failure does not report a committed order as failed", func(t *testing.T) {
		originalRDB := common.RDB
		client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
		require.NoError(t, client.Close())
		common.RDB, common.RedisEnabled = client, true
		err := UpdateSubscriptionDebitOrder(user.Id, []int{first, preferred})
		common.RDB, common.RedisEnabled = originalRDB, false
		require.NoError(t, err)
		var saved User
		require.NoError(t, DB.First(&saved, user.Id).Error)
		assert.Equal(t, []int{first, preferred}, saved.GetSetting().SubscriptionOrder)
		result, err := PreConsumeUserSubscription("cache-failure-order", user.Id, "test", 0, 1)
		require.NoError(t, err)
		assert.Equal(t, first, result.UserSubscriptionId)
		require.NoError(t, RefundSubscriptionPreConsume("cache-failure-order"))
		require.NoError(t, UpdateSubscriptionDebitOrder(user.Id, []int{preferred, first}))
	})

	t.Run("invalid IDs cannot change saved order", func(t *testing.T) {
		cases := [][]int{{preferred, preferred}, {0}, {-1}, {subs[3].Id}, {subs[4].Id}, {subs[5].Id}, {999999}, make([]int, 1001)}
		for i, ids := range cases {
			assert.Error(t, UpdateSubscriptionDebitOrder(user.Id, ids), "case %d", i)
			settings, err := GetUserSetting(user.Id, true)
			require.NoError(t, err)
			assert.Equal(t, []int{preferred, first}, settings.SubscriptionOrder)
		}
	})

	t.Run("insufficient quota skips to the next subscription", func(t *testing.T) {
		require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", preferred).Update("amount_used", 950).Error)
		result, err := PreConsumeUserSubscription("fallback-order", user.Id, "test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, first, result.UserSubscriptionId)
		var untouched UserSubscription
		require.NoError(t, DB.First(&untouched, preferred).Error)
		assert.EqualValues(t, 950, untouched.AmountUsed)
	})

	t.Run("reordering never redirects an existing debit or refund", func(t *testing.T) {
		require.NoError(t, UpdateSubscriptionDebitOrder(user.Id, []int{earliest, preferred, first}))
		result, err := PreConsumeUserSubscription("fallback-order", user.Id, "test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, first, result.UserSubscriptionId)
		assert.EqualValues(t, 100, result.AmountUsedAfter)
		require.NoError(t, RefundSubscriptionPreConsume("fallback-order"))
		require.NoError(t, RefundSubscriptionPreConsume("fallback-order"))
		var refunded UserSubscription
		require.NoError(t, DB.First(&refunded, first).Error)
		assert.Zero(t, refunded.AmountUsed)
		_, err = PreConsumeUserSubscription("fallback-order", user.Id, "test", 0, 100)
		require.ErrorContains(t, err, "already refunded")
	})

	t.Run("refund and idempotency marker roll back together", func(t *testing.T) {
		result, err := PreConsumeUserSubscription("rollback-refund", user.Id, "test", 0, 100)
		require.NoError(t, err)
		injected := errors.New("injected marker write failure")
		require.NoError(t, DB.Callback().Update().Before("gorm:update").Register("test:refund-marker-failure", func(tx *gorm.DB) {
			if tx.Statement.Table == "subscription_pre_consume_records" {
				tx.AddError(injected)
			}
		}))
		err = RefundSubscriptionPreConsume("rollback-refund")
		require.NoError(t, DB.Callback().Update().Remove("test:refund-marker-failure"))
		require.ErrorIs(t, err, injected)
		var unchanged UserSubscription
		require.NoError(t, DB.First(&unchanged, result.UserSubscriptionId).Error)
		assert.Equal(t, result.AmountUsedAfter, unchanged.AmountUsed)
		require.NoError(t, RefundSubscriptionPreConsume("rollback-refund"))
	})

	t.Run("a new subscription follows saved preferences and unlimited quota is usable", func(t *testing.T) {
		added := UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: now + 1000, AmountTotal: 0}
		require.NoError(t, DB.Create(&added).Error)
		list, err := GetAllActiveUserSubscriptions(user.Id)
		require.NoError(t, err)
		require.Len(t, list, 4)
		assert.Equal(t, added.Id, list[3].Subscription.Id)
		require.NoError(t, UpdateSubscriptionDebitOrder(user.Id, []int{added.Id, earliest, preferred, first}))
		result, err := PreConsumeUserSubscription("unlimited-order", user.Id, "test", 0, 2000)
		require.NoError(t, err)
		assert.Equal(t, added.Id, result.UserSubscriptionId)
		// Cancellation removes it from selection even though its ID remains saved.
		require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", added.Id).Update("status", "cancelled").Error)
		result, err = PreConsumeUserSubscription("cancelled-order", user.Id, "test", 0, 1)
		require.NoError(t, err)
		assert.Equal(t, earliest, result.UserSubscriptionId)
	})

	t.Run("expiry removes stale preferences and empty restores default", func(t *testing.T) {
		require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", preferred).Update("end_time", now-1).Error)
		require.NoError(t, UpdateSubscriptionDebitOrder(user.Id, []int{}))
		settings, err := GetUserSetting(user.Id, true)
		require.NoError(t, err)
		assert.Empty(t, settings.SubscriptionOrder)
		list, err := GetAllActiveUserSubscriptions(user.Id)
		require.NoError(t, err)
		require.Len(t, list, 2)
		assert.Equal(t, earliest, list[0].Subscription.Id)
		result, err := PreConsumeUserSubscription("reset-order", user.Id, "test", 0, 100)
		require.NoError(t, err)
		assert.Equal(t, earliest, result.UserSubscriptionId)
	})
}

func TestSortSubscriptionsForDebitBreaksExpiryTiesByID(t *testing.T) {
	subs := []UserSubscription{{Id: 3, EndTime: 10}, {Id: 2, EndTime: 10}, {Id: 1, EndTime: 20}}
	SortSubscriptionsForDebit(subs, []int{999, 1})
	assert.Equal(t, []int{1, 2, 3}, []int{subs[0].Id, subs[1].Id, subs[2].Id})
}

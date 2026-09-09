package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacyInviterRewardTopUp preserves TopUp at 8bfb8de8e, the baseline before
// persisted invitation reward snapshots. This checkout has no release tag;
// the fixture represents that exact commit, not a claimed latest release.
type legacyInviterRewardTopUp struct {
	Id                int     `json:"id"`
	UserId            int     `json:"user_id" gorm:"index"`
	Amount            int64   `json:"amount"`
	Money             float64 `json:"money"`
	PaidAmountMinor   int64   `json:"paid_amount_minor" gorm:"type:bigint;not null;default:0"`
	PaidCurrency      string  `json:"paid_currency" gorm:"type:varchar(16);not null;default:''"`
	TradeNo           string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod     string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider   string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	CreateTime        int64   `json:"create_time"`
	CompleteTime      int64   `json:"complete_time"`
	Status            string  `json:"status"`
	InviterRewardSent bool    `json:"inviter_reward_sent" gorm:"default:false"`
}

func (legacyInviterRewardTopUp) TableName() string { return "top_ups" }

func TestInviterRewardMigrationFreshAndLegacy(t *testing.T) {
	for _, baseline := range []string{"fresh", "legacy_8bfb8de8e"} {
		t.Run(baseline, func(t *testing.T) {
			withCommerceDatabases(t, func(t *testing.T) {
				if baseline != "fresh" {
					// The commerce fixture created this disposable table empty.
					require.NoError(t, DB.Migrator().DropTable(&TopUp{}))
					require.NoError(t, DB.AutoMigrate(&legacyInviterRewardTopUp{}))
					require.False(t, DB.Migrator().HasColumn(&TopUp{}, "inviter_reward_quota"))
					require.False(t, DB.Migrator().HasColumn(&TopUp{}, "inviter_reward_base_quota"))
				}
				assertInviterRewardMigrationPreservesData(t, baseline)
			})
		})
	}
}

func assertInviterRewardMigrationPreservesData(t *testing.T, baseline string) {
	t.Helper()
	user := User{Username: "reward-migration", AffCode: "reward-migration", Quota: 12345678, AffQuota: 1500, AffHistoryQuota: 2500}
	require.NoError(t, DB.Create(&user).Error)
	legacyRows := []legacyInviterRewardTopUp{
		{UserId: user.Id, Amount: 25000000, Money: 50, TradeNo: "reward-already-sent", PaidAmountMinor: 3000000000, PaidCurrency: "JPY", PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, CreateTime: 1700000000, CompleteTime: 1700000100, Status: common.TopUpStatusSuccess, InviterRewardSent: true},
		{UserId: user.Id, Amount: 75, Money: 750, TradeNo: "reward-not-sent", PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, CreateTime: 1700000200, CompleteTime: 1700000300, Status: common.TopUpStatusSuccess},
	}
	require.NoError(t, DB.Create(&legacyRows).Error)
	indexesBefore := invoiceMigrationIndexes(t, DB)
	recorder := &migrationSQLRecorder{}
	migrationDB := DB.Session(&gorm.Session{Logger: recorder})
	// Measure unchanged baseline DDL so the repeat migration check distinguishes
	// existing driver behavior from changes introduced by the reward snapshots.
	require.NoError(t, migrationDB.AutoMigrate(&legacyInviterRewardTopUp{}))
	baselineMutations := recorder.schemaMutations()
	t.Logf("unchanged baseline TopUp migration: %d schema mutations", len(baselineMutations))
	var firstPassColumns []string
	snapshot := TopUp{
		UserId: user.Id, Amount: 60000000000, Money: 120000, TradeNo: "reward-new-snapshot",
		PaidAmountMinor: 12000000, PaidCurrency: "USD", PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		CreateTime: 1700000400, CompleteTime: 1700000500, Status: common.TopUpStatusSuccess, InviterRewardSent: true,
		InviterRewardQuota: 3000000000, InviterRewardBaseQuota: 60000000000,
	}
	for pass := 1; pass <= 2; pass++ {
		recorder.reset()
		require.NoError(t, migrationDB.AutoMigrate(&TopUp{}))
		mutations := recorder.schemaMutations()
		t.Logf("%s inviter reward migration pass %d: %d schema mutations", baseline, pass, len(mutations))
		if baseline == "fresh" || pass == 2 {
			assert.Equal(t, invoiceMigrationMutationTargets(baselineMutations), invoiceMigrationMutationTargets(mutations), "reward snapshots must not introduce additional repeated DDL")
		} else {
			assert.NotEmpty(t, mutations, "the legacy case must exercise the actual upgrade")
		}
		assert.Equal(t, indexesBefore, invoiceMigrationIndexes(t, DB), "migration must preserve existing index names and uniqueness constraints")
		columnTypes, err := DB.Migrator().ColumnTypes(&TopUp{})
		require.NoError(t, err)
		var columns []string
		for _, column := range columnTypes {
			nullable, hasNullable := column.Nullable()
			defaultValue, hasDefault := column.DefaultValue()
			columns = append(columns, fmt.Sprintf("%s:%s:null=%t/%t:default=%s/%t", column.Name(), column.DatabaseTypeName(), nullable, hasNullable, defaultValue, hasDefault))
			if column.Name() == "inviter_reward_quota" || column.Name() == "inviter_reward_base_quota" {
				assert.True(t, hasNullable)
				assert.False(t, nullable, "reward snapshots must not become nullable")
				assert.True(t, hasDefault)
				assert.Equal(t, "0", defaultValue)
			}
		}
		if pass == 1 {
			firstPassColumns = columns
			require.NoError(t, DB.Create(&snapshot).Error)
		} else {
			assert.Equal(t, firstPassColumns, columns, "restarting must preserve all column types, nullability and defaults")
		}
		for _, expected := range legacyRows {
			var preserved legacyInviterRewardTopUp
			require.NoError(t, DB.First(&preserved, expected.Id).Error)
			assert.Equal(t, expected, preserved, "every legacy value, receipt and reward marker must survive")
			var inventedSnapshots int64
			require.NoError(t, DB.Model(&TopUp{}).Where("id = ? AND (inviter_reward_quota IS NULL OR inviter_reward_quota <> 0 OR inviter_reward_base_quota IS NULL OR inviter_reward_base_quota <> 0)", expected.Id).Count(&inventedSnapshots).Error)
			assert.Zero(t, inventedSnapshots, "migration must initialize explicit zero snapshots without inventing or replaying historical rewards")
		}
		var preservedSnapshot TopUp
		require.NoError(t, DB.First(&preservedSnapshot, snapshot.Id).Error)
		assert.Equal(t, snapshot, preservedSnapshot, "new reward and basis snapshots above int32 must be preserved exactly")
		var preservedUser User
		require.NoError(t, DB.First(&preservedUser, user.Id).Error)
		assert.Equal(t, user.Quota, preservedUser.Quota)
		assert.Equal(t, user.AffQuota, preservedUser.AffQuota)
		assert.Equal(t, user.AffHistoryQuota, preservedUser.AffHistoryQuota)
		duplicate := TopUp{UserId: user.Id, TradeNo: legacyRows[0].TradeNo, Amount: 1, Money: 1}
		assert.ErrorContains(t, DB.Create(&duplicate).Error, "trade_no", "a repeated migration must preserve trade number uniqueness")
		insertedWithDefaults := legacyInviterRewardTopUp{UserId: user.Id, TradeNo: fmt.Sprintf("reward-defaults-%d", pass)}
		require.NoError(t, DB.Create(&insertedWithDefaults).Error)
		var explicitDefaults int64
		require.NoError(t, DB.Model(&TopUp{}).Where("id = ? AND inviter_reward_quota = 0 AND inviter_reward_base_quota = 0", insertedWithDefaults.Id).Count(&explicitDefaults).Error)
		assert.EqualValues(t, 1, explicitDefaults, "database defaults must protect insertions that omit both new columns")
	}
}

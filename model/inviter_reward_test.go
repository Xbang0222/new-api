package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configureInviterRewardForTest(t *testing.T, rewardType string, rewardValue int, complianceConfirmed bool) {
	t.Helper()

	oldRewardType := common.InviterRewardType
	oldRewardValue := common.InviterRewardValue
	paymentSetting := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentSetting.ComplianceConfirmed
	oldComplianceTermsVersion := paymentSetting.ComplianceTermsVersion

	common.InviterRewardType = rewardType
	common.InviterRewardValue = rewardValue
	paymentSetting.ComplianceConfirmed = complianceConfirmed
	if complianceConfirmed {
		paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	} else {
		paymentSetting.ComplianceTermsVersion = ""
	}

	t.Cleanup(func() {
		common.InviterRewardType = oldRewardType
		common.InviterRewardValue = oldRewardValue
		paymentSetting.ComplianceConfirmed = oldComplianceConfirmed
		paymentSetting.ComplianceTermsVersion = oldComplianceTermsVersion
	})
}

func configureMinAffTransferQuotaForTest(t *testing.T, minTransferQuota int) {
	t.Helper()

	oldMinTransferQuota := common.MinAffTransferQuota
	common.MinAffTransferQuota = minTransferQuota

	t.Cleanup(func() {
		common.MinAffTransferQuota = oldMinTransferQuota
	})
}

func insertInviterRewardUser(t *testing.T, id int, username string, inviterId int) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:        id,
		Username:  username,
		Status:    common.UserStatusEnabled,
		InviterId: inviterId,
		AffCode:   username,
	}).Error)
}

func insertInviterRewardTopUp(t *testing.T, userId int, tradeNo string) *TopUp {
	t.Helper()
	topUp := &TopUp{
		UserId:          userId,
		Amount:          2,
		Money:           2,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: PaymentProviderStripe,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())
	return topUp
}

func loadInviterRewardUser(t *testing.T, id int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, id).Error)
	return user
}

func TestProcessInviterReward_FixedRewardIsIdempotent(t *testing.T) {
	truncateTables(t)
	configureInviterRewardForTest(t, "fixed", 500, true)

	insertInviterRewardUser(t, 1, "reward_inviter", 0)
	insertInviterRewardUser(t, 2, "reward_invitee", 1)
	topUp := insertInviterRewardTopUp(t, 2, "reward-fixed")

	require.NoError(t, ProcessInviterReward(2, 10000, topUp.Id))
	require.NoError(t, ProcessInviterReward(2, 10000, topUp.Id))

	inviter := loadInviterRewardUser(t, 1)
	assert.Equal(t, 500, inviter.AffQuota)
	assert.Equal(t, 500, inviter.AffHistoryQuota)

	reloadedTopUp := GetTopUpByTradeNo("reward-fixed")
	require.NotNil(t, reloadedTopUp)
	assert.True(t, reloadedTopUp.InviterRewardSent)
}

func TestProcessInviterReward_PercentageReward(t *testing.T) {
	truncateTables(t)
	configureInviterRewardForTest(t, "percentage", 15, true)

	insertInviterRewardUser(t, 21, "reward_percent_inviter", 0)
	insertInviterRewardUser(t, 22, "reward_percent_invitee", 21)
	topUp := insertInviterRewardTopUp(t, 22, "reward-percentage")

	require.NoError(t, ProcessInviterReward(22, 20000, topUp.Id))
	require.NoError(t, ProcessInviterReward(22, 20000, topUp.Id))

	inviter := loadInviterRewardUser(t, 21)
	assert.Equal(t, 3000, inviter.AffQuota)
	assert.Equal(t, 3000, inviter.AffHistoryQuota)
}

func TestProcessInviterReward_SkipsWhenComplianceUnconfirmed(t *testing.T) {
	truncateTables(t)
	configureInviterRewardForTest(t, "percentage", 10, false)

	insertInviterRewardUser(t, 11, "reward_inviter_unconfirmed", 0)
	insertInviterRewardUser(t, 12, "reward_invitee_unconfirmed", 11)
	topUp := insertInviterRewardTopUp(t, 12, "reward-unconfirmed")

	require.NoError(t, ProcessInviterReward(12, 10000, topUp.Id))

	inviter := loadInviterRewardUser(t, 11)
	assert.Zero(t, inviter.AffQuota)
	assert.Zero(t, inviter.AffHistoryQuota)
}

func TestGetInvitationDetails(t *testing.T) {
	truncateTables(t)
	configureInviterRewardForTest(t, "fixed", 500, true)

	insertInviterRewardUser(t, 41, "details_inviter", 0)
	insertInviterRewardUser(t, 42, "details_invitee", 41)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 42).Update("created_at", int64(1779783729)).Error)
	topUp := insertInviterRewardTopUp(t, 42, "reward-details")

	require.NoError(t, ProcessInviterReward(42, 10000, topUp.Id))

	invitedUsers, invitedTotal, err := GetInvitedUserDetails(41, 200)
	require.NoError(t, err)
	require.EqualValues(t, 1, invitedTotal)
	require.Len(t, invitedUsers, 1)
	assert.Equal(t, 42, invitedUsers[0].Id)
	assert.Equal(t, "details_invitee", invitedUsers[0].Username)
	assert.EqualValues(t, 1779783729, invitedUsers[0].CreatedAt)

	rebates, rebateTotal, err := GetInviterRebateDetails(41, 200)
	require.NoError(t, err)
	require.EqualValues(t, 1, rebateTotal)
	require.Len(t, rebates, 1)
	assert.Equal(t, 42, rebates[0].InviteeId)
	assert.Equal(t, "details_invitee", rebates[0].InviteeUsername)
	assert.Equal(t, "reward-details", rebates[0].TradeNo)
	assert.Equal(t, 500, rebates[0].RewardQuota)
	assert.Equal(t, topUp.CreateTime, rebates[0].CompleteTime)
}

func TestDeletingInvitedUserRefreshesAffCount(t *testing.T) {
	truncateTables(t)

	insertInviterRewardUser(t, 51, "delete_inviter", 0)
	insertInviterRewardUser(t, 52, "delete_invitee", 51)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 51).Update("aff_count", 1).Error)

	invitee := User{Id: 52}
	require.NoError(t, DB.Create(&UserAccessToken{UserId: invitee.Id, TokenHash: AccessTokenFingerprint("nap_delete-invitee")}).Error)
	revoked, err := invitee.Delete()
	require.NoError(t, err)
	assert.EqualValues(t, 1, revoked)
	var remainingTokens int64
	require.NoError(t, DB.Model(&UserAccessToken{}).Where("user_id = ?", invitee.Id).Count(&remainingTokens).Error)
	assert.Zero(t, remainingTokens)

	inviter := loadInviterRewardUser(t, 51)
	assert.Equal(t, 0, inviter.AffCount)

	invitedUsers, invitedTotal, err := GetInvitedUserDetails(51, 200)
	require.NoError(t, err)
	assert.Zero(t, invitedTotal)
	assert.Empty(t, invitedUsers)
}

func TestHardDeletingInvitedUserRefreshesAffCount(t *testing.T) {
	truncateTables(t)

	insertInviterRewardUser(t, 61, "hard_delete_inviter", 0)
	insertInviterRewardUser(t, 62, "hard_delete_invitee", 61)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 61).Update("aff_count", 1).Error)

	require.NoError(t, DB.Create(&UserAccessToken{UserId: 62, TokenHash: AccessTokenFingerprint("nap_hard-delete-invitee")}).Error)
	revoked, err := HardDeleteUserById(62)
	require.NoError(t, err)
	assert.EqualValues(t, 1, revoked)
	var remainingTokens int64
	require.NoError(t, DB.Model(&UserAccessToken{}).Where("user_id = ?", 62).Count(&remainingTokens).Error)
	assert.Zero(t, remainingTokens)

	inviter := loadInviterRewardUser(t, 61)
	assert.Equal(t, 0, inviter.AffCount)
}

func TestTransferAffQuotaToQuota_UsesConfiguredMinimum(t *testing.T) {
	truncateTables(t)
	configureMinAffTransferQuotaForTest(t, 1000)

	require.NoError(t, DB.Create(&User{
		Id:       31,
		Username: "reward_transfer_user",
		Status:   common.UserStatusEnabled,
		Quota:    100,
		AffQuota: 700,
		AffCode:  "reward_transfer_user",
	}).Error)

	user := loadInviterRewardUser(t, 31)
	require.Error(t, user.TransferAffQuotaToQuota(700))

	reloaded := loadInviterRewardUser(t, 31)
	assert.Equal(t, 100, reloaded.Quota)
	assert.Equal(t, 700, reloaded.AffQuota)
	var logCount int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ? AND type = ?", 31, LogTypeTopup).Count(&logCount).Error)
	assert.Zero(t, logCount)

	configureMinAffTransferQuotaForTest(t, 500)

	user = loadInviterRewardUser(t, 31)
	require.NoError(t, user.TransferAffQuotaToQuota(500))

	reloaded = loadInviterRewardUser(t, 31)
	assert.Equal(t, 600, reloaded.Quota)
	assert.Equal(t, 200, reloaded.AffQuota)
	var logs []Log
	require.NoError(t, LOG_DB.Where("user_id = ? AND type = ?", 31, LogTypeTopup).Find(&logs).Error)
	require.Len(t, logs, 1)
	assert.Contains(t, logs[0].Content, "Transferred invitation quota to balance")
	assert.Contains(t, logs[0].Content, logger.FormatQuota(500))
}

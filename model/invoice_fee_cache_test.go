package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReserveInvoiceFeeCacheUsesLiveQuota(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, batchEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("batch_%t", batchEnabled), func(t *testing.T) {
				resetBatchUpdateTestState(t)
				server := useUserCacheMiniRedis(t)
				common.BatchUpdateEnabled = batchEnabled
				user := createReserveTestUser(t, 1_000)
				require.NoError(t, populateUserCache(user))
				server.HSet(getUserCacheKey(user.Id), "Quota", "100")

				reserved, err := ReserveInvoiceFeeCache(user.Id, 150)
				require.ErrorIs(t, err, ErrWalletQuotaInsufficient)
				assert.False(t, reserved)
				assert.Equal(t, "100", server.HGet(getUserCacheKey(user.Id), "Quota"))
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id))
			})
		}
	})
}

func TestReserveInvoiceFeeCacheDeductsOnlyCacheAndCompensates(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, batchEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("batch_%t", batchEnabled), func(t *testing.T) {
				resetBatchUpdateTestState(t)
				server := useUserCacheMiniRedis(t)
				common.BatchUpdateEnabled = batchEnabled
				user := createReserveTestUser(t, 1_000)
				require.NoError(t, populateUserCache(user))
				server.HSet(getUserCacheKey(user.Id), "Quota", "150")

				reserved, err := ReserveInvoiceFeeCache(user.Id, 150)
				require.NoError(t, err)
				assert.True(t, reserved)
				assert.Equal(t, "0", server.HGet(getUserCacheKey(user.Id), "Quota"))
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id))
				batchUpdate()
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id), "cache reservation must not queue a second database charge")

				reserved, err = ReserveInvoiceFeeCache(user.Id, 1)
				require.ErrorIs(t, err, ErrWalletQuotaInsufficient)
				assert.False(t, reserved)
				require.NoError(t, SyncUserQuotaCacheDelta(user.Id, 150))
				assert.Equal(t, "150", server.HGet(getUserCacheKey(user.Id), "Quota"))
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id))
			})
		}
	})
}

func TestReserveInvoiceFeeCacheHydratesMissingUser(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		server := useUserCacheMiniRedis(t)
		user := createReserveTestUser(t, 150)
		reserved, err := ReserveInvoiceFeeCache(user.Id, 150)
		require.NoError(t, err)
		assert.True(t, reserved)
		assert.Equal(t, "0", server.HGet(getUserCacheKey(user.Id), "Quota"))
		assert.Equal(t, 150, getUserQuotaFromDB(t, user.Id))

		reserved, err = ReserveInvoiceFeeCache(user.Id+1, 1)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		assert.False(t, reserved)
	})
}

func TestReserveInvoiceFeeCacheFailsClosed(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		for _, batchEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("batch_%t", batchEnabled), func(t *testing.T) {
				resetBatchUpdateTestState(t)
				server := useUserCacheMiniRedis(t)
				common.BatchUpdateEnabled = batchEnabled
				user := createReserveTestUser(t, 1_000)
				require.NoError(t, populateUserCache(user))
				server.HSet(getUserCacheKey(user.Id), "Quota", "150")
				server.SetError("ERR invoice reservation cache failure")

				reserved, err := ReserveInvoiceFeeCache(user.Id, 150)
				require.ErrorContains(t, err, "invoice reservation cache failure")
				assert.False(t, reserved)
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id))
				server.SetError("")
				assert.Equal(t, "150", server.HGet(getUserCacheKey(user.Id), "Quota"))

				server.HDel(getUserCacheKey(user.Id), "Quota")
				reserved, err = ReserveInvoiceFeeCache(user.Id, 150)
				require.ErrorContains(t, err, "remains unavailable")
				assert.False(t, reserved)
				assert.Equal(t, 1_000, getUserQuotaFromDB(t, user.Id))
			})
		}
	})
}

func TestReserveInvoiceFeeCacheNoopAndInvalidInput(t *testing.T) {
	server := useUserCacheMiniRedis(t)
	common.RedisEnabled = false
	reserved, err := ReserveInvoiceFeeCache(1, 150)
	require.NoError(t, err)
	assert.False(t, reserved)
	common.RedisEnabled = true
	reserved, err = ReserveInvoiceFeeCache(1, 0)
	require.NoError(t, err)
	assert.False(t, reserved)
	assert.Empty(t, server.Keys())

	for _, input := range []struct{ userId, quota int }{
		{0, 150}, {1, -1}, {1, common.MaxWalletQuota + 1},
	} {
		reserved, err = ReserveInvoiceFeeCache(input.userId, input.quota)
		assert.Error(t, err)
		assert.False(t, reserved)
	}
	assert.Empty(t, server.Keys())

	client := common.RDB
	common.RDB = nil
	reserved, err = ReserveInvoiceFeeCache(1, 150)
	common.RDB = client
	assert.Error(t, err)
	assert.False(t, reserved)
}

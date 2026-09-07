package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInvoiceFeeCacheTransactionContracts(t *testing.T) {
	withInvoiceDatabases(t, func(t *testing.T) {
		for _, scenario := range []string{"insufficient live balance", "commit and refund", "transaction rollback"} {
			t.Run(scenario, func(t *testing.T) {
				setupInvoiceFeeSettings(t)
				user, order, input := createInvoiceFeeFixture(t, 200_000, 50)
				server := miniredis.RunT(t)
				oldRedis, oldClient, oldBatch := common.RedisEnabled, common.RDB, common.BatchUpdateEnabled
				client := redis.NewClient(&redis.Options{Addr: server.Addr()})
				common.RedisEnabled, common.RDB, common.BatchUpdateEnabled = true, client, true
				t.Cleanup(func() {
					require.NoError(t, client.Close())
					common.RedisEnabled, common.RDB, common.BatchUpdateEnabled = oldRedis, oldClient, oldBatch
				})
				_, err := model.GetUserCache(user.Id)
				require.NoError(t, err)
				// Model an API consumption already reflected in Redis but awaiting
				// the existing batch writer. The invoice must use the live balance.
				liveQuota := 100_000
				if scenario == "insufficient live balance" {
					liveQuota = 0
				}
				require.NoError(t, model.SyncUserQuotaCacheDelta(user.Id, liveQuota-user.Quota))
				writeFailure := errors.New("invoice payment row failed after quota reservation")
				if scenario == "transaction rollback" {
					callbackName := "invoice_cache_rollback"
					require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
						if _, payment := tx.Statement.Dest.(*model.InvoicePaymentOrder); payment {
							tx.AddError(writeFailure)
						}
					}))
					t.Cleanup(func() { require.NoError(t, model.DB.Callback().Create().Remove(callbackName)) })
				}

				application, err := CreateInvoiceApplication(user.Id, input)
				if scenario == "commit and refund" {
					require.NoError(t, err)
					assert.Equal(t, 100_000, application.ChargedQuota)
					cached, cacheErr := model.GetUserCache(user.Id)
					require.NoError(t, cacheErr)
					assert.Zero(t, cached.Quota, "the reserved fee must not be debited again after commit")
					var wallet model.User
					require.NoError(t, model.DB.First(&wallet, user.Id).Error)
					assert.Equal(t, 100_000, wallet.Quota)
					_, err = CancelInvoiceApplication(application.Id, user.Id)
					require.NoError(t, err)
				} else {
					assert.Nil(t, application)
					if scenario == "insufficient live balance" {
						require.ErrorIs(t, err, model.ErrWalletQuotaInsufficient)
					} else {
						require.ErrorIs(t, err, writeFailure)
					}
				}
				cached, cacheErr := model.GetUserCache(user.Id)
				require.NoError(t, cacheErr)
				assert.Equal(t, liveQuota, cached.Quota)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, 200_000, wallet.Quota)
				var count int64
				require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Where("user_id = ?", user.Id).Count(&count).Error)
				assert.Zero(t, count)
				require.NoError(t, model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id = ?", order.Id).Count(&count).Error)
				assert.Zero(t, count)
				require.NoError(t, model.DB.Model(&model.InvoicePaymentOrder{}).Where("user_id = ?", user.Id).Count(&count).Error)
				assert.Zero(t, count)
			})
		}
	})
}

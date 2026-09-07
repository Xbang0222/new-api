package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// ReserveInvoiceFeeCache reserves only the live Redis balance. The caller must
// charge the database in its invoice transaction, compensate a failed transaction
// with SyncUserQuotaCacheDelta, and avoid a second cache debit after committing.
// Redis failures cannot authorize a spend using a potentially stale DB balance.
func ReserveInvoiceFeeCache(userId int, quota int) (bool, error) {
	if userId <= 0 || quota < 0 || quota > common.MaxWalletQuota {
		return false, errors.New("invalid invoice fee cache reservation")
	}
	if quota == 0 || !common.RedisEnabled {
		return false, nil
	}
	if common.RDB == nil {
		return false, errors.New("invoice fee quota cache is unavailable")
	}

	result, err := cacheTryReserveUserQuota(userId, int64(quota))
	if err != nil {
		return false, fmt.Errorf("reserve invoice fee cache: %w", err)
	}
	if result == cacheQuotaMiss {
		if _, err := GetUserCache(userId); err != nil {
			return false, fmt.Errorf("load invoice fee quota cache: %w", err)
		}
		result, err = cacheTryReserveUserQuota(userId, int64(quota))
		if err != nil {
			return false, fmt.Errorf("reserve invoice fee cache after loading: %w", err)
		}
	}
	if result == cacheQuotaInsufficient {
		return false, ErrWalletQuotaInsufficient
	}
	if result != cacheQuotaOK {
		return false, errors.New("invoice fee quota cache remains unavailable")
	}
	return true, nil
}

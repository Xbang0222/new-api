package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Users see their own top-ups from the last 365 days; admins see every record.
func TestUserTopUpHistoryCoversLastYear(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		const day = int64(24 * 60 * 60)
		now := common.GetTimestamp()
		user := User{Username: "history-user", AffCode: "history-user", Group: "default"}
		require.NoError(t, DB.Create(&user).Error)
		other := User{Username: "history-other", AffCode: "history-other", Group: "default"}
		require.NoError(t, DB.Create(&other).Error)
		for _, topup := range []TopUp{
			{UserId: user.Id, TradeNo: "history-400d", CreateTime: now - 400*day},
			{UserId: user.Id, TradeNo: "history-200d", CreateTime: now - 200*day},
			{UserId: user.Id, TradeNo: "history-10d", CreateTime: now - 10*day},
			{UserId: other.Id, TradeNo: "history-other-10d", CreateTime: now - 10*day},
		} {
			topup.Status = common.TopUpStatusSuccess
			require.NoError(t, DB.Create(&topup).Error)
		}
		tradeNos := func(topups []*TopUp) []string {
			result := make([]string, 0, len(topups))
			for _, topup := range topups {
				result = append(result, topup.TradeNo)
			}
			return result
		}
		pageInfo := &common.PageInfo{Page: 1, PageSize: 10}

		listed, total, err := GetUserTopUps(user.Id, pageInfo)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, []string{"history-10d", "history-200d"}, tradeNos(listed))

		searched, total, err := SearchUserTopUps(user.Id, "history%", pageInfo)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, []string{"history-10d", "history-200d"}, tradeNos(searched))

		all, total, err := GetAllTopUps(pageInfo)
		require.NoError(t, err)
		assert.Equal(t, int64(4), total)
		assert.Equal(t, []string{"history-other-10d", "history-10d", "history-200d", "history-400d"}, tradeNos(all))
	})
}

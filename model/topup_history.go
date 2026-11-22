package model

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"golang.org/x/text/currency"
	"gorm.io/gorm"
)

const MaxTopUpHistoryExport = 2000

type TopUpHistoryFilter struct {
	UserID   int
	Username string
	Keyword  string
	Self     bool
}

// TopUpHistoryRecord adds account names and exact receipt amounts without
// changing the provider-specific meaning of the stored Amount and Money.
type TopUpHistoryRecord struct {
	TopUp
	Username   string `json:"username"`
	PaidAmount string `json:"paid_amount"`
}

func GetTopUpHistory(ctx context.Context, filter TopUpHistoryFilter, page *common.PageInfo, export bool) ([]TopUpHistoryRecord, int64, error) {
	query := DB.WithContext(ctx).Model(&TopUp{})
	if filter.Self {
		if filter.UserID <= 0 {
			return nil, 0, common.NewMessage("Invalid user ID")
		}
		query = query.Where("create_time >= ?", topUpQueryCutoff())
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Username != "" {
		// Include deleted accounts so their retained payment records stay searchable.
		users := DB.WithContext(ctx).Unscoped().Model(&User{}).Select("id").Where("username = ?", filter.Username)
		query = query.Where("user_id IN (?)", users)
	}
	if filter.Keyword != "" {
		pattern, err := sanitizeLikePattern(filter.Keyword)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	var total int64
	limit, offset := MaxTopUpHistoryExport+1, 0
	if !export {
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
		limit, offset = page.GetPageSize(), page.GetStartIdx()
	}
	var topups []TopUp
	if err := query.Order("id DESC").Limit(limit).Offset(offset).Find(&topups).Error; err != nil {
		return nil, 0, err
	}
	if export {
		if len(topups) > MaxTopUpHistoryExport {
			return nil, 0, common.NewMessage("Too many records to export. Narrow your search to {{limit}} orders or fewer.", map[string]any{"limit": MaxTopUpHistoryExport})
		}
		total = int64(len(topups))
	}
	userIDs := make([]int, 0, len(topups))
	for _, topup := range topups {
		userIDs = append(userIDs, topup.UserId)
	}
	var users []User
	if len(userIDs) > 0 {
		if err := DB.WithContext(ctx).Unscoped().Select("id", "username").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, 0, err
		}
	}
	usernames := make(map[int]string, len(users))
	for _, user := range users {
		usernames[user.Id] = user.Username
	}
	records := make([]TopUpHistoryRecord, 0, len(topups))
	for _, topup := range topups {
		record := TopUpHistoryRecord{TopUp: topup, Username: usernames[topup.UserId]}
		unit, err := currency.ParseISO(strings.ToUpper(topup.PaidCurrency))
		if err == nil && unit != currency.XXX && topup.PaidAmountMinor > 0 {
			scale, _ := currency.Standard.Rounding(unit)
			record.PaidCurrency = unit.String()
			record.PaidAmount = decimal.NewFromInt(topup.PaidAmountMinor).Shift(-int32(scale)).StringFixed(int32(scale))
		}
		records = append(records, record)
	}
	return records, total, nil
}

// custom: invite reward log
package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
)

// InviteRewardLog 记录一次邀请返利发放的快照。
// 字段类型与 User / TopUp 对齐（int / int64），跨 SQLite / MySQL 5.7.8+ / PostgreSQL 9.6+ 兼容。
// 复合索引 (invitee_id, topup_id) 加速 backfill 的去重 SELECT；
// 不能用 UNIQUE，因为 topup_id=0（兑换码触发）允许同一 invitee 多条记录，
// 跨 SQLite/MySQL 5.7 也无法用 partial unique index 排除 0。
type InviteRewardLog struct {
	Id            int    `json:"id" gorm:"primaryKey;autoIncrement"`
	InviterId     int    `json:"inviter_id" gorm:"index:idx_invite_reward_inviter_created,priority:1"`
	InviteeId     int    `json:"invitee_id" gorm:"index:idx_invite_reward_invitee_topup,priority:1;index"`
	TopUpId       int    `json:"topup_id" gorm:"column:topup_id;index:idx_invite_reward_invitee_topup,priority:2;index"`
	RechargeQuota int    `json:"recharge_quota"`
	RewardQuota   int    `json:"reward_quota"`
	RewardType    string `json:"reward_type" gorm:"type:varchar(16)"`
	RewardValue   int    `json:"reward_value"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime;index:idx_invite_reward_inviter_created,priority:2"`
}

// InvitationSummaryItem 是按人汇总视图的单行结果。
type InvitationSummaryItem struct {
	InviteeId     int    `json:"invitee_id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	InvitedAt     int64  `json:"invited_at"`
	TotalRecharge int64  `json:"total_recharge"`
	TotalReward   int64  `json:"total_reward"`
	LastRewardAt  int64  `json:"last_reward_at"`
}

// InvitationLogItem 是流水视图的单行结果（附带 username / display_name；被邀请人已注销时为空）。
type InvitationLogItem struct {
	Id            int    `json:"id"`
	InviteeId     int    `json:"invitee_id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	TopUpId       int    `json:"topup_id"`
	RechargeQuota int    `json:"recharge_quota"`
	RewardQuota   int    `json:"reward_quota"`
	RewardType    string `json:"reward_type"`
	RewardValue   int    `json:"reward_value"`
	CreatedAt     int64  `json:"created_at"`
}

// GetInvitationSummary 按人聚合：邀请者邀请的所有用户 + 每人累计返利。
// 排序：返利高的在前，等同的按注册时间倒序；未充值的用户 total_reward=0 沉底。
// 软删除策略：list 与 total 都使用同样的 deleted_at IS NULL 过滤，避免双方不一致。
func GetInvitationSummary(inviterId int, page, pageSize int) ([]InvitationSummaryItem, int64, error) {
	if inviterId <= 0 || pageSize <= 0 {
		return nil, 0, nil
	}

	var total int64
	if err := DB.Model(&User{}).Where("inviter_id = ?", inviterId).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var items []InvitationSummaryItem
	err := DB.Table("users u").
		Select(`u.id AS invitee_id, u.username, u.display_name, u.created_at AS invited_at,
                COALESCE(SUM(log.recharge_quota), 0) AS total_recharge,
                COALESCE(SUM(log.reward_quota), 0)   AS total_reward,
                COALESCE(MAX(log.created_at), 0)     AS last_reward_at`).
		Joins("LEFT JOIN invite_reward_logs log ON log.invitee_id = u.id AND log.inviter_id = ?", inviterId).
		Where("u.inviter_id = ? AND u.deleted_at IS NULL", inviterId).
		Group("u.id, u.username, u.display_name, u.created_at").
		Order("total_reward DESC, u.created_at DESC").
		Limit(pageSize).Offset(offset).
		Scan(&items).Error

	return items, total, err
}

// GetInvitationLogs 返利流水视图。LEFT JOIN users 时显式过滤 deleted_at，让软删除
// 被邀请人的 username/display_name 返回 NULL，由前端 `username || '已注销用户 #ID'` fallback。
// 不过滤 → raw join 会返回真实用户名，违反 PII/隐私设计。
func GetInvitationLogs(inviterId int, page, pageSize int) ([]InvitationLogItem, int64, error) {
	if inviterId <= 0 || pageSize <= 0 {
		return nil, 0, nil
	}

	var total int64
	if err := DB.Model(&InviteRewardLog{}).Where("inviter_id = ?", inviterId).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var items []InvitationLogItem
	err := DB.Table("invite_reward_logs log").
		Select(`log.id, log.invitee_id, u.username, u.display_name,
                log.topup_id, log.recharge_quota, log.reward_quota,
                log.reward_type, log.reward_value, log.created_at`).
		Joins("LEFT JOIN users u ON u.id = log.invitee_id AND u.deleted_at IS NULL").
		Where("log.inviter_id = ?", inviterId).
		Order("log.created_at DESC, log.id DESC").
		Limit(pageSize).Offset(offset).
		Scan(&items).Error

	return items, total, err
}

// CountActiveInvitees 统计有过至少一次返利的被邀请人数（DISTINCT invitee_id）。
func CountActiveInvitees(inviterId int) (int64, error) {
	if inviterId <= 0 {
		return 0, nil
	}
	var n int64
	err := DB.Model(&InviteRewardLog{}).
		Where("inviter_id = ?", inviterId).
		Distinct("invitee_id").
		Count(&n).Error
	return n, err
}

// computeBackfillRewardQuota 按当前规则反推一笔 TopUp 应得的返利。
// 与运行时 ProcessInviterReward 的算法保持一致。
func computeBackfillRewardQuota(rechargeQuota int) int {
	if common.InviterRewardType == "percentage" {
		dRecharge := decimal.NewFromInt(int64(rechargeQuota))
		dPercent := decimal.NewFromInt(int64(common.InviterRewardValue))
		return int(dRecharge.Mul(dPercent).Div(decimal.NewFromInt(100)).IntPart())
	}
	return common.InviterRewardValue // fixed
}

// computeBackfillRechargeQuota 按 PaymentProvider 重算一笔 TopUp 当时实际给用户加的 quota。
// 与 ManualCompleteTopUp 主路径一致，并额外处理订阅 TopUp（Amount=0, Money>0）。
// 订阅在 upsertSubscriptionTopUpTx 中绕过 ManualCompleteTopUp 直接写 TopUp，
// Amount 固定为 0，单凭 Money * QuotaPerUnit 还原。
func computeBackfillRechargeQuota(t TopUp) int {
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	if t.PaymentProvider == PaymentProviderStripe {
		return int(decimal.NewFromFloat(t.Money).Mul(dQuotaPerUnit).IntPart())
	}
	if t.PaymentMethod == "creem" {
		return int(t.Amount) // Creem 的 Amount 已经是 quota
	}
	// Subscription TopUp（含 wallet/epay 渠道）：Amount=0 + Money>0，
	// 落入这里时优先用 Money 反推。
	if t.Amount == 0 && t.Money > 0 {
		return int(decimal.NewFromFloat(t.Money).Mul(dQuotaPerUnit).IntPart())
	}
	return int(decimal.NewFromInt(t.Amount).Mul(dQuotaPerUnit).IntPart())
}

// BackfillInviteRewardLogs 一次性扫描所有 inviter_reward_sent=true 的 TopUp，
// 按当前规则反推返利金额，写入 invite_reward_logs 表。
//
// 触发：main.go 启动时异步调用（goroutine + recover），不阻塞 server 启动。
// 幂等：Option["InviteRewardLogBackfilled"]="true" 标记已完成；零 failed 才置标记。
// 不准确点：历史规则若与当前规则不同，反推值会和实际发出值不符（spec 已声明）。
// 并发安全：Count 失败时把 t.Id 计入 failed，避免吞错重复插。
func BackfillInviteRewardLogs() error {
	// 读 OptionMap 加 RLock，与 model/option.go 写路径互斥。
	common.OptionMapRWMutex.RLock()
	backfilled := common.OptionMap["InviteRewardLogBackfilled"]
	rewardType := common.InviterRewardType
	rewardValue := common.InviterRewardValue
	common.OptionMapRWMutex.RUnlock()

	if backfilled == "true" {
		return nil
	}
	if rewardType == "" || rewardValue == 0 {
		// 规则关闭：不补录、不置标记，等管理员开启规则后下次启动重跑
		return nil
	}

	var processed, skipped, failed int
	var lastId int = 0
	const batchSize = 200

	for {
		var topups []TopUp
		err := DB.Where("inviter_reward_sent = ? AND status = ? AND id > ?",
			true, common.TopUpStatusSuccess, lastId).
			Order("id ASC").Limit(batchSize).Find(&topups).Error
		if err != nil {
			return fmt.Errorf("backfill 查询 TopUp 失败: %w", err)
		}
		if len(topups) == 0 {
			break
		}
		lastId = topups[len(topups)-1].Id

		for _, t := range topups {
			// 1) 跳过已有 log；Count 错误必须计入 failed，否则吞错会导致重复 insert
			var existsCount int64
			if err := DB.Model(&InviteRewardLog{}).
				Where("invitee_id = ? AND topup_id = ?", t.UserId, t.Id).
				Count(&existsCount).Error; err != nil {
				failed++
				common.SysError(fmt.Sprintf("backfill dedup count for topup %d failed: %v", t.Id, err))
				continue
			}
			if existsCount > 0 {
				skipped++
				continue
			}

			// 2) 找邀请者
			user, err := GetUserById(t.UserId, false)
			if err != nil || user == nil || user.InviterId == 0 || user.InviterId == t.UserId {
				skipped++
				continue
			}

			// 3) 反推 recharge_quota / reward_quota
			rechargeQuota := computeBackfillRechargeQuota(t)
			if rechargeQuota <= 0 {
				skipped++
				continue
			}
			rewardQuota := computeBackfillRewardQuota(rechargeQuota)
			if rewardQuota <= 0 {
				skipped++
				continue
			}

			// 4) 插入，CreatedAt 用 TopUp 真实时间；优先 CompleteTime，否则 CreateTime。
			//    `CreatedAt` 字段带 GORM `autoCreateTime`（ProcessInviterReward 路径需要它
			//    自动填当前时间），所以传 0 会被 GORM 当零值用 NOW() 覆盖——历史 TopUp 大量
			//    complete_time=0 时会让 backfill 全部写成"跑批那一秒"。给个非零值即可绕过。
			backfillCreatedAt := t.CompleteTime
			if backfillCreatedAt == 0 {
				backfillCreatedAt = t.CreateTime
			}
			log := InviteRewardLog{
				InviterId:     user.InviterId,
				InviteeId:     t.UserId,
				TopUpId:       t.Id,
				RechargeQuota: rechargeQuota,
				RewardQuota:   rewardQuota,
				RewardType:    rewardType,
				RewardValue:   rewardValue,
				CreatedAt:     backfillCreatedAt,
			}
			if err := DB.Create(&log).Error; err != nil {
				failed++
				common.SysError(fmt.Sprintf("backfill invite_reward_log for topup %d failed: %v", t.Id, err))
				continue
			}
			processed++
		}
	}

	common.SysLog(fmt.Sprintf("邀请返利明细补录完成: 处理 %d 条, 跳过 %d 条, 失败 %d 条",
		processed, skipped, failed))

	if failed == 0 {
		if err := UpdateOption("InviteRewardLogBackfilled", "true"); err != nil {
			common.SysError(fmt.Sprintf("set InviteRewardLogBackfilled failed: %v", err))
		}
	}
	return nil
}

// LaunchBackfillInviteRewardLogs 在后台 goroutine 中跑 backfill，配合 main.go
// 启动流程。带 recover 防 panic 拖垮整个 server。
func LaunchBackfillInviteRewardLogs() {
	gopool.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				common.SysError(fmt.Sprintf("BackfillInviteRewardLogs panic recovered: %v", r))
			}
		}()
		if err := BackfillInviteRewardLogs(); err != nil {
			common.SysError("BackfillInviteRewardLogs failed: " + err.Error())
		}
	})
}


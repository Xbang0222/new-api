package service

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 错误定义（集中管理，Controller 层统一映射）
// ---------------------------------------------------------------------------

var (
	ErrInvoiceDisabled         = errors.New("发票功能未启用")
	ErrAmountInsufficient      = errors.New("开票金额不足最低要求")
	ErrTopUpNotFound           = errors.New("充值记录不存在或不属于当前用户")
	ErrTopUpAlreadyInvoiced    = errors.New("部分充值记录已被开票")
	ErrTopUpNotSuccess         = errors.New("只能对已完成的充值记录开票")
	ErrInvoiceNotPending       = errors.New("只能撤销待审核状态的发票")
	ErrInvoiceNotFound         = errors.New("发票申请不存在")
	ErrInvoiceNotOwner         = errors.New("无权操作此发票")
	ErrHeaderLimitExceeded     = errors.New("发票抬头数量已达上限")
	ErrInvalidTaxNumber        = errors.New("纳税人识别号格式不正确")
	ErrNoTopUpsSelected        = errors.New("请至少选择一条充值记录")
	ErrInsufficientQuotaForFee = errors.New("余额不足以支付开票服务费") // custom: invoice fee
)

// ---------------------------------------------------------------------------
// SubmitInvoice — 提交发票申请（完整事务）
// ---------------------------------------------------------------------------

func SubmitInvoice(userId int, req *dto.SubmitInvoiceRequest) (*model.Invoice, error) {
	cfg := operation_setting.GetInvoiceSetting()
	if err := validateInvoiceRequest(cfg, req); err != nil {
		return nil, err
	}

	content := req.Content
	if content == "" {
		content = cfg.DefaultContent
	}

	var invoice model.Invoice
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		topups, err := validateTopUps(tx, req.TopUpIds, userId, cfg.MinAmount)
		if err != nil {
			return err
		}
		if err := createInvoiceAndItems(tx, &invoice, userId, topups, content, req); err != nil {
			return err
		}
		// custom: invoice fee — charge fee inside the same tx so submit is atomic.
		// Snapshots fee_rate / fee_amount / fee_quota onto the invoice row.
		return chargeInvoiceFee(tx, &invoice, cfg.FeeRate)
	})
	if err != nil {
		return nil, err
	}

	// custom: invoice fee — sync cache + audit log after successful commit.
	// LogTypeManage (not LogTypeConsume): the deduction is a platform service
	// charge, not user-driven token consumption, so it lives alongside other
	// admin/platform adjustments in the audit log. The matching refund uses
	// LogTypeRefund so reconciliation can pair them.
	if invoice.FeeQuota > 0 {
		model.SyncInvoiceFeeCacheDecr(userId, invoice.FeeQuota)
		model.RecordLog(userId, model.LogTypeManage,
			fmt.Sprintf("开票服务费 ¥%.2f / quota %d（费率 %.2f%%），发票 #%d",
				invoice.FeeAmount, invoice.FeeQuota, invoice.FeeRate*100, invoice.Id))
	}
	return &invoice, nil
}

// normalizeFeeRate clamps feeRate to [0, 1] and rejects non-finite inputs.
// Returns (rate, ok); ok=false means feature off / invalid input → bail.
// Single source of truth for the clamping contract used by both
// computeInvoiceFee (pure) and chargeInvoiceFee (snapshot writer), so the
// stored snapshot can never disagree with the rate that drove the math.
//
// custom: invoice fee
func normalizeFeeRate(feeRate float64) (float64, bool) {
	if !isFiniteFloat(feeRate) || feeRate <= 0 {
		return 0, false
	}
	if feeRate > 1 {
		return 1, true
	}
	return feeRate, true
}

// computeInvoiceFee is the pure-arithmetic core of chargeInvoiceFee, extracted
// so the rounding contract is unit-testable without DB.
//
// Why decimal (instead of plain float64): inputs are RMB amounts in the
// thousands–tens-of-thousands range and a sub-percent rate (e.g. 0.06).
// Plain float multiplication produces artifacts like 6.0599999998, which
// then propagate into both the user-visible fee_amount and (worse) the
// integer fee_quota via Ceil(), leading to off-by-one drift between the
// snapshot the user sees and the quota actually deducted. shopspring/decimal
// gives us exact base-10 arithmetic.
//
// Returns (feeAmountRMB, feeQuota). feeQuota is the integer quota actually
// deducted (Ceil so rounding favors the platform); feeAmountRMB is rounded
// back from feeQuota so the user-visible RMB equals the actual debit and
// reconciliation between log/UI/DB stays consistent.
//
// custom: invoice fee
func computeInvoiceFee(invoiceAmount, feeRate, price, quotaPerUnit float64) (float64, int) {
	if !isFiniteFloat(invoiceAmount) || !isFiniteFloat(price) || !isFiniteFloat(quotaPerUnit) {
		return 0, 0
	}
	if invoiceAmount <= 0 {
		return 0, 0
	}
	rate, ok := normalizeFeeRate(feeRate)
	if !ok {
		return 0, 0
	}
	feeRate = rate

	dPrice := decimal.NewFromFloat(price)
	if dPrice.Sign() <= 0 {
		dPrice = decimal.NewFromInt(1)
	}
	dQuotaPerUnit := decimal.NewFromFloat(quotaPerUnit)
	if dQuotaPerUnit.Sign() <= 0 {
		dQuotaPerUnit = decimal.NewFromInt(1)
	}

	// fee_quota = ceil( invoice_amount * fee_rate / Price * QuotaPerUnit )
	dRawFee := decimal.NewFromFloat(invoiceAmount).Mul(decimal.NewFromFloat(feeRate))
	dFeeQuota := dRawFee.Div(dPrice).Mul(dQuotaPerUnit).Ceil()
	feeQuota := int(dFeeQuota.IntPart())
	if feeQuota <= 0 {
		return 0, 0
	}

	// fee_amount (RMB) is derived back from fee_quota so the snapshot equals
	// the actual debit (avoids displayed-vs-deducted drift introduced by Ceil).
	dFeeAmount := dFeeQuota.Div(dQuotaPerUnit).Mul(dPrice)
	feeAmount, _ := dFeeAmount.Round(2).Float64()
	return feeAmount, feeQuota
}

func isFiniteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// chargeInvoiceFee deducts the configured service fee from the user's wallet
// inside an existing tx and writes the fee snapshot back to the invoice.
// Skips silently when feeRate <= 0 (feature off / older installs).
//
// Uses a conditional UPDATE (`WHERE quota >= feeQuota`) instead of SELECT FOR
// UPDATE so the deduction is atomic on all three supported databases including
// SQLite (which does not support row-level pessimistic locking). If RowsAffected
// is 0 either the user was not found (impossible — they are authenticated) or
// the balance was insufficient, both of which we surface as
// ErrInsufficientQuotaForFee.
//
// custom: invoice fee
func chargeInvoiceFee(tx *gorm.DB, invoice *model.Invoice, feeRate float64) error {
	// Single clamp via normalizeFeeRate so the snapshot rate written to the
	// invoice row is exactly the rate that drove the math — no double-clamping
	// in two places that could drift apart.
	normalizedRate, ok := normalizeFeeRate(feeRate)
	if !ok {
		return nil
	}
	feeAmount, feeQuota := computeInvoiceFee(invoice.Amount, normalizedRate, operation_setting.Price, common.QuotaPerUnit)
	if feeQuota <= 0 {
		return nil
	}

	// Atomic check-and-deduct: portable across SQLite/MySQL/PostgreSQL,
	// no FOR UPDATE needed. If quota is insufficient the WHERE filter
	// matches zero rows and we abort without touching the user row.
	result := tx.Model(&model.User{}).
		Where("id = ? AND quota >= ?", invoice.UserId, feeQuota).
		Update("quota", gorm.Expr("quota - ?", feeQuota))
	if result.Error != nil {
		return fmt.Errorf("扣除服务费失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrInsufficientQuotaForFee
	}

	// Snapshot fee onto the invoice row (so admin rate changes don't drift).
	// fee_amount is derived from fee_quota so the displayed RMB equals the
	// actual debit (see computeInvoiceFee).
	invoice.FeeRate = normalizedRate
	invoice.FeeAmount = feeAmount
	invoice.FeeQuota = feeQuota
	if err := tx.Model(&model.Invoice{}).Where("id = ?", invoice.Id).Updates(map[string]interface{}{
		"fee_rate":   invoice.FeeRate,
		"fee_amount": invoice.FeeAmount,
		"fee_quota":  invoice.FeeQuota,
	}).Error; err != nil {
		return fmt.Errorf("写入服务费快照失败: %w", err)
	}
	return nil
}

// refundInvoiceFee refunds a previously charged fee inside an existing tx.
// Atomic and idempotent: the `WHERE id = ? AND fee_refunded = false` filter
// flips fee_refunded only once across concurrent transactions; only the
// winning tx then increments the user's quota. Concurrent admin rejects on
// the same invoice cannot double-refund.
//
// Returns whether a refund was actually performed in this call (true) or
// skipped because there was no fee or someone else already refunded (false).
//
// custom: invoice fee
func refundInvoiceFee(tx *gorm.DB, invoice *model.Invoice) (bool, error) {
	if invoice.FeeQuota <= 0 {
		return false, nil
	}
	// CAS-style flip: claim the refund slot atomically.
	result := tx.Model(&model.Invoice{}).
		Where("id = ? AND fee_refunded = ?", invoice.Id, false).
		Update("fee_refunded", true)
	if result.Error != nil {
		return false, fmt.Errorf("更新退款标志失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Already refunded by an earlier tx — no-op (idempotent).
		invoice.FeeRefunded = true
		return false, nil
	}
	if err := tx.Model(&model.User{}).Where("id = ?", invoice.UserId).
		Update("quota", gorm.Expr("quota + ?", invoice.FeeQuota)).Error; err != nil {
		return false, fmt.Errorf("退还服务费失败: %w", err)
	}
	invoice.FeeRefunded = true
	return true, nil
}

// validateInvoiceRequest 前置校验（不占事务）
func validateInvoiceRequest(cfg *operation_setting.InvoiceSetting, req *dto.SubmitInvoiceRequest) error {
	if !cfg.Enabled {
		return ErrInvoiceDisabled
	}
	if len(req.TopUpIds) == 0 {
		return ErrNoTopUpsSelected
	}
	if !dto.IsValidTaxNumber(req.TaxNumber) {
		return ErrInvalidTaxNumber
	}
	return nil
}

// validateTopUps 事务内校验充值记录（行锁 + 状态 + 占用 + 金额）
func validateTopUps(tx *gorm.DB, topUpIds []int, userId int, minAmount float64) ([]*model.TopUp, error) {
	topups, err := model.GetTopUpsByIdsForUpdate(tx, topUpIds, userId)
	if err != nil {
		return nil, fmt.Errorf("查询充值记录失败: %w", err)
	}
	if len(topups) != len(topUpIds) {
		return nil, ErrTopUpNotFound
	}

	var totalAmount float64
	for _, t := range topups {
		if t.Status != common.TopUpStatusSuccess {
			return nil, ErrTopUpNotSuccess
		}
		totalAmount += t.Money
	}

	usedIds, err := model.GetInvoicedTopUpIds(tx, topUpIds)
	if err != nil {
		return nil, fmt.Errorf("查询开票状态失败: %w", err)
	}
	if len(usedIds) > 0 {
		return nil, ErrTopUpAlreadyInvoiced
	}

	if totalAmount < minAmount {
		return nil, fmt.Errorf("%w: 最低 %.0f 元，当前 %.2f 元", ErrAmountInsufficient, minAmount, totalAmount)
	}
	return topups, nil
}

// createInvoiceAndItems 创建发票记录 + 关联条目
func createInvoiceAndItems(tx *gorm.DB, invoice *model.Invoice, userId int, topups []*model.TopUp, content string, req *dto.SubmitInvoiceRequest) error {
	var totalAmount float64
	for _, t := range topups {
		totalAmount += t.Money
	}

	*invoice = model.Invoice{
		UserId:      userId,
		Email:       req.Email,
		CompanyName: req.CompanyName,
		TaxNumber:   req.TaxNumber,
		Amount:      totalAmount,
		Content:     content,
		Remark:      req.Remark,
		Status:      model.InvoiceStatusPending,
	}
	if err := model.CreateInvoice(tx, invoice); err != nil {
		return fmt.Errorf("创建发票申请失败: %w", err)
	}

	items := make([]*model.InvoiceItem, 0, len(topups))
	for _, t := range topups {
		items = append(items, &model.InvoiceItem{
			InvoiceId: invoice.Id,
			TopUpId:   t.Id,
			Amount:    t.Money,
		})
	}
	if err := model.BatchCreateInvoiceItems(tx, items); err != nil {
		return fmt.Errorf("创建发票关联记录失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// CancelInvoice — 用户撤销待审核的发票
// ---------------------------------------------------------------------------

func CancelInvoice(userId, invoiceId int) error {
	var refundedQuota int
	var refundedAmount float64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		invoice, err := model.GetInvoiceById(tx, invoiceId)
		if err != nil {
			return ErrInvoiceNotFound
		}
		if invoice.UserId != userId {
			return ErrInvoiceNotOwner
		}
		if invoice.Status != model.InvoiceStatusPending {
			return ErrInvoiceNotPending
		}

		// CAS transition: if a concurrent admin already moved this invoice off
		// Pending, surface as ErrInvoiceNotPending so the user sees a single
		// consistent message ("only pending invoices can be canceled") instead
		// of silently double-acting on the row.
		if err := model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusPending, model.InvoiceStatusCanceled, map[string]interface{}{}); err != nil {
			if errors.Is(err, model.ErrInvoiceStatusChanged) {
				return ErrInvoiceNotPending
			}
			return fmt.Errorf("更新发票状态失败: %w", err)
		}
		if err := model.DeleteInvoiceItemsByInvoiceId(tx, invoiceId); err != nil {
			return fmt.Errorf("删除关联记录失败: %w", err)
		}
		// custom: invoice fee — refund service fee on user-initiated cancel.
		// Only the tx that actually performs the refund (didRefund == true)
		// emits the cache update + audit log post-commit; concurrent retries
		// see didRefund == false and stay quiet.
		didRefund, err := refundInvoiceFee(tx, invoice)
		if err != nil {
			return err
		}
		if didRefund {
			refundedQuota = invoice.FeeQuota
			refundedAmount = invoice.FeeAmount
		}
		return nil
	})
	if err != nil {
		return err
	}
	if refundedQuota > 0 {
		model.SyncInvoiceFeeCacheIncr(userId, refundedQuota)
		model.RecordLog(userId, model.LogTypeRefund,
			fmt.Sprintf("开票服务费退还 ¥%.2f / quota %d，发票 #%d（用户撤销）",
				refundedAmount, refundedQuota, invoiceId))
	}
	return nil
}

// ---------------------------------------------------------------------------
// ReviewInvoice — 管理员审核发票
// ---------------------------------------------------------------------------

func ReviewInvoice(adminId, invoiceId int, req *dto.ReviewInvoiceRequest) error {
	var refundedUserId, refundedQuota int
	var refundedAmount float64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		invoice, err := model.GetInvoiceById(tx, invoiceId)
		if err != nil {
			return ErrInvoiceNotFound
		}
		if invoice.Status != model.InvoiceStatusPending {
			return ErrInvoiceNotPending
		}

		switch req.Action {
		case "approve":
			// CAS transition: if the user just canceled or another admin
			// already approved/rejected, the in-memory pre-check above can be
			// stale. Map a CAS miss to ErrInvoiceNotPending for consistent UX.
			err := model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusPending, model.InvoiceStatusApproved, map[string]interface{}{
				"admin_id": adminId,
			})
			if errors.Is(err, model.ErrInvoiceStatusChanged) {
				return ErrInvoiceNotPending
			}
			return err
		case "reject":
			// Same CAS guard as the approve branch; on miss, the user-facing
			// "only pending invoices can be reviewed" message is correct.
			if err := model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusPending, model.InvoiceStatusRejected, map[string]interface{}{
				"admin_id":      adminId,
				"reject_reason": req.RejectReason,
			}); err != nil {
				if errors.Is(err, model.ErrInvoiceStatusChanged) {
					return ErrInvoiceNotPending
				}
				return err
			}
			if err := model.DeleteInvoiceItemsByInvoiceId(tx, invoiceId); err != nil {
				return err
			}
			// custom: invoice fee — refund service fee on admin reject.
			// CAS flag inside refundInvoiceFee guarantees only one concurrent
			// reject performs the refund; losers see didRefund == false.
			didRefund, err := refundInvoiceFee(tx, invoice)
			if err != nil {
				return err
			}
			if didRefund {
				refundedUserId = invoice.UserId
				refundedQuota = invoice.FeeQuota
				refundedAmount = invoice.FeeAmount
			}
			return nil
		default:
			return errors.New("无效的审核操作")
		}
	})
	if err != nil {
		return err
	}
	if refundedQuota > 0 {
		model.SyncInvoiceFeeCacheIncr(refundedUserId, refundedQuota)
		model.RecordLog(refundedUserId, model.LogTypeRefund,
			fmt.Sprintf("开票服务费退还 ¥%.2f / quota %d，发票 #%d（管理员驳回）",
				refundedAmount, refundedQuota, invoiceId))
	}
	return nil
}

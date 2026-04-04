package service

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// 错误定义（集中管理，Controller 层统一映射）
// ---------------------------------------------------------------------------

var (
	ErrInvoiceDisabled      = errors.New("发票功能未启用")
	ErrAmountInsufficient   = errors.New("开票金额不足最低要求")
	ErrTopUpNotFound        = errors.New("充值记录不存在或不属于当前用户")
	ErrTopUpAlreadyInvoiced = errors.New("部分充值记录已被开票")
	ErrTopUpNotSuccess      = errors.New("只能对已完成的充值记录开票")
	ErrInvoiceNotPending    = errors.New("只能撤销待审核状态的发票")
	ErrInvoiceNotFound      = errors.New("发票申请不存在")
	ErrInvoiceNotOwner      = errors.New("无权操作此发票")
	ErrHeaderLimitExceeded  = errors.New("发票抬头数量已达上限")
	ErrInvalidTaxNumber     = errors.New("纳税人识别号格式不正确")
	ErrNoTopUpsSelected     = errors.New("请至少选择一条充值记录")
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
		return createInvoiceAndItems(tx, &invoice, userId, topups, content, req)
	})
	if err != nil {
		return nil, err
	}
	return &invoice, nil
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
	return model.DB.Transaction(func(tx *gorm.DB) error {
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

		if err := model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusCanceled, map[string]interface{}{}); err != nil {
			return fmt.Errorf("更新发票状态失败: %w", err)
		}
		if err := model.DeleteInvoiceItemsByInvoiceId(tx, invoiceId); err != nil {
			return fmt.Errorf("删除关联记录失败: %w", err)
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// ReviewInvoice — 管理员审核发票
// ---------------------------------------------------------------------------

func ReviewInvoice(adminId, invoiceId int, req *dto.ReviewInvoiceRequest) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		invoice, err := model.GetInvoiceById(tx, invoiceId)
		if err != nil {
			return ErrInvoiceNotFound
		}
		if invoice.Status != model.InvoiceStatusPending {
			return ErrInvoiceNotPending
		}

		switch req.Action {
		case "approve":
			return model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusApproved, map[string]interface{}{
				"admin_id": adminId,
			})
		case "reject":
			if err := model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusRejected, map[string]interface{}{
				"admin_id":      adminId,
				"reject_reason": req.RejectReason,
			}); err != nil {
				return err
			}
			return model.DeleteInvoiceItemsByInvoiceId(tx, invoiceId)
		default:
			return errors.New("无效的审核操作")
		}
	})
}

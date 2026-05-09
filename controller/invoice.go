package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ===========================================================================
// 用户侧
// ===========================================================================

// GetInvoiceSetting 获取发票配置（前端用于展示最低金额等）
func GetInvoiceSetting(c *gin.Context) {
	cfg := operation_setting.GetInvoiceSetting()
	common.ApiSuccess(c, gin.H{
		"enabled":         cfg.Enabled,
		"min_amount":      cfg.MinAmount,
		"default_content": cfg.DefaultContent,
		// custom: invoice fee
		"fee_rate": cfg.FeeRate,
	})
}

// GetAvailableTopUps 获取用户可开票的充值记录
func GetAvailableTopUps(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	topups, total, err := model.GetAvailableTopUpsForInvoice(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(topups)
	common.ApiSuccess(c, pageInfo)
}

// GetInvoicedTopUpIds 获取当前用户已开票的 topup ID 列表（前端用于标记）
func GetUserInvoicedTopUpIds(c *gin.Context) {
	userId := c.GetInt("id")
	ids, err := model.GetUserInvoicedTopUpIds(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ids)
}

// SubmitInvoice 提交发票申请
func SubmitInvoice(c *gin.Context) {
	userId := c.GetInt("id")
	var req dto.SubmitInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 清洗输入
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.TaxNumber = strings.TrimSpace(req.TaxNumber)
	req.Content = strings.TrimSpace(req.Content)
	req.Remark = strings.TrimSpace(req.Remark)

	invoice, err := service.SubmitInvoice(userId, &req)
	if err != nil {
		invoiceErrorResponse(c, err)
		return
	}
	common.ApiSuccess(c, invoice)
}

// GetUserInvoices 获取我的发票列表
func GetUserInvoices(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)

	invoices, total, err := model.GetUserInvoices(userId, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invoices)
	common.ApiSuccess(c, pageInfo)
}

// CancelInvoice 用户撤销待审核的发票
func CancelInvoice(c *gin.Context) {
	userId := c.GetInt("id")
	invoiceId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的发票ID")
		return
	}

	if err := service.CancelInvoice(userId, invoiceId); err != nil {
		invoiceErrorResponse(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// GetInvoiceItems 获取发票关联的充值记录
func GetInvoiceItems(c *gin.Context) {
	userId := c.GetInt("id")
	invoiceId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的发票ID")
		return
	}

	// 直接带 user_id 查询，避免泄露 ID 存在性且逻辑更简洁
	_, err = model.GetInvoiceByIdAndUserId(model.DB, invoiceId, userId)
	if err != nil {
		invoiceErrorResponse(c, service.ErrInvoiceNotFound)
		return
	}

	items, err := model.GetInvoiceItemsByInvoiceId(invoiceId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

// ===========================================================================
// 发票抬头模板
// ===========================================================================

// GetInvoiceHeaders 获取用户的抬头列表
func GetInvoiceHeaders(c *gin.Context) {
	userId := c.GetInt("id")
	headers, err := model.GetUserInvoiceHeaders(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, headers)
}

// CreateInvoiceHeader 新建抬头模板
func CreateInvoiceHeader(c *gin.Context) {
	userId := c.GetInt("id")
	var req dto.CreateInvoiceHeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 清洗输入
	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.TaxNumber = strings.TrimSpace(req.TaxNumber)

	if !dto.IsValidTaxNumber(req.TaxNumber) {
		common.ApiErrorMsg(c, "纳税人识别号格式不正确")
		return
	}

	// 检查数量限制
	cfg := operation_setting.GetInvoiceSetting()
	count, err := model.CountUserInvoiceHeaders(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= cfg.MaxHeaders {
		invoiceErrorResponse(c, service.ErrHeaderLimitExceeded)
		return
	}

	header := model.InvoiceHeader{
		UserId:      userId,
		CompanyName: req.CompanyName,
		TaxNumber:   req.TaxNumber,
		IsDefault:   req.IsDefault,
	}

	// 事务内：清除旧默认 + 创建新抬头
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if req.IsDefault {
			if err := model.ClearDefaultHeaders(tx, userId); err != nil {
				return err
			}
		}
		return model.CreateInvoiceHeaderTx(tx, &header)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, header)
}

// UpdateInvoiceHeader 编辑抬头模板
func UpdateInvoiceHeader(c *gin.Context) {
	userId := c.GetInt("id")
	headerId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的抬头ID")
		return
	}

	var req dto.UpdateInvoiceHeaderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	updates := make(map[string]interface{})
	if req.CompanyName != nil {
		updates["company_name"] = strings.TrimSpace(*req.CompanyName)
	}
	if req.TaxNumber != nil {
		taxNumber := strings.TrimSpace(*req.TaxNumber)
		if !dto.IsValidTaxNumber(taxNumber) {
			common.ApiErrorMsg(c, "纳税人识别号格式不正确")
			return
		}
		updates["tax_number"] = taxNumber
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := model.ClearDefaultHeaders(tx, userId); err != nil {
				return err
			}
		}
		return model.UpdateInvoiceHeaderTx(tx, userId, headerId, updates)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DeleteInvoiceHeader 删除抬头模板
func DeleteInvoiceHeader(c *gin.Context) {
	userId := c.GetInt("id")
	headerId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的抬头ID")
		return
	}

	if err := model.DeleteInvoiceHeader(userId, headerId); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// ===========================================================================
// 管理员侧
// ===========================================================================

// GetAllInvoicesAdmin 管理员获取所有发票申请
func GetAllInvoicesAdmin(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status, _ := strconv.Atoi(c.Query("status"))
	keyword := c.Query("keyword")

	invoices, total, err := model.GetAllInvoices(pageInfo, status, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invoices)
	common.ApiSuccess(c, pageInfo)
}

// ReviewInvoiceAdmin 管理员审核发票
func ReviewInvoiceAdmin(c *gin.Context) {
	adminId := c.GetInt("id")
	invoiceId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的发票ID")
		return
	}

	var req dto.ReviewInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误: "+err.Error())
		return
	}

	if err := service.ReviewInvoice(adminId, invoiceId, &req); err != nil {
		invoiceErrorResponse(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// MarkInvoiceSent 管理员标记发票已发送（仅 APPROVED 状态可操作）
func MarkInvoiceSent(c *gin.Context) {
	adminId := c.GetInt("id")
	invoiceId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的发票ID")
		return
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		invoice, err := model.GetInvoiceById(tx, invoiceId)
		if err != nil {
			return service.ErrInvoiceNotFound
		}
		if invoice.Status != model.InvoiceStatusApproved {
			return errors.New("只有已通过的发票可以标记为已发送")
		}
		// custom: invoice fee — CAS keeps the in-memory check honest under
		// concurrent admin actions; on miss surface as the same UX message.
		err = model.UpdateInvoiceStatus(tx, invoiceId, model.InvoiceStatusApproved, model.InvoiceStatusSent, map[string]interface{}{
			"admin_id": adminId,
		})
		if errors.Is(err, model.ErrInvoiceStatusChanged) {
			return errors.New("只有已通过的发票可以标记为已发送")
		}
		return err
	}); err != nil {
		invoiceErrorResponse(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// DeletePendingTopUp 用户删除自己的待支付充值记录
func DeletePendingTopUp(c *gin.Context) {
	userId := c.GetInt("id")
	topUpId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的订单ID")
		return
	}

	if err := model.DeletePendingTopUp(userId, topUpId); err != nil {
		common.ApiErrorMsg(c, "删除失败，订单不存在或状态不是待支付")
		return
	}
	common.ApiSuccess(c, nil)
}

// ===========================================================================
// 错误响应映射
// ===========================================================================

func invoiceErrorResponse(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvoiceDisabled),
		errors.Is(err, service.ErrAmountInsufficient),
		errors.Is(err, service.ErrTopUpNotFound),
		errors.Is(err, service.ErrTopUpNotSuccess),
		errors.Is(err, service.ErrNoTopUpsSelected),
		errors.Is(err, service.ErrInvalidTaxNumber),
		errors.Is(err, service.ErrInvoiceNotPending),
		errors.Is(err, service.ErrHeaderLimitExceeded),
		errors.Is(err, service.ErrInsufficientQuotaForFee): // custom: invoice fee
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})

	case errors.Is(err, service.ErrTopUpAlreadyInvoiced):
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})

	case errors.Is(err, service.ErrInvoiceNotFound):
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})

	case errors.Is(err, service.ErrInvoiceNotOwner):
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})

	default:
		common.SysError("invoice error: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "服务器内部错误"})
	}
}

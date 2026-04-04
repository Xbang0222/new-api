package dto

import "regexp"

// ---------------------------------------------------------------------------
// 发票申请
// ---------------------------------------------------------------------------

type SubmitInvoiceRequest struct {
	TopUpIds    []int  `json:"topup_ids" binding:"required,min=1"`
	Email       string `json:"email" binding:"required,email,max=100"`
	CompanyName string `json:"company_name" binding:"required,max=255"`
	TaxNumber   string `json:"tax_number" binding:"required,min=15,max=20"`
	Content     string `json:"content" binding:"max=255"`
	Remark      string `json:"remark" binding:"max=100"`
	HeaderId    *int   `json:"header_id"` // 可选，从模板填充
}

type ReviewInvoiceRequest struct {
	Action       string `json:"action" binding:"required,oneof=approve reject"`
	RejectReason string `json:"reject_reason" binding:"max=255"`
}

// ---------------------------------------------------------------------------
// 发票抬头模板
// ---------------------------------------------------------------------------

type CreateInvoiceHeaderRequest struct {
	CompanyName string `json:"company_name" binding:"required,max=255"`
	TaxNumber   string `json:"tax_number" binding:"required,min=15,max=20"`
	IsDefault   bool   `json:"is_default"`
}

type UpdateInvoiceHeaderRequest struct {
	CompanyName *string `json:"company_name" binding:"omitempty,max=255"`
	TaxNumber   *string `json:"tax_number" binding:"omitempty,min=15,max=20"`
	IsDefault   *bool   `json:"is_default"`
}

// ---------------------------------------------------------------------------
// 校验工具
// ---------------------------------------------------------------------------

// taxNumberRegex 统一社会信用代码：15-20 位大写字母+数字
var taxNumberRegex = regexp.MustCompile(`^[0-9A-Z]{15,20}$`)

func IsValidTaxNumber(s string) bool {
	return taxNumberRegex.MatchString(s)
}

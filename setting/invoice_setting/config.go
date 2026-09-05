package invoice_setting

import "github.com/QuantumNous/new-api/setting/config"

const (
	TaxBurdenIncluded   = "included"
	TaxBurdenSupplement = "supplement_by_customer"

	SupplementPaymentMethodEpay      = "epay"
	SupplementPaymentMethodBalance   = "balance"
	SupplementPaymentMethodOptionKey = "invoice_setting.supplement_payment_method"
)

// InvoiceSetting describes the tax-estimation policy used when an individual
// provides technical services to an enterprise in mainland China. The saved
// application snapshot is authoritative for that application; changing these
// defaults never rewrites an existing application.
type InvoiceSetting struct {
	Enabled                            bool    `json:"enabled"`
	SupplementPaymentMethod            string  `json:"supplement_payment_method"`
	TaxBurdenMode                      string  `json:"tax_burden_mode"`
	MinimumAmount                      float64 `json:"minimum_amount"`
	ApplicationWindowDays              int     `json:"application_window_days"`
	Currency                           string  `json:"currency"`
	InvoiceItemName                    string  `json:"invoice_item_name"`
	FeeRateBasisPoints                 int     `json:"fee_rate_basis_points"`
	VATThresholdCents                  int64   `json:"vat_threshold_cents"`
	VATRateBasisPoints                 int     `json:"vat_rate_basis_points"`
	VATStandardRateBasisPoints         int     `json:"vat_standard_rate_basis_points"`
	VATPreferentialEndDate             string  `json:"vat_preferential_end_date"`
	UrbanMaintenanceTaxRateBasisPoints int     `json:"urban_maintenance_tax_rate_basis_points"`
	EducationSurchargeRateBasisPoints  int     `json:"education_surcharge_rate_basis_points"`
	LocalEducationRateBasisPoints      int     `json:"local_education_rate_basis_points"`
	SurchargeReliefBasisPoints         int     `json:"surcharge_relief_basis_points"`
	PITWithholdingEnabled              bool    `json:"pit_withholding_enabled"`
	PolicyEffectiveDate                string  `json:"policy_effective_date"`
	PolicyNotice                       string  `json:"policy_notice"`
}

var invoiceSetting = InvoiceSetting{
	Enabled:                            false,
	SupplementPaymentMethod:            SupplementPaymentMethodEpay,
	TaxBurdenMode:                      TaxBurdenSupplement,
	MinimumAmount:                      0,
	ApplicationWindowDays:              365,
	Currency:                           "CNY",
	InvoiceItemName:                    "AI Agent服务",
	FeeRateBasisPoints:                 0,
	VATThresholdCents:                  100_000,
	VATRateBasisPoints:                 100,
	VATStandardRateBasisPoints:         300,
	VATPreferentialEndDate:             "2027-12-31",
	UrbanMaintenanceTaxRateBasisPoints: 700,
	EducationSurchargeRateBasisPoints:  300,
	LocalEducationRateBasisPoints:      200,
	SurchargeReliefBasisPoints:         5_000,
	PITWithholdingEnabled:              true,
	PolicyEffectiveDate:                "2026-01-01",
	PolicyNotice:                       "系统按个人向企业提供技术服务、订单金额为个人税后净收入的场景预估税费。客户承担模式会对已启用的增值税、附加税和个人所得税预扣额进行税前还原，并计入建议补款；实际税额、纳税地点、按次或按月口径及开票资格，以电子税务局代开结果、付款企业扣缴申报和完税凭证为准，管理员开票前必须复核。",
}

func init() {
	config.GlobalConfig.Register("invoice_setting", &invoiceSetting)
}

func GetInvoiceSetting() *InvoiceSetting {
	if invoiceSetting.SupplementPaymentMethod != SupplementPaymentMethodBalance {
		invoiceSetting.SupplementPaymentMethod = SupplementPaymentMethodEpay
	}
	if invoiceSetting.InvoiceItemName == "技术服务费" {
		invoiceSetting.InvoiceItemName = "AI Agent服务"
	}
	return &invoiceSetting
}

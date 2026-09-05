package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func invoiceTaxTestSetting() invoice_setting.InvoiceSetting {
	return invoice_setting.InvoiceSetting{
		Enabled:                            true,
		TaxBurdenMode:                      invoice_setting.TaxBurdenSupplement,
		MinimumAmount:                      0,
		ApplicationWindowDays:              30,
		Currency:                           "CNY",
		InvoiceItemName:                    "AI Agent服务",
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
	}
}

func TestCalculateInvoiceTaxVATThresholdUsesFullSalesAtThreshold(t *testing.T) {
	setting := invoiceTaxTestSetting()
	setting.TaxBurdenMode = invoice_setting.TaxBurdenIncluded
	calculationTime := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.Local)

	below, err := CalculateInvoiceTax(100_999, setting, calculationTime)
	require.NoError(t, err)
	atThreshold, err := CalculateInvoiceTax(101_000, setting, calculationTime)
	require.NoError(t, err)
	above, err := CalculateInvoiceTax(101_001, setting, calculationTime)
	require.NoError(t, err)

	assert.True(t, below.VATExemptByThreshold)
	assert.Zero(t, below.EstimatedVATCents)
	assert.False(t, atThreshold.VATExemptByThreshold)
	assert.Equal(t, int64(1_000), atThreshold.EstimatedVATCents)
	assert.Equal(t, int64(1_000), above.EstimatedVATCents)
	assert.Zero(t, atThreshold.SuggestedSupplementCents)
	assert.Equal(t, int64(35), atThreshold.EstimatedUrbanTaxCents)
	assert.Equal(t, int64(15), atThreshold.EstimatedEducationSurchargeCents)
	assert.Equal(t, int64(10), atThreshold.EstimatedLocalEducationCents)
	assert.Equal(t, int64(4_000), atThreshold.EstimatedPITWithholdingCents)
}

func TestCalculateInvoiceTaxGrossesUpAllEnabledTaxesForSellerNetIncome(t *testing.T) {
	setting := invoiceTaxTestSetting()
	calculationTime := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.Local)

	estimate, err := CalculateInvoiceTax(200_000, setting, calculationTime)
	require.NoError(t, err)

	assert.Equal(t, int64(230_174), estimate.TaxableSalesCents)
	assert.Equal(t, int64(2_302), estimate.EstimatedVATCents)
	assert.Equal(t, int64(81), estimate.EstimatedUrbanTaxCents)
	assert.Equal(t, int64(35), estimate.EstimatedEducationSurchargeCents)
	assert.Equal(t, int64(23), estimate.EstimatedLocalEducationCents)
	assert.Equal(t, int64(30_035), estimate.EstimatedPITWithholdingCents)
	assert.Equal(t, int64(32_476), estimate.EstimatedTotalTaxCents)
	assert.Equal(t, int64(32_476), estimate.SuggestedSupplementCents)
	assert.Equal(t, int64(200_000), 200_000+estimate.SuggestedSupplementCents-estimate.EstimatedTotalTaxCents)
}

func TestDeleteInvoiceApplicationRemovesApplicationAndLinkedRecords(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.InvoiceApplication{},
		&model.InvoiceOrder{},
		&model.InvoicePaymentOrder{},
	))

	application := model.InvoiceApplication{
		UserId:        702,
		Status:        model.InvoiceStatusPendingPayment,
		PaymentStatus: model.InvoicePaymentPending,
		InvoiceTitle:  "Delete test",
		Currency:      "CNY",
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)

	directory := filepath.Join("upload", "invoices", strconv.Itoa(application.Id))
	require.NoError(t, os.MkdirAll(directory, 0755))
	invoicePath := filepath.Join(directory, "invoice-delete-test.pdf")
	require.NoError(t, os.WriteFile(invoicePath, []byte("invoice"), 0644))
	require.NoError(t, model.DB.Model(&application).Update("invoice_file_path", invoicePath).Error)

	linkedOrder := model.InvoiceOrder{
		ApplicationId:   application.Id,
		TopUpId:         990_000 + application.Id,
		TradeNo:         fmt.Sprintf("invoice-delete-order-%d", application.Id),
		PaidAmountCents: 10_000,
	}
	require.NoError(t, model.DB.Create(&linkedOrder).Error)
	paymentOrder := model.InvoicePaymentOrder{
		ApplicationId: application.Id,
		UserId:        application.UserId,
		TradeNo:       fmt.Sprintf("invoice-delete-payment-%d", application.Id),
		AmountCents:   100,
		Currency:      "CNY",
		Status:        model.InvoicePaymentOrderFailed,
	}
	require.NoError(t, model.DB.Create(&paymentOrder).Error)
	t.Cleanup(func() {
		model.DB.Where("id = ?", application.Id).Delete(&model.InvoiceApplication{})
		model.DB.Where("application_id = ?", application.Id).Delete(&model.InvoiceOrder{})
		model.DB.Where("application_id = ?", application.Id).Delete(&model.InvoicePaymentOrder{})
		_ = os.Remove(invoicePath)
		_ = os.Remove(directory)
	})

	deleted, err := DeleteInvoiceApplication(application.Id)
	require.NoError(t, err)
	assert.Equal(t, application.Id, deleted.Id)

	var applicationCount int64
	require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Where("id = ?", application.Id).Count(&applicationCount).Error)
	assert.Zero(t, applicationCount)
	var orderCount int64
	require.NoError(t, model.DB.Model(&model.InvoiceOrder{}).Where("application_id = ?", application.Id).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
	var paymentCount int64
	require.NoError(t, model.DB.Model(&model.InvoicePaymentOrder{}).Where("application_id = ?", application.Id).Count(&paymentCount).Error)
	assert.Zero(t, paymentCount)
	_, statErr := os.Stat(invoicePath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestDeleteInvoiceApplicationProtectsIssuedRecords(t *testing.T) {
	application := model.InvoiceApplication{
		UserId:        703,
		Status:        model.InvoiceStatusIssued,
		PaymentStatus: model.InvoicePaymentPaid,
		InvoiceTitle:  "Issued invoice",
		Currency:      "CNY",
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)
	t.Cleanup(func() {
		model.DB.Where("id = ?", application.Id).Delete(&model.InvoiceApplication{})
	})

	deleted, err := DeleteInvoiceApplication(application.Id)
	assert.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
	assert.Nil(t, deleted)

	var applicationCount int64
	require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Where("id = ?", application.Id).Count(&applicationCount).Error)
	assert.Equal(t, int64(1), applicationCount)
}

func TestDeleteInvoiceApplicationProtectsActiveSupplementPayments(t *testing.T) {
	application := model.InvoiceApplication{
		UserId:        704,
		Status:        model.InvoiceStatusPendingPayment,
		PaymentStatus: model.InvoicePaymentPending,
		InvoiceTitle:  "Pending supplement",
		Currency:      "CNY",
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)
	paymentOrder := model.InvoicePaymentOrder{
		ApplicationId: application.Id,
		UserId:        application.UserId,
		TradeNo:       fmt.Sprintf("invoice-active-payment-%d", application.Id),
		AmountCents:   100,
		Currency:      "CNY",
		Status:        model.InvoicePaymentOrderPending,
	}
	require.NoError(t, model.DB.Create(&paymentOrder).Error)
	t.Cleanup(func() {
		model.DB.Where("application_id = ?", application.Id).Delete(&model.InvoicePaymentOrder{})
		model.DB.Where("id = ?", application.Id).Delete(&model.InvoiceApplication{})
	})

	deleted, err := DeleteInvoiceApplication(application.Id)
	assert.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
	assert.Nil(t, deleted)
}

func TestInvoiceReviewDoesNotCreateSupplementPayment(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.InvoicePaymentOrder{}))
	setting := invoice_setting.GetInvoiceSetting()
	original := *setting
	*setting = invoiceTaxTestSetting()
	t.Cleanup(func() {
		*setting = original
		model.LOG_DB.Where("request_id = ?", "invoice-payment-test").Delete(&model.Log{})
		model.DB.Where("trade_no = ?", "invoice-payment-test").Delete(&model.InvoicePaymentOrder{})
		model.DB.Exec("DELETE FROM invoice_orders")
		model.DB.Exec("DELETE FROM invoice_applications")
		model.DB.Where("trade_no LIKE ?", "invoice-test-%").Delete(&model.TopUp{})
	})

	now := time.Now().Unix()
	paidOrder := model.TopUp{
		UserId:        701,
		Money:         1000,
		TradeNo:       "invoice-test-paid",
		PaymentMethod: "stripe",
		CreateTime:    now - 60,
		CompleteTime:  now,
		Status:        common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(&paidOrder).Error)

	application, err := CreateInvoiceApplication(701, CreateInvoiceApplicationInput{
		TopUpIds:       []int{paidOrder.Id},
		InvoiceTitle:   "测试企业",
		TaxNumber:      "91310000TEST",
		RecipientEmail: "finance@example.com",
		ApplicantNote:  "Please include the project name.",
	})
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusPendingReview, application.Status)
	assert.Zero(t, application.SuggestedSupplementCents)
	assert.Zero(t, application.EstimatedPITCents)
	assert.Equal(t, "AI Agent服务", application.InvoiceItemName)
	assert.Equal(t, "Please include the project name.", application.ApplicantNote)

	require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, nil, "", "", "reviewed"))
	reviewed, err := model.GetInvoiceApplication(application.Id, 701)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusApproved, reviewed.Status)
	assert.Equal(t, model.InvoicePaymentPaid, reviewed.PaymentStatus)
	assert.Equal(t, application.OrderAmountCents, reviewed.InvoiceAmountCents)

	_, err = CreateInvoicePaymentOrder(application.Id, 701, "invoice-payment-test", "alipay", model.PaymentProviderEpay)
	require.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
}

func TestInvoiceSupplementBalancePaymentChecksAndDeductsWalletAtomically(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.InvoiceApplication{}, &model.InvoicePaymentOrder{}))
	setting := invoice_setting.GetInvoiceSetting()
	originalSetting := *setting
	originalQuotaPerUnit := common.QuotaPerUnit
	originalExchangeRate := operation_setting.USDExchangeRate
	setting.SupplementPaymentMethod = invoice_setting.SupplementPaymentMethodBalance
	common.QuotaPerUnit = 500_000
	operation_setting.USDExchangeRate = 7.3

	user := model.User{Username: fmt.Sprintf("invoice-balance-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("invoice-balance-code-%d", time.Now().UnixNano()), Quota: 500_000}
	require.NoError(t, model.DB.Create(&user).Error)
	application := model.InvoiceApplication{
		UserId: user.Id, Status: model.InvoiceStatusPendingPayment,
		PaymentStatus: model.InvoicePaymentPending, FinalSupplementCents: 730,
		Currency: "CNY", CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)

	t.Cleanup(func() {
		*setting = originalSetting
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.USDExchangeRate = originalExchangeRate
		model.LOG_DB.Where("request_id LIKE ?", fmt.Sprintf("INVBALUSR%dNO%%", user.Id)).Delete(&model.Log{})
		model.DB.Where("application_id = ?", application.Id).Delete(&model.InvoicePaymentOrder{})
		model.DB.Delete(&model.InvoiceApplication{}, application.Id)
		model.DB.Delete(&model.User{}, user.Id)
	})

	order, err := PayInvoiceSupplementWithBalance(application.Id, user.Id)
	require.NoError(t, err)
	assert.Equal(t, model.PaymentProviderBalance, order.PaymentProvider)
	assert.Equal(t, "charged_quota=500000", order.ProviderPayload)

	quota, err := model.GetUserQuota(user.Id, true)
	require.NoError(t, err)
	assert.Zero(t, quota)
	paid, err := model.GetInvoiceApplication(application.Id, user.Id)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusApproved, paid.Status)
	assert.Equal(t, model.InvoicePaymentPaid, paid.PaymentStatus)
	assert.Equal(t, model.PaymentMethodBalance, paid.PaymentMethod)

	_, err = PayInvoiceSupplementWithBalance(application.Id, user.Id)
	assert.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
	quota, err = model.GetUserQuota(user.Id, true)
	require.NoError(t, err)
	assert.Zero(t, quota, "a repeated payment must not deduct the wallet twice")
}

func TestInvoiceSupplementBalancePaymentLeavesInvoicePendingWhenBalanceIsInsufficient(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.InvoiceApplication{}, &model.InvoicePaymentOrder{}))
	setting := invoice_setting.GetInvoiceSetting()
	originalSetting := *setting
	originalQuotaPerUnit := common.QuotaPerUnit
	originalExchangeRate := operation_setting.USDExchangeRate
	setting.SupplementPaymentMethod = invoice_setting.SupplementPaymentMethodBalance
	common.QuotaPerUnit = 500_000
	operation_setting.USDExchangeRate = 7.3

	user := model.User{Username: fmt.Sprintf("invoice-low-balance-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("invoice-low-balance-code-%d", time.Now().UnixNano()), Quota: 499_999}
	require.NoError(t, model.DB.Create(&user).Error)
	application := model.InvoiceApplication{
		UserId: user.Id, Status: model.InvoiceStatusPendingPayment,
		PaymentStatus: model.InvoicePaymentPending, FinalSupplementCents: 730,
		Currency: "CNY", CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)
	t.Cleanup(func() {
		*setting = originalSetting
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.USDExchangeRate = originalExchangeRate
		model.DB.Delete(&model.InvoiceApplication{}, application.Id)
		model.DB.Delete(&model.User{}, user.Id)
	})

	_, err := PayInvoiceSupplementWithBalance(application.Id, user.Id)
	assert.ErrorIs(t, err, model.ErrWalletQuotaInsufficient)
	quota, err := model.GetUserQuota(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 499_999, quota)
	pending, err := model.GetInvoiceApplication(application.Id, user.Id)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusPendingPayment, pending.Status)
	assert.Equal(t, model.InvoicePaymentPending, pending.PaymentStatus)
}

func TestCreateInvoiceApplicationRejectsApplicantNoteOverLimit(t *testing.T) {
	setting := invoice_setting.GetInvoiceSetting()
	original := *setting
	*setting = invoiceTaxTestSetting()
	t.Cleanup(func() {
		*setting = original
	})

	_, err := CreateInvoiceApplication(702, CreateInvoiceApplicationInput{
		TopUpIds:      []int{1},
		InvoiceTitle:  "Example Technology Co., Ltd.",
		TaxNumber:     "91310000EXAMPLE",
		ApplicantNote: strings.Repeat("a", 2001),
	})

	require.ErrorContains(t, err, "2000 characters or fewer")
}

func TestInvoiceApplicationRejectsMinimumAndExpiredOrders(t *testing.T) {
	setting := invoice_setting.GetInvoiceSetting()
	original := *setting
	rules := invoiceTaxTestSetting()
	rules.TaxBurdenMode = invoice_setting.TaxBurdenIncluded
	rules.MinimumAmount = 100
	*setting = rules
	t.Cleanup(func() {
		*setting = original
		model.DB.Exec("DELETE FROM invoice_orders")
		model.DB.Exec("DELETE FROM invoice_applications")
		model.DB.Where("trade_no LIKE ?", "invoice-boundary-%").Delete(&model.TopUp{})
	})

	now := time.Now()
	smallOrder := model.TopUp{UserId: 703, Money: 99, TradeNo: "invoice-boundary-small", PaymentMethod: "stripe", CreateTime: now.Unix(), CompleteTime: now.Unix(), Status: common.TopUpStatusSuccess}
	expiredOrder := model.TopUp{UserId: 703, Money: 200, TradeNo: "invoice-boundary-expired", PaymentMethod: "stripe", CreateTime: now.AddDate(0, 0, -31).Unix(), CompleteTime: now.AddDate(0, 0, -31).Unix(), Status: common.TopUpStatusSuccess}
	require.NoError(t, model.DB.Create(&smallOrder).Error)
	require.NoError(t, model.DB.Create(&expiredOrder).Error)

	_, err := CreateInvoiceApplication(703, CreateInvoiceApplicationInput{TopUpIds: []int{smallOrder.Id}, InvoiceTitle: "测试企业", TaxNumber: "91310000TEST", RecipientEmail: "finance@example.com"})
	require.ErrorContains(t, err, "minimum invoice amount")
	_, err = CreateInvoiceApplication(703, CreateInvoiceApplicationInput{TopUpIds: []int{expiredOrder.Id}, InvoiceTitle: "测试企业", TaxNumber: "91310000TEST", RecipientEmail: "finance@example.com"})
	require.ErrorContains(t, err, "outside the invoice application window")
}

func TestResolveInvoiceFilePathRejectsPathOutsideInvoiceDirectory(t *testing.T) {
	_, err := resolveInvoiceFilePath(filepath.Join("upload", "outside.pdf"))
	require.ErrorContains(t, err, "invalid invoice file path")
	path, err := resolveInvoiceFilePath(filepath.Join("upload", "invoices", "1", "invoice.pdf"))
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(path))
}

func TestInvoiceOrderCanOnlyBeAppliedOnce(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.InvoiceApplication{}, &model.InvoiceOrder{}))
	setting := invoice_setting.GetInvoiceSetting()
	original := *setting
	*setting = invoiceTaxTestSetting()
	t.Cleanup(func() {
		*setting = original
	})

	now := time.Now().Unix()
	paidOrder := model.TopUp{
		UserId:        704,
		Money:         200,
		TradeNo:       "invoice-unique-order",
		PaymentMethod: "stripe",
		CreateTime:    now,
		CompleteTime:  now,
		Status:        common.TopUpStatusSuccess,
	}
	require.NoError(t, model.DB.Create(&paidOrder).Error)
	t.Cleanup(func() {
		model.DB.Where("top_up_id = ?", paidOrder.Id).Delete(&model.InvoiceOrder{})
		model.DB.Where("user_id = ?", 704).Delete(&model.InvoiceApplication{})
		model.DB.Delete(&model.TopUp{}, paidOrder.Id)
	})
	input := CreateInvoiceApplicationInput{
		TopUpIds:       []int{paidOrder.Id},
		InvoiceTitle:   "Example Technology Co., Ltd.",
		TaxNumber:      "91310000UNIQUE",
		RecipientEmail: "finance@example.com",
	}

	_, err := CreateInvoiceApplication(704, input)
	require.NoError(t, err)
	_, err = CreateInvoiceApplication(704, input)
	require.ErrorIs(t, err, model.ErrInvoiceOrderAlreadyApplied)

	eligible, err := ListEligibleInvoiceOrders(704)
	require.NoError(t, err)
	for _, order := range eligible {
		assert.NotEqual(t, paidOrder.Id, order.Id)
	}
}

func TestInvoiceApplicationChargesSnapshottedFeeAndRejectRefundsOnce(t *testing.T) {
	setting := invoice_setting.GetInvoiceSetting()
	originalSetting := *setting
	originalQuotaPerUnit := common.QuotaPerUnit
	originalDisplay := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalRate := operation_setting.USDExchangeRate
	*setting = invoiceTaxTestSetting()
	setting.FeeRateBasisPoints = 300
	common.QuotaPerUnit = 500_000
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.USDExchangeRate = 7.5
	t.Cleanup(func() {
		*setting = originalSetting
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplay
		operation_setting.USDExchangeRate = originalRate
	})

	user := model.User{Username: fmt.Sprintf("invoice-fee-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("invoice-fee-code-%d", time.Now().UnixNano()), Quota: 200_000}
	require.NoError(t, model.DB.Create(&user).Error)
	order := model.TopUp{UserId: user.Id, Money: 50, TradeNo: fmt.Sprintf("invoice-fee-order-%d", time.Now().UnixNano()),
		PaymentMethod: "alipay", Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix(), CompleteTime: time.Now().Unix()}
	require.NoError(t, model.DB.Create(&order).Error)
	t.Cleanup(func() {
		model.DB.Where("top_up_id = ?", order.Id).Delete(&model.InvoiceOrder{})
		model.DB.Where("user_id = ?", user.Id).Delete(&model.InvoiceApplication{})
		model.DB.Delete(&model.TopUp{}, order.Id)
		model.DB.Delete(&model.User{}, user.Id)
	})

	application, err := CreateInvoiceApplication(user.Id, CreateInvoiceApplicationInput{TopUpIds: []int{order.Id}, InvoiceTitle: "测试企业", TaxNumber: "91310000FEE", RecipientEmail: "finance@example.com"})
	require.NoError(t, err)
	assert.Equal(t, int64(5_000), application.InvoiceAmountCents)
	assert.Equal(t, int64(150), application.FeeAmountCents)
	assert.Equal(t, 100_000, application.FeeQuota)
	assert.Equal(t, 500_000.0, application.QuotaPerUnitSnapshot)
	assert.Equal(t, 7.5, application.ExchangeRateSnapshot)

	var charged model.User
	require.NoError(t, model.DB.First(&charged, user.Id).Error)
	assert.Equal(t, 100_000, charged.Quota)

	common.QuotaPerUnit = 1
	operation_setting.USDExchangeRate = 1
	require.NoError(t, ReviewInvoiceApplication(application.Id, 1, false, nil, "", "invalid", ""))
	require.ErrorIs(t, ReviewInvoiceApplication(application.Id, 1, false, nil, "", "again", ""), model.ErrInvoiceStatusInvalid)
	require.NoError(t, model.DB.First(&charged, user.Id).Error)
	assert.Equal(t, 200_000, charged.Quota)
}

func TestInvoiceApplicationFailsWhenFeeBalanceIsInsufficient(t *testing.T) {
	setting := invoice_setting.GetInvoiceSetting()
	original := *setting
	*setting = invoiceTaxTestSetting()
	setting.FeeRateBasisPoints = 300
	t.Cleanup(func() { *setting = original })

	user := model.User{Username: fmt.Sprintf("invoice-low-balance-%d", time.Now().UnixNano()), AffCode: fmt.Sprintf("invoice-low-code-%d", time.Now().UnixNano()), Quota: 0}
	require.NoError(t, model.DB.Create(&user).Error)
	config := GetInvoiceConfig()
	order := model.TopUp{UserId: user.Id, Money: 50, TradeNo: fmt.Sprintf("invoice-low-order-%d", time.Now().UnixNano()),
		PaymentMethod: "stripe", Status: common.TopUpStatusSuccess, CreateTime: time.Now().Unix(), CompleteTime: time.Now().Unix(),
		SettlementCurrency: config.Currency, SettlementExchangeRate: config.ExchangeRate, SettlementQuotaPerUnit: config.QuotaPerUnit}
	require.NoError(t, model.DB.Create(&order).Error)
	t.Cleanup(func() { model.DB.Delete(&model.TopUp{}, order.Id); model.DB.Delete(&model.User{}, user.Id) })

	_, err := CreateInvoiceApplication(user.Id, CreateInvoiceApplicationInput{TopUpIds: []int{order.Id}, InvoiceTitle: "测试企业", TaxNumber: "91310000LOW", RecipientEmail: "finance@example.com"})
	require.ErrorIs(t, err, model.ErrWalletQuotaInsufficient)
}

func TestSendInvoiceEmailEnforcesOwnershipAndTracksResends(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.InvoiceApplication{}))
	directory := filepath.Join("upload", "invoices", "service-email-test")
	require.NoError(t, os.MkdirAll(directory, 0755))
	filePath := filepath.Join(directory, "invoice.pdf")
	payload := []byte("%PDF-1.4\n%invoice-content\n")
	require.NoError(t, os.WriteFile(filePath, payload, 0600))
	t.Cleanup(func() {
		_ = os.RemoveAll(directory)
	})

	application := model.InvoiceApplication{
		UserId:                 705,
		Status:                 model.InvoiceStatusApproved,
		PaymentStatus:          model.InvoicePaymentNotRequired,
		InvoiceTitle:           "Example Technology Co., Ltd.",
		InvoiceItemName:        "AI Agent服务",
		RecipientEmail:         "finance@example.com",
		InvoiceFileName:        "invoice.pdf",
		InvoiceFilePath:        filePath,
		InvoiceFileContentType: "application/pdf",
		CreatedAt:              time.Now().Unix(),
		UpdatedAt:              time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)
	t.Cleanup(func() {
		model.DB.Delete(&model.InvoiceApplication{}, application.Id)
	})

	originalSender := invoiceEmailSender
	sendCount := 0
	invoiceEmailSender = func(subject string, receiver string, content string, attachments []common.EmailAttachment) error {
		sendCount++
		assert.Equal(t, fmt.Sprintf("发票 #%d", application.Id), subject)
		assert.Equal(t, "finance@example.com", receiver)
		assert.Contains(t, content, "您的发票已随邮件附上")
		assert.Contains(t, content, "AI Agent服务")
		require.Len(t, attachments, 1)
		assert.Equal(t, payload, attachments[0].Data)
		return nil
	}
	t.Cleanup(func() {
		invoiceEmailSender = originalSender
	})

	_, err := SendInvoiceEmail(application.Id, 706)
	require.ErrorIs(t, err, model.ErrInvoiceNotFound)
	assert.Zero(t, sendCount)
	_, err = SendInvoiceEmail(application.Id, 705)
	require.True(t, errors.Is(err, model.ErrInvoiceStatusInvalid))
	assert.Zero(t, sendCount)

	issued, err := SendInvoiceEmail(application.Id, 0)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusIssued, issued.Status)
	assert.NotZero(t, issued.InvoiceEmailSentAt)
	assert.Equal(t, 1, issued.InvoiceEmailSendCount)

	resent, err := SendInvoiceEmail(application.Id, 705)
	require.NoError(t, err)
	assert.Equal(t, 2, resent.InvoiceEmailSendCount)
	assert.Equal(t, 2, sendCount)
}

func TestInvoiceEmailMessageUsesSupportedLanguage(t *testing.T) {
	application := &model.InvoiceApplication{
		Id:              42,
		InvoiceTitle:    "Example & Company",
		InvoiceItemName: "AI Agent服务",
	}
	tests := []struct {
		language string
		subject  string
		fragment string
	}{
		{language: "en", subject: "Invoice #42", fragment: "Your invoice is attached."},
		{language: "zh", subject: "发票 #42", fragment: "您的发票已随邮件附上。"},
		{language: "zh-TW", subject: "發票 #42", fragment: "您的發票已隨郵件附上。"},
		{language: "fr", subject: "Facture n° 42", fragment: "Votre facture est jointe"},
		{language: "ja", subject: "請求書 #42", fragment: "請求書を添付しました。"},
		{language: "ru", subject: "Счёт-фактура №42", fragment: "Счёт-фактура прикреплён"},
		{language: "vi", subject: "Hóa đơn #42", fragment: "Hóa đơn của bạn"},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			subject, content := invoiceEmailMessage(test.language, application)
			assert.Equal(t, test.subject, subject)
			assert.Contains(t, content, test.fragment)
			assert.Contains(t, content, "Example &amp; Company")
			assert.Contains(t, content, "AI Agent服务")
		})
	}
}

func TestInvoiceEmailMessageUsesCurrentItemNameForLegacyUnissuedApplication(t *testing.T) {
	application := &model.InvoiceApplication{
		Id:              43,
		Status:          model.InvoiceStatusApproved,
		InvoiceTitle:    "Example Company",
		InvoiceItemName: "技术服务费",
	}

	_, content := invoiceEmailMessage("zh", application)

	assert.Contains(t, content, "AI Agent服务")
	assert.NotContains(t, content, "技术服务费")
}

func TestUploadInvoiceFileKeepsFileWhenInitialEmailFails(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.InvoiceApplication{}))
	application := model.InvoiceApplication{
		UserId:         707,
		Status:         model.InvoiceStatusApproved,
		PaymentStatus:  model.InvoicePaymentNotRequired,
		InvoiceTitle:   "Example Technology Co., Ltd.",
		RecipientEmail: "finance@example.com",
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&application).Error)
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join("upload", "invoices", strconv.Itoa(application.Id)))
		model.DB.Delete(&model.InvoiceApplication{}, application.Id)
	})

	originalSender := invoiceEmailSender
	invoiceEmailSender = func(_ string, _ string, _ string, _ []common.EmailAttachment) error {
		return errors.New("SMTP unavailable")
	}
	t.Cleanup(func() {
		invoiceEmailSender = originalSender
	})

	stored, err := UploadInvoiceFile(application.Id, "invoice.pdf", []byte("%PDF-1.4\n%test invoice\n"))
	require.ErrorContains(t, err, "file was saved")
	require.NotNil(t, stored)
	assert.Equal(t, model.InvoiceStatusApproved, stored.Status)
	assert.Equal(t, "invoice.pdf", stored.InvoiceFileName)
	_, statErr := os.Stat(stored.InvoiceFilePath)
	require.NoError(t, statErr)
	adminFile, err := GetInvoiceFileContent(application.Id)
	require.NoError(t, err)
	assert.Equal(t, "invoice.pdf", adminFile.FileName)
	assert.Equal(t, "application/pdf", adminFile.ContentType)
	assert.Equal(t, []byte("%PDF-1.4\n%test invoice\n"), adminFile.Data)

	invoiceEmailSender = func(_ string, _ string, _ string, _ []common.EmailAttachment) error {
		return nil
	}
	issued, err := SendInvoiceEmail(application.Id, 0)
	require.NoError(t, err)
	assert.Equal(t, model.InvoiceStatusIssued, issued.Status)
	assert.Equal(t, 1, issued.InvoiceEmailSendCount)
	issuedFile, err := GetInvoiceFileContent(application.Id)
	require.NoError(t, err)
	assert.Equal(t, adminFile.Data, issuedFile.Data)
}

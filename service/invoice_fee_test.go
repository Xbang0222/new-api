package service

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvoiceFeeSettings(t *testing.T) {
	t.Helper()
	originalInvoice := *invoice_setting.GetInvoiceSetting()
	originalGeneral := *operation_setting.GetGeneralSetting()
	originalQuotaPerUnit, originalExchangeRate := common.QuotaPerUnit, operation_setting.USDExchangeRate
	*invoice_setting.GetInvoiceSetting() = invoice_setting.InvoiceSetting{
		Enabled: true, InvoiceItemName: "技术服务费", FeeRateBasisPoints: 300,
	}
	*operation_setting.GetGeneralSetting() = operation_setting.GeneralSetting{
		QuotaDisplayType:     operation_setting.QuotaDisplayTypeCNY,
		CustomCurrencySymbol: "€", CustomCurrencyExchangeRate: 2,
	}
	common.QuotaPerUnit = 500_000
	operation_setting.USDExchangeRate = 7.5
	t.Cleanup(func() {
		*invoice_setting.GetInvoiceSetting() = originalInvoice
		*operation_setting.GetGeneralSetting() = originalGeneral
		common.QuotaPerUnit, operation_setting.USDExchangeRate = originalQuotaPerUnit, originalExchangeRate
	})
}

func TestInvoiceFeeCalculationAndBounds(t *testing.T) {
	setupInvoiceFeeSettings(t)
	tests := []struct {
		name      string
		amount    int64
		rate      int
		quotaUnit float64
		exchange  float64
		wantFee   int64
		wantQuota int
		invalid   bool
	}{
		{name: "50 at 3 percent", amount: 5_000, rate: 300, quotaUnit: 500_000, exchange: 7.5, wantFee: 150, wantQuota: 100_000},
		{name: "zero fee", amount: 5_000, quotaUnit: 500_000, exchange: 7.5},
		{name: "100 percent", amount: 5_000, rate: 10_000, quotaUnit: 500_000, exchange: 1, wantFee: 5_000, wantQuota: 25_000_000},
		{name: "fee half cent rounds up", amount: 50, rate: 100, quotaUnit: 500_000, exchange: 7.5, wantFee: 1, wantQuota: 667},
		{name: "fee below half cent", amount: 49, rate: 100, quotaUnit: 500_000, exchange: 7.5},
		{name: "fractional quota rounds up", amount: 5_000, rate: 300, quotaUnit: 500_000, exchange: 7.3, wantFee: 150, wantQuota: 102_740},
		{name: "tiny positive conversion", amount: 100, rate: 10_000, quotaUnit: 1e-20, exchange: 1, wantFee: 100, wantQuota: 1},
		{name: "amount exceeds bound", amount: maxInvoiceAmountCents + 1, rate: 300, quotaUnit: 500_000, exchange: 1, invalid: true},
		{name: "negative rate", amount: 5_000, rate: -1, quotaUnit: 500_000, exchange: 1, invalid: true},
		{name: "rate above 100 percent", amount: 5_000, rate: 10_001, quotaUnit: 500_000, exchange: 1, invalid: true},
		{name: "wallet overflow", amount: maxInvoiceAmountCents, rate: 10_000, quotaUnit: 1e20, exchange: 1, invalid: true},
		{name: "NaN quota", amount: 5_000, rate: 300, quotaUnit: math.NaN(), exchange: 1, invalid: true},
		{name: "infinite exchange", amount: 5_000, rate: 300, quotaUnit: 500_000, exchange: math.Inf(1), invalid: true},
		{name: "zero exchange", amount: 5_000, rate: 300, quotaUnit: 500_000, invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fee, quota, err := calculateInvoiceFee(test.amount, invoiceFeePolicy{
				FeeRateBasisPoints: test.rate, QuotaPerUnit: test.quotaUnit, USDToCurrencyRate: test.exchange,
			})
			if test.invalid {
				require.Error(t, err)
				assert.Zero(t, fee)
				assert.Zero(t, quota)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantFee, fee)
			assert.Equal(t, test.wantQuota, quota)
		})
	}
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), math.MaxFloat64, 1e100} {
		_, err := invoiceMoneyCents(amount)
		assert.Error(t, err)
	}
	cents, err := invoiceMoneyCents(50.005)
	require.NoError(t, err)
	assert.Equal(t, int64(5_001), cents)
}

func TestInvoicePublicConfigUsesSystemCurrency(t *testing.T) {
	setupInvoiceFeeSettings(t)
	for _, test := range []struct {
		mode     string
		currency string
		symbol   string
		quota    float64
	}{
		{mode: "USD", currency: "USD", symbol: "$", quota: 500_000},
		{mode: "CNY", currency: "CNY", symbol: "¥", quota: 500_000 / 7.5},
		{mode: "CUSTOM", currency: "CUSTOM", symbol: "€", quota: 250_000},
		{mode: "TOKENS", currency: "USD", symbol: "$", quota: 500_000},
	} {
		t.Run(test.mode, func(t *testing.T) {
			operation_setting.GetGeneralSetting().QuotaDisplayType = test.mode
			config := GetInvoiceConfig()
			assert.True(t, config.Enabled)
			assert.Equal(t, test.currency, config.Currency)
			assert.Equal(t, test.symbol, config.CurrencySymbol)
			assert.Equal(t, test.quota, config.QuotaPerCurrencyUnit)
			assert.Equal(t, 300, config.FeeRateBasisPoints)
			assert.Equal(t, "技术服务费", config.InvoiceItemName)
			assert.Equal(t, "balance", config.SupplementPaymentMethod)
		})
	}
	invoice_setting.GetInvoiceSetting().MinimumAmount = math.NaN()
	assert.False(t, GetInvoiceConfig().Enabled)
	_, err := CreateInvoiceApplication(1, CreateInvoiceApplicationInput{})
	require.ErrorContains(t, err, "minimum invoice amount is invalid")
	invoice_setting.GetInvoiceSetting().MinimumAmount = 0
	operation_setting.GetGeneralSetting().QuotaDisplayType = "CUSTOM"
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = math.NaN()
	assert.False(t, GetInvoiceConfig().Enabled)
}

func createInvoiceFeeFixture(t *testing.T, quota int, money float64) (model.User, model.TopUp, CreateInvoiceApplicationInput) {
	t.Helper()
	user := model.User{Username: fmt.Sprintf("invoice-fee-%d", time.Now().UnixNano()), AffCode: common.GetRandomString(12), Quota: quota}
	require.NoError(t, model.DB.Create(&user).Error)
	now := time.Now().Unix()
	paidAmount, amountErr := invoiceMoneyCents(money)
	if amountErr != nil {
		paidAmount = maxInvoiceAmountCents + 1
	}
	order := model.TopUp{UserId: user.Id, Money: money, PaidAmountMinor: paidAmount, PaidCurrency: "CNY", TradeNo: fmt.Sprintf("invoice-fee-%d", time.Now().UnixNano()), PaymentMethod: "stripe", Status: common.TopUpStatusSuccess, CreateTime: now, CompleteTime: now}
	require.NoError(t, model.DB.Create(&order).Error)
	t.Cleanup(func() {
		var ids []int
		require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Where("user_id = ?", user.Id).Pluck("id", &ids).Error)
		if len(ids) > 0 {
			require.NoError(t, model.DB.Where("application_id IN ?", ids).Delete(&model.InvoiceOrder{}).Error)
		}
		require.NoError(t, model.DB.Where("user_id = ?", user.Id).Delete(&model.InvoicePaymentOrder{}).Error)
		require.NoError(t, model.DB.Where("user_id = ?", user.Id).Delete(&model.InvoiceApplication{}).Error)
		require.NoError(t, model.DB.Where("user_id = ?", user.Id).Delete(&model.TopUp{}).Error)
		require.NoError(t, model.LOG_DB.Where("user_id = ?", user.Id).Delete(&model.Log{}).Error)
		require.NoError(t, model.DB.Delete(&model.User{}, user.Id).Error)
	})
	return user, order, CreateInvoiceApplicationInput{
		TopUpIds: []int{order.Id}, InvoiceTitle: "Example company", TaxNumber: "TEST123",
		RecipientEmail: "finance@example.com", ApplicantNote: "Please include the project name.",
	}
}

func TestInvoiceFeeDatabaseContracts(t *testing.T) {
	withInvoiceDatabases(t, runInvoiceFeeDatabaseContracts)
}

func runInvoiceFeeDatabaseContracts(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.InvoicePaymentOrder{}))

	t.Run("fee-paid invoice retains file delivery and cannot refund after upload", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, input := createInvoiceFeeFixture(t, 300_000, 50)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, nil, "", "", ""))
		originalSender := invoiceEmailSender
		t.Cleanup(func() { invoiceEmailSender = originalSender })
		payload := []byte("%PDF-1.4\n%fee-paid-invoice\n")
		invoiceEmailSender = func(_ string, recipient string, _ string, attachments []common.EmailAttachment) error {
			assert.Equal(t, input.RecipientEmail, recipient)
			require.Len(t, attachments, 1)
			assert.Equal(t, payload, attachments[0].Data)
			return fmt.Errorf("mail transport unavailable")
		}
		stored, err := UploadInvoiceFile(application.Id, "invoice.pdf", payload)
		require.ErrorContains(t, err, "email delivery failed")
		require.NotNil(t, stored)
		t.Cleanup(func() {
			assert.NoError(t, os.Remove(stored.InvoiceFilePath))
			_ = os.Remove(filepath.Dir(stored.InvoiceFilePath))
		})
		_, err = CancelInvoiceApplication(application.Id, user.Id)
		require.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
		invoiceEmailSender = func(_ string, recipient string, _ string, attachments []common.EmailAttachment) error {
			assert.Equal(t, input.RecipientEmail, recipient)
			require.Len(t, attachments, 1)
			assert.Equal(t, payload, attachments[0].Data)
			return nil
		}
		issued, err := SendInvoiceEmail(application.Id, 0)
		require.NoError(t, err)
		assert.Equal(t, model.InvoiceStatusIssued, issued.Status)
		content, err := GetInvoiceFileContent(application.Id)
		require.NoError(t, err)
		assert.Equal(t, payload, content.Data)
		_, err = CancelInvoiceApplication(application.Id, user.Id)
		require.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 200_000, wallet.Quota)
	})

	t.Run("invoice list and creation use actual paid cash across currencies", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, usdOrder, input := createInvoiceFeeFixture(t, 2_000_000, 999)
		require.NoError(t, model.DB.Model(&usdOrder).Updates(map[string]any{"paid_amount_minor": 5000, "paid_currency": "USD"}).Error)
		cnyOrder := model.TopUp{UserId: user.Id, Money: 123, PaidAmountMinor: 7500, PaidCurrency: "CNY", TradeNo: common.GetUUID(), PaymentMethod: "stripe", Status: common.TopUpStatusSuccess}
		require.NoError(t, model.DB.Create(&cnyOrder).Error)
		eligible, err := ListEligibleInvoiceOrders(user.Id)
		require.NoError(t, err)
		require.Len(t, eligible, 2)
		var previewTotal int64
		for _, order := range eligible {
			assert.Equal(t, "CNY", order.Currency)
			previewTotal += order.InvoiceAmountCents
			if order.Id == usdOrder.Id {
				assert.Equal(t, float64(999), order.Money)
				assert.Equal(t, int64(37500), order.InvoiceAmountCents)
			}
		}
		assert.Equal(t, int64(45000), previewTotal)
		input.TopUpIds = []int{usdOrder.Id, cnyOrder.Id}
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		assert.Equal(t, previewTotal, application.InvoiceAmountCents)
		assert.Equal(t, int64(1350), application.FinalSupplementCents)
		assert.Equal(t, 900_000, application.ChargedQuota)
		var snapshot struct {
			Policy invoiceFeePolicy        `json:"policy"`
			Orders []invoicePaidOrderQuote `json:"orders"`
		}
		require.NoError(t, common.UnmarshalJsonStr(application.RuleSnapshot, &snapshot))
		assert.Equal(t, 7.5, snapshot.Policy.USDToCNYRate)
		require.Len(t, snapshot.Orders, 2)
		assert.Equal(t, "USD", snapshot.Orders[0].OriginalCurrency)
		assert.Equal(t, int64(5000), snapshot.Orders[0].OriginalPaidAmountCents)
		assert.Equal(t, int64(37500), snapshot.Orders[0].InvoiceAmountCents)
	})

	t.Run("unknown historical cash is excluded and cannot charge", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, order, input := createInvoiceFeeFixture(t, 100_000, 50)
		require.NoError(t, model.DB.Model(&order).Updates(map[string]any{"paid_amount_minor": 0, "paid_currency": "", "payment_provider": model.PaymentProviderStripe}).Error)
		eligible, err := ListEligibleInvoiceOrders(user.Id)
		require.NoError(t, err)
		assert.Empty(t, eligible)
		_, err = CreateInvoiceApplication(user.Id, input)
		require.ErrorIs(t, err, ErrInvoicePaidAmountUnverifiable)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 100_000, wallet.Quota)
	})

	t.Run("legacy subscription uses stored verified receipt", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, topUp, input := createInvoiceFeeFixture(t, 1_000_000, 999)
		require.NoError(t, model.DB.Model(&topUp).Updates(map[string]any{"paid_amount_minor": 0, "paid_currency": ""}).Error)
		legacy := model.SubscriptionOrder{UserId: user.Id, TradeNo: topUp.TradeNo, Money: 999, PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess, ProviderPayload: `{"amount_total":"5000","currency":"USD"}`}
		require.NoError(t, model.DB.Create(&legacy).Error)
		t.Cleanup(func() { require.NoError(t, model.DB.Delete(&legacy).Error) })
		eligible, err := ListEligibleInvoiceOrders(user.Id)
		require.NoError(t, err)
		require.Len(t, eligible, 1)
		assert.Equal(t, int64(37500), eligible[0].InvoiceAmountCents)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		assert.Equal(t, int64(37500), application.InvoiceAmountCents)
		assert.Equal(t, int64(1125), application.FinalSupplementCents)
		assert.Equal(t, 750000, application.ChargedQuota)
	})

	t.Run("charge once then approve without changing amount", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, order, input := createInvoiceFeeFixture(t, 100_000, 50)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		assert.Equal(t, model.InvoiceStatusPendingReview, application.Status)
		assert.Equal(t, model.InvoicePaymentPaid, application.PaymentStatus)
		assert.Equal(t, model.InvoiceFeeModeBalanceOnApply, application.FeeMode)
		assert.Equal(t, int64(150), application.FinalSupplementCents)
		assert.Equal(t, int64(5_000), application.InvoiceAmountCents)
		assert.Equal(t, 100_000, application.ChargedQuota)
		assert.Equal(t, "CNY", application.Currency)
		assert.Equal(t, "¥", application.CurrencySymbol)
		assert.Equal(t, "技术服务费", application.InvoiceItemName)
		assert.True(t, application.CanCancel)
		assert.Equal(t, input.ApplicantNote, application.ApplicantNote)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
		var payments []model.InvoicePaymentOrder
		require.NoError(t, model.DB.Where("application_id = ?", application.Id).Find(&payments).Error)
		require.Len(t, payments, 1)
		assert.Equal(t, model.InvoicePaymentOrderPaid, payments[0].Status)
		assert.Equal(t, model.PaymentProviderBalance, payments[0].PaymentProvider)
		_, err = CreateInvoiceApplication(user.Id, input)
		require.ErrorIs(t, err, model.ErrInvoiceOrderAlreadyApplied)
		eligible, err := ListEligibleInvoiceOrders(user.Id)
		require.NoError(t, err)
		assert.Empty(t, eligible)
		adjusted := int64(151)
		require.ErrorContains(t, ReviewInvoiceApplication(application.Id, 1, true, &adjusted, "change fee", "", ""), "cannot be changed")
		require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, nil, "", "", "reviewed"))
		approved, err := GetInvoiceApplication(application.Id, user.Id)
		require.NoError(t, err)
		assert.Equal(t, model.InvoiceStatusApproved, approved.Status)
		assert.Equal(t, int64(5_000), approved.InvoiceAmountCents)
		assert.Equal(t, int64(150), approved.FinalSupplementCents)
		assert.Equal(t, "技术服务费", approved.InvoiceItemName)
		_, err = CreateInvoicePaymentOrder(application.Id, user.Id, "must-not-exist", "alipay", model.PaymentProviderEpay)
		require.ErrorIs(t, err, model.ErrInvoiceStatusInvalid)
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
		var storedTopUp model.TopUp
		require.NoError(t, model.DB.First(&storedTopUp, order.Id).Error)
		assert.Equal(t, order.Money, storedTopUp.Money)
	})

	t.Run("insufficient balance rolls back every row", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, order, input := createInvoiceFeeFixture(t, 99_999, 50)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.ErrorIs(t, err, model.ErrWalletQuotaInsufficient)
		assert.Nil(t, application)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 99_999, wallet.Quota)
		for _, table := range []any{&model.InvoiceApplication{}, &model.InvoicePaymentOrder{}} {
			var count int64
			require.NoError(t, model.DB.Model(table).Where("user_id = ?", user.Id).Count(&count).Error)
			assert.Zero(t, count)
		}
		var count int64
		require.NoError(t, model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id = ?", order.Id).Count(&count).Error)
		assert.Zero(t, count)
	})

	t.Run("payment record failure rolls back charge and order reservation", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, order, input := createInvoiceFeeFixture(t, 100_000, 50)
		paymentFailure := fmt.Errorf("payment record write failed")
		callbackName := "invoice_fee_payment_failure"
		require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
			if _, payment := tx.Statement.Dest.(*model.InvoicePaymentOrder); payment {
				tx.AddError(paymentFailure)
			}
		}))
		t.Cleanup(func() { model.DB.Callback().Create().Remove(callbackName) })
		application, err := CreateInvoiceApplication(user.Id, input)
		require.ErrorIs(t, err, paymentFailure)
		assert.Nil(t, application)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 100_000, wallet.Quota)
		var count int64
		require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Where("user_id = ?", user.Id).Count(&count).Error)
		assert.Zero(t, count)
		require.NoError(t, model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id = ?", order.Id).Count(&count).Error)
		assert.Zero(t, count)
		require.NoError(t, model.DB.Callback().Create().Remove(callbackName))
		application, err = CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		assert.Equal(t, 100_000, application.ChargedQuota)
	})

	t.Run("zero fee succeeds with zero balance", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		invoice_setting.GetInvoiceSetting().FeeRateBasisPoints = 0
		user, _, input := createInvoiceFeeFixture(t, 0, 50)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		assert.Zero(t, application.ChargedQuota)
		assert.Equal(t, model.InvoicePaymentNotRequired, application.PaymentStatus)
		require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, nil, "", "", ""))
		_, err = CancelInvoiceApplication(application.Id, user.Id)
		require.NoError(t, err)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
	})

	for _, action := range []string{"reject", "cancel", "cancel approved", "admin delete"} {
		t.Run(action+" refunds frozen original quota once", func(t *testing.T) {
			setupInvoiceFeeSettings(t)
			user, _, input := createInvoiceFeeFixture(t, 100_000, 50)
			application, err := CreateInvoiceApplication(user.Id, input)
			require.NoError(t, err)
			invoice_setting.GetInvoiceSetting().FeeRateBasisPoints = 10_000
			common.QuotaPerUnit = 1
			operation_setting.USDExchangeRate = 1
			if action == "reject" {
				require.NoError(t, ReviewInvoiceApplication(application.Id, 1, false, nil, "", "cannot issue", ""))
				stored, err := GetInvoiceApplication(application.Id, user.Id)
				require.NoError(t, err)
				assert.Equal(t, 100_000, stored.RefundedQuota)
				assert.Equal(t, model.InvoiceStatusRejected, stored.Status)
				require.ErrorIs(t, ReviewInvoiceApplication(application.Id, 1, false, nil, "", "cannot issue", ""), model.ErrInvoiceStatusInvalid)
				_, err = CancelInvoiceApplication(application.Id, user.Id)
				require.NoError(t, err)
			} else if action == "admin delete" {
				_, err = DeleteInvoiceApplication(application.Id)
				require.NoError(t, err)
			} else {
				if action == "cancel approved" {
					require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, nil, "", "", ""))
				}
				_, err = CancelInvoiceApplication(application.Id, user.Id)
				require.NoError(t, err)
			}
			_, err = CancelInvoiceApplication(application.Id, user.Id)
			require.ErrorIs(t, err, model.ErrInvoiceNotFound)
			var wallet model.User
			require.NoError(t, model.DB.First(&wallet, user.Id).Error)
			assert.Equal(t, 100_000, wallet.Quota)
			eligible, err := ListEligibleInvoiceOrders(user.Id)
			require.NoError(t, err)
			assert.Len(t, eligible, 1)
			var logs []model.Log
			require.NoError(t, model.LOG_DB.Where("user_id = ?", user.Id).Order("id asc").Find(&logs).Error)
			require.Len(t, logs, 2)
			assert.Contains(t, logs[0].Content, "额度变动 -100000")
			assert.Contains(t, logs[1].Content, "额度变动 +100000")
		})
	}

	for _, currency := range []struct {
		mode     string
		currency string
		symbol   string
		quota    int
	}{
		{mode: "USD", currency: "USD", symbol: "$", quota: 750_000},
		{mode: "CUSTOM", currency: "CUSTOM", symbol: "€", quota: 750_000},
		{mode: "TOKENS", currency: "USD", symbol: "$", quota: 750_000},
	} {
		t.Run(currency.mode+" application follows system conversion", func(t *testing.T) {
			setupInvoiceFeeSettings(t)
			operation_setting.GetGeneralSetting().QuotaDisplayType = currency.mode
			user, order, input := createInvoiceFeeFixture(t, currency.quota, 50)
			require.NoError(t, model.DB.Model(&order).Update("paid_currency", "USD").Error)
			application, err := CreateInvoiceApplication(user.Id, input)
			require.NoError(t, err)
			assert.Equal(t, currency.currency, application.Currency)
			assert.Equal(t, currency.symbol, application.CurrencySymbol)
			assert.Equal(t, currency.quota, application.ChargedQuota)
			if currency.mode == "CUSTOM" {
				assert.Equal(t, int64(300), application.FinalSupplementCents)
				assert.Equal(t, int64(10_000), application.InvoiceAmountCents)
			} else {
				assert.Equal(t, int64(150), application.FinalSupplementCents)
				assert.Equal(t, int64(5_000), application.InvoiceAmountCents)
			}
			operation_setting.GetGeneralSetting().QuotaDisplayType = "CNY"
			_, err = CancelInvoiceApplication(application.Id, user.Id)
			require.NoError(t, err)
			var wallet model.User
			require.NoError(t, model.DB.First(&wallet, user.Id).Error)
			assert.Equal(t, currency.quota, wallet.Quota)
		})
	}

	t.Run("oversized paid amounts never charge or overflow", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, input := createInvoiceFeeFixture(t, 100_000, 1e100)
		_, err := CreateInvoiceApplication(user.Id, input)
		require.ErrorIs(t, err, ErrInvoicePaidAmountUnverifiable)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 100_000, wallet.Quota)
	})

	t.Run("owner and paid-order restrictions are atomic", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, order, input := createInvoiceFeeFixture(t, 300_000, 50)
		other, otherOrder, _ := createInvoiceFeeFixture(t, 300_000, 50)
		input.TopUpIds = []int{order.Id, otherOrder.Id}
		_, err := CreateInvoiceApplication(user.Id, input)
		require.ErrorContains(t, err, "paid orders are invalid")
		require.NoError(t, model.DB.Model(&otherOrder).Updates(map[string]any{"user_id": user.Id, "status": common.TopUpStatusPending}).Error)
		_, err = CreateInvoiceApplication(user.Id, input)
		require.ErrorContains(t, err, "paid orders are invalid")
		input.TopUpIds = []int{order.Id}
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		_, err = CancelInvoiceApplication(application.Id, other.Id)
		require.ErrorIs(t, err, model.ErrInvoiceNotFound)
		_, err = CancelInvoiceApplication(application.Id, 0)
		require.ErrorIs(t, err, model.ErrInvoiceNotFound)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 200_000, wallet.Quota)
	})

	t.Run("concurrent application and refunds never double charge or credit", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, input := createInvoiceFeeFixture(t, 300_000, 50)
		var wg sync.WaitGroup
		start := make(chan struct{})
		applications := make([]*model.InvoiceApplication, 2)
		errors := make([]error, 2)
		for i := range applications {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				applications[index], errors[index] = CreateInvoiceApplication(user.Id, input)
			}(i)
		}
		close(start)
		wg.Wait()
		var application *model.InvoiceApplication
		successes := 0
		for i := range applications {
			if errors[i] == nil {
				successes++
				application = applications[i]
			}
		}
		require.Equal(t, 1, successes)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 200_000, wallet.Quota)
		start = make(chan struct{})
		for i := range errors {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				if index == 0 {
					errors[index] = ReviewInvoiceApplication(application.Id, 1, false, nil, "", "cannot issue", "")
				} else {
					_, errors[index] = CancelInvoiceApplication(application.Id, user.Id)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		assert.True(t, errors[0] == nil || errors[1] == nil)
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, 300_000, wallet.Quota)
	})

	t.Run("legacy review approval never creates another payment step", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, _ := createInvoiceFeeFixture(t, 0, 50)
		application := model.InvoiceApplication{
			UserId: user.Id, Status: model.InvoiceStatusPendingReview,
			PaymentStatus: model.InvoicePaymentNotRequired,
			Currency:      "CNY", OrderAmountCents: 5_000, InvoiceAmountCents: 5_000,
			SuggestedSupplementCents: 613,
		}
		require.NoError(t, model.DB.Create(&application).Error)
		legacyFee := int64(613)
		require.NoError(t, ReviewInvoiceApplication(application.Id, 1, true, &legacyFee, "", "", "approved"))
		approved, err := GetInvoiceApplication(application.Id, user.Id)
		require.NoError(t, err)
		assert.Equal(t, model.InvoiceStatusApproved, approved.Status)
		assert.Equal(t, model.InvoicePaymentNotRequired, approved.PaymentStatus)
		assert.Equal(t, int64(5_000), approved.InvoiceAmountCents)
		assert.Zero(t, approved.FinalSupplementCents)
		assert.Zero(t, approved.ChargedQuota)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
	})

	t.Run("old external payment never creates wallet refund", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, _ := createInvoiceFeeFixture(t, 0, 50)
		application := model.InvoiceApplication{
			UserId: user.Id, Status: model.InvoiceStatusPendingReview,
			PaymentStatus: model.InvoicePaymentNotRequired, FinalSupplementCents: 150,
		}
		require.NoError(t, model.DB.Create(&application).Error)
		require.NoError(t, ReviewInvoiceApplication(application.Id, 1, false, nil, "", "legacy reject", ""))
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
		_, err := CancelInvoiceApplication(application.Id, user.Id)
		require.NoError(t, err)
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
	})

	t.Run("refund is rolled back when state transaction fails", func(t *testing.T) {
		setupInvoiceFeeSettings(t)
		user, _, input := createInvoiceFeeFixture(t, 100_000, 50)
		application, err := CreateInvoiceApplication(user.Id, input)
		require.NoError(t, err)
		rollbackError := fmt.Errorf("review write failed")
		err = model.UpdateInvoiceApplication(application.Id, func(tx *gorm.DB, stored *model.InvoiceApplication) error {
			_, err := model.RefundInvoiceFeeTx(tx, stored)
			require.NoError(t, err)
			return rollbackError
		})
		require.ErrorIs(t, err, rollbackError)
		stored, err := GetInvoiceApplication(application.Id, user.Id)
		require.NoError(t, err)
		assert.Zero(t, stored.RefundedQuota)
		assert.Equal(t, model.InvoiceStatusPendingReview, stored.Status)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Zero(t, wallet.Quota)
	})
}

func TestInvoicePaidOrderQuoteUsesVerifiedCashAndFrozenRates(t *testing.T) {
	tests := []struct {
		name       string
		paid       int64
		source     string
		target     string
		targetRate float64
		want       int64
		invalid    bool
	}{
		{name: "USD to CNY", paid: 5000, source: "USD", target: "CNY", targetRate: 7.5, want: 37500},
		{name: "CNY to USD", paid: 5000, source: "CNY", target: "USD", targetRate: 1, want: 667},
		{name: "CNY to custom display", paid: 5000, source: "CNY", target: "CUSTOM", targetRate: 2, want: 1333},
		{name: "USD to custom display", paid: 5000, source: "USD", target: "CUSTOM", targetRate: 2, want: 10000},
		{name: "half target cent rounds up", paid: 1, source: "USD", target: "CNY", targetRate: 7.5, want: 8},
		{name: "unsupported source", paid: 5000, source: "JPY", target: "USD", targetRate: 1, invalid: true},
		{name: "missing source", paid: 5000, target: "USD", targetRate: 1, invalid: true},
		{name: "zero actual payment", source: "USD", target: "USD", targetRate: 1, invalid: true},
		{name: "oversized source", paid: maxInvoiceAmountCents + 1, source: "USD", target: "USD", targetRate: 1, invalid: true},
		{name: "oversized converted amount", paid: maxInvoiceAmountCents, source: "USD", target: "CNY", targetRate: 7.5, invalid: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quote, err := quoteInvoicePaidOrder(model.TopUp{
				Id: 42, Money: 999, PaidAmountMinor: test.paid, PaidCurrency: test.source,
			}, invoiceFeePolicy{Currency: test.target, USDToCurrencyRate: test.targetRate, USDToCNYRate: 7.5}, model.SubscriptionOrder{})
			if test.invalid {
				require.ErrorIs(t, err, ErrInvoicePaidAmountUnverifiable)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, quote.InvoiceAmountCents)
			assert.Equal(t, test.paid, quote.OriginalPaidAmountCents)
			assert.Equal(t, test.source, quote.OriginalCurrency)
		})
	}
}

func TestInvoiceLegacyReceiptsRequireActualPaidAmountAndCurrency(t *testing.T) {
	originalStoreID := setting.WaffoPancakeStoreID
	setting.WaffoPancakeStoreID = "STO_local"
	t.Cleanup(func() { setting.WaffoPancakeStoreID = originalStoreID })
	for _, test := range []struct {
		name     string
		provider string
		payload  string
		want     int64
		invalid  bool
	}{
		{name: "Stripe saved minor amount", provider: model.PaymentProviderStripe, payload: `{"amount_total":"4200","currency":"usd","event_type":"checkout.session.completed"}`, want: 4200},
		{name: "Stripe missing actual amount", provider: model.PaymentProviderStripe, payload: `{"currency":"USD"}`, invalid: true},
		{name: "Stripe zero charged after coupon", provider: model.PaymentProviderStripe, payload: `{"amount_total":"0","currency":"USD"}`, invalid: true},
		{name: "Creem paid amount", provider: model.PaymentProviderCreem, payload: `{"object":{"request_id":"receipt-test","order":{"amount_paid":4700,"currency":"USD","status":"paid"}}}`, want: 4700},
		{name: "Creem other order", provider: model.PaymentProviderCreem, payload: `{"object":{"request_id":"other","order":{"amount_paid":4700,"currency":"USD","status":"paid"}}}`, invalid: true},
		{name: "Pancake total includes tax", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_local","data":{"amount":"50.00","total":"55.00","taxAmount":"5.00","currency":"USD"}}`, want: 5500},
		{name: "Pancake explicit zero tax", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_local","data":{"amount":"50.00","taxAmount":"0.00","currency":"USD"}}`, want: 5000},
		{name: "Pancake unknown tax", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_local","data":{"amount":"50.00","currency":"USD"}}`, invalid: true},
		{name: "Pancake missing taxed total", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_local","data":{"amount":"50.00","taxAmount":"5.00","currency":"USD"}}`, invalid: true},
		{name: "Pancake foreign store", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_other","data":{"total":"55.00","currency":"USD"}}`, invalid: true},
		{name: "Pancake missing store", provider: model.PaymentProviderWaffoPancake, payload: `{"data":{"total":"55.00","currency":"USD"}}`, invalid: true},
		{name: "Pancake padded store", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":" STO_local ","data":{"total":"55.00","currency":"USD"}}`, invalid: true},
		{name: "unsupported paid currency", provider: model.PaymentProviderStripe, payload: `{"amount_total":"4200","currency":"EUR"}`, invalid: true},
		{name: "unbounded decimal exponent", provider: model.PaymentProviderWaffoPancake, payload: `{"storeId":"STO_local","data":{"total":"1e2147483647","currency":"USD"}}`, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			amount, currency, err := legacyInvoicePaymentReceipt(model.SubscriptionOrder{TradeNo: "receipt-test", PaymentProvider: test.provider, ProviderPayload: test.payload})
			if test.invalid {
				require.ErrorIs(t, err, ErrInvoicePaidAmountUnverifiable)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, amount)
			assert.Equal(t, "USD", currency)
		})
	}
	t.Run("Pancake missing configured store", func(t *testing.T) {
		setting.WaffoPancakeStoreID = ""
		_, _, err := legacyInvoicePaymentReceipt(model.SubscriptionOrder{PaymentProvider: model.PaymentProviderWaffoPancake, ProviderPayload: `{"storeId":"STO_local","data":{"total":"55.00","currency":"USD"}}`})
		require.ErrorIs(t, err, ErrInvoicePaidAmountUnverifiable)
	})
}

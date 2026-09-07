package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInvoiceFeeOptionValidation(t *testing.T) {
	for _, test := range []struct {
		key   string
		value string
		valid bool
	}{
		{invoice_setting.FeeRateBasisPointsOptionKey, "0", true},
		{invoice_setting.FeeRateBasisPointsOptionKey, "300", true},
		{invoice_setting.FeeRateBasisPointsOptionKey, "10000", true},
		{invoice_setting.FeeRateBasisPointsOptionKey, "10001", false},
		{invoice_setting.FeeRateBasisPointsOptionKey, "-1", false},
		{invoice_setting.FeeRateBasisPointsOptionKey, "NaN", false},
		{invoice_setting.FeeRateBasisPointsOptionKey, "1.5", false},
		{"invoice_setting.minimum_amount", "50.00", true},
		{"invoice_setting.minimum_amount", "0", true},
		{"invoice_setting.minimum_amount", "-0.01", false},
		{"invoice_setting.minimum_amount", "NaN", false},
		{"invoice_setting.minimum_amount", "+Inf", false},
		{"invoice_setting.minimum_amount", "1000000001", false},
		{"invoice_setting.invoice_item_name", "技术服务费", true},
		{"invoice_setting.invoice_item_name", "  ", false},
		{"invoice_setting.invoice_item_name", strings.Repeat("项", 256), false},
	} {
		t.Run(test.key+"/"+test.value, func(t *testing.T) {
			err := validateOptionValue(test.key, test.value)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestInvoiceFeeRefundIsExactAndIdempotent(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		user := User{Username: "invoice-refund", Quota: 1000}
		require.NoError(t, DB.Create(&user).Error)
		application := InvoiceApplication{UserId: user.Id, FeeMode: InvoiceFeeModeBalanceOnApply, ChargedQuota: 150, Status: InvoiceStatusPendingReview, PaymentStatus: InvoicePaymentPaid}
		require.NoError(t, DB.Create(&application).Error)
		refund := 0
		require.NoError(t, UpdateInvoiceApplication(application.Id, func(tx *gorm.DB, locked *InvoiceApplication) error {
			var err error
			refund, err = RefundInvoiceFeeTx(tx, locked)
			if err != nil {
				return err
			}
			return tx.Model(locked).Update("status", InvoiceStatusRejected).Error
		}))
		assert.Equal(t, 150, refund)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, 1150, user.Quota)
		require.NoError(t, DB.First(&application, application.Id).Error)
		assert.Equal(t, 150, application.RefundedQuota)
		assert.Equal(t, InvoicePaymentRefunded, application.PaymentStatus)
		deleted, refundedAgain, err := DeleteInvoiceApplicationForUser(application.Id, user.Id)
		require.NoError(t, err)
		assert.Equal(t, InvoiceStatusRejected, deleted.Status)
		assert.Zero(t, refundedAgain)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, 1150, user.Quota)
		_, _, err = DeleteInvoiceApplicationForUser(application.Id, user.Id)
		assert.ErrorIs(t, err, ErrInvoiceNotFound)
	}, &InvoiceApplication{}, &InvoiceOrder{}, &InvoicePaymentOrder{})
}

func TestInvoiceFeeCancellationProtectsOwnershipAndIssuedFiles(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		user := User{Username: "invoice-cancel", Quota: 1000}
		require.NoError(t, DB.Create(&user).Error)
		application := InvoiceApplication{UserId: user.Id, FeeMode: InvoiceFeeModeBalanceOnApply, ChargedQuota: 150, Status: InvoiceStatusApproved, PaymentStatus: InvoicePaymentPaid}
		require.NoError(t, DB.Create(&application).Error)
		_, _, err := DeleteInvoiceApplicationForUser(application.Id, user.Id+1)
		assert.ErrorIs(t, err, ErrInvoiceNotFound)
		require.NoError(t, DB.Model(&application).Update("invoice_file_path", "upload/invoices/prepared.pdf").Error)
		_, _, err = DeleteInvoiceApplicationForUser(application.Id, user.Id)
		assert.ErrorIs(t, err, ErrInvoiceStatusInvalid)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, 1000, user.Quota)
		require.NoError(t, DB.Model(&application).Update("invoice_file_path", "").Error)
		_, refund, err := DeleteInvoiceApplicationForUser(application.Id, user.Id)
		require.NoError(t, err)
		assert.Equal(t, 150, refund)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, 1150, user.Quota)
	}, &InvoiceApplication{}, &InvoiceOrder{}, &InvoicePaymentOrder{})
}

func TestInvoiceFeeRefundRollsBackAtWalletLimitAndNeverCreditsLegacyPayment(t *testing.T) {
	withCommerceDatabases(t, func(t *testing.T) {
		user := User{Username: "invoice-refund-bound", Quota: common.MaxWalletQuota - 149}
		require.NoError(t, DB.Create(&user).Error)
		application := InvoiceApplication{UserId: user.Id, FeeMode: InvoiceFeeModeBalanceOnApply, ChargedQuota: 150, Status: InvoiceStatusPendingReview, PaymentStatus: InvoicePaymentPaid}
		require.NoError(t, DB.Create(&application).Error)
		_, _, err := DeleteInvoiceApplicationForUser(application.Id, user.Id)
		assert.ErrorIs(t, err, ErrWalletQuotaLimitExceeded)
		require.NoError(t, DB.First(&application, application.Id).Error)
		assert.Zero(t, application.RefundedQuota)
		assert.Equal(t, InvoiceStatusPendingReview, application.Status)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, common.MaxWalletQuota-149, user.Quota)
		// A legacy external payment has no recorded wallet debit to return.
		require.NoError(t, DB.Model(&application).Update("fee_mode", "").Error)
		_, refund, err := DeleteInvoiceApplicationForUser(application.Id, user.Id)
		require.NoError(t, err)
		assert.Zero(t, refund)
		require.NoError(t, DB.First(&user, user.Id).Error)
		assert.Equal(t, common.MaxWalletQuota-149, user.Quota)
	}, &InvoiceApplication{}, &InvoiceOrder{}, &InvoicePaymentOrder{})
}

package service

import (
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// computeInvoiceFee — pure-arithmetic table-driven tests (no DB)
// ===========================================================================
//
// Contract:
//   - feeRate <= 0 or invoiceAmount <= 0 → (0, 0): feature off / nothing to bill
//   - feeRate clamped to 1 internally (defense-in-depth)
//   - feeQuota = ceil(invoiceAmount * feeRate / Price * QuotaPerUnit)
//   - feeAmount derived from feeQuota so the displayed RMB equals the actual debit

func TestComputeInvoiceFee(t *testing.T) {
	const price = 7.3
	const quotaPerUnit = 500_000.0

	tests := []struct {
		name              string
		invoiceAmount     float64
		feeRate           float64
		price             float64
		quotaPerUnit      float64
		expectedFeeQuota  int
		expectedFeeAmount float64
	}{
		{
			name:          "fee disabled (rate=0) returns nothing",
			invoiceAmount: 1000,
			feeRate:       0,
			price:         price,
			quotaPerUnit:  quotaPerUnit,
		},
		{
			name:          "negative rate returns nothing",
			invoiceAmount: 1000,
			feeRate:       -0.05,
			price:         price,
			quotaPerUnit:  quotaPerUnit,
		},
		{
			name:          "zero invoice amount returns nothing",
			invoiceAmount: 0,
			feeRate:       0.06,
			price:         price,
			quotaPerUnit:  quotaPerUnit,
		},
		{
			name:          "negative invoice amount returns nothing",
			invoiceAmount: -100,
			feeRate:       0.06,
			price:         price,
			quotaPerUnit:  quotaPerUnit,
		},
		{
			name:              "6% on 1000 RMB at default rates",
			invoiceAmount:     1000,
			feeRate:           0.06,
			price:             price,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  4109590, // ceil(60 / 7.3 * 500000) = ceil(4109589.04...) = 4109590
			expectedFeeAmount: 60.00,
		},
		{
			name:              "ceiling boundary — non-integer quota rounds up",
			invoiceAmount:     500,
			feeRate:           0.06,
			price:             price,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  2054795, // ceil(30 / 7.3 * 500000) = ceil(2054794.52...) = 2054795
			expectedFeeAmount: 30.00,
		},
		{
			name:              "tiny rate that still rounds to >= 1 quota",
			invoiceAmount:     500,
			feeRate:           0.0001,
			price:             price,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  3425, // ceil(0.05 / 7.3 * 500000) = ceil(3424.65...) = 3425
			expectedFeeAmount: 0.05,
		},
		{
			name:              "rate > 1 is clamped to 1 (defense-in-depth)",
			invoiceAmount:     1000,
			feeRate:           5.0, // crazy admin input
			price:             price,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  68493151, // clamped: ceil(1000 / 7.3 * 500000) = ceil(68493150.68...) = 68493151
			expectedFeeAmount: 1000.00,
		},
		{
			name:              "Price=0 falls back to 1 to avoid divide-by-zero",
			invoiceAmount:     10,
			feeRate:           0.5,
			price:             0,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  2500000, // 10 * 0.5 / 1 * 500000 = 2500000
			expectedFeeAmount: 5.00,
		},
		{
			name:              "negative Price falls back to 1",
			invoiceAmount:     10,
			feeRate:           0.5,
			price:             -7.3,
			quotaPerUnit:      quotaPerUnit,
			expectedFeeQuota:  2500000,
			expectedFeeAmount: 5.00,
		},
		{
			name:              "QuotaPerUnit=0 falls back to 1",
			invoiceAmount:     10,
			feeRate:           0.06,
			price:             price,
			quotaPerUnit:      0,
			expectedFeeQuota:  1, // ceil(10 * 0.06 / 7.3 * 1) = ceil(0.0821...) = 1
			expectedFeeAmount: 7.30,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			feeAmount, feeQuota := computeInvoiceFee(tc.invoiceAmount, tc.feeRate, tc.price, tc.quotaPerUnit)
			assert.Equal(t, tc.expectedFeeQuota, feeQuota, "feeQuota mismatch")
			assert.InDelta(t, tc.expectedFeeAmount, feeAmount, 0.01, "feeAmount mismatch")
		})
	}
}

// Reconciliation invariant: the displayed fee_amount must equal the actual
// debit when converted back via the same Price/QuotaPerUnit. This guards
// against the original bug where Ceil() on quota left fee_amount lagging.
func TestComputeInvoiceFee_DisplayMatchesDebit(t *testing.T) {
	const price = 7.3
	const quotaPerUnit = 500_000.0

	cases := []struct {
		amount float64
		rate   float64
	}{
		{1000, 0.06},
		{500, 0.06},
		{12345.67, 0.05},
		{1, 0.01},
		{99.99, 0.0817},
	}

	for _, tc := range cases {
		feeAmount, feeQuota := computeInvoiceFee(tc.amount, tc.rate, price, quotaPerUnit)
		require.Greater(t, feeQuota, 0)

		// The user is told they paid `feeAmount` RMB. If we convert their
		// `feeQuota` debit back to RMB, it must equal `feeAmount` (within
		// 1 fen of rounding for display).
		quotaInRmb := float64(feeQuota) / quotaPerUnit * price
		assert.InDelta(t, feeAmount, quotaInRmb, 0.01,
			"displayed fee_amount %.4f should equal quota-derived RMB %.4f for amount=%.2f rate=%.4f",
			feeAmount, quotaInRmb, tc.amount, tc.rate)
	}
}

func TestComputeInvoiceFee_RejectsNonFiniteInputs(t *testing.T) {
	tests := []struct {
		name          string
		invoiceAmount float64
		feeRate       float64
		price         float64
		quotaPerUnit  float64
	}{
		{
			name:          "NaN fee rate",
			invoiceAmount: 1000,
			feeRate:       math.NaN(),
			price:         7.3,
			quotaPerUnit:  500_000,
		},
		{
			name:          "Inf fee rate",
			invoiceAmount: 1000,
			feeRate:       math.Inf(1),
			price:         7.3,
			quotaPerUnit:  500_000,
		},
		{
			name:          "NaN invoice amount",
			invoiceAmount: math.NaN(),
			feeRate:       0.06,
			price:         7.3,
			quotaPerUnit:  500_000,
		},
		{
			name:          "Inf price",
			invoiceAmount: 1000,
			feeRate:       0.06,
			price:         math.Inf(1),
			quotaPerUnit:  500_000,
		},
		{
			name:          "NaN quota per unit",
			invoiceAmount: 1000,
			feeRate:       0.06,
			price:         7.3,
			quotaPerUnit:  math.NaN(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				feeAmount, feeQuota := computeInvoiceFee(tc.invoiceAmount, tc.feeRate, tc.price, tc.quotaPerUnit)
				assert.Equal(t, 0, feeQuota)
				assert.Equal(t, 0.0, feeAmount)
			})
		})
	}
}

// ===========================================================================
// chargeInvoiceFee / refundInvoiceFee — DB-backed integration tests
// ===========================================================================

func seedInvoiceUser(t *testing.T, id, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "invoice_test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedInvoiceRow(t *testing.T, userId int, amount float64) *model.Invoice {
	t.Helper()
	inv := &model.Invoice{
		UserId: userId,
		Amount: amount,
		Status: model.InvoiceStatusPending,
	}
	require.NoError(t, model.DB.Create(inv).Error)
	return inv
}

func currentQuota(t *testing.T, userId int) int {
	t.Helper()
	var u model.User
	require.NoError(t, model.DB.Where("id = ?", userId).First(&u).Error)
	return u.Quota
}

func TestChargeInvoiceFee_FeatureDisabled_NoOp(t *testing.T) {
	truncate(t)
	seedInvoiceUser(t, 8001, 1_000_000)
	inv := seedInvoiceRow(t, 8001, 1000)

	// Save & restore Price so we don't leak state between tests.
	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	require.NoError(t, chargeInvoiceFee(model.DB, inv, 0))
	assert.Equal(t, 1_000_000, currentQuota(t, 8001), "balance must be unchanged when fee disabled")
	assert.Equal(t, 0, inv.FeeQuota)
	assert.Equal(t, 0.0, inv.FeeAmount)
}

func TestChargeInvoiceFee_DeductsAndSnapshots(t *testing.T) {
	truncate(t)
	const userQuota = 100_000_000 // ample
	seedInvoiceUser(t, 8002, userQuota)
	inv := seedInvoiceRow(t, 8002, 1000)

	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	require.NoError(t, chargeInvoiceFee(model.DB, inv, 0.06))
	assert.Greater(t, inv.FeeQuota, 0)
	assert.Greater(t, inv.FeeAmount, 0.0)
	assert.Equal(t, 0.06, inv.FeeRate)

	// User row was actually debited by exactly feeQuota.
	assert.Equal(t, userQuota-inv.FeeQuota, currentQuota(t, 8002))

	// Invoice snapshot persisted to DB.
	var reloaded model.Invoice
	require.NoError(t, model.DB.Where("id = ?", inv.Id).First(&reloaded).Error)
	assert.Equal(t, inv.FeeQuota, reloaded.FeeQuota)
	assert.InDelta(t, inv.FeeAmount, reloaded.FeeAmount, 0.001)
	assert.Equal(t, 0.06, reloaded.FeeRate)
}

func TestChargeInvoiceFee_InsufficientQuota_RejectsAtomically(t *testing.T) {
	truncate(t)
	seedInvoiceUser(t, 8003, 100) // far less than fee will require
	inv := seedInvoiceRow(t, 8003, 1000)

	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	err := chargeInvoiceFee(model.DB, inv, 0.06)
	require.ErrorIs(t, err, ErrInsufficientQuotaForFee)

	// User quota was NOT touched (atomic conditional UPDATE).
	assert.Equal(t, 100, currentQuota(t, 8003))

	// Invoice snapshot was NOT written.
	var reloaded model.Invoice
	require.NoError(t, model.DB.Where("id = ?", inv.Id).First(&reloaded).Error)
	assert.Equal(t, 0, reloaded.FeeQuota)
	assert.Equal(t, 0.0, reloaded.FeeAmount)
}

func TestRefundInvoiceFee_RefundsAndCASIdempotent(t *testing.T) {
	truncate(t)
	const startingQuota = 100_000_000
	seedInvoiceUser(t, 8004, startingQuota)
	inv := seedInvoiceRow(t, 8004, 1000)

	origPrice := operation_setting.Price
	t.Cleanup(func() { operation_setting.Price = origPrice })
	operation_setting.Price = 7.3

	// Charge first.
	require.NoError(t, chargeInvoiceFee(model.DB, inv, 0.06))
	require.Greater(t, inv.FeeQuota, 0)
	debitedQuota := inv.FeeQuota
	balanceAfterCharge := currentQuota(t, 8004)
	assert.Equal(t, startingQuota-debitedQuota, balanceAfterCharge)

	// First refund: must succeed and credit user back.
	didRefund, err := refundInvoiceFee(model.DB, inv)
	require.NoError(t, err)
	assert.True(t, didRefund, "first refund call must perform the credit")
	assert.True(t, inv.FeeRefunded)
	assert.Equal(t, startingQuota, currentQuota(t, 8004), "user must be made whole after refund")

	// Second refund (CAS idempotency): must be a no-op, no double-credit.
	didRefund2, err := refundInvoiceFee(model.DB, inv)
	require.NoError(t, err)
	assert.False(t, didRefund2, "second refund must report no-op (CAS idempotency)")
	assert.Equal(t, startingQuota, currentQuota(t, 8004), "user balance must NOT be double-refunded")

	// fee_refunded flag is sticky in DB.
	var reloaded model.Invoice
	require.NoError(t, model.DB.Where("id = ?", inv.Id).First(&reloaded).Error)
	assert.True(t, reloaded.FeeRefunded)
}

func TestRefundInvoiceFee_NoFeeCharged_SkipsCleanly(t *testing.T) {
	truncate(t)
	seedInvoiceUser(t, 8005, 1000)
	inv := seedInvoiceRow(t, 8005, 1000)
	// FeeQuota is 0 (never charged).

	didRefund, err := refundInvoiceFee(model.DB, inv)
	require.NoError(t, err)
	assert.False(t, didRefund, "must not perform a refund when no fee was charged")
	assert.Equal(t, 1000, currentQuota(t, 8005), "no-fee case must not touch user balance")
}

// ===========================================================================
// model.UpdateInvoiceStatus — CAS contract guards cancel-vs-approve race
// ===========================================================================

func TestUpdateInvoiceStatus_CAS_FirstWinsSecondMisses(t *testing.T) {
	truncate(t)
	seedInvoiceUser(t, 8006, 1000)
	inv := seedInvoiceRow(t, 8006, 1000) // status = Pending

	// First transition: Pending → Canceled. Should succeed.
	require.NoError(t, model.UpdateInvoiceStatus(
		model.DB, inv.Id,
		model.InvoiceStatusPending, model.InvoiceStatusCanceled,
		map[string]interface{}{},
	))

	// Second transition with the same fromStatus must miss (row is no longer
	// Pending). This is the guarantee that protects user-cancel from racing
	// admin-approve once money is in flight.
	err := model.UpdateInvoiceStatus(
		model.DB, inv.Id,
		model.InvoiceStatusPending, model.InvoiceStatusApproved,
		map[string]interface{}{},
	)
	require.ErrorIs(t, err, model.ErrInvoiceStatusChanged)

	// Final DB state is the winner's target, untouched by the loser.
	var reloaded model.Invoice
	require.NoError(t, model.DB.Where("id = ?", inv.Id).First(&reloaded).Error)
	assert.Equal(t, model.InvoiceStatusCanceled, reloaded.Status,
		"loser must not have overwritten the winning transition")
}

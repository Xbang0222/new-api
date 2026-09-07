package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// legacyInvoiceFeeApplication preserves the complete invoice application schema
// from 15227624eaa558f66a2e4864301b44dc49b82b8e, the last commit before balance
// fees. That invoice schema is identical to initial commit 2e415c6c69fc4959712c7eedabcbef9f802d178c.
// No release tag was available for this checkout. TopUp's baseline is preserved
// separately below; InvoiceOrder, InvoicePaymentOrder and User are unchanged.
type legacyInvoiceFeeApplication struct {
	Id                           int            `json:"id"`
	UserId                       int            `json:"user_id" gorm:"index"`
	Status                       string         `json:"status" gorm:"type:varchar(32);index"`
	PaymentStatus                string         `json:"payment_status" gorm:"type:varchar(32);index"`
	InvoiceTitle                 string         `json:"invoice_title" gorm:"type:varchar(255)"`
	TaxNumber                    string         `json:"tax_number" gorm:"type:varchar(64)"`
	CompanyAddress               string         `json:"company_address" gorm:"type:varchar(255)"`
	CompanyPhone                 string         `json:"company_phone" gorm:"type:varchar(64)"`
	BankName                     string         `json:"bank_name" gorm:"type:varchar(255)"`
	BankAccount                  string         `json:"bank_account" gorm:"type:varchar(128)"`
	RecipientEmail               string         `json:"recipient_email" gorm:"type:varchar(255)"`
	ApplicantNote                string         `json:"applicant_note" gorm:"type:text"`
	InvoiceItemName              string         `json:"invoice_item_name" gorm:"type:varchar(255)"`
	Currency                     string         `json:"currency" gorm:"type:varchar(8)"`
	OrderAmountCents             int64          `json:"order_amount_cents"`
	InvoiceAmountCents           int64          `json:"invoice_amount_cents"`
	EstimatedVATCents            int64          `json:"estimated_vat_cents"`
	EstimatedUrbanTaxCents       int64          `json:"estimated_urban_tax_cents"`
	EstimatedEducationCents      int64          `json:"estimated_education_surcharge_cents"`
	EstimatedLocalEducationCents int64          `json:"estimated_local_education_surcharge_cents"`
	EstimatedPITCents            int64          `json:"estimated_pit_withholding_cents"`
	EstimatedTotalTaxCents       int64          `json:"estimated_total_tax_cents"`
	SuggestedSupplementCents     int64          `json:"suggested_supplement_cents"`
	FinalSupplementCents         int64          `json:"final_supplement_cents"`
	TaxBreakdown                 string         `json:"-" gorm:"type:text"`
	RuleSnapshot                 string         `json:"-" gorm:"type:text"`
	TaxAdjustmentReason          string         `json:"tax_adjustment_reason" gorm:"type:text"`
	AdminNote                    string         `json:"admin_note" gorm:"type:text"`
	RejectReason                 string         `json:"reject_reason" gorm:"type:text"`
	ReviewerId                   int            `json:"reviewer_id"`
	ReviewedAt                   int64          `json:"reviewed_at"`
	PaymentTradeNo               string         `json:"payment_trade_no" gorm:"type:varchar(128);index"`
	PaymentMethod                string         `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentConfirmedAt           int64          `json:"payment_confirmed_at"`
	InvoiceFileName              string         `json:"invoice_file_name" gorm:"type:varchar(255)"`
	InvoiceFilePath              string         `json:"-" gorm:"type:text"`
	InvoiceFileContentType       string         `json:"invoice_file_content_type" gorm:"type:varchar(128)"`
	InvoiceEmailSentAt           int64          `json:"invoice_email_sent_at"`
	InvoiceEmailSendCount        int            `json:"invoice_email_send_count"`
	CreatedAt                    int64          `json:"created_at" gorm:"index"`
	UpdatedAt                    int64          `json:"updated_at"`
	IssuedAt                     int64          `json:"issued_at"`
	Orders                       []InvoiceOrder `json:"orders" gorm:"foreignKey:ApplicationId"`
}

func (legacyInvoiceFeeApplication) TableName() string { return "invoice_applications" }

type legacyInvoiceFeeTopUp struct {
	Id                int     `json:"id"`
	UserId            int     `json:"user_id" gorm:"index"`
	Amount            int64   `json:"amount"`
	Money             float64 `json:"money"`
	TradeNo           string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod     string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider   string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	CreateTime        int64   `json:"create_time"`
	CompleteTime      int64   `json:"complete_time"`
	Status            string  `json:"status"`
	InviterRewardSent bool    `json:"inviter_reward_sent" gorm:"default:false"`
}

func (legacyInvoiceFeeTopUp) TableName() string { return "top_ups" }

func TestInvoiceFeeMigrationFreshAndLegacy(t *testing.T) {
	for _, baseline := range []string{"fresh", "legacy_15227624e"} {
		t.Run(baseline, func(t *testing.T) {
			applicationModel := any(&InvoiceApplication{})
			if baseline != "fresh" {
				applicationModel = &legacyInvoiceFeeApplication{}
			}
			withCommerceDatabases(t, func(t *testing.T) {
				if baseline != "fresh" {
					// This table was created empty by our isolated fixture.
					require.NoError(t, DB.Migrator().DropTable(&TopUp{}))
					require.NoError(t, DB.AutoMigrate(&legacyInvoiceFeeTopUp{}))
				}
				assertInvoiceFeeMigrationPreservesData(t, baseline)
			}, applicationModel, &InvoiceOrder{}, &InvoicePaymentOrder{})
		})
	}
}

func assertInvoiceFeeMigrationPreservesData(t *testing.T, baseline string) {
	t.Helper()
	user := User{Username: "invoice-migration", AffCode: "invoice-migration", Group: "default", Quota: 12345678}
	require.NoError(t, DB.Create(&user).Error)
	topup := legacyInvoiceFeeTopUp{UserId: user.Id, Amount: 25000000, Money: 50, TradeNo: "preserved-topup", PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, CreateTime: 1700000000, CompleteTime: 1700000100, Status: common.TopUpStatusSuccess}
	require.NoError(t, DB.Create(&topup).Error)
	legacy := legacyInvoiceFeeApplication{
		UserId: user.Id, Status: InvoiceStatusPendingPayment, PaymentStatus: InvoicePaymentPending,
		InvoiceTitle: "迁移保留公司", TaxNumber: "913100000000000001", CompanyAddress: "上海市测试路 1 号", CompanyPhone: "02100000000",
		BankName: "测试银行", BankAccount: "1234567890", RecipientEmail: "billing@example.test", ApplicantNote: "legacy applicant note",
		InvoiceItemName: "技术服务费", Currency: "CNY", OrderAmountCents: 5000, InvoiceAmountCents: 5000,
		EstimatedVATCents: 100, EstimatedUrbanTaxCents: 7, EstimatedEducationCents: 3,
		EstimatedLocalEducationCents: 2, EstimatedPITCents: 38, EstimatedTotalTaxCents: 150,
		SuggestedSupplementCents: 150, FinalSupplementCents: 125, TaxBreakdown: `{"vat":100}`, RuleSnapshot: `{"legacy":true}`,
		TaxAdjustmentReason: "manual review adjustment", AdminNote: "reviewed", ReviewerId: 99, ReviewedAt: 1700000200,
		PaymentTradeNo: "preserved-payment", PaymentMethod: "alipay", CreatedAt: 1700000110, UpdatedAt: 1700000200,
		Orders: []InvoiceOrder{{TopUpId: topup.Id, TradeNo: topup.TradeNo, PaidAmountCents: 5000, PaymentMethod: topup.PaymentMethod, CompletedAt: topup.CompleteTime}},
	}
	require.NoError(t, DB.Create(&legacy).Error)
	issued := legacyInvoiceFeeApplication{
		UserId: user.Id, Status: InvoiceStatusIssued, PaymentStatus: InvoicePaymentPaid,
		InvoiceTitle: "Previously issued", Currency: "USD", OrderAmountCents: 10000, InvoiceAmountCents: 10000,
		FinalSupplementCents: 300, PaymentTradeNo: "issued-payment", PaymentMethod: PaymentMethodStripe,
		PaymentConfirmedAt: 1699900200, InvoiceFileName: "issued.pdf", InvoiceFilePath: "invoice/history/issued.pdf",
		InvoiceFileContentType: "application/pdf", InvoiceEmailSentAt: 1699900300, InvoiceEmailSendCount: 2,
		IssuedAt: 1699900300, CreatedAt: 1699900000, UpdatedAt: 1699900300,
	}
	require.NoError(t, DB.Create(&issued).Error)
	payment := InvoicePaymentOrder{
		ApplicationId: legacy.Id, UserId: user.Id, TradeNo: legacy.PaymentTradeNo, AmountCents: 125, Currency: "CNY",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, Status: InvoicePaymentOrderPending,
		ProviderPayload: `{"checkout_id":"old-checkout"}`, CreateTime: 1700000300,
	}
	require.NoError(t, DB.Create(&payment).Error)
	paidPayment := InvoicePaymentOrder{
		ApplicationId: issued.Id, UserId: user.Id, TradeNo: issued.PaymentTradeNo, AmountCents: 300, Currency: "USD",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, Status: InvoicePaymentOrderPaid,
		ProviderPayload: `{"payment_id":"old-paid"}`, CreateTime: 1699900100, CompleteTime: 1699900200, UsageLogRecordedAt: 1699900250,
	}
	require.NoError(t, DB.Create(&paidPayment).Error)
	indexesBefore := invoiceMigrationIndexes(t, DB)
	recorder := &migrationSQLRecorder{}
	migrationDB := DB.Session(&gorm.Session{Logger: recorder})
	// Measure unchanged legacy migration first. The current SQLite driver
	// rebuilds the two unchanged unique-index tables on every AutoMigrate;
	// this feature must not add repeated DDL to that existing behavior.
	require.NoError(t, migrationDB.AutoMigrate(&legacyInvoiceFeeApplication{}, &InvoiceOrder{}, &InvoicePaymentOrder{}, &legacyInvoiceFeeTopUp{}))
	baselineMutations := recorder.schemaMutations()
	t.Logf("unchanged legacy migration: %d schema mutations", len(baselineMutations))
	var firstPassColumns map[string][]string
	snapshot := InvoiceApplication{
		UserId: user.Id, Status: InvoiceStatusRejected, PaymentStatus: InvoicePaymentRefunded,
		Currency: "CNY", CurrencySymbol: "元", FeeMode: InvoiceFeeModeBalanceOnApply, FeeRateBasisPoints: 300,
		InvoiceAmountCents: 20000000, FinalSupplementCents: 600000, ChargedQuota: 3000000000, RefundedQuota: 3000000000,
		CanCancel: true,
	}
	paidSnapshot := TopUp{UserId: user.Id, Amount: 9, Money: 90, TradeNo: "verified-new-payment", PaymentProvider: PaymentProviderStripe, Status: common.TopUpStatusSuccess, PaidAmountMinor: 3000000000, PaidCurrency: "JPY"}
	for pass := 1; pass <= 2; pass++ {
		recorder.reset()
		require.NoError(t, migrationDB.AutoMigrate(&InvoiceApplication{}, &InvoiceOrder{}, &InvoicePaymentOrder{}, &TopUp{}))
		mutations := recorder.schemaMutations()
		t.Logf("%s migration pass %d: %d schema mutations", baseline, pass, len(mutations))
		if baseline == "fresh" || pass == 2 {
			assert.Equal(t, invoiceMigrationMutationTargets(baselineMutations), invoiceMigrationMutationTargets(mutations), "new fields must not introduce additional repeated table rewrites beyond the unchanged legacy migration")
		} else {
			assert.NotEmpty(t, mutations, "legacy fixture must exercise the actual upgrade")
		}
		assert.Equal(t, indexesBefore, invoiceMigrationIndexes(t, DB), "existing indexes must survive migration")
		columns := invoiceMigrationColumns(t, DB)
		if pass == 1 {
			firstPassColumns = columns
			require.NoError(t, DB.Create(&snapshot).Error)
			require.NoError(t, DB.Create(&paidSnapshot).Error)
		} else {
			assert.Equal(t, firstPassColumns, columns, "a repeated migration must preserve every column and default")
		}
		assert.False(t, DB.Migrator().HasColumn("invoice_applications", "can_cancel"), "cancellation hints must not create a database column")
		for _, expected := range []legacyInvoiceFeeApplication{legacy, issued} {
			var preserved legacyInvoiceFeeApplication
			require.NoError(t, DB.Preload("Orders").First(&preserved, expected.Id).Error)
			// GORM preloads an empty relation as an empty slice.
			if len(expected.Orders) == 0 {
				expected.Orders = []InvoiceOrder{}
			}
			assert.Equal(t, expected, preserved, "every legacy invoice column and associated order must be preserved")
			var upgraded InvoiceApplication
			require.NoError(t, DB.First(&upgraded, expected.Id).Error)
			assert.Empty(t, upgraded.CurrencySymbol)
			assert.Empty(t, upgraded.FeeMode)
			assert.Zero(t, upgraded.FeeRateBasisPoints)
			assert.Zero(t, upgraded.ChargedQuota)
			assert.Zero(t, upgraded.RefundedQuota)
			var nonzeroSnapshots int64
			require.NoError(t, DB.Model(&InvoiceApplication{}).Where("id = ? AND (currency_symbol IS NULL OR currency_symbol <> '' OR fee_mode IS NULL OR fee_mode <> '' OR fee_rate_basis_points IS NULL OR fee_rate_basis_points <> 0 OR charged_quota IS NULL OR charged_quota <> 0 OR refunded_quota IS NULL OR refunded_quota <> 0)", expected.Id).Count(&nonzeroSnapshots).Error)
			assert.Zero(t, nonzeroSnapshots, "migration must persist explicit empty/zero fee snapshots, without inventing a charge or refund")
			require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
				refund, err := RefundInvoiceFeeTx(tx, &upgraded)
				assert.Zero(t, refund, "legacy external payments must not become wallet refunds")
				return err
			}))
		}
		var preservedUser User
		require.NoError(t, DB.First(&preservedUser, user.Id).Error)
		assert.Equal(t, user.Quota, preservedUser.Quota)
		var preservedTopup legacyInvoiceFeeTopUp
		require.NoError(t, DB.First(&preservedTopup, topup.Id).Error)
		assert.Equal(t, topup, preservedTopup)
		var upgradedTopUp TopUp
		require.NoError(t, DB.First(&upgradedTopUp, topup.Id).Error)
		assert.Zero(t, upgradedTopUp.PaidAmountMinor)
		assert.Empty(t, upgradedTopUp.PaidCurrency)
		var inventedReceipts int64
		require.NoError(t, DB.Model(&TopUp{}).Where("id = ? AND (paid_amount_minor IS NULL OR paid_amount_minor <> 0 OR paid_currency IS NULL OR paid_currency <> '')", topup.Id).Count(&inventedReceipts).Error)
		assert.Zero(t, inventedReceipts, "migration must not invent a receipt from legacy Money")
		for _, expected := range []InvoicePaymentOrder{payment, paidPayment} {
			var preserved InvoicePaymentOrder
			require.NoError(t, DB.First(&preserved, expected.Id).Error)
			assert.Equal(t, expected, preserved)
		}
		duplicateOrder := InvoiceOrder{ApplicationId: issued.Id, TopUpId: topup.Id, TradeNo: "different-order", PaidAmountCents: 1}
		assert.ErrorContains(t, DB.Create(&duplicateOrder).Error, "top_up_id", "one paid order must not acquire a second invoice association")
		duplicatePayment := InvoicePaymentOrder{ApplicationId: issued.Id, UserId: user.Id, TradeNo: payment.TradeNo, AmountCents: 1}
		assert.ErrorContains(t, DB.Create(&duplicatePayment).Error, "trade_no", "payment trade numbers must stay unique")
		duplicateTopUp := TopUp{UserId: user.Id, TradeNo: topup.TradeNo, Amount: 1, Money: 1}
		assert.ErrorContains(t, DB.Create(&duplicateTopUp).Error, "trade_no", "top-up trade numbers must stay unique after receipt columns are added")
	}
	var preservedPaidSnapshot TopUp
	require.NoError(t, DB.First(&preservedPaidSnapshot, paidSnapshot.Id).Error)
	assert.Equal(t, paidSnapshot, preservedPaidSnapshot, "verified payment snapshots and nominal credit fields must survive a second migration")
	var preservedSnapshot InvoiceApplication
	require.NoError(t, DB.First(&preservedSnapshot, snapshot.Id).Error)
	assert.Equal(t, snapshot.CurrencySymbol, preservedSnapshot.CurrencySymbol)
	assert.Equal(t, snapshot.FeeMode, preservedSnapshot.FeeMode)
	assert.Equal(t, snapshot.FeeRateBasisPoints, preservedSnapshot.FeeRateBasisPoints)
	assert.Equal(t, snapshot.ChargedQuota, preservedSnapshot.ChargedQuota, "wallet snapshots must preserve values above int32")
	assert.Equal(t, snapshot.RefundedQuota, preservedSnapshot.RefundedQuota)
	assert.False(t, preservedSnapshot.CanCancel, "the per-request cancellation hint must not be persisted")
}

// Read engine metadata rather than Go tags so SQLite table rebuilds or missing
// database indexes cannot pass merely because the model still declares an index.
func invoiceMigrationIndexes(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	result := make(map[string][]string)
	for _, table := range []string{"invoice_applications", "invoice_orders", "invoice_payment_orders", "users", "top_ups"} {
		var names []string
		switch common.MainDatabaseType() {
		case common.DatabaseTypeSQLite:
			require.NoError(t, db.Raw("SELECT name FROM pragma_index_list(?) ORDER BY name", table).Scan(&names).Error)
		case common.DatabaseTypeMySQL:
			require.NoError(t, db.Raw("SELECT DISTINCT index_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? ORDER BY index_name", table).Scan(&names).Error)
		case common.DatabaseTypePostgreSQL:
			require.NoError(t, db.Raw("SELECT indexname FROM pg_indexes WHERE schemaname = current_schema() AND tablename = ? ORDER BY indexname", table).Scan(&names).Error)
		default:
			t.Fatalf("unsupported invoice migration database: %s", common.MainDatabaseType())
		}
		result[table] = names
	}
	return result
}

func invoiceMigrationColumns(t *testing.T, db *gorm.DB) map[string][]string {
	t.Helper()
	result := make(map[string][]string)
	for _, table := range []string{"invoice_applications", "invoice_orders", "invoice_payment_orders", "users", "top_ups"} {
		columns, err := db.Migrator().ColumnTypes(table)
		require.NoError(t, err)
		for _, column := range columns {
			nullable, hasNullable := column.Nullable()
			defaultValue, hasDefault := column.DefaultValue()
			result[table] = append(result[table], fmt.Sprintf("%s:%s:null=%t/%t:default=%s/%t", column.Name(), column.DatabaseTypeName(), nullable, hasNullable, defaultValue, hasDefault))
		}
	}
	return result
}

// Compare DDL operations and targets: an unchanged SQLite index rebuild may
// include the newly added columns in its CREATE statement after the upgrade.
func invoiceMigrationMutationTargets(statements []string) []string {
	result := make([]string, 0, len(statements))
	for _, statement := range statements {
		words := strings.Fields(statement)
		result = append(result, strings.Join(words[:3], " "))
	}
	return result
}

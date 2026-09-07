package model

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	InvoiceStatusPendingReview  = "pending_review"
	InvoiceStatusPendingPayment = "pending_payment"
	InvoiceStatusApproved       = "approved"
	InvoiceStatusRejected       = "rejected"
	InvoiceStatusIssued         = "issued"

	InvoicePaymentNotRequired = "not_required"
	InvoicePaymentPending     = "pending"
	InvoicePaymentPaid        = "paid"
	InvoicePaymentRefunded    = "refunded"

	InvoiceFeeModeBalanceOnApply = "balance_on_apply"

	InvoicePaymentOrderPending = "pending"
	InvoicePaymentOrderPaid    = "paid"
	InvoicePaymentOrderFailed  = "failed"
	InvoicePaymentOrderExpired = "expired"
)

var (
	ErrInvoiceNotFound              = errors.New("invoice application not found")
	ErrInvoiceOrderAlreadyApplied   = errors.New("one or more paid orders already have an invoice application")
	ErrInvoiceStatusInvalid         = errors.New("invoice application status invalid")
	ErrInvoicePaymentRequired       = errors.New("invoice tax supplement payment is required")
	ErrInvoicePaymentOrderNotFound  = errors.New("invoice payment order not found")
	ErrInvoicePaymentOrderInvalid   = errors.New("invoice payment order status invalid")
	ErrInvoicePaymentMethodMismatch = errors.New("invoice payment method mismatch")
	ErrWalletQuotaInsufficient      = errors.New("额度不足")
)

type InvoiceApplication struct {
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
	CurrencySymbol               string         `json:"currency_symbol" gorm:"type:varchar(64);not null;default:''"`
	FeeMode                      string         `json:"fee_mode" gorm:"type:varchar(32);not null;default:''"`
	FeeRateBasisPoints           int            `json:"fee_rate_basis_points" gorm:"type:int;not null;default:0"`
	ChargedQuota                 int            `json:"charged_quota" gorm:"type:bigint;not null;default:0"`
	RefundedQuota                int            `json:"refunded_quota" gorm:"type:bigint;not null;default:0"`
	CanCancel                    bool           `json:"can_cancel" gorm:"-"`
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

type InvoiceOrder struct {
	Id              int    `json:"id"`
	ApplicationId   int    `json:"application_id" gorm:"index"`
	TopUpId         int    `json:"top_up_id" gorm:"uniqueIndex"`
	TradeNo         string `json:"trade_no" gorm:"type:varchar(128)"`
	PaidAmountCents int64  `json:"paid_amount_cents"`
	PaymentMethod   string `json:"payment_method" gorm:"type:varchar(50)"`
	CompletedAt     int64  `json:"completed_at"`
}

type InvoicePaymentOrder struct {
	Id                 int    `json:"id"`
	ApplicationId      int    `json:"application_id" gorm:"index"`
	UserId             int    `json:"user_id" gorm:"index"`
	TradeNo            string `json:"trade_no" gorm:"type:varchar(128);uniqueIndex"`
	AmountCents        int64  `json:"amount_cents"`
	Currency           string `json:"currency" gorm:"type:varchar(8)"`
	PaymentMethod      string `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider    string `json:"payment_provider" gorm:"type:varchar(50)"`
	Status             string `json:"status" gorm:"type:varchar(32);index"`
	ProviderPayload    string `json:"provider_payload" gorm:"type:text"`
	CreateTime         int64  `json:"create_time"`
	CompleteTime       int64  `json:"complete_time"`
	UsageLogRecordedAt int64  `json:"usage_log_recorded_at"`
}

func ListInvoiceApplications(userId int, status string, keyword string, pageInfo *common.PageInfo) ([]*InvoiceApplication, int64, error) {
	query := DB.Model(&InvoiceApplication{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		if numericKeyword, err := strconv.Atoi(keyword); err == nil && numericKeyword > 0 {
			query = query.Where("id = ? OR user_id = ? OR invoice_title LIKE ? OR tax_number LIKE ? OR recipient_email LIKE ?", numericKeyword, numericKeyword, like, like, like)
		} else {
			query = query.Where("invoice_title LIKE ? OR tax_number LIKE ? OR recipient_email LIKE ?", like, like, like)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var applications []*InvoiceApplication
	err := query.Preload("Orders").Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&applications).Error
	return applications, total, err
}

func GetInvoiceApplication(id int, userId int) (*InvoiceApplication, error) {
	query := DB.Preload("Orders").Where("id = ?", id)
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	var application InvoiceApplication
	if err := query.First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	return &application, nil
}

func UpdateInvoiceApplication(id int, update func(*gorm.DB, *InvoiceApplication) error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var application InvoiceApplication
		if err := lockForUpdate(tx).Where("id = ?", id).First(&application).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvoiceNotFound
			}
			return err
		}
		return update(tx, &application)
	})
}

func GetInvoiceApplicationForUpdate(tx *gorm.DB, id int) (*InvoiceApplication, error) {
	if tx == nil {
		return nil, errors.New("invoice transaction is required")
	}
	var application InvoiceApplication
	if err := lockForUpdate(tx).Where("id = ?", id).First(&application).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	return &application, nil
}

func DeleteInvoiceApplication(id int) (*InvoiceApplication, error) {
	application, _, err := DeleteInvoiceApplicationForUser(id, 0)
	return application, err
}

// DeleteInvoiceApplicationForUser cancels an unissued application, returns its
// original wallet fee, and releases its orders in the same transaction. A zero
// userId is reserved for the administrator service; self-service callers pass
// their authenticated user ID. The second return value is the committed refund.
func DeleteInvoiceApplicationForUser(id int, userId int) (*InvoiceApplication, int, error) {
	if id <= 0 || userId < 0 {
		return nil, 0, ErrInvoiceNotFound
	}
	var deleted InvoiceApplication
	refundedQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		application, err := GetInvoiceApplicationForUpdate(tx, id)
		if err != nil {
			return err
		}
		if userId > 0 && application.UserId != userId {
			return ErrInvoiceNotFound
		}
		deletable, err := canCancelInvoiceApplication(tx, application)
		if err != nil {
			return err
		}
		if !deletable {
			return ErrInvoiceStatusInvalid
		}
		refundedQuota, err = RefundInvoiceFeeTx(tx, application)
		if err != nil {
			return err
		}
		deleted = *application

		if err = tx.Where("application_id = ?", id).Delete(&InvoicePaymentOrder{}).Error; err != nil {
			return err
		}
		if err = tx.Where("application_id = ?", id).Delete(&InvoiceOrder{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&InvoiceApplication{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrInvoiceNotFound
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return &deleted, refundedQuota, nil
}

// CanCancelInvoiceApplication is a display hint; deletion repeats this check
// under the application lock before returning any money or releasing orders.
func CanCancelInvoiceApplication(application *InvoiceApplication) (bool, error) {
	return canCancelInvoiceApplication(DB, application)
}

func canCancelInvoiceApplication(tx *gorm.DB, application *InvoiceApplication) (bool, error) {
	deletable := application.Status == InvoiceStatusPendingReview || application.Status == InvoiceStatusRejected
	if application.FeeMode == InvoiceFeeModeBalanceOnApply {
		// Upload precedes email delivery. A prepared invoice must remain protected
		// even when its delivery has failed and status is still approved.
		deletable = deletable || application.Status == InvoiceStatusApproved
		return deletable && application.IssuedAt == 0 && application.InvoiceFilePath == "" && application.InvoiceEmailSentAt == 0, nil
	}
	if application.Status == InvoiceStatusPendingPayment && application.PaymentStatus == InvoicePaymentPending && application.PaymentConfirmedAt == 0 {
		var activePayments int64
		if err := tx.Model(&InvoicePaymentOrder{}).
			Where("application_id = ? AND status IN ?", application.Id, []string{InvoicePaymentOrderPending, InvoicePaymentOrderPaid}).
			Count(&activePayments).Error; err != nil {
			return false, err
		}
		return activePayments == 0, nil
	}
	return deletable, nil
}

// RefundInvoiceFeeTx refunds only the recorded balance fee, never a legacy
// external payment. Callers must hold the application lock and change its state
// in the same transaction. RefundedQuota makes rejection followed by deletion
// idempotent and keeps refunds independent from later currency/rate changes.
func RefundInvoiceFeeTx(tx *gorm.DB, application *InvoiceApplication) (int, error) {
	if tx == nil || application == nil || application.Id <= 0 || application.UserId <= 0 {
		return 0, errors.New("invalid invoice fee refund")
	}
	if application.FeeMode != InvoiceFeeModeBalanceOnApply {
		return 0, nil
	}
	if application.Status == InvoiceStatusIssued || application.IssuedAt > 0 || application.InvoiceFilePath != "" || application.InvoiceEmailSentAt > 0 {
		return 0, ErrInvoiceStatusInvalid
	}
	if application.ChargedQuota < 0 || application.ChargedQuota > common.MaxWalletQuota || application.RefundedQuota < 0 || application.RefundedQuota > application.ChargedQuota {
		return 0, errors.New("invalid invoice fee refund amount")
	}
	refund := application.ChargedQuota - application.RefundedQuota
	if refund == 0 {
		return 0, nil
	}
	var user User
	if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", application.UserId).First(&user).Error; err != nil {
		return 0, err
	}
	result := tx.Model(&User{}).Where("id = ? AND quota <= ?", application.UserId, common.MaxWalletQuota-refund).
		Update("quota", gorm.Expr("quota + ?", refund))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, ErrWalletQuotaLimitExceeded
	}
	if err := tx.Model(application).Updates(map[string]interface{}{
		"refunded_quota": application.ChargedQuota,
		"payment_status": InvoicePaymentRefunded,
		"updated_at":     common.GetTimestamp(),
	}).Error; err != nil {
		return 0, err
	}
	application.RefundedQuota = application.ChargedQuota
	application.PaymentStatus = InvoicePaymentRefunded
	return refund, nil
}

func GetInvoicePaymentOrderByTradeNo(tradeNo string) (*InvoicePaymentOrder, error) {
	var order InvoicePaymentOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvoicePaymentOrderNotFound
		}
		return nil, err
	}
	return &order, nil
}

func UpdateInvoicePaymentOrder(tradeNo string, update func(*gorm.DB, *InvoicePaymentOrder) error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var order InvoicePaymentOrder
		if err := lockForUpdate(tx).Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvoicePaymentOrderNotFound
			}
			return err
		}
		return update(tx, &order)
	})
}

// ChargeUserQuotaTx atomically checks and deducts wallet quota in an existing
// transaction. The caller must update its business record in the same tx.
func ChargeUserQuotaTx(tx *gorm.DB, userId int, quota int) error {
	if tx == nil || userId <= 0 || quota <= 0 {
		return errors.New("invalid wallet quota charge")
	}
	var user User
	if err := lockForUpdate(tx).Select("id", "quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	if user.Quota < quota {
		return ErrWalletQuotaInsufficient
	}
	result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, quota).
		Update("quota", gorm.Expr("quota - ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrWalletQuotaInsufficient
	}
	return nil
}

// SyncUserQuotaCacheDelta applies a committed wallet delta to Redis when enabled.
func SyncUserQuotaCacheDelta(userId int, delta int) error {
	if !common.RedisEnabled || delta == 0 {
		return nil
	}
	_, err := cacheApplyUserQuotaDelta(userId, int64(delta))
	return err
}

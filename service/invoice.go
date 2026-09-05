package service

import (
	"errors"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const maxInvoiceOrdersPerApplication = 100
const maxInvoiceAmountCents = int64(100_000_000_000)
const InvoiceFileMaxBytes = int64(20 << 20)

var ErrInvoiceFileUnavailable = errors.New("invoice file is not available")

var invoiceFileContentTypes = map[string]string{
	"application/pdf": ".pdf",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
}

var invoiceEmailSender = common.SendEmailWithAttachments

type InvoiceFileContent struct {
	FileName    string
	ContentType string
	Data        []byte
}

type invoiceLocalizedCopy struct {
	EmailSubject                string
	EmailAttached               string
	EmailTitle                  string
	EmailItem                   string
	EmailSentBy                 string
	SupplementPaymentName       string
	SupplementPaymentLogContent string
}

var invoiceLocalizedCopies = map[string]invoiceLocalizedCopy{
	"en": {
		EmailSubject:                "Invoice #%d",
		EmailAttached:               "Your invoice is attached.",
		EmailTitle:                  "Invoice title",
		EmailItem:                   "Invoice item",
		EmailSentBy:                 "This email was sent by %s.",
		SupplementPaymentName:       "Invoice tax supplement #%d",
		SupplementPaymentLogContent: "Invoice supplement payment completed: application #%d, amount %s %s.",
	},
	"zh": {
		EmailSubject:                "发票 #%d",
		EmailAttached:               "您的发票已随邮件附上。",
		EmailTitle:                  "发票抬头",
		EmailItem:                   "发票项目",
		EmailSentBy:                 "此邮件由 %s 发送。",
		SupplementPaymentName:       "发票补税 #%d",
		SupplementPaymentLogContent: "发票补税支付成功：申请 #%d，金额 %s %s。",
	},
	"zh-TW": {
		EmailSubject:                "發票 #%d",
		EmailAttached:               "您的發票已隨郵件附上。",
		EmailTitle:                  "發票抬頭",
		EmailItem:                   "發票項目",
		EmailSentBy:                 "此郵件由 %s 傳送。",
		SupplementPaymentName:       "發票補稅 #%d",
		SupplementPaymentLogContent: "發票補稅付款成功：申請 #%d，金額 %s %s。",
	},
	"fr": {
		EmailSubject:                "Facture n° %d",
		EmailAttached:               "Votre facture est jointe à cet e-mail.",
		EmailTitle:                  "Intitulé de la facture",
		EmailItem:                   "Prestation facturée",
		EmailSentBy:                 "Cet e-mail a été envoyé par %s.",
		SupplementPaymentName:       "Complément de taxe de facture n° %d",
		SupplementPaymentLogContent: "Paiement du complément de taxe effectué : demande n° %d, montant %s %s.",
	},
	"ja": {
		EmailSubject:                "請求書 #%d",
		EmailAttached:               "請求書を添付しました。",
		EmailTitle:                  "請求書の宛名",
		EmailItem:                   "請求項目",
		EmailSentBy:                 "このメールは %s から送信されました。",
		SupplementPaymentName:       "請求書の追加税額 #%d",
		SupplementPaymentLogContent: "請求書の税額追加分を支払いました：申請 #%d、金額 %s %s。",
	},
	"ru": {
		EmailSubject:                "Счёт-фактура №%d",
		EmailAttached:               "Счёт-фактура прикреплён к письму.",
		EmailTitle:                  "Наименование покупателя",
		EmailItem:                   "Предмет счёта",
		EmailSentBy:                 "Письмо отправлено системой %s.",
		SupplementPaymentName:       "Доплата налога по счёту №%d",
		SupplementPaymentLogContent: "Доплата налога по счёту выполнена: заявка №%d, сумма %s %s.",
	},
	"vi": {
		EmailSubject:                "Hóa đơn #%d",
		EmailAttached:               "Hóa đơn của bạn được đính kèm trong email này.",
		EmailTitle:                  "Tên trên hóa đơn",
		EmailItem:                   "Hạng mục hóa đơn",
		EmailSentBy:                 "Email này được gửi bởi %s.",
		SupplementPaymentName:       "Thuế bổ sung hóa đơn #%d",
		SupplementPaymentLogContent: "Đã thanh toán phần thuế bổ sung của hóa đơn: đơn #%d, số tiền %s %s.",
	},
}

func normalizeInvoiceLanguage(language string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(language), "_", "-"))
	switch {
	case normalized == "zh-tw", normalized == "zh-hk", normalized == "zh-hant":
		return "zh-TW"
	case normalized == "en" || strings.HasPrefix(normalized, "en-"):
		return "en"
	case normalized == "fr" || strings.HasPrefix(normalized, "fr-"):
		return "fr"
	case normalized == "ja" || strings.HasPrefix(normalized, "ja-"):
		return "ja"
	case normalized == "ru" || strings.HasPrefix(normalized, "ru-"):
		return "ru"
	case normalized == "vi" || strings.HasPrefix(normalized, "vi-"):
		return "vi"
	default:
		return "zh"
	}
}

func normalizeInvoiceApplicationItemName(application *model.InvoiceApplication) {
	if application != nil && application.Status != model.InvoiceStatusIssued && application.InvoiceItemName == "技术服务费" {
		application.InvoiceItemName = "AI Agent服务"
	}
}

func invoiceEmailMessage(language string, application *model.InvoiceApplication) (string, string) {
	copy := invoiceLocalizedCopies[normalizeInvoiceLanguage(language)]
	normalizeInvoiceApplicationItemName(application)
	subject := fmt.Sprintf(copy.EmailSubject, application.Id)
	content := fmt.Sprintf(
		"<p>%s</p><p>%s: %s</p><p>%s: %s</p><p>%s</p>",
		copy.EmailAttached,
		copy.EmailTitle,
		html.EscapeString(application.InvoiceTitle),
		copy.EmailItem,
		html.EscapeString(application.InvoiceItemName),
		fmt.Sprintf(copy.EmailSentBy, html.EscapeString(common.SystemName)),
	)
	return subject, content
}

func InvoiceSupplementPaymentName(userId int, applicationId int) string {
	copy := invoiceLocalizedCopies[normalizeInvoiceLanguage(model.GetUserLanguage(userId))]
	return fmt.Sprintf(copy.SupplementPaymentName, applicationId)
}

type InvoicePublicConfig struct {
	Enabled            bool    `json:"enabled"`
	MinimumAmount      float64 `json:"minimum_amount"`
	Currency           string  `json:"currency"`
	FeeRateBasisPoints int     `json:"fee_rate_basis_points"`
	ExchangeRate       float64 `json:"exchange_rate"`
	QuotaPerUnit       float64 `json:"quota_per_unit"`
}

type CreateInvoiceApplicationInput struct {
	TopUpIds       []int
	InvoiceTitle   string
	TaxNumber      string
	RecipientEmail string
	ApplicantNote  string
}

func GetInvoiceConfig() InvoicePublicConfig {
	setting := invoice_setting.GetInvoiceSetting()
	currency := strings.ToUpper(strings.TrimSpace(operation_setting.GetQuotaDisplayType()))
	return InvoicePublicConfig{
		Enabled:            setting.Enabled && (currency == "USD" || currency == "CNY"),
		MinimumAmount:      setting.MinimumAmount,
		Currency:           currency,
		FeeRateBasisPoints: setting.FeeRateBasisPoints,
		ExchangeRate:       operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate),
		QuotaPerUnit:       common.QuotaPerUnit,
	}
}

func ListInvoiceApplications(userId int, status string, keyword string, pageInfo *common.PageInfo) ([]*model.InvoiceApplication, int64, error) {
	applications, total, err := model.ListInvoiceApplications(userId, status, keyword, pageInfo)
	if err != nil {
		return nil, 0, err
	}
	for _, application := range applications {
		normalizeInvoiceApplicationItemName(application)
	}
	return applications, total, nil
}

func GetInvoiceApplication(applicationId int, userId int) (*model.InvoiceApplication, error) {
	application, err := model.GetInvoiceApplication(applicationId, userId)
	if err != nil {
		return nil, err
	}
	normalizeInvoiceApplicationItemName(application)
	return application, nil
}

func validateInvoiceRecipientEmail(value string) (string, error) {
	recipient := strings.TrimSpace(value)
	if recipient == "" {
		return "", errors.New("invoice recipient email is required")
	}
	if len(recipient) > 255 {
		return "", errors.New("invoice recipient email is too long")
	}
	address, err := mail.ParseAddress(recipient)
	if err != nil || !strings.EqualFold(address.Address, recipient) {
		return "", errors.New("invoice recipient email is invalid")
	}
	return recipient, nil
}

func ListEligibleInvoiceOrders(userId int) ([]model.TopUp, error) {
	setting := invoice_setting.GetInvoiceSetting()
	if !setting.Enabled {
		return []model.TopUp{}, nil
	}

	config := GetInvoiceConfig()
	query := model.DB.Where("user_id = ? AND status = ? AND money > ? AND settlement_currency = ? AND settlement_exchange_rate > 0 AND settlement_quota_per_unit > 0",
		userId, common.TopUpStatusSuccess, 0, config.Currency)
	if setting.ApplicationWindowDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -setting.ApplicationWindowDays).Unix()
		query = query.Where("CASE WHEN complete_time > 0 THEN complete_time ELSE create_time END >= ?", cutoff)
	}

	var topUps []model.TopUp
	if err := query.Order("id desc").Limit(500).Find(&topUps).Error; err != nil {
		return nil, err
	}
	if len(topUps) == 0 {
		return topUps, nil
	}

	topUpIds := make([]int, 0, len(topUps))
	for _, topUp := range topUps {
		topUpIds = append(topUpIds, topUp.Id)
	}
	var usedIds []int
	if err := model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id IN ?", topUpIds).Pluck("top_up_id", &usedIds).Error; err != nil {
		return nil, err
	}
	used := make(map[int]struct{}, len(usedIds))
	for _, id := range usedIds {
		used[id] = struct{}{}
	}
	eligible := make([]model.TopUp, 0, len(topUps))
	for _, topUp := range topUps {
		if _, exists := used[topUp.Id]; !exists {
			eligible = append(eligible, topUp)
		}
	}
	return eligible, nil
}

func CreateInvoiceApplication(userId int, input CreateInvoiceApplicationInput) (*model.InvoiceApplication, error) {
	setting := invoice_setting.GetInvoiceSetting()
	if !setting.Enabled {
		return nil, errors.New("invoice applications are disabled")
	}
	config := GetInvoiceConfig()
	if !config.Enabled || config.ExchangeRate <= 0 || config.QuotaPerUnit <= 0 ||
		math.IsNaN(config.ExchangeRate) || math.IsInf(config.ExchangeRate, 0) ||
		math.IsNaN(config.QuotaPerUnit) || math.IsInf(config.QuotaPerUnit, 0) ||
		setting.FeeRateBasisPoints < 0 || setting.FeeRateBasisPoints > 10000 {
		return nil, errors.New("invoice settlement settings are invalid")
	}
	if len(input.TopUpIds) == 0 || len(input.TopUpIds) > maxInvoiceOrdersPerApplication {
		return nil, fmt.Errorf("select between 1 and %d paid orders", maxInvoiceOrdersPerApplication)
	}
	applicantNote := strings.TrimSpace(input.ApplicantNote)
	if len([]rune(applicantNote)) > 2000 {
		return nil, errors.New("invoice application note must be 2000 characters or fewer")
	}
	recipientEmail, err := validateInvoiceRecipientEmail(input.RecipientEmail)
	if err != nil {
		return nil, err
	}

	uniqueIds := make([]int, 0, len(input.TopUpIds))
	seen := make(map[int]struct{}, len(input.TopUpIds))
	for _, id := range input.TopUpIds {
		if id <= 0 {
			return nil, errors.New("invalid paid order")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIds = append(uniqueIds, id)
	}

	var created model.InvoiceApplication
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var topUps []model.TopUp
		if err := tx.Where("id IN ? AND user_id = ? AND status = ? AND money > ? AND settlement_currency = ? AND settlement_exchange_rate = ? AND settlement_quota_per_unit = ?",
			uniqueIds, userId, common.TopUpStatusSuccess, 0, config.Currency, config.ExchangeRate, config.QuotaPerUnit).
			Order("id asc").Find(&topUps).Error; err != nil {
			return err
		}
		if len(topUps) != len(uniqueIds) {
			return errors.New("one or more paid orders are invalid")
		}

		if setting.ApplicationWindowDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -setting.ApplicationWindowDays).Unix()
			for _, topUp := range topUps {
				completedAt := topUp.CompleteTime
				if completedAt <= 0 {
					completedAt = topUp.CreateTime
				}
				if completedAt < cutoff {
					return errors.New("one or more paid orders are outside the invoice application window")
				}
			}
		}

		var existing int64
		if err := tx.Model(&model.InvoiceOrder{}).Where("top_up_id IN ?", uniqueIds).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return model.ErrInvoiceOrderAlreadyApplied
		}

		var totalCents int64
		orders := make([]model.InvoiceOrder, 0, len(topUps))
		for _, topUp := range topUps {
			if math.IsNaN(topUp.Money) || math.IsInf(topUp.Money, 0) || topUp.Money <= 0 {
				return errors.New("paid order amount is invalid")
			}
			paidCents := decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
			if paidCents <= 0 || totalCents > maxInvoiceAmountCents-paidCents {
				return errors.New("paid order amount is invalid")
			}
			totalCents += paidCents
			completedAt := topUp.CompleteTime
			if completedAt <= 0 {
				completedAt = topUp.CreateTime
			}
			orders = append(orders, model.InvoiceOrder{
				TopUpId:         topUp.Id,
				TradeNo:         topUp.TradeNo,
				PaidAmountCents: paidCents,
				PaymentMethod:   topUp.PaymentMethod,
				CompletedAt:     completedAt,
			})
		}

		minimumCents := decimal.NewFromFloat(setting.MinimumAmount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
		if totalCents < minimumCents {
			return errors.New("selected paid orders do not meet the minimum invoice amount")
		}
		feeCents := decimal.NewFromInt(totalCents).Mul(decimal.NewFromInt(int64(setting.FeeRateBasisPoints))).Div(decimal.NewFromInt(10000)).Round(0).IntPart()
		feeQuotaDecimal := decimal.NewFromInt(feeCents).Div(decimal.NewFromInt(100)).
			Div(decimal.NewFromFloat(config.ExchangeRate)).Mul(decimal.NewFromFloat(config.QuotaPerUnit))
		feeQuota, err := common.WalletQuotaFromDecimalStrict(feeQuotaDecimal)
		if err != nil || feeQuota < 0 {
			return errors.New("invoice fee cannot be converted to wallet quota")
		}
		if feeQuota > 0 {
			if err := model.ChargeUserQuotaTx(tx, userId, feeQuota); err != nil {
				return err
			}
		}
		snapshotBytes, err := common.Marshal(config)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		created = model.InvoiceApplication{
			UserId:               userId,
			Status:               model.InvoiceStatusPendingReview,
			PaymentStatus:        model.InvoicePaymentPaid,
			InvoiceTitle:         strings.TrimSpace(input.InvoiceTitle),
			TaxNumber:            strings.TrimSpace(input.TaxNumber),
			RecipientEmail:       recipientEmail,
			ApplicantNote:        applicantNote,
			InvoiceItemName:      strings.TrimSpace(setting.InvoiceItemName),
			Currency:             config.Currency,
			OrderAmountCents:     totalCents,
			InvoiceAmountCents:   totalCents,
			RuleSnapshot:         string(snapshotBytes),
			FeeRateBasisPoints:   setting.FeeRateBasisPoints,
			FeeAmountCents:       feeCents,
			FeeQuota:             feeQuota,
			QuotaPerUnitSnapshot: config.QuotaPerUnit,
			ExchangeRateSnapshot: config.ExchangeRate,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		for index := range orders {
			orders[index].ApplicationId = created.Id
		}
		if err := tx.Create(&orders).Error; err != nil {
			return err
		}
		created.Orders = orders
		return nil
	})
	if err != nil {
		var conflictCount int64
		conflictErr := model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id IN ?", uniqueIds).Count(&conflictCount).Error
		if conflictErr == nil && conflictCount > 0 {
			return nil, model.ErrInvoiceOrderAlreadyApplied
		}
		return nil, err
	}
	if created.FeeQuota > 0 {
		if err := model.SyncUserQuotaCacheDelta(userId, -created.FeeQuota); err != nil {
			common.SysError("failed to sync invoice fee quota deduction: " + err.Error())
		}
		if err := model.RecordInvoiceFeeLog(&created, false); err != nil {
			common.SysError("failed to record invoice fee charge: " + err.Error())
		}
	}
	return &created, nil
}

func ReviewInvoiceApplication(applicationId int, reviewerId int, approve bool, finalSupplementCents *int64, adjustmentReason string, rejectReason string, note string) error {
	refundedQuota := 0
	userId := 0
	err := model.UpdateInvoiceApplication(applicationId, func(tx *gorm.DB, application *model.InvoiceApplication) error {
		if application.Status != model.InvoiceStatusPendingReview {
			return model.ErrInvoiceStatusInvalid
		}
		now := time.Now().Unix()
		if !approve {
			if strings.TrimSpace(rejectReason) == "" {
				return errors.New("rejection reason is required")
			}
			refunded, err := model.RefundInvoiceFeeTx(tx, application)
			if err != nil {
				return err
			}
			if refunded > 0 {
				refundedQuota = refunded
				userId = application.UserId
			}
			return tx.Model(application).Updates(map[string]interface{}{
				"status":        model.InvoiceStatusRejected,
				"reviewer_id":   reviewerId,
				"reviewed_at":   now,
				"reject_reason": strings.TrimSpace(rejectReason),
				"admin_note":    strings.TrimSpace(note),
				"updated_at":    now,
			}).Error
		}

		return tx.Model(application).Updates(map[string]interface{}{
			"status":                 model.InvoiceStatusApproved,
			"payment_status":         model.InvoicePaymentPaid,
			"final_supplement_cents": 0,
			"invoice_amount_cents":   application.OrderAmountCents,
			"reviewer_id":            reviewerId,
			"reviewed_at":            now,
			"reject_reason":          "",
			"admin_note":             strings.TrimSpace(note),
			"updated_at":             now,
		}).Error
	})
	if err == nil && refundedQuota > 0 {
		_ = model.SyncUserQuotaCacheDelta(userId, refundedQuota)
		if application, getErr := model.GetInvoiceApplication(applicationId, 0); getErr == nil {
			_ = model.RecordInvoiceFeeLog(application, true)
		}
	}
	return err
}

func CancelInvoiceApplication(applicationId int, userId int) error {
	refundedQuota := 0
	err := model.UpdateInvoiceApplication(applicationId, func(tx *gorm.DB, application *model.InvoiceApplication) error {
		if application.UserId != userId {
			return model.ErrInvoiceNotFound
		}
		if application.Status != model.InvoiceStatusPendingReview {
			return model.ErrInvoiceStatusInvalid
		}
		refunded, err := model.RefundInvoiceFeeTx(tx, application)
		if err != nil {
			return err
		}
		refundedQuota = refunded
		now := common.GetTimestamp()
		return tx.Model(application).Updates(map[string]interface{}{
			"status": model.InvoiceStatusRejected, "reject_reason": "cancelled_by_user", "updated_at": now,
		}).Error
	})
	if err == nil && refundedQuota > 0 {
		_ = model.SyncUserQuotaCacheDelta(userId, refundedQuota)
		if application, getErr := model.GetInvoiceApplication(applicationId, 0); getErr == nil {
			_ = model.RecordInvoiceFeeLog(application, true)
		}
	}
	return err
}

func DeleteInvoiceApplication(applicationId int) (*model.InvoiceApplication, error) {
	existing, err := model.GetInvoiceApplication(applicationId, 0)
	if err != nil {
		return nil, err
	}
	if existing.QuotaPerUnitSnapshot > 0 || existing.ExchangeRateSnapshot > 0 {
		return nil, model.ErrInvoiceStatusInvalid
	}
	application, err := model.DeleteInvoiceApplication(applicationId)
	if err != nil {
		return nil, err
	}
	if application.InvoiceFilePath == "" {
		return application, nil
	}

	target, resolveErr := resolveInvoiceFilePath(application.InvoiceFilePath)
	if resolveErr != nil {
		common.SysError(fmt.Sprintf("failed to resolve deleted invoice file path: application_id=%d, err=%v", applicationId, resolveErr))
		return application, nil
	}
	if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		common.SysError(fmt.Sprintf("failed to remove deleted invoice file: application_id=%d, err=%v", applicationId, removeErr))
		return application, nil
	}
	if removeErr := os.Remove(filepath.Dir(target)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		common.SysError(fmt.Sprintf("failed to remove deleted invoice directory: application_id=%d, err=%v", applicationId, removeErr))
	}
	return application, nil
}

func CreateInvoicePaymentOrder(applicationId int, userId int, tradeNo string, paymentMethod string, paymentProvider string) (*model.InvoicePaymentOrder, error) {
	var created model.InvoicePaymentOrder
	err := model.UpdateInvoiceApplication(applicationId, func(tx *gorm.DB, application *model.InvoiceApplication) error {
		if application.UserId != userId {
			return model.ErrInvoiceNotFound
		}
		if application.Status != model.InvoiceStatusPendingPayment || application.PaymentStatus != model.InvoicePaymentPending || application.FinalSupplementCents <= 0 {
			return model.ErrInvoiceStatusInvalid
		}
		created = model.InvoicePaymentOrder{
			ApplicationId:   application.Id,
			UserId:          userId,
			TradeNo:         tradeNo,
			AmountCents:     application.FinalSupplementCents,
			Currency:        application.Currency,
			PaymentMethod:   paymentMethod,
			PaymentProvider: paymentProvider,
			Status:          model.InvoicePaymentOrderPending,
			CreateTime:      time.Now().Unix(),
		}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func invoiceSupplementQuota(amountCents int64, currency string) (int, error) {
	if strings.ToUpper(strings.TrimSpace(currency)) != "CNY" || amountCents <= 0 ||
		common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) ||
		operation_setting.USDExchangeRate <= 0 || math.IsNaN(operation_setting.USDExchangeRate) || math.IsInf(operation_setting.USDExchangeRate, 0) {
		return 0, errors.New("invoice supplement amount cannot be converted to wallet quota")
	}
	quota := decimal.NewFromInt(amountCents).
		Div(decimal.NewFromInt(100)).
		Div(decimal.NewFromFloat(operation_setting.USDExchangeRate)).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Ceil()
	return common.WalletQuotaFromDecimalStrict(quota)
}

// PayInvoiceSupplementWithBalance settles an approved invoice supplement from
// the user's wallet and records a paid invoice payment order atomically.
func PayInvoiceSupplementWithBalance(applicationId int, userId int) (*model.InvoicePaymentOrder, error) {
	if invoice_setting.GetInvoiceSetting().SupplementPaymentMethod != invoice_setting.SupplementPaymentMethodBalance {
		return nil, errors.New("invoice balance payment is disabled")
	}
	var created model.InvoicePaymentOrder
	chargedQuota := 0
	tradeNo := fmt.Sprintf("INVBALUSR%dNO%s%d", userId, common.GetRandomString(6), time.Now().UnixNano())
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		application, err := model.GetInvoiceApplicationForUpdate(tx, applicationId)
		if err != nil {
			return err
		}
		if application.UserId != userId {
			return model.ErrInvoiceNotFound
		}
		if application.Status != model.InvoiceStatusPendingPayment || application.PaymentStatus != model.InvoicePaymentPending || application.FinalSupplementCents <= 0 {
			return model.ErrInvoiceStatusInvalid
		}
		quota, err := invoiceSupplementQuota(application.FinalSupplementCents, application.Currency)
		if err != nil {
			return err
		}
		if err := model.ChargeUserQuotaTx(tx, userId, quota); err != nil {
			return err
		}
		chargedQuota = quota
		now := time.Now().Unix()
		if err := tx.Model(application).Updates(map[string]interface{}{
			"status": model.InvoiceStatusApproved, "payment_status": model.InvoicePaymentPaid,
			"payment_trade_no": tradeNo, "payment_method": model.PaymentMethodBalance,
			"payment_confirmed_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		created = model.InvoicePaymentOrder{ApplicationId: applicationId, UserId: userId, TradeNo: tradeNo,
			AmountCents: application.FinalSupplementCents, Currency: application.Currency,
			PaymentMethod: model.PaymentMethodBalance, PaymentProvider: model.PaymentProviderBalance,
			Status: model.InvoicePaymentOrderPaid, ProviderPayload: fmt.Sprintf("charged_quota=%d", quota),
			CreateTime: now, CompleteTime: now}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, err
	}
	if err := model.SyncUserQuotaCacheDelta(userId, -chargedQuota); err != nil {
		common.SysLog("failed to decrease user quota cache after invoice balance payment: " + err.Error())
	}
	if err := recordInvoiceSupplementPaymentLog(&created); err != nil {
		common.SysError("failed to record invoice balance payment log: " + err.Error())
		return &created, nil
	}
	_ = model.DB.Model(&model.InvoicePaymentOrder{}).Where("trade_no = ?", created.TradeNo).Update("usage_log_recorded_at", time.Now().Unix()).Error
	return &created, nil
}

func recordInvoiceSupplementPaymentLog(order *model.InvoicePaymentOrder) error {
	language := model.GetUserLanguage(order.UserId)
	copy := invoiceLocalizedCopies[normalizeInvoiceLanguage(language)]
	amount := decimal.NewFromInt(order.AmountCents).Shift(-2).StringFixed(2)
	content := fmt.Sprintf(copy.SupplementPaymentLogContent, order.ApplicationId, amount, order.Currency)
	return model.RecordInvoiceSupplementLog(order.UserId, order.ApplicationId, order.TradeNo, order.AmountCents, order.Currency, order.PaymentMethod, order.PaymentProvider, content)
}

func FailInvoicePaymentOrder(tradeNo string) {
	_ = model.DB.Model(&model.InvoicePaymentOrder{}).
		Where("trade_no = ? AND status = ?", tradeNo, model.InvoicePaymentOrderPending).
		Updates(map[string]interface{}{"status": model.InvoicePaymentOrderFailed, "complete_time": time.Now().Unix()}).Error
}

func CompleteInvoicePaymentOrder(tradeNo string, providerPayload string, expectedProvider string, actualPaymentMethod string) error {
	var completedOrder model.InvoicePaymentOrder
	needsUsageLog := false
	err := model.UpdateInvoicePaymentOrder(tradeNo, func(tx *gorm.DB, order *model.InvoicePaymentOrder) error {
		if expectedProvider != "" && order.PaymentProvider != expectedProvider {
			return model.ErrInvoicePaymentMethodMismatch
		}
		if order.Status == model.InvoicePaymentOrderPaid {
			completedOrder = *order
			needsUsageLog = order.UsageLogRecordedAt == 0
			return nil
		}
		if order.Status != model.InvoicePaymentOrderPending {
			return model.ErrInvoicePaymentOrderInvalid
		}
		application, err := model.GetInvoiceApplicationForUpdate(tx, order.ApplicationId)
		if err != nil {
			return err
		}
		if application.UserId != order.UserId || application.FinalSupplementCents != order.AmountCents {
			return model.ErrInvoicePaymentOrderInvalid
		}
		if application.PaymentStatus != model.InvoicePaymentPaid {
			if application.Status != model.InvoiceStatusPendingPayment || application.PaymentStatus != model.InvoicePaymentPending {
				return model.ErrInvoiceStatusInvalid
			}
			now := time.Now().Unix()
			method := order.PaymentMethod
			if actualPaymentMethod != "" {
				method = actualPaymentMethod
			}
			if err := tx.Model(application).Updates(map[string]interface{}{
				"status":               model.InvoiceStatusApproved,
				"payment_status":       model.InvoicePaymentPaid,
				"payment_trade_no":     order.TradeNo,
				"payment_method":       method,
				"payment_confirmed_at": now,
				"updated_at":           now,
			}).Error; err != nil {
				return err
			}
		}
		order.Status = model.InvoicePaymentOrderPaid
		order.CompleteTime = time.Now().Unix()
		order.ProviderPayload = providerPayload
		if actualPaymentMethod != "" {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		completedOrder = *order
		needsUsageLog = true
		return nil
	})
	if err != nil || !needsUsageLog {
		return err
	}

	if err := recordInvoiceSupplementPaymentLog(&completedOrder); err != nil {
		return err
	}

	return model.DB.Model(&model.InvoicePaymentOrder{}).
		Where("trade_no = ? AND usage_log_recorded_at = ?", completedOrder.TradeNo, 0).
		Update("usage_log_recorded_at", time.Now().Unix()).Error
}

func UploadInvoiceFile(applicationId int, originalName string, data []byte) (*model.InvoiceApplication, error) {
	if len(data) == 0 {
		return nil, errors.New("invoice file is empty")
	}
	if int64(len(data)) > InvoiceFileMaxBytes {
		return nil, errors.New("invoice file must be 20 MB or smaller")
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	contentType := http.DetectContentType(head)
	extension, allowed := invoiceFileContentTypes[contentType]
	if !allowed {
		return nil, errors.New("invoice file must be PDF, PNG, or JPEG")
	}
	application, err := model.GetInvoiceApplication(applicationId, 0)
	if err != nil {
		return nil, err
	}
	if application.Status != model.InvoiceStatusApproved && application.Status != model.InvoiceStatusIssued {
		return nil, model.ErrInvoiceStatusInvalid
	}
	directory := filepath.Join("upload", "invoices", strconv.Itoa(applicationId))
	if err = os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create invoice upload directory: %w", err)
	}
	fileName := fmt.Sprintf("invoice-%d-%d%s", applicationId, time.Now().UnixNano(), extension)
	targetPath := filepath.Join(directory, fileName)
	tempPath := targetPath + ".tmp"
	if err = os.WriteFile(tempPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to save invoice file: %w", err)
	}
	if err = os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("failed to finalize invoice file: %w", err)
	}
	displayName := filepath.Base(strings.TrimSpace(originalName))
	if displayName == "." || displayName == "" {
		displayName = fileName
	}
	err = model.UpdateInvoiceApplication(applicationId, func(tx *gorm.DB, current *model.InvoiceApplication) error {
		if current.Status != model.InvoiceStatusApproved && current.Status != model.InvoiceStatusIssued {
			return model.ErrInvoiceStatusInvalid
		}
		return tx.Model(current).Updates(map[string]interface{}{
			"invoice_file_name":         displayName,
			"invoice_file_path":         targetPath,
			"invoice_file_content_type": contentType,
			"updated_at":                time.Now().Unix(),
		}).Error
	})
	if err != nil {
		_ = os.Remove(targetPath)
		return nil, err
	}
	if application.InvoiceFilePath != "" && application.InvoiceFilePath != targetPath {
		oldPath, resolveErr := resolveInvoiceFilePath(application.InvoiceFilePath)
		if resolveErr == nil {
			_ = os.Remove(oldPath)
		}
	}
	stored, err := model.GetInvoiceApplication(applicationId, 0)
	if err != nil {
		return nil, err
	}
	sent, err := SendInvoiceEmail(applicationId, 0)
	if err != nil {
		return stored, fmt.Errorf("invoice file was saved but email delivery failed: %w", err)
	}
	return sent, nil
}

func readInvoiceFile(application *model.InvoiceApplication) (*InvoiceFileContent, error) {
	if application.InvoiceFilePath == "" {
		return nil, ErrInvoiceFileUnavailable
	}
	target, err := resolveInvoiceFilePath(application.InvoiceFilePath)
	if err != nil {
		return nil, err
	}
	fileInfo, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrInvoiceFileUnavailable
		}
		return nil, err
	}
	if !fileInfo.Mode().IsRegular() || fileInfo.Size() <= 0 || fileInfo.Size() > InvoiceFileMaxBytes {
		return nil, ErrInvoiceFileUnavailable
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	contentType := http.DetectContentType(head)
	if _, allowed := invoiceFileContentTypes[contentType]; !allowed {
		return nil, ErrInvoiceFileUnavailable
	}
	fileName := filepath.Base(strings.TrimSpace(application.InvoiceFileName))
	if fileName == "." || fileName == "" {
		fileName = filepath.Base(target)
	}
	return &InvoiceFileContent{
		FileName:    fileName,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func GetInvoiceFileContent(applicationId int) (*InvoiceFileContent, error) {
	application, err := model.GetInvoiceApplication(applicationId, 0)
	if err != nil {
		return nil, err
	}
	if application.Status != model.InvoiceStatusApproved && application.Status != model.InvoiceStatusIssued {
		return nil, model.ErrInvoiceStatusInvalid
	}
	return readInvoiceFile(application)
}

func SendInvoiceEmail(applicationId int, userId int) (*model.InvoiceApplication, error) {
	application, err := model.GetInvoiceApplication(applicationId, userId)
	if err != nil {
		return nil, err
	}
	if userId > 0 && application.Status != model.InvoiceStatusIssued {
		return nil, model.ErrInvoiceStatusInvalid
	}
	if userId == 0 && application.Status != model.InvoiceStatusApproved && application.Status != model.InvoiceStatusIssued {
		return nil, model.ErrInvoiceStatusInvalid
	}
	recipient, err := validateInvoiceRecipientEmail(application.RecipientEmail)
	if err != nil {
		return nil, err
	}
	invoiceFile, err := readInvoiceFile(application)
	if err != nil {
		return nil, err
	}
	subject, content := invoiceEmailMessage(model.GetUserLanguage(application.UserId), application)
	if err = invoiceEmailSender(
		subject,
		recipient,
		content,
		[]common.EmailAttachment{{
			FileName:    invoiceFile.FileName,
			ContentType: invoiceFile.ContentType,
			Data:        invoiceFile.Data,
		}},
	); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	err = model.UpdateInvoiceApplication(applicationId, func(tx *gorm.DB, current *model.InvoiceApplication) error {
		if userId > 0 && current.UserId != userId {
			return model.ErrInvoiceNotFound
		}
		if userId > 0 && current.Status != model.InvoiceStatusIssued {
			return model.ErrInvoiceStatusInvalid
		}
		if userId == 0 && current.Status != model.InvoiceStatusApproved && current.Status != model.InvoiceStatusIssued {
			return model.ErrInvoiceStatusInvalid
		}
		if current.InvoiceFilePath != application.InvoiceFilePath {
			return errors.New("invoice file changed while email was being sent")
		}
		updates := map[string]interface{}{
			"status":                   model.InvoiceStatusIssued,
			"invoice_email_sent_at":    now,
			"invoice_email_send_count": gorm.Expr("invoice_email_send_count + ?", 1),
			"updated_at":               now,
		}
		if current.IssuedAt == 0 {
			updates["issued_at"] = now
		}
		return tx.Model(current).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return model.GetInvoiceApplication(applicationId, userId)
}

func resolveInvoiceFilePath(storedPath string) (string, error) {
	root, err := filepath.Abs(filepath.Join("upload", "invoices"))
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(storedPath)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid invoice file path")
	}
	return target, nil
}

package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// invoiceFeePolicy freezes the monetary settings used by one application.
// Refunds use InvoiceApplication.ChargedQuota rather than recalculating this policy.
type invoiceFeePolicy struct {
	Currency             string  `json:"currency"`
	CurrencySymbol       string  `json:"currency_symbol"`
	FeeRateBasisPoints   int     `json:"fee_rate_basis_points"`
	MinimumAmountCents   int64   `json:"minimum_amount_cents"`
	QuotaPerUnit         float64 `json:"quota_per_unit"`
	USDToCNYRate         float64 `json:"usd_to_cny_rate"`
	USDToCurrencyRate    float64 `json:"usd_to_currency_rate"`
	QuotaPerCurrencyUnit float64 `json:"quota_per_currency_unit"`
}

// invoiceMoneyCents validates the decimal before converting it to an integer.
// In particular, IntPart must never see values outside the supported amount range.
func invoiceMoneyCents(amount float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return 0, errors.New("invoice amount is invalid")
	}
	cents := decimal.NewFromFloat(amount).Shift(2).Round(0)
	if cents.GreaterThan(decimal.NewFromInt(maxInvoiceAmountCents)) {
		return 0, errors.New("invoice amount is invalid")
	}
	return cents.IntPart(), nil
}

func currentInvoiceFeePolicy(setting invoice_setting.InvoiceSetting) (invoiceFeePolicy, error) {
	general := *operation_setting.GetGeneralSetting()
	policy := invoiceFeePolicy{
		Currency: "USD", CurrencySymbol: "$", USDToCurrencyRate: 1, USDToCNYRate: operation_setting.USDExchangeRate,
		QuotaPerUnit: common.QuotaPerUnit, FeeRateBasisPoints: setting.FeeRateBasisPoints,
	}
	switch general.QuotaDisplayType {
	case operation_setting.QuotaDisplayTypeUSD, operation_setting.QuotaDisplayTypeTokens:
	case operation_setting.QuotaDisplayTypeCNY:
		policy.Currency, policy.CurrencySymbol = "CNY", "¥"
		policy.USDToCurrencyRate = policy.USDToCNYRate
	case operation_setting.QuotaDisplayTypeCustom:
		policy.Currency, policy.CurrencySymbol = "CUSTOM", strings.TrimSpace(general.CustomCurrencySymbol)
		if policy.CurrencySymbol == "" {
			policy.CurrencySymbol = "¤"
		}
		policy.USDToCurrencyRate = general.CustomCurrencyExchangeRate
	default:
		return policy, errors.New("invoice currency setting is invalid")
	}
	if len([]rune(policy.CurrencySymbol)) > 64 {
		return policy, errors.New("invoice currency symbol must be 64 characters or fewer")
	}
	if policy.FeeRateBasisPoints < 0 || policy.FeeRateBasisPoints > 10_000 {
		return policy, errors.New("invoice fee rate must be between 0 and 100 percent")
	}
	minimumCents, err := invoiceMoneyCents(setting.MinimumAmount)
	if err != nil {
		return policy, errors.New("minimum invoice amount is invalid")
	}
	policy.MinimumAmountCents = minimumCents
	itemName := strings.TrimSpace(setting.InvoiceItemName)
	if itemName == "" || len([]rune(itemName)) > 255 {
		return policy, errors.New("invoice item name must be between 1 and 255 characters")
	}
	if policy.USDToCNYRate <= 0 || math.IsNaN(policy.USDToCNYRate) || math.IsInf(policy.USDToCNYRate, 0) ||
		policy.QuotaPerUnit <= 0 || math.IsNaN(policy.QuotaPerUnit) || math.IsInf(policy.QuotaPerUnit, 0) ||
		policy.USDToCurrencyRate <= 0 || math.IsNaN(policy.USDToCurrencyRate) || math.IsInf(policy.USDToCurrencyRate, 0) {
		return policy, errors.New("invoice currency conversion setting is invalid")
	}
	quotaPerCurrency := policy.QuotaPerUnit / policy.USDToCurrencyRate
	if quotaPerCurrency <= 0 || math.IsNaN(quotaPerCurrency) || math.IsInf(quotaPerCurrency, 0) || quotaPerCurrency > float64(common.MaxWalletQuota) {
		return policy, errors.New("invoice currency conversion setting is invalid")
	}
	policy.QuotaPerCurrencyUnit = quotaPerCurrency
	return policy, nil
}

// calculateInvoiceFee rounds the fee to the nearest currency cent, then rounds
// the exact quota conversion upward so fractions of a quota unit are not lost.
func calculateInvoiceFee(amountCents int64, policy invoiceFeePolicy) (int64, int, error) {
	if amountCents <= 0 || amountCents > maxInvoiceAmountCents || policy.FeeRateBasisPoints < 0 || policy.FeeRateBasisPoints > 10_000 {
		return 0, 0, errors.New("invoice fee amount is invalid")
	}
	fee := decimal.NewFromInt(amountCents).Mul(decimal.NewFromInt(int64(policy.FeeRateBasisPoints))).Shift(-4).Round(0)
	if fee.IsZero() {
		return 0, 0, nil
	}
	if policy.QuotaPerUnit <= 0 || math.IsNaN(policy.QuotaPerUnit) || math.IsInf(policy.QuotaPerUnit, 0) ||
		policy.USDToCurrencyRate <= 0 || math.IsNaN(policy.USDToCurrencyRate) || math.IsInf(policy.USDToCurrencyRate, 0) {
		return 0, 0, errors.New("invoice currency conversion setting is invalid")
	}
	quota, remainder := fee.Mul(decimal.NewFromFloat(policy.QuotaPerUnit)).QuoRem(decimal.NewFromFloat(policy.USDToCurrencyRate).Shift(2), 0)
	if remainder.IsPositive() {
		quota = quota.Add(decimal.NewFromInt(1))
	}
	chargedQuota, err := common.WalletQuotaFromDecimalStrict(quota)
	if err != nil {
		return 0, 0, err
	}
	return fee.IntPart(), chargedQuota, nil
}

// ErrInvoicePaidAmountUnverifiable prevents nominal wallet credit, current
// product settings, or an ambiguous legacy payment from becoming invoice money.
var ErrInvoicePaidAmountUnverifiable = errors.New("Paid order amount or currency cannot be verified")

type InvoiceEligibleOrder struct {
	model.TopUp
	InvoiceAmountCents int64  `json:"invoice_amount_cents"`
	Currency           string `json:"currency"`
	CurrencySymbol     string `json:"currency_symbol"`
}

type invoicePaidOrderQuote struct {
	TopUpId                 int    `json:"top_up_id"`
	OriginalPaidAmountCents int64  `json:"original_paid_amount_cents"`
	OriginalCurrency        string `json:"original_currency"`
	InvoiceAmountCents      int64  `json:"invoice_amount_cents"`
	EvidenceSource          string `json:"evidence_source"`
}

// quoteInvoicePaidOrder converts a verified cash payment to the invoice's
// frozen currency. Only successful legacy Epay orders have a known CNY Money
// contract; other providers may store nominal wallet credit in that field.
func quoteInvoicePaidOrder(topUp model.TopUp, policy invoiceFeePolicy, legacy model.SubscriptionOrder) (invoicePaidOrderQuote, error) {
	quote := invoicePaidOrderQuote{
		TopUpId: topUp.Id, OriginalPaidAmountCents: topUp.PaidAmountMinor,
		OriginalCurrency: strings.ToUpper(strings.TrimSpace(topUp.PaidCurrency)),
		EvidenceSource:   "payment_receipt_snapshot",
	}
	if topUp.PaidAmountMinor == 0 && topUp.PaidCurrency == "" &&
		topUp.PaymentProvider == model.PaymentProviderEpay && topUp.Status == common.TopUpStatusSuccess {
		// Before receipt snapshots, Epay stored the original CNY payable amount
		// in Money. Completion came from a verified callback or admin completion.
		// Retain that historical invoice contract without changing the order.
		amount, err := invoiceMoneyCents(topUp.Money)
		if err != nil || amount <= 0 {
			return quote, ErrInvoicePaidAmountUnverifiable
		}
		quote.OriginalPaidAmountCents, quote.OriginalCurrency = amount, "CNY"
		quote.EvidenceSource = "legacy_epay_completed_order"
	}
	if quote.OriginalPaidAmountCents == 0 && quote.OriginalCurrency == "" {
		if legacy.UserId != topUp.UserId || legacy.TradeNo != topUp.TradeNo || legacy.Status != common.TopUpStatusSuccess {
			return quote, ErrInvoicePaidAmountUnverifiable
		}
		amount, currency, err := legacyInvoicePaymentReceipt(legacy)
		if err != nil {
			return quote, err
		}
		quote.OriginalPaidAmountCents, quote.OriginalCurrency = amount, currency
		quote.EvidenceSource = "verified_subscription_receipt"
	}
	if quote.OriginalPaidAmountCents <= 0 || quote.OriginalPaidAmountCents > maxInvoiceAmountCents {
		return quote, ErrInvoicePaidAmountUnverifiable
	}
	sourceRate := 1.0
	switch quote.OriginalCurrency {
	case "USD":
	case "CNY":
		sourceRate = policy.USDToCNYRate
	default:
		return quote, ErrInvoicePaidAmountUnverifiable
	}
	if sourceRate <= 0 || math.IsNaN(sourceRate) || math.IsInf(sourceRate, 0) ||
		policy.USDToCurrencyRate <= 0 || math.IsNaN(policy.USDToCurrencyRate) || math.IsInf(policy.USDToCurrencyRate, 0) {
		return quote, ErrInvoicePaidAmountUnverifiable
	}
	divisor := decimal.NewFromFloat(sourceRate)
	amount, remainder := decimal.NewFromInt(quote.OriginalPaidAmountCents).
		Mul(decimal.NewFromFloat(policy.USDToCurrencyRate)).QuoRem(divisor, 0)
	if remainder.Mul(decimal.NewFromInt(2)).GreaterThanOrEqual(divisor) {
		amount = amount.Add(decimal.NewFromInt(1))
	}
	if !amount.IsPositive() || amount.GreaterThan(decimal.NewFromInt(maxInvoiceAmountCents)) {
		return quote, ErrInvoicePaidAmountUnverifiable
	}
	quote.InvoiceAmountCents = amount.IntPart()
	return quote, nil
}

// loadInvoicePaymentEvidence reads legacy receipts in one query, using the
// same transaction as application creation. Current paid snapshots need no join.
func loadInvoicePaymentEvidence(tx *gorm.DB, topUps []model.TopUp) (map[string]model.SubscriptionOrder, error) {
	receipts := make(map[string]model.SubscriptionOrder)
	tradeNos := make([]string, 0, len(topUps))
	for _, topUp := range topUps {
		if topUp.PaidAmountMinor == 0 && topUp.PaidCurrency == "" {
			tradeNos = append(tradeNos, topUp.TradeNo)
		}
	}
	if len(tradeNos) == 0 {
		return receipts, nil
	}
	var orders []model.SubscriptionOrder
	if err := tx.Where("trade_no IN ? AND status = ?", tradeNos, common.TopUpStatusSuccess).Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, order := range orders {
		receipts[order.TradeNo] = order
	}
	return receipts, nil
}

// These payloads were saved only after the existing provider callback verified
// the signature and matched the successful subscription payment to its order.
// Unknown formats never fall back to nominal Money or current product settings.
func legacyInvoicePaymentReceipt(order model.SubscriptionOrder) (int64, string, error) {
	var amount int64
	var currency string
	switch order.PaymentProvider {
	case model.PaymentProviderStripe:
		var receipt struct {
			AmountTotal string `json:"amount_total"`
			Currency    string `json:"currency"`
		}
		if err := common.UnmarshalJsonStr(order.ProviderPayload, &receipt); err != nil {
			return 0, "", ErrInvoicePaidAmountUnverifiable
		}
		parsed, err := strconv.ParseInt(receipt.AmountTotal, 10, 64)
		if err != nil {
			return 0, "", ErrInvoicePaidAmountUnverifiable
		}
		amount, currency = parsed, receipt.Currency
	case model.PaymentProviderCreem:
		var receipt struct {
			Object struct {
				RequestID string `json:"request_id"`
				Order     struct {
					AmountPaid *int64 `json:"amount_paid"`
					Currency   string `json:"currency"`
					Status     string `json:"status"`
				} `json:"order"`
			} `json:"object"`
		}
		if err := common.UnmarshalJsonStr(order.ProviderPayload, &receipt); err != nil || receipt.Object.RequestID != order.TradeNo || receipt.Object.Order.Status != "paid" || receipt.Object.Order.AmountPaid == nil {
			return 0, "", ErrInvoicePaidAmountUnverifiable
		}
		amount, currency = *receipt.Object.Order.AmountPaid, receipt.Object.Order.Currency
	case model.PaymentProviderWaffoPancake:
		var receipt struct {
			StoreID string `json:"storeId"`
			Data    struct {
				Amount    string `json:"amount"`
				Total     string `json:"total"`
				TaxAmount string `json:"taxAmount"`
				Currency  string `json:"currency"`
			} `json:"data"`
		}
		if err := common.UnmarshalJsonStr(order.ProviderPayload, &receipt); err != nil {
			return 0, "", ErrInvoicePaidAmountUnverifiable
		}
		// Older callbacks verified a platform signature without store binding.
		// A saved successful order alone is not proof of payment to this store.
		if err := validateWaffoPancakeStoreID(receipt.StoreID); err != nil {
			return 0, "", ErrInvoicePaidAmountUnverifiable
		}
		paid := receipt.Data.Total
		if paid == "" {
			tax, err := invoiceReceiptDisplayCents(receipt.Data.TaxAmount)
			if err != nil || tax != 0 {
				return 0, "", ErrInvoicePaidAmountUnverifiable
			}
			paid = receipt.Data.Amount
		}
		parsed, err := invoiceReceiptDisplayCents(paid)
		if err != nil {
			return 0, "", err
		}
		amount, currency = parsed, receipt.Data.Currency
	default:
		return 0, "", ErrInvoicePaidAmountUnverifiable
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if amount <= 0 || amount > maxInvoiceAmountCents || (currency != "USD" && currency != "CNY") {
		return 0, "", ErrInvoicePaidAmountUnverifiable
	}
	return amount, currency, nil
}

func invoiceReceiptDisplayCents(value string) (int64, error) {
	if len(value) == 0 || len(value) > 32 {
		return 0, ErrInvoicePaidAmountUnverifiable
	}
	for _, char := range value {
		if (char < '0' || char > '9') && char != '.' {
			return 0, ErrInvoicePaidAmountUnverifiable
		}
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return 0, ErrInvoicePaidAmountUnverifiable
	}
	cents := amount.Shift(2)
	if cents.IsNegative() || cents.GreaterThan(decimal.NewFromInt(maxInvoiceAmountCents)) || !cents.Equal(cents.Truncate(0)) {
		return 0, ErrInvoicePaidAmountUnverifiable
	}
	return cents.IntPart(), nil
}

// syncInvoiceFeeBalance runs only after the database transaction commits. Logs
// describe the authoritative quota delta, including refunds after settings change.
func syncInvoiceFeeBalance(application *model.InvoiceApplication, delta int, updateCache bool) {
	if delta == 0 {
		return
	}
	if updateCache {
		if err := model.SyncUserQuotaCacheDelta(application.UserId, delta); err != nil {
			common.SysError("failed to sync invoice fee balance: " + err.Error())
		}
	}
	direction := "发票扣点扣除余额"
	if delta > 0 {
		direction = "发票扣点退回余额"
	}
	amount := decimal.NewFromInt(application.FinalSupplementCents).Shift(-2).StringFixed(2)
	model.RecordLog(application.UserId, model.LogTypeSystem, fmt.Sprintf("%s：申请 #%d，金额 %s %s，额度变动 %+d。", direction, application.Id, amount, application.Currency, delta))
}

func CancelInvoiceApplication(applicationId int, userId int) (*model.InvoiceApplication, error) {
	if userId <= 0 {
		return nil, model.ErrInvoiceNotFound
	}
	return deleteInvoiceApplication(applicationId, userId)
}

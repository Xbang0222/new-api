package controller

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type createInvoiceApplicationRequest struct {
	TopUpIds       []int  `json:"top_up_ids"`
	InvoiceTitle   string `json:"invoice_title"`
	TaxNumber      string `json:"tax_number"`
	RecipientEmail string `json:"recipient_email"`
	ApplicantNote  string `json:"applicant_note"`
}

type reviewInvoiceApplicationRequest struct {
	Action                     string `json:"action"`
	FinalSupplementAmountCents *int64 `json:"final_supplement_amount_cents"`
	TaxAdjustmentReason        string `json:"tax_adjustment_reason"`
	Reason                     string `json:"reason"`
	Note                       string `json:"note"`
}

func invoiceApplicationId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid invoice application id")
		return 0, false
	}
	return id, true
}

func GetInvoiceConfig(c *gin.Context) {
	common.ApiSuccess(c, service.GetInvoiceConfig())
}

func GetInvoicePaymentMethods(c *gin.Context) {
	common.ApiSuccess(c, []map[string]string{})
}

func CancelInvoiceApplication(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	if err := service.CancelInvoiceApplication(id, c.GetInt("id")); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}

func GetEligibleInvoiceOrders(c *gin.Context) {
	orders, err := service.ListEligibleInvoiceOrders(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, orders)
}

func CreateInvoiceApplication(c *gin.Context) {
	var request createInvoiceApplicationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid invoice application")
		return
	}
	request.InvoiceTitle = strings.TrimSpace(request.InvoiceTitle)
	request.TaxNumber = strings.TrimSpace(request.TaxNumber)
	request.RecipientEmail = strings.TrimSpace(request.RecipientEmail)
	request.ApplicantNote = strings.TrimSpace(request.ApplicantNote)
	if request.InvoiceTitle == "" || len(request.InvoiceTitle) > 255 {
		common.ApiErrorMsg(c, "invoice title is required and must be 255 characters or fewer")
		return
	}
	if request.TaxNumber == "" || len(request.TaxNumber) > 64 {
		common.ApiErrorMsg(c, "tax number is required and must be 64 characters or fewer")
		return
	}
	if request.RecipientEmail == "" || len(request.RecipientEmail) > 255 {
		common.ApiErrorMsg(c, "recipient email is required and must be 255 characters or fewer")
		return
	}
	address, err := mail.ParseAddress(request.RecipientEmail)
	if err != nil || !strings.EqualFold(address.Address, request.RecipientEmail) {
		common.ApiErrorMsg(c, "recipient email is invalid")
		return
	}
	if len([]rune(request.ApplicantNote)) > 2000 {
		common.ApiErrorMsg(c, "invoice application note must be 2000 characters or fewer")
		return
	}

	application, err := service.CreateInvoiceApplication(c.GetInt("id"), service.CreateInvoiceApplicationInput{
		TopUpIds:       request.TopUpIds,
		InvoiceTitle:   request.InvoiceTitle,
		TaxNumber:      request.TaxNumber,
		RecipientEmail: request.RecipientEmail,
		ApplicantNote:  request.ApplicantNote,
	})
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, application)
}

func ListInvoiceApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	applications, total, err := service.ListInvoiceApplications(c.GetInt("id"), c.Query("status"), strings.TrimSpace(c.Query("keyword")), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(applications)
	common.ApiSuccess(c, pageInfo)
}

func AdminListInvoiceApplications(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	applications, total, err := service.ListInvoiceApplications(0, c.Query("status"), strings.TrimSpace(c.Query("keyword")), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(applications)
	common.ApiSuccess(c, pageInfo)
}

func GetInvoiceApplication(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	application, err := service.GetInvoiceApplication(id, c.GetInt("id"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, application)
}

func AdminGetInvoiceApplication(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	application, err := service.GetInvoiceApplication(id, 0)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, application)
}

func GetInvoiceFile(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	serveInvoiceFile(c, id)
}

func GetOwnInvoiceFile(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	if _, err := service.GetInvoiceApplication(id, c.GetInt("id")); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	serveInvoiceFile(c, id)
}

func serveInvoiceFile(c *gin.Context, id int) {
	invoiceFile, err := service.GetInvoiceFileContent(id)
	if err != nil {
		status := http.StatusInternalServerError
		message := "failed to read invoice file"
		switch {
		case errors.Is(err, model.ErrInvoiceNotFound), errors.Is(err, service.ErrInvoiceFileUnavailable):
			status = http.StatusNotFound
			message = err.Error()
		case errors.Is(err, model.ErrInvoiceStatusInvalid):
			status = http.StatusForbidden
			message = err.Error()
		}
		c.JSON(status, gin.H{"success": false, "message": message})
		return
	}

	disposition := mime.FormatMediaType("inline", map[string]string{"filename": invoiceFile.FileName})
	if disposition == "" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", invoiceFile.ContentType)
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	http.ServeContent(c.Writer, c.Request, invoiceFile.FileName, time.Time{}, bytes.NewReader(invoiceFile.Data))
}

func AdminReviewInvoiceApplication(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	var request reviewInvoiceApplicationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "invalid review request")
		return
	}
	if request.Action != "approve" && request.Action != "reject" {
		common.ApiErrorMsg(c, "review action must be approve or reject")
		return
	}
	if len(request.Reason) > 2000 || len(request.Note) > 2000 || len(request.TaxAdjustmentReason) > 2000 {
		common.ApiErrorMsg(c, "review note is too long")
		return
	}
	application, err := service.GetInvoiceApplication(id, 0)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	err = service.ReviewInvoiceApplication(
		id,
		c.GetInt("id"),
		request.Action == "approve",
		request.FinalSupplementAmountCents,
		request.TaxAdjustmentReason,
		request.Reason,
		request.Note,
	)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAuditFor(c, application.UserId, "invoice.review", map[string]interface{}{
		"invoice_application_id": id,
		"action":                 request.Action,
		"final_supplement_cents": request.FinalSupplementAmountCents,
	})
	common.ApiSuccess(c, nil)
}

func AdminDeleteInvoiceApplication(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	application, err := service.DeleteInvoiceApplication(id)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAuditFor(c, application.UserId, "invoice.delete", map[string]interface{}{
		"invoice_application_id": id,
		"status":                 application.Status,
	})
	common.ApiSuccess(c, nil)
}

func RequestInvoiceSupplementPayment(c *gin.Context) {
	common.ApiErrorMsg(c, "invoice supplement payments are no longer available")
}

func invoiceEpayParams(c *gin.Context) (map[string]string, bool) {
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			return nil, false
		}
		return lo.Reduce(lo.Keys(c.Request.PostForm), func(result map[string]string, key string, _ int) map[string]string {
			result[key] = c.Request.PostForm.Get(key)
			return result
		}, map[string]string{}), true
	}
	return lo.Reduce(lo.Keys(c.Request.URL.Query()), func(result map[string]string, key string, _ int) map[string]string {
		result[key] = c.Request.URL.Query().Get(key)
		return result
	}, map[string]string{}), true
}

func invoiceEpayAmountMatches(tradeNo string, params map[string]string) bool {
	order, err := model.GetInvoicePaymentOrderByTradeNo(tradeNo)
	if err != nil {
		return false
	}
	paidAmount, err := decimal.NewFromString(strings.TrimSpace(params["money"]))
	if err != nil {
		return false
	}
	return paidAmount.Shift(2).Round(0).Equal(decimal.NewFromInt(order.AmountCents))
}

func InvoiceEpayNotify(c *gin.Context) {
	params, ok := invoiceEpayParams(c)
	client := GetEpayClient()
	if !ok || len(params) == 0 || client == nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus || verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if !invoiceEpayAmountMatches(verifyInfo.ServiceTradeNo, params) {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)
	if err := service.CompleteInvoicePaymentOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type); err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_, _ = c.Writer.Write([]byte("success"))
}

func InvoiceEpayReturn(c *gin.Context) {
	params, ok := invoiceEpayParams(c)
	client := GetEpayClient()
	if !ok || len(params) == 0 || client == nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=fail"))
		return
	}
	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=pending"))
		return
	}
	if !invoiceEpayAmountMatches(verifyInfo.ServiceTradeNo, params) {
		c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=fail"))
		return
	}
	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)
	if err := service.CompleteInvoicePaymentOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), model.PaymentProviderEpay, verifyInfo.Type); err != nil {
		c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=fail"))
		return
	}
	c.Redirect(http.StatusFound, paymentReturnPath("/invoices?pay=success"))
}

func AdminUploadInvoiceFile(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.InvoiceFileMaxBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "missing invoice file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, service.InvoiceFileMaxBytes+1))
	if err != nil || len(data) == 0 {
		common.ApiErrorMsg(c, "failed to read invoice file")
		return
	}
	if int64(len(data)) > service.InvoiceFileMaxBytes {
		common.ApiErrorMsg(c, "invoice file must be 20 MB or smaller")
		return
	}
	application, err := service.UploadInvoiceFile(id, header.Filename, data)
	if application != nil {
		recordManageAuditFor(c, application.UserId, "invoice.file.upload", map[string]interface{}{"invoice_application_id": id})
	}
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, nil)
}

func SendInvoiceEmail(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	application, err := service.SendInvoiceEmail(id, c.GetInt("id"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, application)
}

func AdminSendInvoiceEmail(c *gin.Context) {
	id, ok := invoiceApplicationId(c)
	if !ok {
		return
	}
	application, err := service.SendInvoiceEmail(id, 0)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAuditFor(c, application.UserId, "invoice.email.send", map[string]interface{}{"invoice_application_id": id})
	common.ApiSuccess(c, application)
}

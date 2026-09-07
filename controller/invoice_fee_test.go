package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/invoice_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvoiceController(t *testing.T, quota int) (*gin.Engine, model.TopUp) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}, &model.TopUp{}, &model.InvoiceApplication{}, &model.InvoiceOrder{}, &model.InvoicePaymentOrder{}))
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousQuota := common.QuotaPerUnit
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	previousInvoice := *invoice_setting.GetInvoiceSetting()
	previousGeneral := *operation_setting.GetGeneralSetting()
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.QuotaPerUnit, common.RedisEnabled, common.MemoryCacheEnabled = 500000, false, false
	*invoice_setting.GetInvoiceSetting() = invoice_setting.InvoiceSetting{Enabled: true, InvoiceItemName: "技术服务费", FeeRateBasisPoints: 300}
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.QuotaPerUnit, common.RedisEnabled, common.MemoryCacheEnabled = previousQuota, previousRedis, previousMemory
		*invoice_setting.GetInvoiceSetting() = previousInvoice
		*operation_setting.GetGeneralSetting() = previousGeneral
	})
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "invoice-owner", Quota: quota}).Error)
	order := model.TopUp{UserId: 1, Money: 50, PaidAmountMinor: 5000, PaidCurrency: "USD", TradeNo: "paid-invoice-order", Status: common.TopUpStatusSuccess, CompleteTime: time.Now().Unix()}
	require.NoError(t, db.Create(&order).Error)
	router := gin.New()
	// Authentication is a router/middleware contract. Supply its resolved actor
	// here so these handler tests can exercise ownership without session fixtures.
	router.Use(func(c *gin.Context) { id, _ := strconv.Atoi(c.GetHeader("Test-Actor")); c.Set("id", id); c.Next() })
	router.POST("/applications", CreateInvoiceApplication)
	router.POST("/applications/:id/review", AdminReviewInvoiceApplication)
	router.POST("/applications/:id/cancel", CancelInvoiceApplication)
	router.POST("/applications/:id/pay", RequestInvoiceSupplementPayment)
	return router, order
}

func invoiceControllerRequest(t *testing.T, router *gin.Engine, actor int, path string, body any) (*httptest.ResponseRecorder, struct {
	Success bool                     `json:"success"`
	Message string                   `json:"message"`
	Data    model.InvoiceApplication `json:"data"`
}) {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Test-Actor", strconv.Itoa(actor))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	var result struct {
		Success bool                     `json:"success"`
		Message string                   `json:"message"`
		Data    model.InvoiceApplication `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	return response, result
}

func TestInvoiceApplicationControllerChargesThenCancelsOriginalBalance(t *testing.T) {
	router, order := setupInvoiceController(t, 1000000)
	_, created := invoiceControllerRequest(t, router, 1, "/applications", createInvoiceApplicationRequest{TopUpIds: []int{order.Id}, InvoiceTitle: "Example Co", TaxNumber: "TAX123", RecipientEmail: "billing@example.com"})
	require.True(t, created.Success, created.Message)
	assert.EqualValues(t, 5000, created.Data.InvoiceAmountCents)
	assert.EqualValues(t, 150, created.Data.FinalSupplementCents)
	assert.Equal(t, 750000, created.Data.ChargedQuota)
	assert.Equal(t, "技术服务费", created.Data.InvoiceItemName)
	path := "/applications/" + strconv.Itoa(created.Data.Id)
	_, stranger := invoiceControllerRequest(t, router, 2, path+"/cancel", nil)
	assert.False(t, stranger.Success)
	assert.Equal(t, model.ErrInvoiceNotFound.Error(), stranger.Message)
	payResponse, pay := invoiceControllerRequest(t, router, 1, path+"/pay", nil)
	assert.Equal(t, http.StatusGone, payResponse.Code)
	assert.False(t, pay.Success)
	_, approved := invoiceControllerRequest(t, router, 99, path+"/review", reviewInvoiceApplicationRequest{Action: "approve"})
	require.True(t, approved.Success, approved.Message)
	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	assert.Equal(t, 250000, user.Quota, "approval does not deduct a second fee")
	_, cancelled := invoiceControllerRequest(t, router, 1, path+"/cancel", nil)
	require.True(t, cancelled.Success, cancelled.Message)
	require.NoError(t, model.DB.First(&user, 1).Error)
	assert.Equal(t, 1000000, user.Quota)
	var remaining int64
	require.NoError(t, model.DB.Model(&model.InvoiceOrder{}).Where("top_up_id = ?", order.Id).Count(&remaining).Error)
	assert.Zero(t, remaining, "cancellation releases the paid order for reapplication")
}

func TestInvoiceApplicationControllerReportsInsufficientQuotaWithoutCreatingApplication(t *testing.T) {
	router, order := setupInvoiceController(t, 749999)
	_, result := invoiceControllerRequest(t, router, 1, "/applications", createInvoiceApplicationRequest{TopUpIds: []int{order.Id}, InvoiceTitle: "Example Co", TaxNumber: "TAX123", RecipientEmail: "billing@example.com"})
	assert.False(t, result.Success)
	assert.Equal(t, "额度不足", result.Message)
	var count int64
	require.NoError(t, model.DB.Model(&model.InvoiceApplication{}).Count(&count).Error)
	assert.Zero(t, count)
	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	assert.Equal(t, 749999, user.Quota)
}

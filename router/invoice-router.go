package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

// SetInvoiceRouter 注册所有发票相关路由（独立文件，减少上游冲突）
func SetInvoiceRouter(apiRouter *gin.RouterGroup) {
	invoiceRoute := apiRouter.Group("/invoice")
	{
		// 公开：获取发票功能配置
		invoiceRoute.GET("/setting", controller.GetInvoiceSetting)

		// 用户侧（需要登录）
		userRoute := invoiceRoute.Group("/")
		userRoute.Use(middleware.UserAuth())
		{
			// 可开票的充值记录
			userRoute.GET("/available-topups", controller.GetAvailableTopUps)
			userRoute.GET("/invoiced-topup-ids", controller.GetUserInvoicedTopUpIds)
			// 发票申请 CRUD
			userRoute.POST("/", middleware.CriticalRateLimit(), controller.SubmitInvoice)
			userRoute.GET("/self", controller.GetUserInvoices)
			userRoute.PUT("/:id/cancel", middleware.CriticalRateLimit(), controller.CancelInvoice)
			userRoute.GET("/:id/items", controller.GetInvoiceItems)
			// 发票抬头模板
			userRoute.GET("/headers", controller.GetInvoiceHeaders)
			userRoute.POST("/headers", middleware.CriticalRateLimit(), controller.CreateInvoiceHeader)
			userRoute.PUT("/headers/:id", controller.UpdateInvoiceHeader)
			userRoute.DELETE("/headers/:id", controller.DeleteInvoiceHeader)
			// 删除待支付充值记录
			userRoute.DELETE("/topup/:id", controller.DeletePendingTopUp)
		}

		// 管理员侧
		adminRoute := invoiceRoute.Group("/admin")
		adminRoute.Use(middleware.AdminAuth())
		{
			adminRoute.GET("/", controller.GetAllInvoicesAdmin)
			adminRoute.PUT("/:id/review", controller.ReviewInvoiceAdmin)
			adminRoute.PUT("/:id/send", controller.MarkInvoiceSent)
		}
	}
}

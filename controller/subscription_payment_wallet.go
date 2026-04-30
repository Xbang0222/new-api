// custom: wallet subscription payment
package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type SubscriptionWalletPayRequest struct {
	PlanId int `json:"plan_id"`
}

func SubscriptionRequestWallet(c *gin.Context) {
	var req SubscriptionWalletPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}

	userId := c.GetInt("id")
	// custom: subscription cycle purchase limit — count rolling window, not lifetime
	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountPurchasesInWindow(userId, plan)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}

	// Calculate quota cost: PriceAmount (USD) * QuotaPerUnit
	dPrice := decimal.NewFromFloat(plan.PriceAmount)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
	quotaCost := int(dPrice.Mul(dQuotaPerUnit).IntPart())
	if quotaCost <= 0 {
		common.ApiErrorMsg(c, "套餐价格计算异常")
		return
	}

	if err := model.PurchaseSubscriptionWithWallet(userId, plan, quotaCost); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "data": true})
}

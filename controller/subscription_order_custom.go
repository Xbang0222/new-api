package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// custom: subscription deduction order
type SubscriptionDeductionOrderRequest struct {
	SubscriptionIds []int `json:"subscription_ids"`
}

// custom: subscription deduction order
func UpdateSubscriptionDeductionOrder(c *gin.Context) {
	userId := c.GetInt("id")
	var req SubscriptionDeductionOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.UpdateUserSubscriptionDeductionOrder(userId, req.SubscriptionIds); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"subscription_ids": req.SubscriptionIds})
}

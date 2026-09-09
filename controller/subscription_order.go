package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func UpdateSubscriptionOrder(c *gin.Context) {
	// Bound decoding before allocating the array. This accommodates 1000
	// int64 IDs plus JSON syntax, including requests without Content-Length.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req struct {
		SubscriptionIDs *[]int `json:"subscription_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SubscriptionIDs == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.UpdateSubscriptionDebitOrder(c.GetInt("id"), *req.SubscriptionIDs); err != nil {
		common.ApiError(c, err)
		return
	}
	GetSubscriptionSelf(c)
}

// custom: invite reward log
package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	maxInvitePageSize = 50
	maxInvitePage     = 10000 // 防御 ?page=巨大值 触发 OFFSET 全表扫的慢查询 DoS
)

func parseInvitePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > maxInvitePage {
		page = maxInvitePage
	}
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > maxInvitePageSize {
		pageSize = maxInvitePageSize
	}
	return
}

// GetInvitationSummary 返回当前用户的"按人汇总"邀请明细。
// GET /api/user/invites/summary?page=1&page_size=20
func GetInvitationSummary(c *gin.Context) {
	inviterId := c.GetInt("id")
	if inviterId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "未登录"})
		return
	}

	page, pageSize := parseInvitePagination(c)

	items, total, err := model.GetInvitationSummary(inviterId, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	activeInvitees, err := model.CountActiveInvitees(inviterId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	// 头部统计直接读 User 表的累计字段
	user, _ := model.GetUserById(inviterId, false)
	var totalInvitees, totalRewardAll int
	if user != nil {
		totalInvitees = user.AffCount
		totalRewardAll = user.AffHistoryQuota
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
			"stats": gin.H{
				"total_invitees":   totalInvitees,
				"active_invitees":  activeInvitees,
				"total_reward_all": totalRewardAll,
			},
		},
	})
}

// GetInvitationLogs 返回当前用户的返利流水。
// GET /api/user/invites/logs?page=1&page_size=20
func GetInvitationLogs(c *gin.Context) {
	inviterId := c.GetInt("id")
	if inviterId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "未登录"})
		return
	}

	page, pageSize := parseInvitePagination(c)

	items, total, err := model.GetInvitationLogs(inviterId, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

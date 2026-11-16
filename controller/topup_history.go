package controller

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// Both listing and export use the same query and existing route authorization.
// A personal request always takes its account ID from the authenticated context.
func getTopUpHistory(c *gin.Context, self bool) {
	filter := model.TopUpHistoryFilter{Keyword: strings.TrimSpace(c.Query("keyword")), Self: self}
	if self {
		filter.UserID = c.GetInt("id")
	} else {
		filter.Username = strings.TrimSpace(c.Query("username"))
		if value := strings.TrimSpace(c.Query("user_id")); value != "" {
			id, err := strconv.Atoi(value)
			if err != nil || id <= 0 {
				common.ApiError(c, common.NewMessage("Invalid user ID"))
				return
			}
			filter.UserID = id
		}
	}
	if utf8.RuneCountInString(filter.Username) > 256 || utf8.RuneCountInString(filter.Keyword) > 256 {
		common.ApiError(c, common.NewMessage("Search text must be 256 characters or fewer"))
		return
	}
	page := common.GetPageQuery(c)
	if page.Page < 1 || page.PageSize < 1 || page.Page > 1000000 {
		common.ApiError(c, common.NewMessage("Invalid page parameters"))
		return
	}
	export := c.Query("export") == "true"
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	records, total, err := model.GetTopUpHistory(ctx, filter, page, export)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if export {
		common.ApiSuccess(c, gin.H{"items": records, "total": total, "generated_at": common.GetTimestamp()})
		return
	}
	page.SetTotal(int(total))
	page.SetItems(records)
	common.ApiSuccess(c, page)
}

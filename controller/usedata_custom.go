package controller

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// maskUsername 对用户名做脱敏处理，如 "admin" -> "a***n"
func maskUsername(name string) string {
	runes := []rune(name)
	n := len(runes)
	if n <= 1 {
		return "***"
	}
	if n == 2 {
		return string(runes[0:1]) + "***"
	}
	return string(runes[0:1]) + "***" + string(runes[n-1:])
}

// GetQuotaDatesByUserAnonymized 返回脱敏的用户消耗排行数据，普通用户可访问
// 返回：data（脱敏后的时序数据）、self_rank（当前用户排名）、total（总用户数）
func GetQuotaDatesByUserAnonymized(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	dates, err := model.GetQuotaDataGroupByUser(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	currentUsername := c.GetString("username")

	// 1. 汇总每个用户的总消耗，用于计算排名
	userTotals := make(map[string]int)
	for _, d := range dates {
		userTotals[d.Username] += d.Quota
	}

	// 2. 按总消耗降序排列，分配排名
	type kv struct {
		Username string
		Total    int
	}
	var sortedUsers []kv
	for u, t := range userTotals {
		sortedUsers = append(sortedUsers, kv{u, t})
	}
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].Total > sortedUsers[j].Total
	})

	rankMap := make(map[string]int)
	for i, u := range sortedUsers {
		rankMap[u.Username] = i + 1
	}

	// 3. 找到当前用户的排名
	selfRank := 0
	if r, ok := rankMap[currentUsername]; ok {
		selfRank = r
	}

	// 4. 构建脱敏映射（同一个用户名映射到同一个脱敏名，避免重复脱敏结果不一致）
	maskedNames := make(map[string]string)
	usedMasked := make(map[string]int) // 用于处理脱敏后重名
	for _, u := range sortedUsers {
		if u.Username == currentUsername {
			maskedNames[u.Username] = u.Username
		} else {
			masked := maskUsername(u.Username)
			if count, exists := usedMasked[masked]; exists {
				// 脱敏后重名，加上序号区分
				usedMasked[masked] = count + 1
				masked = masked + strconv.Itoa(count+1)
			} else {
				usedMasked[masked] = 1
			}
			maskedNames[u.Username] = masked
		}
	}

	// 5. 对原始时序数据做脱敏
	for i := range dates {
		if m, ok := maskedNames[dates[i].Username]; ok {
			dates[i].Username = m
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "",
		"data":      dates,
		"self_rank": selfRank,
		"total":     len(sortedUsers),
	})
}

package controller

// custom: token ranking — Token consumption leaderboard API

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// maskUsername masks a username for privacy: "admin" → "a***n", "ab" → "a***b", "a" → "a***"
func maskUsername(name string) string {
	runes := []rune(name)
	if len(runes) <= 1 {
		return string(runes) + "***"
	}
	return string(runes[0:1]) + "***" + string(runes[len(runes)-1:])
}

type TokenRankResponse struct {
	Ranking    []TokenRankEntry `json:"ranking"`
	SelfRank   int              `json:"self_rank"`
	SelfTokens int64            `json:"self_tokens"`
	TotalUsers int              `json:"total_users"`
}

type TokenRankEntry struct {
	Username  string `json:"username"`
	TokenUsed int64  `json:"token_used"`
	IsSelf    bool   `json:"is_self"`
}

// GetTokenRanking returns the top 10 token consumers with privacy masking
// GET /api/data/token-ranking?start_timestamp=xxx&end_timestamp=xxx
func GetTokenRanking(c *gin.Context) {
	username := c.GetString("username")
	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser // admins see real usernames
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	// Limit time span to 30 days
	if endTimestamp-startTimestamp > 2592000 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "时间跨度不能超过 1 个月",
		})
		return
	}

	// Get top 10
	topItems, err := model.GetTokenRankingTop(startTimestamp, endTimestamp, 10)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Get current user's rank
	selfRank, selfTokens, totalUsers, err := model.GetUserTokenRank(username, startTimestamp, endTimestamp)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Build response with privacy masking
	ranking := make([]TokenRankEntry, 0, len(topItems))
	maskedNames := make(map[string]int) // track duplicate masked names

	for _, item := range topItems {
		isSelf := item.Username == username
		displayName := item.Username
		if !isSelf && !isAdmin {
			masked := maskUsername(item.Username)
			maskedNames[masked]++
			if maskedNames[masked] > 1 {
				masked = masked + "(" + strconv.Itoa(maskedNames[masked]) + ")"
			}
			displayName = masked
		}
		ranking = append(ranking, TokenRankEntry{
			Username:  displayName,
			TokenUsed: item.TokenUsed,
			IsSelf:    isSelf,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": TokenRankResponse{
			Ranking:    ranking,
			SelfRank:   selfRank,
			SelfTokens: selfTokens,
			TotalUsers: totalUsers,
		},
	})
}

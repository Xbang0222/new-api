package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type TokenRankingResponse struct {
	Ranking    []TokenRankingEntry `json:"ranking"`
	SelfRank   int                 `json:"self_rank"`
	SelfTokens int64               `json:"self_tokens"`
	TotalUsers int64               `json:"total_users"`
	Limit      int                 `json:"limit"`
	IsLimited  bool                `json:"is_limited"`
}

type TokenRankingEntry struct {
	Rank      int     `json:"rank"`
	UserID    int     `json:"user_id,omitempty"`
	Username  string  `json:"username"`
	TokenUsed int64   `json:"token_used"`
	Share     float64 `json:"share"`
	IsSelf    bool    `json:"is_self"`
}

func maskRankingUsername(username string, fallback string) string {
	if username == "" {
		username = fallback
	}
	runes := []rune(username)
	if len(runes) <= 1 {
		return username + "***"
	}
	if len(runes) == 2 {
		return string(runes[:1]) + "***"
	}
	return string(runes[:1]) + "***" + string(runes[len(runes)-1:])
}

func GetTokenRanking(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if startTimestamp > 0 && endTimestamp > 0 && endTimestamp < startTimestamp {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "end_timestamp must be greater than start_timestamp",
		})
		return
	}

	role := c.GetInt("role")
	userId := c.GetInt("id")
	username := c.GetString("username")
	isAdmin := role >= common.RoleAdminUser
	limit := 10
	if isAdmin {
		limit = 0
	}

	filterUsername := c.Query("username")
	query := model.TokenRankingQuery{
		StartTime: startTimestamp,
		EndTime:   endTimestamp,
		Username:  filterUsername,
		ModelName: c.Query("model_name"),
		Limit:     limit,
	}
	items, totalUsers, err := model.GetTokenRanking(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	selfRank := 0
	var selfTokens int64
	if !isAdmin || filterUsername == "" || filterUsername == username {
		selfRank, selfTokens, err = model.GetUserTokenRanking(model.TokenRankingQuery{
			StartTime: startTimestamp,
			EndTime:   endTimestamp,
			UserID:    userId,
			ModelName: c.Query("model_name"),
		})
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if selfRank == 0 && selfTokens == 0 && totalUsers > 0 {
			selfRank = int(totalUsers) + 1
		}
	}

	var totalTokens int64
	for _, item := range items {
		totalTokens += item.TokenUsed
	}

	maskedNames := make(map[string]int)
	ranking := make([]TokenRankingEntry, 0, len(items))
	for idx, item := range items {
		isSelf := item.UserID == userId || (item.UserID == 0 && item.Username == username)
		displayName := item.Username
		responseUserID := item.UserID
		if !isAdmin && !isSelf {
			displayName = maskRankingUsername(item.Username, "User")
			maskedNames[displayName]++
			if maskedNames[displayName] > 1 {
				displayName += " #" + strconv.Itoa(maskedNames[displayName])
			}
		}
		if !isAdmin {
			responseUserID = 0
		}
		if displayName == "" {
			displayName = "User #" + strconv.Itoa(item.UserID)
		}

		var share float64
		if totalTokens > 0 {
			share = float64(item.TokenUsed) / float64(totalTokens)
		}
		ranking = append(ranking, TokenRankingEntry{
			Rank:      idx + 1,
			UserID:    responseUserID,
			Username:  displayName,
			TokenUsed: item.TokenUsed,
			Share:     share,
			IsSelf:    isSelf,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": TokenRankingResponse{
			Ranking:    ranking,
			SelfRank:   selfRank,
			SelfTokens: selfTokens,
			TotalUsers: totalUsers,
			Limit:      limit,
			IsLimited:  !isAdmin,
		},
	})
}

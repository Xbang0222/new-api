package model

// custom: token ranking — Token consumption leaderboard queries

// TokenRankItem represents a single entry in the token ranking
type TokenRankItem struct {
	Username  string `json:"username"`
	TokenUsed int64  `json:"token_used"`
}

// GetTokenRankingTop returns top N users by token consumption within a time range
func GetTokenRankingTop(startTime, endTime int64, limit int) ([]TokenRankItem, error) {
	var items []TokenRankItem
	err := DB.Table("quota_data").
		Select("username, SUM(token_used) as token_used").
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("username").
		Order("token_used DESC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// GetUserTokenRank returns the user's rank (1-based) and total token consumption
// Also returns total number of users with token usage in the time range
func GetUserTokenRank(username string, startTime, endTime int64) (rank int, tokenUsed int64, totalUsers int, err error) {
	// Get user's total token consumption
	type Result struct {
		TokenUsed int64
	}
	var userResult Result
	err = DB.Table("quota_data").
		Select("COALESCE(SUM(token_used), 0) as token_used").
		Where("username = ? AND created_at >= ? AND created_at <= ?", username, startTime, endTime).
		Scan(&userResult).Error
	if err != nil {
		return 0, 0, 0, err
	}
	tokenUsed = userResult.TokenUsed

	if tokenUsed == 0 {
		// User has no consumption, count total users and return rank as totalUsers+1
		var count int64
		err = DB.Table("quota_data").
			Select("COUNT(DISTINCT username)").
			Where("created_at >= ? AND created_at <= ?", startTime, endTime).
			Count(&count).Error
		if err != nil {
			return 0, 0, 0, err
		}
		return int(count) + 1, 0, int(count), nil
	}

	// Count how many users have more tokens than current user (rank = count + 1)
	type CountResult struct {
		Cnt int64
	}
	var cntResult CountResult
	subQuery := DB.Table("quota_data").
		Select("username, SUM(token_used) as total").
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("username").
		Having("SUM(token_used) > ?", tokenUsed)
	err = DB.Table("(?) as sub", subQuery).Select("COUNT(*) as cnt").Scan(&cntResult).Error
	if err != nil {
		return 0, 0, 0, err
	}
	rank = int(cntResult.Cnt) + 1

	// Total users
	var totalResult CountResult
	subQueryTotal := DB.Table("quota_data").
		Select("username").
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Group("username")
	err = DB.Table("(?) as sub", subQueryTotal).Select("COUNT(*) as cnt").Scan(&totalResult).Error
	if err != nil {
		return 0, 0, 0, err
	}
	totalUsers = int(totalResult.Cnt)

	return rank, tokenUsed, totalUsers, nil
}
